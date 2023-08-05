package rssole

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
)

func index(w http.ResponseWriter, _ *http.Request) {
	if err := templates["base.go.html"].Execute(w, nil); err != nil {
		log.Println(err)
	}
}

func feedlistCommon(w http.ResponseWriter, selected string) {
	allFeeds.mu.RLock()
	defer allFeeds.mu.RUnlock()

	w.Header().Add("Last-Modified", lastmodified.Format(http.TimeFormat))

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
		log.Println(err)
	}
}

func feedsNotModified(req *http.Request) bool {
	// make precision equal for test
	lastmod, _ := http.ParseTime(lastmodified.Format(http.TimeFormat))

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
	// To greatly reduce the bandwidth from polling we use Last-Modified/If-Modified-Since
	// which is respected by htmx.
	if feedsNotModified(req) {
		w.WriteHeader(http.StatusNotModified)

		return
	}

	selected := req.URL.Query().Get("selected")
	feedlistCommon(w, selected)
}

func items(w http.ResponseWriter, req *http.Request) {
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
						log.Println("marking read", i.MarkReadID())
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
				log.Println(err)
			}

			// update feed list (oob)
			feedlistCommon(w, f.Title())
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
						log.Println(err)
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
	var f *feed

	feedID := req.URL.Query().Get("feed")
	if feedID != "" {
		f = allFeeds.getFeedByID(feedID)
	}

	if err := templates["crudfeed.go.html"].Execute(w, f); err != nil {
		log.Println(err)
	}
}

func crudfeedPost(w http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		log.Println(err)
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
			feedlistCommon(w, "_")
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
				feedlistCommon(w, f.Title())
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
		allFeeds.addFeed(feed)

		fmt.Fprintf(w, `<div hx-get="/items?url=%s" hx-trigger="load" hx-target="#items"></div>`, url.QueryEscape(feed.URL))
	}
	// something may have changed, so save it.
	if err := allFeeds.saveFeedsFile(); err != nil {
		log.Println(err)
	}
}

func crudfeed(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {
		crudfeedGet(w, req)
	} else if req.Method == http.MethodPost {
		crudfeedPost(w, req)
	}
}
