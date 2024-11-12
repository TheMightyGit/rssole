package rssole

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/k3a/html2text"
	"github.com/mmcdole/gofeed"
	"github.com/mpvl/unique"
)

type wrappedItem struct {
	IsUnread bool
	Feed     *feed
	*gofeed.Item

	summary         *string
	description     *string
	images          *[]string
	onceDescription sync.Once
}

func (w *wrappedItem) MarkReadID() string {
	id := w.Link
	if id == "" {
		id = w.GUID
		if id == "" {
			id = url.QueryEscape(w.Title)
		}
	}

	return id
}

func (w *wrappedItem) Images() []string {
	if w.images != nil { // used cached version
		return *w.images
	}

	images := []string{}

	// standard supplied image
	if w.Item.Image != nil {
		images = append(images, w.Item.Image.URL)
	}

	// mastodon/gibiz images
	if media, found := w.Item.Extensions["media"]; found {
		if content, found := media["content"]; found {
			for _, v := range content {
				if v.Attrs["medium"] == "image" {
					imageURL := v.Attrs["url"]
					images = append(images, imageURL)
				}
			}
		}
	}

	// youtube style media:group
	group := w.Item.Extensions["media"]["group"]
	if len(group) > 0 {
		thumbnail := group[0].Children["thumbnail"]
		if len(thumbnail) > 0 {
			url := thumbnail[0].Attrs["url"]
			if url != "" {
				images = append(images, url)
			}
		}
	}

	// also add images found in enclosures
	for _, enclosure := range w.Enclosures {
		if strings.HasPrefix(enclosure.Type, "image/") {
			images = append(images, enclosure.URL)
		}
	}

	// Now... remove any meta images that are embedded in the description.
	// Ignore any query string args.

	dedupedImages := []string{}

	// Remove any image sources already within the description...
	for _, img := range images {
		srcNoQueryString := strings.Split(img, "?")[0]
		if !strings.Contains(w.Description(), srcNoQueryString) {
			dedupedImages = append(dedupedImages, img)
		} else {
			slog.Info("dedeuped meta image as already found in content", "src", img)
		}
	}

	// Remove any internal duplicates within the list...
	unique.Strings(&dedupedImages)

	w.images = &dedupedImages

	return *w.images
}

func (w *wrappedItem) Description() string {
	w.onceDescription.Do(func() {
		// create a list of descriptions from various sources,
		// we'll pick the longest later on.
		descSources := []*string{
			&w.Item.Description,
			&w.Item.Content,
		}

		// youtube style media:group ?
		group := w.Item.Extensions["media"]["group"]
		if len(group) > 0 {
			description := group[0].Children["description"]
			if len(description) > 0 {
				descSources = append(descSources, &description[0].Value)
			}
		}

		// IFLS a10 ?
		a10content := w.Item.Extensions["a10"]["content"]
		if len(a10content) > 0 {
			description := a10content[0].Value
			if len(description) > 0 {
				descSources = append(descSources, &description)
			}
		}

		var desc *string

		// pick the longest description as the story content
		for _, d := range descSources {
			if desc == nil || len(*desc) < len(*d) {
				desc = d
			}
		}

		// Now simplify the (potential) HTML by converting
		// it to and from markdown.

		// First convert rando HTML to Markdown....
		doc, err := htmltomarkdown.ConvertString(*desc)

		switch {
		case err != nil:
			slog.Warn("htmltomarkdown.ConvertString failed, returning unsanitised content", "error", err)

			w.description = desc
		case doc == "":
			slog.Warn("htmltomarkdown.ConvertString result blank, using original.")

			w.description = desc
		default:
			// parse markdown
			p := parser.NewWithExtensions(parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock)
			md := p.Parse([]byte(doc))

			absRoot := ""

			if u, err := url.Parse(w.Link); err == nil {
				// some stories (e.g. Go Blog) have root relative links, so we need to supply a root (of the site, not story).
				absRoot = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
			}

			// render to HTML (we choose to exclude embedded images and rely on them being passed in metadata)
			renderer := html.NewRenderer(html.RendererOptions{
				AbsolutePrefix: absRoot,
				Flags:          html.CommonFlags | html.HrefTargetBlank,
			})
			mdHTML := string(markdown.Render(md, renderer))
			w.description = &mdHTML
		}
	})

	return *w.description
}

const maxDescriptionLength = 200

func (w *wrappedItem) Summary() string {
	if w.summary != nil {
		return *w.summary
	}

	plainDesc := html2text.HTML2TextWithOptions(w.Description())
	if len(plainDesc) > maxDescriptionLength {
		plainDesc = plainDesc[:maxDescriptionLength]
	}

	plainDesc = strings.TrimSpace(plainDesc)

	// if summary is identical to title return nothing
	if plainDesc == w.Title {
		plainDesc = ""
	}

	// if summary is just a url then return nothing (hacker news does this)
	if _, err := url.ParseRequestURI(plainDesc); err == nil {
		plainDesc = ""
	}

	w.summary = &plainDesc

	return *w.summary
}

func (w *wrappedItem) ID() string {
	hash := md5.Sum([]byte(w.MarkReadID()))

	return hex.EncodeToString(hash[:])
}
