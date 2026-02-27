package cmd

import (
	"fmt"
	"strconv"

	"github.com/djcp/gorecipes/internal/db"
	"github.com/djcp/gorecipes/internal/ui"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Display a recipe by ID",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func runShow(_ *cobra.Command, args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || id <= 0 {
		return fmt.Errorf("invalid recipe ID: %q", args[0])
	}

	recipe, err := db.GetRecipe(sqlDB, id)
	if err != nil {
		return fmt.Errorf("recipe not found: %w", err)
	}

	fmt.Print(ui.RenderRecipeDetail(recipe, termWidth()))
	fmt.Println(ui.MutedStyle.Render(fmt.Sprintf("  ID: %d", recipe.ID)))
	fmt.Println()

	return nil
}
