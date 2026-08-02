# enplace

[![Latest Release](https://img.shields.io/github/v/release/djcp/enplace?include_prereleases)](https://github.com/djcp/enplace/releases/latest)
[![Build Status](https://github.com/djcp/enplace/actions/workflows/go.yml/badge.svg)](https://github.com/djcp/enplace/actions/workflows/go.yml)

A CLI recipe manager that captures recipes from URLs or pasted text and uses Claude AI to extract structured data — ingredients, directions, timing, and classification tags — stored locally in SQLite.

The UI adapts to your terminal's color scheme.

| Recipe list | Recipe detail |
|---|---|
| ![Recipe list — dark](screenshots/dark-list.svg) | ![Recipe detail — dark](screenshots/dark-detail.svg) |
| ![Recipe list — light](screenshots/light-list.svg) | ![Recipe detail — light](screenshots/light-detail.svg) |

## Features

- **Add by URL** — fetch any recipe page; schema.org JSON-LD is parsed first with an HTML fallback
- **Add by paste** — pipe or interactively paste raw recipe text
- **Add manually** — fill in a full-screen form with autocomplete for ingredients, units, and tags
- **AI extraction** — Claude parses free-form text into a structured recipe: named ingredients with quantity, unit, descriptor, and section; numbered directions; prep/cook time; servings; and four classification tag contexts (courses, cooking methods, cultural influences, dietary restrictions). The AI also classifies each ingredient as `flour`, `dry`, `wet`, `fat`, or `starter`, and marks bread and dough recipes with an `is_bread` flag
- **Bread/dough hydration and baker's percentages** — recipes marked as bread or dough automatically show hydration percentage and a full baker's percentages table. Hydration = total wet ÷ total dry × 100, where "dry" includes flour, salt, yeast, seeds, and all other non-fat solids. Baker's percentages use total flour weight as the 100% base (the standard baker's math definition), so every other ingredient is expressed relative to flour. Saturated fats (butter, lard, etc.) appear in the baker's percentages table but are excluded from the hydration ratio. Sourdough starters and levains are split 50/50 between wet and dry (assuming a 100% hydration starter). These figures appear in the detail view, the scale view, and all export formats (PDF, RTF, Markdown, plain text)
- **Edit recipes** — open a pre-populated form from the list or detail view with `e`; supports the same autocomplete as manual entry
- **Print preview & export** — `p` in the detail view opens a full-screen preview with options to save as PDF, RTF, Markdown, or plain text to `~/Downloads/`, or send directly to the system printer via CUPS (`lp`/`lpr`); duplicate filenames are deduplicated automatically with a `-2`, `-3`, … suffix
- **Interactive browser** — full-screen recipe list with live `/` search and keyboard navigation
- **Styled output** — ingredient tables, markdown-rendered directions, tag pills, and timing summaries in the terminal
- **Data management** — `m` from the list or detail view opens a manage screen for cleaning up tags (rename, merge, delete by context), ingredients (rename, merge), and serving units (rename, merge); also browses AI run history with individual delete and bulk prune of runs older than 30 days
- **Quiet/scripted mode** — `add --quiet <url>` runs the pipeline silently and exits non-zero with an error on stderr on failure
- **Onboarding** — prompts for an Anthropic API key on first run and stores it at `~/.config/enplace/config.json`
- **Audit trail** — every AI call is recorded with its prompt, raw response, duration, and success/failure status; browsable and manageable via the manage screen
- **Ratings and personal notes** — press `r` in the detail view to rate a recipe 1–5 stars using a selector menu; press `N` to open a freeform notes field. Ratings appear inline in the list view (★★★★☆) and a 📝 indicator appears when notes exist. The rating value is included in all export formats; notes are personal and never exported
- **No external dependencies at runtime** — single static binary; SQLite is compiled in with no CGO requirement

## Installation

enplace ships as a single, statically-linked binary — no runtime dependencies, no C compiler, no separate SQLite install. Every method below (except `go install`) fetches the same prebuilt binary from a [GitHub release](https://github.com/djcp/enplace/releases): the release archives are cross-compiled by [GoReleaser](https://goreleaser.com) and published alongside a `checksums.txt`, and each installer downloads the archive matching your OS/architecture, **verifies its SHA-256 against `checksums.txt`**, and unpacks the binary onto your `PATH`.

Pick whichever method fits your platform. Where each one puts the binary — and where your recipes and config live — is summarized in [Where things are stored](#where-things-are-stored).

### Homebrew (macOS & Linux)

```sh
brew install djcp/tap/enplace
```

Installs from the [`djcp/homebrew-tap`](https://github.com/djcp/homebrew-tap) tap as a Homebrew **cask**. Homebrew downloads the release archive, verifies its checksum, stages it under the Caskroom, and symlinks `enplace` into your Homebrew `bin`. Upgrade with `brew upgrade enplace`.

### Scoop (Windows)

```powershell
scoop bucket add djcp https://github.com/djcp/scoop-bucket
scoop install enplace
```

Installs from the [`djcp/scoop-bucket`](https://github.com/djcp/scoop-bucket) bucket. Scoop downloads and checksum-verifies the release zip, unpacks it under `~/scoop/apps/enplace`, and adds a shim to `~/scoop/shims` (already on your `PATH`). Upgrade with `scoop update enplace`.

### Install script (Linux & macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/djcp/enplace/main/install.sh | sh
```

The script detects your OS (`uname -s`) and architecture (`uname -m`), resolves the latest release tag via the GitHub API, downloads the matching `.tar.gz`, verifies its SHA-256 against `checksums.txt`, and installs `enplace` to `~/.local/bin`. Environment overrides:

- `INSTALL_DIR=/usr/local/bin` — install somewhere else (e.g. a system-wide, already-on-`PATH` directory).
- `ENPLACE_VERSION=v1.5.0` — pin a specific release instead of the latest.

If the target directory isn't on your `PATH`, the script prints the line to add to your shell profile.

### Install script (Windows, PowerShell)

```powershell
irm https://raw.githubusercontent.com/djcp/enplace/main/install.ps1 | iex
```

Detects your architecture, resolves the latest release, downloads and checksum-verifies the matching `.zip`, unpacks `enplace.exe` to `%LOCALAPPDATA%\Programs\enplace`, and adds that directory to your **user** `PATH` (restart your shell to pick it up). Override with `$env:INSTALL_DIR` and `$env:ENPLACE_VERSION`.

### From source

Requires Go 1.21+ (no C compiler needed). Unlike the other methods this compiles locally rather than downloading a release binary, and installs to your Go bin directory (`$GOBIN`, or `$GOPATH/bin` — typically `~/go/bin`):

```sh
go install github.com/djcp/enplace@latest
```

### Where things are stored

**The binary** — location depends on how you installed it:

| Install method | Binary location |
|---|---|
| Homebrew (Apple Silicon) | `/opt/homebrew/bin/enplace` → `…/Caskroom/enplace/<version>/enplace` |
| Homebrew (Intel macOS) | `/usr/local/bin/enplace` → `…/Caskroom/enplace/<version>/enplace` |
| Scoop | `~/scoop/shims/enplace.exe` → `~/scoop/apps/enplace/current/enplace.exe` |
| Install script (Unix) | `~/.local/bin/enplace` (or `$INSTALL_DIR`) |
| Install script (Windows) | `%LOCALAPPDATA%\Programs\enplace\enplace.exe` (or `$env:INSTALL_DIR`) |
| `go install` | `$GOBIN` or `$GOPATH/bin` (usually `~/go/bin/enplace`) |

**Your data** — the same regardless of install method. enplace follows the [XDG Base Directory](https://specifications.freedesktop.org/basedir-spec/latest/) spec:

| What | Path (Linux/macOS) | Path (Windows) |
|---|---|---|
| Config (`config.json`) | `~/.config/enplace/` | `%USERPROFILE%\.config\enplace\` |
| Database (`recipes.db`) + log (`enplace.log`) | `~/.local/share/enplace/` | `%USERPROFILE%\.local\share\enplace\` |

Set `XDG_CONFIG_HOME` / `XDG_DATA_HOME` to relocate these. Uninstalling the binary never touches this data — see [Uninstalling](#uninstalling). (If you've configured a PostgreSQL backend, your recipes live in that database instead of `recipes.db`.)

### Updating

```sh
enplace update            # download & install the latest release
enplace update --check    # report whether a newer version exists
```

If enplace was installed with Homebrew or Scoop, `enplace update` detects this and tells you to run `brew upgrade enplace` / `scoop update enplace` instead of replacing the package-managed binary.

### Uninstalling

Save your recipes first if you want to keep them:

```sh
enplace export            # writes ~/enplace_recipe_backup_<date>.json by default
```

Then remove enplace the same way you installed it:

- **Homebrew:** `brew uninstall enplace`
- **Scoop:** `scoop uninstall enplace`
- **Install script / source:** delete the `enplace` binary from its [install location](#where-things-are-stored)

Removing the binary never deletes your recipes, config, or logs. To remove those too, delete the two data directories by hand (`~/.config/enplace` and `~/.local/share/enplace`, or their `%USERPROFILE%\…` equivalents on Windows — see [Where things are stored](#where-things-are-stored)). A PostgreSQL database, if configured, is left untouched.

## Commands

```
enplace                          Open the interactive recipe browser (default)
enplace add                      Choose how to add: URL, paste, or manual form
enplace add <url>                Add a recipe from a URL
enplace add --paste              Add a recipe from pasted text
enplace add --quiet <url>        Extract and save silently (for scripting)
enplace list                     Open the interactive recipe browser
enplace list --query foo         Non-interactive filtered list (also when stdout is not a TTY)
enplace show <id>                Display a recipe by ID
enplace config                   View or update configuration (API key, model)
enplace export                   Export all recipes to one JSON or text file
enplace update                   Update enplace to the latest release
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

**Manual mode** (`Enter manually` or `enplace add` → select the option) opens the edit form with all fields blank.

**Quiet mode** (`-q` / `--quiet`) requires a URL argument, runs the pipeline with no TUI, and produces no output on success. On failure it exits with code 1 and writes the error to stderr — useful for automation:

```sh
enplace add -q https://example.com/recipe && echo "saved"
```

### list / browser

Opens a full-screen browser:

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate |
| `/` | Open the filter pane (press Enter to confirm, Esc to cancel) |
| `enter` | Open recipe detail |
| `e` | Edit the selected recipe |
| `d` | Delete (with confirmation) |
| `a` | Add a new recipe |
| `m` | Open manage |
| `h` | Clear filter and go home |
| `q` / `esc` | Quit |

The filter pane (right side) supports text search, course and cultural-influence tag filters, status, a **Recipe type** toggle, and a **Min rating** selector (★☆☆☆☆+ through ★★★★★) — so you can narrow to e.g. "Italian recipes rated 4 or above" in one step. Bread recipes are marked with a 🍞 prefix; rated recipes show their star glyphs inline.

When the database is empty a centered prompt appears with instructions for adding a first recipe.

Falls back to a plain table when stdout is not a TTY or `--query` is set.

### Recipe detail view

| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Scroll |
| `/` | Search (carries the query back to the list on `h`) |
| `e` | Edit this recipe |
| `p` | Open print preview / export |
| `s` | Open ingredient scaling |
| `a` | Add a new recipe |
| `d` | Delete (with confirmation) |
| `m` | Open manage |
| `r` | Rate this recipe (1–5 stars) — or retry AI extraction when status is `processing_failed` |
| `N` | Open / edit personal notes |
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

### Ingredient scaling

Opened with `s` from the detail view. Enter a target factor (e.g. `2` to double, `0.5` to halve) or a target yield, and the view shows every ingredient scaled to the new amount.

For bread and dough recipes, the scaling view also shows a **baker's percentages** table — each ingredient's weight as a percentage of total flour (dry ingredients). This makes it easy to compare a recipe to a target hydration or adjust a formula without changing the overall structure.

After reviewing the scaled recipe, press `p` to open the print preview with the scaled quantities so you can export or print the adjusted version.

### Manage

Opened with `m` from the list or detail view. A landing screen with five sections navigated by `↑`/`↓`/`j`/`k`; `enter` opens the selected section, `esc` returns to where you came from.

| Section | What you can do |
|---------|-----------------|
| Configure | Update API key, AI model, credits, PostgreSQL DSN, max log lines, and log level |
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
- **Bread/dough recipe** toggle — marks the recipe so hydration is displayed and ingredient types are shown
- Tag pills for each context (courses, cooking methods, cultural influences, dietary restrictions)
- Ingredient rows (quantity, unit, name, descriptor, section, **ingredient type**) with unlimited rows; ingredient type accepts `dry`, `wet`, `starter`, or blank (for non-hydration ingredients such as salt or yeast)
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

Displays and edits the current configuration: API key (masked), AI model, export credits line, PostgreSQL DSN, max log lines, and log level. The model and log level cycle with `◄`/`►`; `ctrl+s` saves, `esc` cancels. Database path, log file location, and config file location are shown below the editable fields.

Changes to **log level** and **PostgreSQL DSN** take effect on the next launch — a modal notice appears after saving to confirm.

Also accessible from the interactive browser via `m` → **Configure**.

## Database backends

By default enplace stores recipes in a local SQLite file at `~/.local/share/enplace/recipes.db` — no configuration required.

### PostgreSQL

To use a PostgreSQL database instead, open the configuration screen with `enplace config` and fill in the **PostgreSQL DSN** field. You can also set `postgres_dsn` directly in `~/.config/enplace/config.json`.

`enplace config` works without an active database connection, so it is safe to use for fixing a broken or missing DSN.

Connection string formats:

```
# Local Unix socket (simplest — no password required for peer auth)
host=/var/run/postgresql dbname=enplace

# Local Unix socket (URL form)
postgres:///enplace?host=/var/run/postgresql

# Remote with TLS
postgres://user:password@host:5432/dbname?sslmode=require
```

On first launch with a PostgreSQL DSN configured, enplace connects, runs migrations, and prompts you to import any existing local recipes. After a successful import the local SQLite data is cleared. The SQLite file itself is not deleted.

If the PostgreSQL connection fails at startup, enplace exits with an error — it does not silently fall back to SQLite. Fix the connection string with `enplace config`.

## Building

Requires Go 1.21+. No C compiler needed.

```sh
git clone ...
cd enplace
go build -o enplace .
```

See [Installation](#installation) for user-facing install methods (Homebrew, Scoop, install scripts, `go install`).

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

On first run, `enplace` prompts for an Anthropic API key and writes:

```
~/.config/enplace/config.json   — API key, model name, database path
~/.local/share/enplace/         — SQLite database directory and log file
```

Both paths follow the XDG Base Directory spec. Set `XDG_CONFIG_HOME` or `XDG_DATA_HOME` to override.

Run `enplace config` to open the interactive configuration screen, or edit `config.json` directly. Available settings:

| Setting | Default | Notes |
|---------|---------|-------|
| `anthropic_api_key` | — | Required. Get one at console.anthropic.com |
| `anthropic_model` | `claude-haiku-4-5-20251001` | Cycles with `◄`/`►` in the config screen |
| `credits` | — | Shown in the footer of exported files (e.g. `Chef Jane · myrecipeblog.com`) |
| `postgres_dsn` | — | Leave blank to use local SQLite |
| `max_log_lines` | `10000` | Log file is trimmed to this many lines on startup |
| `log_level` | `info` | One of `debug`, `info`, `warn`, `error` |

### Log level and hydration debugging

Setting `log_level` to `debug` writes detailed traces to `~/.local/share/enplace/enplace.log`, including a step-by-step breakdown of every hydration calculation: each ingredient's type, gram weight, and its individual contribution to the wet and dry totals, followed by the final hydration percentage and baker's percentages. This is useful for verifying that ingredients are correctly classified and understanding exactly how a recipe's hydration figure is derived.

Changes to `log_level` take effect on the next launch.

## Data model

The SQLite schema mirrors the [milk_steak](https://github.com/djcp/milk_steak) Rails app it was designed alongside.

| Table | Purpose |
|---|---|
| `recipes` | Core recipe data: name, description, directions, timing, servings, status, source URL/text; also `rating` (nullable 1–5) and `notes` (freeform text) — user annotations never overwritten by AI extraction |
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

2. **AIExtractor** (`internal/services/ai_extractor.go`) — sends the cleaned text to Claude with a detailed system prompt that specifies canonical ingredient naming, descriptor encoding for prep methods and ingredient alternatives, section grouping, quantity formatting (maximum 10 characters), and tag classification rules. The prompt also instructs Claude to set `is_bread: true` for any bread, roll, loaf, flatbread, pizza dough, focaccia, bagel, pretzel, brioche, croissant, or other yeasted dough; and to classify each ingredient as `dry` (flour, oats, sugar, cocoa, and similar), `wet` (water, milk, eggs, oil, honey, and similar), or `starter` (sourdough starter, levain, poolish, biga). Returns a typed `ExtractedRecipe` struct parsed from the JSON response.

3. **AIApplier** (`internal/services/ai_applier.go`) — writes the extracted data to SQLite: find-or-create for ingredients and tags, replace-on-update for ingredient lines and tag associations, and a status transition to `published`.

## Internal library choices

### [Cobra](https://github.com/spf13/cobra)
Standard Go CLI framework. Handles subcommands, flags, and `--help` output with minimal boilerplate.

### [Charmbracelet / Bubbletea](https://github.com/charmbracelet/bubbletea)
Elm-architecture TUI framework. Used for the interactive recipe browser, add-command progress display, and edit form. The `Msg`/`Update`/`View` pattern keeps UI state immutable and easily testable.

### [Charmbracelet / Bubbles](https://github.com/charmbracelet/bubbles)
Reusable Bubbletea components. `textinput` and `textarea` drive the edit form fields, including inline autocomplete suggestions for ingredients, units, and tags.

### [Charmbracelet / Huh](https://github.com/charmbracelet/huh)
Form and prompt library built on Bubbletea. Used for API key onboarding, config selection menus, and the inline star-rating selector in the recipe detail view (embedded directly in the running TUI rather than launched as a separate program).

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
