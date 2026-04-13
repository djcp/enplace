-- +goose Up
-- Enforce URL uniqueness case-insensitively using a functional index on LOWER().
-- Empty source_url is excluded (paste-mode and manual recipes all have source_url = ''
-- and must be allowed in multiples).
CREATE UNIQUE INDEX IF NOT EXISTS idx_recipes_source_url
    ON recipes(LOWER(source_url)) WHERE source_url != '';

-- +goose Down
DROP INDEX IF EXISTS idx_recipes_source_url;
