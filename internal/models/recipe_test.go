package models_test

import (
	"testing"

	"github.com/djcp/gorecipes/internal/models"
)

func ptr[T any](v T) *T { return &v }

func TestRecipe_StatusHelpers(t *testing.T) {
	tests := []struct {
		status     string
		published  bool
		processing bool
		failed     bool
	}{
		{models.StatusPublished, true, false, false},
		{models.StatusProcessing, false, true, false},
		{models.StatusProcessingFailed, false, false, true},
		{models.StatusDraft, false, false, false},
		{models.StatusReview, false, false, false},
		{models.StatusRejected, false, false, false},
	}

	for _, tt := range tests {
		r := &models.Recipe{Status: tt.status}
		if r.IsPublished() != tt.published {
			t.Errorf("status %q: IsPublished() = %v, want %v", tt.status, r.IsPublished(), tt.published)
		}
		if r.IsProcessing() != tt.processing {
			t.Errorf("status %q: IsProcessing() = %v, want %v", tt.status, r.IsProcessing(), tt.processing)
		}
		if r.IsFailed() != tt.failed {
			t.Errorf("status %q: IsFailed() = %v, want %v", tt.status, r.IsFailed(), tt.failed)
		}
	}
}

func TestRecipe_TagsByContext(t *testing.T) {
	r := &models.Recipe{
		Tags: []models.Tag{
			{Name: "bake", Context: models.TagContextCookingMethods},
			{Name: "italian", Context: models.TagContextCulturalInfluences},
			{Name: "dinner", Context: models.TagContextCourses},
			{Name: "roast", Context: models.TagContextCookingMethods},
		},
	}

	methods := r.TagsByContext(models.TagContextCookingMethods)
	if len(methods) != 2 {
		t.Errorf("expected 2 cooking methods, got %d", len(methods))
	}

	influences := r.TagsByContext(models.TagContextCulturalInfluences)
	if len(influences) != 1 || influences[0] != "italian" {
		t.Errorf("expected [italian], got %v", influences)
	}

	dietary := r.TagsByContext(models.TagContextDietaryRestrictions)
	if len(dietary) != 0 {
		t.Errorf("expected empty dietary restrictions, got %v", dietary)
	}
}

func TestRecipe_TimingSummary(t *testing.T) {
	tests := []struct {
		name     string
		prep     *int
		cook     *int
		expected string
	}{
		{"no times", nil, nil, ""},
		{"prep only", ptr(10), nil, "Prep 10m"},
		{"cook only", nil, ptr(30), "Cook 30m"},
		{"both", ptr(15), ptr(45), "Prep 15m  ·  Cook 45m"},
		{"over 1h", ptr(90), nil, "Prep 1h 30m"},
		{"exactly 1h", nil, ptr(60), "Cook 1h"},
		{"zero ignored", ptr(0), ptr(20), "Cook 20m"},
	}

	for _, tt := range tests {
		r := &models.Recipe{PreparationTime: tt.prep, CookingTime: tt.cook}
		got := r.TimingSummary()
		if got != tt.expected {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.expected)
		}
	}
}

func TestRecipeIngredient_DisplayString(t *testing.T) {
	tests := []struct {
		ri   models.RecipeIngredient
		want string
	}{
		{
			models.RecipeIngredient{Quantity: "1", Unit: "cup", IngredientName: "flour", Descriptor: "sifted"},
			"1 cup flour, sifted",
		},
		{
			models.RecipeIngredient{Quantity: "2", Unit: "", IngredientName: "eggs", Descriptor: ""},
			"2 eggs",
		},
		{
			models.RecipeIngredient{Quantity: "to taste", Unit: "", IngredientName: "salt", Descriptor: ""},
			"to taste salt",
		},
		{
			models.RecipeIngredient{Quantity: "1/2", Unit: "lb", IngredientName: "beef", Descriptor: "or turkey"},
			"1/2 lb beef, or turkey",
		},
	}

	for _, tt := range tests {
		got := tt.ri.DisplayString()
		if got != tt.want {
			t.Errorf("DisplayString() = %q, want %q", got, tt.want)
		}
	}
}
