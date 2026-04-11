package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/djcp/enplace/internal/models"
)

// newTestListModel returns a minimal ListModel with autocomplete suggestions set.
func newTestListModel() ListModel {
	return NewListModel(
		[]models.Recipe{{ID: 1, Name: "Pasta", Status: models.StatusPublished}},
		FilterState{},
		SearchData{
			Courses:    []string{"breakfast", "lunch", "dinner"},
			Influences: []string{"italian", "japanese", "mexican"},
		},
	)
}

// updateList calls Update and returns the resulting ListModel.
func updateList(m ListModel, msg tea.Msg) ListModel {
	m2, _ := m.Update(msg)
	return m2.(ListModel)
}

// ── findFirstMatch ────────────────────────────────────────────────────────────

func TestFindFirstMatch_EmptyBuffer(t *testing.T) {
	got := findFirstMatch("", []string{"breakfast"})
	if got != "" {
		t.Errorf("empty buffer: want \"\", got %q", got)
	}
}

func TestFindFirstMatch_ReturnsFirstMatch(t *testing.T) {
	// "br" matches both "brunch" and "breakfast" — first in slice wins.
	got := findFirstMatch("br", []string{"brunch", "breakfast", "dinner"})
	if got != "brunch" {
		t.Errorf("want \"brunch\" (first prefix match), got %q", got)
	}
}

func TestFindFirstMatch_CaseInsensitive(t *testing.T) {
	got := findFirstMatch("BRE", []string{"breakfast"})
	if got != "breakfast" {
		t.Errorf("case-insensitive: want \"breakfast\", got %q", got)
	}
}

func TestFindFirstMatch_NoMatch(t *testing.T) {
	got := findFirstMatch("xyz", []string{"breakfast", "lunch"})
	if got != "" {
		t.Errorf("no match: want \"\", got %q", got)
	}
}

func TestFindFirstMatch_FullWord(t *testing.T) {
	got := findFirstMatch("breakfast", []string{"breakfast"})
	if got != "breakfast" {
		t.Errorf("full word: want \"breakfast\", got %q", got)
	}
}

// ── resolveMatch ──────────────────────────────────────────────────────────────

func TestResolveMatch_UsesMatchWhenAvailable(t *testing.T) {
	got := resolveMatch("bre", []string{"breakfast"})
	if got != "breakfast" {
		t.Errorf("want \"breakfast\", got %q", got)
	}
}

func TestResolveMatch_FallsBackToBuffer(t *testing.T) {
	got := resolveMatch("xyz", []string{"breakfast"})
	if got != "xyz" {
		t.Errorf("fallback: want \"xyz\", got %q", got)
	}
}

// ── nextStatus / prevStatus ───────────────────────────────────────────────────

func TestNextStatus(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "draft"},
		{"draft", "review"},
		{"review", "published"},
		{"published", ""},
	}
	for _, tc := range tests {
		got := nextStatus(tc.in)
		if got != tc.want {
			t.Errorf("nextStatus(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestPrevStatus(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "published"},
		{"draft", ""},
		{"review", "draft"},
		{"published", "review"},
	}
	for _, tc := range tests {
		got := prevStatus(tc.in)
		if got != tc.want {
			t.Errorf("prevStatus(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

// ── hasActiveFilters ──────────────────────────────────────────────────────────

func TestHasActiveFilters(t *testing.T) {
	base := newTestListModel()

	if base.hasActiveFilters() {
		t.Error("fresh model should have no active filters")
	}

	withQuery := base
	withQuery.query = "pasta"
	if !withQuery.hasActiveFilters() {
		t.Error("model with query should have active filters")
	}

	withCourses := base
	withCourses.filterCourses = []string{"breakfast"}
	if !withCourses.hasActiveFilters() {
		t.Error("model with courses should have active filters")
	}

	withInfluences := base
	withInfluences.filterInfluences = []string{"italian"}
	if !withInfluences.hasActiveFilters() {
		t.Error("model with influences should have active filters")
	}

	withStatus := base
	withStatus.filterStatus = "draft"
	if !withStatus.hasActiveFilters() {
		t.Error("model with status should have active filters")
	}
}

// ── enterTypingMode ───────────────────────────────────────────────────────────

func TestEnterTypingMode_SavesState(t *testing.T) {
	m := newTestListModel()
	m.query = "soup"
	m.filterCourses = []string{"lunch"}
	m.filterInfluences = []string{"japanese"}
	m.filterStatus = "draft"

	m2 := m.enterTypingMode()

	if !m2.typing {
		t.Error("typing should be true after enterTypingMode")
	}
	if m2.savedQuery != "soup" {
		t.Errorf("savedQuery: want \"soup\", got %q", m2.savedQuery)
	}
	if len(m2.savedCourses) != 1 || m2.savedCourses[0] != "lunch" {
		t.Errorf("savedCourses: want [lunch], got %v", m2.savedCourses)
	}
	if len(m2.savedInfluences) != 1 || m2.savedInfluences[0] != "japanese" {
		t.Errorf("savedInfluences: want [japanese], got %v", m2.savedInfluences)
	}
	if m2.savedStatus != "draft" {
		t.Errorf("savedStatus: want \"draft\", got %q", m2.savedStatus)
	}
}

func TestEnterTypingMode_CopiesSlices(t *testing.T) {
	m := newTestListModel()
	m.filterCourses = []string{"lunch"}
	m2 := m.enterTypingMode()

	// Mutating the original slice must not affect the saved copy.
	m.filterCourses[0] = "mutated"
	if m2.savedCourses[0] != "lunch" {
		t.Error("savedCourses should be an independent copy")
	}
}

// ── handleTypingKey — Esc ─────────────────────────────────────────────────────

func TestTypingKey_Esc_RestoresState(t *testing.T) {
	m := newTestListModel()
	m.query = "soup"
	m.filterCourses = []string{"lunch"}
	m.filterStatus = "draft"
	m = m.enterTypingMode()

	// Change state while typing.
	m.query = "changed"
	m.filterCourses = []string{"breakfast", "dinner"}
	m.filterStatus = "published"

	m2 := updateList(m, keySpecial(tea.KeyEsc))

	if m2.typing {
		t.Error("typing should be false after Esc")
	}
	if m2.query != "soup" {
		t.Errorf("query: want \"soup\", got %q", m2.query)
	}
	if len(m2.filterCourses) != 1 || m2.filterCourses[0] != "lunch" {
		t.Errorf("filterCourses: want [lunch], got %v", m2.filterCourses)
	}
	if m2.filterStatus != "draft" {
		t.Errorf("filterStatus: want \"draft\", got %q", m2.filterStatus)
	}
	// Saved state should be cleared.
	if m2.savedQuery != "" || len(m2.savedCourses) != 0 || m2.savedStatus != "" {
		t.Error("saved state should be cleared after Esc")
	}
}

func TestTypingKey_Esc_ClearsBuffers(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.courseBuffer = "bre"
	m.influenceBuffer = "ita"

	m2 := updateList(m, keySpecial(tea.KeyEsc))

	if m2.courseBuffer != "" || m2.influenceBuffer != "" {
		t.Errorf("buffers should be cleared after Esc: course=%q influence=%q",
			m2.courseBuffer, m2.influenceBuffer)
	}
}

// ── handleTypingKey — Tab / ShiftTab ─────────────────────────────────────────

func TestTypingKey_Tab_CyclesFocus(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffText

	m2 := updateList(m, keySpecial(tea.KeyTab))
	if m2.filterFocus != ffCourses {
		t.Errorf("tab from ffText: want ffCourses, got %d", m2.filterFocus)
	}

	m3 := updateList(m2, keySpecial(tea.KeyTab))
	if m3.filterFocus != ffInfluences {
		t.Errorf("tab from ffCourses: want ffInfluences, got %d", m3.filterFocus)
	}

	m4 := updateList(m3, keySpecial(tea.KeyTab))
	if m4.filterFocus != ffStatus {
		t.Errorf("tab from ffInfluences: want ffStatus, got %d", m4.filterFocus)
	}

	m5 := updateList(m4, keySpecial(tea.KeyTab))
	if m5.filterFocus != ffIsBread {
		t.Errorf("tab from ffStatus: want ffIsBread, got %d", m5.filterFocus)
	}

	m6 := updateList(m5, keySpecial(tea.KeyTab))
	if m6.filterFocus != ffSearch {
		t.Errorf("tab from ffIsBread: want ffSearch, got %d", m6.filterFocus)
	}

	m7 := updateList(m6, keySpecial(tea.KeyTab))
	if m7.filterFocus != ffText {
		t.Errorf("tab from ffSearch should wrap to ffText, got %d", m7.filterFocus)
	}
}

func TestTypingKey_ShiftTab_CyclesBackward(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffText

	m2 := updateList(m, keySpecial(tea.KeyShiftTab))
	if m2.filterFocus != ffSearch {
		t.Errorf("shift-tab from ffText should wrap to ffSearch, got %d", m2.filterFocus)
	}
}

func TestTypingKey_UpDown_NavigatesFields(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffText

	m2 := updateList(m, keySpecial(tea.KeyDown))
	if m2.filterFocus != ffCourses {
		t.Errorf("down from ffText: want ffCourses, got %d", m2.filterFocus)
	}

	m3 := updateList(m2, keySpecial(tea.KeyUp))
	if m3.filterFocus != ffText {
		t.Errorf("up from ffCourses: want ffText, got %d", m3.filterFocus)
	}

	// down wraps from ffSearch back to ffText
	m4 := m
	m4.filterFocus = ffSearch
	m5 := updateList(m4, keySpecial(tea.KeyDown))
	if m5.filterFocus != ffText {
		t.Errorf("down from ffSearch should wrap to ffText, got %d", m5.filterFocus)
	}

	// up wraps from ffText back to ffSearch
	m6 := updateList(m, keySpecial(tea.KeyUp))
	if m6.filterFocus != ffSearch {
		t.Errorf("up from ffText should wrap to ffSearch, got %d", m6.filterFocus)
	}
}

func TestTypingKey_Tab_DiscardsBuffers(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffCourses
	m.courseBuffer = "bre"
	m.influenceBuffer = "ita"

	m2 := updateList(m, keySpecial(tea.KeyTab))

	if m2.courseBuffer != "" || m2.influenceBuffer != "" {
		t.Errorf("Tab should clear buffers: course=%q influence=%q",
			m2.courseBuffer, m2.influenceBuffer)
	}
}

// ── handleTypingKey — Left / Right on status row ──────────────────────────────

func TestTypingKey_Right_CyclesStatusWhenOnStatusRow(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffStatus
	m.filterStatus = ""

	m2 := updateList(m, keySpecial(tea.KeyRight))
	if m2.filterStatus != "draft" {
		t.Errorf("right on status: want \"draft\", got %q", m2.filterStatus)
	}
}

func TestTypingKey_Left_CyclesStatusWhenOnStatusRow(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffStatus
	m.filterStatus = "draft"

	m2 := updateList(m, keySpecial(tea.KeyLeft))
	if m2.filterStatus != "" {
		t.Errorf("left on status: want \"\", got %q", m2.filterStatus)
	}
}

func TestTypingKey_Right_IgnoredOnTextRow(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffText
	m.filterStatus = ""

	m2 := updateList(m, keySpecial(tea.KeyRight))
	if m2.filterStatus != "" {
		t.Errorf("right on text row should not change status, got %q", m2.filterStatus)
	}
}

// ── handleTypingKey — Backspace ───────────────────────────────────────────────

func TestTypingKey_Backspace_RemovesQueryChar(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffText
	m.query = "pas"

	m2 := updateList(m, keySpecial(tea.KeyBackspace))
	if m2.query != "pa" {
		t.Errorf("backspace on query: want \"pa\", got %q", m2.query)
	}
}

func TestTypingKey_Backspace_RemovesCourseBufferChar(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffCourses
	m.courseBuffer = "bre"

	m2 := updateList(m, keySpecial(tea.KeyBackspace))
	if m2.courseBuffer != "br" {
		t.Errorf("backspace on courseBuffer: want \"br\", got %q", m2.courseBuffer)
	}
}

func TestTypingKey_Backspace_RemovesLastPillWhenBufferEmpty(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffCourses
	m.courseBuffer = ""
	m.filterCourses = []string{"breakfast", "lunch"}

	m2 := updateList(m, keySpecial(tea.KeyBackspace))
	if len(m2.filterCourses) != 1 || m2.filterCourses[0] != "breakfast" {
		t.Errorf("backspace with empty buffer: want [breakfast], got %v", m2.filterCourses)
	}
}

func TestTypingKey_Backspace_RemovesInfluenceBufferChar(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffInfluences
	m.influenceBuffer = "ita"

	m2 := updateList(m, keySpecial(tea.KeyBackspace))
	if m2.influenceBuffer != "it" {
		t.Errorf("backspace on influenceBuffer: want \"it\", got %q", m2.influenceBuffer)
	}
}

func TestTypingKey_Backspace_RemovesLastInfluencePillWhenBufferEmpty(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffInfluences
	m.influenceBuffer = ""
	m.filterInfluences = []string{"italian", "japanese"}

	m2 := updateList(m, keySpecial(tea.KeyBackspace))
	if len(m2.filterInfluences) != 1 || m2.filterInfluences[0] != "italian" {
		t.Errorf("backspace with empty influence buffer: want [italian], got %v", m2.filterInfluences)
	}
}

// ── handleTypingKey — Rune routing ────────────────────────────────────────────

func TestTypingKey_Runes_AppendToQuery(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffText
	m.query = "pa"

	m2 := updateList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if m2.query != "pas" {
		t.Errorf("rune on text row: want \"pas\", got %q", m2.query)
	}
}

func TestTypingKey_Runes_AppendToCourseBuffer(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffCourses
	m.courseBuffer = "bre"

	m2 := updateList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if m2.courseBuffer != "brea" {
		t.Errorf("rune on courses row: want \"brea\", got %q", m2.courseBuffer)
	}
}

func TestTypingKey_Runes_AppendToInfluenceBuffer(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffInfluences
	m.influenceBuffer = "it"

	m2 := updateList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if m2.influenceBuffer != "ita" {
		t.Errorf("rune on influences row: want \"ita\", got %q", m2.influenceBuffer)
	}
}

func TestTypingKey_Runes_IgnoredOnStatusRow(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffStatus
	m.query = "original"

	m2 := updateList(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if m2.query != "original" {
		t.Errorf("rune on status row should not affect query, got %q", m2.query)
	}
}

// ── handleTypingKey — Enter ───────────────────────────────────────────────────

func TestTypingKey_Enter_AddsCoursesPillWhenBufferNonEmpty(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffCourses
	m.courseBuffer = "bre" // matches "breakfast"

	m2 := updateList(m, keySpecial(tea.KeyEnter))

	if m2.typing != true {
		t.Error("typing should remain true after adding a pill")
	}
	if m2.searchConfirmed {
		t.Error("searchConfirmed should not be set when adding a pill")
	}
	if len(m2.filterCourses) != 1 || m2.filterCourses[0] != "breakfast" {
		t.Errorf("filterCourses: want [breakfast] (resolved), got %v", m2.filterCourses)
	}
	if m2.courseBuffer != "" {
		t.Errorf("courseBuffer should be cleared after adding pill, got %q", m2.courseBuffer)
	}
}

func TestTypingKey_Enter_AddsCoursesPillExactlyAsTypedWhenNoMatch(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffCourses
	m.courseBuffer = "brunch" // no suggestion matches

	m2 := updateList(m, keySpecial(tea.KeyEnter))

	if len(m2.filterCourses) != 1 || m2.filterCourses[0] != "brunch" {
		t.Errorf("filterCourses: want [brunch], got %v", m2.filterCourses)
	}
}

func TestTypingKey_Enter_AddsInfluencePill(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffInfluences
	m.influenceBuffer = "ita" // matches "italian"

	m2 := updateList(m, keySpecial(tea.KeyEnter))

	if len(m2.filterInfluences) != 1 || m2.filterInfluences[0] != "italian" {
		t.Errorf("filterInfluences: want [italian], got %v", m2.filterInfluences)
	}
	if m2.influenceBuffer != "" {
		t.Errorf("influenceBuffer should be cleared, got %q", m2.influenceBuffer)
	}
}

func TestTypingKey_Enter_ConfirmsSearchWhenBuffersEmpty(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffText
	m.query = "soup"

	m2, cmd := m.Update(keySpecial(tea.KeyEnter))
	mm := m2.(ListModel)

	if !mm.searchConfirmed {
		t.Error("searchConfirmed should be true after Enter with empty buffers")
	}
	if mm.typing {
		t.Error("typing should be false after confirming search")
	}
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestTypingKey_Enter_ConfirmsOnStatusRowWithEmptyBuffer(t *testing.T) {
	m := newTestListModel()
	m = m.enterTypingMode()
	m.filterFocus = ffStatus
	m.filterStatus = "draft"

	m2, cmd := m.Update(keySpecial(tea.KeyEnter))
	mm := m2.(ListModel)

	if !mm.searchConfirmed {
		t.Error("searchConfirmed should be true when Enter pressed on status row")
	}
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

// ── handleNavKey — slash opens panel ─────────────────────────────────────────

func TestNavKey_Slash_OpensTypingMode(t *testing.T) {
	m := newTestListModel()
	m2 := updateList(m, keyMsg("/"))

	if !m2.typing {
		t.Error("/ should set typing=true")
	}
	if m2.filterFocus != ffText {
		t.Errorf("/ should focus ffText, got %d", m2.filterFocus)
	}
}

func TestNavKey_Slash_SavesCurrentFilter(t *testing.T) {
	m := newTestListModel()
	m.query = "chicken"
	m.filterCourses = []string{"dinner"}

	m2 := updateList(m, keyMsg("/"))

	if m2.savedQuery != "chicken" {
		t.Errorf("savedQuery: want \"chicken\", got %q", m2.savedQuery)
	}
	if len(m2.savedCourses) != 1 || m2.savedCourses[0] != "dinner" {
		t.Errorf("savedCourses: want [dinner], got %v", m2.savedCourses)
	}
}

// ── handleNavKey — Esc with active filters ────────────────────────────────────

func TestNavKey_Esc_WithActiveFilters_SetsGoHome(t *testing.T) {
	m := newTestListModel()
	m.query = "soup"

	m2, cmd := m.Update(keyMsg("esc"))
	mm := m2.(ListModel)

	if !mm.goHome {
		t.Error("esc with active filters should set goHome=true")
	}
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestNavKey_Esc_WithActiveCoursesFilter_SetsGoHome(t *testing.T) {
	m := newTestListModel()
	m.filterCourses = []string{"breakfast"}

	m2, _ := m.Update(keyMsg("esc"))
	mm := m2.(ListModel)

	if !mm.goHome {
		t.Error("esc with courses filter should set goHome=true")
	}
}

func TestNavKey_Esc_WithoutFilters_DoesNothing(t *testing.T) {
	m := newTestListModel()

	m2, cmd := m.Update(keyMsg("esc"))
	mm := m2.(ListModel)

	if mm.goHome {
		t.Error("esc without filters should not set goHome")
	}
	if cmd != nil {
		t.Error("esc without filters should not quit")
	}
}

// ── handleNavKey — up at top row ─────────────────────────────────────────────

func TestNavKey_UpAtTop_DoesNothing(t *testing.T) {
	m := newTestListModel()
	m.cursor = 0

	m2 := updateList(m, keySpecial(tea.KeyUp))

	if m2.typing {
		t.Error("up at top row should not open typing mode")
	}
	if m2.cursor != 0 {
		t.Errorf("cursor should stay at 0, got %d", m2.cursor)
	}
}

// ── Filter() accessor ─────────────────────────────────────────────────────────

func TestFilter_ReturnsCurrentState(t *testing.T) {
	m := newTestListModel()
	m.query = "pasta"
	m.filterCourses = []string{"lunch"}
	m.filterInfluences = []string{"italian"}
	m.filterStatus = "published"

	f := m.Filter()
	if f.Query != "pasta" {
		t.Errorf("Filter.Query: want \"pasta\", got %q", f.Query)
	}
	if len(f.Courses) != 1 || f.Courses[0] != "lunch" {
		t.Errorf("Filter.Courses: want [lunch], got %v", f.Courses)
	}
	if len(f.Influences) != 1 || f.Influences[0] != "italian" {
		t.Errorf("Filter.Influences: want [italian], got %v", f.Influences)
	}
	if f.Status != "published" {
		t.Errorf("Filter.Status: want \"published\", got %q", f.Status)
	}
}

// ── NewListModel — initialises from FilterState ───────────────────────────────

func TestNewListModel_InitialisesFromFilterState(t *testing.T) {
	initial := FilterState{
		Query:      "chicken",
		Courses:    []string{"dinner"},
		Influences: []string{"japanese"},
		Status:     "review",
	}
	sd := SearchData{Courses: []string{"dinner"}, Influences: []string{"japanese"}}
	m := NewListModel(nil, initial, sd)

	if m.query != "chicken" {
		t.Errorf("query: want \"chicken\", got %q", m.query)
	}
	if len(m.filterCourses) != 1 || m.filterCourses[0] != "dinner" {
		t.Errorf("filterCourses: want [dinner], got %v", m.filterCourses)
	}
	if len(m.filterInfluences) != 1 || m.filterInfluences[0] != "japanese" {
		t.Errorf("filterInfluences: want [japanese], got %v", m.filterInfluences)
	}
	if m.filterStatus != "review" {
		t.Errorf("filterStatus: want \"review\", got %q", m.filterStatus)
	}
	if len(m.allCourses) != 1 || m.allCourses[0] != "dinner" {
		t.Errorf("allCourses: want [dinner], got %v", m.allCourses)
	}
}
