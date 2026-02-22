package rssole

import (
	"fmt"
	"sync"
	"testing"
)

func TestFeedList_Add(t *testing.T) {
	fl := newFeedList()

	f1 := &feed{URL: "http://example.com/1"}
	f2 := &feed{URL: "http://example.com/2"}

	fl.Add(f1)
	fl.Add(f2)

	all := fl.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 feeds, got %d", len(all))
	}

	if all[0] != f1 || all[1] != f2 {
		t.Fatal("feeds not in expected order")
	}
}

func TestFeedList_Remove(t *testing.T) {
	fl := newFeedList()

	f1 := &feed{URL: "http://example.com/1"}
	f2 := &feed{URL: "http://example.com/2"}
	f3 := &feed{URL: "http://example.com/3"}

	fl.Add(f1)
	fl.Add(f2)
	fl.Add(f3)

	// Remove middle element
	removed := fl.Remove(f2.ID())
	if removed != f2 {
		t.Fatal("expected to get removed feed back")
	}

	all := fl.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 feeds after removal, got %d", len(all))
	}

	if all[0] != f1 || all[1] != f3 {
		t.Fatal("wrong feeds remaining")
	}

	// Remove non-existent
	removed = fl.Remove("no-such-id")
	if removed != nil {
		t.Fatal("expected nil when removing non-existent feed")
	}
}

func TestFeedList_Find(t *testing.T) {
	fl := newFeedList()

	f1 := &feed{URL: "http://example.com/1"}
	f2 := &feed{URL: "http://example.com/2"}

	fl.Add(f1)
	fl.Add(f2)

	found := fl.Find(f1.ID())
	if found != f1 {
		t.Fatal("expected to find f1")
	}

	found = fl.Find(f2.ID())
	if found != f2 {
		t.Fatal("expected to find f2")
	}

	found = fl.Find("no-such-id")
	if found != nil {
		t.Fatal("expected nil for non-existent id")
	}
}

func TestFeedList_FindByURL(t *testing.T) {
	fl := newFeedList()

	f1 := &feed{URL: "http://example.com/1"}
	f2 := &feed{URL: "http://example.com/2"}

	fl.Add(f1)
	fl.Add(f2)

	found := fl.FindByURL("http://example.com/1")
	if found != f1 {
		t.Fatal("expected to find f1 by URL")
	}

	found = fl.FindByURL("http://example.com/2")
	if found != f2 {
		t.Fatal("expected to find f2 by URL")
	}

	found = fl.FindByURL("http://example.com/no-such")
	if found != nil {
		t.Fatal("expected nil for non-existent URL")
	}
}

func TestFeedList_Set(t *testing.T) {
	fl := newFeedList()

	f1 := &feed{URL: "http://example.com/1"}
	f2 := &feed{URL: "http://example.com/2"}

	fl.Set([]*feed{f1, f2})

	all := fl.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 feeds, got %d", len(all))
	}

	// Set replaces entirely
	f3 := &feed{URL: "http://example.com/3"}
	fl.Set([]*feed{f3})

	all = fl.All()
	if len(all) != 1 || all[0] != f3 {
		t.Fatal("Set should replace all feeds")
	}
}

func TestFeedList_ConcurrentAdd(t *testing.T) {
	fl := newFeedList()

	const n = 1000

	var wg sync.WaitGroup

	wg.Add(n)

	// All goroutines start at the same time to maximize contention
	start := make(chan struct{})

	for i := range n {
		go func(id int) {
			defer wg.Done()

			<-start

			f := &feed{URL: fmt.Sprintf("http://example.com/%d", id)}
			fl.Add(f)
		}(i)
	}

	close(start)
	wg.Wait()

	all := fl.All()
	if len(all) != n {
		t.Fatalf("lost updates: expected %d, got %d", n, len(all))
	}

	// Verify all unique URLs present
	seen := make(map[string]bool)

	for _, f := range all {
		if seen[f.URL] {
			t.Fatalf("duplicate feed URL: %s", f.URL)
		}

		seen[f.URL] = true
	}
}

func TestFeedList_ConcurrentRemove(t *testing.T) {
	fl := newFeedList()

	const n = 1000

	feeds := make([]*feed, n)

	for i := range n {
		feeds[i] = &feed{URL: fmt.Sprintf("http://example.com/%d", i)}
		fl.Add(feeds[i])
	}

	var wg sync.WaitGroup

	wg.Add(n)

	start := make(chan struct{})

	for i := range n {
		go func(id int) {
			defer wg.Done()

			<-start

			fl.Remove(feeds[id].ID())
		}(i)
	}

	close(start)
	wg.Wait()

	all := fl.All()
	if len(all) != 0 {
		t.Fatalf("expected 0 feeds after removing all, got %d", len(all))
	}
}

func TestFeedList_ConcurrentAddRemoveRead(t *testing.T) {
	fl := newFeedList()

	const n = 500

	var wg sync.WaitGroup

	// Pre-populate
	toRemove := make([]*feed, n)

	for i := range n {
		toRemove[i] = &feed{URL: fmt.Sprintf("http://example.com/remove-%d", i)}
		fl.Add(toRemove[i])
	}

	start := make(chan struct{})

	// Concurrent adds
	wg.Add(n)

	for i := range n {
		go func(id int) {
			defer wg.Done()

			<-start

			f := &feed{URL: fmt.Sprintf("http://example.com/add-%d", id)}
			fl.Add(f)
		}(i)
	}

	// Concurrent removes
	wg.Add(n)

	for i := range n {
		go func(id int) {
			defer wg.Done()

			<-start

			fl.Remove(toRemove[id].ID())
		}(i)
	}

	// Concurrent reads - verify we always get a consistent snapshot
	wg.Add(n)

	for range n {
		go func() {
			defer wg.Done()

			<-start

			all := fl.All()
			// Snapshot must be internally consistent
			seen := make(map[string]bool)

			for _, f := range all {
				if seen[f.URL] {
					t.Errorf("duplicate in snapshot: %s", f.URL)
				}

				seen[f.URL] = true
			}
		}()
	}

	close(start)
	wg.Wait()

	// Final state: all removes done, all adds done
	all := fl.All()
	if len(all) != n {
		t.Fatalf("expected %d feeds (added), got %d", n, len(all))
	}

	// Verify only "add-" URLs remain
	for _, f := range all {
		if f.URL[:len("http://example.com/add-")] != "http://example.com/add-" {
			t.Errorf("unexpected feed remained: %s", f.URL)
		}
	}
}
