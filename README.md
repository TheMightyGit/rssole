# rssole

A simple RSS Reader inspired by Google Reader.

![Screenshot 2023-07-22 at 21 16 25](https://github.com/TheMightyGit/rssole/assets/888751/703bd856-e777-4010-91ea-a8e5487f48b4)

It's just a single binary with a single config file, feeds are viewed via your web browser. No apps, no web servers, no signing in, no analytics,
no ads, no recommendations, no growth objectives, absolutely nothing special at all.

## Background

I miss Google Reader, it put me in charge of how I triaged information. One place, one interface. When it was discontinued in 2013 I tried a few alternatives but nothing really stuck.
With the growth of Twitter at the time I could get some aspect of what I lost, and so it was for many years. However, between 2013 and the 2020 Twitter greatly lessened my ability to
control how I consumed information.

A few years on from leaving Twitter I realised I'd gone back to jumping between various sites to scan their headlines, maintaining that list of sites purely in my head.

Then I had a Clown College moment but for creating an RSS reader that gave me the same feels as Google Reader. Not only is it a fun exercise, it turns out RSS is more alive and
kicking than I gave it credit for. In only a few short hours I had something that started to feel right. I've been using it as my main information triage ever since.

The name is a pun on 'rissole'. Firstly it contains the letters r s and s and secondly because "a compressed mixture of meat and spices, coated in breadcrumbs and fried"
didn't strike me as that dissimilar from the job of an RSS reader.

## Pre-Built Binaries

Check out the [Releases](https://github.com/TheMightyGit/rssole/releases/) section in github, there should be a good selection of pre-built binaries
for various platforms.

## Building

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

...but I've only tested `darwin/amd64` and `linux/amd64`.

### Smallest Binary

Go binaries can be a tad chunky, so if you're really space constrained then...

```console
$ go build -ldflags "-s -w" ./cmd/...
$ upx rssole
```

## Running

```console
$ ./rssole
```

Now open your browser on `<hostname/ip>:8090` e.g. http://localhost:8090

## Network

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

## Notes / TODOs / Warnings

- `readcache.json` (which notes which URLs you've marked as read) grows unbounded at the moment.
- No tests and some poor error handling in places. It's very much prototype code right now.

## Key Dependencies

I haven't had to implement anything actually difficult, I just do a bit of plumbing.
All the difficult stuff has been done for me by these projects...

- github.com/mmcdole/gofeed - for reading all sorts of RSS formats.
- github.com/andybalholm/cascadia - for css selectors during website scrapes.
- github.com/k3a/html2text - for making a plain text summary of html.
- HTMX - for the javascript framework (a b/e engineers delight).
- Bootstrap 5 - for HTML niceness because I know it slightly better than the alternatives.
