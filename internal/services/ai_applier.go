package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/djcp/enplace/internal/db"
	"github.com/djcp/enplace/internal/models"
	"github.com/djcp/enplace/internal/scaling"
)

// ApplyExtractedRecipe writes all AI-extracted data to an existing recipe row.
// It replaces ingredients and tags, then sets the recipe status to "review".
// The entire operation runs in a transaction so partial failures are rolled back.
func ApplyExtractedRecipe(sqlDB *db.DB, recipeID int64, extracted *ExtractedRecipe) error {
	// Preserve fields the AI result must not overwrite (e.g. source_url).
	existing, err := db.GetRecipe(sqlDB, recipeID)
	if err != nil {
		return err
	}

	tx, err := sqlDB.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op if committed

	now := time.Now()
	r := &models.Recipe{
		ID:              recipeID,
		Name:            extracted.Name,
		Description:     extracted.Description,
		Directions:      extracted.Directions,
		PreparationTime: extracted.PreparationTime,
		CookingTime:     extracted.CookingTime,
		Servings:        extracted.Servings,
		Status:          models.StatusReview,
		SourceURL:       existing.SourceURL,
		SourceText:      existing.SourceText,
		IsBread:         extracted.IsBread,
	}
	if extracted.ServingUnits != nil {
		r.ServingUnits = *extracted.ServingUnits
	}

	if _, err := tx.Exec(sqlDB.Rebind(
		`UPDATE recipes
		SET name = ?, description = ?, directions = ?,
		    preparation_time = ?, cooking_time = ?,
		    servings = ?, serving_units = ?,
		    is_bread = ?,
		    source_url = ?,
		    status = ?, updated_at = ?
		WHERE id = ?`),
		r.Name, r.Description, r.Directions,
		r.PreparationTime, r.CookingTime,
		r.Servings, r.ServingUnits,
		r.IsBread,
		r.SourceURL,
		r.Status, now, r.ID,
	); err != nil {
		return fmt.Errorf("update recipe fields: %w", err)
	}

	// Replace ingredients.
	if _, err := tx.Exec(sqlDB.Rebind(`DELETE FROM recipe_ingredients WHERE recipe_id = ?`), recipeID); err != nil {
		return fmt.Errorf("delete recipe ingredients: %w", err)
	}
	for pos, ing := range extracted.Ingredients {
		ingName := strings.ToLower(strings.TrimSpace(ing.Name))
		var ingID int64
		if sqlDB.Driver() == "postgres" {
			err := tx.QueryRow(sqlDB.Rebind(`SELECT id FROM ingredients WHERE name = ?`), ingName).Scan(&ingID)
			if err != nil {
				err := tx.QueryRow(sqlDB.Rebind(
					`INSERT INTO ingredients (name, created_at) VALUES (?, ?) RETURNING id`),
					ingName, now,
				).Scan(&ingID)
				if err != nil {
					return fmt.Errorf("find or create ingredient: %w", err)
				}
			}
		} else {
			err := tx.QueryRow(sqlDB.Rebind(`SELECT id FROM ingredients WHERE name = ?`), ingName).Scan(&ingID)
			if err != nil {
				res, err := tx.Exec(sqlDB.Rebind(`INSERT INTO ingredients (name, created_at) VALUES (?, ?)`), ingName, now)
				if err != nil {
					return fmt.Errorf("find or create ingredient: %w", err)
				}
				ingID, _ = res.LastInsertId()
			}
		}

		ri := &models.RecipeIngredient{
			RecipeID:     recipeID,
			IngredientID: ingID,
			Quantity:     ing.Quantity,
			Unit:         ing.Unit,
			UnitWeightG:  ing.UnitWeightG,
			Position:     pos,
		}
		if ing.Descriptor != nil {
			ri.Descriptor = *ing.Descriptor
		}
		if ing.Section != nil {
			ri.Section = *ing.Section
		}
		if v, ok := scaling.ParseQuantity(ing.Quantity); ok {
			ri.QuantityNumeric = &v
		}
		if _, err := tx.Exec(sqlDB.Rebind(
			`INSERT INTO recipe_ingredients
			  (recipe_id, ingredient_id, quantity, quantity_numeric, unit, unit_weight_g, descriptor, section, position)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`),
			ri.RecipeID, ri.IngredientID, ri.Quantity, ri.QuantityNumeric, ri.Unit, ri.UnitWeightG, ri.Descriptor, ri.Section, ri.Position,
		); err != nil {
			return fmt.Errorf("insert recipe ingredient: %w", err)
		}
		if ing.IngredientType != nil && *ing.IngredientType != "" {
			if _, err := tx.Exec(sqlDB.Rebind(`UPDATE ingredients SET ingredient_type = ? WHERE id = ?`),
				*ing.IngredientType, ingID); err != nil {
				return fmt.Errorf("set ingredient type: %w", err)
			}
		}
	}

	// Replace tags.
	if _, err := tx.Exec(sqlDB.Rebind(`DELETE FROM recipe_tags WHERE recipe_id = ?`), recipeID); err != nil {
		return fmt.Errorf("delete recipe tags: %w", err)
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
			tagName := strings.ToLower(strings.TrimSpace(name))
			var tagID int64
			if sqlDB.Driver() == "postgres" {
				err := tx.QueryRow(sqlDB.Rebind(`SELECT id FROM tags WHERE name = ? AND context = ?`), tagName, ctx).Scan(&tagID)
				if err != nil {
					err := tx.QueryRow(sqlDB.Rebind(
						`INSERT INTO tags (name, context) VALUES (?, ?) RETURNING id`),
						tagName, ctx,
					).Scan(&tagID)
					if err != nil {
						return fmt.Errorf("find or create tag: %w", err)
					}
				}
			} else {
				err := tx.QueryRow(sqlDB.Rebind(`SELECT id FROM tags WHERE name = ? AND context = ?`), tagName, ctx).Scan(&tagID)
				if err != nil {
					res, err := tx.Exec(sqlDB.Rebind(`INSERT INTO tags (name, context) VALUES (?, ?)`), tagName, ctx)
					if err != nil {
						return fmt.Errorf("find or create tag: %w", err)
					}
					tagID, _ = res.LastInsertId()
				}
			}
			if sqlDB.Driver() == "postgres" {
				if _, err := tx.Exec(sqlDB.Rebind(
					`INSERT INTO recipe_tags (recipe_id, tag_id) VALUES (?, ?) ON CONFLICT DO NOTHING`),
					recipeID, tagID,
				); err != nil {
					return fmt.Errorf("attach tag: %w", err)
				}
			} else {
				if _, err := tx.Exec(sqlDB.Rebind(
					`INSERT OR IGNORE INTO recipe_tags (recipe_id, tag_id) VALUES (?, ?)`),
					recipeID, tagID,
				); err != nil {
					return fmt.Errorf("attach tag: %w", err)
				}
			}
		}
	}

	return tx.Commit()
}
