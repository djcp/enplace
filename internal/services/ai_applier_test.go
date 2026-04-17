package services_test

import (
	"testing"

	"github.com/djcp/enplace/internal/db"
	"github.com/djcp/enplace/internal/models"
	"github.com/djcp/enplace/internal/services"
)

func ptr[T any](v T) *T { return &v }

func TestApplyExtractedRecipe_BasicFields(t *testing.T) {
	d := mustOpenMemory(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{
		Name:   "(importing...)",
		Status: models.StatusProcessing,
	})

	extracted := &services.ExtractedRecipe{
		Name:            "Classic Carbonara",
		Description:     "A rich Italian pasta.",
		Directions:      "1. Boil pasta.\n2. Mix eggs and cheese.",
		PreparationTime: ptr(10),
		CookingTime:     ptr(20),
		Servings:        ptr(4),
		ServingUnits:    ptr("servings"),
		Ingredients:     []services.ExtractedIngredient{},
		CookingMethods:  []string{"boil"},
		Courses:         []string{"dinner"},
	}

	if err := services.ApplyExtractedRecipe(d, recipeID, extracted); err != nil {
		t.Fatalf("ApplyExtractedRecipe() error: %v", err)
	}

	r, err := db.GetRecipe(d, recipeID)
	if err != nil {
		t.Fatal(err)
	}

	if r.Name != "Classic Carbonara" {
		t.Errorf("name: got %q", r.Name)
	}
	if r.Status != models.StatusReview {
		t.Errorf("status: got %q, want review", r.Status)
	}
	if r.PreparationTime == nil || *r.PreparationTime != 10 {
		t.Errorf("prep_time: got %v", r.PreparationTime)
	}
	if r.ServingUnits != "servings" {
		t.Errorf("serving_units: got %q", r.ServingUnits)
	}
}

func TestApplyExtractedRecipe_Ingredients(t *testing.T) {
	d := mustOpenMemory(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{
		Name:   "(importing...)",
		Status: models.StatusProcessing,
	})

	descriptor := "sifted"
	section := "Crust"

	extracted := &services.ExtractedRecipe{
		Name:        "Pie",
		Description: "A pie.",
		Directions:  "1. Make crust.",
		Ingredients: []services.ExtractedIngredient{
			{Name: "flour", Quantity: "2", Unit: "cup", Descriptor: &descriptor, Section: &section},
			{Name: "butter", Quantity: "1/2", Unit: "cup"},
		},
	}

	if err := services.ApplyExtractedRecipe(d, recipeID, extracted); err != nil {
		t.Fatal(err)
	}

	ris, err := db.GetRecipeIngredients(d, recipeID)
	if err != nil {
		t.Fatal(err)
	}

	if len(ris) != 2 {
		t.Fatalf("expected 2 ingredients, got %d", len(ris))
	}

	// Find flour by name (ordering is section, position; empty section sorts before named).
	var flour *models.RecipeIngredient
	for i := range ris {
		if ris[i].IngredientName == "flour" {
			flour = &ris[i]
		}
	}
	if flour == nil {
		t.Fatal("flour ingredient not found")
	}
	if flour.Quantity != "2" {
		t.Errorf("flour quantity: got %q, want %q", flour.Quantity, "2")
	}
	if flour.Descriptor != "sifted" {
		t.Errorf("flour descriptor: got %q, want %q", flour.Descriptor, "sifted")
	}
	if flour.Section != "Crust" {
		t.Errorf("flour section: got %q, want %q", flour.Section, "Crust")
	}
}

func TestApplyExtractedRecipe_ReusesExistingIngredient(t *testing.T) {
	d := mustOpenMemory(t)

	// Create two recipes that share "garlic".
	r1, _ := db.CreateRecipe(d, &models.Recipe{Name: "R1", Status: models.StatusProcessing})
	r2, _ := db.CreateRecipe(d, &models.Recipe{Name: "R2", Status: models.StatusProcessing})

	ing := []services.ExtractedIngredient{{Name: "garlic", Quantity: "3", Unit: "clove"}}

	_ = services.ApplyExtractedRecipe(d, r1, &services.ExtractedRecipe{
		Name: "R1", Description: "d", Directions: "1. Cook.", Ingredients: ing,
	})
	_ = services.ApplyExtractedRecipe(d, r2, &services.ExtractedRecipe{
		Name: "R2", Description: "d", Directions: "1. Cook.", Ingredients: ing,
	})

	ris1, _ := db.GetRecipeIngredients(d, r1)
	ris2, _ := db.GetRecipeIngredients(d, r2)

	if ris1[0].IngredientID != ris2[0].IngredientID {
		t.Error("expected shared ingredient ID for 'garlic', but got different IDs")
	}
}

func TestApplyExtractedRecipe_Tags(t *testing.T) {
	d := mustOpenMemory(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "Test", Status: models.StatusProcessing})

	extracted := &services.ExtractedRecipe{
		Name:                "Test",
		Description:         "d",
		Directions:          "1. Cook.",
		Ingredients:         []services.ExtractedIngredient{},
		CookingMethods:      []string{"bake", "roast"},
		CulturalInfluences:  []string{"italian"},
		Courses:             []string{"dinner"},
		DietaryRestrictions: []string{"vegetarian"},
	}

	if err := services.ApplyExtractedRecipe(d, recipeID, extracted); err != nil {
		t.Fatal(err)
	}

	tags, err := db.GetRecipeTags(d, recipeID)
	if err != nil {
		t.Fatal(err)
	}

	// bake, roast (cooking_methods) + italian (cultural_influences) + dinner (courses) + vegetarian (dietary)
	if len(tags) != 5 {
		t.Errorf("expected 5 tags, got %d: %v", len(tags), tags)
	}

	// Find vegetarian tag.
	var found bool
	for _, tag := range tags {
		if tag.Name == "vegetarian" && tag.Context == models.TagContextDietaryRestrictions {
			found = true
		}
	}
	if !found {
		t.Error("expected 'vegetarian' dietary restriction tag")
	}
}

func TestApplyExtractedRecipe_ReplacesIngredients(t *testing.T) {
	d := mustOpenMemory(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "Test", Status: models.StatusProcessing})

	// First application.
	_ = services.ApplyExtractedRecipe(d, recipeID, &services.ExtractedRecipe{
		Name: "Test", Description: "d", Directions: "1. Cook.",
		Ingredients: []services.ExtractedIngredient{
			{Name: "old ingredient", Quantity: "1", Unit: "cup"},
		},
	})

	// Second application should replace.
	_ = services.ApplyExtractedRecipe(d, recipeID, &services.ExtractedRecipe{
		Name: "Test", Description: "d", Directions: "1. Cook.",
		Ingredients: []services.ExtractedIngredient{
			{Name: "new ingredient", Quantity: "2", Unit: "tbsp"},
		},
	})

	ris, _ := db.GetRecipeIngredients(d, recipeID)
	if len(ris) != 1 {
		t.Errorf("expected 1 ingredient after replace, got %d", len(ris))
	}
	if ris[0].IngredientName != "new ingredient" {
		t.Errorf("expected 'new ingredient', got %q", ris[0].IngredientName)
	}
}

func mustOpenMemory(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}
