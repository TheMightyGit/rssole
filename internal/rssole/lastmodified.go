package rssole

import (
	"sync"
	"time"
)

// global last modified for use in Last-Modified/If-Modified-Since
var (
	muLastmodified sync.Mutex
	lastmodified   time.Time
)

func updateLastmodified() {
	muLastmodified.Lock()
	lastmodified = time.Now()
	muLastmodified.Unlock()
}

func getLastmodified() time.Time {
	muLastmodified.Lock()
	defer muLastmodified.Unlock()

	return lastmodified
}
