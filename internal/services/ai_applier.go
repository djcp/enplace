package services

import (
	"github.com/djcp/gorecipes/internal/db"
	"github.com/djcp/gorecipes/internal/models"
	"github.com/jmoiron/sqlx"
)

// ApplyExtractedRecipe writes all AI-extracted data to an existing recipe row.
// It replaces ingredients and tags, then sets the recipe status to "review".
func ApplyExtractedRecipe(sqlDB *sqlx.DB, recipeID int64, extracted *ExtractedRecipe) error {
	// Build the updated recipe model.
	r := &models.Recipe{
		ID:          recipeID,
		Name:        extracted.Name,
		Description: extracted.Description,
		Directions:  extracted.Directions,
		PreparationTime: extracted.PreparationTime,
		CookingTime:     extracted.CookingTime,
		Servings:        extracted.Servings,
		Status:      models.StatusReview,
	}
	if extracted.ServingUnits != nil {
		r.ServingUnits = *extracted.ServingUnits
	}

	if err := db.UpdateRecipeFields(sqlDB, r); err != nil {
		return err
	}

	// Replace ingredients.
	if err := db.DeleteRecipeIngredients(sqlDB, recipeID); err != nil {
		return err
	}
	for pos, ing := range extracted.Ingredients {
		ingID, err := db.FindOrCreateIngredient(sqlDB, ing.Name)
		if err != nil {
			return err
		}
		ri := &models.RecipeIngredient{
			RecipeID:     recipeID,
			IngredientID: ingID,
			Quantity:     ing.Quantity,
			Unit:         ing.Unit,
			Position:     pos,
		}
		if ing.Descriptor != nil {
			ri.Descriptor = *ing.Descriptor
		}
		if ing.Section != nil {
			ri.Section = *ing.Section
		}
		if err := db.InsertRecipeIngredient(sqlDB, ri); err != nil {
			return err
		}
	}

	// Replace tags.
	if err := db.DeleteRecipeTags(sqlDB, recipeID); err != nil {
		return err
	}
	tagContexts := map[string][]string{
		models.TagContextCookingMethods:      extracted.CookingMethods,
		models.TagContextCulturalInfluences:  extracted.CulturalInfluences,
		models.TagContextCourses:             extracted.Courses,
		models.TagContextDietaryRestrictions: extracted.DietaryRestrictions,
	}
	for ctx, names := range tagContexts {
		for _, name := range names {
			if name == "" {
				continue
			}
			tagID, err := db.FindOrCreateTag(sqlDB, name, ctx)
			if err != nil {
				return err
			}
			if err := db.AttachTag(sqlDB, recipeID, tagID); err != nil {
				return err
			}
		}
	}

	return nil
}
