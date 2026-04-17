-- +goose Up
ALTER TABLE recipe_ingredients ADD COLUMN IF NOT EXISTS unit_weight_g REAL;

-- +goose Down
ALTER TABLE recipe_ingredients DROP COLUMN IF EXISTS unit_weight_g;
