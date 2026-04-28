package db

import "time"

// UpdateRecipeRating sets or clears the rating (1–5) for a recipe.
// Pass nil to clear the rating.
func UpdateRecipeRating(db *DB, id int64, rating *int) error {
	_, err := db.Exec(
		`UPDATE recipes SET rating = ?, updated_at = ? WHERE id = ?`,
		rating, time.Now(), id,
	)
	return err
}

// UpdateRecipeNotes saves the freeform notes text for a recipe.
func UpdateRecipeNotes(db *DB, id int64, notes string) error {
	_, err := db.Exec(
		`UPDATE recipes SET notes = ?, updated_at = ? WHERE id = ?`,
		notes, time.Now(), id,
	)
	return err
}
