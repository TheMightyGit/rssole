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

	summary *string
}

func (w *wrappedItem) Description() string {
	// TODO: cache to prevent overwork
	// try and sanitise any html
	doc, err := html.Parse(strings.NewReader(w.Item.Description))
	if err != nil {
		// ...
		log.Println(err)
		return w.Item.Description
	}

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

	return renderBuf.String()
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
