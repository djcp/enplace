package cmd

import (
	"github.com/djcp/gorecipes/internal/db"
	"github.com/djcp/gorecipes/internal/models"
	"github.com/djcp/gorecipes/internal/ui"
)

func loadEditData() (ui.EditData, error) {
	ingNames, err := db.AllIngredientNames(sqlDB)
	if err != nil {
		return ui.EditData{}, err
	}
	units, err := db.AllUnits(sqlDB)
	if err != nil {
		return ui.EditData{}, err
	}
	tags := make(map[string][]string)
	for _, ctx := range models.AllTagContexts {
		names, err := db.AllTagsByContext(sqlDB, ctx)
		if err != nil {
			return ui.EditData{}, err
		}
		tags[ctx] = names
	}
	return ui.EditData{TagsByContext: tags, IngredientNames: ingNames, Units: units}, nil
}
