package services

// SQLiteSchema is the complete DDL for the enplace SQLite database.
// AUTO-GENERATED from migrations — do not edit manually.
// Regenerate after adding migrations: see CLAUDE.md "Schema files".
const SQLiteSchema = `
-- enplace database schema (SQLite)
-- AUTO-GENERATED from migrations/sqlite/*.sql

CREATE TABLE IF NOT EXISTS recipes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
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
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_bread BOOLEAN NOT NULL DEFAULT 0,
    rating INTEGER CHECK(rating BETWEEN 1 AND 5),
    notes TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_recipes_source_url
    ON recipes(source_url COLLATE NOCASE) WHERE source_url != '';

CREATE TABLE IF NOT EXISTS ingredients (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    ingredient_type TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS recipe_ingredients (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    recipe_id INTEGER NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    ingredient_id INTEGER NOT NULL REFERENCES ingredients(id) ON DELETE CASCADE,
    quantity TEXT NOT NULL DEFAULT '',
    quantity_numeric REAL,
    unit TEXT NOT NULL DEFAULT '',
    descriptor TEXT NOT NULL DEFAULT '',
    section TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL DEFAULT 0,
    unit_weight_g REAL
);

CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    context TEXT NOT NULL,
    UNIQUE(name, context)
);

CREATE TABLE IF NOT EXISTS recipe_tags (
    recipe_id INTEGER NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (recipe_id, tag_id)
);

CREATE TABLE IF NOT EXISTS ai_classifier_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    recipe_id INTEGER REFERENCES recipes(id) ON DELETE SET NULL,
    service_class TEXT NOT NULL,
    adapter TEXT NOT NULL DEFAULT '',
    ai_model TEXT NOT NULL DEFAULT '',
    system_prompt TEXT NOT NULL DEFAULT '',
    user_prompt TEXT NOT NULL DEFAULT '',
    raw_response TEXT NOT NULL DEFAULT '',
    success INTEGER NOT NULL DEFAULT 0,
    error_class TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    started_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`
