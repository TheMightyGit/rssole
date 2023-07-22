package rssole

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScrape(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `<html>
<body>
	<div class="item">
		<p class="title">Title 1</p>
		<a class="link" href="http://title1.com/">Title 1</a>
	</div>
	<div class="item">
		<p class="title">Title 2</p>
		<a class="link" href="http://title2.com/">Title 1</a>
	</div>
</body>
</html>`)
	}))
	defer ts.Close()

	expectedFeedStr := `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
  <title>Scraped Feed</title>
  <link>` + ts.URL + `</link>
  <description>This RSS was scraped</description>
  <item>
    <title>Title 1</title>
    <link>http://title1.com/</link>
    <description>Title 1</description>
  </item>
  <item>
    <title>Title 2</title>
    <link>http://title2.com/</link>
    <description>Title 2</description>
  </item>
</channel>
</rss>`

	conf := scrape{
		URLs: []string{
			ts.URL,
		},
		Item:  ".item",
		Title: ".title",
		Link:  ".link",
	}

	feedStr := conf.GeneratePseudoRssFeed()

	if feedStr != expectedFeedStr {
		t.Fatal(feedStr, "does not equal", expectedFeedStr)
	}
}
