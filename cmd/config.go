package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/gorecipes/internal/config"
	"github.com/djcp/gorecipes/internal/ui"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or update configuration",
	RunE:  runConfig,
}

func runConfig(_ *cobra.Command, _ []string) error {
	path, _ := config.FilePath()

	// Display current config.
	maskedKey := maskKey(cfg.AnthropicAPIKey)
	fmt.Println()
	fmt.Println(ui.TitleStyle.Render("  Configuration"))

	rows := [][]string{
		{"API Key", maskedKey},
		{"Model", cfg.AnthropicModel},
		{"Database", cfg.DBPath},
		{"Config file", path},
	}
	for _, row := range rows {
		label := lipgloss.NewStyle().
			Foreground(ui.ColorMuted).
			Width(14).
			Render(row[0])
		fmt.Printf("  %s  %s\n", label, row[1])
	}
	fmt.Println()

	// Offer to update.
	var action string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(
					huh.NewOption("Update API key", "key"),
					huh.NewOption("Change AI model", "model"),
					huh.NewOption("Exit", "exit"),
				).
				Value(&action),
		),
	)
	if err := form.Run(); err != nil {
		return nil // User pressed Esc
	}

	switch action {
	case "key":
		return updateAPIKey()
	case "model":
		return updateModel()
	}
	return nil
}

func updateAPIKey() error {
	var newKey string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("New Anthropic API Key").
				Password(true).
				Value(&newKey).
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
		return nil
	}
	cfg.AnthropicAPIKey = strings.TrimSpace(newKey)
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Println(ui.SuccessStyle.Render("✓ API key updated."))
	return nil
}

func updateModel() error {
	var newModel string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select Claude model").
				Description("Haiku is fastest and cheapest; Sonnet is more accurate.").
				Options(
					huh.NewOption("claude-haiku-4-5-20251001 (default, fastest)", "claude-haiku-4-5-20251001"),
					huh.NewOption("claude-sonnet-4-6 (more accurate)", "claude-sonnet-4-6"),
					huh.NewOption("claude-opus-4-6 (most accurate)", "claude-opus-4-6"),
				).
				Value(&newModel),
		),
	)
	if err := form.Run(); err != nil {
		return nil
	}
	cfg.AnthropicModel = newModel
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Printf("%s Model set to %s\n", ui.SuccessStyle.Render("✓"), newModel)
	return nil
}

func maskKey(key string) string {
	if key == "" {
		return ui.MutedStyle.Render("(not set)")
	}
	if len(key) <= 12 {
		return "sk-ant-***"
	}
	return key[:10] + "..." + key[len(key)-4:]
}
