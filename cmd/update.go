package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/djcp/enplace/internal/selfmanage"
	"github.com/djcp/enplace/internal/ui"
	"github.com/djcp/enplace/internal/version"
	"github.com/spf13/cobra"
)

// updateRepo is the GitHub owner/repo that hosts enplace releases.
const updateRepo = "djcp/enplace"

var updateCheckOnly bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update enplace to the latest release",
	Long: "Download and install the latest enplace release from GitHub.\n\n" +
		"If enplace was installed with a package manager (Homebrew, Scoop), this\n" +
		"command tells you the package-manager command to run instead of replacing\n" +
		"the managed binary.",
	// No database needed — mirror configCmd so `update` works even when the DB
	// or config is unavailable.
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error { return nil },
	SilenceUsage:      true, // don't dump usage text to stderr on error
	RunE:              runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateCheckOnly, "check", false, "check for a newer version without installing")
}

func runUpdate(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	current := strings.TrimPrefix(version.Version, "v")

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})
	if err != nil {
		return fmt.Errorf("initialising updater: %w", err)
	}

	repo := selfupdate.ParseSlug(updateRepo)
	release, found, err := updater.DetectLatest(ctx, repo)
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}
	if !found || release == nil {
		fmt.Println(ui.MutedStyle.Render("No published release found for " + updateRepo + "."))
		return nil
	}

	if !release.GreaterThan(current) {
		fmt.Println(ui.SuccessStyle.Render(fmt.Sprintf("✓ enplace %s is already up to date.", version.Version)))
		return nil
	}

	fmt.Println(ui.SuccessStyle.Render(fmt.Sprintf("A new version is available: %s (current: %s)", release.Version(), version.Version)))

	if updateCheckOnly {
		fmt.Println(ui.MutedStyle.Render("Run `enplace update` to install it."))
		return nil
	}

	// Resolve the real binary path (through any symlink) so both package-manager
	// detection and in-place replacement act on the actual file.
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	// If a package manager owns the binary, defer to it rather than clobbering
	// the file it tracks.
	if method := selfmanage.Detect(exe); method.ManagesBinary() {
		fmt.Println()
		fmt.Println(ui.MutedStyle.Render(fmt.Sprintf(
			"enplace was installed with %s. To upgrade, run:", method)))
		fmt.Println("    " + method.UpgradeCommand())
		return nil
	}

	if err := updater.UpdateTo(ctx, release, exe); err != nil {
		return fmt.Errorf("installing update: %w", err)
	}

	fmt.Println(ui.SuccessStyle.Render(fmt.Sprintf("✓ Updated enplace to %s.", release.Version())))
	return nil
}
