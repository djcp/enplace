package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/djcp/gorecipes/internal/db"
	"github.com/djcp/gorecipes/internal/export"
	"github.com/djcp/gorecipes/internal/models"
	"github.com/djcp/gorecipes/internal/services"
	"github.com/djcp/gorecipes/internal/ui"
	"github.com/spf13/cobra"
)

var (
	pasteFlag bool
	quietFlag bool
)

var addCmd = &cobra.Command{
	Use:   "add [url]",
	Short: "Add a recipe from a URL or pasted text",
	Long: `Add a recipe using AI extraction.

Provide a URL as an argument, or use --paste to enter text directly:

  gorecipes add https://example.com/recipe
  gorecipes add --paste

Use --quiet to extract and save without launching the UI (exits 0 on success,
non-zero with an error message on stderr on failure — useful for scripting):

  gorecipes add --quiet https://example.com/recipe`,
	Args:         cobra.MaximumNArgs(1),
	RunE:         runAdd,
	SilenceUsage: true, // don't dump usage text to stderr on error
}

func init() {
	addCmd.Flags().BoolVarP(&pasteFlag, "paste", "p", false, "Paste recipe text instead of providing a URL")
	addCmd.Flags().BoolVarP(&quietFlag, "quiet", "q", false, "Extract and save without launching the UI (for scripting)")
}

func runAdd(_ *cobra.Command, args []string) error {
	// If a URL was provided as a CLI argument, validate it and skip the input UI.
	initialURL := ""
	if len(args) == 1 {
		initialURL = strings.TrimSpace(args[0])
		if !isURL(initialURL) {
			return fmt.Errorf("URL must start with http:// or https://")
		}
	}

	if quietFlag {
		if initialURL == "" {
			return fmt.Errorf("--quiet requires a URL argument")
		}
		if pasteFlag {
			return fmt.Errorf("--quiet and --paste cannot be used together")
		}
		return runAddQuiet(initialURL)
	}

	// launchFn creates the draft recipe and runs the extraction pipeline.
	launchFn := ui.PipelineLaunchFn(func(ctx context.Context, sourceURL, sourceText string, onStep func(int, string)) (int64, error) {
		draft := &models.Recipe{
			Status:     models.StatusDraft,
			SourceURL:  sourceURL,
			SourceText: sourceText,
			Name:       "(importing...)",
		}
		recipeID, err := db.CreateRecipe(sqlDB, draft)
		if err != nil {
			return 0, fmt.Errorf("creating recipe: %w", err)
		}
		pipelineCfg := services.PipelineConfig{
			DB:     sqlDB,
			Client: services.NewAnthropicClient(cfg.AnthropicAPIKey),
			Model:  cfg.AnthropicModel,
			OnStep: onStep,
		}
		_, err = services.RunPipeline(ctx, pipelineCfg, recipeID)
		return recipeID, err
	})

	recipeID, goHome, goAdd, goManual, pipeErr := ui.RunAddUI(pasteFlag, initialURL, launchFn)

	if goManual {
		editData, err := loadEditData()
		if err != nil {
			return err
		}
		toSave, tagNames, goHome2, err := ui.RunEditUI(nil, editData)
		if err != nil {
			return err
		}
		if goHome2 {
			return runList(nil, nil)
		}
		if toSave != nil {
			if err := db.SaveRecipe(sqlDB, toSave, tagNames); err != nil {
				return fmt.Errorf("saving recipe: %w", err)
			}
			recipe, err := db.GetRecipe(sqlDB, toSave.ID)
			if err != nil {
				return err
			}
			return runDetailLoop(recipe)
		}
		return nil
	}

	if goAdd {
		return runAdd(nil, nil)
	}
	if goHome {
		return runList(nil, nil)
	}
	// pipeErr was already shown in the TUI; return nil so cobra doesn't double-print.
	if pipeErr != nil || recipeID <= 0 {
		return nil
	}

	// Pipeline succeeded — open the recipe in the interactive detail view.
	recipe, err := db.GetRecipe(sqlDB, recipeID)
	if err != nil {
		return err
	}
	return runDetailLoop(recipe)
}

// runAddQuiet runs the extraction pipeline without any TUI.
// On success it returns nil (exit 0). On failure it returns the error (exit 1,
// cobra writes the message to stderr), and removes the orphaned draft record.
func runAddQuiet(sourceURL string) error {
	draft := &models.Recipe{
		Status:    models.StatusDraft,
		SourceURL: sourceURL,
		Name:      "(importing...)",
	}
	recipeID, err := db.CreateRecipe(sqlDB, draft)
	if err != nil {
		return fmt.Errorf("creating recipe: %w", err)
	}

	pipelineCfg := services.PipelineConfig{
		DB:     sqlDB,
		Client: services.NewAnthropicClient(cfg.AnthropicAPIKey),
		Model:  cfg.AnthropicModel,
		OnStep: func(int, string) {}, // silence progress callbacks
	}
	if _, err := services.RunPipeline(context.Background(), pipelineCfg, recipeID); err != nil {
		_ = db.DeleteRecipe(sqlDB, recipeID) // clean up the failed draft
		return err
	}
	return nil
}

// runDetailLoop opens the detail view for a recipe and handles navigation signals.
func runDetailLoop(recipe *models.Recipe) error {
	sd, _ := loadSearchData()
	for {
		goHome, goAdd, goEdit, goPrint, goManage, goRetry, deleteConfirmed, returnFilter, err := ui.RunDetailUI(recipe, ui.FilterState{}, sd)
		if err != nil {
			return err
		}
		if deleteConfirmed {
			if err := db.DeleteRecipe(sqlDB, recipe.ID); err != nil {
				return fmt.Errorf("deleting recipe: %w", err)
			}
			return runList(nil, nil)
		}
		if goRetry {
			_ = runRetryPipeline(recipe.ID)
			recipe, err = db.GetRecipe(sqlDB, recipe.ID)
			if err != nil {
				return err
			}
			continue
		}
		if goManage {
			if err := runManageUI(); err != nil {
				return err
			}
			continue
		}
		if goPrint {
			quit, err := ui.RunPrintUI(recipe, export.Options{Credits: cfg.Credits})
			if err != nil {
				return err
			}
			if quit {
				return nil
			}
			continue
		}
		if goEdit {
			editData, err := loadEditData()
			if err != nil {
				return err
			}
			toSave, tagNames, _, err := ui.RunEditUI(recipe, editData)
			if err != nil {
				return err
			}
			if toSave != nil {
				if err := db.SaveRecipe(sqlDB, toSave, tagNames); err != nil {
					return fmt.Errorf("saving recipe: %w", err)
				}
				recipe, err = db.GetRecipe(sqlDB, recipe.ID)
				if err != nil {
					return err
				}
			}
			continue
		}
		if goAdd {
			return runAdd(nil, nil)
		}
		if goHome {
			listQuery = returnFilter.Query
			return runList(nil, nil)
		}
		return nil
	}
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
