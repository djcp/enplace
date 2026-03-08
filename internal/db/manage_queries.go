package db

import (
	"database/sql"
	"strings"
	"time"

	"github.com/djcp/enplace/internal/models"
	"github.com/jmoiron/sqlx"
)

// execTx runs each step inside a single transaction, rolling back on the first error.
func execTx(db *sqlx.DB, steps ...func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for _, step := range steps {
		if err := step(tx); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// TagWithCount is a tag row augmented with the number of recipes using it.
type TagWithCount struct {
	ID      int64  `db:"id"`
	Name    string `db:"name"`
	Context string `db:"context"`
	Count   int    `db:"count"`
}

// IngredientWithCount is an ingredient augmented with the number of recipe_ingredient rows.
type IngredientWithCount struct {
	ID    int64  `db:"id"`
	Name  string `db:"name"`
	Count int    `db:"count"`
}

// UnitWithCount is a distinct unit value with its usage count.
type UnitWithCount struct {
	Name  string `db:"name"`
	Count int    `db:"count"`
}

// AIRunSummary is a lightweight view of an ai_classifier_runs row.
type AIRunSummary struct {
	ID           int64      `db:"id"`
	RecipeName   string     `db:"recipe_name"` // "" when recipe has been deleted
	ServiceClass string     `db:"service_class"`
	AIModel      string     `db:"ai_model"`
	Success      bool       `db:"success"`
	DurationMS   int64      // computed after scan
	StartedAt    *time.Time `db:"started_at"`
	CompletedAt  *time.Time `db:"completed_at"`
	CreatedAt    time.Time  `db:"created_at"`
}

// --- Tag management ---

// ListTagsByContext returns all tags in a context with their recipe counts, sorted by name.
func ListTagsByContext(db *sqlx.DB, context string) ([]TagWithCount, error) {
	var rows []TagWithCount
	err := db.Select(&rows, `
		SELECT t.id, t.name, t.context,
		       COUNT(rt.recipe_id) AS count
		FROM tags t
		LEFT JOIN recipe_tags rt ON rt.tag_id = t.id
		WHERE t.context = ?
		GROUP BY t.id, t.name, t.context
		ORDER BY t.name`,
		context,
	)
	return rows, err
}

// RenameTag updates a tag's name in-place.
func RenameTag(db *sqlx.DB, id int64, newName string) error {
	_, err := db.Exec(`UPDATE tags SET name = ? WHERE id = ?`, newName, id)
	return err
}

// MergeTag repoints all recipe_tags from sourceID to targetID, then deletes the source tag.
func MergeTag(db *sqlx.DB, sourceID, targetID int64) error {
	return execTx(db,
		func(tx *sql.Tx) error {
			_, err := tx.Exec(`
				INSERT OR IGNORE INTO recipe_tags (recipe_id, tag_id)
				SELECT recipe_id, ? FROM recipe_tags WHERE tag_id = ?`,
				targetID, sourceID,
			)
			return err
		},
		func(tx *sql.Tx) error {
			_, err := tx.Exec(`DELETE FROM recipe_tags WHERE tag_id = ?`, sourceID)
			return err
		},
		func(tx *sql.Tx) error {
			_, err := tx.Exec(`DELETE FROM tags WHERE id = ?`, sourceID)
			return err
		},
	)
}

// DeleteTag removes a tag row; recipe_tags cascade-deletes automatically.
func DeleteTag(db *sqlx.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM tags WHERE id = ?`, id)
	return err
}

// --- Ingredient management ---

// ListIngredientsWithCount returns ingredients with their usage counts, sorted alphabetically.
// When search is non-empty, only ingredients whose name contains the search string (case-insensitive) are returned.
func ListIngredientsWithCount(db *sqlx.DB, search string) ([]IngredientWithCount, error) {
	var rows []IngredientWithCount
	if search == "" {
		err := db.Select(&rows, `
			SELECT i.id, i.name,
			       COUNT(ri.id) AS count
			FROM ingredients i
			LEFT JOIN recipe_ingredients ri ON ri.ingredient_id = i.id
			GROUP BY i.id, i.name
			ORDER BY i.name`)
		return rows, err
	}
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(search)
	err := db.Select(&rows, `
		SELECT i.id, i.name,
		       COUNT(ri.id) AS count
		FROM ingredients i
		LEFT JOIN recipe_ingredients ri ON ri.ingredient_id = i.id
		WHERE i.name LIKE ? ESCAPE '\'
		GROUP BY i.id, i.name
		ORDER BY i.name`, "%"+escaped+"%")
	return rows, err
}

// RenameIngredient updates an ingredient's name.
func RenameIngredient(db *sqlx.DB, id int64, newName string) error {
	_, err := db.Exec(`UPDATE ingredients SET name = ? WHERE id = ?`, newName, id)
	return err
}

// MergeIngredient repoints all recipe_ingredient rows from source to target,
// then deletes the source ingredient.
func MergeIngredient(db *sqlx.DB, sourceID, targetID int64) error {
	return execTx(db,
		func(tx *sql.Tx) error {
			_, err := tx.Exec(
				`UPDATE recipe_ingredients SET ingredient_id = ? WHERE ingredient_id = ?`,
				targetID, sourceID,
			)
			return err
		},
		func(tx *sql.Tx) error {
			_, err := tx.Exec(`DELETE FROM ingredients WHERE id = ?`, sourceID)
			return err
		},
	)
}

// --- Unit management ---

// ListUnitsWithCount returns all distinct non-empty unit values with usage counts.
func ListUnitsWithCount(db *sqlx.DB) ([]UnitWithCount, error) {
	var rows []UnitWithCount
	err := db.Select(&rows, `
		SELECT unit AS name, COUNT(*) AS count
		FROM recipe_ingredients
		WHERE unit != ''
		GROUP BY unit
		ORDER BY unit`)
	return rows, err
}

// RenameUnit updates every recipe_ingredient row with oldName to newName.
func RenameUnit(db *sqlx.DB, oldName, newName string) error {
	_, err := db.Exec(
		`UPDATE recipe_ingredients SET unit = ? WHERE unit = ?`,
		newName, oldName,
	)
	return err
}

// MergeUnit is equivalent to RenameUnit — all rows with sourceName get targetName.
func MergeUnit(db *sqlx.DB, sourceName, targetName string) error {
	return RenameUnit(db, sourceName, targetName)
}

// --- AI Run management ---

// ListAIRunSummaries returns all AI runs, newest first, with recipe name when available.
func ListAIRunSummaries(db *sqlx.DB) ([]AIRunSummary, error) {
	var rows []AIRunSummary
	err := db.Select(&rows, `
		SELECT a.id, COALESCE(r.name, '') AS recipe_name,
		       a.service_class, a.ai_model, a.success,
		       a.started_at, a.completed_at, a.created_at
		FROM ai_classifier_runs a
		LEFT JOIN recipes r ON r.id = a.recipe_id
		ORDER BY a.created_at DESC`)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].StartedAt != nil && rows[i].CompletedAt != nil {
			rows[i].DurationMS = rows[i].CompletedAt.Sub(*rows[i].StartedAt).Milliseconds()
		} else {
			rows[i].DurationMS = -1
		}
	}
	return rows, nil
}

// GetAIRun returns the full AIClassifierRun record by ID.
func GetAIRun(db *sqlx.DB, id int64) (*models.AIClassifierRun, error) {
	var r models.AIClassifierRun
	err := db.Get(&r, `SELECT * FROM ai_classifier_runs WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// DeleteAIRun removes a single AI run by ID.
func DeleteAIRun(db *sqlx.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM ai_classifier_runs WHERE id = ?`, id)
	return err
}

// DeleteAIRunsOlderThan removes AI runs created before now-age and returns the count deleted.
func DeleteAIRunsOlderThan(db *sqlx.DB, age time.Duration) (int64, error) {
	cutoff := time.Now().Add(-age)
	result, err := db.Exec(
		`DELETE FROM ai_classifier_runs WHERE created_at < ?`, cutoff,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
