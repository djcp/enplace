// Package selfmanage detects how the running binary was installed so that
// self-update can defer to the owning package manager instead of overwriting a
// binary the package manager tracks.
package selfmanage

import (
	"strings"
)

// Method identifies how the enplace binary was installed.
type Method int

const (
	// MethodUnknown is a manual/script install (curl|sh, go install, hand-copied).
	// Self-update may replace such a binary directly.
	MethodUnknown Method = iota
	// MethodHomebrew is a Homebrew-managed install (macOS or Linuxbrew).
	MethodHomebrew
	// MethodScoop is a Scoop-managed install (Windows).
	MethodScoop
)

// Detect infers the install method from the executable's path. Callers should
// pass an os.Executable() path, ideally with symlinks resolved via
// filepath.EvalSymlinks so package-manager Cellar/apps paths are visible.
func Detect(execPath string) Method {
	// Normalise both separators so Windows paths are matched regardless of the
	// host OS running the detection (filepath.ToSlash is a no-op off Windows).
	p := strings.ReplaceAll(strings.ToLower(execPath), "\\", "/")
	switch {
	case strings.Contains(p, "/cellar/"),
		strings.Contains(p, "/homebrew/"),
		strings.Contains(p, "/linuxbrew/"):
		return MethodHomebrew
	case strings.Contains(p, "/scoop/"):
		return MethodScoop
	default:
		return MethodUnknown
	}
}

// ManagesBinary reports whether a package manager owns the binary, meaning
// self-update must not replace it in place.
func (m Method) ManagesBinary() bool { return m != MethodUnknown }

// UpgradeCommand returns the shell command a user should run to upgrade via the
// owning package manager, or "" for a manual install.
func (m Method) UpgradeCommand() string {
	switch m {
	case MethodHomebrew:
		return "brew upgrade enplace"
	case MethodScoop:
		return "scoop update enplace"
	default:
		return ""
	}
}

// String returns a human-readable name for the install method.
func (m Method) String() string {
	switch m {
	case MethodHomebrew:
		return "Homebrew"
	case MethodScoop:
		return "Scoop"
	default:
		return "manual"
	}
}
