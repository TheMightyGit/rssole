package rssole

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

/* TODO:

Scrape during Update - test it works

*/

func feedSetUpTearDown(t *testing.T) func(t *testing.T) {
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

	err := feed.Update()

	if err != nil {
		t.Fatal("unexpected error for a valid", err)
	}

	if feed.feed == nil {
		t.Fatal("expected feed not to be nil")
	}
}
