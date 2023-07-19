package rssole

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

var (
	readLut   = map[string]time.Time{}
	readLutMu sync.Mutex
)

func loadReadLut() {
	readLutMu.Lock()
	defer readLutMu.Unlock()

	f, err := os.ReadFile("readcache.json")
	if err != nil {
		log.Println(err)
	} else {
		err = json.Unmarshal(f, &readLut)
		if err != nil {
			log.Fatalln("unmarshall readlut:", err)
		}
	}
}

func isUnread(url string) bool {
	_, found := readLut[url]
	return !found
}

func persistReadLut() {
	readLutMu.Lock()
	defer readLutMu.Unlock()

	jsonString, err := json.Marshal(readLut)
	if err != nil {
		log.Fatalln("marshalling readlut:", err)
	}
	err = os.WriteFile("readcache.json", jsonString, 0644)
	if err != nil {
		log.Fatalln("writefile readcache.json:", err)
	}
}
