package rssole

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/k3a/html2text"
	"github.com/mmcdole/gofeed"
	"golang.org/x/exp/slog"
	"golang.org/x/net/html"
)

type wrappedItem struct {
	IsUnread bool
	Feed     *feed
	*gofeed.Item

	summary                    *string
	description                *string
	descriptionImagesForDedupe *[]string
	images                     *[]string
	onceDescription            sync.Once
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

	// NOTE: we exclude images that already appear in the description (gibiz)

	// standard supplied image
	if w.Item.Image != nil {
		if !w.isDescriptionImage(w.Item.Image.URL) {
			// fmt.Println(w.Item.Image.URL)
			images = append(images, w.Item.Image.URL)
		}
	}

	// mastodon/gibiz images
	if media, found := w.Item.Extensions["media"]; found {
		if content, found := media["content"]; found {
			for _, v := range content {
				if v.Attrs["medium"] == "image" {
					imageURL := v.Attrs["url"]
					if !w.isDescriptionImage(imageURL) {
						// fmt.Println(w.Description())
						// fmt.Printf("%v = %+v\n", k, imageUrl)
						images = append(images, imageURL)
					}
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

	w.images = &images

	return *w.images
}

func (w *wrappedItem) isDescriptionImage(src string) bool {
	// strip anything after ? to get rid of query string part
	srcNoQueryString := strings.Split(src, "?")[0]

	if w.descriptionImagesForDedupe == nil {
		// force lazy load if it hasn't already
		_ = w.Description()
	}

	for _, v := range *w.descriptionImagesForDedupe {
		// fmt.Println(v, "==", src)
		if v == srcNoQueryString {
			return true
		}
	}

	return false
}

var (
	tagsToRemoveRe  = regexp.MustCompile("script|style|link|meta|iframe|form")
	attrsToRemoveRe = regexp.MustCompile("style|class|hx-.*|data-.*|srcset|width|height|sizes|loading|decoding|target")
)

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

		// try and sanitise any html
		doc, err := html.Parse(strings.NewReader(*desc))
		if err != nil {
			// failed to sanitise, so just return as is...
			slog.Warn("html.Parse failed, returning unsanitised content", "error", err)
			w.description = desc
		} else {
			w.descriptionImagesForDedupe = &[]string{}
			toDelete := []*html.Node{}

			var f func(*html.Node)
			f = func(n *html.Node) {
				// fmt.Println(n)
				if n.Type == html.ElementNode {
					// fmt.Println(n.Data)

					if tagsToRemoveRe.MatchString(n.Data) {
						// fmt.Println("removing", n.Data, "tag")
						toDelete = append(toDelete, n)

						return
					}

					allowedAttrs := []html.Attribute{}
					for i := range n.Attr {
						if !attrsToRemoveRe.MatchString(n.Attr[i].Key) {
							allowedAttrs = append(allowedAttrs, n.Attr[i])
						}
					}
					n.Attr = allowedAttrs

					if n.Data == "a" {
						// fmt.Println("making", n.Data, "tag target new tab")
						n.Attr = append(n.Attr, html.Attribute{
							Namespace: "",
							Key:       "target",
							Val:       "_new",
						})
						// disable href if it starts with #
						for i := range n.Attr {
							if n.Attr[i].Key == "href" && n.Attr[i].Val[0] == '#' {
								n.Attr[i].Key = "xxxhref" // easier than removing the attr

								break
							}
						}
					}

					if n.Data == "img" || n.Data == "svg" {
						// fmt.Println("making", n.Data, "tag style max-width 60%")
						n.Attr = append(n.Attr, html.Attribute{
							Namespace: "",
							Key:       "style",
							Val:       "max-width: 60%;",
						})
						// keep a note of images so we can de-dupe attached
						// images that also appear in the content.
						for _, a := range n.Attr {
							if a.Key == "src" {
								// strip anything after ? to get rid of query string part
								bits := strings.Split(a.Val, "?")
								*w.descriptionImagesForDedupe = append(*w.descriptionImagesForDedupe, bits[0])
							}
						}
					}
				}

				for c := n.FirstChild; c != nil; c = c.NextSibling {
					f(c)
				}
			}
			f(doc)

			for _, n := range toDelete {
				n.Parent.RemoveChild(n)
			}

			renderBuf := bytes.NewBufferString("")
			_ = html.Render(renderBuf, doc)
			desc := renderBuf.String()
			w.description = &desc
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
