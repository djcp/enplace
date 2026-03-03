# gorecipes

A CLI recipe manager that captures recipes from URLs or pasted text and uses Claude AI to extract structured data — ingredients, directions, timing, and classification tags — stored locally in SQLite.

## Features

- **Add by URL** — fetch any recipe page; schema.org JSON-LD is parsed first with an HTML fallback
- **Add by paste** — pipe or interactively paste raw recipe text
- **Add manually** — fill in a full-screen form with autocomplete for ingredients, units, and tags
- **AI extraction** — Claude parses free-form text into a structured recipe: named ingredients with quantity, unit, descriptor, and section; numbered directions; prep/cook time; servings; and four classification tag contexts (courses, cooking methods, cultural influences, dietary restrictions)
- **Edit recipes** — open a pre-populated form from the list or detail view with `e`; supports the same autocomplete as manual entry
- **Print preview & export** — `p` in the detail view opens a full-screen preview with options to save as PDF, RTF, Markdown, or plain text to `~/Downloads/`, or send directly to the system printer via CUPS (`lp`/`lpr`); duplicate filenames are deduplicated automatically with a `-2`, `-3`, … suffix
- **Interactive browser** — full-screen recipe list with live `/` search and keyboard navigation
- **Styled output** — ingredient tables, markdown-rendered directions, tag pills, and timing summaries in the terminal
- **Data management** — `m` from the list or detail view opens a manage screen for cleaning up tags (rename, merge, delete by context), ingredients (rename, merge), and serving units (rename, merge); also browses AI run history with individual delete and bulk prune of runs older than 30 days
- **Quiet/scripted mode** — `add --quiet <url>` runs the pipeline silently and exits non-zero with an error on stderr on failure
- **Onboarding** — prompts for an Anthropic API key on first run and stores it at `~/.config/gorecipes/config.json`
- **Audit trail** — every AI call is recorded with its prompt, raw response, duration, and success/failure status; browsable and manageable via the manage screen
- **No external dependencies at runtime** — single static binary; SQLite is compiled in with no CGO requirement

## Commands

```
gorecipes                          Open the interactive recipe browser (default)
gorecipes add                      Choose how to add: URL, paste, or manual form
gorecipes add <url>                Add a recipe from a URL
gorecipes add --paste              Add a recipe from pasted text
gorecipes add --quiet <url>        Extract and save silently (for scripting)
gorecipes list                     Open the interactive recipe browser
gorecipes list --query foo         Non-interactive filtered list (also when stdout is not a TTY)
gorecipes show <id>                Display a recipe by ID
gorecipes config                   View or update configuration (API key, model)
```

### add

When run without a URL argument or `--paste`, a mode-selection screen appears:

```
  How would you like to add this recipe?

  ▶ From a URL
    Paste recipe text
    Enter manually
```

**URL / paste modes** run a three-step pipeline shown as inline progress:

```
  ✓ Fetching recipe content
  ⠋ Extracting with AI (claude-haiku-4-5-20251001)
  ○ Saving to database
```

On completion the recipe detail view opens. On pipeline failure the status is set to `processing_failed` and the recipe is preserved for inspection.

**Manual mode** (`Enter manually` or `gorecipes add` → select the option) opens the edit form with all fields blank.

**Quiet mode** (`-q` / `--quiet`) requires a URL argument, runs the pipeline with no TUI, and produces no output on success. On failure it exits with code 1 and writes the error to stderr — useful for automation:

```sh
gorecipes add -q https://example.com/recipe && echo "saved"
```

### list / browser

Opens a full-screen browser:

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate |
| `/` | Filter by name or ingredient (press Enter to confirm) |
| `enter` | Open recipe detail |
| `e` | Edit the selected recipe |
| `d` | Delete (with confirmation) |
| `a` | Add a new recipe |
| `m` | Open manage |
| `h` | Clear filter and go home |
| `q` / `esc` | Quit |

When the database is empty a centered prompt appears with instructions for adding a first recipe.

Falls back to a plain table when stdout is not a TTY or `--query` is set.

### Recipe detail view

| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Scroll |
| `/` | Search (carries the query back to the list on `h`) |
| `e` | Edit this recipe |
| `p` | Open print preview / export |
| `a` | Add a new recipe |
| `d` | Delete (with confirmation) |
| `m` | Open manage |
| `r` | Retry AI extraction (only shown when status is `processing_failed`) |
| `h` | Go back to the list |
| `q` / `esc` | Quit |

### Print preview

Opened with `p` from the detail view. Shows a plain-text rendering of the recipe in a scrollable full-screen view.

| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Scroll |
| `s` | Open the export format chooser |
| `p` | Send to system printer immediately |
| `esc` / `q` | Return to the detail view |

**Export formats** (chosen via `s`):

| Format | Saved to |
|--------|----------|
| PDF (`.pdf`) | `~/Downloads/<recipe-slug>.pdf` |
| Rich Text (`.rtf`) | `~/Downloads/<recipe-slug>.rtf` |
| Markdown (`.md`) | `~/Downloads/<recipe-slug>.md` |
| Plain Text (`.txt`) | `~/Downloads/<recipe-slug>.txt` |
| Print to printer | Sent via `lp` / `lpr` (CUPS) |

If the target filename already exists, a numeric suffix is appended before the extension: `chocolate-chip-cookies-2.pdf`, `chocolate-chip-cookies-3.pdf`, and so on. The full path of the saved file is shown in a confirmation overlay after each export.

### Manage

Opened with `m` from the list or detail view. A landing screen with five sections navigated by `↑`/`↓`/`j`/`k`; `enter` opens the selected section, `esc` returns to where you came from.

| Section | What you can do |
|---------|-----------------|
| Configure | Update API key, AI model, and credits on exported recipes |
| Tags | Browse tags by context; rename, merge, or delete |
| Ingredients | Search and browse ingredients with usage counts; rename or merge |
| Serving Units | Browse serving units with usage counts; rename or merge |
| AI Classifier Runs | View extraction history; delete individual runs or bulk-prune runs older than 30 days |

**Tags** — pick a context (courses, cooking methods, cultural influences, dietary restrictions), then browse the tag list. `e` to rename in-place, `m` to merge into another tag (recipe associations are repointed, source tag is deleted), `d` to delete with confirmation.

**Ingredients** — `/` focuses the search bar for a client-side filter; `e` renames, `m` merges (all `recipe_ingredients` rows are repointed to the target, source ingredient row is deleted).

**Serving Units** — same rename/merge flow; units are inline strings in `recipe_ingredients.unit`, so merge is a bulk `UPDATE` with no orphan row cleanup needed.

**AI Classifier Runs** — scrollable list showing date, service, model, success/failure, duration, and recipe name. `enter` opens a scrollable detail view with the full system prompt, user prompt, and raw AI response (with humanized timestamps and timezone). `r` in the detail view triggers a retry of AI extraction for the associated recipe — available for any run tied to an existing recipe (not limited to failed runs). `d` deletes an individual run with a brief inline confirmation overlay; the list shows a notice on return. `p` prompts to prune all runs older than 30 days and displays the count deleted.

### Edit form

Accessible via `e` from the list or detail view, or via "Enter manually" in the add flow. The form supports all recipe fields:

- Name, status (draft / review / published), description
- Prep time, cook time, servings, serving units, source URL
- Tag pills for each context (courses, cooking methods, cultural influences, dietary restrictions)
- Ingredient rows (quantity, unit, name, descriptor, section) with unlimited rows
- Directions (Markdown)

**Navigation:**

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` | Move between fields |
| `↑` / `↓` (ingredient grid) | Move between ingredient rows |
| `ctrl+a` | Add an ingredient row |
| `ctrl+d` | Remove the current ingredient row |
| `enter` (tag field) | Add the typed text as a tag pill |
| `backspace` on empty tag input | Remove the last tag pill |
| `◄` / `►` or `h` / `l` (status) | Cycle status |
| `ctrl+s` | Save |
| `esc` | Cancel without saving |

**Autocomplete** is available on ingredient name and unit fields, and on each tag input, using values already in the database. Press `tab` to accept a suggestion; if no suggestion is active `tab` advances to the next field instead.

### config

Displays and edits the current configuration: API key (masked), AI model, and export credits line. The model cycles with `◄`/`►`; `ctrl+s` saves, `esc` cancels. Database path and config file location are shown below the editable fields.

Also accessible from the interactive browser via `m` → **Configure**.

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

The CLI skips the `review` step and publishes immediately after successful extraction. Manually created or edited recipes can be set to any status directly in the edit form.

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
Elm-architecture TUI framework. Used for the interactive recipe browser, add-command progress display, and edit form. The `Msg`/`Update`/`View` pattern keeps UI state immutable and easily testable.

### [Charmbracelet / Bubbles](https://github.com/charmbracelet/bubbles)
Reusable Bubbletea components. `textinput` and `textarea` drive the edit form fields, including inline autocomplete suggestions for ingredients, units, and tags.

### [Charmbracelet / Huh](https://github.com/charmbracelet/huh)
Form and prompt library built on Bubbletea. Used for API key onboarding and config selection menus.

### [Charmbracelet / Lipgloss](https://github.com/charmbracelet/lipgloss)
Declarative terminal styling — colors, borders, padding, width constraints. Drives the recipe detail view, edit form, status badges, tag pills, and the shared style palette in `internal/ui/styles.go`.

### [Charmbracelet / Glamour](https://github.com/charmbracelet/glamour)
Renders Markdown to styled terminal output. Used to display recipe directions, which Claude returns as numbered Markdown steps.

### [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)
A pure-Go SQLite driver transpiled from C using `cgo`-free techniques. The entire SQLite engine is compiled into the binary — no system library, no CGO, no build toolchain dependency beyond the Go compiler. WAL journal mode and foreign key enforcement are enabled at connection time.

### [sqlx](https://github.com/jmoiron/sqlx)
Thin extension to `database/sql` that adds struct scanning (`db.Get`, `db.Select`) and named parameter support. Keeps queries in plain SQL in `internal/db/queries.go` without a full ORM.

### [Anthropic Go SDK](https://github.com/anthropics/anthropic-sdk-go)
Official SDK for the Anthropic Messages API. The `AIClient` interface in `internal/services/ai_extractor.go` wraps the SDK's `Complete` call, which is what allows tests to inject a `mockAIClient` without making real API calls.

### [go-pdf/fpdf](https://github.com/go-pdf/fpdf)
Pure-Go PDF generation library (a maintained fork of gofpdf). Used in `internal/export/pdf.go` to render recipe PDFs using the built-in Helvetica core font — no font files to embed, no CGO requirement. Text is translated from UTF-8 to cp1252 via `UnicodeTranslatorFromDescriptor` before being passed to the library, which is required for correct rendering of characters such as `°` and `•` with core fonts.

### [golang.org/x/net/html](https://pkg.go.dev/golang.org/x/net/html)
The standard Go HTML parser from the `x/net` extended library. Used in `TextExtractor` to walk the DOM, strip noise nodes, and extract recipe content without pulling in a third-party HTML library.

## License

[MIT](LICENSE)
