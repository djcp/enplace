-- +goose Up
ALTER TABLE recipes ADD COLUMN rating INTEGER CHECK(rating BETWEEN 1 AND 5);
ALTER TABLE recipes ADD COLUMN notes TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE recipes DROP COLUMN notes;
ALTER TABLE recipes DROP COLUMN rating;
