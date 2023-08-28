package rssole

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/slog"
)

func index(w http.ResponseWriter, req *http.Request) {
	logger := slog.Default().With("endpoint", req.URL, "method", req.Method)

	if err := templates["base.go.html"].Execute(w, map[string]any{
		"Version": Version,
	}); err != nil {
		logger.Error("base.go.html", "error", err)
	}
}

func feedlistCommon(w http.ResponseWriter, selected string, logger *slog.Logger) {
	allFeeds.mu.RLock()
	defer allFeeds.mu.RUnlock()

	w.Header().Add("Last-Modified", getLastmodified().Format(http.TimeFormat))

	for _, f := range allFeeds.Feeds {
		f.mu.RLock()
	}

	defer func() {
		for _, f := range allFeeds.Feeds {
			f.mu.RUnlock()
		}
	}()

	allFeeds.Selected = selected

	if err := templates["feedlist.go.html"].Execute(w, allFeeds); err != nil {
		logger.Error("feedlist.go.html", "error", err)
	}
}

func feedsNotModified(req *http.Request) bool {
	// make precision equal for test
	lastmod, _ := http.ParseTime(getLastmodified().Format(http.TimeFormat))

	imsRaw := req.Header.Get("if-modified-since")
	if imsRaw != "" {
		// has any feed (or mark as read) been modified since last time?
		ims, err := http.ParseTime(req.Header.Get("if-modified-since"))
		if err == nil {
			if ims.After(lastmod) ||
				ims.Equal(lastmod) {
				return true
			}
		}
	}

	return false
}

func feedlist(w http.ResponseWriter, req *http.Request) {
	logger := slog.Default().With("endpoint", req.URL, "method", req.Method)

	// To greatly reduce the bandwidth from polling we use Last-Modified/If-Modified-Since
	// which is respected by htmx.
	if feedsNotModified(req) {
		w.WriteHeader(http.StatusNotModified)

		return
	}

	selected := req.URL.Query().Get("selected")
	feedlistCommon(w, selected, logger)
}

func items(w http.ResponseWriter, req *http.Request) {
	logger := slog.Default().With("endpoint", req.URL, "method", req.Method)

	feedURL := req.URL.Query().Get("url")

	allFeeds.mu.RLock()
	defer allFeeds.mu.RUnlock()

	if req.Method == http.MethodPost {
		_ = req.ParseForm()
		markRead := map[string]bool{}

		for k, v := range req.Form {
			if k == "read" {
				for _, v2 := range v {
					markRead[v2] = true
				}
			}
		}

		for _, f := range allFeeds.Feeds {
			if f.feed != nil && f.URL == feedURL {
				f.mu.Lock()
				for _, i := range f.Items() {
					if markRead[i.MarkReadID()] {
						logger.Info("marking read", "MarkReadID", i.MarkReadID())
						i.IsUnread = false
						readLut.markRead(i.MarkReadID())
					}
				}
				f.mu.Unlock()
			}
		}

		readLut.persistReadLut()
	}

	for _, f := range allFeeds.Feeds {
		f.mu.RLock()
		if f.URL == feedURL {
			if err := templates["items.go.html"].Execute(w, f); err != nil {
				logger.Error("items.go.html", "error", err)
			}

			// update feed list (oob)
			feedlistCommon(w, f.Title(), logger)
		}
		f.mu.RUnlock()
	}
}

func item(w http.ResponseWriter, req *http.Request) {
	feedURL := req.URL.Query().Get("url")
	id := req.URL.Query().Get("id")

	allFeeds.mu.RLock()
	for _, f := range allFeeds.Feeds {
		f.mu.RLock()
		if f.feed != nil && f.URL == feedURL {
			for _, item := range f.Items() {
				if item.ID() == id {
					item.IsUnread = false
					if err := templates["item.go.html"].Execute(w, item); err != nil {
						slog.Error("item.go.html", "error", err)
					}

					readLut.markRead(item.MarkReadID())
					readLut.persistReadLut()

					break
				}
			}
		}
		f.mu.RUnlock()
	}
	allFeeds.mu.RUnlock()
}

func crudfeedGet(w http.ResponseWriter, req *http.Request) {
	logger := slog.Default().With("endpoint", req.URL, "method", req.Method)

	var f *feed

	feedID := req.URL.Query().Get("feed")
	if feedID != "" {
		f = allFeeds.getFeedByID(feedID)
	}

	if err := templates["crudfeed.go.html"].Execute(w, f); err != nil {
		logger.Error("crudfeed.go.html", "error", err)
	}
}

func crudfeedPost(w http.ResponseWriter, req *http.Request) {
	logger := slog.Default().With("endpoint", req.URL, "method", req.Method)

	err := req.ParseForm()
	if err != nil {
		logger.Error("ParseForm", "error", err)
	}

	id := req.FormValue("id")
	feedurl := req.FormValue("url")
	name := req.FormValue("name")
	category := req.FormValue("category")

	scrapeURLs := req.FormValue("scrape.urls")
	scrapeItem := req.FormValue("scrape.item")
	scrapeTitle := req.FormValue("scrape.title")
	scrapeLink := req.FormValue("scrape.link")

	var scr *scrape
	if scrapeURLs != "" || scrapeItem != "" || scrapeTitle != "" || scrapeLink != "" {
		scr = &scrape{
			URLs:  strings.Split(strings.TrimSpace(scrapeURLs), "\n"),
			Item:  scrapeItem,
			Title: scrapeTitle,
			Link:  scrapeLink,
		}
	}

	if id != "" { // edit or delete
		del := req.FormValue("delete")
		if del != "" {
			allFeeds.delFeed(id)
			fmt.Fprint(w, `Deleted.`)
			feedlistCommon(w, "_", logger)
		} else {
			// update
			f := allFeeds.getFeedByID(id)
			if f != nil {
				f.mu.Lock()
				f.URL = feedurl
				f.Name = name
				f.Category = category
				f.Scrape = scr
				f.mu.Unlock()
				feedlistCommon(w, f.Title(), logger)
				fmt.Fprintf(w, `<div hx-get="/items?url=%s" hx-trigger="load" hx-target="#items"></div>`, url.QueryEscape(f.URL))
			} else {
				fmt.Fprint(w, `Not found.`)
			}
		}
	} else { // add
		feed := &feed{
			URL:      feedurl,
			Name:     name,
			Category: category,
			Scrape:   scr,
		}
		feed.Init()
		allFeeds.addFeed(feed)

		fmt.Fprintf(w, `<div hx-get="/items?url=%s" hx-trigger="load" hx-target="#items"></div>`, url.QueryEscape(feed.URL))
	}
	// something may have changed, so save it.
	if err := allFeeds.saveFeedsFile(); err != nil {
		logger.Error("saveFeedsFile", "error", err)
	}
}

func crudfeed(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {
		crudfeedGet(w, req)
	} else if req.Method == http.MethodPost {
		crudfeedPost(w, req)
	}
}

func settingsGet(w http.ResponseWriter, req *http.Request) {
	logger := slog.Default().With("endpoint", req.URL, "method", req.Method)

	if err := templates["settings.go.html"].Execute(w, allFeeds.Config); err != nil {
		logger.Error("settings.go.html", "error", err)
	}
}

func settingsPost(w http.ResponseWriter, req *http.Request) {
	logger := slog.Default().With("endpoint", req.URL, "method", req.Method)

	err := req.ParseForm()
	if err != nil {
		logger.Error("ParseForm", "error", err)
	}

	update_seconds, err := strconv.Atoi(req.FormValue("update_seconds"))
	if err != nil {
		logger.Error("Cannot parse update_seconds", "error", err)
		return
	}

	if update_seconds < 900 {
		logger.Error("Error, update_seconds is below 900")
		return
	}

	if update_seconds != allFeeds.Config.UpdateSeconds {
		allFeeds.ChangeTickedUpdate(time.Duration(update_seconds) * time.Second)
	}

	// something may have changed, so save it.
	if err := allFeeds.saveFeedsFile(); err != nil {
		logger.Error("saveFeedsFile", "error", err)
	}
}

func settings(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {
		settingsGet(w, req)
	} else if req.Method == http.MethodPost {
		settingsPost(w, req)
	}
}
