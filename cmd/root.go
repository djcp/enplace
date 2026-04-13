package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/enplace/internal/config"
	"github.com/djcp/enplace/internal/db"
	"github.com/djcp/enplace/internal/logging"
	"github.com/djcp/enplace/internal/ui"
	"github.com/djcp/enplace/internal/version"
	"github.com/spf13/cobra"
)

var (
	cfg   *config.Config
	sqlDB *db.DB
)

// Root is the top-level command. Running it with no subcommand opens the recipe browser.
var Root = &cobra.Command{
	Use:     "enplace",
	Short:   "A CLI recipe manager powered by Claude AI",
	Long:    "enplace — save recipes from URLs or pasted text.\nClaude extracts structured data automatically.",
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

	logPath, err := cfg.LogPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving log path: %v\n", err)
		os.Exit(1)
	}
	logger, logFile, err := logging.Open(logPath, cfg.MaxLogLines)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		os.Exit(1)
	}
	Root.PersistentPostRun = func(_ *cobra.Command, _ []string) { logFile.Close() }

	sqlDB, err = db.Open(cfg, logging.GooseLogger(logger))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}

	if err := db.BackfillQuantityNumeric(sqlDB); err != nil {
		// Non-fatal: log and continue. Scaling won't work for unparsed rows.
		logger.Warn("quantity_numeric backfill failed", "error", err)
	}
	if err := db.BackfillIngredientTypes(sqlDB); err != nil {
		// Non-fatal: log and continue. Existing bread recipes will re-extract cleanly.
		logger.Warn("ingredient_type backfill failed", "error", err)
	}

	if cfg.Driver() == "postgres" {
		sqliteCount, err := db.SQLiteHasRecipes(cfg.DBPath)
		if err != nil {
			logger.Warn("could not check sqlite recipe count", "error", err)
		} else if sqliteCount > 0 {
			if err := runMigrationFlow(sqlDB, cfg, logger); err != nil {
				fmt.Fprintf(os.Stderr, "Migration error: %v\n", err)
				os.Exit(1)
			}
		}
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
		Render("🍳  Welcome to enplace\n\n" +
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
				Description("Your key is stored in ~/.config/enplace/config.json\nGet one at https://console.anthropic.com/").
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

	// Step 2: Database choice.
	var dbChoice string
	form2 := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Where would you like to store your recipes?").
				Options(
					huh.NewOption("Local SQLite (recommended — no setup required)", "sqlite"),
					huh.NewOption("PostgreSQL", "postgres"),
				).
				Value(&dbChoice),
		),
	)
	if err := form2.Run(); err != nil {
		// Non-fatal: default to sqlite
		dbChoice = "sqlite"
	}

	if dbChoice == "postgres" {
		for {
			var dsn string
			form3 := huh.NewForm(huh.NewGroup(
				huh.NewInput().
					Title("PostgreSQL connection string").
					Description("Examples:\n  Remote: postgres://user:pass@host:5432/dbname?sslmode=require\n  Local:  host=/run/postgresql dbname=enplace").
					Value(&dsn).
					Validate(func(s string) error {
						s = strings.TrimSpace(s)
						if s == "" {
							return fmt.Errorf("connection string required")
						}
						if err := db.TestPostgresConnection(s); err != nil {
							return fmt.Errorf("could not connect: %v", err)
						}
						return nil
					}),
			))
			err := form3.Run()
			if err == nil {
				cfg.PostgresDSN = strings.TrimSpace(dsn)
				break
			}
			// ErrUserAborted (Esc pressed) — offer fallback
			if errors.Is(err, huh.ErrUserAborted) {
				var fallback string
				form4 := huh.NewForm(huh.NewGroup(
					huh.NewSelect[string]().
						Title("Connection failed. What would you like to do?").
						Options(
							huh.NewOption("Try a different connection string", "retry"),
							huh.NewOption("Use local SQLite instead", "sqlite"),
						).
						Value(&fallback),
				))
				if err2 := form4.Run(); err2 != nil || fallback == "sqlite" {
					cfg.PostgresDSN = ""
					break
				}
				// fallback == "retry": loop continues
			} else {
				return err
			}
		}
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	path, _ := config.FilePath()
	fmt.Println()
	if cfg.PostgresDSN != "" {
		fmt.Println(ui.SuccessStyle.Render("✓ Config saved — using PostgreSQL: " + config.MaskDSN(cfg.PostgresDSN)))
	} else {
		fmt.Println(ui.SuccessStyle.Render("✓ Config saved — using local SQLite: " + cfg.DBPath))
	}
	fmt.Println(ui.MutedStyle.Render("  Config file: " + path))
	fmt.Println()

	return nil
}
