package rssole

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/exp/slog"
)

type feeds struct {
	Config     ConfigSection `json:"config"`
	Feeds      []*feed       `json:"feeds"`
	UpdateTime time.Duration `json:"-"`
	Selected   string        `json:"-"` // FIXME: Ugh! viewer state held here is bad as we coud have multiple simultaneous viewers.
	mu         sync.RWMutex
	filename   string
}

type ConfigSection struct {
	Listen        string `json:"listen"`
	UpdateSeconds int    `json:"update_seconds"`
}

func (f *feeds) addFeed(feedToAdd *feed) {
	f.mu.Lock()
	defer f.mu.Unlock()

	feedToAdd.StartTickedUpdate(f.UpdateTime)
	f.Feeds = append(f.Feeds, feedToAdd)
}

func (f *feeds) delFeed(feedID string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	newFeeds := []*feed{}

	for _, f := range f.Feeds {
		if f.ID() != feedID {
			newFeeds = append(newFeeds, f)
		} else {
			slog.Info("Removed feed", "url", f.URL)
		}
	}

	f.Feeds = newFeeds
}

func (f *feeds) getFeedByID(id string) *feed {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, f := range f.Feeds {
		if f.ID() == id {
			return f
		}
	}

	return nil
}

func (f *feeds) readFeedsFile(filename string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.filename = filename

	jsonFile, err := os.Open(f.filename)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer jsonFile.Close()

	d := json.NewDecoder(jsonFile)

	err = d.Decode(f)
	if err != nil {
		return fmt.Errorf("error unmarshalling JSON: %w", err)
	}

	// NOTE: we must .Init() every loaded feed or logging will break
	for _, f := range f.Feeds {
		f.Init()
	}

	return nil
}

func (f *feeds) saveFeedsFile() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	jsonFile, err := os.Create(f.filename)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer jsonFile.Close()

	e := json.NewEncoder(jsonFile)
	e.SetIndent("", "  ")

	err = e.Encode(f)
	if err != nil {
		return fmt.Errorf("error marshalling JSON: %w", err)
	}

	return nil
}

func (f *feeds) FeedTree() map[string][]*feed {
	f.mu.RLock()
	defer f.mu.RUnlock()

	cats := map[string][]*feed{}
	for _, feed := range f.Feeds {
		cats[feed.Category] = append(cats[feed.Category], feed)
	}

	return cats
}

func (f *feeds) BeginFeedUpdates() {
	// ignore cert errors
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, feed := range f.Feeds {
		feed.StartTickedUpdate(f.UpdateTime)
	}
}

func (f *feeds) ChangeTickedUpdate(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Config.UpdateSeconds = int(d.Seconds())
	for _, feed := range f.Feeds {
		feed.ChangeTickedUpdate(d)
	}
}
