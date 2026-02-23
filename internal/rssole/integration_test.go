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
