package ham

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"golang.org/x/net/html"
)

type Layout struct {
	ID     string   `json:"id"`
	Src    string   `json:"layout"`
	Path   string   `json:"path"`
	CSS    []string `json:"css,omitempty"`
	Js     []string `json:"js,omitempty"`
	JsMod  []string `json:"js-mod,omitempty"`
	Embeds []Embed
}

type Page struct {
	Layout *Layout
	Embeds []Embed
}

type Embed struct {
	Type    string
	Src     string
	Replace string
}

func ParseLayout(doc *html.Node) Layout {
	var layout Layout
	parseLayout(doc, &layout)
	return layout
}

func parseLayout(start *html.Node, layout *Layout) {
	if layout == nil {
		panic("layout is nil")
	}
	// fmt.Println("layout transversing ", start.Data)
	if start.Type == html.ElementNode {
		switch start.Data {
		case "link":
			for _, attr := range start.Attr {
				if attr.Key == "type" && attr.Val == "ham/layout-css" {
					start.Data = "{ham:css}"
					start.Type = 1
					layout.Embeds = append(layout.Embeds, Embed{Type: attr.Val})
				}
			}
		case "embed":
			em := Embed{}
			for _, attr := range start.Attr {
				switch attr.Key {
				case "type":
					em.Type = attr.Val
				case "src":
					em.Src = attr.Val
				case "data-ham-replace":
					em.Replace = attr.Val
				}
			}
			layout.Embeds = append(layout.Embeds, em)
			start.Type = 1
			switch em.Type {
			case "ham/partial":
				start.Data = embedPlaceholder(em.Src)
			case "ham/page":
				start.Data = "{ham:page}"
			case "ham/layout-js":
				start.Data = "{ham:js}"
			case "ham/layout-css":
				start.Data = "{ham:css}"
			}
		}
	}

	for start = start.FirstChild; start != nil; start = start.NextSibling {
		parseLayout(start, layout)
	}
}

func ParsePage(doc *html.Node) Page {
	var page Page
	parsePage(doc, &page)
	return page
}

func parsePage(start *html.Node, page *Page) {
	if page.Layout == nil {
		page.Layout = &Layout{}
	}
	// fmt.Println("page transversing ", start.Data)
	if start.Type == html.ElementNode {
		for _, attr := range start.Attr {
			if attr.Key == "class" && strings.Contains(attr.Val, "ham-remove") {
				start.Parent.RemoveChild(start)
			}
		}
		switch start.Data {
		case "embed":
			em := Embed{}
			for _, attr := range start.Attr {
				switch attr.Key {
				case "type":
					em.Type = attr.Val
				case "src":
					em.Src = attr.Val
				case "data-ham-replace":
					em.Replace = attr.Val
				}
			}
			start.Type = 1
			switch em.Type {
			case "ham/partial":
				// replace <embed> tag with placeholders
				start.Data = embedPlaceholder(em.Src)
				page.Embeds = append(page.Embeds, em)
			case "ham/page":
				start.Data = "{ham:page}"
			case "ham/layout-js":
				start.Data = "{ham:js}"
			case "ham/layout-css":
				start.Data = "{ham:css}"
			}
		case "div":
			var newAttr []html.Attribute
			for _, attr := range start.Attr {
				switch attr.Key {
				case "data-ham-page-config":
					if err := json.Unmarshal([]byte(attr.Val), page.Layout); err != nil {
						log.Println("config decode error", err.Error())
						continue
					}
				default:
					newAttr = append(newAttr, attr)
				}
				start.Attr = newAttr
			}
		}
	}

	for start = start.FirstChild; start != nil; start = start.NextSibling {
		parsePage(start, page)
	}
}

func embedPlaceholder(src string) string {
	return "{embed:" + src + "}"
}

func embedReplaceKey(key string) string {
	return fmt.Sprintf("__%s__", key)
}
