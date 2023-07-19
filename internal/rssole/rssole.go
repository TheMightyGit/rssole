package rssole

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"text/template"
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

	content, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	err = json.Unmarshal(content, allFeeds)
	if err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}
}

func loadTemplates() error {
	if templates == nil {
		templates = make(map[string]*template.Template)
	}
	tmplFiles, err := fs.ReadDir(files, templatesDir)
	if err != nil {
		return err
	}

	for _, tmpl := range tmplFiles {
		if tmpl.IsDir() {
			continue
		}

		pt, err := template.ParseFS(files, templatesDir+"/"+tmpl.Name())
		if err != nil {
			return err
		}

		templates[tmpl.Name()] = pt
	}
	return nil
}

func Start() {
	loadTemplates()
	loadReadLut()
	readFeedsFile()

	allFeeds.BeginFeedUpdates()

	http.HandleFunc("/", index)
	http.HandleFunc("/feeds", feedlist)
	http.HandleFunc("/items", items)
	http.HandleFunc("/item", item)
	http.HandleFunc("/addfeed", addfeed)

	hostAndPort := "0.0.0.0:8090"
	fmt.Printf("Listening on %s\n", hostAndPort)
	http.ListenAndServe(hostAndPort, nil)
}
