package export_test

import (
	"strings"
	"testing"

	"github.com/djcp/enplace/internal/export"
	"github.com/djcp/enplace/internal/models"
)

func fptr(f float64) *float64 { return &f }

// breadRecipe returns a minimal bread recipe with enough ingredient data for
// BreadMetrics to succeed (dry + wet ingredients with weight units).
func breadRecipe() *models.Recipe {
	return &models.Recipe{
		Name:    "Simple Sourdough",
		IsBread: true,
		Ingredients: []models.RecipeIngredient{
			{IngredientName: "bread flour", IngredientType: "dry", QuantityNumeric: fptr(500), Unit: "g", Quantity: "500"},
			{IngredientName: "water", IngredientType: "wet", QuantityNumeric: fptr(325), Unit: "g", Quantity: "325"},
			{IngredientName: "salt", IngredientType: "", QuantityNumeric: fptr(10), Unit: "g", Quantity: "10"},
		},
	}
}

func nonBreadRecipe() *models.Recipe {
	return &models.Recipe{
		Name:    "Tomato Pasta",
		IsBread: false,
		Ingredients: []models.RecipeIngredient{
			{IngredientName: "pasta", QuantityNumeric: fptr(200), Unit: "g", Quantity: "200"},
		},
	}
}

// ---- ToText ------------------------------------------------------------------

func TestToText_BreadRecipe_IncludesHydration(t *testing.T) {
	out := export.ToText(breadRecipe(), export.Options{})
	if !strings.Contains(out, "Hydration:") {
		t.Errorf("ToText: expected hydration line for bread recipe, got:\n%s", out)
	}
	if !strings.Contains(out, "65.0%") {
		t.Errorf("ToText: expected 65.0%% hydration (325/500), got:\n%s", out)
	}
}

func TestToText_NonBreadRecipe_NoHydration(t *testing.T) {
	out := export.ToText(nonBreadRecipe(), export.Options{})
	if strings.Contains(out, "Hydration:") {
		t.Errorf("ToText: unexpected hydration line for non-bread recipe, got:\n%s", out)
	}
}

func TestToText_BreadRecipe_StarterNote(t *testing.T) {
	r := breadRecipe()
	r.Ingredients = append(r.Ingredients, models.RecipeIngredient{
		IngredientName: "sourdough starter", IngredientType: "starter",
		QuantityNumeric: fptr(100), Unit: "g", Quantity: "100",
	})
	out := export.ToText(r, export.Options{})
	if !strings.Contains(out, "100% hydration starter assumed") {
		t.Errorf("ToText: expected starter assumption note, got:\n%s", out)
	}
}

// ---- ToMarkdown --------------------------------------------------------------

func TestToMarkdown_BreadRecipe_IncludesHydration(t *testing.T) {
	out := export.ToMarkdown(breadRecipe(), export.Options{})
	if !strings.Contains(out, "Hydration:") {
		t.Errorf("ToMarkdown: expected hydration line for bread recipe, got:\n%s", out)
	}
}

func TestToMarkdown_NonBreadRecipe_NoHydration(t *testing.T) {
	out := export.ToMarkdown(nonBreadRecipe(), export.Options{})
	if strings.Contains(out, "Hydration:") {
		t.Errorf("ToMarkdown: unexpected hydration line for non-bread recipe")
	}
}
