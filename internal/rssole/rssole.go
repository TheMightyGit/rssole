package rssole

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
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
)

func readFeedsFile() {
	jsonFile, err := os.Open("feeds.json")
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer jsonFile.Close()

	d := json.NewDecoder(jsonFile)
	err = d.Decode(allFeeds)
	if err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}
}

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

func Start(listenAddress string, updateTimeSeconds time.Duration) {
	loadTemplates()
	loadReadLut()
	readFeedsFile()

	allFeeds.BeginFeedUpdates(updateTimeSeconds)

	http.HandleFunc("/", index)
	http.HandleFunc("/feeds", feedlist)
	http.HandleFunc("/items", items)
	http.HandleFunc("/item", item)
	http.HandleFunc("/addfeed", addfeed)

	fmt.Printf("Listening on %s\n", listenAddress)
	if err := http.ListenAndServe(listenAddress, nil); err != nil {
		log.Fatalln(err)
	}
}
