package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/djcp/enplace/internal/db"
)

// ── buildAIRunsTable helpers ──────────────────────────────────────────────────

// makeRun constructs a minimal AIRunSummary for table rendering tests.
func makeRun(recipeName, service, model string, success bool, durMS int64) db.AIRunSummary {
	t := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	return db.AIRunSummary{
		RecipeName:   recipeName,
		ServiceClass: service,
		AIModel:      model,
		Success:      success,
		DurationMS:   durMS,
		CreatedAt:    t,
	}
}

// ── buildAIRunsTable ──────────────────────────────────────────────────────────

func TestBuildAIRunsTable_HeaderOnly(t *testing.T) {
	out := buildAIRunsTable(nil, -1, 120)
	// lipgloss MaxHeight strips the trailing \n from header-only output.
	got := strings.Count(out, "\n")
	if got != 0 {
		t.Errorf("header-only: want 0 newlines, got %d", got)
	}
	plain := stripANSI(out)
	// "OK" is intentionally clipped: successColWidth = 1+listColPad = 3 cols,
	// with 2 cols of left padding that leaves 1 content col — "OK" renders as "O".
	// Test only the columns whose headers fit their allocated width.
	for _, col := range []string{"Date", "Service", "Model", "Duration", "Recipe"} {
		if !strings.Contains(plain, col) {
			t.Errorf("header should contain %q", col)
		}
	}
}

func TestBuildAIRunsTable_LineCount(t *testing.T) {
	runs := []db.AIRunSummary{
		makeRun("Pasta", "openai", "gpt-4o", true, 1200),
		makeRun("Bread", "anthropic", "claude-3-5-sonnet", false, 800),
		makeRun("Soup", "openai", "gpt-4o-mini", true, 400),
	}
	out := buildAIRunsTable(runs, -1, 120)
	// Each AI run row is single-line (Wrap false), so N rows → N newlines.
	got := strings.Count(out, "\n")
	want := len(runs)
	if got != want {
		t.Errorf("lineCount: want %d newlines, got %d\noutput:\n%s", want, got, out)
	}
}

func TestBuildAIRunsTable_SelectedRowContent(t *testing.T) {
	runs := []db.AIRunSummary{
		makeRun("Pasta Carbonara", "openai", "gpt-4o", true, 1200),
		makeRun("Sourdough Bread", "anthropic", "claude-3-5-sonnet", false, 800),
	}
	out := buildAIRunsTable(runs, 0, 120)
	plain := stripANSI(out)
	if !strings.Contains(plain, "Pasta Carbonara") {
		t.Error("selected row (idx 0) should contain its recipe name")
	}
}

func TestBuildAIRunsTable_NoSelectionShowsAllRows(t *testing.T) {
	runs := []db.AIRunSummary{
		makeRun("Pasta", "openai", "gpt-4o", true, 1200),
		makeRun("Bread", "anthropic", "claude-3", false, 800),
	}
	out := buildAIRunsTable(runs, -1, 120)
	plain := stripANSI(out)
	for _, name := range []string{"Pasta", "Bread"} {
		if !strings.Contains(plain, name) {
			t.Errorf("non-selected table should show all recipes; missing %q", name)
		}
	}
}

func TestBuildAIRunsTable_DeletedRecipeRendersPlaceholder(t *testing.T) {
	runs := []db.AIRunSummary{
		makeRun("", "openai", "gpt-4o", true, 500),
	}
	out := buildAIRunsTable(runs, -1, 120)
	if !strings.Contains(stripANSI(out), "(deleted)") {
		t.Error("empty recipe name should render as (deleted)")
	}
}

func TestBuildAIRunsTable_SuccessMarks(t *testing.T) {
	runs := []db.AIRunSummary{
		makeRun("Pasta", "openai", "gpt-4o", true, 1200),
		makeRun("Bread", "anthropic", "claude-3", false, 800),
	}
	out := buildAIRunsTable(runs, -1, 120)
	plain := stripANSI(out)
	if !strings.Contains(plain, "✓") {
		t.Error("successful run should render ✓")
	}
	if !strings.Contains(plain, "✗") {
		t.Error("failed run should render ✗")
	}
}

func TestBuildAIRunsTable_NegativeDurationOmitted(t *testing.T) {
	runs := []db.AIRunSummary{
		makeRun("Pasta", "openai", "gpt-4o", true, -1),
	}
	out := buildAIRunsTable(runs, -1, 120)
	plain := stripANSI(out)
	if strings.Contains(plain, "ms") {
		t.Error("negative DurationMS should produce an empty duration cell, not '...ms'")
	}
}

func TestBuildAIRunsTable_DateFormat(t *testing.T) {
	runs := []db.AIRunSummary{
		makeRun("Pasta", "openai", "gpt-4o", true, 100),
	}
	out := buildAIRunsTable(runs, -1, 120)
	// makeRun uses 2024-06-01.
	if !strings.Contains(stripANSI(out), "2024-06-01") {
		t.Error("date column should render in YYYY-MM-DD format")
	}
}

// TestBuildAIRunsTable_RecipeColumnAlignment verifies that the Recipe header and
// every data row's recipe name land at the same horizontal offset, regardless of
// the content widths in earlier columns.
func TestBuildAIRunsTable_RecipeColumnAlignment(t *testing.T) {
	runs := []db.AIRunSummary{
		makeRun("Short", "openai", "gpt-4o", true, 100),
		makeRun("A much longer recipe name", "anthropic", "claude-3-5-sonnet-20241022", false, 9999),
	}
	const width = 140
	out := buildAIRunsTable(runs, -1, width)
	lines := strings.Split(strings.TrimRight(stripANSI(out), "\n"), "\n")

	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (header + 2 data), got %d\noutput:\n%s", len(lines), out)
	}

	headerPos := displayOffset(lines[0], "Recipe")
	if headerPos < 0 {
		t.Fatalf("Recipe not found in header line: %q", lines[0])
	}
	row0Pos := displayOffset(lines[1], "Short")
	if row0Pos < 0 {
		t.Fatalf("recipe name not found in row 0: %q", lines[1])
	}
	row1Pos := displayOffset(lines[2], "A much longer")
	if row1Pos < 0 {
		t.Fatalf("recipe name not found in row 1: %q", lines[2])
	}

	if row0Pos != headerPos {
		t.Errorf("row 0: recipe col at display col %d, header at %d (want equal)", row0Pos, headerPos)
	}
	if row1Pos != headerPos {
		t.Errorf("row 1: recipe col at display col %d, header at %d (want equal)", row1Pos, headerPos)
	}
}

// ── listDataRows / scroll alignment ──────────────────────────────────────────

func TestListDataRows_AlwaysPositive(t *testing.T) {
	for _, h := range []int{1, 5, 6, 7, 8, 24, 50} {
		m := manageAIRunsModel{height: h}
		if got := m.listDataRows(); got < 1 {
			t.Errorf("height=%d: listDataRows()=%d, want >= 1", h, got)
		}
	}
}

func TestListDataRows_LessThanListVisibleRows(t *testing.T) {
	// listDataRows must always be strictly less than listVisibleRows so that the
	// scroll threshold in handleListKey (cursor >= offset+dataRows) fires before
	// the cursor leaves the rendered window.
	for _, h := range []int{10, 24, 40} {
		m := manageAIRunsModel{height: h}
		visible := m.listVisibleRows()
		data := m.listDataRows()
		if data >= visible {
			t.Errorf("height=%d: listDataRows()=%d >= listVisibleRows()=%d; selected row could go off-screen",
				h, data, visible)
		}
	}
}
