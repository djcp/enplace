# enplace — development notes

## Before committing or opening a PR

Always run these two commands and fix any issues before committing or opening a PR:

```sh
go test ./...        # all tests must pass
gofmt -l .           # any listed files need gofmt -w <file>
```

If you changed anything visible in the TUI (layout, colors, new screens), regenerate the screenshots:

```sh
./screenshots/regenerate.sh
```

Requires `tmux`, `asciinema`, and `termtosvg` — the script will tell you what's missing and how to install it.

## Creating a release

Releases are built by **GoReleaser** (`.goreleaser.yaml`), run by `.github/workflows/release.yaml` **on a pushed `v*` tag**. GoReleaser creates the GitHub release itself, cross-compiles all six targets, uploads the archives + a single `checksums.txt`, and (for non-prerelease tags) publishes the Homebrew cask and Scoop manifest. There is nothing to build locally, and **no `gh release create` step** — pushing the tag is what triggers everything.

### Pre-flight

```sh
go test ./...   # all tests must pass
gofmt -l .      # fix any listed files with gofmt -w <file>
./screenshots/regenerate.sh   # refresh screenshots if UI changed
```

### Steps

1. **Bump the version** in `internal/version/version.go` to match the release tag, commit, and push (do this on the feature branch before merging):

```sh
# edit Version = "1.0.x-alpha" in internal/version/version.go
git add internal/version/version.go
git commit -m "Bump version to 1.0.x-alpha"
git push
```

2. **Tag and push** the tag — this is the trigger:

```sh
git tag v1.0.x-alpha
git push origin v1.0.x-alpha
```

That's it. GoReleaser cross-compiles all six targets (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/386, windows/amd64), creates the GitHub release, and attaches the archives + `checksums.txt`.

The version string embedded in released binaries comes from GoReleaser's ldflags injection (`-X …/internal/version.Version={{.Version}}`), so it always matches the tag. The hardcoded default in `version.go` is only the fallback for `go install` / `go build` dev builds. `enplace update` relies on the released binary reporting the true version.

**Prerelease tags** (e.g. `v1.4.0-alpha`) are published as GitHub prereleases (`release.prerelease: auto`), and the cask/manifest are **not** pushed for them (`skip_upload: auto`). Only a stable (non-prerelease) tag updates Homebrew/Scoop.

## Distribution

- **`.goreleaser.yaml`** — the single source of truth for release artifacts. Six build targets, `CGO_ENABLED=0` (pure-Go sqlite → static binaries), archive name template `enplace_{{.Version}}_{{.Os}}_{{.Arch}}` (`.tar.gz` on unix, `.zip` on windows), sha256 `checksums.txt`. `homebrew_casks` and `scoops` publish to the external repos below. Validate locally with `goreleaser check` and dry-run with `goreleaser release --snapshot --clean`.
- **Install scripts** — `install.sh` (POSIX, `curl | sh`) and `install.ps1` (PowerShell, `irm | iex`) live in the repo root and are served raw from GitHub. Each detects OS/arch, resolves the latest release tag via the GitHub API (falling back to the newest prerelease when `/releases/latest` 404s, since the project currently ships alpha tags), downloads the matching archive, verifies its sha256 against `checksums.txt`, and installs the binary. They expect **GoReleaser** asset naming — they will not work against the old `go-release-action` releases.
- **External repos (manual, one-time prerequisites):** `djcp/homebrew-tap` and `djcp/scoop-bucket` must exist, and a classic PAT with `repo` scope must be added as the `HOMEBREW_TAP_GITHUB_TOKEN` secret on `djcp/enplace` so GoReleaser can push the cask/manifest cross-repo.

### `enplace update` (`cmd/update.go`)

Self-update via `github.com/creativeprojects/go-selfupdate`, configured with a `ChecksumValidator{UniqueFilename: "checksums.txt"}` so downloads are sha256-verified against the GoReleaser checksums file. Because the validator resolves that asset during detection, `update` only works against GoReleaser releases (old `go-release-action` releases lack `checksums.txt` and will error). Before replacing the binary it resolves the real path through symlinks and calls `selfmanage.Detect` (`internal/selfmanage/detect.go`) — if a package manager owns the binary (Homebrew `Caskroom`/`Cellar`/`homebrew`/`linuxbrew` or Scoop path — note the Homebrew integration ships as a **cask**, so the binary stages under Caskroom, and on Intel macOS `Caskroom` is the only marker in the path), it prints `brew upgrade` / `scoop update` instead of clobbering the managed file. Like `configCmd`, it overrides `PersistentPreRunE` to a no-op so it runs without a DB.

### `enplace export` (`cmd/export.go`, `internal/export/archive.go`)

Bulk-exports every recipe to one file. `export.WriteArchive(w, recipes, format)` takes **already-hydrated** `[]*models.Recipe` (keeping the export package free of a `db` dependency) and writes either a versioned JSON envelope (`export.Archive`, `schema` 1 — the models carry `json:` tags for a stable, future-importable shape) or concatenated `ToText`. `cmd/export.go` does the DB fetch (`ListRecipes` → `GetRecipe` per id for full ingredients+tags), resolves the output path (from `--out`, else a `huh` prompt pre-filled with `~/enplace_recipe_backup_<date>.<ext>`, else the default silently in a non-TTY), and dedups via `export.UniqueFilePath`.

## Database layer

### `*db.DB` wrapper type

`internal/db/db.go` defines a `DB` struct that embeds `*sqlx.DB` and overrides `Get`, `Select`, `Exec`, and `QueryRow` to call `d.DB.Rebind(query)` before executing. This means all callers can write `?` placeholders universally — the wrapper translates them to `$1/$2/…` for PostgreSQL automatically.

Key methods on `*db.DB`:
- `Driver()` — returns `"postgres"` or `"sqlite3"`
- `insertReturningID(query, args...)` — uses `RETURNING id` on postgres, `LastInsertId()` on sqlite
- `onConflictDoNothing(query)` — prepends `INSERT OR IGNORE` on sqlite, appends `ON CONFLICT DO NOTHING` on postgres

### Dialect-specific query callsites

Two places cannot be handled by the wrapper's auto-rebind and require explicit branching:

1. **`GetRecipeByURL`** — SQLite uses `COLLATE NOCASE`, PostgreSQL uses `LOWER(source_url) = LOWER($1)`. The query is built with `if db.Driver() == "postgres"` and passed directly to `db.DB.Get` (bypassing the wrapper's rebind since the pg variant uses `$1`).

2. **`AttachTag` / `MergeTag`** — INSERT idempotency: `onConflictDoNothing` wraps the query string before passing to `db.Exec`.

### `execTx` callbacks need explicit `db.Rebind()`

The `execTx` helper in `manage_queries.go` uses raw `*sql.Tx` (obtained via `db.Begin()`). A raw `*sql.Tx` bypasses the `*db.DB` wrapper — there is no automatic rebind. Every query string inside a tx callback must be wrapped with `db.Rebind(query)` before passing to `tx.Exec`.

### Migration directories

- `internal/db/migrations/sqlite/` — 5 goose files for SQLite schema
- `internal/db/migrations/postgres/` — 5 parallel goose files with PostgreSQL-compatible DDL (`BIGSERIAL`, `TIMESTAMPTZ`, `BOOLEAN`, partial unique index on `LOWER()`)

Both directories are embedded via `//go:embed` in `db.go`. Goose's `goose_db_version` table is the schema_migrations equivalent — it is managed automatically.

### `MigrateToPostgres`

Uses `SaveRecipe` (the normal find-or-create path) to copy recipes, not a bulk INSERT. This means:
- Ingredient and tag deduplication works correctly when postgres already has some data
- Duplicate URLs are skipped (`GetRecipeByURL` check before each recipe)
- `r.ID = 0` before calling `SaveRecipe` forces the `CreateRecipe` path

After a successful migration, call `ClearSQLiteData` to remove all data from the SQLite file (schema and file are preserved).

### Integration tests

Integration tests are in `internal/db/migrate_postgres_test.go` with a `//go:build integration` tag. They are skipped unless `TEST_POSTGRES_DSN` env var is set. Run with:

```sh
TEST_POSTGRES_DSN="postgres://..." go test -tags integration ./internal/db/...
```

## Logging (`internal/logging/logging.go`)

### Log level

`logging.Open(logPath string, maxLines int, level slog.Level)` creates a `slog.TextHandler` at the given level. The level comes from `cfg.SlogLevel()` in `initConfig` (`cmd/root.go`), which reads `Config.LogLevel` (a string: `"debug"`, `"info"`, `"warn"`, `"error"`). The default is `"info"`.

The level is set once at startup — changing it in the config screen takes effect on the next launch. The config screen shows a restart-required modal after saving when the log level (or PostgreSQL DSN) changes.

### Log level audit — what belongs at each level

| Level | Used for |
|-------|---------|
| `Error` | Goose migration failures (`gooseAdapter.Fatalf`) |
| `Warn` | Non-fatal startup failures: backfill errors, unable to check SQLite recipe count |
| `Info` | Significant lifecycle events: migration started/complete, backup started/complete, SQLite cleanup started/complete |
| `Debug` | Per-item detail: individual recipes imported/skipped, per-table cleanup counts, goose per-migration step output (`gooseAdapter.Printf`), hydration calculation traces |

The hydration debug traces (`debugHydration` in `internal/scaling/scaling.go`) log per-ingredient type, gram weight, dry/wet contribution, totals, hydration percentage, and baker's percentages. They are gated at `Debug` so they are invisible at the default `Info` level and only appear when the user explicitly sets `log_level = "debug"` in their config.

### Restart-required modal in the config screen

After `ctrl+s` in the config screen (`internal/ui/config_screen.go`), if any restart-required setting changed (currently: log level or PostgreSQL DSN), `doSave()` collects the notices into `m.restartNotices`, sets `m.showRestartModal = true`, and returns **without** `tea.Quit`. `View()` renders a centered rounded-border modal showing "Configuration saved" plus bullet points for each notice. Any keypress dismisses the modal and exits.

This pattern avoids the previous approach of setting `m.dsnNotice` (a dead field that was never rendered) and instead surfaces all restart notices through one consistent modal.

## UI / lipgloss rendering

### Chrome layer (`internal/ui/chrome.go`)

All screen chrome (btop-inspired) is built from the primitives in `chrome.go`:

- **`framePanel(content, width, innerHeight, topLeft, topRight, bottom, borderColor, titleColor)`** —
  full titled panel frame (`╭─┫ title ┣───╮ … ╰──┫ bottom ┣─╯`). Content is
  clipped (ANSI-aware `MaxWidth`) and padded to exactly `innerHeight` lines.
  Used by the recipe list panel and the filter panel.
- **`flatRule(width, title, right, …)` / `flatRuleStyled(…)`** — one-line titled
  rule used as the banner on every non-panel screen (`─┫ 🍳 enplace / … ┣───`).
  `flatRuleStyled` takes a pre-styled title segment; build breadcrumbs with
  `breadcrumbTitle(trail)` (in `recipe_detail.go`).
- **`sectionRule(width, label)`** — quiet titled rule (`─┤ Ingredients ├──`) for
  section headers inside reading views. Deliberately lighter than panel borders.
- **`keyHint(key, label)`** — footer hint: bold-accent key + muted label. All
  footers use these; `renderManageFooter` splits plain `"key label"` strings
  itself, so manage call sites keep passing plain strings.
- **`renderGauge(frac, width)`** — gradient block gauge (btop-style: cells
  coloured by absolute position along the ramp). Hydration uses
  `hydrationGaugeFrac(pct)` which maps 50–105% hydration onto the gauge.
- **`renderBar(frac, width, color)`** — single-colour bar with eighth-block
  partial end, for the baker's-percentages chart. Colours come from
  `ingredientTypeColor(type)` (flour=primary, wet=teal, dry=amber,
  starter=purple, fat=muted).

**Design rule: dashboard surfaces get panels, reading surfaces stay calm.**
The recipe list + filter pane are framed panels; the detail/scale/print views
get only the one-line banner rule, `sectionRule` headers, and the bread gauges —
no boxes around prose.

### Screen height accounting — the "-4" fill convention

Banners are exactly **1 row** (`flatRule`), standard footers are **2 rows**
(border-top + hint line). Screens pin their footer with newline-counting fill.
Two patterns exist — do not mix them up:

1. **Outer-builder screens** (recipe list overlays, detail, config, manage
   landing): `used := strings.Count(sb.String(), "\n")` counts everything
   including the banner → fill target is `m.height - used - 3`.
2. **Manage phase views** (tags/units/ingredients/AI-runs sub-views,
   `viewManageResult`, `buildCenteredBox`): the local builder does **not**
   include the banner (written by the outer `View()`), so the fill target is
   `m.height - used - 4` — one extra for the banner row.

Fixed row-count methods (`visibleRows`, `listVisibleRows`, `browseVisibleRows`,
`detailViewportHeight`, etc.) each document their overhead sum in a comment.
If you change any screen's banner/footer structure, re-verify empirically:
render `View()` at a fixed `tea.WindowSizeMsg` height in a scratch test and
count `strings.Split(out, "\n")` — it must equal the terminal height exactly.

`footerLine` clips its hints (ANSI-aware) instead of wrapping when the hints +
version tag exceed the width; never let a footer wrap, it breaks height math.

### Centering multi-line blocks (dialogs, forms, overlays)

Never use `strings.Repeat(" ", leftPad) + block` to center a multi-line lipgloss-rendered string.
That only pads the **first** line; every subsequent line starts at column 0.

Always use `lipgloss.PlaceHorizontal`:

```go
sb.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, block))
```

This applies to any multi-line element: confirmation dialogs, bordered form inputs, info boxes, overlays — anything that spans more than one terminal line and needs to be centered.

### Left-indenting multi-line blocks (form inputs, bordered bars)

Never write a manual indent string before a multi-line lipgloss block:

```go
// WRONG — only the first line gets the indent
sb.WriteString("  ")
sb.WriteString(bar)
```

Use `MarginLeft` in the lipgloss style instead so every line is indented consistently:

```go
bar := lipgloss.NewStyle().
    Border(...).
    Width(m.width - 6).
    MarginLeft(2).   // ← lipgloss applies this to all lines
    Render(content)
sb.WriteString(bar)
```

### Borderless list tables (`build*Table` pattern)

Both `buildRecipeTable` (`recipe_list.go`) and `buildAIRunsTable` (`manage_ai_runs.go`) follow the same contract:

- **`nil` (or empty) slice → header-only output**: pass `nil` when there are no data rows; the function still renders the column header so the empty-state layout matches the non-empty layout. Callers append `"\n"` and then an empty-state message below.
- **`selectedIdx = -1` → no highlighted row**: compute as `cursor - offset`, then guard with `if selectedIdx < 0 || selectedIdx >= len(window) { selectedIdx = -1 }`.
- **`width` is the container width in display columns**: each table function computes column widths that sum to `width`. If the fixed columns alone exceed `width` (very narrow terminal), the table will overflow the pane; that is accepted over a crash.

**`listColPad` constant** (`recipe_list.go:502`) is the shared left-padding value applied to every table column. Both table builders use it. It lives in `recipe_list.go` but is package-visible, so `manage_ai_runs.go` can reference it without import.

**Lipgloss `MaxHeight` quirk**: `t.String()` strips the trailing newline from the rendered table block. Every call site must append `"\n"` manually after the table string — that is why both `renderListPane` and `viewList` do:

```go
sb.WriteString(buildXxxTable(...))
sb.WriteString("\n") // MaxHeight strips t.String()'s trailing \n; restore it
```

**Scroll alignment — always derive `dataRows` from a shared method**: the row count used to slice `m.runs`/`m.recipes` for display and the row count used as the scroll threshold in the key handler must be identical. If they differ by even 1, the selected row can scroll off-screen silently (the `selectedIdx >= rendered` guard snaps selection to -1).

The pattern for manage-style screens is:

```go
// A method on the model — single source of truth for how many rows are displayed.
func (m fooModel) listDataRows() int {
    // listVisibleRows() = height - (banner + footer overhead).
    // Subtract 1 for each line consumed before the first data row
    // (e.g. leading blank line, table header).
    v := m.listVisibleRows() - 2
    if v < 1 { v = 1 }
    return v
}

// In viewList:
dataRows := m.listDataRows()
end := m.offset + dataRows

// In handleListKey "down":
dataRows := m.listDataRows()
if m.cursor >= m.offset+dataRows {
    m.offset = m.cursor - dataRows + 1
}
```

**`truncateW` vs `truncate`**: use `truncateW` (display-width-aware, in `recipe_list.go`) for any string that might contain emoji, rating glyphs (★☆), or other wide/non-ASCII characters — recipe names, descriptions, tag values, user-supplied text of any kind. Use `truncate` (rune-count, in `recipe_list.go`) only for strings that are guaranteed ASCII, such as fixed-format timestamps or known-ASCII status badges.

**2-line row invariant in `buildRecipeTable`**: every cell in the name column embeds exactly one `"\n"` — either `nameLine + "\n"` (non-selected, blank second line) or `nameLine + "\n" + descLine` (selected, description second line). This keeps every recipe row at exactly 2 terminal lines with `Wrap(true)`. The description must be truncated with `truncateW` before embedding; otherwise a wide-character description could wrap inside the cell and push the row to 3+ lines, breaking layout stability.

## Export package (`internal/export/`)

### Adding a new export format

The export table in `recipe_print.go` drives the format-select menu:

```go
var exportFormats = []struct{ label, ext string }{
    {"PDF (.pdf)", "pdf"},
    ...
}
```

To add a format:
1. Add a new `To<Format>(r *models.Recipe) (string or []byte, error)` function in `internal/export/<format>.go`.
2. Append an entry to `exportFormats` in `recipe_print.go` — `ext` is what `execute()` switches on.
3. Add a `case "<ext>":` branch in `execute()` that calls the new function and assigns `data`.

The printer entry uses `ext == ""` as its sentinel; all non-empty `ext` values write a file via `export.UniqueFilePath`.

### Encoding rules per format

| Format | Encoding | How Unicode is handled |
|--------|----------|------------------------|
| `.txt` | UTF-8 | Go strings are UTF-8; `os.WriteFile` emits bytes as-is — correct |
| `.md`  | UTF-8 | Same as plain text — no special handling needed |
| `.rtf` | cp1252 + RTF escapes | `rtfEnc()` in `rtf.go` translates every rune (see below) |
| `.pdf` | cp1252 via fpdf `tr` | `UnicodeTranslatorFromDescriptor("")` in `pdf.go` (see below) |

The root cause of mojibake in both RTF and PDF is the same: the output format uses
**cp1252** as its default character encoding, but Go strings are **UTF-8**. A
character like `•` (U+2022) is three UTF-8 bytes (`E2 80 A2`); without translation,
those three bytes are each interpreted as separate cp1252 characters, producing
`â€¢`. Characters in the Latin-1 supplement (e.g. `°`, U+00B0) are two UTF-8 bytes
(`C2 B0`), producing `Â°`.

### RTF encoding — always use `rtfEnc`

`rtfEnc` in `internal/export/rtf.go` encodes each Unicode rune into the RTF escape
sequence the format requires:

- ASCII (0x20–0x7E): pass through (after escaping `\`, `{`, `}`)
- `\n`: converted to `\par` (RTF paragraph break)
- Latin-1 supplement (U+00A0–U+00FF): `\'XX` where XX = the byte value (identical in cp1252)
- cp1252 special range (•, –, —, curly quotes, €, …): `\'XX` via the `cp1252Special` lookup table
- Everything else: `\uN?` RTF Unicode escape (signed 16-bit decimal, `?` fallback)

Pass **every** user-data string through `rtfEnc` before embedding in the RTF stream.
The `\ansicpg1252` header tag also needs to be present — it tells RTF readers which
code page governs `\'XX` escapes.

### PDF encoding — always translate strings through `tr`

`github.com/go-pdf/fpdf` uses **cp1252** (Windows-1252) for its built-in core fonts
(Helvetica, Times, Courier). Go source strings are UTF-8. Any character outside
plain ASCII that is not translated will be silently misread byte-by-byte, producing
mojibake (e.g. `•` → `â€¢`, `°` → `Â°`).

The fix is to obtain a translator immediately after creating the `Fpdf` instance and
pass **every** string through it before handing it to fpdf:

```go
f := fpdf.New("P", "mm", "Letter", "")
tr := f.UnicodeTranslatorFromDescriptor("") // cp1252 (the default)
// ...
f.MultiCell(pw, 6, tr(someString), "", "L", false)
```

`UnicodeTranslatorFromDescriptor("")` maps cp1252-representable characters correctly
and replaces unmappable ones with a fallback `?`. If you ever switch to a TrueType
font (which supports full Unicode natively) you can drop the `tr` calls — but with
core fonts it is always required.

### `UniqueFilePath` — deduplication of saved files

`export.UniqueFilePath(dir, base, ext string) string` probes the filesystem and
returns the first non-conflicting path: `base.ext`, then `base-2.ext`, `base-3.ext`,
etc. It is the only place that constructs output paths for file saves. Do not
construct paths with `filepath.Join(dir, base+"."+ext)` directly — you'll lose
deduplication.

## Manage screens (`internal/ui/manage*.go`)

### Dispatch loop pattern

The manage system uses a loop in `cmd/helpers.go` (`runManageUI()`): show the landing page (`RunManageUI`) → dispatch to the selected sub-screen's `Run*UI` function → loop back to the landing page. Each sub-screen is its own Bubbletea program that returns when done. `ManageSectionBack` (the zero/iota value) exits the loop.

### Phase-driven sub-screen pattern

Each manage sub-screen (`manage_tags.go`, `manage_ingredients.go`, `manage_units.go`, `manage_ai_runs.go`) uses an explicit `phase` enum. `Update` routes key messages to phase-specific handlers; each phase has its own `view*` and `renderFooter*` methods. Keep this pattern consistent — resist merging phase logic into one large `Update` or `View`.

### Retry action — availability guard

The `r retry` action in the AI runs detail view is available for **any** run tied to an existing recipe (`m.fullRun.RecipeID != nil`). Do not add a `!m.fullRun.Success` guard — retry is valid for succeeded runs too (e.g. to re-extract with a better prompt or model).

The retry action in the recipe detail view (`recipe_detail.go`) uses `m.recipe.IsFailed()` to guard the `r` key and conditionally show the footer hint. This is a different guard because it only makes sense to prompt a retry directly on a recipe that is in `processing_failed` status.

Both code paths call `runRetryPipeline(recipeID)` in `cmd/helpers.go`, which runs the progress TUI and returns; the caller then reloads the recipe from the DB and continues the loop.

### Inline list notice (no result page)

After a destructive operation that returns the user to the list view (e.g. delete in AI runs), set `listNotice string` and `listNoticeErr bool` on the model instead of transitioning to a result phase. `viewList()` renders the notice above the footer using `SuccessStyle`/`ErrorStyle`. This avoids an extra keypress to dismiss a result page.

### `truncate()` — must guard negative max

`truncate(s string, max int)` in `recipe_list.go` slices runes by index. Always guard `max <= 0` at the top (`return ""`). Call sites that compute `nameWidth := m.width - constant` must clamp to `if nameWidth < 1 { nameWidth = 1 }` before passing to `truncate` to prevent panics on narrow terminals.

### DB layer (`internal/db/manage_queries.go`)

Tag and ingredient merge operations use transactions: repoint foreign-key joins (`recipe_tags` or `recipe_ingredients`) then delete the source row. Unit merge is a plain bulk `UPDATE recipe_ingredients SET unit=target WHERE unit=source` — units are inline strings, not a separate table.

## Bread/dough recipes and hydration (`is_bread`, `ingredient_type`)

### The `is_bread` flag

`recipes.is_bread` (BOOLEAN NOT NULL DEFAULT 0, added in migration `004_is_bread.sql`) gates all bread-specific UI and calculations. Only when `r.IsBread` is true will the app:

- Show the 🍞 pill on tag rows and banners (`BreadPill` in `styles.go`)
- Show the 🍞 prefix on list rows (`renderRecipeRow` in `recipe_list.go`)
- Compute and display hydration in the detail view (`buildRecipeBlock` in `recipe_detail.go`)
- Show baker's percentages in the scale view (`renderBreadMetrics` in `recipe_scale.go`)
- Emit a `Hydration:` line in all export formats (text, markdown, RTF, PDF)

The flag is set by the AI extractor (see below) and is also a toggle in the edit form (`efIsBread` in `recipe_edit.go`, toggled with left/right/space). It is also a filter in the recipe list pane (`ffIsBread` in `filter_pane.go`, "recipe type" label, toggled with left/right/space).

### Ingredient types: `flour`, `dry`, `wet`, `fat`, `starter`

`recipe_ingredients.ingredient_type` is stored on the canonical `ingredients` table (so all uses of the same ingredient share one classification). Five values carry meaning for hydration and baker's percentages:

| Value | Meaning | Hydration | Baker's % |
|-------|---------|-----------|-----------|
| `flour` | Any milled flour: AP, bread, whole wheat, rye, spelt, corn, almond, oat flour, semolina, etc. | dry side | 100% base |
| `dry` | All other non-fat dry solids: oats, seeds, sugar, salt, yeast, spices, potato flakes, baking powder, cocoa powder, starches, etc. | dry side | % of flour |
| `wet` | All liquids: water, milk, cream, buttermilk, eggs, oil, honey, syrup, juice, beer, wine, yogurt, etc. | wet side | % of flour |
| `fat` | Saturated fats excluded from hydration: butter, lard, shortening, margarine, cocoa butter, suet | excluded | % of flour |
| `starter` | Pre-ferments (sourdough starter, levain, poolish, biga) — split 50/50 between wet and dry | split 50/50 | % of flour |
| `` (blank) | Truly unweighable items: herb sprigs, whole vanilla beans, bay leaves, decorative toppings | excluded | excluded |

The edit form placeholder is "flour / dry / wet / fat / starter / (blank)".

### Hydration calculation (`internal/scaling/scaling.go`)

`BreadMetrics(ingredients []models.RecipeIngredient) (BreadMetricsResult, error)` iterates all ingredients and accumulates:

- `TotalFlourGrams` — sum of weights for `ingredient_type == "flour"` — the baker's % base (= 100%)
- `TotalDryGrams` — flour + all `"dry"` ingredients + half of any `"starter"` weight — the hydration denominator
- `TotalWetGrams` — sum of weights for `ingredient_type == "wet"` + half of any `"starter"` weight — the hydration numerator
- `TotalFatGrams` — sum of weights for `ingredient_type == "fat"` — excluded from hydration, included in total dough weight
- `StarterCount` — count of starter ingredients encountered

The 50/50 starter split reflects the assumption of a **100% hydration starter** (equal parts flour and water by weight). This is the most common sourdough starter maintenance ratio; it is always assumed and never configurable. A footnote is shown wherever hydration is displayed when `StarterCount > 0`.

```
HydrationPct = TotalWetGrams / TotalDryGrams × 100
```

Baker's percentages use `TotalFlourGrams` as the 100% base:
```
IngredientPct = IngredientWeightGrams / TotalFlourGrams × 100
```

`PerIngredient` covers all typed ingredients (flour, dry, wet, fat, starter) and is only populated when `TotalFlourGrams > 0`.

Only ingredients with a weight unit (g, kg, oz, lb) or a `unit_weight_g` set contribute; others are counted in `ExcludedCount`. `BreadMetrics` returns an error if `TotalDryGrams == 0` (nothing to compute hydration from).

Total dough weight = `TotalDryGrams + TotalWetGrams + TotalFatGrams`.

### Hydration display

Hydration flows through the `Renderer` interface in `internal/export/renderer.go`:

```go
type Renderer interface {
    // ... other methods ...
    Hydration(pct float64, totalGrams int, starterAssumed bool)
}
```

`RenderRecipe` calls `ren.Hydration(bm.HydrationPct, totalG, bm.StarterCount > 0)` after the ingredient block when `r.IsBread` is true and `BreadMetrics` succeeds. Each renderer formats the line appropriately:

- **text**: `Hydration: 65.0%  ·  864g total  (100% hydration starter assumed)`
- **markdown**: `**Hydration:** 65.0%  ·  864g total  *(100% hydration starter assumed)*`
- **RTF**: bold terracotta line with `\par`
- **PDF**: bold Helvetica at 11pt in terracotta

The detail TUI (`buildRecipeBlock` in `recipe_detail.go`) and scale view (`renderBreadMetrics` in `recipe_scale.go`) compute hydration independently using `scaling.BreadMetrics` and render it with lipgloss styles.

### AI classification of bread recipes and ingredient types

The AIExtractor system prompt instructs the model to:

1. Set `is_bread: true` for any recipe that produces bread, rolls, loaves, flatbreads, pizza dough, focaccia, bagels, pretzels, brioche, croissants, or other yeasted or leavened doughs. Set `false` for all other recipes.

2. Set `ingredient_type` on each ingredient to one of the six values in the table above. The most important distinctions:
   - Flours (any kind) → `"flour"`, not `"dry"`
   - Butter, lard, shortening → `"fat"`, not `"wet"`
   - Salt, yeast, sugar, seeds, spices → `"dry"` (they contribute to total dough weight and the hydration denominator)
   - Herb sprigs, whole vanilla beans → `""` (truly unweighable)

This means a freshly extracted bread recipe will have correct hydration and baker's percentages immediately, without any manual editing.

### Backfill at startup (`internal/db/backfill.go`)

`BackfillIngredientTypes` runs at every startup (alongside `BackfillQuantityNumeric`). It migrates canonical `ingredients` rows from the old three-type scheme to the new five-type scheme:

- `"dry"` ingredients with flour-like names (e.g. `%flour%`, `semolina`) → `"flour"`
- `"wet"` or `""` ingredients with fat names (butter, lard, shortening, margarine, cocoa butter, suet) → `"fat"`

All other existing type values are left unchanged. The function is idempotent.

## Print preview TUI (`internal/ui/recipe_print.go`)

### Phase model

`PrintModel` uses an explicit `printPhase` enum (`printPhasePreview` →
`printPhaseFormatSelect` → `printPhaseResult`). Each phase has its own key handler
(`handlePreviewKey`, `handleFormatKey`) and its own `render*` method. Keep this
separation: resist the urge to fold phase logic into a single large `Update` or `View`.

### `execute()` is a pure value transform

`execute()` takes a `PrintModel` by value and returns a new `PrintModel` by value —
no pointer receivers, no side effects on `m` before the call. The only I/O it does
is writing the file or forking `lp`/`lpr`. Keep it this way so it stays easy to test
in isolation.

### `buildPreviewLines` couples `ToText` and the TUI

`buildPreviewLines` calls `export.ToText` and applies lipgloss highlights to the
result. The highlights rely on knowing that line 0 is the recipe name and that
section headers are the exact strings `"INGREDIENTS"` and `"DIRECTIONS"`. If `ToText`
ever changes those strings or their line positions, `buildPreviewLines` must be
updated in step.

### Vertical fill in `renderFormatSelect`

The format-select overlay is rendered with `\n\n` before the box and then the
remaining vertical space is computed by counting `\n` in `sb` and subtracting from
`m.height`. This is a fragile heuristic — it works because the banner always
contributes the same number of lines. If the banner height changes (e.g. wrapping on
very narrow terminals), the fill calculation will be off. A more robust approach
would be to track consumed lines explicitly rather than counting newlines post-hoc.

## Ratings and notes

`recipes.rating` (nullable INTEGER 1–5) and `recipes.notes` (TEXT NOT NULL DEFAULT '')
are user annotations. Never write them through `UpdateRecipeFields` or `SaveRecipe`
(both handle AI-extracted data only). Use `UpdateRecipeRating` and `UpdateRecipeNotes`.

### huh form value-binding pattern

The rating selector uses `huh.NewSelect[int]().Value(m.ratingPending)`.
Bubbletea copies the model on every `Update`, so the binding pointer must be
heap-allocated (`m.ratingPending = &v`) and outlive model copies — never bind to a
plain struct field. `handleRatingMsg` handles all `tea.Msg` types (not just `KeyMsg`)
so the huh form receives cursor-blink and other internal events it needs.

### PDF export

PDF uses `"Rating: 4/5"` (plain text) rather than ★/☆ glyphs —
those code points are outside cp1252, same as the existing `•`/`°` constraints.
