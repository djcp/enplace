# gorecipes

A CLI recipe manager that captures recipes from URLs or pasted text and uses Claude AI to extract structured data — ingredients, directions, timing, and classification tags — stored locally in SQLite.

## Features

- **Add by URL** — fetch any recipe page; schema.org JSON-LD is parsed first with an HTML fallback
- **Add by paste** — pipe or interactively paste raw recipe text
- **AI extraction** — Claude parses free-form text into a structured recipe: named ingredients with quantity, unit, descriptor, and section; numbered directions; prep/cook time; servings; and four classification tag contexts (courses, cooking methods, cultural influences, dietary restrictions)
- **Interactive browser** — full-screen recipe list with live `/` search and keyboard navigation
- **Styled output** — ingredient tables, markdown-rendered directions, tag pills, and timing summaries in the terminal
- **Onboarding** — prompts for an Anthropic API key on first run and stores it at `~/.config/gorecipes/config.json`
- **Audit trail** — every AI call is recorded with its prompt, raw response, duration, and success/failure status
- **No external dependencies at runtime** — single static binary; SQLite is compiled in with no CGO requirement

## Commands

```
gorecipes                    Open the interactive recipe browser (default)
gorecipes add <url>          Add a recipe from a URL
gorecipes add --paste        Add a recipe from pasted text
gorecipes list               Open the interactive recipe browser
gorecipes list --query foo   Non-interactive filtered list (also works when stdout is not a TTY)
gorecipes show <id>          Display a recipe by ID
gorecipes config             View or update configuration (API key, model)
```

### add

Runs a three-step pipeline shown as inline progress:

```
  ✓ Fetching recipe content
  ⠋ Extracting with AI (claude-haiku-4-5-20251001)
  ○ Saving to database
```

On completion the recipe is printed in full. On failure the status is set to `processing_failed` and the recipe is preserved with its ID for inspection.

### list

Opens a full-screen browser:

- **`↑` / `↓`** or **`j` / `k`** — navigate
- **`/`** — type to filter by name or ingredient
- **`enter`** — view recipe detail
- **`q`** or **`esc`** — quit

Falls back to a plain table when stdout is not a TTY or `--query` is set.

### config

Displays the current API key (masked), model, database path, and config file location. Lets you switch between Claude models (Haiku, Sonnet, Opus).

## Building

Requires Go 1.21+. No C compiler needed.

```sh
git clone ...
cd gorecipes
go build -o gorecipes .
```

Install to your PATH:

```sh
go install .
```

## Running tests

```sh
go test ./...
```

With the race detector (recommended):

```sh
go test -race ./...
```

Tests use an in-memory SQLite database and a mock `AIClient` interface — no API key or network access required.

## Configuration

On first run, `gorecipes` prompts for an Anthropic API key and writes:

```
~/.config/gorecipes/config.json   — API key, model name, database path
~/.local/share/gorecipes/         — SQLite database directory
```

Both paths follow the XDG Base Directory spec. Set `XDG_CONFIG_HOME` or `XDG_DATA_HOME` to override.

The model defaults to `claude-haiku-4-5-20251001`. To use a more capable model:

```sh
gorecipes config
```

Or edit `config.json` directly and set `"anthropic_model"` to any Claude model ID.

## Data model

The SQLite schema mirrors the [milk_steak](https://github.com/djcp/milk_steak) Rails app it was designed alongside.

| Table | Purpose |
|---|---|
| `recipes` | Core recipe data: name, description, directions, timing, servings, status, source URL/text |
| `ingredients` | Canonical ingredient dictionary (lowercase, deduplicated) |
| `recipe_ingredients` | Join table with quantity, unit, descriptor, section, and position |
| `tags` | Tag values scoped by context |
| `recipe_tags` | Recipe-to-tag associations |
| `ai_classifier_runs` | Audit log for every AI pipeline call |

### Recipe status workflow

```
draft → processing → review → published
                  ↘ processing_failed
```

The CLI skips the `review` step and publishes immediately after successful extraction.

### Tag contexts

- `courses` — dinner, dessert, breakfast, etc.
- `cooking_methods` — bake, sauté, grill, etc.
- `cultural_influences` — italian, thai, mexican, etc.
- `dietary_restrictions` — vegetarian, vegan, gluten-free, etc.

## AI extraction

The extraction pipeline has three stages, each recorded as an `ai_classifier_runs` row:

1. **TextExtractor** (`internal/services/text_extractor.go`) — fetches the URL with redirect following, strips navigation/ads/scripts, extracts schema.org Recipe JSON-LD if present, otherwise falls back to `article`, `main`, `[role=main]`, and similar content selectors. Truncates to 15,000 characters before passing to the AI.

2. **AIExtractor** (`internal/services/ai_extractor.go`) — sends the cleaned text to Claude with a detailed system prompt that specifies canonical ingredient naming, descriptor encoding for prep methods and ingredient alternatives, section grouping, quantity formatting (maximum 10 characters), and tag classification rules. Returns a typed `ExtractedRecipe` struct parsed from the JSON response.

3. **AIApplier** (`internal/services/ai_applier.go`) — writes the extracted data to SQLite: find-or-create for ingredients and tags, replace-on-update for ingredient lines and tag associations, and a status transition to `published`.

## Internal library choices

### [Cobra](https://github.com/spf13/cobra)
Standard Go CLI framework. Handles subcommands, flags, and `--help` output with minimal boilerplate.

### [Charmbracelet / Bubbletea](https://github.com/charmbracelet/bubbletea)
Elm-architecture TUI framework. Used for the interactive recipe browser and the add-command progress display. The `Msg`/`Update`/`View` pattern keeps UI state immutable and easily testable.

### [Charmbracelet / Huh](https://github.com/charmbracelet/huh)
Form and prompt library built on Bubbletea. Used for all interactive input: API key onboarding, URL prompts, text paste input, and config selection menus. Provides validation callbacks so errors surface inline before submission.

### [Charmbracelet / Lipgloss](https://github.com/charmbracelet/lipgloss)
Declarative terminal styling — colors, borders, padding, width constraints. Drives the recipe detail view, status badges, tag pills, and the shared style palette in `internal/ui/styles.go`.

### [Charmbracelet / Glamour](https://github.com/charmbracelet/glamour)
Renders Markdown to styled terminal output. Used to display recipe directions, which Claude returns as numbered Markdown steps.

### [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)
A pure-Go SQLite driver transpiled from C using `cgo`-free techniques. The entire SQLite engine is compiled into the binary — no system library, no CGO, no build toolchain dependency beyond the Go compiler. WAL journal mode and foreign key enforcement are enabled at connection time.

### [sqlx](https://github.com/jmoiron/sqlx)
Thin extension to `database/sql` that adds struct scanning (`db.Get`, `db.Select`) and named parameter support. Keeps queries in plain SQL in `internal/db/queries.go` without a full ORM.

### [Anthropic Go SDK](https://github.com/anthropics/anthropic-sdk-go)
Official SDK for the Anthropic Messages API. The `AIClient` interface in `internal/services/ai_extractor.go` wraps the SDK's `Complete` call, which is what allows tests to inject a `mockAIClient` without making real API calls.

### [golang.org/x/net/html](https://pkg.go.dev/golang.org/x/net/html)
The standard Go HTML parser from the `x/net` extended library. Used in `TextExtractor` to walk the DOM, strip noise nodes, and extract recipe content without pulling in a third-party HTML library.
