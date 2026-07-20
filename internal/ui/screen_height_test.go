package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/enplace/internal/db"
	"github.com/djcp/enplace/internal/export"
	"github.com/djcp/enplace/internal/models"
	"github.com/djcp/enplace/internal/scaling"
)

// Every full-screen view must render to exactly the terminal height (footer
// pinned to the bottom row) and never exceed it — an overflowing view pushes
// the banner off-screen or clips the footer. See CLAUDE.md "Screen height
// accounting" for the fill conventions these tests pin down.

// assertExactHeight fails unless out has exactly height rows and no row
// wider than width display columns.
func assertExactHeight(t *testing.T, name, out string, width, height int) {
	t.Helper()
	lines := strings.Split(out, "\n")
	if len(lines) != height {
		t.Errorf("%s: want exactly %d rows, got %d", name, height, len(lines))
	}
	assertMaxWidth(t, name, lines, width)
}

// assertNoOverflow fails if out has more than height rows or a row wider
// than width (screens with intentional bottom slack use this).
func assertNoOverflow(t *testing.T, name, out string, width, height int) {
	t.Helper()
	lines := strings.Split(out, "\n")
	if len(lines) > height {
		t.Errorf("%s: %d rows overflow terminal height %d", name, len(lines), height)
	}
	assertMaxWidth(t, name, lines, width)
}

func assertMaxWidth(t *testing.T, name string, lines []string, width int) {
	t.Helper()
	for i, line := range lines {
		if lw := lipgloss.Width(line); lw > width {
			t.Errorf("%s: row %d is %d cols wide (max %d): %q", name, i, lw, width, stripANSI(line))
		}
	}
}

func fptr(f float64) *float64 { return &f }
func iptr(n int) *int         { return &n }

// breadTestRecipe returns a bread recipe with fully classified ingredients
// so hydration and baker's percentages are computable.
func breadTestRecipe() *models.Recipe {
	r4 := 4
	return &models.Recipe{
		ID: 1, Name: "No-Knead Sourdough Bread",
		Description: "A slow-fermented rustic loaf with an open crumb.",
		Status:      models.StatusPublished, IsBread: true, Rating: &r4,
		Notes:           "less salt next time",
		PreparationTime: iptr(30), CookingTime: iptr(50),
		Directions: "1. Mix.\n2. Rest.\n3. Bake.",
		Servings:   iptr(1), ServingUnits: "loaf",
		Tags: []models.Tag{{Name: "breakfast", Context: "courses"}},
		Ingredients: []models.RecipeIngredient{
			{IngredientName: "bread flour", Quantity: "500", QuantityNumeric: fptr(500), Unit: "g", IngredientType: "flour"},
			{IngredientName: "water", Quantity: "360", QuantityNumeric: fptr(360), Unit: "g", IngredientType: "wet"},
			{IngredientName: "sourdough starter", Quantity: "100", QuantityNumeric: fptr(100), Unit: "g", IngredientType: "starter"},
			{IngredientName: "salt", Quantity: "10", QuantityNumeric: fptr(10), Unit: "g", IngredientType: "dry"},
			{IngredientName: "butter", Quantity: "25", QuantityNumeric: fptr(25), Unit: "g", IngredientType: "fat"},
		},
	}
}

func heightTestRecipes() []models.Recipe {
	rs := []models.Recipe{*breadTestRecipe()}
	for i := 2; i <= 30; i++ {
		rs = append(rs, models.Recipe{
			ID: int64(i), Name: fmt.Sprintf("Recipe %d", i),
			Description: "desc", Status: models.StatusPublished,
		})
	}
	return rs
}

// screenSizes covers a small and a large terminal.
var screenSizes = []struct{ w, h int }{{80, 24}, {140, 45}}

// ── recipe list ──────────────────────────────────────────────────────────────

func TestScreenHeight_List(t *testing.T) {
	for _, sz := range screenSizes {
		ws := tea.WindowSizeMsg{Width: sz.w, Height: sz.h}
		label := fmt.Sprintf("@%dx%d", sz.w, sz.h)

		m := updateList(NewListModel(heightTestRecipes(), FilterState{}, SearchData{}), ws)
		assertExactHeight(t, "list normal "+label, m.View(), sz.w, sz.h)

		// Filter pane active (typing mode).
		mt := m.enterTypingMode()
		assertExactHeight(t, "list filter-active "+label, mt.View(), sz.w, sz.h)

		// Delete confirmation overlay.
		md := updateList(m, keyMsg("d"))
		assertExactHeight(t, "list confirm-delete "+label, md.View(), sz.w, sz.h)

		// Empty DB.
		me := updateList(NewListModel(nil, FilterState{}, SearchData{}), ws)
		assertExactHeight(t, "list empty-db "+label, me.View(), sz.w, sz.h)

		// Filters active but no matches.
		mn := updateList(NewListModel(nil, FilterState{Query: "zzz"}, SearchData{}), ws)
		assertExactHeight(t, "list no-match "+label, mn.View(), sz.w, sz.h)
	}
}

// TestScreenHeight_List_ScrollPositions guards the scroll/selection alignment:
// the selected row must always be inside the rendered window.
func TestScreenHeight_List_ScrollPositions(t *testing.T) {
	ws := tea.WindowSizeMsg{Width: 100, Height: 24}
	m := updateList(NewListModel(heightTestRecipes(), FilterState{}, SearchData{}), ws)
	for i := 0; i < len(heightTestRecipes())-1; i++ {
		m = updateList(m, keySpecial(tea.KeyDown))
		if m.cursor < m.offset || m.cursor >= m.offset+m.visibleRows() {
			t.Fatalf("after %d downs: cursor %d outside window [%d,%d)",
				i+1, m.cursor, m.offset, m.offset+m.visibleRows())
		}
		assertExactHeight(t, fmt.Sprintf("list scrolled down=%d", i+1), m.View(), 100, 24)
	}
}

// ── recipe detail ────────────────────────────────────────────────────────────

func TestScreenHeight_Detail(t *testing.T) {
	for _, sz := range screenSizes {
		ws := tea.WindowSizeMsg{Width: sz.w, Height: sz.h}
		label := fmt.Sprintf("@%dx%d", sz.w, sz.h)

		d := NewDetailModel(breadTestRecipe(), FilterState{}, SearchData{})
		dm, _ := d.Update(ws)
		dd := dm.(DetailModel)
		assertExactHeight(t, "detail normal "+label, dd.View(), sz.w, sz.h)

		// Delete confirmation overlay.
		dc := dd
		dc.confirmingDelete = true
		assertExactHeight(t, "detail confirm-delete "+label, dc.View(), sz.w, sz.h)

		// Notes overlay.
		dn, _ := dd.Update(keyMsg("N"))
		assertExactHeight(t, "detail notes "+label, dn.(DetailModel).View(), sz.w, sz.h)

		// Filter pane open.
		df, _ := dd.Update(keyMsg("/"))
		assertExactHeight(t, "detail filter-open "+label, df.(DetailModel).View(), sz.w, sz.h)
	}
}

// ── scale ────────────────────────────────────────────────────────────────────

func TestScreenHeight_Scale(t *testing.T) {
	for _, sz := range screenSizes {
		ws := tea.WindowSizeMsg{Width: sz.w, Height: sz.h}
		label := fmt.Sprintf("@%dx%d", sz.w, sz.h)

		s := newScaleModel(breadTestRecipe(), export.Options{})
		sm, _ := s.Update(ws)
		sc := sm.(ScaleModel)
		assertExactHeight(t, "scale input "+label, sc.View(), sz.w, sz.h)

		sc.factorInput.SetValue("2")
		sv, _ := sc.handleInputKey(keySpecial(tea.KeyEnter))
		assertExactHeight(t, "scale view "+label, sv.(ScaleModel).View(), sz.w, sz.h)
	}
}

// ── manage landing ───────────────────────────────────────────────────────────

func TestScreenHeight_ManageLanding(t *testing.T) {
	for _, sz := range screenSizes {
		m, _ := newManageModel().Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
		assertExactHeight(t, fmt.Sprintf("manage landing @%dx%d", sz.w, sz.h),
			m.(ManageModel).View(), sz.w, sz.h)
	}
}

// ── manage tags ──────────────────────────────────────────────────────────────

func TestScreenHeight_ManageTags(t *testing.T) {
	tags := make([]db.TagWithCount, 40)
	for i := range tags {
		tags[i] = db.TagWithCount{ID: int64(i + 1), Name: fmt.Sprintf("tag%d", i), Count: i}
	}
	for _, sz := range screenSizes {
		label := fmt.Sprintf("@%dx%d", sz.w, sz.h)
		m := manageTagsModel{width: sz.w, height: sz.h, tags: tags, mergeList: tags}

		assertExactHeight(t, "tags context "+label, m.View(), sz.w, sz.h)

		m.phase = manageTagsPhaseBrowse
		m.selectedContext = "courses"
		assertExactHeight(t, "tags browse "+label, m.View(), sz.w, sz.h)

		m.phase = manageTagsPhaseEdit
		assertExactHeight(t, "tags edit "+label, m.View(), sz.w, sz.h)

		m.phase = manageTagsPhaseMerge
		m.mergeSourceName = "tag0"
		assertExactHeight(t, "tags merge "+label, m.View(), sz.w, sz.h)

		m.phase = manageTagsPhaseConfirm
		m.confirmName = "tag0"
		m.confirmCount = 3
		assertExactHeight(t, "tags confirm-delete "+label, m.View(), sz.w, sz.h)

		m.phase = manageTagsPhaseResult
		m.resultMsg = "Renamed."
		assertExactHeight(t, "tags result "+label, m.View(), sz.w, sz.h)
	}
}

// ── manage units ─────────────────────────────────────────────────────────────

func TestScreenHeight_ManageUnits(t *testing.T) {
	units := make([]db.UnitWithCount, 40)
	for i := range units {
		units[i] = db.UnitWithCount{Name: fmt.Sprintf("unit%d", i), Count: i}
	}
	for _, sz := range screenSizes {
		label := fmt.Sprintf("@%dx%d", sz.w, sz.h)
		m := manageUnitsModel{width: sz.w, height: sz.h, units: units, mergeList: units}

		assertExactHeight(t, "units browse "+label, m.View(), sz.w, sz.h)

		m.phase = manageUnitsPhaseEdit
		assertExactHeight(t, "units edit "+label, m.View(), sz.w, sz.h)

		m.phase = manageUnitsPhaseMerge
		m.mergeSourceName = "unit0"
		assertExactHeight(t, "units merge "+label, m.View(), sz.w, sz.h)
	}
}

// ── manage ingredients ───────────────────────────────────────────────────────

func TestScreenHeight_ManageIngredients(t *testing.T) {
	ings := make([]db.IngredientWithCount, 40)
	for i := range ings {
		ings[i] = db.IngredientWithCount{ID: int64(i + 1), Name: fmt.Sprintf("ing%d", i), Count: i}
	}
	for _, sz := range screenSizes {
		label := fmt.Sprintf("@%dx%d", sz.w, sz.h)
		m := manageIngredientsModel{width: sz.w, height: sz.h, filtered: ings, mergeList: ings}

		assertExactHeight(t, "ingredients browse "+label, m.View(), sz.w, sz.h)

		m.phase = manageIngPhaseEdit
		assertExactHeight(t, "ingredients edit "+label, m.View(), sz.w, sz.h)

		m.phase = manageIngPhaseMerge
		m.mergeSourceName = "ing0"
		assertExactHeight(t, "ingredients merge "+label, m.View(), sz.w, sz.h)
	}
}

// ── manage AI runs ───────────────────────────────────────────────────────────

func TestScreenHeight_ManageAIRuns(t *testing.T) {
	runs := make([]db.AIRunSummary, 60)
	for i := range runs {
		runs[i] = db.AIRunSummary{ID: int64(i + 1), RecipeName: "Recipe", AIModel: "m", Success: true, CreatedAt: time.Now()}
	}
	for _, sz := range screenSizes {
		label := fmt.Sprintf("@%dx%d", sz.w, sz.h)
		m := newManageAIRunsModel(nil)
		m.runs = runs
		mm, _ := m.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
		m = mm.(manageAIRunsModel)

		assertExactHeight(t, "ai-runs list "+label, m.View(), sz.w, sz.h)

		// Inline notice shrinks the data window instead of pushing the footer.
		m.listNotice = "Deleted run #3."
		assertExactHeight(t, "ai-runs list+notice "+label, m.View(), sz.w, sz.h)
		m.listNotice = ""

		m.phase = manageAIRunsPhaseDetail
		m.detailLines = make([]string, 200)
		assertExactHeight(t, "ai-runs detail "+label, m.View(), sz.w, sz.h)

		m.phase = manageAIRunsPhaseDeleteConfirm
		m.deleteTargetID = 3
		assertExactHeight(t, "ai-runs delete-confirm "+label, m.View(), sz.w, sz.h)
	}
}

// ── print preview ────────────────────────────────────────────────────────────

func TestScreenHeight_PrintPreview(t *testing.T) {
	for _, sz := range screenSizes {
		p := newPrintModel(breadTestRecipe(), export.Options{})
		pm, _ := p.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
		assertExactHeight(t, fmt.Sprintf("print preview @%dx%d", sz.w, sz.h),
			pm.(PrintModel).View(), sz.w, sz.h)
	}
}

// ── add / edit (intentional slack: must never overflow) ──────────────────────

func TestScreenHeight_AddAndEdit_NoOverflow(t *testing.T) {
	for _, sz := range screenSizes {
		label := fmt.Sprintf("@%dx%d", sz.w, sz.h)

		a := NewAddModel(false, "", nil)
		am, _ := a.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
		assertNoOverflow(t, "add "+label, am.(AddModel).View(), sz.w, sz.h)

		e := newEditModel(breadTestRecipe(), EditData{})
		em, _ := e.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
		assertNoOverflow(t, "edit "+label, em.(EditModel).View(), sz.w, sz.h)
	}
}

// ── bread metrics rendering ──────────────────────────────────────────────────

func TestRenderHydrationGauge_Content(t *testing.T) {
	bm, err := scaling.BreadMetrics(breadTestRecipe().Ingredients)
	if err != nil {
		t.Fatalf("BreadMetrics: %v", err)
	}
	out := stripANSI(renderHydrationGauge(bm, 100))

	if !strings.Contains(out, "Hydration") {
		t.Error("missing Hydration label")
	}
	if !strings.Contains(out, fmt.Sprintf("%.1f%%", bm.HydrationPct)) {
		t.Errorf("missing hydration percentage, got %q", out)
	}
	if !strings.Contains(out, "g dough") {
		t.Errorf("missing dough weight, got %q", out)
	}
	if !strings.Contains(out, "starter assumed") {
		t.Errorf("starter recipe must show the 100%%-hydration footnote, got %q", out)
	}
	if !strings.Contains(out, "█") {
		t.Errorf("gauge glyphs missing, got %q", out)
	}
}

func TestRenderHydrationGauge_NoStarterNoFootnote(t *testing.T) {
	ings := []models.RecipeIngredient{
		{IngredientName: "flour", QuantityNumeric: fptr(500), Unit: "g", IngredientType: "flour"},
		{IngredientName: "water", QuantityNumeric: fptr(350), Unit: "g", IngredientType: "wet"},
	}
	bm, err := scaling.BreadMetrics(ings)
	if err != nil {
		t.Fatalf("BreadMetrics: %v", err)
	}
	out := stripANSI(renderHydrationGauge(bm, 100))
	if strings.Contains(out, "starter assumed") {
		t.Errorf("no starter → no footnote, got %q", out)
	}
}

func TestRenderBakerBars_Content(t *testing.T) {
	bm, err := scaling.BreadMetrics(breadTestRecipe().Ingredients)
	if err != nil {
		t.Fatalf("BreadMetrics: %v", err)
	}
	out := stripANSI(renderBakerBars(bm, 100))
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	// One bar row per classified ingredient + the starter footnote.
	if want := len(bm.PerIngredient) + 1; len(lines) != want {
		t.Fatalf("want %d lines (bars + footnote), got %d:\n%s", want, len(lines), out)
	}
	if !strings.Contains(out, "bread flour") || !strings.Contains(out, "100.0%") {
		t.Errorf("flour should anchor the chart at 100%%, got:\n%s", out)
	}
	if !strings.Contains(out, "sourdough starter*") {
		t.Errorf("starter name must carry the * marker, got:\n%s", out)
	}
	if !strings.Contains(out, "* 100% hydration starter assumed") {
		t.Errorf("missing starter footnote, got:\n%s", out)
	}
	if !strings.Contains(out, "█") {
		t.Errorf("bar glyphs missing, got:\n%s", out)
	}
}

func TestBuildRecipeBlock_BreadSections(t *testing.T) {
	out := stripANSI(buildRecipeBlock(breadTestRecipe(), 96))
	for _, want := range []string{
		"Hydration",
		"┤ Ingredients ├",
		"┤ Directions ├",
		"┤ Notes ├",
		"┤ Baker's Percentages ├",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("bread recipe block missing %q", want)
		}
	}
}

// TestBuildRecipeBlock_ConsistentIndent pins the detail-body alignment: every
// content line sits at the shared 2-column indent. Only section rules (full-
// width structural chrome) may start at column 0.
func TestBuildRecipeBlock_ConsistentIndent(t *testing.T) {
	r := breadTestRecipe()
	r.SourceURL = "https://example.com/recipe"
	out := stripANSI(buildRecipeBlock(r, 96))

	for i, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue // blank spacer line
		}
		if strings.Contains(line, "─┤") {
			continue // section rule — full width by design
		}
		if !strings.HasPrefix(line, contentIndent) {
			t.Errorf("line %d not indented %d cols: %q", i, len(contentIndent), line)
		}
	}
}

func TestBuildRecipeBlock_NonBreadOmitsMetrics(t *testing.T) {
	r := breadTestRecipe()
	r.IsBread = false
	out := stripANSI(buildRecipeBlock(r, 96))
	if strings.Contains(out, "Hydration") {
		t.Error("non-bread recipe must not show hydration")
	}
	if strings.Contains(out, "Baker's Percentages") {
		t.Error("non-bread recipe must not show baker's percentages")
	}
}

// ── filter panel ─────────────────────────────────────────────────────────────

func TestRenderFilterPanel_Geometry(t *testing.T) {
	fs := filterState{}
	out := renderFilterPanel(fs, 40, 20)
	lines := strings.Split(out, "\n")
	if len(lines) != 22 { // 20 content + 2 borders
		t.Fatalf("want 22 lines, got %d", len(lines))
	}
	for i, line := range lines {
		if lw := lipgloss.Width(line); lw != 40 {
			t.Errorf("filter panel line %d: want width 40, got %d", i, lw)
		}
	}
	if !strings.Contains(stripANSI(out), "⚙ filters") {
		t.Error("missing panel title")
	}
}

func TestRenderFilterPanel_ActiveBadge(t *testing.T) {
	fs := filterState{query: "sour"}
	out := stripANSI(renderFilterPanel(fs, 40, 20))
	if !strings.Contains(out, "● active") {
		t.Error("active filters should show the ● active badge in the bottom border")
	}
}
