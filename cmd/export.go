package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/djcp/enplace/internal/db"
	"github.com/djcp/enplace/internal/export"
	"github.com/djcp/enplace/internal/models"
	"github.com/djcp/enplace/internal/ui"
	"github.com/spf13/cobra"
)

var (
	exportFormat string
	exportOut    string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all recipes to a single JSON or text file",
	Long: "Archive every saved recipe to one file.\n\n" +
		"JSON (default) is a lossless, structured archive; text is human-readable.\n" +
		"When --out is omitted you'll be prompted for a file location.",
	SilenceUsage: true, // don't dump usage text to stderr on error
	RunE:         runExport,
}

func init() {
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "json", "export format: json or text")
	exportCmd.Flags().StringVarP(&exportOut, "out", "o", "", "output file path (prompted if omitted)")
}

func runExport(_ *cobra.Command, _ []string) error {
	format, ext, ok := canonicalFormat(exportFormat)
	if !ok {
		return fmt.Errorf("invalid --format %q: want \"json\" or \"text\"", exportFormat)
	}

	recipes, err := loadAllRecipes()
	if err != nil {
		return err
	}
	if len(recipes) == 0 {
		fmt.Println(ui.MutedStyle.Render("No recipes to export."))
		return nil
	}

	outPath, err := resolveExportPath(ext)
	if err != nil {
		return err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", outPath, err)
	}
	defer f.Close()

	n, err := export.WriteArchive(f, recipes, format)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	fmt.Println(ui.SuccessStyle.Render(fmt.Sprintf("✓ Exported %d recipe%s to %s", n, plural(n), outPath)))
	return nil
}

// loadAllRecipes returns every recipe fully hydrated with ingredients and tags.
func loadAllRecipes() ([]*models.Recipe, error) {
	list, err := db.ListRecipes(sqlDB, db.RecipeFilter{})
	if err != nil {
		return nil, fmt.Errorf("listing recipes: %w", err)
	}
	recipes := make([]*models.Recipe, 0, len(list))
	for i := range list {
		full, err := db.GetRecipe(sqlDB, list[i].ID)
		if err != nil {
			return nil, fmt.Errorf("loading recipe %d: %w", list[i].ID, err)
		}
		recipes = append(recipes, full)
	}
	return recipes, nil
}

// resolveExportPath returns the file path to write. If --out was supplied it is
// used verbatim (with ~ expansion). Otherwise the user is prompted, pre-filled
// with a dated default; in a non-interactive context the default is used silently.
func resolveExportPath(ext string) (string, error) {
	if exportOut != "" {
		return expandHome(exportOut), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	base := "enplace_recipe_backup_" + time.Now().Format("2006-01-02")
	def := export.UniqueFilePath(home, base, ext)

	path := def
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Where should the export be saved?").
			Description("Press Enter to accept the default.").
			Value(&path),
	))
	if err := form.Run(); err != nil {
		// Non-interactive terminal or aborted (Esc): fall back to the default.
		return def, nil
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return def, nil
	}
	return expandHome(path), nil
}

// canonicalFormat normalises a user-supplied format name to the WriteArchive
// format ("json"/"text") and its file extension ("json"/"txt").
func canonicalFormat(format string) (canonical, ext string, ok bool) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return "json", "json", true
	case "text", "txt":
		return "text", "txt", true
	default:
		return "", "", false
	}
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
