package services

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/djcp/gorecipes/internal/db"
	"github.com/djcp/gorecipes/internal/models"
	"github.com/jmoiron/sqlx"
)

// Step constants for progress reporting.
const (
	StepFetch   = 1
	StepExtract = 2
	StepSave    = 3
	StepDone    = 4
)

// StepLabels maps step numbers to display labels.
var StepLabels = map[int]string{
	StepFetch:   "Fetching recipe content",
	StepExtract: "Extracting with AI",
	StepSave:    "Saving to database",
}

// ProgressFunc is called at the start of each pipeline step.
type ProgressFunc func(step int, label string)

// PipelineConfig holds dependencies for the extraction pipeline.
type PipelineConfig struct {
	DB       *sqlx.DB
	Client   AIClient
	Model    string
	OnStep   ProgressFunc // optional; called at start of each step
}

// RunPipeline orchestrates the three-step recipe extraction pipeline:
// 1. Fetch & parse content (URL or paste)
// 2. Extract structured data via Claude
// 3. Write data to database
//
// It records an AIClassifierRun for steps 1 and 2, and updates recipe
// status throughout. Returns the populated recipe on success.
func RunPipeline(ctx context.Context, cfg PipelineConfig, recipeID int64) (*models.Recipe, error) {
	progress := func(step int, label string) {
		if cfg.OnStep != nil {
			cfg.OnStep(step, label)
		}
	}

	// Load the draft recipe.
	recipe, err := db.GetRecipe(cfg.DB, recipeID)
	if err != nil {
		return nil, fmt.Errorf("loading recipe: %w", err)
	}

	// Transition to processing.
	if err := db.UpdateRecipeStatus(cfg.DB, recipeID, models.StatusProcessing); err != nil {
		return nil, err
	}

	var text string

	// Step 1: Fetch content (skip if we already have source_text).
	if recipe.SourceURL != "" {
		progress(StepFetch, StepLabels[StepFetch])
		runID, runErr := recordRunStart(cfg.DB, recipeID, "TextExtractor", cfg.Model, "", recipe.SourceURL)

		text, err = ExtractTextFromURL(recipe.SourceURL)
		if runErr == nil {
			if err != nil {
				_ = db.FailAIRun(cfg.DB, runID, reflect.TypeOf(err).String(), err.Error())
			} else {
				_ = db.CompleteAIRun(cfg.DB, runID, text)
			}
		}
		if err != nil {
			_ = db.UpdateRecipeStatus(cfg.DB, recipeID, models.StatusProcessingFailed)
			return nil, fmt.Errorf("fetching content: %w", err)
		}
	} else {
		text = ExtractTextFromPaste(recipe.SourceText)
		// No AIClassifierRun for paste — no network/AI call needed here.
	}

	// Step 2: AI extraction.
	progress(StepExtract, StepLabels[StepExtract])
	userMessage := "Extract the recipe from this text:\n\n" + text
	runID, runErr := recordRunStart(cfg.DB, recipeID, "AIExtractor", cfg.Model, systemPrompt, userMessage)

	extracted, rawResponse, err := ExtractRecipe(ctx, cfg.Client, cfg.Model, text)
	if runErr == nil {
		if err != nil {
			_ = db.FailAIRun(cfg.DB, runID, reflect.TypeOf(err).String(), err.Error())
		} else {
			_ = db.CompleteAIRun(cfg.DB, runID, rawResponse)
		}
	}
	if err != nil {
		_ = db.UpdateRecipeStatus(cfg.DB, recipeID, models.StatusProcessingFailed)
		return nil, fmt.Errorf("AI extraction: %w", err)
	}

	// Step 3: Apply to database.
	progress(StepSave, StepLabels[StepSave])
	if err := ApplyExtractedRecipe(cfg.DB, recipeID, extracted); err != nil {
		_ = db.UpdateRecipeStatus(cfg.DB, recipeID, models.StatusProcessingFailed)
		return nil, fmt.Errorf("saving recipe: %w", err)
	}

	// Publish immediately (no admin review step in the CLI).
	if err := db.UpdateRecipeStatus(cfg.DB, recipeID, models.StatusPublished); err != nil {
		return nil, err
	}

	progress(StepDone, "Done")
	return db.GetRecipe(cfg.DB, recipeID)
}

func recordRunStart(sqlDB *sqlx.DB, recipeID int64, serviceClass, model, sysprompt, userPrompt string) (int64, error) {
	now := time.Now()
	run := &models.AIClassifierRun{
		RecipeID:     &recipeID,
		ServiceClass: serviceClass,
		Adapter:      "anthropic",
		AIModel:      model,
		SystemPrompt: sysprompt,
		UserPrompt:   userPrompt,
		StartedAt:    &now,
	}
	return db.CreateAIRun(sqlDB, run)
}
