![badge](./badge.svg) ![workflow status](https://github.com/TheMightyGit/rssole/actions/workflows/build.yml/badge.svg)

# rssole (aka rissole)

An RSS Reader inspired by the late Google Reader. Runs on your local machine or local network serving your RSS feeds via a clean responsive web interface.

![Screenshot 2023-08-03 at 09 41 52](https://github.com/TheMightyGit/rssole/assets/888751/bf202040-2976-4570-8c2e-f6c21d61613e)

A single executable with a single config file that can largely be configured within the web UI.

Its greatest feature is the lack of excess features. It tries to do a simple job well and not get in the way.

## Background

I really miss Google Reader. Recently I noticed I'd gone back to an old habbit of jumping between various sites to scan their headlines, maintaining that sitelist purely in my head. So I looked at a few of the well knows RSS readers out there and nothing really grabbed me, either I didn't like the UI, or the install process seemed overly complicated, or there were just too many features, or ads. I like things simple.

So I made this non-SaaS ode to Google Reader so I can triage my incoming information in one place with one interface in a way I like. At heart this is a very self serving project solely based around my needs, and because of that it's something I use constantly. Hopefully it's of use to some other people, or you can build upon it (MIT license, do what you want to it - make it comfortable for you).

The name is supposed to be a pun on 'rissole'. As well as 'rissole' containing the letters R S and S, a rissole is a "*a compressed mixture of meat and spices, coated in breadcrumbs and fried*" and that struck me as similar to the role of an RSS reader (compressing the mixed meat of the internet into a handy faceful).

## Pre-Built Binaries and Packages

Check out the [Releases](https://github.com/TheMightyGit/rssole/releases/) section in github, there should be a good selection of pre-built binaries
and packages for various platforms.

## Installing via Brew

```console
$ brew install themightygit/rssole/rssole
```

## Installing via Go

You can install the binary with go install:

```console
$ go install github.com/TheMightyGit/rssole/cmd/rssole@latest
```

## Building

NOTE: You can ignore the `Makefile`, that's really just a helper for me during development.

To build for your local architecture/OS...

```console
$ go build ./cmd/...
```

It should also cross build for all the usual golang targets fine as well (as no CGO is used)...

```console
$ GOOS=linux GOARCH=amd64 go build ./cmd/...
$ GOOS=linux GOARCH=arm64 go build ./cmd/...
$ GOOS=darwin GOARCH=amd64 go build ./cmd/...
$ GOOS=darwin GOARCH=arm64 go build ./cmd/...
$ GOOS=windows GOARCH=amd64 go build ./cmd/...
$ GOOS=windows GOARCH=arm64 go build ./cmd/...
```

...but I only regularly test on `darwin/amd64` and `linux/amd64`.
I've seen it run on `windows/amd64`, but it's not something I try regularly.

### Smallest Binary

Go binaries can be a tad chunky, so if you're really space constrained then...

```console
$ go build -ldflags "-s -w" ./cmd/...
$ upx rssole
```

## Running

### Command Line

If you built locally then it should be in the current directory:

```console
$ ./rssole
```

If you used `go install` or brew then it should be on your path already:

```console
$ rssole
```

### GUI

Double click on the file, I guess.

If your system has restrictions on which binaries it will run then try compiling locally instead of
using the pre-built binaries.

## Now read your feeds with your browser

Now open your browser on `<hostname/ip>:8090` e.g. http://localhost:8090

## Network Options

By default it binds to `0.0.0.0:8090`, so it will be available on all network adaptors
on your host. You can change this in the `feeds.json` config file.

I run rssole within a private network so this is good enough for me so that I can run it once but
access it from all my devices. If you run this on an alien network then someone else can mess with
the UI (there's no protection at all on it) - change the `listen` value in `feeds.json` to
`127.0.0.1:8090` if you only want it to serve locally.

If you want to protect rssole behind a username and password or encryption (because you want rssole wide
open on the net so you can use it from anywhere) then you'll need a web proxy that can be configured
to sit in front of it to provide that protection. I'm highly unlikely to add username/password or encryption
directly to rssole as I don't need it. Maybe someone will create a docker image that autoconfigures all of that... maybe that someone is you?

## Config

### Arguments

```console
$ ./rssole -h
Usage of ./rssole:
  -c string
        config filename (default "feeds.json")
  -r string
        readcache location (default "readcache.json")
```

### `feeds.json`

There are two types of feed definition...

- Regular RSS URLs.
- Scrape from website (for those pesky sites that have no RSS feed).
  - Scraping uses css selectors and is not well documented yet.

Use `category` to group similar feeds together.

```json
{
  "config": {
    "listen": "0.0.0.0:8090",
    "update_seconds": 300
  },
  "feeds": [
    {"url":"https://github.com/TheMightyGit/rssole/releases.atom", "category":"Github Releases"},
    {"url":"https://news.ycombinator.com/rss", "category":"Nerd"},
    {"url":"http://feeds.bbci.co.uk/news/rss.xml", "category":"News"},
    {
      "url":"https://www.pcgamer.com/uk/news/", "category":"Games",
      "name":"PCGamer News",
      "scrape": {
        "urls": [
          "https://www.pcgamer.com/uk/news/",
          "https://www.pcgamer.com/uk/news/page/2/",
          "https://www.pcgamer.com/uk/news/page/3/"
        ],
        "item": ".listingResult",
        "title": ".article-name",
        "link": ".article-link"
      }
    }
  ]
}
```

## Key Dependencies

I haven't had to implement anything actually difficult, I just do a bit of plumbing.
All the difficult stuff has been done for me by these projects...

- github.com/mmcdole/gofeed - for reading all sorts of RSS formats.
- github.com/andybalholm/cascadia - for css selectors during website scrapes.
- github.com/k3a/html2text - for making a plain text summary of html.
- HTMX - for the javascript framework (a b/e engineers delight).
- Bootstrap 5 - for HTML niceness because I know it slightly better than the alternatives.
