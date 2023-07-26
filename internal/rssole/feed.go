package rssole

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
)

type feed struct {
	URL      string  `json:"url"`
	Name     string  `json:"name"`     // optional override name
	Category string  `json:"category"` // optional grouping
	Scrape   *scrape `json:"scrape"`

	feed         *gofeed.Feed
	mu           sync.RWMutex
	wrappedItems []*wrappedItem
	updateTime   time.Duration
}

func (f *feed) Link() string {
	if f.feed != nil {
		return f.feed.Link
	}
	return ""
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

func (f *feed) Update() error {
	var err error

	fp := gofeed.NewParser()
	var feed *gofeed.Feed

	if f.Scrape != nil {
		pseudoRss := f.Scrape.GeneratePseudoRssFeed()
		feed, err = fp.ParseString(pseudoRss)
		if err != nil {
			return fmt.Errorf("rss parsestring %s %w", f.URL, err)
		}
	} else {
		feed, err = fp.ParseURL(f.URL)
		if err != nil {
			return fmt.Errorf("rss parseurl %s %w", f.URL, err)
		}
	}

	f.mu.Lock()
	f.feed = feed
	f.wrappedItems = make([]*wrappedItem, len(f.feed.Items))
	for idx, item := range f.feed.Items {
		f.wrappedItems[idx] = &wrappedItem{
			IsUnread: readLut.isUnread(item.Link),
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

	return nil
}

func (f *feed) StartTickedUpdate() {
	go func() {
		if err := f.Update(); err != nil {
			log.Println("error during update of", f.URL, err)
		}
		ticker := time.NewTicker(f.updateTime)
		log.Println("Started update ticker of", f.updateTime, "for", f.URL)
		for range ticker.C {
			if err := f.Update(); err != nil {
				log.Println("error during update of", f.URL, err)
			}
		}
	}()
}

func (f *feed) ID() string {
	hash := md5.Sum([]byte(f.URL))
	return hex.EncodeToString(hash[:])
}
