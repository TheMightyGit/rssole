package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"log/slog"

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
		return cfgFile.Config, fmt.Errorf("error opening file: %w", err)
	}
	defer jsonFile.Close()

	decoder := json.NewDecoder(jsonFile)

	err = decoder.Decode(&cfgFile)
	if err != nil {
		return cfgFile.Config, fmt.Errorf("error unmarshalling JSON: %w", err)
	}

	return cfgFile.Config, nil
}

func handleFlags(configFilename, configReadCacheFilename *string) {
	originalUsage := flag.Usage
	flag.Usage = func() {
		fmt.Println("RiSSOLE version", rssole.Version)
		fmt.Println()
		originalUsage()
	}

	flag.StringVar(configFilename, "c", "feeds.json", "config filename, must be writable")
	flag.StringVar(configReadCacheFilename, "r", "readcache.json", "readcache filename, must be writable")
	flag.Parse()
}

func loadConfig(configFilename string) rssole.ConfigSection {
	cfg, err := getFeedsFileConfigSection(configFilename)
	if err != nil {
		slog.Error("unable to get config section of config file", "filename", configFilename, "error", err)
		os.Exit(1)
	}

	if cfg.Listen == "" {
		cfg.Listen = defaultListenAddress
	}

	if cfg.UpdateSeconds == 0 {
		cfg.UpdateSeconds = defaultUpdateTimeSeconds
	}

	return cfg
}

func main() {
	var configFilename, configReadCacheFilename string

	handleFlags(&configFilename, &configReadCacheFilename)

	cfg := loadConfig(configFilename)

	// Start service
	err := rssole.Start(configFilename, configReadCacheFilename, cfg.Listen, time.Duration(cfg.UpdateSeconds)*time.Second)
	if err != nil {
		slog.Error("rssole.Start exited with error", "error", err)
		os.Exit(1)
	}
}
