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

Cross-compilation and asset uploading are handled automatically by `.github/workflows/release.yaml` (using `wangyoucao577/go-release-action`) when a GitHub release is created. There is nothing to build locally.

### Pre-flight

```sh
go test ./...   # all tests must pass
gofmt -l .      # fix any listed files with gofmt -w <file>
./screenshots/regenerate.sh   # refresh screenshots if UI changed
```

### Steps

1. **Bump the version** in `internal/version/version.go` to match the release tag, commit, and push:

```sh
# edit Version = "1.0.x-alpha" in internal/version/version.go
git add internal/version/version.go
git commit -m "Bump version to 1.0.x-alpha"
git push
```

2. **Tag and push** the tag:

```sh
git tag v1.0.x-alpha
git push origin v1.0.x-alpha
```

3. **Create the GitHub release** — this triggers CI to build and attach binaries:

```sh
gh release create v1.0.x-alpha \
  --title "v1.0.x-alpha - <short description>" \
  --notes "<release notes>"
```

That's it. GitHub Actions will cross-compile for all six targets (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/386, windows/amd64) and attach the archives and MD5 checksums to the release automatically.

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

## UI / lipgloss rendering

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
