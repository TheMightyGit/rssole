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
	Name     string  `json:"name,omitempty"`     // optional override name
	Category string  `json:"category,omitempty"` // optional grouping
	Scrape   *scrape `json:"scrape,omitempty"`

	ticker       *time.Ticker
	feed         *gofeed.Feed
	mu           sync.RWMutex
	wrappedItems []*wrappedItem
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
		pseudoRss, err := f.Scrape.GeneratePseudoRssFeed()
		if err != nil {
			return fmt.Errorf("rss GeneratePseudoRssFeed %s %w", f.URL, err)
		}
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

func (f *feed) StartTickedUpdate(updateTime time.Duration) {
	if f.ticker != nil {
		return // already running
	}
	go func() {
		if err := f.Update(); err != nil {
			log.Println("error during update of", f.URL, err)
		}
		f.ticker = time.NewTicker(updateTime)
		log.Println("Started update ticker of", updateTime, "for", f.URL)
		for range f.ticker.C {
			if err := f.Update(); err != nil {
				log.Println("error during update of", f.URL, err)
			}
		}
	}()
}

func (f *feed) StopTickedUpdate() {
	if f.ticker != nil {
		log.Println("Stopped update ticker for", f.URL)
		f.ticker.Stop()
		f.ticker = nil
	}
}

func (f *feed) ID() string {
	hash := md5.Sum([]byte(f.URL))
	return hex.EncodeToString(hash[:])
}
