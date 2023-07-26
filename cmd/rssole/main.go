package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/TheMightyGit/rssole/internal/rssole"
)

const (
	defaultListenAddress     = "0.0.0.0:8090"
	defaultUpdateTimeSeconds = 300
)

type configFile struct {
	Config configSection `json:"config"`
}

type configSection struct {
	Listen        string `json:"listen"`
	UpdateSeconds int    `json:"update_seconds"`
}

func getFeedsFileConfigSection(filename string) configSection {
	jsonFile, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer jsonFile.Close()

	var c configFile
	d := json.NewDecoder(jsonFile)
	err = d.Decode(&c)
	if err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}
	return c.Config
}

func main() {
	cfg := getFeedsFileConfigSection("feeds.json")

	if cfg.Listen == "" {
		cfg.Listen = defaultListenAddress
	}
	if cfg.UpdateSeconds == 0 {
		cfg.UpdateSeconds = defaultUpdateTimeSeconds
	}

	rssole.Start(cfg.Listen, time.Duration(cfg.UpdateSeconds)*time.Second)
}
