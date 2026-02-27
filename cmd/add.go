package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/djcp/gorecipes/internal/db"
	"github.com/djcp/gorecipes/internal/models"
	"github.com/djcp/gorecipes/internal/services"
	"github.com/djcp/gorecipes/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/net/html"
	"golang.org/x/term"
)

var pasteFlag bool

var addCmd = &cobra.Command{
	Use:   "add [url]",
	Short: "Add a recipe from a URL or pasted text",
	Long: `Add a recipe using AI extraction.

Provide a URL as an argument, or use --paste to enter text directly:

  gorecipes add https://example.com/recipe
  gorecipes add --paste`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAdd,
}

func init() {
	addCmd.Flags().BoolVarP(&pasteFlag, "paste", "p", false, "Paste recipe text instead of providing a URL")
}

func runAdd(cmd *cobra.Command, args []string) error {
	var sourceURL, sourceText string

	if pasteFlag {
		var pasted string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewText().
					Title("Paste Recipe Text").
					Description("Paste the full recipe text below. Press Ctrl+D or Esc when done.").
					Value(&pasted).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("recipe text cannot be empty")
						}
						return nil
					}),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}
		sourceText = strings.TrimSpace(pasted)
	} else if len(args) == 1 {
		sourceURL = strings.TrimSpace(args[0])
		if !strings.HasPrefix(sourceURL, "http://") && !strings.HasPrefix(sourceURL, "https://") {
			return fmt.Errorf("URL must start with http:// or https://")
		}
	} else {
		// Prompt for URL interactively.
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Recipe URL").
					Description("Enter the URL of the recipe page").
					Value(&sourceURL).
					Validate(func(s string) error {
						s = strings.TrimSpace(s)
						if s == "" {
							return fmt.Errorf("URL is required")
						}
						if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
							return fmt.Errorf("must be a valid http:// or https:// URL")
						}
						return nil
					}),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}
		sourceURL = strings.TrimSpace(sourceURL)
	}

	// Create a draft recipe.
	draft := &models.Recipe{
		Status:     models.StatusDraft,
		SourceURL:  sourceURL,
		SourceText: sourceText,
		Name:       "(importing...)",
	}
	recipeID, err := db.CreateRecipe(sqlDB, draft)
	if err != nil {
		return fmt.Errorf("creating recipe: %w", err)
	}

	// Set up channels for the pipeline <-> progress UI communication.
	stepCh := make(chan ui.StepUpdate, 8)
	doneCh := make(chan error, 1)

	stepLabels := []string{
		services.StepLabels[services.StepFetch],
		services.StepLabels[services.StepExtract],
		services.StepLabels[services.StepSave],
	}
	// When pasting, step 1 (fetch) is skipped — relabel it.
	if sourceText != "" {
		stepLabels[0] = "Preparing text"
	}

	client := services.NewAnthropicClient(cfg.AnthropicAPIKey)

	// Run the pipeline in the background so the progress UI can render.
	go func() {
		var finalErr error
		pipelineCfg := services.PipelineConfig{
			DB:     sqlDB,
			Client: client,
			Model:  cfg.AnthropicModel,
			OnStep: func(step int, label string) {
				stepCh <- ui.StepUpdate{Step: step, Label: label}
			},
		}
		_, finalErr = services.RunPipeline(context.Background(), pipelineCfg, recipeID)
		close(stepCh)
		doneCh <- finalErr
	}()

	// Trigger step 1 immediately so the UI doesn't sit blank.
	go func() {
		stepCh <- ui.StepUpdate{Step: services.StepFetch, Label: stepLabels[0]}
	}()

	if err := ui.RunProgressUI(stepLabels, stepCh, doneCh); err != nil {
		return err
	}

	// Check if pipeline succeeded.
	recipe, err := db.GetRecipe(sqlDB, recipeID)
	if err != nil {
		return err
	}
	if recipe.IsFailed() {
		fmt.Fprintln(os.Stderr, ui.ErrorStyle.Render("\n✗ Recipe extraction failed. Run `gorecipes show "+fmt.Sprint(recipeID)+"` for details."))
		os.Exit(1)
	}

	// Print the result.
	fmt.Print(ui.RenderRecipeDetail(recipe, termWidth()))
	fmt.Println(ui.MutedStyle.Render(fmt.Sprintf("  ID: %d", recipe.ID)))

	return nil
}

// termWidth returns a best-effort terminal width.
func termWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w == 0 {
		return 80
	}
	return w
}

// isURL is used internally for validation.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// Ensure html import is used (for future HTML validation helpers).
var _ = html.EscapeString
