package services_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/djcp/gorecipes/internal/services"
)

func TestExtractTextFromURL_SchemaOrg(t *testing.T) {
	html := `<!DOCTYPE html><html><head>
<script type="application/ld+json">
{
  "@context": "https://schema.org/",
  "@type": "Recipe",
  "name": "Spaghetti Carbonara",
  "description": "A classic Italian pasta dish.",
  "prepTime": "PT10M",
  "cookTime": "PT20M",
  "recipeYield": "4 servings",
  "recipeIngredient": ["200g spaghetti", "100g pancetta", "2 eggs"],
  "recipeInstructions": [
    {"@type": "HowToStep", "text": "Boil the pasta."},
    {"@type": "HowToStep", "text": "Fry the pancetta."},
    {"@type": "HowToStep", "text": "Mix eggs and cheese."}
  ]
}
</script>
</head><body><p>Some blog text</p></body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	text, err := services.ExtractTextFromURL(srv.URL)
	if err != nil {
		t.Fatalf("ExtractTextFromURL() error: %v", err)
	}

	if !strings.Contains(text, "Spaghetti Carbonara") {
		t.Errorf("expected recipe name in output, got:\n%s", text)
	}
	if !strings.Contains(text, "spaghetti") {
		t.Errorf("expected ingredient in output, got:\n%s", text)
	}
	if !strings.Contains(text, "Boil the pasta") {
		t.Errorf("expected instruction in output, got:\n%s", text)
	}
	// Should NOT include "Some blog text" since schema.org takes priority.
	if strings.Contains(text, "Some blog text") {
		t.Error("schema.org extraction should not include blog body text")
	}
}

func TestExtractTextFromURL_FallbackMainContent(t *testing.T) {
	html := `<!DOCTYPE html><html><body>
<nav>Navigation links here</nav>
<header>Site header</header>
<article>
  <h1>Chocolate Cake</h1>
  <p>This is the recipe for chocolate cake.</p>
  <ul><li>2 cups flour</li><li>1 cup sugar</li></ul>
</article>
<footer>Footer content</footer>
<script>alert('ad')</script>
</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	text, err := services.ExtractTextFromURL(srv.URL)
	if err != nil {
		t.Fatalf("ExtractTextFromURL() error: %v", err)
	}

	if !strings.Contains(text, "Chocolate Cake") {
		t.Errorf("expected recipe title in output, got:\n%s", text)
	}
	// Noise should be stripped.
	if strings.Contains(text, "Navigation links") {
		t.Error("nav content should be stripped")
	}
	if strings.Contains(text, "Footer content") {
		t.Error("footer content should be stripped")
	}
	if strings.Contains(text, "alert") {
		t.Error("script content should be stripped")
	}
}

func TestExtractTextFromURL_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := services.ExtractTextFromURL(srv.URL)
	if err == nil {
		t.Error("expected error for 404 response")
	}
}

func TestExtractTextFromURL_NestedSchemaOrg(t *testing.T) {
	// Schema.org recipe nested inside a @graph array.
	html := `<!DOCTYPE html><html><head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@graph": [
    {"@type": "WebSite", "name": "Food Blog"},
    {
      "@type": "Recipe",
      "name": "Banana Bread",
      "recipeIngredient": ["3 bananas", "2 cups flour"],
      "recipeInstructions": [{"@type": "HowToStep", "text": "Mash bananas."}]
    }
  ]
}
</script>
</head><body><p>Irrelevant blog text</p></body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	text, err := services.ExtractTextFromURL(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(text, "Banana Bread") {
		t.Errorf("expected recipe name from nested schema.org, got:\n%s", text)
	}
}

func TestExtractTextFromPaste_CleansTrimsTruncates(t *testing.T) {
	// Lots of whitespace.
	input := "   Recipe Title\n\n\n\n\nIngredients:\n- flour\n- eggs   "
	got := services.ExtractTextFromPaste(input)

	if strings.HasPrefix(got, " ") || strings.HasSuffix(got, " ") {
		t.Errorf("expected trimmed output, got: %q", got[:min(50, len(got))])
	}
	if strings.Contains(got, "\n\n\n") {
		t.Error("expected collapsed newlines")
	}
}

func TestExtractTextFromPaste_TruncatesLongText(t *testing.T) {
	// Build a string longer than 15,000 runes.
	long := strings.Repeat("a", 20_000)
	got := services.ExtractTextFromPaste(long)

	if len([]rune(got)) > 15_000 {
		t.Errorf("expected truncation to 15000 chars, got %d", len([]rune(got)))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
