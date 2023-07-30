package rssole

import (
	"fmt"
	"net/http"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

type scrape struct {
	URLs  []string `json:"urls"`
	Item  string   `json:"item"`
	Title string   `json:"title"`
	Link  string   `json:"link"`
}

func (conf *scrape) GeneratePseudoRssFeed() (string, error) {
	rss := `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
  <title>` + conf.URLs[0] + `</title>
  <link>` + conf.URLs[0] + `</link>
  <description>This RSS was scraped</description>
`

	for _, url := range conf.URLs {
		if url == "" {
			continue
		}

		resp, err := http.Get(url)
		if err != nil {
			return "", fmt.Errorf("get %s %w", url, err)
		}

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return "", fmt.Errorf("get non-success %d %s %w", resp.StatusCode, url, err)
		}

		doc, err := html.Parse(resp.Body)
		if err != nil {
			return "", fmt.Errorf("parse %s %w", url, err)
		}

		for _, p := range queryAll(doc, conf.Item) {
			titleNode := query(p, conf.Title)
			if titleNode != nil {
				titleChild := titleNode.FirstChild
				title := titleChild.Data
				// title := Query(p, f.Scrape.Title).FirstChild.Data
				link := attrOr(query(p, conf.Link), "href", "(No link available)")
				itemRss := `  <item>
    <title>` + title + `</title>
    <link>` + link + `</link>
    <description>` + title + `</description>
  </item>
`
				rss += itemRss
			}
		}
	}

	rss += `</channel>
</rss>`

	return rss, nil
}

func query(n *html.Node, query string) *html.Node {
	sel, err := cascadia.Parse(query)
	if err != nil {
		return &html.Node{}
	}

	return cascadia.Query(n, sel)
}

func queryAll(n *html.Node, query string) []*html.Node {
	sel, err := cascadia.Parse(query)
	if err != nil {
		return []*html.Node{}
	}

	return cascadia.QueryAll(n, sel)
}

func attrOr(n *html.Node, attrName, or string) string {
	for _, a := range n.Attr {
		if a.Key == attrName {
			return a.Val
		}
	}

	return or
}
