package services_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/djcp/gorecipes/internal/db"
	"github.com/djcp/gorecipes/internal/models"
	"github.com/djcp/gorecipes/internal/services"
	"github.com/jmoiron/sqlx"
)

func openPipelineDB(t *testing.T) *sqlx.DB {
	t.Helper()
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

const pipelineRecipeJSON = `{
  "name": "Pipeline Test Recipe",
  "description": "A test recipe.",
  "directions": "1. Do something.",
  "preparation_time": 5,
  "cooking_time": 10,
  "servings": 2,
  "serving_units": "servings",
  "ingredients": [
    {"quantity": "1", "unit": "cup", "name": "water", "descriptor": null, "section": null}
  ],
  "cooking_methods": ["boil"],
  "cultural_influences": [],
  "courses": ["dinner"],
  "dietary_restrictions": []
}`

func TestRunPipeline_URL_Success(t *testing.T) {
	d := openPipelineDB(t)

	// Create draft recipe with source URL.
	recipeID, _ := db.CreateRecipe(d, &models.Recipe{
		Name:       "(importing...)",
		Status:     models.StatusDraft,
		SourceURL:  "https://example.com/recipe",
		SourceText: "",
	})

	// Mock text extractor: inject custom extractor via a test http server.
	// For this unit test, we override the URL fetch by using a paste-based recipe.
	// Test by using source_text instead (URL fetch is tested separately in text_extractor_test.go).
	// Re-create as paste-based to avoid real HTTP.
	_ = db.UpdateRecipeStatus(d, recipeID, models.StatusDraft)
	// Update to use source_text.
	_, err := d.Exec(`UPDATE recipes SET source_url = '', source_text = 'Test recipe text for a pasta dish.' WHERE id = ?`, recipeID)
	if err != nil {
		t.Fatal(err)
	}

	client := &mockAIClient{response: pipelineRecipeJSON}

	var steps []int
	cfg := services.PipelineConfig{
		DB:     d,
		Client: client,
		Model:  "claude-haiku",
		OnStep: func(step int, _ string) {
			steps = append(steps, step)
		},
	}

	recipe, err := services.RunPipeline(context.Background(), cfg, recipeID)
	if err != nil {
		t.Fatalf("RunPipeline() error: %v", err)
	}

	if recipe.Name != "Pipeline Test Recipe" {
		t.Errorf("name: got %q", recipe.Name)
	}
	if recipe.Status != models.StatusPublished {
		t.Errorf("status: got %q, want published", recipe.Status)
	}
	if len(recipe.Ingredients) != 1 {
		t.Errorf("expected 1 ingredient, got %d", len(recipe.Ingredients))
	}

	// Steps 2 (extract) and 3 (save) should have been called.
	if len(steps) < 2 {
		t.Errorf("expected at least 2 progress steps, got %d: %v", len(steps), steps)
	}
}

func TestRunPipeline_AIError_SetsProcessingFailed(t *testing.T) {
	d := openPipelineDB(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{
		Name:       "(importing...)",
		Status:     models.StatusDraft,
		SourceText: "Some recipe text.",
	})

	client := &mockAIClient{err: fmt.Errorf("API unavailable")}

	cfg := services.PipelineConfig{
		DB:     d,
		Client: client,
		Model:  "claude-haiku",
	}

	_, err := services.RunPipeline(context.Background(), cfg, recipeID)
	if err == nil {
		t.Error("expected error from pipeline, got nil")
	}

	recipe, _ := db.GetRecipe(d, recipeID)
	if recipe.Status != models.StatusProcessingFailed {
		t.Errorf("expected processing_failed status, got %q", recipe.Status)
	}
}

func TestRunPipeline_MissingRecipe_Errors(t *testing.T) {
	d := openPipelineDB(t)

	cfg := services.PipelineConfig{
		DB:     d,
		Client: &mockAIClient{response: pipelineRecipeJSON},
		Model:  "claude-haiku",
	}

	_, err := services.RunPipeline(context.Background(), cfg, 99999)
	if err == nil {
		t.Error("expected error for missing recipe, got nil")
	}
}
