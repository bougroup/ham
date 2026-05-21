package ham

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// partialEmbedRe matches <embed type="ham/partial" src="X" .../> (attribute order-independent,
// with or without trailing slash). This pre-parse substitution runs BEFORE html.Parse so the Go
// HTML5 parser cannot relocate embed elements out of <head> into <body>.
var partialEmbedRe = regexp.MustCompile(`(?i)<embed\s[^>]*type="ham/partial"[^>]*>`)

// partialEmbedSrcRe extracts the src attribute value from a ham/partial embed tag.
var partialEmbedSrcRe = regexp.MustCompile(`(?i)\bsrc="([^"]*)"`)

// partialEmbedReplaceRe extracts the data-ham-replace attribute value from a ham/partial embed tag.
var partialEmbedReplaceRe = regexp.MustCompile(`(?i)\bdata-ham-replace="([^"]*)"`)

// partialEmbedComment encodes a ham/partial embed as an HTML comment so it survives html.Parse
// without being hoisted. Format: <!--ham-embed:PATH;REPLACE--> where REPLACE may be empty.
// The "ham-embed:" prefix makes accidental collision with user comments vanishingly unlikely.
func partialEmbedComment(src, replace string) string {
	return "<!--ham-embed:" + src + ";" + replace + "-->"
}

// partialEmbedCommentRe matches comment markers written by partialEmbedComment, capturing the
// encoded payload (everything between "ham-embed:" and "-->").
var partialEmbedCommentRe = regexp.MustCompile(`<!--ham-embed:(.*?)-->`)

// rewritePartialEmbeds converts all <embed type="ham/partial" src="X"> tags to comment markers
// so that html.Parse cannot relocate them. Returns the rewritten HTML and the extracted Embeds.
func rewritePartialEmbeds(rawHTML []byte) ([]byte, []Embed) {
	var embeds []Embed
	result := partialEmbedRe.ReplaceAllFunc(rawHTML, func(match []byte) []byte {
		srcMatch := partialEmbedSrcRe.FindSubmatch(match)
		if srcMatch == nil {
			return match // no src — leave as-is
		}
		src := string(srcMatch[1])
		replace := ""
		if repMatch := partialEmbedReplaceRe.FindSubmatch(match); repMatch != nil {
			replace = string(repMatch[1])
		}
		embeds = append(embeds, Embed{Type: "ham/partial", Src: src, Replace: replace})
		return []byte(partialEmbedComment(src, replace))
	})
	return result, embeds
}

// restorePartialEmbedPlaceholders converts comment markers back to {embed:X} tokens so that the
// substitution loop can inject partial content at the correct position.
func restorePartialEmbedPlaceholders(html []byte) []byte {
	return partialEmbedCommentRe.ReplaceAllFunc(html, func(match []byte) []byte {
		m := partialEmbedCommentRe.FindSubmatch(match)
		if m == nil {
			return match
		}
		payload := string(m[1])
		// payload is "src;replace" — split on first ";" only
		idx := strings.Index(payload, ";")
		if idx < 0 {
			return []byte(embedPlaceholder(payload))
		}
		src := payload[:idx]
		return []byte(embedPlaceholder(src))
	})
}

const parseLimit = 1000 // max number of times to iterate and find partials inside partials
type Compiler struct {
	workingDir string
	outputDir  string
	pageHTML   []byte
	layoutHTML []byte
}

func New(workingDir, outputDir string) (*Compiler, error) {
	if _, err := os.Stat(filepath.Join(workingDir, "ham.json")); err != nil {
		return nil, fmt.Errorf("%s  is not a valid HAM project", workingDir)
	}

	return &Compiler{workingDir: workingDir, outputDir: outputDir}, nil
}

func (c *Compiler) Compile() error {
	// clear read cache so rebuilds pick up changes
	readCache = nil

	// create output directory
	if err := os.MkdirAll(c.outputDir, 0744); err != nil {
		return err
	}

	if err := c.compilePages(srcDir); err != nil {
		return err
	}

	return nil
}

func (c *Compiler) compilePages(dir string) error {
	pagesFiles, err := os.ReadDir(filepath.Join(c.workingDir, dir))
	if err != nil {
		return err
	}

	for _, page := range pagesFiles {
		pageName := page.Name()
		if page.IsDir() {
			if err := c.compilePages(filepath.Join(dir, pageName)); err != nil {
				return err
			}
			continue
		}

		// get file extension
		if filepath.Ext(page.Name()) != ".html" {
			log.Println("skipping file: " + page.Name())
			continue
		}

		srcFileName := filepath.Join(c.workingDir, dir, pageName)
		pageDir := strings.Replace(dir, "src", "", 1)
		pageFileName := filepath.Join(c.workingDir, c.outputDir, pageDir, pageName)
		file, err := os.Open(srcFileName)
		if err != nil {
			return err
		}

		// parse dom
		doc, err := html.Parse(file)
		if err != nil {
			return err
		}

		hasEmbeds := true
		i := 0
		for hasEmbeds && i < parseLimit {
			doc, hasEmbeds, err = c.compile(doc, srcFileName)
			if err != nil {
				return err
			}
			i++
		}

		// this should take care of any "ham-remove" found in embedded partials
		c.compile(doc, srcFileName)

		// write final html to file
		log.Println("Creating page: " + pageFileName + " from " + srcFileName)
		if err := os.MkdirAll(filepath.Dir(pageFileName), os.ModePerm); err != nil {
			return err
		}
		if err := os.WriteFile(pageFileName, c.pageHTML, os.ModePerm); err != nil {
			return err
		}
		c.Reset()
	}
	return nil
}

func (c *Compiler) compile(doc *html.Node, pageFilePath string) (*html.Node, bool, error) {
	page := ParsePage(doc)

	pageCssFileName := strings.ReplaceAll(pageFilePath, ".html", ".css")
	page.Layout.CSS = append(page.Layout.CSS, pageCssFileName)

	pageTsFileName := strings.ReplaceAll(pageFilePath, ".html", ".ts")
	page.Layout.JsMod = append(page.Layout.JsMod, pageTsFileName)

	log.Println("Resources", pageFilePath, page.Layout.CSS, page.Layout.Js, page.Layout.JsMod)
	dedupe := make(map[string]bool)
	pageResources := append([]string{}, page.Layout.CSS...)
	pageResources = append(pageResources, page.Layout.Js...)
	pageResources = append(pageResources, page.Layout.JsMod...)

	pageCSS := make([]string, len(page.Layout.CSS))
	pageJs := make([]string, len(page.Layout.Js))
	for _, res := range pageResources {
		if !filepath.IsAbs(res) {
			res = filepath.Join(filepath.Dir(pageFilePath), res) // re-adjust res path
		}
		if _, ok := dedupe[res]; ok {
			continue
		}
		dedupe[res] = true
		if err := createFile(res, nil, false); err != nil {
			log.Println("error writing css file", err.Error())
		}
		i := strings.Index(res, string(filepath.Separator)+"assets") // make path os portable
		if i >= 0 {
			res = res[i:]
		}

		basePath := strings.Split(res, srcDir)[1]
		root := ""
		if len(strings.Split(basePath, string(os.PathSeparator))) > 1 {
			root = "/"
		}

		switch filepath.Ext(res) {
		case ".css":
			assetPath := filepath.ToSlash(filepath.Join(root, "assets", "css", basePath))
			pageCSS = append(pageCSS, `<link rel="stylesheet" href="`+assetPath+`">`)
		case ".js":
			assetPath := filepath.ToSlash(filepath.Join(root, "assets", "js", basePath))
			pageJs = append(pageJs, `<script src="`+assetPath+`"></script>`)
		case ".ts":
			assetPath := filepath.ToSlash(filepath.Join(root, "assets", "js", basePath))
			assetPath = strings.Replace(assetPath, ".ts", ".js", 1)
			pageJs = append(pageJs, `<script type="module" src="`+assetPath+`"></script>`)
		}
	}

	buf := &bytes.Buffer{}
	if err := html.Render(buf, doc); err != nil {
		return nil, false, err
	}

	c.pageHTML = make([]byte, buf.Len())
	copy(c.pageHTML, buf.Bytes())

	if c.layoutHTML == nil && page.Layout.Src != "" {
		layoutFilePath := filepath.Join(filepath.Dir(pageFilePath), page.Layout.Src)
		if _, err := os.Stat(layoutFilePath); err != nil {
			return nil, false, fmt.Errorf("failed to compile %s. Layout file %s not found", pageFilePath, layoutFilePath)
		}
		log.Printf("Compiling Page: %s with %s\n", pageFilePath, layoutFilePath)

		c.layoutHTML = readFile(layoutFilePath)

		// Pre-parse: replace ALL <embed type="ham/partial" src="X"> tags with comment markers so
		// the HTML5 parser cannot relocate them (e.g. from <head> to <body>). The rewrite captures
		// src and data-ham-replace so we can synthesise Embed entries for the substitution loop.
		var layoutEmbeds []Embed
		c.layoutHTML, layoutEmbeds = rewritePartialEmbeds(c.layoutHTML)

		lDoc, err := html.Parse(bytes.NewBuffer(c.layoutHTML))
		if err != nil {
			return nil, false, err
		}

		layout := ParseLayout(lDoc)
		layout.Path = layoutFilePath

		// Register partial embeds (extracted before html.Parse) so they get substituted below.
		layout.Embeds = append(layout.Embeds, layoutEmbeds...)

		buf.Reset()
		if err := html.Render(buf, lDoc); err != nil {
			return nil, false, err
		}

		// Post-render: convert comment markers back to {embed:X} tokens so the substitution
		// loop below can inject partial content at the correct position.
		c.layoutHTML = restorePartialEmbedPlaceholders(buf.Bytes())
		c.pageHTML = bytes.Replace(c.pageHTML, []byte("<html><head></head><body>"), []byte(""), 1) // strip out <html><head></head><body>
		c.pageHTML = bytes.Replace(c.pageHTML, []byte("</body></html>"), []byte(""), 1)            // strip out </body></html>
		c.pageHTML = bytes.Replace(c.layoutHTML, []byte("{ham:page}"), c.pageHTML, 1)
		if page.Layout.ID != "" {
			bodyTagRe := regexp.MustCompile(`<body([^>]*)>`)
			c.pageHTML = bodyTagRe.ReplaceAll(c.pageHTML, []byte(`<body$1 id="`+page.Layout.ID+`">`))
		}

		// find and replace layout embeds
		for _, embed := range layout.Embeds {
			if embed.Src != "" {
				embedFilePath := filepath.Join(filepath.Dir(layout.Path), embed.Src)
				log.Println("embedding", embedFilePath)
				embedContent := readFile(embedFilePath)

				if embed.Replace != "" {
					embedContent = c.handleEmbedReplacements(embedContent, embed.Replace)
				}

				c.pageHTML = bytes.ReplaceAll(c.pageHTML, []byte(embedPlaceholder(embed.Src)), embedContent)
			}
		}
	}

	c.pageHTML = bytes.ReplaceAll(c.pageHTML, []byte("{ham:css}"), []byte(strings.Join(pageCSS, "\n")))
	c.pageHTML = bytes.ReplaceAll(c.pageHTML, []byte("{ham:js}"), []byte(strings.Join(pageJs, "\n")))

	// find and replace page embeds (body-level partials collected by parsePage DOM walk)
	for _, embed := range page.Embeds {
		if embed.Src != "" {
			embedFilePath := filepath.Join(filepath.Dir(pageFilePath), embed.Src)
			log.Println("embedding", embedFilePath)
			embedContent := readFile(embedFilePath)

			if embed.Replace != "" {
				embedContent = c.handleEmbedReplacements(embedContent, embed.Replace)
			}

			c.pageHTML = bytes.ReplaceAll(c.pageHTML, []byte(embedPlaceholder(embed.Src)), embedContent)
		}
	}

	// Pre-parse pass on the merged pageHTML before each re-parse: converts any ham/partial embed
	// tags introduced by substituted content (e.g. partials-inside-partials) to comment markers so
	// html.Parse cannot hoist them. Loop until no further embeds are introduced.
	foundNested := false
	for {
		var nestedEmbeds []Embed
		c.pageHTML, nestedEmbeds = rewritePartialEmbeds(c.pageHTML)
		if len(nestedEmbeds) == 0 {
			break
		}
		foundNested = true
		c.pageHTML = restorePartialEmbedPlaceholders(c.pageHTML)
		for _, embed := range nestedEmbeds {
			if embed.Src != "" {
				embedFilePath := filepath.Join(filepath.Dir(pageFilePath), embed.Src)
				log.Println("embedding nested", embedFilePath)
				embedContent := readFile(embedFilePath)
				if embed.Replace != "" {
					embedContent = c.handleEmbedReplacements(embedContent, embed.Replace)
				}
				c.pageHTML = bytes.ReplaceAll(c.pageHTML, []byte(embedPlaceholder(embed.Src)), embedContent)
			}
		}
	}

	doc, err := html.Parse(bytes.NewBuffer(c.pageHTML))
	if err != nil {
		return nil, false, err
	}

	return doc, len(page.Embeds) > 0 || foundNested, nil
}

func (c *Compiler) handleEmbedReplacements(content []byte, replacements string) []byte {
	replaces := strings.Split(replacements, ",")
	for _, replace := range replaces {
		find := strings.Split(replace, ":")
		if len(find) == 2 {
			log.Println("replacing", embedReplaceKey(find[0]), find[1])
			content = bytes.ReplaceAll(content, []byte(embedReplaceKey(find[0])), []byte(find[1]))
		}
	}

	return content
}

func (c *Compiler) Reset() {
	c.pageHTML = nil
	c.layoutHTML = nil
}

var readCache map[string][]byte

func readFile(filename string) []byte {
	if readCache == nil {
		readCache = make(map[string][]byte)
	}

	if _, ok := readCache[filename]; !ok {
		file, err := os.ReadFile(filename)
		if err != nil {
			return nil
		}
		readCache[filename] = file
	}

	return readCache[filename]
}

func createFile(filePath string, content []byte, override bool) error {
	if !override {
		if _, err := os.Stat(filePath); err == nil {
			return nil
		}
	}
	log.Println("Creating file: " + filePath)
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}
	return os.WriteFile(filePath, content, os.ModePerm)
}
