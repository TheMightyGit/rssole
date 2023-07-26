package rssole

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

type feeds struct {
	Feeds    []*feed `json:"feeds"`
	Selected string  // FIXME: Ugh! viewer state held here is bad as we coud have mutiple simultaneous viewers.
	mu       sync.RWMutex
}

func (f *feeds) readFeedsFile(filename string) error {
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

func (f *feeds) BeginFeedUpdates(updateTime time.Duration) {
	// ignore cert errors
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, feed := range f.Feeds {
		feed.updateTime = updateTime
		feed.StartTickedUpdate()
	}
}
