package models

// Tag represents a classification label in a specific context.
type Tag struct {
	ID      int64  `db:"id" json:"id"`
	Name    string `db:"name" json:"name"`
	Context string `db:"context" json:"context"`
}
