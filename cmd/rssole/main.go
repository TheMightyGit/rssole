package main

import (
	"encoding/json"
	"flag"
	"fmt"
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

func getFeedsFileConfigSection(filename string) (rssole.ConfigSection, error) {
	var cfgFile configFile

	jsonFile, err := os.Open(filename)
	if err != nil {
		return cfgFile.Config, fmt.Errorf("error opening file: %v", err)
	}
	defer jsonFile.Close()

	decoder := json.NewDecoder(jsonFile)

	err = decoder.Decode(&cfgFile)
	if err != nil {
		return cfgFile.Config, fmt.Errorf("error unmarshalling JSON: %v", err)
	}

	return cfgFile.Config, nil
}

func main() {
	var configFilename, configReadCacheFilename string

	flag.StringVar(&configFilename, "c", "feeds.json", "config filename")
	flag.StringVar(&configReadCacheFilename, "r", "readcache.json", "readcache location")
	flag.Parse()

	cfg, err := getFeedsFileConfigSection(configFilename)
	if err != nil {
		log.Fatal(err)
	}

	if cfg.Listen == "" {
		cfg.Listen = defaultListenAddress
	}

	if cfg.UpdateSeconds == 0 {
		cfg.UpdateSeconds = defaultUpdateTimeSeconds
	}

	err = rssole.Start(configFilename, configReadCacheFilename, cfg.Listen, time.Duration(cfg.UpdateSeconds)*time.Second)
	if err != nil {
		log.Fatal(err)
	}
}
