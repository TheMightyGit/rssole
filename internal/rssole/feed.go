package rssole

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
)

// global last modified for use in Last-Modified/If-Modified-Since
var lastmodified time.Time

type feed struct {
	URL        string        `json:"url"`
	Name       string        `json:"name,omitempty"`     // optional override name
	Category   string        `json:"category,omitempty"` // optional grouping
	Scrape     *scrape       `json:"scrape,omitempty"`
	RecentLogs *bytes.Buffer `json:"-"`

	ticker       *time.Ticker
	feed         *gofeed.Feed
	mu           sync.RWMutex
	wrappedItems []*wrappedItem
	log          *log.Logger
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
	var (
		err  error
		feed *gofeed.Feed
	)

	fp := gofeed.NewParser()

	if f.Scrape != nil {
		f.Logln("Scraping website pages:", f.Scrape.URLs)

		pseudoRss, err := f.Scrape.GeneratePseudoRssFeed()
		if err != nil {
			return fmt.Errorf("rss GeneratePseudoRssFeed %s %w", f.URL, err)
		}

		f.Logln("Parsing pseudo feed")

		feed, err = fp.ParseString(pseudoRss)
		if err != nil {
			return fmt.Errorf("rss parsestring %s %w", f.URL, err)
		}
	} else {
		f.Logln("Fetching and parsing feed:", f.URL)
		feed, err = fp.ParseURL(f.URL)
		if err != nil {
			return fmt.Errorf("rss parseurl %s %w", f.URL, err)
		}
	}

	f.mu.Lock()
	f.feed = feed
	f.wrappedItems = make([]*wrappedItem, len(f.feed.Items))

	f.Logln("Items in feed:", len(f.feed.Items))

	for idx, item := range f.feed.Items {
		wItem := &wrappedItem{
			Feed: f,
			Item: item,
		}
		wItem.IsUnread = readLut.isUnread(wItem.MarkReadID())
		f.wrappedItems[idx] = wItem
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

	f.Logln("Finished updating feed:", f.URL)
	lastmodified = time.Now()

	return nil
}

const maxRecentLogLines = 30

func (f *feed) Logln(args ...any) {
	log.Println(args...)

	if f.log == nil {
		f.RecentLogs = bytes.NewBufferString("")
		f.log = log.New(f.RecentLogs, "", log.LstdFlags)
	}

	f.log.Println(args...)

	numLines := strings.Count(f.RecentLogs.String(), "\n")
	if numLines > maxRecentLogLines {
		cappedLines := strings.Join(
			strings.Split(f.RecentLogs.String(), "\n")[numLines-maxRecentLogLines:],
			"\n",
		)

		f.RecentLogs.Reset()
		f.RecentLogs.WriteString(cappedLines)
	}
}

func (f *feed) StartTickedUpdate(updateTime time.Duration) {
	if f.ticker != nil {
		return // already running
	}

	go func() {
		if err := f.Update(); err != nil {
			f.Logln("error during update of", f.URL, err)
		}

		f.Logln("Starting update ticker of", updateTime, "for", f.URL)

		f.ticker = time.NewTicker(updateTime)
		for range f.ticker.C {
			if err := f.Update(); err != nil {
				f.Logln("error during update of", f.URL, err)
			}
		}
	}()
}

func (f *feed) StopTickedUpdate() {
	if f.ticker != nil {
		f.Logln("Stopped update ticker for", f.URL)
		f.ticker.Stop()
		f.ticker = nil
	}
}

func (f *feed) ID() string {
	hash := md5.Sum([]byte(f.URL))

	return hex.EncodeToString(hash[:])
}
