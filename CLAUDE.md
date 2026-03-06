# enplace — development notes

## Before committing or opening a PR

Always run these two commands and fix any issues before committing or opening a PR:

```sh
go test ./...        # all tests must pass
gofmt -l .           # any listed files need gofmt -w <file>
```

## Creating a release

Cross-compilation and asset uploading are handled automatically by `.github/workflows/release.yaml` (using `wangyoucao577/go-release-action`) when a GitHub release is created. There is nothing to build locally.

### Pre-flight

```sh
go test ./...   # all tests must pass
gofmt -l .      # fix any listed files with gofmt -w <file>
```

### Steps

1. **Tag and push** the tag:

```sh
git tag v1.0.x-alpha
git push origin v1.0.x-alpha
```

2. **Create the GitHub release** — this triggers CI to build and attach binaries:

```sh
gh release create v1.0.x-alpha \
  --title "v1.0.x-alpha - <short description>" \
  --notes "<release notes>" \
  --prerelease
```

That's it. GitHub Actions will cross-compile for all six targets (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/386, windows/amd64) and attach the archives and MD5 checksums to the release automatically.

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
