package rssole

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/k3a/html2text"
	"github.com/mmcdole/gofeed"
	"golang.org/x/net/html"
)

type feed struct {
	URL      string  `json:"url"`
	Name     string  `json:"name"`     // optional override name
	Category string  `json:"category"` // optional grouping
	Scrape   *scrape `json:"scrape"`

	feed         *gofeed.Feed
	mu           sync.RWMutex
	id           string
	wrappedItems []*wrappedItem
	updateTime   time.Duration
}

func (f *feed) Link() string {
	return f.feed.Link
}

func (f *feed) Title() string {
	if f.Name != "" {
		return f.Name
	}
	if f.feed != nil {
		return f.feed.Title
	}
	return f.URL
}

func (f *feed) UnreadItemCount() int {
	if f.feed == nil {
		return 0
	}
	cnt := 0
	for _, item := range f.Items() {
		if item.IsUnread {
			cnt++
		}
	}
	return cnt
}

func (f *feed) Items() []*wrappedItem {
	return f.wrappedItems
}

func (f *feed) Update() {
	var err error

	fp := gofeed.NewParser()
	var feed *gofeed.Feed

	if f.Scrape != nil {
		pseudoRss := f.Scrape.GeneratePseudoRssFeed()
		feed, err = fp.ParseString(pseudoRss)
		if err != nil {
			log.Fatalln("rss parsestring", f.URL, err)
		}
	} else {
		feed, err = fp.ParseURL(f.URL)
		if err != nil {
			log.Println("rss parseurl", f.URL, err)
			return
		}
	}

	f.mu.Lock()
	f.feed = feed
	f.wrappedItems = make([]*wrappedItem, len(f.feed.Items))
	for idx, item := range f.feed.Items {
		f.wrappedItems[idx] = &wrappedItem{
			IsUnread: isUnread(item.Link),
			Feed:     f,
			Item:     item,
		}
	}

	sort.Slice(f.wrappedItems, func(i, j int) bool {
		// unread always higher than read
		if f.wrappedItems[i].IsUnread && !f.wrappedItems[j].IsUnread {
			return true
		}
		if !f.wrappedItems[i].IsUnread && f.wrappedItems[j].IsUnread {
			return false
		}

		iDate := f.wrappedItems[i].UpdatedParsed
		if iDate == nil {
			iDate = f.wrappedItems[i].PublishedParsed
		}

		jDate := f.wrappedItems[j].UpdatedParsed
		if jDate == nil {
			jDate = f.wrappedItems[j].PublishedParsed
		}

		if iDate != nil && jDate != nil {
			return jDate.Before(*iDate)
		}
		return false // retain current order
	})

	f.mu.Unlock()

	log.Println("Updated:", f.URL)
}

func (f *feed) StartTickedUpdate() {
	go func() {
		f.Update()
		ticker := time.NewTicker(f.updateTime)
		log.Println("Started update ticker of", f.updateTime, "for", f.URL)
		for range ticker.C {
			f.Update()
		}
	}()
}

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
				fmt.Println("removing", n.Data, "tag")
				toDelete = append(toDelete, n)
				return
			}
			if n.Data == "a" {
				fmt.Println("making", n.Data, "tag target new tab")
				n.Attr = append(n.Attr, html.Attribute{
					Namespace: "",
					Key:       "target",
					Val:       "_new",
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
	html.Render(renderBuf, doc)

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

func (f *feed) ID() string {
	hash := md5.Sum([]byte(f.URL))
	return hex.EncodeToString(hash[:])
}
