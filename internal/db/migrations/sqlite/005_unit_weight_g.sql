-- +goose Up
ALTER TABLE recipe_ingredients ADD COLUMN unit_weight_g REAL;

-- +goose Down
ALTER TABLE recipe_ingredients DROP COLUMN unit_weight_g;
