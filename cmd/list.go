package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/djcp/gorecipes/internal/db"
	"github.com/djcp/gorecipes/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	listQuery   string
	listStatus  string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Browse and search recipes",
	Long: `Open the interactive recipe browser.

Use / to search, arrow keys to navigate, enter to view a recipe.`,
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVarP(&listQuery, "query", "q", "", "Filter by name or ingredient (non-interactive)")
	listCmd.Flags().StringVarP(&listStatus, "status", "s", "", "Filter by status (published, review, draft, etc.)")
}

func runList(_ *cobra.Command, _ []string) error {
	filter := db.RecipeFilter{
		Query:        listQuery,
		StatusFilter: listStatus,
	}

	recipes, err := db.ListRecipes(sqlDB, filter)
	if err != nil {
		return fmt.Errorf("loading recipes: %w", err)
	}

	if len(recipes) == 0 {
		fmt.Println(ui.MutedStyle.Render("\n  No recipes found."))
		fmt.Println(ui.MutedStyle.Render("  Add one with: gorecipes add <url>"))
		fmt.Println()
		return nil
	}

	// Non-interactive mode: plain output when not a TTY or when query is set.
	if listQuery != "" || !isTerminal() {
		fmt.Printf("\n  Found %d recipe(s):\n\n", len(recipes))
		for _, r := range recipes {
			courses := strings.Join(r.TagsByContext("courses"), ", ")
			fmt.Printf("  %3d  %-40s  %s\n", r.ID, r.Name, ui.MutedStyle.Render(courses))
		}
		fmt.Println()
		return nil
	}

	// Interactive TUI browser.
	selectedID, err := ui.RunListUI(recipes)
	if err != nil {
		return err
	}

	if selectedID > 0 {
		recipe, err := db.GetRecipe(sqlDB, selectedID)
		if err != nil {
			return err
		}
		// Clear screen then show detail.
		fmt.Print("\033[2J\033[H")
		fmt.Print(ui.RenderRecipeDetail(recipe, termWidth()))
		fmt.Println(ui.MutedStyle.Render(fmt.Sprintf("  ID: %d  ·  gorecipes show %d", recipe.ID, recipe.ID)))
		fmt.Println()
	}

	return nil
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
