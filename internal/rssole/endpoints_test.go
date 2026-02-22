package rssole

import (
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

// testService holds the Service instance used across tests.
var testService *Service

var testItem1 = &wrappedItem{
	IsUnread: true,
	Feed:     &feed{},
	Item: &gofeed.Item{
		Title:       "Story 1 Title",
		Description: "Story 1 Description",
		Link:        "http://example.com/story/1",
	},
}

func init() {
	// Create a test service instance
	testService = NewService()

	// Load templates onto the service
	_ = testService.loadTemplates()

	testItem1.Feed.Init()

	// Set up some test feeds and items.
	feed1 := &feed{
		URL:  "http://example.com/woo_feed",
		Name: "Woo Feed!",
	}
	feed1.Init()

	feed2 := &feed{
		URL:  "http://example.com/yay_feed",
		Name: "Yay Feed!",
		feed: &gofeed.Feed{},
		wrappedItems: []*wrappedItem{
			testItem1,
		},
	}
	feed2.Init()

	testService.feeds.list.Add(feed1)
	testService.feeds.list.Add(feed2)

	// zero will cause errors if UpdateTime is not set positive
	testService.feeds.UpdateTime = 10

	testService.feeds.Config.Listen = "1.2.3.4:5678"
	testService.feeds.Config.UpdateSeconds = 987
}

var readCacheDir string

func setUpTearDown(_ *testing.T) func(t *testing.T) {
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

	// swap the service's readLut with one that uses a tmp file
	testService.readLut = &unreadLut{
		Filename: file.Name(),
	}

	return func(_ *testing.T) {
		os.RemoveAll(readCacheDir)
	}
}

func TestIndex(t *testing.T) {
	defer setUpTearDown(t)(t)

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(testService.index)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

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

	req, err := http.NewRequest(http.MethodGet, "/feeds", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(testService.feedlist)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

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

func TestFeedlist_NotModified(t *testing.T) {
	defer setUpTearDown(t)(t)

	req, err := http.NewRequest(http.MethodGet, "/feeds", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("If-Modified-Since", time.Now().Format(http.TimeFormat))

	yesterday := time.Now().Add(-time.Hour * 24)

	testService.muLastmodified.Lock()
	testService.lastmodified = yesterday
	testService.muLastmodified.Unlock()

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(testService.feedlist)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotModified {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNotModified)
	}
}

func TestFeedlist_Modified(t *testing.T) {
	defer setUpTearDown(t)(t)

	req, err := http.NewRequest(http.MethodGet, "/feeds", nil)
	if err != nil {
		t.Fatal(err)
	}

	yesterday := time.Now().Add(-time.Hour * 24)
	req.Header.Add("If-Modified-Since", yesterday.Format(http.TimeFormat))

	testService.muLastmodified.Lock()
	testService.lastmodified = time.Now()
	testService.muLastmodified.Unlock()

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(testService.feedlist)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestItemsGet(t *testing.T) {
	defer setUpTearDown(t)(t)

	req, err := http.NewRequest(http.MethodGet, "/items?url=http://example.com/yay_feed", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(testService.items)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	for _, expectedToFind := range []string{
		"Yay Feed!",
		"Mark All Read",
		"Story 1 Title",
		"Story 1 Description",
		"http://example.com/story/1",
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

	req, err := http.NewRequest(http.MethodPost, "/items?url=http://example.com/yay_feed", body)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(testService.items)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	for _, expectedToFind := range []string{
		"Yay Feed!",
		"Mark All Read",
		"Story 1 Title",
		// "Story 1 Description",
		"http://example.com/story/1",
	} {
		if !strings.Contains(rr.Body.String(), expectedToFind) {
			t.Errorf("handler returned page without expected content: got %v could not find '%v'",
				rr.Body.String(), expectedToFind)
		}
	}

	if testService.readLut.IsUnread("http://example.com/story/1") {
		t.Fatal("story should have been marked read")
	}
}

func TestItem(t *testing.T) {
	defer setUpTearDown(t)(t)

	storyID := testItem1.ID()

	req, err := http.NewRequest(http.MethodGet, "/item?id="+storyID+"&url=http://example.com/yay_feed", nil)
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(testService.item)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	for _, expectedToFind := range []string{
		"Story 1 Description",
	} {
		if !strings.Contains(rr.Body.String(), expectedToFind) {
			t.Errorf("handler returned page without expected content: got\n%v\ncould not find '%v'",
				rr.Body.String(), expectedToFind)
		}
	}
}

func TestCrudFeed_Get(t *testing.T) {
	defer setUpTearDown(t)(t)

	req, err := http.NewRequest(http.MethodGet, "/crudfeed?feed=12345", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(testService.crudfeedGet)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestCrudFeed_Post_AddRssFeed(t *testing.T) {
	defer setUpTearDown(t)(t)

	currentNumFeeds := len(testService.feeds.All())

	data := url.Values{}
	data.Add("url", "http://example.com/added_feed_url")
	data.Add("name", "Feed Nickname")
	data.Add("category", "Super Category")

	body := strings.NewReader(data.Encode())

	req, err := http.NewRequest(http.MethodPost, "/crudfeed", body)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(testService.crudfeedPost)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// did a feed get added?
	if len(testService.feeds.All()) != currentNumFeeds+1 {
		t.Errorf("expected number of feeds to be higher now, but got: %d", len(testService.feeds.All()))
	}

	newFeed := testService.feeds.All()[currentNumFeeds]
	if newFeed.URL != "http://example.com/added_feed_url" {
		t.Error("expected new feed url to match, got:", newFeed.URL)
	}

	if newFeed.Name != "Feed Nickname" {
		t.Error("expected new feed name to match, got:", newFeed.Name)
	}

	if newFeed.Category != "Super Category" {
		t.Error("expected new feed category to match, got:", newFeed.Category)
	}
}

func TestCrudFeed_Post_AddRssFeed_WithScrape(t *testing.T) {
	defer setUpTearDown(t)(t)

	// do we start with the expected number of feeds?
	currentNumFeeds := len(testService.feeds.All())

	data := url.Values{}
	data.Add("url", "http://example.com/added_feed_url")
	data.Add("name", "Feed Nickname")
	data.Add("category", "Super Category")

	data.Add("scrape.urls", "http://example.com/1\nhttp://example.com/2")
	data.Add("scrape.item", "Scrape Item CSS Selector")
	data.Add("scrape.title", "Scrape Title CSS Selector")
	data.Add("scrape.link", "Scrape Link CSS Selector")

	body := strings.NewReader(data.Encode())

	req, err := http.NewRequest(http.MethodPost, "/crudfeed", body)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(testService.crudfeedPost)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// did a feed get added?
	if len(testService.feeds.All()) != currentNumFeeds+1 {
		t.Errorf("expected number of feeds to be higher now, but got: %d", len(testService.feeds.All()))
	}

	newFeed := testService.feeds.All()[currentNumFeeds]
	if newFeed.URL != "http://example.com/added_feed_url" {
		t.Error("expected new feed url to match, got:", newFeed.URL)
	}

	if newFeed.Name != "Feed Nickname" {
		t.Error("expected new feed name to match, got:", newFeed.Name)
	}

	if newFeed.Category != "Super Category" {
		t.Error("expected new feed category to match, got:", newFeed.Category)
	}

	if newFeed.Scrape == nil {
		t.Fatal("expected new feed scrape not to be nil")
	}

	if newFeed.Scrape.URLs[0] != "http://example.com/1" ||
		newFeed.Scrape.URLs[1] != "http://example.com/2" {
		t.Error("expected new feed scrape urls to match, got:", newFeed.Scrape.URLs)
	}

	if newFeed.Scrape.Item != "Scrape Item CSS Selector" {
		t.Error("expected new feed scrape item to match, got:", newFeed.Scrape.Item)
	}

	if newFeed.Scrape.Title != "Scrape Title CSS Selector" {
		t.Error("expected new feed scrape title to match, got:", newFeed.Scrape.Title)
	}

	if newFeed.Scrape.Link != "Scrape Link CSS Selector" {
		t.Error("expected new feed scrape link to match, got:", newFeed.Scrape.Link)
	}
}

func TestCrudFeed_Post_DeleteRssFeed(t *testing.T) {
	defer setUpTearDown(t)(t)

	currentNumFeeds := len(testService.feeds.All())

	data := url.Values{}
	data.Add("id", testService.feeds.All()[0].ID())
	data.Add("delete", "delete")

	body := strings.NewReader(data.Encode())

	req, err := http.NewRequest(http.MethodPost, "/crudfeed", body)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(testService.crudfeedPost)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// did a feed get removed?
	if len(testService.feeds.All()) != currentNumFeeds-1 {
		t.Errorf("expected number of feeds to be lower now, but got: %d", len(testService.feeds.All()))
	}
}

func TestCrudFeed_Post_UpdateRssFeed_WithScrape(t *testing.T) {
	defer setUpTearDown(t)(t)

	data := url.Values{}
	data.Add("id", testService.feeds.All()[0].ID()) // replace whatever's there
	data.Add("url", "http://example.com/added_feed_url")
	data.Add("name", "Feed Nickname")
	data.Add("category", "Super Category")

	data.Add("scrape.urls", "http://example.com/1\nhttp://example.com/2")
	data.Add("scrape.item", "Scrape Item CSS Selector")
	data.Add("scrape.title", "Scrape Title CSS Selector")
	data.Add("scrape.link", "Scrape Link CSS Selector")

	body := strings.NewReader(data.Encode())

	req, err := http.NewRequest(http.MethodPost, "/crudfeed", body)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(testService.crudfeedPost)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	updatedFeed := testService.feeds.All()[0]
	if updatedFeed.URL != "http://example.com/added_feed_url" {
		t.Error("expected new feed url to match, got:", updatedFeed.URL)
	}

	if updatedFeed.Name != "Feed Nickname" {
		t.Error("expected new feed name to match, got:", updatedFeed.Name)
	}

	if updatedFeed.Category != "Super Category" {
		t.Error("expected new feed category to match, got:", updatedFeed.Category)
	}

	if updatedFeed.Scrape == nil {
		t.Fatal("expected new feed scrape not to be nil")
	}

	if updatedFeed.Scrape.URLs[0] != "http://example.com/1" ||
		updatedFeed.Scrape.URLs[1] != "http://example.com/2" {
		t.Error("expected new feed scrape urls to match, got:", updatedFeed.Scrape.URLs)
	}

	if updatedFeed.Scrape.Item != "Scrape Item CSS Selector" {
		t.Error("expected new feed scrape item to match, got:", updatedFeed.Scrape.Item)
	}

	if updatedFeed.Scrape.Title != "Scrape Title CSS Selector" {
		t.Error("expected new feed scrape title to match, got:", updatedFeed.Scrape.Title)
	}

	if updatedFeed.Scrape.Link != "Scrape Link CSS Selector" {
		t.Error("expected new feed scrape link to match, got:", updatedFeed.Scrape.Link)
	}
}

func TestSettings_Get(t *testing.T) {
	defer setUpTearDown(t)(t)

	req, err := http.NewRequest(http.MethodGet, "/settings", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(testService.settingsGet)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	for _, expectedToFind := range []string{
		"1.2.3.4:5678",
		"987",
	} {
		if !strings.Contains(rr.Body.String(), expectedToFind) {
			t.Errorf("handler returned page without expected content: got %v could not find '%v'",
				rr.Body.String(), expectedToFind)
		}
	}
}

func TestSettings_Post(t *testing.T) {
	defer setUpTearDown(t)(t)

	data := url.Values{}
	data.Add("update_seconds", "999")

	body := strings.NewReader(data.Encode())

	req, err := http.NewRequest(http.MethodPost, "/settings", body)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(testService.settingsPost)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	for _, expectedToFind := range []string{
		"1.2.3.4:5678",
		"999", // updated
	} {
		if !strings.Contains(rr.Body.String(), expectedToFind) {
			t.Errorf("handler returned page without expected content: got %v could not find '%v'",
				rr.Body.String(), expectedToFind)
		}
	}
}
