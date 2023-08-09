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

func feedSetUpTearDown(_ *testing.T) func(t *testing.T) {
	// We don't want to make a mess of the local fs
	// so clobber the readcache with one that uses a tmp file.
	readCacheDir, err := os.MkdirTemp("", "Test_Feed")
	if err != nil {
		log.Fatal(err)
	}

	file, err := os.CreateTemp(readCacheDir, "*")
	if err != nil {
		log.Fatal(err)
	}

	// swap the global one out to a safe one
	readLut = &unreadLut{
		Filename: file.Name(),
	}

	return func(t *testing.T) {
		os.RemoveAll(readCacheDir)
	}
}

func TestUpdate_InvalidRssFeed(t *testing.T) {
	defer feedSetUpTearDown(t)(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Invalid RSS Feed")
	}))
	defer ts.Close()

	feed := &feed{
		URL: ts.URL,
	}
	feed.Init()

	err := feed.Update()

	if err == nil {
		t.Fatal("expected an error for an invalid feed")
	}
}

func TestUpdate_ValidRssFeed(t *testing.T) {
	defer feedSetUpTearDown(t)(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		URL: ts.URL,
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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

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
	defer feedSetUpTearDown(t)(t)

	updateCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	feed.StartTickedUpdate(10 * time.Millisecond)
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
