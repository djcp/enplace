-- +goose Up
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
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recipes_status ON recipes(status);
CREATE INDEX IF NOT EXISTS idx_recipes_created_at ON recipes(created_at);

CREATE TABLE IF NOT EXISTS ingredients (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS recipe_ingredients (
    id BIGSERIAL PRIMARY KEY,
    recipe_id BIGINT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    ingredient_id BIGINT NOT NULL REFERENCES ingredients(id) ON DELETE CASCADE,
    quantity TEXT NOT NULL DEFAULT '',
    unit TEXT NOT NULL DEFAULT '',
    descriptor TEXT NOT NULL DEFAULT '',
    section TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_recipe_ingredients_recipe_id ON recipe_ingredients(recipe_id);
CREATE INDEX IF NOT EXISTS idx_recipe_ingredients_position ON recipe_ingredients(recipe_id, position);

CREATE TABLE IF NOT EXISTS tags (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    context TEXT NOT NULL,
    UNIQUE(name, context)
);

CREATE INDEX IF NOT EXISTS idx_tags_context ON tags(context);

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

CREATE INDEX IF NOT EXISTS idx_ai_runs_recipe_id ON ai_classifier_runs(recipe_id);
CREATE INDEX IF NOT EXISTS idx_ai_runs_created_at ON ai_classifier_runs(created_at);

-- +goose Down
DROP TABLE IF EXISTS ai_classifier_runs;
DROP TABLE IF EXISTS recipe_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS recipe_ingredients;
DROP TABLE IF EXISTS ingredients;
DROP TABLE IF EXISTS recipes;
