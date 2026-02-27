package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"
)

const (
	maxTextLength = 15_000
	httpTimeout   = 30 * time.Second
)

// ExtractTextFromURL fetches a URL and returns clean recipe text.
// It prioritizes Schema.org JSON-LD, then falls back to main content.
func ExtractTextFromURL(url string) (string, error) {
	client := &http.Client{
		Timeout: httpTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; gorecipes/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5 MB cap
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	return parseHTML(string(body))
}

// ExtractTextFromPaste cleans and truncates pasted recipe text.
func ExtractTextFromPaste(text string) string {
	return cleanText(text)
}

func parseHTML(body string) (string, error) {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("parsing HTML: %w", err)
	}

	// Priority 1: Schema.org JSON-LD.
	if text := extractSchemaRecipe(doc); text != "" {
		return cleanText(text), nil
	}

	// Priority 2: Main content selectors.
	stripNonContent(doc)
	if text := extractMainContent(doc); text != "" {
		return cleanText(text), nil
	}

	// Fallback: full body text.
	return cleanText(extractText(doc)), nil
}

// --- Schema.org extraction ---

func extractSchemaRecipe(doc *html.Node) string {
	var result string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if result != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "script" {
			for _, attr := range n.Attr {
				if attr.Key == "type" && attr.Val == "application/ld+json" {
					if text := parseSchemaJSON(extractText(n)); text != "" {
						result = text
						return
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return result
}

func parseSchemaJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Try to decode; handle both objects and arrays.
	var top interface{}
	if err := json.Unmarshal([]byte(raw), &top); err != nil {
		return ""
	}

	recipe := findRecipeInJSON(top)
	if recipe == nil {
		return ""
	}
	return formatSchemaRecipe(recipe)
}

func findRecipeInJSON(v interface{}) map[string]interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		if t, ok := val["@type"]; ok {
			ts := fmt.Sprintf("%v", t)
			if strings.Contains(ts, "Recipe") {
				return val
			}
		}
		for _, child := range val {
			if r := findRecipeInJSON(child); r != nil {
				return r
			}
		}
	case []interface{}:
		for _, item := range val {
			if r := findRecipeInJSON(item); r != nil {
				return r
			}
		}
	}
	return nil
}

func formatSchemaRecipe(r map[string]interface{}) string {
	var parts []string

	schemaFields := []struct{ key, label string }{
		{"name", "Name"},
		{"description", "Description"},
		{"prepTime", "Prep Time"},
		{"cookTime", "Cook Time"},
		{"recipeYield", "Yield"},
	}
	for _, f := range schemaFields {
		if v, ok := r[f.key]; ok && v != nil {
			parts = append(parts, f.label+": "+fmt.Sprintf("%v", v))
		}
	}

	if ingredients, ok := r["recipeIngredient"].([]interface{}); ok {
		parts = append(parts, "Ingredients:")
		for _, ing := range ingredients {
			parts = append(parts, "- "+fmt.Sprintf("%v", ing))
		}
	}

	if instructions, ok := r["recipeInstructions"]; ok {
		parts = append(parts, "Instructions:")
		switch inst := instructions.(type) {
		case []interface{}:
			for i, step := range inst {
				switch s := step.(type) {
				case map[string]interface{}:
					if text, ok := s["text"]; ok {
						parts = append(parts, fmt.Sprintf("%d. %v", i+1, text))
					}
				default:
					parts = append(parts, fmt.Sprintf("%d. %v", i+1, s))
				}
			}
		case string:
			parts = append(parts, inst)
		}
	}

	return strings.Join(parts, "\n")
}

// --- HTML content extraction ---

var stripTags = map[string]bool{
	"script": true, "style": true, "nav": true, "footer": true,
	"header": true, "iframe": true, "noscript": true, "svg": true,
}

var noiseClassRe = regexp.MustCompile(`(?i)\bad(s|vert(isement)?s?)?\b`)

func stripNonContent(doc *html.Node) {
	var toRemove []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if stripTags[n.Data] {
				toRemove = append(toRemove, n)
				return
			}
			// Strip elements with role="navigation" or ad-like class names.
			for _, attr := range n.Attr {
				if (attr.Key == "role" && attr.Val == "navigation") ||
					(attr.Key == "class" && noiseClassRe.MatchString(attr.Val)) {
					toRemove = append(toRemove, n)
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	for _, n := range toRemove {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
	}
}

// mainContentSelectors mirrors the Rails MAIN_CONTENT_SELECTORS.
var mainContentSelectors = []struct {
	tag   string
	attr  string
	value string
}{
	{"article", "", ""},
	{"main", "", ""},
	{"", "role", "main"},
	{"", "class", "recipe"},
	{"", "class", "entry-content"},
	{"", "class", "post-content"},
}

func extractMainContent(doc *html.Node) string {
	var result string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if result != "" {
			return
		}
		if n.Type == html.ElementNode {
			for _, sel := range mainContentSelectors {
				if matchesSelector(n, sel.tag, sel.attr, sel.value) {
					result = extractText(n)
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return result
}

func matchesSelector(n *html.Node, tag, attr, value string) bool {
	if tag != "" && n.Data != tag {
		return false
	}
	if attr == "" {
		return true
	}
	for _, a := range n.Attr {
		if a.Key == attr {
			if value == "" {
				return true
			}
			for _, cls := range strings.Fields(a.Val) {
				if cls == value {
					return true
				}
			}
		}
	}
	return false
}

func extractText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

var multiSpaceRe = regexp.MustCompile(`[ \t]+`)
var multiNewlineRe = regexp.MustCompile(`\n{3,}`)

func cleanText(text string) string {
	text = multiSpaceRe.ReplaceAllString(text, " ")
	text = multiNewlineRe.ReplaceAllString(text, "\n\n")
	text = strings.TrimSpace(text)
	return truncateString(text, maxTextLength)
}

func truncateString(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}
