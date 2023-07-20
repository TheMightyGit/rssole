package main

import (
	"os"

	"github.com/TheMightyGit/rssole/internal/rssole"
)

const (
	defaultListenAddress = "0.0.0.0:8090"
)

func main() {
	listenAddress := os.Getenv("RSSOLE_ADDRESS")
	if listenAddress == "" {
		listenAddress = defaultListenAddress
	}
	rssole.Start(listenAddress)
}
