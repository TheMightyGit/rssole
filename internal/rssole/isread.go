package rssole

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

type unreadLut struct {
	Filename string

	lut map[string]time.Time
	mu  sync.RWMutex
}

func (u *unreadLut) loadReadLut() {
	u.mu.Lock()
	defer u.mu.Unlock()

	body, err := os.ReadFile(u.Filename)
	if err != nil {
		log.Println(err)
	} else {
		err = json.Unmarshal(body, &u.lut)
		if err != nil {
			log.Println("error unmarshall readlut:", err)
		}
	}
}

const (
	minusSixtyDays  = -60 * time.Hour * 24 // 60 days ago
	updateFrequency = 6 * time.Hour
)

func (u *unreadLut) startCleanupTicker() {
	ago := minusSixtyDays
	before := time.Now().Add(ago)
	readLut.removeOldEntries(before)
	readLut.persistReadLut()

	go func() {
		ticker := time.NewTicker(updateFrequency)
		for range ticker.C {
			before = time.Now().Add(ago)
			readLut.removeOldEntries(before)
			readLut.persistReadLut()
		}
	}()
}

func (u *unreadLut) removeOldEntries(before time.Time) {
	u.mu.Lock()
	defer u.mu.Unlock()

	log.Println("removing readcache entries before", before)

	for url, when := range u.lut {
		if when.Before(before) {
			log.Println("removing old from readcache", url, when)
			delete(u.lut, url)
		}
	}
}

func (u *unreadLut) isUnread(url string) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()

	_, found := u.lut[url]

	return !found
}

func (u *unreadLut) markRead(url string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.lut == nil {
		u.lut = map[string]time.Time{}
	}

	u.lut[url] = time.Now()

	updateLastmodified()
}

const lutFilePerms = 0o644

func (u *unreadLut) persistReadLut() {
	u.mu.Lock()
	defer u.mu.Unlock()

	jsonString, err := json.Marshal(u.lut)
	if err != nil {
		log.Println("error marshalling readlut:", err)

		return
	}

	err = os.WriteFile(u.Filename, jsonString, lutFilePerms)
	if err != nil {
		log.Println("error writefile", u.Filename, ":", err)
	}
}
