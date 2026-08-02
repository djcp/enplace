package export

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/djcp/enplace/internal/models"
	"github.com/djcp/enplace/internal/version"
)

// ArchiveSchema is the version of the JSON archive envelope. Bump it if the
// on-disk shape changes in a way a future importer needs to distinguish.
const ArchiveSchema = 1

// Archive is the top-level JSON envelope written by WriteArchive for the "json"
// format. It wraps the fully-hydrated recipes with enough metadata for a future
// `enplace import` to validate and round-trip them.
type Archive struct {
	Schema     int              `json:"schema"`
	AppVersion string           `json:"app_version"`
	ExportedAt time.Time        `json:"exported_at"`
	Count      int              `json:"recipe_count"`
	Recipes    []*models.Recipe `json:"recipes"`
}

// archiveRule separates recipes in the plain-text archive format.
const archiveRule = "\n\n" + "════════════════════════════════════════════════════════════════════════════════" + "\n\n"

// WriteArchive serialises fully-hydrated recipes to w in the given format
// ("json" or "text") and returns the number of recipes written.
//
// Callers are responsible for loading each recipe with its ingredients and tags
// (e.g. via db.GetRecipe) before passing them in — this keeps the export package
// free of any database dependency.
func WriteArchive(w io.Writer, recipes []*models.Recipe, format string) (int, error) {
	switch format {
	case "json":
		arc := Archive{
			Schema:     ArchiveSchema,
			AppVersion: version.Version,
			ExportedAt: time.Now().UTC(),
			Count:      len(recipes),
			Recipes:    recipes,
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(arc); err != nil {
			return 0, fmt.Errorf("encoding json archive: %w", err)
		}
		return len(recipes), nil
	case "text":
		for i, r := range recipes {
			if i > 0 {
				if _, err := io.WriteString(w, archiveRule); err != nil {
					return i, err
				}
			}
			if _, err := io.WriteString(w, strings.TrimRight(ToText(r, Options{}), "\n")+"\n"); err != nil {
				return i, err
			}
		}
		return len(recipes), nil
	default:
		return 0, fmt.Errorf("unknown archive format %q (want \"json\" or \"text\")", format)
	}
}
