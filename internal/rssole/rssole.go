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
	files     embed.FS
	templates map[string]*template.Template

	//go:embed libs/*
	wwwlibs embed.FS
)

var (
	allFeeds = &feeds{}
	readLut  = &unreadLut{}
)

func loadTemplates() error {
	if templates == nil {
		templates = make(map[string]*template.Template)
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

		templates[tmpl.Name()] = pt
	}

	return nil
}

func Start(configFilename, configReadCacheFilename, listenAddress string, updateTime time.Duration) error {
	slog.Info("RSSOLE", "version", Version)

	err := loadTemplates()
	if err != nil {
		return err
	}

	readLut.Filename = configReadCacheFilename
	readLut.loadReadLut()
	readLut.startCleanupTicker()

	if err := allFeeds.readFeedsFile(configFilename); err != nil {
		return err
	}

	allFeeds.UpdateTime = updateTime
	allFeeds.BeginFeedUpdates()

	http.HandleFunc("GET /{$}", index)
	http.HandleFunc("GET /feeds", feedlist)
	http.HandleFunc("GET /items", items)
	http.HandleFunc("POST /items", items)
	http.HandleFunc("GET /item", item)
	http.HandleFunc("GET /crudfeed", crudfeedGet)
	http.HandleFunc("POST /crudfeed", crudfeedPost)
	http.HandleFunc("GET /settings", settingsGet)
	http.HandleFunc("POST /settings", settingsPost)

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
