package services_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/djcp/gorecipes/internal/services"
)

// mockAIClient implements services.AIClient for testing.
type mockAIClient struct {
	response string
	err      error
}

func (m *mockAIClient) Complete(_ context.Context, _, _, _ string) (string, error) {
	return m.response, m.err
}

const validRecipeJSON = `{
  "name": "Simple Pasta",
  "description": "A quick weeknight pasta.",
  "directions": "1. Boil water.\n2. Cook pasta.\n3. Add sauce.",
  "preparation_time": 5,
  "cooking_time": 15,
  "servings": 4,
  "serving_units": "servings",
  "ingredients": [
    {"quantity": "200", "unit": "g", "name": "spaghetti", "descriptor": null, "section": null},
    {"quantity": "1", "unit": "cup", "name": "tomato sauce", "descriptor": "jarred", "section": null}
  ],
  "cooking_methods": ["boil"],
  "cultural_influences": ["italian"],
  "courses": ["dinner"],
  "dietary_restrictions": ["vegetarian"]
}`

func TestExtractRecipe_ValidResponse(t *testing.T) {
	client := &mockAIClient{response: validRecipeJSON}

	recipe, raw, err := services.ExtractRecipe(context.Background(), client, "claude-haiku", "some text")
	if err != nil {
		t.Fatalf("ExtractRecipe() error: %v", err)
	}
	if raw == "" {
		t.Error("expected non-empty raw response")
	}

	if recipe.Name != "Simple Pasta" {
		t.Errorf("name: got %q, want %q", recipe.Name, "Simple Pasta")
	}
	if recipe.CookingTime == nil || *recipe.CookingTime != 15 {
		t.Errorf("cooking_time: got %v, want 15", recipe.CookingTime)
	}
	if len(recipe.Ingredients) != 2 {
		t.Errorf("expected 2 ingredients, got %d", len(recipe.Ingredients))
	}
	if len(recipe.Courses) != 1 || recipe.Courses[0] != "dinner" {
		t.Errorf("courses: got %v, want [dinner]", recipe.Courses)
	}
}

func TestExtractRecipe_StripsMarkdownFences(t *testing.T) {
	wrapped := "```json\n" + validRecipeJSON + "\n```"
	client := &mockAIClient{response: wrapped}

	recipe, _, err := services.ExtractRecipe(context.Background(), client, "claude-haiku", "text")
	if err != nil {
		t.Fatalf("ExtractRecipe() with fenced JSON error: %v", err)
	}
	if recipe.Name != "Simple Pasta" {
		t.Errorf("name: got %q, want %q", recipe.Name, "Simple Pasta")
	}
}

func TestExtractRecipe_NullOptionalFields(t *testing.T) {
	json := `{
  "name": "Mystery Dish",
  "description": "Unknown.",
  "directions": "1. Do something.",
  "preparation_time": null,
  "cooking_time": null,
  "servings": null,
  "serving_units": null,
  "ingredients": [],
  "cooking_methods": [],
  "cultural_influences": [],
  "courses": [],
  "dietary_restrictions": []
}`
	client := &mockAIClient{response: json}

	recipe, _, err := services.ExtractRecipe(context.Background(), client, "claude-haiku", "text")
	if err != nil {
		t.Fatalf("ExtractRecipe() error: %v", err)
	}
	if recipe.PreparationTime != nil {
		t.Errorf("expected nil preparation_time, got %v", recipe.PreparationTime)
	}
	if recipe.ServingUnits != nil {
		t.Errorf("expected nil serving_units, got %v", recipe.ServingUnits)
	}
}

func TestExtractRecipe_APIError(t *testing.T) {
	client := &mockAIClient{err: fmt.Errorf("rate limit exceeded")}

	_, _, err := services.ExtractRecipe(context.Background(), client, "claude-haiku", "text")
	if err == nil {
		t.Error("expected error from API failure, got nil")
	}
}

func TestExtractRecipe_MalformedJSON(t *testing.T) {
	client := &mockAIClient{response: `{not valid json}`}

	_, _, err := services.ExtractRecipe(context.Background(), client, "claude-haiku", "text")
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

func TestExtractRecipe_DescriptorAndSection(t *testing.T) {
	json := `{
  "name": "Layered Pie",
  "description": "A layered dessert.",
  "directions": "1. Make crust.\n2. Add filling.",
  "preparation_time": null,
  "cooking_time": null,
  "servings": 8,
  "serving_units": "slices",
  "ingredients": [
    {"quantity": "2", "unit": "cup", "name": "flour", "descriptor": "sifted", "section": "Crust"},
    {"quantity": "1", "unit": "cup", "name": "sugar", "descriptor": null, "section": "Filling"}
  ],
  "cooking_methods": ["bake"],
  "cultural_influences": [],
  "courses": ["dessert"],
  "dietary_restrictions": []
}`
	client := &mockAIClient{response: json}

	recipe, _, err := services.ExtractRecipe(context.Background(), client, "claude-haiku", "text")
	if err != nil {
		t.Fatal(err)
	}

	if len(recipe.Ingredients) != 2 {
		t.Fatalf("expected 2 ingredients, got %d", len(recipe.Ingredients))
	}

	flour := recipe.Ingredients[0]
	if flour.Name != "flour" {
		t.Errorf("ingredient 0 name: got %q", flour.Name)
	}
	if flour.Descriptor == nil || *flour.Descriptor != "sifted" {
		t.Errorf("ingredient 0 descriptor: got %v", flour.Descriptor)
	}
	if flour.Section == nil || *flour.Section != "Crust" {
		t.Errorf("ingredient 0 section: got %v", flour.Section)
	}

	sugar := recipe.Ingredients[1]
	if sugar.Section == nil || *sugar.Section != "Filling" {
		t.Errorf("ingredient 1 section: got %v", sugar.Section)
	}
}
