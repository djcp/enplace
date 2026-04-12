-- +goose Up
ALTER TABLE recipe_ingredients ADD COLUMN quantity_numeric REAL;
ALTER TABLE ingredients ADD COLUMN ingredient_type TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE recipe_ingredients DROP COLUMN quantity_numeric;
ALTER TABLE ingredients DROP COLUMN ingredient_type;
