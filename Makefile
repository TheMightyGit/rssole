##
## NOTE: This makefile is just helpful stuff for the developer.
## You don't need it to build this project, just use the regular go tooling.
##
GO?=go
GO_CODE=$(shell find . -name '*.go')
GO_TEMPLATES=$(shell find . -name '*.go.html')
SOURCES=go.mod Makefile $(GO_CODE) $(GO_TEMPLATES)

.PHONY: all loc test lint gotest build run updatedeps clean releaselocal

all: build

loc:
	wc -l `git ls-files '*.go'` | sort
	wc -l `git ls-files '*.go.html'` | sort

test: lint gotest badge.svg

lint:
	golangci-lint run --timeout 5m0s ./...

gotest:
	$(GO) test -race -cover ./...

badge.svg: $(SOURCES)
	AMOUNT=$(shell $(GO) test -cover ./internal/rssole | cut -f 4 | cut -f 2 -d ' ' | cut -f 1 -d '.'); \
	sed "s/100%/$$AMOUNT%/g" $@.template >$@

build: rssole

run:
	$(GO) run -race ./cmd/rssole

rssole: $(SOURCES)
	$(GO) build ./cmd/rssole

updatedeps:
	$(GO) get -u ./...

coveragereport:
	go test -v -coverprofile cover.out ./...
	go tool cover -html cover.out -o cover.html
	open cover.html

releaselocal:
	goreleaser release --snapshot --clean

clean:
	$(GO) clean
	$(GO) clean -cache -modcache -testcache
	rm -Rf dist
	rm -Rf .test_dummy
	rm -f rssole

deploycolin:
	-ssh colin.local "killall rssole"
	GOOS=linux make build
	scp rssole colin.local:.
	ssh colin.local "nohup ./rssole 2>rssole.out 1>rssole.out &"
