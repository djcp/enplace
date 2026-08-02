package models

// RecipeIngredient is the join model between Recipe and Ingredient,
// carrying quantity, unit, descriptor, and section metadata.
type RecipeIngredient struct {
	ID              int64    `db:"id" json:"id"`
	RecipeID        int64    `db:"recipe_id" json:"recipe_id"`
	IngredientID    int64    `db:"ingredient_id" json:"ingredient_id"`
	Quantity        string   `db:"quantity" json:"quantity"`
	QuantityNumeric *float64 `db:"quantity_numeric" json:"quantity_numeric"`
	Unit            string   `db:"unit" json:"unit"`
	UnitWeightG     *float64 `db:"unit_weight_g" json:"unit_weight_g"`
	Descriptor      string   `db:"descriptor" json:"descriptor"`
	Section         string   `db:"section" json:"section"`
	Position        int      `db:"position" json:"position"`

	// Populated via JOIN when loading recipe ingredients.
	IngredientName string `db:"ingredient_name" json:"ingredient_name"`
	IngredientType string `db:"ingredient_type" json:"ingredient_type"`
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
