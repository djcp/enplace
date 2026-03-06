package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/gorecipes/internal/config"
	"github.com/djcp/gorecipes/internal/db"
	"github.com/djcp/gorecipes/internal/ui"
	"github.com/djcp/gorecipes/internal/version"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/cobra"
)

var (
	cfg   *config.Config
	sqlDB *sqlx.DB
)

// Root is the top-level command. Running it with no subcommand opens the recipe browser.
var Root = &cobra.Command{
	Use:     "gorecipes",
	Short:   "A CLI recipe manager powered by Claude AI",
	Long:    "gorecipes — save recipes from URLs or pasted text.\nClaude extracts structured data automatically.",
	Version: version.Version,
	RunE:    runList,
}

func init() {
	Root.AddCommand(addCmd)
	Root.AddCommand(listCmd)
	Root.AddCommand(showCmd)
	Root.AddCommand(configCmd)

	cobra.OnInitialize(initApp)
}

// Execute runs the root command.
func Execute() {
	if err := Root.Execute(); err != nil {
		os.Exit(1)
	}
}

func initApp() {
	var err error
	cfg, err = config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if !cfg.IsConfigured() {
		if err := runOnboarding(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Setup cancelled.\n")
			os.Exit(1)
		}
	}

	sqlDB, err = db.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
}

func runOnboarding(cfg *config.Config) error {
	fmt.Println()

	banner := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.ColorPrimary).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorBorder).
		Padding(1, 3).
		Render("🍳  Welcome to gorecipes\n\n" +
			lipgloss.NewStyle().
				Bold(false).
				Foreground(ui.ColorMuted).
				Render("Save recipes from URLs or pasted text.\nClaude AI extracts structured data automatically."))
	fmt.Println(banner)
	fmt.Println()

	var apiKey string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Anthropic API Key").
				Description("Your key is stored in ~/.config/gorecipes/config.json\nGet one at https://console.anthropic.com/").
				Password(true).
				Value(&apiKey).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						return fmt.Errorf("API key is required")
					}
					if !strings.HasPrefix(s, "sk-ant-") {
						return fmt.Errorf("Anthropic API keys start with sk-ant-")
					}
					return nil
				}),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	cfg.AnthropicAPIKey = strings.TrimSpace(apiKey)
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	path, _ := config.FilePath()
	fmt.Println()
	fmt.Println(ui.SuccessStyle.Render("✓ API key saved to " + path))
	fmt.Println()

	return nil
}
