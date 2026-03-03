package cmd

import (
	"context"
	"fmt"

	"github.com/djcp/gorecipes/internal/config"
	"github.com/djcp/gorecipes/internal/db"
	"github.com/djcp/gorecipes/internal/models"
	"github.com/djcp/gorecipes/internal/services"
	"github.com/djcp/gorecipes/internal/ui"
)

// runConfigUI opens the interactive config editor and saves changes if the user confirms.
func runConfigUI() error {
	configPath, _ := config.FilePath()
	saved, err := ui.RunConfigUI(cfg, configPath)
	if err != nil {
		return err
	}
	if saved {
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
	}
	return nil
}

// runManageUI runs the manage landing page and dispatches to sub-sections in a loop.
func runManageUI() error {
	for {
		section, err := ui.RunManageUI()
		if err != nil {
			return err
		}
		switch section {
		case ui.ManageSectionConfig:
			if err := runConfigUI(); err != nil {
				return err
			}
		case ui.ManageSectionTags:
			if err := ui.RunManageTagsUI(sqlDB); err != nil {
				return err
			}
		case ui.ManageSectionIngredients:
			if err := ui.RunManageIngredientsUI(sqlDB); err != nil {
				return err
			}
		case ui.ManageSectionUnits:
			if err := ui.RunManageUnitsUI(sqlDB); err != nil {
				return err
			}
		case ui.ManageSectionAIRuns:
			for {
				retryRecipeID, err := ui.RunManageAIRunsUI(sqlDB)
				if err != nil {
					return err
				}
				if retryRecipeID == 0 {
					break
				}
				_ = runRetryPipeline(retryRecipeID)
			}
		default: // ManageSectionBack
			return nil
		}
	}
}

func loadEditData() (ui.EditData, error) {
	ingNames, err := db.AllIngredientNames(sqlDB)
	if err != nil {
		return ui.EditData{}, err
	}
	units, err := db.AllUnits(sqlDB)
	if err != nil {
		return ui.EditData{}, err
	}
	tags := make(map[string][]string)
	for _, ctx := range models.AllTagContexts {
		names, err := db.AllTagsByContext(sqlDB, ctx)
		if err != nil {
			return ui.EditData{}, err
		}
		tags[ctx] = names
	}
	return ui.EditData{TagsByContext: tags, IngredientNames: ingNames, Units: units}, nil
}

func loadSearchData() (ui.SearchData, error) {
	courses, err := db.AllTagsByContext(sqlDB, models.TagContextCourses)
	if err != nil {
		return ui.SearchData{}, err
	}
	influences, err := db.AllTagsByContext(sqlDB, models.TagContextCulturalInfluences)
	if err != nil {
		return ui.SearchData{}, err
	}
	return ui.SearchData{Courses: courses, Influences: influences}, nil
}

// runRetryPipeline re-runs AI extraction for an existing recipe via the progress TUI.
// Any pipeline error is shown in the TUI; the returned error covers only setup failures.
func runRetryPipeline(recipeID int64) error {
	recipe, err := db.GetRecipe(sqlDB, recipeID)
	if err != nil {
		return err
	}
	pasteMode := recipe.SourceURL == ""
	launchFn := ui.PipelineLaunchFn(func(ctx context.Context, _, _ string, onStep func(int, string)) (int64, error) {
		pipelineCfg := services.PipelineConfig{
			DB:     sqlDB,
			Client: services.NewAnthropicClient(cfg.AnthropicAPIKey),
			Model:  cfg.AnthropicModel,
			OnStep: onStep,
		}
		_, err := services.RunPipeline(ctx, pipelineCfg, recipeID)
		return recipeID, err
	})
	return ui.RunRetryUI(pasteMode, launchFn)
}
