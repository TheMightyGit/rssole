package rssole

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	"golang.org/x/exp/slog"
)

type feed struct {
	URL        string            `json:"url"`
	Name       string            `json:"name,omitempty"`     // optional override name
	Category   string            `json:"category,omitempty"` // optional grouping
	Scrape     *scrape           `json:"scrape,omitempty"`
	RecentLogs *limitLinesBuffer `json:"-"`

	ticker       *time.Ticker
	feed         *gofeed.Feed
	mu           sync.RWMutex
	wrappedItems []*wrappedItem
	log          *slog.Logger

	eTag         string
	lastModified time.Time
}

var ErrNotModified = errors.New("not modified")

var gmtTimeZoneLocation *time.Location

func init() {
	loc, err := time.LoadLocation("GMT")
	if err != nil {
		panic(err)
	}

	gmtTimeZoneLocation = loc
}

const maxRecentLogLines = 30

type limitLinesBuffer struct {
	MaxLines int
	*bytes.Buffer
}

func (llw *limitLinesBuffer) Write(p []byte) (int, error) {
	n, err := llw.Buffer.Write(p)

	numLines := strings.Count(llw.Buffer.String(), "\n")
	if numLines > llw.MaxLines {
		cappedLines := strings.Join(
			strings.Split(llw.Buffer.String(), "\n")[numLines-maxRecentLogLines:],
			"\n",
		)

		llw.Buffer.Reset()
		llw.Buffer.WriteString(cappedLines)
	}

	return n, fmt.Errorf("limitLinesWriter error - %w", err)
}

func (f *feed) Init() {
	f.RecentLogs = &limitLinesBuffer{
		MaxLines: maxRecentLogLines,
		Buffer:   bytes.NewBufferString(""),
	}

	th := slog.NewTextHandler(io.MultiWriter(os.Stdout, f.RecentLogs), nil)
	f.log = slog.New(th).With("feed", f.URL)
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
	var feed *gofeed.Feed

	fp := gofeed.NewParser()

	if f.Scrape != nil {
		f.log.Info("Scraping website pages", "urls", f.Scrape.URLs)

		pseudoRss, err := f.Scrape.GeneratePseudoRssFeed()
		if err != nil {
			return fmt.Errorf("rss GeneratePseudoRssFeed %s %w", f.URL, err)
		}

		f.log.Info("Parsing pseudo feed")

		feed, err = fp.ParseString(pseudoRss)
		if err != nil {
			return fmt.Errorf("rss parsestring %s %w", f.URL, err)
		}
	} else {
		f.log.Info("Fetching and parsing feed", "url", f.URL)

		req, err := http.NewRequest(http.MethodGet, f.URL, nil)
		if err != nil {
			return fmt.Errorf("cannot create new request: %w", err)
		}
		req.Header.Set("User-Agent", "Gofeed/1.0")

		if f.eTag != "" {
			req.Header.Set("If-None-Match", fmt.Sprintf(`"%s"`, f.eTag))
		}

		req.Header.Set("If-Modified-Since", f.lastModified.In(gmtTimeZoneLocation).Format(time.RFC1123))

		client := &http.Client{} // FIXME: better defaults!
		resp, err := client.Do(req)

		if err != nil {
			return fmt.Errorf("unable to do request: %w", err)
		}

		if resp != nil {
			defer func() {
				ce := resp.Body.Close()
				if ce != nil {
					err = ce
				}
			}()
		}

		if resp.StatusCode == http.StatusNotModified {
			return ErrNotModified
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return gofeed.HTTPError{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
			}
		}

		if eTag := resp.Header.Get("Etag"); eTag != "" {
			f.eTag = eTag
		}

		if lastModified := resp.Header.Get("Last-Modified"); lastModified != "" {
			parsed, err := time.ParseInLocation(time.RFC1123, lastModified, gmtTimeZoneLocation)
			if err == nil {
				f.lastModified = parsed
			}
		}

		feed, err = fp.Parse(resp.Body)
		if err != nil {
			return fmt.Errorf("rss parseurl %s %w", f.URL, err)
		}
	}

	f.mu.Lock()
	f.feed = feed
	f.wrappedItems = make([]*wrappedItem, len(f.feed.Items))

	f.log.Info("Items in feed", "length", len(f.feed.Items))

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

	f.log.Info("Finished updating feed")

	// extend the life of anything valid still in the
	// read cache.
	for _, wi := range f.wrappedItems {
		readLut.extendLifeIfFound(wi.MarkReadID())
	}

	readLut.persistReadLut()

	updateLastmodified()

	return nil
}

func (f *feed) StartTickedUpdate(updateTime time.Duration) {
	if f.ticker != nil {
		return // already running
	}

	f.log.Info("Starting feed update ticker", "duration", updateTime)
	f.ticker = time.NewTicker(updateTime)

	go func() {
		if err := f.Update(); err != nil {
			f.log.Error("update failed", "error", err)
		}

		for range f.ticker.C {
			if err := f.Update(); err != nil {
				f.log.Error("update failed", "error", err)
			}
		}
	}()
}

func (f *feed) StopTickedUpdate() {
	if f.ticker != nil {
		f.log.Info("Stopped update ticker")
		f.ticker.Stop()
		f.ticker = nil
	}
}

func (f *feed) ID() string {
	hash := md5.Sum([]byte(f.URL))

	return hex.EncodeToString(hash[:])
}
