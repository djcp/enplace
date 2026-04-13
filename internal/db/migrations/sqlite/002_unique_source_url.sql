-- +goose Up
-- Enforce URL uniqueness, case-insensitively. Empty source_url is excluded
-- (paste-mode and manual recipes all have source_url = '' and must be allowed
-- in multiples). NOCASE covers ASCII only, which is sufficient for URLs.
CREATE UNIQUE INDEX IF NOT EXISTS idx_recipes_source_url
    ON recipes(source_url COLLATE NOCASE) WHERE source_url != '';
