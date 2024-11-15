package rssole

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
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
<link foo="Should Be Deleted">
<meta foo="Should Be Deleted">
<iframe>Should Be Deleted</iframe>
<a></a>
<img foo="bar" width="10000" src="http://example.com/example.gif" alt="my alt" />
<svg width="200" height="250" version="1.1" xmlns="http://www.w3.org/2000/svg">
</svg>
<form></form>
`,
		},
	}
	expectedHTML := `<p><img src="http://example.com/example.gif" alt="my alt" /></p>
`

	d := w.Description()

	if d != expectedHTML {
		t.Fatal("description not as expected. got:", d, "expected:", expectedHTML)
	}
}

func TestImages_ShouldDedupe(t *testing.T) {
	w := wrappedItem{
		Item: &gofeed.Item{
			Image: &gofeed.Image{
				URL: "this_image_is_present_in_both",
			},
			Description: `
<img src='this_image_is_present_in_both' />
<svg src='this_svg_is_present_only_in_content' />
			`,
		},
	}

	images := w.Images()

	if len(images) != 0 {
		t.Error("expected image list to be zero as it should be de-duped")
	}
}

func TestImages_ShouldDedupeIgnoringAllQueryStrings(t *testing.T) {
	w := wrappedItem{
		Item: &gofeed.Item{
			Image: &gofeed.Image{
				URL: "this_image_is_present_in_both?also_ignores_query_string_here=7",
			},
			Description: `
<img src='this_image_is_present_in_both?query_string_is_ignored=1' />
<svg src='this_svg_is_present_only_in_content' />
			`,
		},
	}

	images := w.Images()

	if len(images) != 0 {
		t.Error("expected image list to be zero as it should be de-duped")
	}
}

func TestImages_ShouldNotDedupe(t *testing.T) {
	w := wrappedItem{
		Item: &gofeed.Item{
			Image: &gofeed.Image{
				URL: "http://example.com/this_image_is_only_present_in_meta.gif",
			},
			Description: `
<img src="http://example.com/this_other_image_is_present_only_in_content.gif" />
			`,
		},
	}

	images := w.Images()

	fmt.Println(images)

	if len(images) != 1 {
		t.Error("expected image list to be 1 as it should not be de-duped")
	}
}

func TestImages_MastodonExtensionImages(t *testing.T) {
	w := wrappedItem{
		Item: &gofeed.Item{
			Extensions: map[string]map[string][]ext.Extension{
				"media": {
					"content": {
						{
							Attrs: map[string]string{
								"medium": "image",
								"url":    "image_url_1",
							},
						},
						{
							Attrs: map[string]string{
								"medium": "image",
								"url":    "image_url_2",
							},
						},
					},
				},
			},
		},
	}

	images := w.Images()

	if len(images) != 2 {
		t.Error("expected image list to be 2")
	}
}
