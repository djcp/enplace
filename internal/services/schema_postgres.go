package services

// PostgresSchema is the complete DDL for the enplace PostgreSQL database.
// AUTO-GENERATED from migrations/postgres/*.sql — do not edit manually.
// Regenerate after adding migrations: see CLAUDE.md "Schema files".
const PostgresSchema = `
-- enplace database schema (PostgreSQL)
-- AUTO-GENERATED from migrations/postgres/*.sql

CREATE TABLE IF NOT EXISTS recipes (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    directions TEXT NOT NULL DEFAULT '',
    preparation_time INTEGER,
    cooking_time INTEGER,
    servings INTEGER,
    serving_units TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    source_text TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_bread BOOLEAN NOT NULL DEFAULT FALSE,
    rating INTEGER CHECK(rating BETWEEN 1 AND 5),
    notes TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_recipes_source_url
    ON recipes(LOWER(source_url)) WHERE source_url != '';

CREATE TABLE IF NOT EXISTS ingredients (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    ingredient_type TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS recipe_ingredients (
    id BIGSERIAL PRIMARY KEY,
    recipe_id BIGINT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    ingredient_id BIGINT NOT NULL REFERENCES ingredients(id) ON DELETE CASCADE,
    quantity TEXT NOT NULL DEFAULT '',
    quantity_numeric REAL,
    unit TEXT NOT NULL DEFAULT '',
    descriptor TEXT NOT NULL DEFAULT '',
    section TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL DEFAULT 0,
    unit_weight_g REAL
);

CREATE TABLE IF NOT EXISTS tags (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    context TEXT NOT NULL,
    UNIQUE(name, context)
);

CREATE TABLE IF NOT EXISTS recipe_tags (
    recipe_id BIGINT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (recipe_id, tag_id)
);

CREATE TABLE IF NOT EXISTS ai_classifier_runs (
    id BIGSERIAL PRIMARY KEY,
    recipe_id BIGINT REFERENCES recipes(id) ON DELETE SET NULL,
    service_class TEXT NOT NULL,
    adapter TEXT NOT NULL DEFAULT '',
    ai_model TEXT NOT NULL DEFAULT '',
    system_prompt TEXT NOT NULL DEFAULT '',
    user_prompt TEXT NOT NULL DEFAULT '',
    raw_response TEXT NOT NULL DEFAULT '',
    success BOOLEAN NOT NULL DEFAULT FALSE,
    error_class TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`
