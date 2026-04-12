package db

import (
	"fmt"

	"github.com/djcp/enplace/internal/scaling"
	"github.com/jmoiron/sqlx"
)

// BackfillQuantityNumeric populates quantity_numeric for any recipe_ingredients
// rows where it is NULL and quantity is non-empty. Safe to call at every startup:
// it is a no-op once all rows are populated. Uses a single transaction with a
// prepared statement so the entire backfill is one atomic disk write.
func BackfillQuantityNumeric(sqlDB *sqlx.DB) error {
	var rows []struct {
		ID       int64  `db:"id"`
		Quantity string `db:"quantity"`
	}
	if err := sqlDB.Select(&rows, `
		SELECT id, quantity
		FROM recipe_ingredients
		WHERE quantity_numeric IS NULL AND quantity != ''`,
	); err != nil {
		return fmt.Errorf("backfill select: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}

	tx, err := sqlDB.Beginx()
	if err != nil {
		return fmt.Errorf("backfill begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(`UPDATE recipe_ingredients SET quantity_numeric = ? WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("backfill prepare: %w", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		if v, ok := scaling.ParseQuantity(row.Quantity); ok {
			if _, err := stmt.Exec(v, row.ID); err != nil {
				return fmt.Errorf("backfill update id=%d: %w", row.ID, err)
			}
		}
	}

	return tx.Commit()
}

// BackfillIngredientTypes migrates ingredient_type values to the five-type
// scheme introduced alongside baker's-percentage rework:
//
//   - "dry" ingredients whose names match flour patterns → "flour"
//   - "wet" ingredients whose names match fat patterns   → "fat"
//
// All other existing type values are left unchanged. Safe to call at every
// startup: rows that already carry the correct type are not touched.
func BackfillIngredientTypes(sqlDB *sqlx.DB) error {
	// Patterns that identify flour ingredients by name.
	flourPatterns := []string{
		"%flour%",
		"%semolina%",
	}
	// Patterns that identify fat ingredients by name (currently classified "wet").
	fatPatterns := []string{
		"butter",
		"lard",
		"shortening",
		"margarine",
		"cocoa butter",
		"suet",
	}

	tx, err := sqlDB.Beginx()
	if err != nil {
		return fmt.Errorf("backfill ingredient types begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	for _, p := range flourPatterns {
		if _, err := tx.Exec(
			`UPDATE ingredients SET ingredient_type = 'flour' WHERE ingredient_type = 'dry' AND name LIKE ?`,
			p,
		); err != nil {
			return fmt.Errorf("backfill flour pattern %q: %w", p, err)
		}
	}

	for _, p := range fatPatterns {
		if _, err := tx.Exec(
			`UPDATE ingredients SET ingredient_type = 'fat' WHERE ingredient_type IN ('wet', '') AND name = ?`,
			p,
		); err != nil {
			return fmt.Errorf("backfill fat pattern %q: %w", p, err)
		}
	}

	return tx.Commit()
}
