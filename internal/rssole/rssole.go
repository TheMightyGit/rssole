package rssole

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"text/template"
	"time"
)

const (
	templatesDir = "templates"
)

var (
	//go:embed templates/*
	files     embed.FS
	templates map[string]*template.Template
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

func Start(configFilename, configReadCacheFilename, listenAddress string, updateTimeSeconds time.Duration) error {
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
	allFeeds.UpdateTime = updateTimeSeconds
	allFeeds.BeginFeedUpdates()

	http.HandleFunc("/", index)
	http.HandleFunc("/feeds", feedlist)
	http.HandleFunc("/items", items)
	http.HandleFunc("/item", item)
	http.HandleFunc("/crudfeed", crudfeed)

	log.Printf("Listening on %s\n", listenAddress)
	if err := http.ListenAndServe(listenAddress, nil); err != nil {
		return err
	}
	return nil
}
