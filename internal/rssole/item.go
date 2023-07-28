package rssole

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"log"
	"net/url"
	"strings"

	"github.com/k3a/html2text"
	"github.com/mmcdole/gofeed"
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
					imageUrl := v.Attrs["url"]
					if !w.isDescriptionImage(imageUrl) {
						// fmt.Println(w.Description())
						// fmt.Printf("%v = %+v\n", k, imageUrl)
						images = append(images, imageUrl)
					}
				}
			}
		}
	}

	w.images = &images
	return *w.images
}

func (w *wrappedItem) isDescriptionImage(src string) bool {
	if w.descriptionImagesForDedupe == nil {
		// force lazy load if it hasn't already
		_ = w.Description()
	}
	for _, v := range *w.descriptionImagesForDedupe {
		// fmt.Println(v, "==", src)
		if v == src {
			return true
		}
	}
	return false
}

func (w *wrappedItem) Description() string {
	if w.description != nil { // used cached version
		return *w.description
	}

	// try and sanitise any html
	doc, err := html.Parse(strings.NewReader(w.Item.Description))
	if err != nil {
		// ...
		log.Println(err)
		return w.Item.Description
	}

	w.descriptionImagesForDedupe = &[]string{}
	toDelete := []*html.Node{}

	var f func(*html.Node)
	f = func(n *html.Node) {
		//fmt.Println(n)
		if n.Type == html.ElementNode {
			//fmt.Println(n.Data)
			if n.Data == "script" || n.Data == "style" || n.Data == "link" || n.Data == "meta" || n.Data == "iframe" {
				// fmt.Println("removing", n.Data, "tag")
				toDelete = append(toDelete, n)
				return
			}
			if n.Data == "a" {
				// fmt.Println("making", n.Data, "tag target new tab")
				n.Attr = append(n.Attr, html.Attribute{
					Namespace: "",
					Key:       "target",
					Val:       "_new",
				})
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
						*w.descriptionImagesForDedupe = append(*w.descriptionImagesForDedupe, a.Val)
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
	return *w.description
}

func (w *wrappedItem) Summary() string {
	if w.summary != nil {
		return *w.summary
	}

	plainDesc := html2text.HTML2Text(w.Item.Description)
	if len(plainDesc) > 200 {
		plainDesc = plainDesc[:200]
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
	hash := md5.Sum([]byte(w.Link))
	return hex.EncodeToString(hash[:])
}
