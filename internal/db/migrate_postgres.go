package db

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/djcp/enplace/internal/models"
)

// MigrationResult holds counts from a completed migration.
type MigrationResult struct {
	Imported int
	Skipped  int
}

// MigrateToPostgres copies recipes from sqlite to pg using the same
// find-or-create paths as normal recipe saves. Recipes with a source_url
// that already exists in pg are skipped to avoid duplicates. Manual recipes
// (no source_url) are always imported.
//
// AI classifier runs are copied with remapped recipe IDs.
// The sqlite DB is NOT modified by this function — call ClearSQLiteData after
// confirming success.
func MigrateToPostgres(sqlite *DB, pg *DB, logger *slog.Logger) (MigrationResult, error) {
	var result MigrationResult

	logger.Info("migration started")

	summaries, err := ListRecipes(sqlite, RecipeFilter{})
	if err != nil {
		return result, fmt.Errorf("listing sqlite recipes: %w", err)
	}
	logger.Info("migration check", "sqlite_recipe_count", len(summaries))

	for _, summary := range summaries {
		r, err := GetRecipe(sqlite, summary.ID)
		if err != nil {
			return result, fmt.Errorf("reading recipe %d: %w", summary.ID, err)
		}

		// Deduplicate by URL.
		if r.SourceURL != "" {
			existing, err := GetRecipeByURL(pg, r.SourceURL)
			if err != nil {
				return result, fmt.Errorf("checking duplicate url for recipe %d: %w", r.ID, err)
			}
			if existing != nil {
				logger.Info("migration skipped duplicate", "recipe", r.Name, "url", r.SourceURL)
				result.Skipped++
				continue
			}
		}

		oldID := r.ID

		// Build tag names map from loaded tags.
		tagNames := make(map[string][]string)
		for _, t := range r.Tags {
			tagNames[t.Context] = append(tagNames[t.Context], t.Name)
		}

		// Force insert (ID=0 → CreateRecipe path in SaveRecipe).
		r.ID = 0
		if err := SaveRecipe(pg, r, tagNames); err != nil {
			return result, fmt.Errorf("saving recipe %q: %w", r.Name, err)
		}
		newID := r.ID

		logger.Info("migration imported recipe", "name", r.Name, "old_id", oldID, "new_id", newID)

		// Copy AI runs.
		if err := copyAIRuns(sqlite, pg, oldID, newID, logger); err != nil {
			return result, fmt.Errorf("copying ai runs for recipe %d: %w", oldID, err)
		}

		result.Imported++
	}

	logger.Info("migration complete", "imported", result.Imported, "skipped", result.Skipped)
	return result, nil
}

// copyAIRuns copies all ai_classifier_runs for oldRecipeID in sqlite to pg
// using newRecipeID as the foreign key.
func copyAIRuns(sqlite *DB, pg *DB, oldRecipeID, newRecipeID int64, logger *slog.Logger) error {
	var runs []models.AIClassifierRun
	err := sqlite.Select(&runs,
		`SELECT * FROM ai_classifier_runs WHERE recipe_id = ? ORDER BY created_at`,
		oldRecipeID,
	)
	if err != nil {
		return err
	}
	for _, run := range runs {
		run.RecipeID = &newRecipeID
		_, err := pg.insertReturningID(`
			INSERT INTO ai_classifier_runs
			  (recipe_id, service_class, adapter, ai_model,
			   system_prompt, user_prompt, raw_response,
			   success, error_class, error_message,
			   started_at, completed_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			run.RecipeID, run.ServiceClass, run.Adapter, run.AIModel,
			run.SystemPrompt, run.UserPrompt, run.RawResponse,
			run.Success, run.ErrorClass, run.ErrorMessage,
			run.StartedAt, run.CompletedAt, run.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("inserting ai run: %w", err)
		}
	}
	logger.Info("migration copied ai runs", "recipe_old_id", oldRecipeID, "count", len(runs))
	return nil
}

// ClearSQLiteData deletes all recipe data from the sqlite DB in reverse
// foreign-key order. The schema and file are preserved; only data is removed.
// Call this only after MigrateToPostgres has completed successfully.
func ClearSQLiteData(sqlite *DB, logger *slog.Logger) error {
	logger.Info("sqlite cleanup started")

	tables := []string{
		"ai_classifier_runs",
		"recipe_tags",
		"recipe_ingredients",
		"recipes",
		"tags",
		"ingredients",
	}
	for _, table := range tables {
		res, err := sqlite.Exec(fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			return fmt.Errorf("clearing %s: %w", table, err)
		}
		n, _ := res.RowsAffected()
		logger.Info("sqlite cleanup", "table", table, "rows_deleted", n)
	}
	logger.Info("sqlite cleanup complete")
	return nil
}

// BackupSQLite copies the SQLite database file at srcPath to destPath.
func BackupSQLite(srcPath, destPath string, logger *slog.Logger) error {
	logger.Info("backup started", "src", srcPath, "dest", destPath)
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("opening sqlite file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating backup file: %w", err)
	}
	defer dst.Close()

	n, err := io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("copying backup: %w", err)
	}
	logger.Info("backup complete", "dest", destPath, "bytes", n)
	return nil
}
