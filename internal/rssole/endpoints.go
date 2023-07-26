package rssole

import (
	"log"
	"net/http"
)

func index(w http.ResponseWriter, req *http.Request) {
	if err := templates["base.go.html"].Execute(w, nil); err != nil {
		log.Println(err)
	}
}

func feedlistCommon(w http.ResponseWriter, selected string) {
	allFeeds.mu.RLock()
	defer allFeeds.mu.RUnlock()

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

func feedlist(w http.ResponseWriter, req *http.Request) {
	feedlistCommon(w, "")
}

func items(w http.ResponseWriter, req *http.Request) {
	feedURL := req.URL.Query().Get("url")

	allFeeds.mu.RLock()
	defer allFeeds.mu.RUnlock()

	if req.Method == "POST" {
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
					if markRead[i.Link] {
						log.Println("marking read", i.Link)
						i.IsUnread = false
						readLut.markRead(i.Link)
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
					readLut.markRead(item.Link)
					readLut.persistReadLut()
					break
				}
			}
		}
		f.mu.RUnlock()
	}
	allFeeds.mu.RUnlock()
}

/*
func addfeed(w http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		if err := templates["addfeed.go.html"].Execute(w, nil); err != nil {
			log.Println(err)
		}
	} else if req.Method == "POST" {
		err := req.ParseForm()
		if err != nil {
			log.Println(err)
		}

		feedurl := req.FormValue("url")
		name := req.FormValue("name")
		category := req.FormValue("category")

		allFeeds.mu.Lock()
		defer allFeeds.mu.Unlock()

		feed := &feed{
			URL:      feedurl,
			Name:     name,
			Category: category,
		}
		feed.StartTickedUpdate()
		allFeeds.Feeds = append(allFeeds.Feeds, feed)

		fmt.Fprintf(w, `<div id="items" hx-get="/items?url=%s" hx-trigger="load" hx-target="#items"></div>`, url.QueryEscape(feed.URL))
	}
}
*/
