-- +goose Up
ALTER TABLE recipe_ingredients ADD COLUMN IF NOT EXISTS quantity_numeric REAL;
ALTER TABLE ingredients ADD COLUMN IF NOT EXISTS ingredient_type TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE ingredients DROP COLUMN IF EXISTS ingredient_type;
ALTER TABLE recipe_ingredients DROP COLUMN IF EXISTS quantity_numeric;
