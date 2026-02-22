package rssole

import "sync/atomic"

// feedList is a copy-on-write list of feeds.
// Reads are lock-free; writes clone-modify-swap.
type feedList struct {
	ptr atomic.Pointer[[]*feed]
}

func newFeedList() *feedList {
	fl := &feedList{}
	fl.ptr.Store(&[]*feed{})

	return fl
}

// All returns the current snapshot of feeds.
// Safe to iterate without locks.
func (fl *feedList) All() []*feed {
	return *fl.ptr.Load()
}

// Add appends a feed to the list.
func (fl *feedList) Add(f *feed) {
	for {
		old := fl.ptr.Load()
		newSlice := make([]*feed, len(*old)+1)
		copy(newSlice, *old)
		newSlice[len(*old)] = f

		if fl.ptr.CompareAndSwap(old, &newSlice) {
			return
		}
	}
}

// Remove deletes a feed by ID.
// Returns the removed feed, or nil if not found.
func (fl *feedList) Remove(id string) *feed {
	for {
		old := fl.ptr.Load()
		idx := -1

		for i, f := range *old {
			if f.ID() == id {
				idx = i

				break
			}
		}

		if idx == -1 {
			return nil
		}

		removed := (*old)[idx]
		newSlice := make([]*feed, len(*old)-1)
		copy(newSlice[:idx], (*old)[:idx])
		copy(newSlice[idx:], (*old)[idx+1:])

		if fl.ptr.CompareAndSwap(old, &newSlice) {
			return removed
		}
	}
}

// Find returns the feed with the given ID, or nil.
func (fl *feedList) Find(id string) *feed {
	for _, f := range fl.All() {
		if f.ID() == id {
			return f
		}
	}

	return nil
}

// FindByURL returns the feed with the given URL, or nil.
func (fl *feedList) FindByURL(url string) *feed {
	for _, f := range fl.All() {
		if f.URL == url {
			return f
		}
	}

	return nil
}

// Set replaces the entire list (used on initial load).
func (fl *feedList) Set(feeds []*feed) {
	fl.ptr.Store(&feeds)
}
