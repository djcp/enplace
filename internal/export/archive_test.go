package export_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/djcp/enplace/internal/export"
	"github.com/djcp/enplace/internal/models"
)

func archiveRecipes() []*models.Recipe {
	return []*models.Recipe{
		{
			Name:        "Simple Sourdough",
			Description: "A no-knead loaf",
			IsBread:     true,
			Ingredients: []models.RecipeIngredient{
				{IngredientName: "bread flour", IngredientType: "flour", QuantityNumeric: fptr(500), Unit: "g", Quantity: "500"},
				{IngredientName: "water", IngredientType: "wet", QuantityNumeric: fptr(325), Unit: "g", Quantity: "325"},
			},
			Tags: []models.Tag{{Name: "bread", Context: "courses"}},
		},
		{
			Name: "Tomato Pasta",
			Ingredients: []models.RecipeIngredient{
				{IngredientName: "pasta", QuantityNumeric: fptr(200), Unit: "g", Quantity: "200"},
			},
		},
	}
}

func TestWriteArchive_JSON_RoundTrips(t *testing.T) {
	recipes := archiveRecipes()

	var buf bytes.Buffer
	n, err := export.WriteArchive(&buf, recipes, "json")
	if err != nil {
		t.Fatalf("WriteArchive json: %v", err)
	}
	if n != len(recipes) {
		t.Fatalf("WriteArchive returned count %d, want %d", n, len(recipes))
	}

	var arc export.Archive
	if err := json.Unmarshal(buf.Bytes(), &arc); err != nil {
		t.Fatalf("unmarshal archive: %v", err)
	}
	if arc.Schema != export.ArchiveSchema {
		t.Errorf("schema = %d, want %d", arc.Schema, export.ArchiveSchema)
	}
	if arc.Count != len(recipes) || len(arc.Recipes) != len(recipes) {
		t.Fatalf("archive count = %d / %d recipes, want %d", arc.Count, len(arc.Recipes), len(recipes))
	}
	if arc.Recipes[0].Name != "Simple Sourdough" {
		t.Errorf("recipe[0].Name = %q", arc.Recipes[0].Name)
	}
	if got := len(arc.Recipes[0].Ingredients); got != 2 {
		t.Errorf("recipe[0] ingredients = %d, want 2", got)
	}
	if got := arc.Recipes[0].Ingredients[0].IngredientName; got != "bread flour" {
		t.Errorf("recipe[0] ingredient[0] name = %q", got)
	}
	if got := len(arc.Recipes[0].Tags); got != 1 || arc.Recipes[0].Tags[0].Name != "bread" {
		t.Errorf("recipe[0] tags not preserved: %+v", arc.Recipes[0].Tags)
	}
}

func TestWriteArchive_Text_ContainsNames(t *testing.T) {
	recipes := archiveRecipes()

	var buf bytes.Buffer
	n, err := export.WriteArchive(&buf, recipes, "text")
	if err != nil {
		t.Fatalf("WriteArchive text: %v", err)
	}
	if n != len(recipes) {
		t.Fatalf("WriteArchive returned count %d, want %d", n, len(recipes))
	}
	out := buf.String()
	for _, r := range recipes {
		if !strings.Contains(out, r.Name) {
			t.Errorf("text archive missing recipe name %q, got:\n%s", r.Name, out)
		}
	}
}

func TestWriteArchive_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if _, err := export.WriteArchive(&buf, archiveRecipes(), "yaml"); err == nil {
		t.Fatal("expected error for unknown format, got nil")
	}
}
