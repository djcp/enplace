package cmd

import (
	"fmt"

	"github.com/djcp/enplace/internal/config"
	"github.com/djcp/enplace/internal/ui"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or update configuration",
	// Override PersistentPreRunE so the config command works without a
	// database connection — useful when fixing a broken PostgreSQL DSN.
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error { return nil },
	RunE:              runConfig,
}

func runConfig(_ *cobra.Command, _ []string) error {
	configPath, _ := config.FilePath()
	logPath, _ := cfg.LogPath()

	saved, err := ui.RunConfigUI(cfg, configPath, logPath)
	if err != nil {
		return err
	}
	if saved {
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Println(ui.SuccessStyle.Render("✓ Configuration saved."))
	}
	return nil
}
