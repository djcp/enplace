package models

// RecipeIngredient is the join model between Recipe and Ingredient,
// carrying quantity, unit, descriptor, and section metadata.
type RecipeIngredient struct {
	ID           int64  `db:"id"`
	RecipeID     int64  `db:"recipe_id"`
	IngredientID int64  `db:"ingredient_id"`
	Quantity     string `db:"quantity"`
	Unit         string `db:"unit"`
	Descriptor   string `db:"descriptor"`
	Section      string `db:"section"`
	Position     int    `db:"position"`

	// Populated via JOIN when loading recipe ingredients.
	IngredientName string `db:"ingredient_name"`
}

// DisplayString returns a human-readable ingredient line.
// e.g. "1 cup flour, sifted" or "2 large eggs"
func (ri *RecipeIngredient) DisplayString() string {
	s := ""
	if ri.Quantity != "" {
		s += ri.Quantity
	}
	if ri.Unit != "" {
		if s != "" {
			s += " "
		}
		s += ri.Unit
	}
	if ri.IngredientName != "" {
		if s != "" {
			s += " "
		}
		s += ri.IngredientName
	}
	if ri.Descriptor != "" {
		s += ", " + ri.Descriptor
	}
	return s
}
