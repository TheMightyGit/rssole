package rssole

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// testEnv holds all the resources for an integration test.
type testEnv struct {
	svc     *Service
	tempDir string
	mux     *http.ServeMux
}

// newTestEnv creates a fully isolated test environment with its own Service instance.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "rssole_integration_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	svc := NewService()

	// Set up read cache with temp file
	readCacheFile := filepath.Join(tempDir, "readcache.json")

	if err := os.WriteFile(readCacheFile, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to create read cache file: %v", err)
	}

	svc.readLut.Filename = readCacheFile
	svc.readLut.activity = svc // wire up the activity tracker

	// Set up feeds file
	feedsFile := filepath.Join(tempDir, "feeds.json")
	svc.feeds.filename = feedsFile

	// Load templates
	if err := svc.loadTemplates(); err != nil {
		t.Fatalf("failed to load templates: %v", err)
	}

	// Set reasonable defaults
	svc.feeds.UpdateTime = 100 * time.Millisecond
	svc.feeds.Config.UpdateSeconds = 1

	// Create HTTP mux with all routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", svc.index)
	mux.HandleFunc("/feeds", svc.feedlist)
	mux.HandleFunc("/items", svc.items)
	mux.HandleFunc("/item", svc.item)
	mux.HandleFunc("/crudfeed", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			svc.crudfeedPost(w, r)
		} else {
			svc.crudfeedGet(w, r)
		}
	})
	mux.HandleFunc("/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			svc.settingsPost(w, r)
		} else {
			svc.settingsGet(w, r)
		}
	})

	return &testEnv{
		svc:     svc,
		tempDir: tempDir,
		mux:     mux,
	}
}

// cleanup removes all temp files and stops any running goroutines.
func (e *testEnv) cleanup() {
	// Stop all feed tickers
	for _, f := range e.svc.feeds.All() {
		f.StopTickedUpdate()
	}

	os.RemoveAll(e.tempDir)
}

// request makes an HTTP request to the test server.
func (e *testEnv) request(method, path string, body string) *httptest.ResponseRecorder {
	var req *http.Request

	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	rr := httptest.NewRecorder()
	e.mux.ServeHTTP(rr, req)

	return rr
}

// mockFeedServer creates a test HTTP server that serves RSS feed content.
func mockFeedServer(content string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, content)
	}))
}

// validRSSFeed returns a valid RSS feed XML with the given title and items.
func validRSSFeed(title string, items []string) string {
	itemsXML := ""

	for i, item := range items {
		itemsXML += fmt.Sprintf(`
    <item>
      <title>%s</title>
      <link>http://example.com/item/%d</link>
      <description>Description for %s</description>
    </item>`, item, i, item)
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
  <title>%s</title>
  <link>http://example.com</link>
  <description>Test feed</description>%s
</channel>
</rss>`, title, itemsXML)
}

// =============================================================================
// Full Request/Response Lifecycle Tests
// =============================================================================

func TestIntegration_IndexPage(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	rr := env.request(http.MethodGet, "/", "")

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()

	for _, expected := range []string{"<html", "<body", "hx-"} {
		if !strings.Contains(body, expected) {
			t.Errorf("expected response to contain %q", expected)
		}
	}
}

func TestIntegration_AddFeedAndViewItems(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Start a mock feed server
	feedContent := validRSSFeed("Test Feed", []string{"Article One", "Article Two"})
	feedServer := mockFeedServer(feedContent)

	defer feedServer.Close()

	// Add the feed via POST
	formData := url.Values{}
	formData.Set("url", feedServer.URL)
	formData.Set("name", "My Test Feed")
	formData.Set("category", "Testing")

	rr := env.request(http.MethodPost, "/crudfeed", formData.Encode())

	if rr.Code != http.StatusOK {
		t.Fatalf("failed to add feed: status %d, body: %s", rr.Code, rr.Body.String())
	}

	// Verify feed was added
	if len(env.svc.feeds.All()) != 1 {
		t.Fatalf("expected 1 feed, got %d", len(env.svc.feeds.All()))
	}

	feed := env.svc.feeds.All()[0]
	if feed.Name != "My Test Feed" {
		t.Errorf("expected feed name 'My Test Feed', got %q", feed.Name)
	}

	// Wait for the feed to update
	time.Sleep(200 * time.Millisecond)

	// View items for the feed
	rr = env.request(http.MethodGet, "/items?url="+url.QueryEscape(feedServer.URL), "")

	if rr.Code != http.StatusOK {
		t.Fatalf("failed to get items: status %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Article One") {
		t.Errorf("expected response to contain 'Article One', got: %s", body)
	}
}

func TestIntegration_MarkItemAsRead(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	feedContent := validRSSFeed("Read Test Feed", []string{"Unread Story"})
	feedServer := mockFeedServer(feedContent)

	defer feedServer.Close()

	// Add feed
	formData := url.Values{}
	formData.Set("url", feedServer.URL)
	formData.Set("name", "Read Test")

	env.request(http.MethodPost, "/crudfeed", formData.Encode())

	// Wait for update
	time.Sleep(200 * time.Millisecond)

	// Verify item is unread
	feed := env.svc.feeds.All()[0]
	if len(feed.Items()) == 0 {
		t.Fatal("no items in feed")
	}

	item := feed.Items()[0]
	if !item.IsUnread {
		t.Error("expected item to be unread initially")
	}

	// Mark as read via POST to /items
	markReadData := url.Values{}
	markReadData.Add("read", item.MarkReadID())

	env.request(http.MethodPost, "/items?url="+url.QueryEscape(feedServer.URL), markReadData.Encode())

	// Verify item is now read
	if item.IsUnread {
		t.Error("expected item to be marked as read")
	}

	// Verify it's persisted in the read cache
	if env.svc.readLut.IsUnread(item.MarkReadID()) {
		t.Error("expected item to be marked read in cache")
	}
}

func TestIntegration_DeleteFeed(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	feedServer := mockFeedServer(validRSSFeed("Delete Me", []string{"Item"}))
	defer feedServer.Close()

	// Add feed
	formData := url.Values{}
	formData.Set("url", feedServer.URL)
	formData.Set("name", "To Delete")

	env.request(http.MethodPost, "/crudfeed", formData.Encode())

	if len(env.svc.feeds.All()) != 1 {
		t.Fatal("feed was not added")
	}

	feedID := env.svc.feeds.All()[0].ID()

	// Delete the feed
	deleteData := url.Values{}
	deleteData.Set("id", feedID)
	deleteData.Set("delete", "delete")

	rr := env.request(http.MethodPost, "/crudfeed", deleteData.Encode())

	if rr.Code != http.StatusOK {
		t.Errorf("delete request failed: %d", rr.Code)
	}

	if len(env.svc.feeds.All()) != 0 {
		t.Errorf("expected 0 feeds after delete, got %d", len(env.svc.feeds.All()))
	}
}

// =============================================================================
// Feed Update Behavior Tests
// =============================================================================

func TestIntegration_FeedUpdateOnActivity(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	updateCount := 0
	mu := sync.Mutex{}

	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		updateCount++
		mu.Unlock()

		fmt.Fprint(w, validRSSFeed("Update Test", []string{"Item"}))
	}))
	defer feedServer.Close()

	// Add feed (this triggers BeginFeedUpdates on first activity)
	formData := url.Values{}
	formData.Set("url", feedServer.URL)
	formData.Set("name", "Update Test")

	env.request(http.MethodPost, "/crudfeed", formData.Encode())

	// The /feedlist endpoint records activity and triggers updates
	env.request(http.MethodGet, "/feeds", "")

	// Wait for initial update
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	initialCount := updateCount
	mu.Unlock()

	if initialCount < 1 {
		t.Errorf("expected at least 1 update, got %d", initialCount)
	}

	// Wait for ticker to fire again
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	finalCount := updateCount
	mu.Unlock()

	if finalCount <= initialCount {
		t.Errorf("expected more updates over time, got %d -> %d", initialCount, finalCount)
	}
}

func TestIntegration_IdleDetection(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Record activity
	env.svc.recordActivity()

	// Should not be idle immediately
	if env.svc.IsIdle() {
		t.Error("should not be idle immediately after activity")
	}

	// Manually set last activity to the past
	env.svc.lastActivityMu.Lock()
	env.svc.lastActivity = time.Now().Add(-20 * time.Minute)
	env.svc.lastActivityMu.Unlock()

	// Now should be idle
	if !env.svc.IsIdle() {
		t.Error("should be idle after 20 minutes of inactivity")
	}
}

// =============================================================================
// Persistence Roundtrip Tests
// =============================================================================

func TestIntegration_FeedsFilePersistence(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	feedServer := mockFeedServer(validRSSFeed("Persist Test", []string{"Item"}))
	defer feedServer.Close()

	// Add a feed
	formData := url.Values{}
	formData.Set("url", feedServer.URL)
	formData.Set("name", "Persisted Feed")
	formData.Set("category", "Saved")

	env.request(http.MethodPost, "/crudfeed", formData.Encode())

	// Stop the ticker before we reload
	env.svc.feeds.All()[0].StopTickedUpdate()

	// Create a new service and load the feeds file
	svc2 := NewService()

	err := svc2.feeds.readFeedsFile(env.svc.feeds.filename)
	if err != nil {
		t.Fatalf("failed to read feeds file: %v", err)
	}

	// Verify the feed was persisted
	if len(svc2.feeds.All()) != 1 {
		t.Fatalf("expected 1 feed after reload, got %d", len(svc2.feeds.All()))
	}

	reloadedFeed := svc2.feeds.All()[0]

	if reloadedFeed.URL != feedServer.URL {
		t.Errorf("URL mismatch: expected %q, got %q", feedServer.URL, reloadedFeed.URL)
	}

	if reloadedFeed.Name != "Persisted Feed" {
		t.Errorf("Name mismatch: expected 'Persisted Feed', got %q", reloadedFeed.Name)
	}

	if reloadedFeed.Category != "Saved" {
		t.Errorf("Category mismatch: expected 'Saved', got %q", reloadedFeed.Category)
	}
}

func TestIntegration_ReadCachePersistence(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Mark something as read
	env.svc.readLut.MarkRead("http://example.com/article/123")
	env.svc.readLut.Persist()

	// Create new unreadLut and load from same file
	lut2 := &unreadLut{
		Filename: env.svc.readLut.Filename,
	}
	lut2.loadReadLut()

	// Verify the read state was persisted
	if lut2.IsUnread("http://example.com/article/123") {
		t.Error("expected article to still be marked as read after reload")
	}

	// Verify unread items are still unread
	if !lut2.IsUnread("http://example.com/article/456") {
		t.Error("expected unknown article to be unread")
	}
}

// =============================================================================
// Concurrent Access Pattern Tests
// =============================================================================

func TestIntegration_ConcurrentFeedlistRequests(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	feedServer := mockFeedServer(validRSSFeed("Concurrent Test", []string{"Item"}))
	defer feedServer.Close()

	// Add a feed first
	formData := url.Values{}
	formData.Set("url", feedServer.URL)
	formData.Set("name", "Concurrent Feed")

	env.request(http.MethodPost, "/crudfeed", formData.Encode())

	time.Sleep(200 * time.Millisecond)

	// Hammer the feedlist endpoint concurrently
	const numRequests = 50

	var wg sync.WaitGroup

	wg.Add(numRequests)

	errors := make(chan error, numRequests)

	for range numRequests {
		go func() {
			defer wg.Done()

			rr := env.request(http.MethodGet, "/feeds", "")

			if rr.Code != http.StatusOK && rr.Code != http.StatusNotModified {
				errors <- fmt.Errorf("unexpected status: %d", rr.Code)
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestIntegration_ConcurrentMarkAsRead(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Add multiple items
	items := make([]string, 20)
	for i := range items {
		items[i] = fmt.Sprintf("Item %d", i)
	}

	feedServer := mockFeedServer(validRSSFeed("Concurrent Read", items))
	defer feedServer.Close()

	formData := url.Values{}
	formData.Set("url", feedServer.URL)
	formData.Set("name", "Concurrent Read Test")

	env.request(http.MethodPost, "/crudfeed", formData.Encode())

	time.Sleep(200 * time.Millisecond)

	feed := env.svc.feeds.All()[0]
	feedItems := feed.Items()

	if len(feedItems) == 0 {
		t.Fatal("no items in feed")
	}

	// Concurrently mark items as read
	var wg sync.WaitGroup

	for _, item := range feedItems {
		wg.Add(1)

		go func(markID string) {
			defer wg.Done()

			markData := url.Values{}
			markData.Add("read", markID)

			env.request(http.MethodPost, "/items?url="+url.QueryEscape(feedServer.URL), markData.Encode())
		}(item.MarkReadID())
	}

	wg.Wait()

	// Verify all items are marked as read
	for _, item := range feedItems {
		if env.svc.readLut.IsUnread(item.MarkReadID()) {
			t.Errorf("item %q should be marked as read", item.Title)
		}
	}
}

// =============================================================================
// Error Handling Path Tests
// =============================================================================

func TestIntegration_InvalidFeedURL(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Server that returns invalid content
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "This is not valid RSS")
	}))
	defer badServer.Close()

	formData := url.Values{}
	formData.Set("url", badServer.URL)
	formData.Set("name", "Bad Feed")

	env.request(http.MethodPost, "/crudfeed", formData.Encode())

	// Feed should be added but have an error state
	if len(env.svc.feeds.All()) != 1 {
		t.Fatal("feed should still be added even if invalid")
	}

	// Wait for update attempt
	time.Sleep(200 * time.Millisecond)

	feed := env.svc.feeds.All()[0]

	// Feed should have no parsed content
	if feed.feed != nil {
		t.Error("expected feed.feed to be nil for invalid RSS")
	}
}

func TestIntegration_FeedServerDown(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Create server and immediately close it
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, validRSSFeed("Test", []string{"Item"}))
	}))

	serverURL := badServer.URL
	badServer.Close() // Server is now unreachable

	formData := url.Values{}
	formData.Set("url", serverURL)
	formData.Set("name", "Offline Feed")

	rr := env.request(http.MethodPost, "/crudfeed", formData.Encode())

	// Adding the feed should succeed (we add it regardless of fetch success)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	if len(env.svc.feeds.All()) != 1 {
		t.Error("feed should be added even when server is down")
	}
}

func TestIntegration_NonexistentFeedInItemsRequest(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Request items for a feed that doesn't exist
	rr := env.request(http.MethodGet, "/items?url=http://nonexistent.example.com/feed", "")

	// Should return 200 but empty (no crash)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for nonexistent feed, got %d", rr.Code)
	}
}

func TestIntegration_LastModifiedCaching(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Make initial request to set lastmodified
	env.request(http.MethodGet, "/feeds", "")

	// Get the last modified time
	lastMod := env.svc.getLastmodified()

	// Make request with If-Modified-Since in the future
	req := httptest.NewRequest(http.MethodGet, "/feeds", nil)
	req.Header.Set("If-Modified-Since", lastMod.Add(time.Hour).Format(http.TimeFormat))

	rr := httptest.NewRecorder()
	env.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotModified {
		t.Errorf("expected 304 Not Modified, got %d", rr.Code)
	}
}

func TestIntegration_MarkAllAsReadThenRefresh(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Create a feed with multiple items
	feedContent := validRSSFeed("Unread Test", []string{"Item 1", "Item 2", "Item 3"})
	feedServer := mockFeedServer(feedContent)

	defer feedServer.Close()

	// Add the feed
	formData := url.Values{}
	formData.Set("url", feedServer.URL)
	formData.Set("name", "Unread Test Feed")

	env.request(http.MethodPost, "/crudfeed", formData.Encode())

	// Wait for feed to update
	time.Sleep(200 * time.Millisecond)

	// User loads the feed list page - should see 3 unread
	rr := env.request(http.MethodGet, "/feeds", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Capture Last-Modified header like browser would
	lastModified := rr.Header().Get("Last-Modified")
	if lastModified == "" {
		t.Fatal("expected Last-Modified header")
	}

	body := rr.Body.String()
	if !strings.Contains(body, ">3<") {
		t.Logf("Initial response: %s", body)
		t.Fatal("expected to see 3 unread items initially")
	}

	// User clicks on the feed to view items
	rr = env.request(http.MethodGet, "/items?url="+url.QueryEscape(feedServer.URL), "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// User marks all as read
	markReadData := url.Values{}
	markReadData.Add("read", "http://example.com/item/0")
	markReadData.Add("read", "http://example.com/item/1")
	markReadData.Add("read", "http://example.com/item/2")

	rr = env.request(http.MethodPost, "/items?url="+url.QueryEscape(feedServer.URL), markReadData.Encode())
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// User refreshes the page - browser sends If-Modified-Since from previous request
	req := httptest.NewRequest(http.MethodGet, "/feeds", nil)
	req.Header.Set("If-Modified-Since", lastModified)

	rr = httptest.NewRecorder()
	env.mux.ServeHTTP(rr, req)

	// If we get 304, the browser would show cached (old) content - that's the bug
	if rr.Code == http.StatusNotModified {
		t.Errorf("BUG: got 304 Not Modified after marking items as read - browser will show stale unread count")
	}

	// If we get 200, check the body has updated count
	if rr.Code == http.StatusOK {
		body = rr.Body.String()
		if strings.Contains(body, ">3<") {
			t.Errorf("BUG: response still shows old unread count of 3 after marking all as read\nResponse: %s", body)
		}
	}
}

func TestIntegration_FeedListCountMatchesItemsCount(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Create a feed with multiple items
	feedContent := validRSSFeed("Count Test", []string{"Item 1", "Item 2", "Item 3", "Item 4", "Item 5"})
	feedServer := mockFeedServer(feedContent)

	defer feedServer.Close()

	// Add the feed
	formData := url.Values{}
	formData.Set("url", feedServer.URL)
	formData.Set("name", "Count Test Feed")

	env.request(http.MethodPost, "/crudfeed", formData.Encode())

	// Wait for feed to update
	time.Sleep(200 * time.Millisecond)

	// Mark some items as read (not all)
	markReadData := url.Values{}
	markReadData.Add("read", "http://example.com/item/0")
	markReadData.Add("read", "http://example.com/item/1")

	env.request(http.MethodPost, "/items?url="+url.QueryEscape(feedServer.URL), markReadData.Encode())

	// Now get the feed list - what count does it show?
	rrFeeds := env.request(http.MethodGet, "/feeds", "")
	if rrFeeds.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rrFeeds.Code)
	}
	feedsBody := rrFeeds.Body.String()

	// Get the items page - what count does it show?
	rrItems := env.request(http.MethodGet, "/items?url="+url.QueryEscape(feedServer.URL), "")
	if rrItems.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rrItems.Code)
	}
	itemsBody := rrItems.Body.String()

	// Count unread in items response (look for unread indicator in HTML)
	// The items template should show which items are unread
	t.Logf("Feeds response:\n%s", feedsBody)
	t.Logf("Items response:\n%s", itemsBody)

	// After marking 2 of 5 as read, we should have 3 unread
	// Check feeds list shows 3
	if !strings.Contains(feedsBody, ">3<") {
		t.Errorf("BUG: feeds list does not show expected unread count of 3")
	}

	// The items response also includes an OOB feedlist update - check it matches
	// Count how many items have "unread" class or indicator in the items response
	unreadCount := strings.Count(itemsBody, "unread")
	t.Logf("Found %d occurrences of 'unread' in items response", unreadCount)
}

// TestIntegration_RefreshVsClickCountMismatch reproduces a bug where:
// - Refreshing the page shows one set of unread counts
// - Clicking on any feed shows different (higher) counts
//
// The issue is that /feeds uses If-Modified-Since caching, so on refresh the browser
// may get a 304 and show stale cached HTML. But /items returns fresh HTML with an
// OOB feedlist update that bypasses the cache.
func TestIntegration_RefreshVsClickCountMismatch(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Start with 2 items
	itemCount := 2
	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		items := make([]string, itemCount)
		for i := range items {
			items[i] = fmt.Sprintf("Item %d", i)
		}
		fmt.Fprint(w, validRSSFeed("Dynamic Feed", items))
	}))
	defer feedServer.Close()

	// Add the feed
	formData := url.Values{}
	formData.Set("url", feedServer.URL)
	formData.Set("name", "Dynamic Feed")

	env.request(http.MethodPost, "/crudfeed", formData.Encode())

	// Wait for initial feed update
	time.Sleep(200 * time.Millisecond)

	// === SIMULATE: User loads page for the first time ===
	// Browser requests /feeds (no If-Modified-Since yet)
	rr1 := env.request(http.MethodGet, "/feeds", "")
	if rr1.Code != http.StatusOK {
		t.Fatalf("expected 200 on first request, got %d", rr1.Code)
	}

	body1 := rr1.Body.String()
	lastModified1 := rr1.Header().Get("Last-Modified")
	t.Logf("First /feeds request: status=%d, Last-Modified=%s", rr1.Code, lastModified1)
	t.Logf("Body shows count: %s", body1)

	// Should show 2 unread items
	if !strings.Contains(body1, ">2<") {
		t.Errorf("expected 2 unread items initially, body: %s", body1)
	}

	// === SIMULATE: Feed gets more items (background update) ===
	itemCount = 5 // Now 5 items

	// Trigger a feed update
	feed := env.svc.feeds.All()[0]
	feed.Update()

	// Wait for update to complete
	time.Sleep(100 * time.Millisecond)

	// === SIMULATE: User refreshes the page ===
	// Browser sends If-Modified-Since from cached response
	req := httptest.NewRequest(http.MethodGet, "/feeds", nil)
	req.Header.Set("If-Modified-Since", lastModified1)

	rr2 := httptest.NewRecorder()
	env.mux.ServeHTTP(rr2, req)

	t.Logf("Refresh /feeds request with If-Modified-Since=%s: status=%d", lastModified1, rr2.Code)

	// BUG: If we get 304, the browser shows stale HTML with count=2
	// but the actual feed now has 5 items
	if rr2.Code == http.StatusNotModified {
		t.Logf("BUG REPRODUCED: Server returned 304, browser will show stale count of 2")
		t.Logf("But actual unread count is: %d", feed.UnreadItemCount())

		// Verify the actual count is different from what browser would show
		if feed.UnreadItemCount() != 2 {
			t.Errorf("BUG CONFIRMED: Browser shows cached count=2, but actual count=%d", feed.UnreadItemCount())
		}
	}

	// === SIMULATE: User clicks on the feed ===
	// /items returns fresh HTML with OOB feedlist update
	rr3 := env.request(http.MethodGet, "/items?url="+url.QueryEscape(feedServer.URL), "")
	if rr3.Code != http.StatusOK {
		t.Fatalf("expected 200 for /items, got %d", rr3.Code)
	}

	body3 := rr3.Body.String()
	t.Logf("Click /items response shows: %s", body3)

	// This should show the correct count of 5
	if !strings.Contains(body3, ">5<") {
		t.Errorf("expected /items OOB update to show 5 unread, body: %s", body3)
	}

	// === The mismatch ===
	// If rr2 was 304, browser shows count=2 (from cache)
	// After clicking, rr3 shows count=5 (fresh)
	// This is the bug the user is experiencing
	if rr2.Code == http.StatusNotModified {
		t.Error("BUG: Refresh returned 304 (stale count=2) but click returned fresh count=5")
	}
}

// TestIntegration_RefreshShowsStaleCountsNoFeedUpdates tests a scenario where:
// - User loads page, sees counts
// - Time passes but NO feed updates occur (upstream returns 304 not modified)
// - User refreshes - should still see correct counts, not stale cached HTML
//
// This tests whether the If-Modified-Since logic works correctly when feeds
// haven't changed but the browser has a cached response.
func TestIntegration_RefreshShowsStaleCountsNoFeedUpdates(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Track if feed server was hit
	requestCount := 0
	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// Always return the same content
		fmt.Fprint(w, validRSSFeed("Static Feed", []string{"Item 1", "Item 2", "Item 3"}))
	}))
	defer feedServer.Close()

	// Add the feed
	formData := url.Values{}
	formData.Set("url", feedServer.URL)
	formData.Set("name", "Static Feed")

	env.request(http.MethodPost, "/crudfeed", formData.Encode())

	// Wait for initial feed update
	time.Sleep(200 * time.Millisecond)

	// Stop the ticker to prevent background updates
	feed := env.svc.feeds.All()[0]
	feed.StopTickedUpdate()

	t.Logf("Initial request count to feed server: %d", requestCount)

	// === User loads page ===
	rr1 := env.request(http.MethodGet, "/feeds", "")
	lastModified1 := rr1.Header().Get("Last-Modified")
	t.Logf("First /feeds: status=%d, Last-Modified=%s", rr1.Code, lastModified1)

	body1 := rr1.Body.String()
	if !strings.Contains(body1, ">3<") {
		t.Fatalf("expected 3 unread items, got: %s", body1)
	}

	// === User marks one item as read ===
	markReadData := url.Values{}
	markReadData.Add("read", "http://example.com/item/0")
	env.request(http.MethodPost, "/items?url="+url.QueryEscape(feedServer.URL), markReadData.Encode())

	// Now we have 2 unread items
	t.Logf("After marking read, actual unread count: %d", feed.UnreadItemCount())

	// === User refreshes the page ===
	// Browser sends If-Modified-Since from first request
	req := httptest.NewRequest(http.MethodGet, "/feeds", nil)
	req.Header.Set("If-Modified-Since", lastModified1)

	rr2 := httptest.NewRecorder()
	env.mux.ServeHTTP(rr2, req)

	t.Logf("Refresh /feeds with If-Modified-Since=%s: status=%d", lastModified1, rr2.Code)

	if rr2.Code == http.StatusNotModified {
		t.Error("BUG: Got 304 after marking item as read - browser will show stale count of 3 instead of 2")
	} else if rr2.Code == http.StatusOK {
		body2 := rr2.Body.String()
		if strings.Contains(body2, ">3<") {
			t.Errorf("BUG: Response shows old count of 3, should be 2. Body: %s", body2)
		}
		if !strings.Contains(body2, ">2<") {
			t.Errorf("Expected count of 2, got: %s", body2)
		}
	}

	// === User clicks on feed ===
	rr3 := env.request(http.MethodGet, "/items?url="+url.QueryEscape(feedServer.URL), "")
	body3 := rr3.Body.String()

	// Should show 2 unread
	if !strings.Contains(body3, ">2<") {
		t.Errorf("Expected /items to show 2 unread, got: %s", body3)
	}
}

// TestIntegration_BrowserCacheStaleness tests what happens when:
// 1. Browser caches a /feeds response with Last-Modified header
// 2. Data changes server-side
// 3. Browser keeps sending the OLD If-Modified-Since (from step 1) on every request
//
// This simulates mobile pull-to-refresh behavior where the browser doesn't
// update its cached If-Modified-Since value between requests.
func TestIntegration_BrowserCacheStaleness(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	feedServer := mockFeedServer(validRSSFeed("Cache Test", []string{"Item 1", "Item 2", "Item 3"}))
	defer feedServer.Close()

	// Add feed
	formData := url.Values{}
	formData.Set("url", feedServer.URL)
	formData.Set("name", "Cache Test Feed")
	env.request(http.MethodPost, "/crudfeed", formData.Encode())

	time.Sleep(200 * time.Millisecond)

	// === Browser loads page for first time ===
	rr1 := env.request(http.MethodGet, "/feeds", "")
	cachedLastModified := rr1.Header().Get("Last-Modified")
	t.Logf("Initial request: status=%d, Last-Modified=%s", rr1.Code, cachedLastModified)

	if !strings.Contains(rr1.Body.String(), ">3<") {
		t.Fatalf("Expected 3 unread initially")
	}

	// === Time passes, user marks items as read ===
	markData := url.Values{}
	markData.Add("read", "http://example.com/item/0")
	env.request(http.MethodPost, "/items?url="+url.QueryEscape(feedServer.URL), markData.Encode())

	t.Logf("After marking read, server lastModified=%s", env.svc.getLastmodified().Format(http.TimeFormat))

	// === User does pull-to-refresh ===
	// Browser sends If-Modified-Since from its cache (the OLD value from step 1)
	req := httptest.NewRequest(http.MethodGet, "/feeds", nil)
	req.Header.Set("If-Modified-Since", cachedLastModified)

	rr2 := httptest.NewRecorder()
	env.mux.ServeHTTP(rr2, req)

	t.Logf("Pull-to-refresh with cached If-Modified-Since=%s: status=%d", cachedLastModified, rr2.Code)

	if rr2.Code == http.StatusNotModified {
		t.Error("BUG: Server returned 304 even though data changed!")
		t.Logf("Browser will show stale cached HTML with 3 unread instead of 2")
	}

	// === User clicks on a feed (no caching on /items OOB response) ===
	rr3 := env.request(http.MethodGet, "/items?url="+url.QueryEscape(feedServer.URL), "")
	body3 := rr3.Body.String()

	// This shows the OOB feedlist with correct count
	if strings.Contains(body3, ">3<") && !strings.Contains(body3, ">2<") {
		t.Logf("OOB response shows stale count too!")
	}
	if strings.Contains(body3, ">2<") {
		t.Logf("OOB response shows correct count of 2")
	}

	// The bug: if rr2 was 304, browser shows 3, but clicking shows 2
	if rr2.Code == http.StatusNotModified {
		t.Error("CONFIRMED: Refresh shows 3 (304 cached), click shows 2 (fresh) - this is the bug!")
	}
}

// TestIntegration_IdleTriggersUpdateButReturns304 tests the race condition where:
// 1. User is idle for 15+ minutes
// 2. User refreshes - recordActivity() triggers async feed updates
// 3. feedsNotModified() checks If-Modified-Since BEFORE updates complete
// 4. Server returns 304, browser shows stale cached HTML
// 5. Updates complete, lastModified advances (too late!)
// 6. User clicks feed - gets fresh HTML with correct counts
//
// This is the suspected cause of "refresh shows lower counts than clicking".
func TestIntegration_IdleTriggersUpdateButReturns304(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Create a feed server that tracks requests and can delay responses
	var requestCount int
	var mu sync.Mutex
	updateDelay := 100 * time.Millisecond

	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		count := requestCount
		mu.Unlock()

		// Simulate slow feed fetch
		time.Sleep(updateDelay)

		// First request: 2 items. Later: 5 items (simulating new content)
		var items []string
		if count <= 2 {
			items = []string{"Item 1", "Item 2"}
		} else {
			items = []string{"Item 1", "Item 2", "Item 3", "Item 4", "Item 5"}
		}
		fmt.Fprint(w, validRSSFeed("Idle Test", items))
	}))
	defer feedServer.Close()

	// Add feed and wait for initial update
	formData := url.Values{}
	formData.Set("url", feedServer.URL)
	formData.Set("name", "Idle Test Feed")
	env.request(http.MethodPost, "/crudfeed", formData.Encode())

	time.Sleep(300 * time.Millisecond) // Wait for initial update

	// === User loads page (not idle yet) ===
	rr1 := env.request(http.MethodGet, "/feeds", "")
	cachedLastModified := rr1.Header().Get("Last-Modified")
	t.Logf("Initial load: status=%d, Last-Modified=%s, body has 2 items: %v",
		rr1.Code, cachedLastModified, strings.Contains(rr1.Body.String(), ">2<"))

	// === Simulate user going idle for 15+ minutes ===
	env.svc.lastActivityMu.Lock()
	env.svc.lastActivity = time.Now().Add(-20 * time.Minute)
	env.svc.lastActivityMu.Unlock()

	t.Logf("Simulated 20 minutes of idle time")
	t.Logf("Server lastModified before refresh: %s", env.svc.getLastmodified().Format(http.TimeFormat))

	// === User refreshes (pull-to-refresh) ===
	// This should trigger recordActivity() which triggers async updates
	// But the If-Modified-Since check happens immediately
	req := httptest.NewRequest(http.MethodGet, "/feeds", nil)
	req.Header.Set("If-Modified-Since", cachedLastModified)

	rr2 := httptest.NewRecorder()
	env.mux.ServeHTTP(rr2, req)

	t.Logf("Refresh after idle: status=%d", rr2.Code)
	t.Logf("Server lastModified after refresh: %s", env.svc.getLastmodified().Format(http.TimeFormat))

	// Wait for async updates to complete
	time.Sleep(300 * time.Millisecond)

	t.Logf("Server lastModified after updates complete: %s", env.svc.getLastmodified().Format(http.TimeFormat))
	t.Logf("Actual unread count: %d", env.svc.feeds.All()[0].UnreadItemCount())

	// === User clicks on feed ===
	rr3 := env.request(http.MethodGet, "/items?url="+url.QueryEscape(feedServer.URL), "")
	body3 := rr3.Body.String()

	// Check what counts are shown
	refreshShows2 := strings.Contains(rr2.Body.String(), ">2<")
	refreshShows5 := strings.Contains(rr2.Body.String(), ">5<")
	clickShows2 := strings.Contains(body3, ">2<")
	clickShows5 := strings.Contains(body3, ">5<")

	t.Logf("Refresh response shows 2: %v, shows 5: %v", refreshShows2, refreshShows5)
	t.Logf("Click response shows 2: %v, shows 5: %v", clickShows2, clickShows5)

	// THE BUG: If refresh returned 304, browser shows cached "2"
	// but clicking shows fresh "5"
	if rr2.Code == http.StatusNotModified {
		t.Error("BUG REPRODUCED: Server returned 304 on refresh after idle")
		t.Error("Browser will show stale cached count while click shows fresh count")
	}

	// Even if we got 200, check if counts differ
	if rr2.Code == http.StatusOK && refreshShows2 && clickShows5 {
		t.Error("BUG: Refresh shows 2 but click shows 5 - updates completed between requests")
	}
}
