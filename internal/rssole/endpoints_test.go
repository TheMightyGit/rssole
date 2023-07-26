package rssole

import (
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/mmcdole/gofeed"
)

var (
	testItem1 = &wrappedItem{
		IsUnread: false,
		Feed:     &feed{},
		Item: &gofeed.Item{
			Title:       "Story 1 Title",
			Description: "Story 1 Description",
			Link:        "http://example.com/story/1",
		},
	}
)

func init() {
	// We need the templates lodaed for endpoint tests.
	loadTemplates()

	// Set up some test feeds and items.
	allFeeds.Feeds = append(allFeeds.Feeds, &feed{
		URL:  "http://example.com/woo_feed",
		Name: "Woo Feed!",
	})
	allFeeds.Feeds = append(allFeeds.Feeds, &feed{
		URL:  "http://example.com/yay_feed",
		Name: "Yay Feed!",
		feed: &gofeed.Feed{},
		wrappedItems: []*wrappedItem{
			testItem1,
		},
	})
}

var (
	readCacheDir string
)

func setUpTearDown(t *testing.T) func(t *testing.T) {
	// We don't want to make a mess of the local fs
	// so clobber the readcache with one that uses a tmp file.
	var err error
	readCacheDir, err = os.MkdirTemp("", "Test_Endpoints")
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

func TestIndex(t *testing.T) {
	defer setUpTearDown(t)(t)

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(index)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response contains at least a <body openeing
	for _, expectedToFind := range []string{
		"<html",
		"<body",
	} {
		if !strings.Contains(rr.Body.String(), expectedToFind) {
			t.Errorf("handler returned page without expected content: got %v could not find '%v'",
				rr.Body.String(), expectedToFind)
		}
	}
}

func TestFeedlist(t *testing.T) {
	defer setUpTearDown(t)(t)

	req, err := http.NewRequest("GET", "/feeds", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(feedlist)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response contains the feeds we added
	for _, expectedToFind := range []string{
		"Woo Feed!",
		"Yay Feed!",
	} {
		if !strings.Contains(rr.Body.String(), expectedToFind) {
			t.Errorf("handler returned page without expected content: got %v could not find '%v'",
				rr.Body.String(), expectedToFind)
		}
	}
}

func TestItemsGet(t *testing.T) {
	defer setUpTearDown(t)(t)

	req, err := http.NewRequest("GET", "/items?url=http://example.com/yay_feed", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(items)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	for _, expectedToFind := range []string{
		"Yay Feed!",
		"Mark All Read",
		"Story 1 Title",
		"Story 1 Description",
	} {
		if !strings.Contains(rr.Body.String(), expectedToFind) {
			t.Errorf("handler returned page without expected content: got %v could not find '%v'",
				rr.Body.String(), expectedToFind)
		}
	}
}

func TestItemsPostMarkAsRead(t *testing.T) {
	defer setUpTearDown(t)(t)

	data := url.Values{}
	data.Add("read", "http://example.com/story/66")
	data.Add("read", "http://example.com/story/1")
	data.Add("read", "http://example.com/story/99")

	body := strings.NewReader(data.Encode())
	req, err := http.NewRequest("POST", "/items?url=http://example.com/yay_feed", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(items)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	for _, expectedToFind := range []string{
		"Yay Feed!",
		"Mark All Read",
		"Story 1 Title",
		"Story 1 Description",
	} {
		if !strings.Contains(rr.Body.String(), expectedToFind) {
			t.Errorf("handler returned page without expected content: got %v could not find '%v'",
				rr.Body.String(), expectedToFind)
		}
	}

	if readLut.isUnread("http://example.com/story/1") {
		t.Fatal("story should have been marked read")
	}
}

func TestItem(t *testing.T) {
	defer setUpTearDown(t)(t)

	storyId := testItem1.ID()
	req, err := http.NewRequest("GET", "/item?id="+storyId+"&url=http://example.com/yay_feed", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(item)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	for _, expectedToFind := range []string{
		"Story 1 Description",
		"http://example.com/story/1",
	} {
		if !strings.Contains(rr.Body.String(), expectedToFind) {
			t.Errorf("handler returned page without expected content: got\n%v\ncould not find '%v'",
				rr.Body.String(), expectedToFind)
		}
	}
}
