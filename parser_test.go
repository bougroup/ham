package ham

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestParsePage(t *testing.T) {
	file, _ := os.Open("./test-site/src/index.html")

	// parse dom
	doc, _ := html.Parse(file)
	page := ParsePage(doc)

	want := 4
	got := len(page.Embeds)
	if got != want {
		t.Errorf("parse failed: expected %d but got %d page embeds", want, got)
	}

	want = 0
	got = len(page.Layout.Js)
	if got != want {
		t.Errorf("parse failed: expected %d but got %d layout Js embeds", want, got)
	}

	want = 0
	got = len(page.Layout.CSS)
	if got != want {
		t.Errorf("parse failed: expected %d but got %d layout CSS embeds", want, got)
	}

	buf := &bytes.Buffer{}
	if err := html.Render(buf, doc); err != nil {
		t.Errorf("render failed: %v", err)
	}

	fmt.Println(buf.String())
}

func TestParseLayout(t *testing.T) {
	file, _ := os.Open("./test-site/src/default.lhtml")

	// parse dom
	doc, _ := html.Parse(file)
	layout := ParseLayout(doc)

	want := 3
	got := len(layout.Embeds)
	if got != want {
		t.Errorf("parse failed: expected %d but got %d page embeds", want, got)
	}
}

// TestPartialEmbedRewrite verifies that rewritePartialEmbeds converts ham/partial embed tags to
// comment markers (preserving src and data-ham-replace), and that restorePartialEmbedPlaceholders
// converts them back to {embed:X} tokens ready for the substitution loop.
func TestPartialEmbedRewrite(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		src     string
		replace string
	}{
		{
			name:    "simple self-closing in head",
			input:   `<head><embed type="ham/partial" src="../partials/head.phtml"/></head>`,
			src:     "../partials/head.phtml",
			replace: "",
		},
		{
			name:    "attribute order reversed",
			input:   `<head><embed src="../partials/head.phtml" type="ham/partial" /></head>`,
			src:     "../partials/head.phtml",
			replace: "",
		},
		{
			name:    "path with hyphens",
			input:   `<head><embed type="ham/partial" src="../partials/app-head.phtml"/></head>`,
			src:     "../partials/app-head.phtml",
			replace: "",
		},
		{
			name:    "with data-ham-replace",
			input:   `<head><embed type="ham/partial" src="../partials/head.phtml" data-ham-replace="TITLE:Hello,COLOR:red"/></head>`,
			src:     "../partials/head.phtml",
			replace: "TITLE:Hello,COLOR:red",
		},
		{
			name:    "data-ham-replace before src",
			input:   `<body><embed data-ham-replace="KEY:val" type="ham/partial" src="sidebar.phtml"/></body>`,
			src:     "sidebar.phtml",
			replace: "KEY:val",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rewritten, embeds := rewritePartialEmbeds([]byte(tc.input))
			if len(embeds) != 1 {
				t.Fatalf("expected 1 extracted embed, got %d", len(embeds))
			}
			if embeds[0].Src != tc.src {
				t.Errorf("extracted src: got %q, want %q", embeds[0].Src, tc.src)
			}
			if embeds[0].Replace != tc.replace {
				t.Errorf("extracted replace: got %q, want %q", embeds[0].Replace, tc.replace)
			}

			// The rewritten form should contain the comment marker, not the embed tag.
			expectedComment := partialEmbedComment(tc.src, tc.replace)
			if !strings.Contains(string(rewritten), expectedComment) {
				t.Errorf("rewritten HTML missing comment marker %q\ngot: %s", expectedComment, rewritten)
			}
			if strings.Contains(string(rewritten), `type="ham/partial"`) {
				t.Errorf("rewritten HTML still contains ham/partial embed tag")
			}

			// Simulate html.Parse + html.Render round-trip.
			doc, err := html.Parse(strings.NewReader(string(rewritten)))
			if err != nil {
				t.Fatalf("html.Parse failed: %v", err)
			}
			buf := &bytes.Buffer{}
			if err := html.Render(buf, doc); err != nil {
				t.Fatalf("html.Render failed: %v", err)
			}
			rendered := buf.Bytes()

			// The comment marker must survive the parse/render round-trip.
			if !strings.Contains(string(rendered), expectedComment) {
				t.Errorf("comment marker lost after html.Parse+Render round-trip\ngot: %s", rendered)
			}

			// Restore to {embed:X} placeholder.
			restored := restorePartialEmbedPlaceholders(rendered)
			expectedPlaceholder := embedPlaceholder(tc.src)
			if !strings.Contains(string(restored), expectedPlaceholder) {
				t.Errorf("restored HTML missing placeholder %q\ngot: %s", expectedPlaceholder, restored)
			}
		})
	}
}

// TestPartialEmbedReplaceAttribute verifies that data-ham-replace survives the pre-pass and is
// correctly carried in the Embed struct so handleEmbedReplacements can apply it.
func TestPartialEmbedReplaceAttribute(t *testing.T) {
	input := `<embed type="ham/partial" src="widget.phtml" data-ham-replace="TITLE:Hello World,COLOR:blue"/>`
	_, embeds := rewritePartialEmbeds([]byte(input))

	if len(embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(embeds))
	}
	em := embeds[0]
	if em.Src != "widget.phtml" {
		t.Errorf("src: got %q, want %q", em.Src, "widget.phtml")
	}
	if em.Replace != "TITLE:Hello World,COLOR:blue" {
		t.Errorf("replace: got %q, want %q", em.Replace, "TITLE:Hello World,COLOR:blue")
	}

	// Simulate handleEmbedReplacements applying the replace string.
	c := &Compiler{}
	partial := []byte("<h1>__TITLE__</h1><span class=\"__COLOR__\">test</span>")
	result := c.handleEmbedReplacements(partial, em.Replace)
	if !strings.Contains(string(result), "<h1>Hello World</h1>") {
		t.Errorf("TITLE replacement failed, got: %s", result)
	}
	if !strings.Contains(string(result), `class="blue"`) {
		t.Errorf("COLOR replacement failed, got: %s", result)
	}
}

// TestPartialEmbedInHeadSurvivesParse verifies that a ham/partial embed in <head> correctly
// survives html.Parse without being relocated to <body>.
func TestPartialEmbedInHeadSurvivesParse(t *testing.T) {
	input := `<html><head><embed type="ham/partial" src="../partials/head.phtml"/></head><body><p>content</p></body></html>`

	// Pre-pass converts embed to comment.
	rewritten, embeds := rewritePartialEmbeds([]byte(input))
	if len(embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(embeds))
	}

	// Parse and render — the comment must stay in <head>.
	doc, err := html.Parse(strings.NewReader(string(rewritten)))
	if err != nil {
		t.Fatalf("html.Parse failed: %v", err)
	}
	buf := &bytes.Buffer{}
	if err := html.Render(buf, doc); err != nil {
		t.Fatalf("html.Render failed: %v", err)
	}
	rendered := buf.String()

	// Comment must appear before </head>, not in <body>.
	headIdx := strings.Index(rendered, "</head>")
	bodyIdx := strings.Index(rendered, "<body>")
	commentIdx := strings.Index(rendered, "<!--ham-embed:")
	if commentIdx < 0 {
		t.Fatalf("comment marker not found in rendered HTML: %s", rendered)
	}
	if commentIdx > headIdx {
		t.Errorf("comment marker appeared after </head> — expected inside <head>\nrendered: %s", rendered)
	}
	if commentIdx > bodyIdx {
		t.Errorf("comment marker appeared inside <body> — expected inside <head>\nrendered: %s", rendered)
	}

	// Restore to {embed:X}.
	restored := restorePartialEmbedPlaceholders(buf.Bytes())
	expectedPlaceholder := embedPlaceholder("../partials/head.phtml")
	if !strings.Contains(string(restored), expectedPlaceholder) {
		t.Errorf("restored HTML missing placeholder %q\ngot: %s", expectedPlaceholder, restored)
	}
}

// TestPartialEmbedBodyUnchangedByPrePass verifies that a body-level ham/partial embed is processed
// correctly by rewritePartialEmbeds (it should be rewritten to a comment, surviving parse).
func TestPartialEmbedBodyUnchangedByPrePass(t *testing.T) {
	// Body embeds are typically handled via parsePage DOM walk, not the pre-pass.
	// But if passed through the pre-pass (e.g. nested partial from substituted content),
	// they should be correctly converted to comment markers.
	input := `<body><embed type="ham/partial" src="footer.phtml"/></body>`
	rewritten, embeds := rewritePartialEmbeds([]byte(input))
	if len(embeds) != 1 {
		t.Errorf("expected 1 embed from body partial, got %d", len(embeds))
	}
	if strings.Contains(string(rewritten), `type="ham/partial"`) {
		t.Errorf("ham/partial embed tag not replaced by comment")
	}
	if !strings.Contains(string(rewritten), "<!--ham-embed:footer.phtml;-->") {
		t.Errorf("comment marker not found, got: %s", rewritten)
	}
}

func TestConfigParse(t *testing.T) {
	config := `{
     "layout": "../layouts/authed.html",
     "css": [
     "../assets/css/global.css"
     ],
     "js":[
     "../assets/js/app.js"
     ]}`

	var layout Layout
	err := json.Unmarshal([]byte(config), &layout)
	if err != nil {
		t.Errorf("parse failed: %v", err)
		return
	}

	if layout.Src != "../layouts/authed.html" {
		t.Errorf("parse failed: expected %s but got %s", "../layouts/authed.html", layout.Src)
	}
	if len(layout.CSS) != 1 {
		t.Errorf("parse failed: expected %d but got %d", 1, len(layout.CSS))
	}
	if len(layout.Js) != 1 {
		t.Errorf("parse failed: expected %d but got %d", 1, len(layout.Js))
	}
	if len(layout.JsMod) != 0 {
		t.Errorf("parse failed: expected %d but got %d", 0, len(layout.JsMod))
	}
}
