package rssole

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"golang.org/x/exp/slog"
)

const idleTimeout = 15 * time.Minute

func (f *feeds) triggerUpdates() {
	for _, fd := range f.list.All() {
		fd.RequestUpdate()
	}
}

type feeds struct {
	Config     ConfigSection `json:"config"`
	UpdateTime time.Duration `json:"-"`
	filename   string
	list       *feedList
}

// feedsJSON is used for JSON serialization only.
type feedsJSON struct {
	Config ConfigSection `json:"config"`
	Feeds  []*feed       `json:"feeds"`
}

func (f *feeds) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(&feedsJSON{
		Config: f.Config,
		Feeds:  f.list.All(),
	})
	if err != nil {
		return nil, fmt.Errorf("error marshalling feeds: %w", err)
	}

	return data, nil
}

func (f *feeds) UnmarshalJSON(data []byte) error {
	var fj feedsJSON
	if err := json.Unmarshal(data, &fj); err != nil {
		return fmt.Errorf("error unmarshalling feeds: %w", err)
	}

	f.Config = fj.Config

	if f.list == nil {
		f.list = newFeedList()
	}

	for _, fd := range fj.Feeds {
		fd.Init()
	}

	f.list.Set(fj.Feeds)

	return nil
}

type ConfigSection struct {
	Listen        string `json:"listen"`
	UpdateSeconds int    `json:"update_seconds"`
}

func (f *feeds) All() []*feed {
	return f.list.All()
}

func (f *feeds) addFeed(feedToAdd *feed, readCache ReadCache, activity ActivityTracker) {
	feedToAdd.StartTickedUpdate(f.UpdateTime, readCache, activity)
	f.list.Add(feedToAdd)
}

func (f *feeds) delFeed(feedID string) {
	if removed := f.list.Remove(feedID); removed != nil {
		removed.StopTickedUpdate()
		slog.Info("Removed feed", "url", removed.URL)
	}
}

func (f *feeds) getFeedByID(id string) *feed {
	return f.list.Find(id)
}

func (f *feeds) readFeedsFile(filename string) error {
	f.filename = filename
	f.list = newFeedList()

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

	return nil
}

func (f *feeds) saveFeedsFile() error {
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
	cats := map[string][]*feed{}
	for _, feed := range f.list.All() {
		cats[feed.Category] = append(cats[feed.Category], feed)
	}

	return cats
}

func (f *feeds) BeginFeedUpdates(readCache ReadCache, activity ActivityTracker) {
	// ignore cert errors
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	for _, feed := range f.list.All() {
		feed.StartTickedUpdate(f.UpdateTime, readCache, activity)
	}
}

func (f *feeds) ChangeTickedUpdate(d time.Duration) {
	f.Config.UpdateSeconds = int(d.Seconds())
	for _, feed := range f.list.All() {
		feed.ChangeTickedUpdate(d)
	}
}
