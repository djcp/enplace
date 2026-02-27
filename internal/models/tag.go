package models

// Tag represents a classification label in a specific context.
type Tag struct {
	ID      int64  `db:"id"`
	Name    string `db:"name"`
	Context string `db:"context"`
}
