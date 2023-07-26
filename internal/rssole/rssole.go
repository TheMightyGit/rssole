package rssole

import (
	"embed"
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
	readLut  = &unreadLut{
		Filename: "readcache.json",
	}
)

func loadTemplates() {
	if templates == nil {
		templates = make(map[string]*template.Template)
	}
	tmplFiles, err := fs.ReadDir(files, templatesDir)
	if err != nil {
		log.Fatalln(err)
	}

	for _, tmpl := range tmplFiles {
		if tmpl.IsDir() {
			continue
		}

		pt, err := template.ParseFS(files, templatesDir+"/"+tmpl.Name(), templatesDir+"/components/*.go.html")
		if err != nil {
			log.Fatalln(err)
		}

		templates[tmpl.Name()] = pt
	}
}

func Start(configFilename, listenAddress string, updateTimeSeconds time.Duration) {
	loadTemplates()

	readLut.loadReadLut()
	readLut.startCleanupTicker()

	if err := allFeeds.readFeedsFile(configFilename); err != nil {
		log.Fatalln(err)
	}
	allFeeds.BeginFeedUpdates(updateTimeSeconds)

	http.HandleFunc("/", index)
	http.HandleFunc("/feeds", feedlist)
	http.HandleFunc("/items", items)
	http.HandleFunc("/item", item)
	// http.HandleFunc("/addfeed", addfeed)

	log.Printf("Listening on %s\n", listenAddress)
	if err := http.ListenAndServe(listenAddress, nil); err != nil {
		log.Fatalln(err)
	}
}
