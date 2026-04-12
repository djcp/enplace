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
