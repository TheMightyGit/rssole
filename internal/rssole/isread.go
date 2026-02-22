package rssole

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"golang.org/x/exp/slog"
)

// Ensure unreadLut implements ReadCache.
var _ ReadCache = (*unreadLut)(nil)

type unreadLut struct {
	Filename string

	lut      map[string]time.Time
	mu       sync.RWMutex
	activity ActivityTracker // for updating last modified on markRead
}

func (u *unreadLut) loadReadLut() {
	u.mu.Lock()
	defer u.mu.Unlock()

	body, err := os.ReadFile(u.Filename)
	if err != nil {
		slog.Error("ReadFile failed", "filename", u.Filename, "error", err)
	} else {
		err = json.Unmarshal(body, &u.lut)
		if err != nil {
			slog.Error("error unmarshal", "filename", u.Filename, "error", err)
		}
	}
}

const (
	minusTwoDays    = -2 * time.Hour * 24 // 2 days ago
	updateFrequency = 1 * time.Hour
)

func (u *unreadLut) startCleanupTicker() {
	ago := minusTwoDays

	go func() {
		ticker := time.NewTicker(updateFrequency)
		for range ticker.C {
			before := time.Now().Add(ago)
			u.removeOldEntries(before)
			u.Persist()
		}
	}()
}

func (u *unreadLut) removeOldEntries(before time.Time) {
	u.mu.Lock()
	defer u.mu.Unlock()

	slog.Info("removing old readcache entries", "before", before)

	for url, when := range u.lut {
		if when.Before(before) {
			slog.Info("removing old readcache entry", "url", url, "when", when)
			delete(u.lut, url)
		}
	}
}

// IsUnread returns true if the item has not been marked as read.
func (u *unreadLut) IsUnread(id string) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()

	_, found := u.lut[id]

	return !found
}

// MarkRead marks an item as read.
func (u *unreadLut) MarkRead(id string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.lut == nil {
		u.lut = map[string]time.Time{}
	}

	u.lut[id] = time.Now()

	if u.activity != nil {
		u.activity.UpdateLastModified()
	}
}

// ExtendLifeIfFound extends the cache lifetime of an item if it exists.
func (u *unreadLut) ExtendLifeIfFound(id string) {
	if !u.IsUnread(id) {
		u.MarkRead(id)
	}
}

const lutFilePerms = 0o644

// Persist saves the read cache to disk.
func (u *unreadLut) Persist() {
	u.mu.Lock()
	defer u.mu.Unlock()

	jsonString, err := json.Marshal(u.lut)
	if err != nil {
		slog.Error("error marshaling readlut", "error", err)

		return
	}

	err = os.WriteFile(u.Filename, jsonString, lutFilePerms)
	if err != nil {
		slog.Error("error writefile", "filename", u.Filename, "error", err)
	}
}
