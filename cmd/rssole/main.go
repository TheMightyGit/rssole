package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"golang.org/x/exp/slog"

	"github.com/TheMightyGit/rssole/internal/rssole"
)

const (
	defaultListenAddress     = "0.0.0.0:8090"
	defaultUpdateTimeSeconds = 300

	defaultConfigFilename       = "rssole.json"
	defaultReadCacheFilename    = "rssole_readcache.json"
	oldDefaultConfigFilename    = "feeds.json"
	oldDefaultReadCacheFilename = "readcache.json"
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
		fmt.Println("RSSOLE version", rssole.Version)
		fmt.Println()
		originalUsage()
	}

	flag.StringVar(configFilename, "c", defaultConfigFilename, "config filename, must be writable")
	flag.StringVar(configReadCacheFilename, "r", defaultReadCacheFilename, "readcache filename, must be writable")
	flag.Parse()
}

func loadConfig(configFilename string) (rssole.ConfigSection, error) {
	cfg, err := getFeedsFileConfigSection(configFilename)
	if err != nil {
		return rssole.ConfigSection{}, err
	}

	if cfg.Listen == "" {
		cfg.Listen = defaultListenAddress
	}

	if cfg.UpdateSeconds == 0 {
		cfg.UpdateSeconds = defaultUpdateTimeSeconds
	}

	return cfg, nil
}

func main() {
	var configFilename, configReadCacheFilename string

	handleFlags(&configFilename, &configReadCacheFilename)

	// If the config file doesn't exist, try the old default name.
	if _, err := os.Stat(configFilename); errors.Is(err, os.ErrNotExist) {
		if configFilename != oldDefaultConfigFilename {
			if _, err := os.Stat(oldDefaultConfigFilename); err == nil {
				slog.Info("Falling back to old config filename:", "filename", oldDefaultConfigFilename)
				configFilename = oldDefaultConfigFilename
			}
		}
	}

	// If the readcache file doesn't exist, try the old default name.
	if _, err := os.Stat(configReadCacheFilename); errors.Is(err, os.ErrNotExist) {
		if configReadCacheFilename != oldDefaultReadCacheFilename {
			if _, err := os.Stat(oldDefaultReadCacheFilename); err == nil {
				slog.Info("Falling back to old readcache filename:", "filename", oldDefaultReadCacheFilename)
				configReadCacheFilename = oldDefaultReadCacheFilename
			}
		}
	}

	cfg, err := loadConfig(configFilename)
	if err != nil {
		slog.Error("unable to get config section of config file", "filename", configFilename, "error", err)
		os.Exit(1)
	}

	// Start service
	err = rssole.Start(configFilename, configReadCacheFilename, cfg.Listen, time.Duration(cfg.UpdateSeconds)*time.Second)
	if err != nil {
		slog.Error("rssole.Start exited with error", "error", err)
		os.Exit(1)
	}
}
