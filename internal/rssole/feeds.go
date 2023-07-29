package rssole

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type feeds struct {
	Feeds      []*feed `json:"feeds"`
	UpdateTime time.Duration
	Selected   string // FIXME: Ugh! viewer state held here is bad as we coud have mutiple simultaneous viewers.
	mu         sync.RWMutex
}

func (f *feeds) addFeed(feedToAdd *feed) {
	allFeeds.mu.Lock()
	defer allFeeds.mu.Unlock()

	feedToAdd.StartTickedUpdate(f.UpdateTime)
	allFeeds.Feeds = append(allFeeds.Feeds, feedToAdd)
}

func (f *feeds) delFeed(feedId string) {
	allFeeds.mu.Lock()
	defer allFeeds.mu.Unlock()

	newFeeds := []*feed{}
	for _, f := range f.Feeds {
		if f.ID() != feedId {
			newFeeds = append(newFeeds, f)
		} else {
			log.Println("Removed feed", f.URL)
		}
	}
	f.Feeds = newFeeds
}

func (f *feeds) getFeedByID(id string) *feed {
	allFeeds.mu.Lock()
	defer allFeeds.mu.Unlock()

	for _, f := range f.Feeds {
		if f.ID() == id {
			return f
		}
	}
	return nil
}

func (f *feeds) readFeedsFile(filename string) error {
	allFeeds.mu.Lock()
	defer allFeeds.mu.Unlock()

	jsonFile, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Error opening file: %w", err)
	}
	defer jsonFile.Close()

	d := json.NewDecoder(jsonFile)
	err = d.Decode(f)
	if err != nil {
		return fmt.Errorf("Error unmarshalling JSON: %w", err)
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
