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

const MinUpdateSeconds = 900

func (s *Service) index(w http.ResponseWriter, req *http.Request) {
	logger := slog.Default().With("endpoint", req.URL, "method", req.Method)

	if err := s.templates["base.go.html"].Execute(w, map[string]any{
		"Version": Version,
	}); err != nil {
		logger.Error("base.go.html", "error", err)
	}
}

func (s *Service) feedlistCommon(w http.ResponseWriter, selected string, logger *slog.Logger) {
	w.Header().Add("Last-Modified", s.getLastmodified().Format(http.TimeFormat))

	feeds := s.feeds.list.All()
	for _, f := range feeds {
		f.mu.RLock()
	}

	defer func() {
		for _, f := range feeds {
			f.mu.RUnlock()
		}
	}()

	if err := s.templates["feedlist.go.html"].Execute(w, map[string]any{
		"Selected": selected,
		"Feeds":    s.feeds,
	}); err != nil {
		logger.Error("feedlist.go.html", "error", err)
	}
}

func (s *Service) feedsNotModified(req *http.Request) bool {
	// make precision equal for test
	lastmod, _ := http.ParseTime(s.getLastmodified().Format(http.TimeFormat))

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

func (s *Service) feedlist(w http.ResponseWriter, req *http.Request) {
	s.recordActivity()

	logger := slog.Default().With("endpoint", req.URL, "method", req.Method)

	// To greatly reduce the bandwidth from polling we use Last-Modified/If-Modified-Since
	// which is respected by htmx.
	if s.feedsNotModified(req) {
		w.WriteHeader(http.StatusNotModified)

		return
	}

	selected := req.URL.Query().Get("selected")
	s.feedlistCommon(w, selected, logger)
}

func (s *Service) items(w http.ResponseWriter, req *http.Request) {
	logger := slog.Default().With("endpoint", req.URL, "method", req.Method)

	feedURL := req.URL.Query().Get("url")

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

		if f := s.feeds.list.FindByURL(feedURL); f != nil && f.feed != nil {
			f.mu.Lock()
			for _, i := range f.Items() {
				if markRead[i.MarkReadID()] {
					logger.Info("marking read", "MarkReadID", i.MarkReadID())
					i.IsUnread = false
					s.readLut.MarkRead(i.MarkReadID())
				}
			}
			f.mu.Unlock()
		}

		s.readLut.Persist()
	}

	if f := s.feeds.list.FindByURL(feedURL); f != nil {
		f.mu.RLock()

		if err := s.templates["items.go.html"].Execute(w, f); err != nil {
			logger.Error("items.go.html", "error", err)
		}

		f.mu.RUnlock()

		// update feed list (oob)
		s.feedlistCommon(w, f.Title(), logger)
	}
}

func (s *Service) item(w http.ResponseWriter, req *http.Request) {
	feedURL := req.URL.Query().Get("url")
	id := req.URL.Query().Get("id")

	f := s.feeds.list.FindByURL(feedURL)
	if f == nil || f.feed == nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, item := range f.Items() {
		if item.ID() == id {
			item.IsUnread = false
			if err := s.templates["item.go.html"].Execute(w, item); err != nil {
				slog.Error("item.go.html", "error", err)
			}

			s.readLut.MarkRead(item.MarkReadID())
			s.readLut.Persist()

			break
		}
	}
}

func (s *Service) crudfeedGet(w http.ResponseWriter, req *http.Request) {
	logger := slog.Default().With("endpoint", req.URL, "method", req.Method)

	var f *feed

	feedID := req.URL.Query().Get("feed")
	if feedID != "" {
		f = s.feeds.getFeedByID(feedID)
	}

	if err := s.templates["crudfeed.go.html"].Execute(w, f); err != nil {
		logger.Error("crudfeed.go.html", "error", err)
	}
}

func (s *Service) crudfeedPost(w http.ResponseWriter, req *http.Request) {
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
			s.feeds.delFeed(id)
			fmt.Fprint(w, `Deleted.`)
			s.feedlistCommon(w, "_", logger)
		} else {
			// update
			f := s.feeds.getFeedByID(id)
			if f != nil {
				f.mu.Lock()
				f.URL = feedurl
				f.Name = name
				f.Category = category
				f.Scrape = scr
				f.mu.Unlock()
				s.feedlistCommon(w, f.Title(), logger)
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
		s.feeds.addFeed(feed, s.readLut, s)

		fmt.Fprintf(w, `<div hx-get="/items?url=%s" hx-trigger="load" hx-target="#items"></div>`, url.QueryEscape(feed.URL))
	}
	// something may have changed, so save it.
	if err := s.feeds.saveFeedsFile(); err != nil {
		logger.Error("saveFeedsFile", "error", err)
	}
}

func (s *Service) settingsGet(w http.ResponseWriter, req *http.Request) {
	logger := slog.Default().With("endpoint", req.URL, "method", req.Method)

	if err := s.templates["settings.go.html"].Execute(w, s.feeds.Config); err != nil {
		logger.Error("settings.go.html", "error", err)
	}
}

func (s *Service) settingsPost(w http.ResponseWriter, req *http.Request) {
	defer s.settingsGet(w, req)

	logger := slog.Default().With("endpoint", req.URL, "method", req.Method)

	err := req.ParseForm()
	if err != nil {
		logger.Error("ParseForm", "error", err)
	}

	updateSeconds, err := strconv.Atoi(req.FormValue("update_seconds"))
	if err != nil {
		logger.Error("Cannot parse update_seconds", "error", err)

		return
	}

	if updateSeconds < MinUpdateSeconds {
		logger.Error("Error, update_seconds is below 900")

		return
	}

	if updateSeconds != s.feeds.Config.UpdateSeconds {
		s.feeds.ChangeTickedUpdate(time.Duration(updateSeconds) * time.Second)
	}

	// something may have changed, so save it.
	if err := s.feeds.saveFeedsFile(); err != nil {
		logger.Error("saveFeedsFile", "error", err)
	}
}
