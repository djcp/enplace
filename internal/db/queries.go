package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/djcp/enplace/internal/models"
	"github.com/djcp/enplace/internal/scaling"
)

// RecipeFilter holds search/filter parameters for listing recipes.
type RecipeFilter struct {
	Query               string // name or ingredient substring
	Courses             []string
	CookingMethods      []string
	CulturalInfluences  []string
	DietaryRestrictions []string
	StatusFilter        string // empty = all statuses
	IsBread             bool   // false = all recipes; true = bread/dough only
}

// --- Recipes ---

// CreateRecipe inserts a new recipe and returns its ID.
func CreateRecipe(db *DB, r *models.Recipe) (int64, error) {
	now := time.Now()
	return db.insertReturningID(`
		INSERT INTO recipes (name, description, directions, preparation_time, cooking_time,
		  servings, serving_units, is_bread, source_url, source_text, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Name, r.Description, r.Directions, r.PreparationTime, r.CookingTime,
		r.Servings, r.ServingUnits, r.IsBread, r.SourceURL, r.SourceText, r.Status, now, now,
	)
}

// UpdateRecipeStatus changes only the status and updated_at fields.
func UpdateRecipeStatus(db *DB, id int64, status string) error {
	_, err := db.Exec(
		`UPDATE recipes SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	return err
}

// UpdateRecipeFields applies extracted AI data to an existing recipe.
func UpdateRecipeFields(db *DB, r *models.Recipe) error {
	_, err := db.Exec(`
		UPDATE recipes
		SET name = ?, description = ?, directions = ?,
		    preparation_time = ?, cooking_time = ?,
		    servings = ?, serving_units = ?,
		    is_bread = ?,
		    source_url = ?,
		    status = ?, updated_at = ?
		WHERE id = ?`,
		r.Name, r.Description, r.Directions,
		r.PreparationTime, r.CookingTime,
		r.Servings, r.ServingUnits,
		r.IsBread,
		r.SourceURL,
		r.Status, time.Now(), r.ID,
	)
	return err
}

// DeleteRecipe permanently removes a recipe. Related rows in recipe_ingredients
// and recipe_tags are removed automatically via ON DELETE CASCADE.
func DeleteRecipe(db *DB, id int64) error {
	_, err := db.Exec(`DELETE FROM recipes WHERE id = ?`, id)
	return err
}

// GetRecipeByURL returns the recipe whose source_url matches url
// (case-insensitively), or nil if no such recipe exists.
func GetRecipeByURL(sqlDB *DB, url string) (*models.Recipe, error) {
	var r models.Recipe
	var query string
	if sqlDB.Driver() == "postgres" {
		query = `SELECT * FROM recipes WHERE LOWER(source_url) = LOWER($1)`
	} else {
		query = `SELECT * FROM recipes WHERE source_url = ? COLLATE NOCASE`
	}
	err := sqlDB.DB.Get(&r, query, url)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// GetRecipe retrieves a recipe by ID with all associations loaded.
func GetRecipe(db *DB, id int64) (*models.Recipe, error) {
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
func ListRecipes(db *DB, f RecipeFilter) ([]models.Recipe, error) {
	args := []interface{}{}
	conditions := []string{}

	if f.StatusFilter != "" {
		conditions = append(conditions, "r.status = ?")
		args = append(args, f.StatusFilter)
	}

	if f.IsBread {
		conditions = append(conditions, "r.is_bread = 1")
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
func FindOrCreateIngredient(db *DB, name string) (int64, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	var id int64
	err := db.Get(&id, `SELECT id FROM ingredients WHERE name = ?`, name)
	if err == nil {
		return id, nil
	}
	return db.insertReturningID(
		`INSERT INTO ingredients (name, created_at) VALUES (?, ?)`,
		name, time.Now(),
	)
}

// InsertRecipeIngredient adds one ingredient line to a recipe.
func InsertRecipeIngredient(db *DB, ri *models.RecipeIngredient) error {
	_, err := db.Exec(`
		INSERT INTO recipe_ingredients
		  (recipe_id, ingredient_id, quantity, quantity_numeric, unit, unit_weight_g, descriptor, section, position)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ri.RecipeID, ri.IngredientID, ri.Quantity, ri.QuantityNumeric, ri.Unit, ri.UnitWeightG, ri.Descriptor, ri.Section, ri.Position,
	)
	return err
}

// DeleteRecipeIngredients removes all ingredient lines for a recipe.
func DeleteRecipeIngredients(db *DB, recipeID int64) error {
	_, err := db.Exec(`DELETE FROM recipe_ingredients WHERE recipe_id = ?`, recipeID)
	return err
}

// GetRecipeIngredients loads all ingredient lines for a recipe, ordered by section then position.
func GetRecipeIngredients(db *DB, recipeID int64) ([]models.RecipeIngredient, error) {
	var ris []models.RecipeIngredient
	err := db.Select(&ris, `
		SELECT ri.*, i.name AS ingredient_name, i.ingredient_type AS ingredient_type
		FROM recipe_ingredients ri
		JOIN ingredients i ON i.id = ri.ingredient_id
		WHERE ri.recipe_id = ?
		ORDER BY ri.section, ri.position`,
		recipeID,
	)
	return ris, err
}

// SetIngredientType updates the ingredient_type field on the canonical
// ingredients table. Called after AI extraction to persist classification.
func SetIngredientType(db *DB, ingredientID int64, ingredientType string) error {
	_, err := db.Exec(`UPDATE ingredients SET ingredient_type = ? WHERE id = ?`, ingredientType, ingredientID)
	return err
}

// --- Tags ---

// FindOrCreateTag returns the existing tag ID or creates a new one.
func FindOrCreateTag(db *DB, name, context string) (int64, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	var id int64
	err := db.Get(&id, `SELECT id FROM tags WHERE name = ? AND context = ?`, name, context)
	if err == nil {
		return id, nil
	}
	return db.insertReturningID(
		`INSERT INTO tags (name, context) VALUES (?, ?)`, name, context,
	)
}

// AttachTag links a tag to a recipe (idempotent).
func AttachTag(db *DB, recipeID, tagID int64) error {
	_, err := db.Exec(
		db.onConflictDoNothing(`INSERT INTO recipe_tags (recipe_id, tag_id) VALUES (?, ?)`),
		recipeID, tagID,
	)
	return err
}

// DeleteRecipeTags removes all tags for a recipe.
func DeleteRecipeTags(db *DB, recipeID int64) error {
	_, err := db.Exec(`DELETE FROM recipe_tags WHERE recipe_id = ?`, recipeID)
	return err
}

// GetRecipeTags loads all tags for a recipe.
func GetRecipeTags(db *DB, recipeID int64) ([]models.Tag, error) {
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

// AllIngredientNames returns every ingredient name in alphabetical order.
func AllIngredientNames(db *DB) ([]string, error) {
	var names []string
	err := db.Select(&names, `SELECT name FROM ingredients ORDER BY name`)
	return names, err
}

// AllUnits returns every distinct unit used in recipe ingredients, alphabetically.
func AllUnits(db *DB) ([]string, error) {
	var units []string
	err := db.Select(&units,
		`SELECT DISTINCT unit FROM recipe_ingredients WHERE unit != '' ORDER BY unit`)
	return units, err
}

// SaveRecipe creates (r.ID==0) or updates (r.ID>0) a recipe with its tags and
// ingredients. r.Ingredients[*].IngredientName must be set; IDs are ignored.
func SaveRecipe(db *DB, r *models.Recipe, tagNames map[string][]string) error {
	if r.ID == 0 {
		id, err := CreateRecipe(db, r)
		if err != nil {
			return err
		}
		r.ID = id
	} else {
		if err := UpdateRecipeFields(db, r); err != nil {
			return err
		}
	}
	if err := DeleteRecipeTags(db, r.ID); err != nil {
		return err
	}
	for ctx, names := range tagNames {
		for _, name := range names {
			if name == "" {
				continue
			}
			tagID, err := FindOrCreateTag(db, name, ctx)
			if err != nil {
				return err
			}
			if err := AttachTag(db, r.ID, tagID); err != nil {
				return err
			}
		}
	}
	if err := DeleteRecipeIngredients(db, r.ID); err != nil {
		return err
	}
	for pos, ing := range r.Ingredients {
		if ing.IngredientName == "" {
			continue
		}
		ingID, err := FindOrCreateIngredient(db, ing.IngredientName)
		if err != nil {
			return err
		}
		ing.RecipeID = r.ID
		ing.IngredientID = ingID
		ing.Position = pos
		if v, ok := scaling.ParseQuantity(ing.Quantity); ok {
			ing.QuantityNumeric = &v
		}
		if err := InsertRecipeIngredient(db, &ing); err != nil {
			return err
		}
		if ing.IngredientType != "" {
			if err := SetIngredientType(db, ingID, ing.IngredientType); err != nil {
				return err
			}
		}
	}
	return nil
}

// AllTagsByContext returns every tag value for a given context (for filter menus).
func AllTagsByContext(db *DB, context string) ([]string, error) {
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

// RecipeCount returns the number of recipes in the database.
// Used to decide whether to offer migration.
func RecipeCount(db *DB) (int, error) {
	var count int
	err := db.Get(&count, `SELECT COUNT(*) FROM recipes`)
	return count, err
}

// --- AI Classifier Runs ---

// CreateAIRun inserts a new (in-progress) AI run record and returns its ID.
func CreateAIRun(db *DB, run *models.AIClassifierRun) (int64, error) {
	now := time.Now()
	return db.insertReturningID(`
		INSERT INTO ai_classifier_runs
		  (recipe_id, service_class, adapter, ai_model,
		   system_prompt, user_prompt, success, started_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		run.RecipeID, run.ServiceClass, run.Adapter, run.AIModel,
		run.SystemPrompt, run.UserPrompt, now, now,
	)
}

// CompleteAIRun marks an existing run as succeeded.
func CompleteAIRun(db *DB, id int64, rawResponse string) error {
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
func FailAIRun(db *DB, id int64, errClass, errMsg string) error {
	now := time.Now()
	_, err := db.Exec(`
		UPDATE ai_classifier_runs
		SET success = 0, error_class = ?, error_message = ?, completed_at = ?
		WHERE id = ?`,
		errClass, errMsg, now, id,
	)
	return err
}
