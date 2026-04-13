//go:build integration

package db_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/djcp/enplace/internal/config"
	"github.com/djcp/enplace/internal/db"
	"github.com/djcp/enplace/internal/models"
	"github.com/pressly/goose/v3"
)

func openTestPostgresDB(t *testing.T) *db.DB {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	cfg := &config.Config{PostgresDSN: dsn}
	d, err := db.Open(cfg, goose.NopLogger())
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestMigrateToPostgres(t *testing.T) {
	sqlite, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlite.Close()

	pg := openTestPostgresDB(t)

	// Seed SQLite with a recipe.
	recipeID, err := db.CreateRecipe(sqlite, &models.Recipe{
		Name:      "Test Bread",
		Status:    models.StatusPublished,
		SourceURL: "https://example.com/bread",
		IsBread:   true,
	})
	if err != nil {
		t.Fatalf("create recipe: %v", err)
	}

	// Add an ingredient.
	ingID, err := db.FindOrCreateIngredient(sqlite, "flour")
	if err != nil {
		t.Fatalf("find or create ingredient: %v", err)
	}
	if err := db.InsertRecipeIngredient(sqlite, &models.RecipeIngredient{
		RecipeID:     recipeID,
		IngredientID: ingID,
		Quantity:     "500",
		Unit:         "g",
	}); err != nil {
		t.Fatalf("insert ingredient: %v", err)
	}

	// Add a tag.
	tagID, err := db.FindOrCreateTag(sqlite, "bread", models.TagContextCourses)
	if err != nil {
		t.Fatalf("find or create tag: %v", err)
	}
	if err := db.AttachTag(sqlite, recipeID, tagID); err != nil {
		t.Fatalf("attach tag: %v", err)
	}

	// Add an AI run.
	runID, err := db.CreateAIRun(sqlite, &models.AIClassifierRun{
		RecipeID:     &recipeID,
		ServiceClass: "AIExtractor",
		Adapter:      "anthropic",
	})
	if err != nil {
		t.Fatalf("create ai run: %v", err)
	}
	if err := db.CompleteAIRun(sqlite, runID, `{"name":"Test Bread"}`); err != nil {
		t.Fatalf("complete ai run: %v", err)
	}

	// Run migration.
	result, err := db.MigrateToPostgres(sqlite, pg, slog.Default())
	if err != nil {
		t.Fatalf("migration: %v", err)
	}
	if result.Imported != 1 {
		t.Errorf("imported: got %d, want 1", result.Imported)
	}
	if result.Skipped != 0 {
		t.Errorf("skipped: got %d, want 0", result.Skipped)
	}

	// Verify recipe exists in postgres.
	r, err := db.GetRecipeByURL(pg, "https://example.com/bread")
	if err != nil {
		t.Fatalf("get recipe by url: %v", err)
	}
	if r == nil {
		t.Fatal("recipe not found in postgres")
	}
	if r.Name != "Test Bread" {
		t.Errorf("name: got %q, want %q", r.Name, "Test Bread")
	}

	// Verify ingredients.
	ings, err := db.GetRecipeIngredients(pg, r.ID)
	if err != nil {
		t.Fatalf("get ingredients: %v", err)
	}
	if len(ings) != 1 || ings[0].IngredientName != "flour" {
		t.Errorf("ingredients: got %v", ings)
	}

	// Verify tags.
	tags, err := db.GetRecipeTags(pg, r.ID)
	if err != nil {
		t.Fatalf("get tags: %v", err)
	}
	if len(tags) != 1 || tags[0].Name != "bread" {
		t.Errorf("tags: got %v", tags)
	}

	// Verify AI run copied.
	runs, err := db.ListAIRunSummaries(pg)
	if err != nil {
		t.Fatalf("list ai runs: %v", err)
	}
	if len(runs) == 0 {
		t.Error("expected at least one ai run in postgres")
	}

	// Test idempotency — re-running skips the already-imported recipe.
	result2, err := db.MigrateToPostgres(sqlite, pg, slog.Default())
	if err != nil {
		t.Fatalf("second migration: %v", err)
	}
	if result2.Skipped != 1 {
		t.Errorf("second run: want 1 skipped, got %d", result2.Skipped)
	}

	// Test ClearSQLiteData.
	if err := db.ClearSQLiteData(sqlite, slog.Default()); err != nil {
		t.Fatalf("clear: %v", err)
	}
	count, err := db.RecipeCount(sqlite)
	if err != nil {
		t.Fatalf("recipe count after clear: %v", err)
	}
	if count != 0 {
		t.Errorf("after clear, sqlite count = %d, want 0", count)
	}
}
