-- +goose Up
CREATE INDEX IF NOT EXISTS idx_recipes_updated_at ON recipes(updated_at);
CREATE INDEX IF NOT EXISTS idx_recipe_ingredients_ingredient_id ON recipe_ingredients(ingredient_id);

-- +goose Down
DROP INDEX IF EXISTS idx_recipes_updated_at;
DROP INDEX IF EXISTS idx_recipe_ingredients_ingredient_id;
