# rssole

A simple RSS Reader inspired by Google Reader.

![Screenshot 2023-07-19 at 10 58 50](https://github.com/TheMightyGit/rssole/assets/888751/73fbbc46-dccb-4550-a048-a09ab0f35757)

It's just a single binary with a single config file, feeds are viewed via your web browser. No apps, no web servers, no signing in, nothing special at all.

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
on your host.

I run rssole within a private network so this is good enough for me so that I can run it once but
access it from all my devices. If you run this on an alien network then someone else can mess with
the UI (there's no protection at all on it) - change the code to `127.0.0.1:8090` if you only want
it to serve locally.

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
  "feeds": [
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

- Feeds are updated every 300 seconds. Currently not configurable.
- `readcache.json` (which notes which URLs you've marked as read) grows unbounded at the moment.
- 'Add Feed' in the UI doesn't work (yet). Well, it does, but it doesn't persist it so it'll vanish on next restart.
- No tests and some poor error handling in places.

## Key Dependencies

I haven't had to implement anything actually difficult, I just do a bit of plumbing.
All the difficult stuff has been done for me by these projects...

- github.com/mmcdole/gofeed - for reading all sorts of RSS formats.
- github.com/andybalholm/cascadia - for css selectors during website scrapes.
- github.com/k3a/html2text - for making a plain text summary of html.
- HTMX - for the javascript framework (a b/e engineers delight).
- Bootstrap 5 - for HTML niceness because I know it slightly better than the alternatives.
