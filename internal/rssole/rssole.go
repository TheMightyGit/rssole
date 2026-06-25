package rssole

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"text/template"
	"time"

	"github.com/NYTimes/gziphandler"
	"golang.org/x/exp/slog"
)

const (
	templatesDir = "templates"
)

var Version = "dev"

var (
	//go:embed templates/*
	files embed.FS

	//go:embed libs/*
	wwwlibs embed.FS
)

func (s *Service) loadTemplates() error {
	if s.templates == nil {
		s.templates = make(map[string]*template.Template)
	}

	tmplFiles, err := fs.ReadDir(files, templatesDir)
	if err != nil {
		return fmt.Errorf("loadTemplates readdir - %w", err)
	}

	for _, tmpl := range tmplFiles {
		if tmpl.IsDir() {
			continue
		}

		pt, err := template.ParseFS(files, templatesDir+"/"+tmpl.Name(), templatesDir+"/components/*.go.html")
		if err != nil {
			return fmt.Errorf("loadTemplates parsefs - %w", err)
		}

		s.templates[tmpl.Name()] = pt
	}

	return nil
}

func Start(configFilename, configReadCacheFilename, listenAddress string, updateTime time.Duration) error {
	slog.Info("RSSOLE", "version", Version)

	svc := NewService()

	err := svc.loadTemplates()
	if err != nil {
		return err
	}

	svc.readLut.Filename = configReadCacheFilename
	svc.readLut.activity = svc // wire up the activity tracker
	svc.readLut.loadReadLut()
	svc.readLut.startCleanupTicker()

	if err := svc.feeds.readFeedsFile(configFilename); err != nil {
		return err
	}

	svc.feeds.UpdateTime = updateTime
	// Feed updates start on first client connection (see recordActivity)

	http.HandleFunc("GET /{$}", svc.index)
	http.HandleFunc("GET /feeds", svc.feedlist)
	http.HandleFunc("GET /items", svc.items)
	http.HandleFunc("POST /items", svc.items)
	http.HandleFunc("GET /item", svc.item)
	http.HandleFunc("GET /crudfeed", svc.crudfeedGet)
	http.HandleFunc("POST /crudfeed", svc.crudfeedPost)
	http.HandleFunc("GET /settings", svc.settingsGet)
	http.HandleFunc("POST /settings", svc.settingsPost)

	// As the static files won't change we force the browser to cache them.
	httpFS := http.FileServer(http.FS(wwwlibs))
	http.Handle("GET /libs/", forceCache(httpFS))

	slog.Info("Listening", "address", listenAddress)

	if err := http.ListenAndServe(listenAddress, gziphandler.GzipHandler(http.DefaultServeMux)); err != nil {
		return fmt.Errorf("error during ListenAndServe - %w", err)
	}

	return nil
}

func forceCache(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=86400") // 24 hours
		h.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
