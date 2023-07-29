package main

import (
	"encoding/json"
	"flag"
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
	Config rssole.ConfigSection `json:"config"`
}

func getFeedsFileConfigSection(filename string) rssole.ConfigSection {
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
	var configFilename, configReadCacheFilename string
	flag.StringVar(&configFilename, "c", "feeds.json", "config filename")
	flag.StringVar(&configReadCacheFilename, "r", "readcache.json", "readcache location")
	flag.Parse()

	cfg := getFeedsFileConfigSection(configFilename)

	if cfg.Listen == "" {
		cfg.Listen = defaultListenAddress
	}
	if cfg.UpdateSeconds == 0 {
		cfg.UpdateSeconds = defaultUpdateTimeSeconds
	}

	err := rssole.Start(configFilename, configReadCacheFilename, cfg.Listen, time.Duration(cfg.UpdateSeconds)*time.Second)
	if err != nil {
		log.Fatal(err)
	}
}
