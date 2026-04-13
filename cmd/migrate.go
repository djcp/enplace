package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/djcp/enplace/internal/config"
	"github.com/djcp/enplace/internal/db"
	"github.com/djcp/enplace/internal/export"
	"github.com/pressly/goose/v3"
)

// backupPickerModel wraps the filepicker and returns on directory selection.
type backupPickerModel struct {
	fp       filepicker.Model
	selected string
	done     bool
	quitting bool
	width    int
	height   int
}

func (m backupPickerModel) Init() tea.Cmd {
	return m.fp.Init()
}

func (m backupPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.fp, cmd = m.fp.Update(msg)

	if didSelect, path := m.fp.DidSelectFile(msg); didSelect {
		m.selected = path
		m.done = true
		return m, tea.Quit
	}

	return m, cmd
}

func (m backupPickerModel) View() string {
	if m.quitting {
		return ""
	}
	return fmt.Sprintf("  Choose a directory for the backup:\n\n%s\n\n  Press esc to skip backup.", m.fp.View())
}

// runMigrationFlow checks for existing SQLite data and offers to import it
// into the postgres database. Called from initApp when postgres is configured
// and sqlite has recipes.
func runMigrationFlow(pgDB *db.DB, cfg *config.Config, logger *slog.Logger) error {
	sqliteCount, err := db.SQLiteHasRecipes(cfg.DBPath)
	if err != nil {
		logger.Warn("could not count sqlite recipes", "error", err)
		sqliteCount = 0
	}

	pgCount, err := db.RecipeCount(pgDB)
	if err != nil {
		logger.Warn("could not count postgres recipes", "error", err)
	}

	fmt.Println()
	fmt.Printf("  Found %d recipe(s) in local SQLite and %d in PostgreSQL.\n\n", sqliteCount, pgCount)

	// Phase 1: import offer
	var importChoice string
	form1 := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Import local recipes into PostgreSQL?").
			Options(
				huh.NewOption("Yes, import them now", "yes"),
				huh.NewOption("No, not now", "no"),
			).
			Value(&importChoice),
	))
	if err := form1.Run(); err != nil || importChoice == "no" {
		return nil
	}

	// Phase 2: backup offer
	var backupChoice string
	form2 := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Back up the SQLite database before migrating?").
			Options(
				huh.NewOption("Yes, choose backup location", "yes"),
				huh.NewOption("No, skip backup", "no"),
			).
			Value(&backupChoice),
	))
	if err := form2.Run(); err != nil {
		backupChoice = "no"
	}

	// Phase 3 (conditional): filepicker for backup directory
	if backupChoice == "yes" {
		fp := filepicker.New()
		homeDir, _ := os.UserHomeDir()
		fp.CurrentDirectory = homeDir
		fp.DirAllowed = true
		fp.FileAllowed = false

		m := backupPickerModel{fp: fp}
		prog := tea.NewProgram(m, tea.WithAltScreen())
		final, runErr := prog.Run()
		if runErr == nil {
			fm := final.(backupPickerModel)
			if fm.selected != "" {
				dateSuffix := time.Now().Format("2006-01-02")
				destPath := export.UniqueFilePath(fm.selected, "enplace-backup-"+dateSuffix, "db")
				fmt.Printf("\n  Backing up to %s…\n", destPath)
				if err := db.BackupSQLite(cfg.DBPath, destPath, logger); err != nil {
					fmt.Printf("  Warning: backup failed: %v\n", err)
				} else {
					fmt.Printf("  Backup complete.\n")
				}
			}
		}
	}

	// Phase 4: migration
	fmt.Println("\n  Importing recipes to PostgreSQL…")

	sqliteDB, err := db.OpenSQLite(cfg.DBPath, goose.NopLogger())
	if err != nil {
		return fmt.Errorf("opening sqlite for migration: %w", err)
	}
	defer sqliteDB.Close()

	result, err := db.MigrateToPostgres(sqliteDB, pgDB, logger)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Phase 5: clear sqlite data
	if err := db.ClearSQLiteData(sqliteDB, logger); err != nil {
		return fmt.Errorf("clearing sqlite after migration: %w", err)
	}

	fmt.Printf("\n  Migration complete: %d imported, %d skipped (already in PostgreSQL).\n\n",
		result.Imported, result.Skipped)
	return nil
}
