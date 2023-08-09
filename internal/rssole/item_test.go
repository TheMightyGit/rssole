package rssole

import (
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

func TestIsDescriptionImage(t *testing.T) {
	w := wrappedItem{
		Item: &gofeed.Item{
			Description: `
<img src='this_image_is_present' />
<svg src='this_svg_is_present' />
<button src='this_not_an_image' />
			`,
		},
	}

	if !w.isDescriptionImage("this_image_is_present") {
		t.Error("expected to find 'this_image_is_present' in description images")
	}

	if !w.isDescriptionImage("this_svg_is_present") {
		t.Error("expected to find 'this_svg_is_present' in description images")
	}

	if w.isDescriptionImage("this_not_an_image") {
		t.Error("expected not to find 'this_not_an_image' in description images")
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

func TestImages_ShouldNotDedupe(t *testing.T) {
	w := wrappedItem{
		Item: &gofeed.Item{
			Image: &gofeed.Image{
				URL: "this_image_is_only_present_in_image",
			},
			Description: `
<img src='this_image_is_present_only_in_content' />
<svg src='this_svg_is_present_only_in_content' />
			`,
		},
	}

	images := w.Images()

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
