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
			log.Fatalln("unmarshall readlut:", err)
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
}

func (u *unreadLut) persistReadLut() {
	u.mu.Lock()
	defer u.mu.Unlock()

	jsonString, err := json.Marshal(u.lut)
	if err != nil {
		log.Fatalln("marshalling readlut:", err)
	}
	err = os.WriteFile(u.Filename, jsonString, 0644)
	if err != nil {
		log.Fatalln("writefile", u.Filename, ":", err)
	}
}
