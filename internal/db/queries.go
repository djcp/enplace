package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/djcp/gorecipes/internal/models"
	"github.com/jmoiron/sqlx"
)

// RecipeFilter holds search/filter parameters for listing recipes.
type RecipeFilter struct {
	Query              string // name or ingredient substring
	Courses            []string
	CookingMethods     []string
	CulturalInfluences []string
	DietaryRestrictions []string
	StatusFilter       string // empty = published only
}

// --- Recipes ---

// CreateRecipe inserts a new recipe and returns its ID.
func CreateRecipe(db *sqlx.DB, r *models.Recipe) (int64, error) {
	now := time.Now()
	result, err := db.Exec(`
		INSERT INTO recipes (name, description, directions, preparation_time, cooking_time,
		  servings, serving_units, source_url, source_text, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Name, r.Description, r.Directions, r.PreparationTime, r.CookingTime,
		r.Servings, r.ServingUnits, r.SourceURL, r.SourceText, r.Status, now, now,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// UpdateRecipeStatus changes only the status and updated_at fields.
func UpdateRecipeStatus(db *sqlx.DB, id int64, status string) error {
	_, err := db.Exec(
		`UPDATE recipes SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	return err
}

// UpdateRecipeFields applies extracted AI data to an existing recipe.
func UpdateRecipeFields(db *sqlx.DB, r *models.Recipe) error {
	_, err := db.Exec(`
		UPDATE recipes
		SET name = ?, description = ?, directions = ?,
		    preparation_time = ?, cooking_time = ?,
		    servings = ?, serving_units = ?,
		    status = ?, updated_at = ?
		WHERE id = ?`,
		r.Name, r.Description, r.Directions,
		r.PreparationTime, r.CookingTime,
		r.Servings, r.ServingUnits,
		r.Status, time.Now(), r.ID,
	)
	return err
}

// GetRecipe retrieves a recipe by ID with all associations loaded.
func GetRecipe(db *sqlx.DB, id int64) (*models.Recipe, error) {
	var r models.Recipe
	if err := db.Get(&r, `SELECT * FROM recipes WHERE id = ?`, id); err != nil {
		return nil, fmt.Errorf("recipe %d not found: %w", id, err)
	}

	ingredients, err := GetRecipeIngredients(db, id)
	if err != nil {
		return nil, err
	}
	r.Ingredients = ingredients

	tags, err := GetRecipeTags(db, id)
	if err != nil {
		return nil, err
	}
	r.Tags = tags

	return &r, nil
}

// ListRecipes returns recipes matching the filter, newest first.
func ListRecipes(db *sqlx.DB, f RecipeFilter) ([]models.Recipe, error) {
	args := []interface{}{}
	conditions := []string{}

	if f.StatusFilter != "" {
		conditions = append(conditions, "r.status = ?")
		args = append(args, f.StatusFilter)
	} else {
		conditions = append(conditions, "r.status = 'published'")
	}

	if f.Query != "" {
		conditions = append(conditions, `(
			r.name LIKE ? OR EXISTS (
				SELECT 1 FROM recipe_ingredients ri
				JOIN ingredients i ON i.id = ri.ingredient_id
				WHERE ri.recipe_id = r.id AND i.name LIKE ?
			)
		)`)
		q := "%" + f.Query + "%"
		args = append(args, q, q)
	}

	// Tag filters: AND across contexts, OR within a context.
	for ctx, values := range map[string][]string{
		models.TagContextCourses:             f.Courses,
		models.TagContextCookingMethods:      f.CookingMethods,
		models.TagContextCulturalInfluences:  f.CulturalInfluences,
		models.TagContextDietaryRestrictions: f.DietaryRestrictions,
	} {
		if len(values) == 0 {
			continue
		}
		placeholders := strings.Repeat("?,", len(values))
		placeholders = placeholders[:len(placeholders)-1]
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM recipe_tags rt
			JOIN tags t ON t.id = rt.tag_id
			WHERE rt.recipe_id = r.id AND t.context = ? AND t.name IN (%s)
		)`, placeholders))
		args = append(args, ctx)
		for _, v := range values {
			args = append(args, v)
		}
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`SELECT r.* FROM recipes r %s ORDER BY r.created_at DESC`, where)

	var recipes []models.Recipe
	if err := db.Select(&recipes, query, args...); err != nil {
		return nil, err
	}

	// Load tags for all recipes.
	for i := range recipes {
		tags, err := GetRecipeTags(db, recipes[i].ID)
		if err != nil {
			return nil, err
		}
		recipes[i].Tags = tags
	}

	return recipes, nil
}

// --- Ingredients & RecipeIngredients ---

// FindOrCreateIngredient returns the existing ingredient ID or creates a new one.
func FindOrCreateIngredient(db *sqlx.DB, name string) (int64, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	var id int64
	err := db.Get(&id, `SELECT id FROM ingredients WHERE name = ?`, name)
	if err == nil {
		return id, nil
	}
	result, err := db.Exec(
		`INSERT INTO ingredients (name, created_at) VALUES (?, ?)`,
		name, time.Now(),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// InsertRecipeIngredient adds one ingredient line to a recipe.
func InsertRecipeIngredient(db *sqlx.DB, ri *models.RecipeIngredient) error {
	_, err := db.Exec(`
		INSERT INTO recipe_ingredients
		  (recipe_id, ingredient_id, quantity, unit, descriptor, section, position)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ri.RecipeID, ri.IngredientID, ri.Quantity, ri.Unit, ri.Descriptor, ri.Section, ri.Position,
	)
	return err
}

// DeleteRecipeIngredients removes all ingredient lines for a recipe.
func DeleteRecipeIngredients(db *sqlx.DB, recipeID int64) error {
	_, err := db.Exec(`DELETE FROM recipe_ingredients WHERE recipe_id = ?`, recipeID)
	return err
}

// GetRecipeIngredients loads all ingredient lines for a recipe, ordered by section then position.
func GetRecipeIngredients(db *sqlx.DB, recipeID int64) ([]models.RecipeIngredient, error) {
	var ris []models.RecipeIngredient
	err := db.Select(&ris, `
		SELECT ri.*, i.name AS ingredient_name
		FROM recipe_ingredients ri
		JOIN ingredients i ON i.id = ri.ingredient_id
		WHERE ri.recipe_id = ?
		ORDER BY ri.section, ri.position`,
		recipeID,
	)
	return ris, err
}

// --- Tags ---

// FindOrCreateTag returns the existing tag ID or creates a new one.
func FindOrCreateTag(db *sqlx.DB, name, context string) (int64, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	var id int64
	err := db.Get(&id, `SELECT id FROM tags WHERE name = ? AND context = ?`, name, context)
	if err == nil {
		return id, nil
	}
	result, err := db.Exec(
		`INSERT INTO tags (name, context) VALUES (?, ?)`, name, context,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// AttachTag links a tag to a recipe (idempotent).
func AttachTag(db *sqlx.DB, recipeID, tagID int64) error {
	_, err := db.Exec(
		`INSERT OR IGNORE INTO recipe_tags (recipe_id, tag_id) VALUES (?, ?)`,
		recipeID, tagID,
	)
	return err
}

// DeleteRecipeTags removes all tags for a recipe.
func DeleteRecipeTags(db *sqlx.DB, recipeID int64) error {
	_, err := db.Exec(`DELETE FROM recipe_tags WHERE recipe_id = ?`, recipeID)
	return err
}

// GetRecipeTags loads all tags for a recipe.
func GetRecipeTags(db *sqlx.DB, recipeID int64) ([]models.Tag, error) {
	var tags []models.Tag
	err := db.Select(&tags, `
		SELECT t.*
		FROM tags t
		JOIN recipe_tags rt ON rt.tag_id = t.id
		WHERE rt.recipe_id = ?
		ORDER BY t.context, t.name`,
		recipeID,
	)
	return tags, err
}

// AllTagsByContext returns every tag value for a given context (for filter menus).
func AllTagsByContext(db *sqlx.DB, context string) ([]string, error) {
	var names []string
	err := db.Select(&names, `
		SELECT DISTINCT t.name
		FROM tags t
		JOIN recipe_tags rt ON rt.tag_id = t.id
		WHERE t.context = ?
		ORDER BY t.name`,
		context,
	)
	return names, err
}

// --- AI Classifier Runs ---

// CreateAIRun inserts a new (in-progress) AI run record and returns its ID.
func CreateAIRun(db *sqlx.DB, run *models.AIClassifierRun) (int64, error) {
	now := time.Now()
	result, err := db.Exec(`
		INSERT INTO ai_classifier_runs
		  (recipe_id, service_class, adapter, ai_model,
		   system_prompt, user_prompt, success, started_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		run.RecipeID, run.ServiceClass, run.Adapter, run.AIModel,
		run.SystemPrompt, run.UserPrompt, now, now,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// CompleteAIRun marks an existing run as succeeded.
func CompleteAIRun(db *sqlx.DB, id int64, rawResponse string) error {
	now := time.Now()
	_, err := db.Exec(`
		UPDATE ai_classifier_runs
		SET success = 1, raw_response = ?, completed_at = ?
		WHERE id = ?`,
		rawResponse, now, id,
	)
	return err
}

// FailAIRun marks an existing run as failed with error details.
func FailAIRun(db *sqlx.DB, id int64, errClass, errMsg string) error {
	now := time.Now()
	_, err := db.Exec(`
		UPDATE ai_classifier_runs
		SET success = 0, error_class = ?, error_message = ?, completed_at = ?
		WHERE id = ?`,
		errClass, errMsg, now, id,
	)
	return err
}
