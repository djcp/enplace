-- +goose Up
ALTER TABLE recipes ADD COLUMN is_bread BOOLEAN NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE recipes DROP COLUMN is_bread;
