package rssole

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

/* TODO:

Scrape during Update - test it works
Unread count
Sorting

*/

// feedTestReadCache is a mock ReadCache for feed tests.
type feedTestReadCache struct {
	filename string
}

func (m *feedTestReadCache) IsUnread(_ string) bool     { return true }
func (m *feedTestReadCache) MarkRead(_ string)          {}
func (m *feedTestReadCache) ExtendLifeIfFound(_ string) {}
func (m *feedTestReadCache) Persist()                   {}

// feedTestActivityTracker is a mock ActivityTracker for feed tests.
type feedTestActivityTracker struct{}

func (m *feedTestActivityTracker) IsIdle() bool        { return false }
func (m *feedTestActivityTracker) UpdateLastModified() {}

func feedSetUpTearDown(_ *testing.T) (*feedTestReadCache, *feedTestActivityTracker, func(t *testing.T)) {
	// We don't want to make a mess of the local fs
	// so create a temp directory for the test.
	readCacheDir, err := os.MkdirTemp("", "Test_Feed")
	if err != nil {
		log.Fatal(err)
	}

	file, err := os.CreateTemp(readCacheDir, "*")
	if err != nil {
		log.Fatal(err)
	}

	mockRC := &feedTestReadCache{filename: file.Name()}
	mockAT := &feedTestActivityTracker{}

	return mockRC, mockAT, func(_ *testing.T) {
		os.RemoveAll(readCacheDir)
	}
}

func TestUpdate_InvalidRssFeed(t *testing.T) {
	mockRC, mockAT, teardown := feedSetUpTearDown(t)
	defer teardown(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "Invalid RSS Feed")
	}))
	defer ts.Close()

	feed := &feed{
		URL:       ts.URL,
		readCache: mockRC,
		activity:  mockAT,
	}
	feed.Init()

	err := feed.Update()
	if err == nil {
		t.Fatal("expected an error for an invalid feed")
	}
}

func TestUpdate_ValidRssFeed(t *testing.T) {
	mockRC, mockAT, teardown := feedSetUpTearDown(t)
	defer teardown(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
  <title>Feed Title</title>
  <link>Feed Link</link>
  <description>This is a test</description>
  <item>
    <title>Title 1</title>
    <link>http://title1.com/</link>
    <description>Title 1</description>
  </item>
  <item>
    <title>Title 2</title>
    <link>http://title2.com/</link>
    <description>Title 2</description>
  </item>
  <item>
    <title>Title 3</title>
    <link>http://title3.com/</link>
    <description>Title 3</description>
  </item>
</channel>
</rss>`)
	}))
	defer ts.Close()

	feed := &feed{
		URL:       ts.URL,
		readCache: mockRC,
		activity:  mockAT,
	}
	feed.Init()

	err := feed.Update()
	if err != nil {
		t.Fatal("unexpected error for a valid", err)
	}

	if feed.feed == nil {
		t.Fatal("expected feed not to be nil")
	}
}

func TestUpdate_ValidScrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, `<html>
<body>
	<div class="item">
		<p class="title">Title 1</p>
		<a class="link" href="http://title1.com/">Title 1</a>
	</div>
	<div class="item">
		<p class="title">Title 2</p>
		<a class="link" href="http://title2.com/">Title 2</a>
	</div>
</body>
</html>`)
	}))
	defer ts.Close()

	mockRC := &feedTestReadCache{}
	mockAT := &feedTestActivityTracker{}

	feed := &feed{
		URL: ts.URL,
		Scrape: &scrape{
			URLs: []string{
				ts.URL,
				ts.URL,
			},
			Item:  ".item",
			Title: ".title",
			Link:  ".link",
		},
		readCache: mockRC,
		activity:  mockAT,
	}
	feed.Init()

	err := feed.Update()
	if err != nil {
		t.Fatal("unexpected error for a valid", err)
	}

	if feed.feed == nil {
		t.Fatal("expected feed not to be nil")
	}
}

func TestUpdate_InvalidScrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	mockRC := &feedTestReadCache{}
	mockAT := &feedTestActivityTracker{}

	feed := &feed{
		URL: ts.URL,
		Scrape: &scrape{
			URLs: []string{
				ts.URL,
				ts.URL,
			},
			Item:  ".item",
			Title: ".title",
			Link:  ".link",
		},
		readCache: mockRC,
		activity:  mockAT,
	}
	feed.Init()

	err := feed.Update()
	if err == nil {
		t.Fatal("expected error for an invalid", err)
	}

	if feed.feed != nil {
		t.Fatal("expected feed to be nil")
	}
}

func TestStartTickedUpdate(t *testing.T) {
	mockRC, mockAT, teardown := feedSetUpTearDown(t)
	defer teardown(t)

	updateCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		updateCount++

		fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
  <title>Feed Title</title>
  <link>Feed Link</link>
  <description>This is a test</description>
  <item>
    <title>Title 1</title>
    <link>http://title1.com/</link>
    <description>Title 1</description>
  </item>
</channel>
</rss>`)
	}))
	defer ts.Close()

	feed := &feed{
		URL: ts.URL,
	}
	feed.Init()

	feed.StartTickedUpdate(10*time.Millisecond, mockRC, mockAT)
	time.Sleep(45 * time.Millisecond)
	feed.StopTickedUpdate()

	if updateCount == 1 {
		t.Fatal("expected more than 1 updates to have happened, got", updateCount)
	}

	if feed.Title() != "Feed Title" {
		t.Fatal("unexpected feed title of:", feed.Title())
	}
}

func TestLog(t *testing.T) {
	feed := &feed{}
	feed.Init()

	feed.log.Info("line 1")

	if !strings.Contains(feed.RecentLogs.String(), "line 1") {
		t.Fatal("expected to find line 1 in:", feed.RecentLogs.String())
	}
}

func TestLog_ExceedMaxLines(t *testing.T) {
	feed := &feed{}
	feed.Init()

	// overflow the max by 1
	for i := 0; i <= maxRecentLogLines+1; i++ {
		feed.log.Info(fmt.Sprintf("line %d here", i))
	}

	if strings.Contains(feed.RecentLogs.String(), "line 1 here") {
		t.Fatal("expected not to find line 1 in:", feed.RecentLogs.String())
	}

	if !strings.Contains(feed.RecentLogs.String(), "line 2 here") {
		t.Fatal("expected to find line 2 in:", feed.RecentLogs.String())
	}
}
