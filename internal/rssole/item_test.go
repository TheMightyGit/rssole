package rssole

import (
	"strings"
	"testing"

	"github.com/mmcdole/gofeed"
)

func TestSummary_TruncateAt200(t *testing.T) {
	w := wrappedItem{
		Item: &gofeed.Item{
			Description: strings.Repeat("x", 300),
		},
	}

	s := w.Summary()

	if len(s) != 200 {
		t.Fatal("summary not truncted to 200")
	}
}

func TestSummary_BlankIfSummaryIdenticalToTitle(t *testing.T) {
	w := wrappedItem{
		Item: &gofeed.Item{
			Title:       "These are the same",
			Description: "These are the same",
		},
	}

	s := w.Summary()

	if s != "" {
		t.Fatal("summary was not blanked when identical to title")
	}
}

func TestSummary_BlankIfSummaryAURL(t *testing.T) {
	w := wrappedItem{
		Item: &gofeed.Item{
			Description: "http://example.com",
		},
	}

	s := w.Summary()

	if s != "" {
		t.Fatal("summary was not blanked when a url")
	}
}

func TestDescription_HtmlSanitised(t *testing.T) {
	w := wrappedItem{
		Item: &gofeed.Item{
			Description: `
<script>Should Be Deleted</script>
<style>Should Be Deleted</style>
<meta foo="Should Be Deleted">
<iframe>Should Be Deleted</iframe>
<a></a>
<img >
<svg >
`,
		},
	}
	expectedHTML := `<html><head>


</head><body>
<a target="_new"></a>
<img style="max-width: 60%;"/>
<svg style="max-width: 60%;">
</svg></body></html>`

	d := w.Description()

	if d != expectedHTML {
		t.Fatal("description not as expected. got", d, "expected:", expectedHTML)
	}
}
