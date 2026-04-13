package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/djcp/enplace/internal/db"
	"github.com/djcp/enplace/internal/models"
)

// openTestDB opens an in-memory DB for UI-layer tests.
func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("db.OpenMemory: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// keyMsg builds a tea.KeyMsg for the given key string.
func keyMsg(key string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

func keySpecial(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

// ── ManageModel ──────────────────────────────────────────────────────────────

func TestManageModel_CursorNavigation(t *testing.T) {
	m := newManageModel()

	// j / down should advance cursor.
	m2, _ := m.Update(keyMsg("j"))
	mm := m2.(ManageModel)
	if mm.cursor != 1 {
		t.Errorf("after j: cursor want 1, got %d", mm.cursor)
	}

	// k / up should retreat.
	m3, _ := mm.Update(keyMsg("k"))
	mm = m3.(ManageModel)
	if mm.cursor != 0 {
		t.Errorf("after k: cursor want 0, got %d", mm.cursor)
	}
}

func TestManageModel_CursorDoesNotWrapBelowZero(t *testing.T) {
	m := newManageModel()
	m2, _ := m.Update(keyMsg("k"))
	mm := m2.(ManageModel)
	if mm.cursor != 0 {
		t.Errorf("cursor should stay at 0, got %d", mm.cursor)
	}
}

func TestManageModel_CursorDoesNotExceedOptions(t *testing.T) {
	m := newManageModel()
	m.cursor = len(manageOptions) - 1
	m2, _ := m.Update(keyMsg("j"))
	mm := m2.(ManageModel)
	if mm.cursor != len(manageOptions)-1 {
		t.Errorf("cursor should stay at last option, got %d", mm.cursor)
	}
}

func TestManageModel_EnterSelectsSection(t *testing.T) {
	tests := []struct {
		cursor int
		want   ManageSection
	}{
		{0, ManageSectionConfig},
		{1, ManageSectionTags},
		{2, ManageSectionIngredients},
		{3, ManageSectionUnits},
		{4, ManageSectionAIRuns},
	}
	for _, tc := range tests {
		m := newManageModel()
		m.cursor = tc.cursor
		m2, cmd := m.Update(keySpecial(tea.KeyEnter))
		mm := m2.(ManageModel)
		if mm.section != tc.want {
			t.Errorf("cursor %d: want section %d, got %d", tc.cursor, tc.want, mm.section)
		}
		if cmd == nil {
			t.Errorf("cursor %d: expected quit cmd after enter", tc.cursor)
		}
	}
}

func TestManageModel_EscReturnsBack(t *testing.T) {
	m := newManageModel()
	m.cursor = 2
	m2, _ := m.Update(keySpecial(tea.KeyEsc))
	mm := m2.(ManageModel)
	if mm.section != ManageSectionBack {
		t.Errorf("esc: want ManageSectionBack, got %d", mm.section)
	}
}

// ── manageIngredientsModel — search via DB ───────────────────────────────────

func TestManageIngredientsModel_Search(t *testing.T) {
	d := openTestDB(t)

	db.FindOrCreateIngredient(d, "all-purpose flour")
	db.FindOrCreateIngredient(d, "almond flour")
	db.FindOrCreateIngredient(d, "butter")

	m := newManageIngredientsModel(d)

	// Empty query returns all.
	if err := m.loadIngredients(); err != nil {
		t.Fatal(err)
	}
	if len(m.filtered) != 3 {
		t.Errorf("empty query: want 3, got %d", len(m.filtered))
	}

	// Substring match.
	m.searchInput.SetValue("flour")
	if err := m.loadIngredients(); err != nil {
		t.Fatal(err)
	}
	if len(m.filtered) != 2 {
		t.Errorf("'flour': want 2, got %d", len(m.filtered))
	}

	// No match.
	m.searchInput.SetValue("zzzz")
	if err := m.loadIngredients(); err != nil {
		t.Fatal(err)
	}
	if len(m.filtered) != 0 {
		t.Errorf("'zzzz': want 0, got %d", len(m.filtered))
	}
}

// ── manageAIRunsModel — delete flow ──────────────────────────────────────────

func setupAIRunsModel(t *testing.T) (manageAIRunsModel, int64) {
	t.Helper()
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("db.OpenMemory: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	r, _ := db.CreateRecipe(d, &models.Recipe{Name: "Cookies", Status: models.StatusPublished})
	runID, _ := db.CreateAIRun(d, &models.AIClassifierRun{
		RecipeID:     &r,
		ServiceClass: "RecipeScraper",
		Adapter:      "anthropic",
	})

	m := newManageAIRunsModel(d)
	if err := m.loadRuns(); err != nil {
		t.Fatalf("loadRuns: %v", err)
	}
	m.phase = manageAIRunsPhaseDeleteConfirm
	m.deleteTargetID = runID
	return m, runID
}

func TestManageAIRunsModel_DeleteConfirm_YDeletesAndReturnsToList(t *testing.T) {
	m, runID := setupAIRunsModel(t)

	m2, _ := m.Update(keyMsg("y"))
	mm := m2.(manageAIRunsModel)

	if mm.phase != manageAIRunsPhaseList {
		t.Errorf("after y: want phase list, got %d", mm.phase)
	}
	// Run should be gone from the loaded list.
	for _, r := range mm.runs {
		if r.ID == runID {
			t.Error("deleted run still appears in list after y")
		}
	}
}

func TestManageAIRunsModel_DeleteConfirm_YSetsListNotice(t *testing.T) {
	m, _ := setupAIRunsModel(t)
	m2, _ := m.Update(keyMsg("y"))
	mm := m2.(manageAIRunsModel)

	if mm.listNotice == "" {
		t.Error("expected listNotice to be set after delete, got empty")
	}
	if mm.listNoticeErr {
		t.Errorf("expected success notice, got error notice: %q", mm.listNotice)
	}
}

func TestManageAIRunsModel_DeleteConfirm_EscCancels(t *testing.T) {
	m, runID := setupAIRunsModel(t)
	m2, _ := m.Update(keySpecial(tea.KeyEsc))
	mm := m2.(manageAIRunsModel)

	if mm.phase != manageAIRunsPhaseList {
		t.Errorf("esc: want list phase, got %d", mm.phase)
	}
	// Run must still be present.
	found := false
	for _, r := range mm.runs {
		if r.ID == runID {
			found = true
		}
	}
	if !found {
		t.Error("run should still exist after cancel")
	}
}

func TestManageAIRunsModel_DeleteConfirm_NCancels(t *testing.T) {
	m, _ := setupAIRunsModel(t)
	beforeCount := len(m.runs)

	m2, _ := m.Update(keyMsg("n"))
	mm := m2.(manageAIRunsModel)

	if mm.phase != manageAIRunsPhaseList {
		t.Errorf("n: want list phase, got %d", mm.phase)
	}
	if len(mm.runs) != beforeCount {
		t.Errorf("n: run count changed from %d to %d", beforeCount, len(mm.runs))
	}
}

func TestManageAIRunsModel_DeleteConfirm_CursorClampedWhenLastItemDeleted(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })

	// Create exactly one run.
	r, _ := db.CreateRecipe(d, &models.Recipe{Name: "X", Status: models.StatusDraft})
	runID, _ := db.CreateAIRun(d, &models.AIClassifierRun{RecipeID: &r, ServiceClass: "S", Adapter: "a"})

	m := newManageAIRunsModel(d)
	_ = m.loadRuns()
	m.cursor = 0
	m.phase = manageAIRunsPhaseDeleteConfirm
	m.deleteTargetID = runID

	m2, _ := m.Update(keyMsg("y"))
	mm := m2.(manageAIRunsModel)

	if mm.cursor != 0 {
		t.Errorf("cursor should be 0 after deleting sole item, got %d", mm.cursor)
	}
	if len(mm.runs) != 0 {
		t.Errorf("expected empty run list, got %d", len(mm.runs))
	}
}

// ── manageTagsModel — phase transitions (with DB) ────────────────────────────

func TestManageTagsModel_BrowsePhase_EscReturnsToContext(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })

	m := newManageTagsModel(d)
	m.phase = manageTagsPhaseBrowse
	m.contextCursor = 0

	m2, _ := m.Update(keySpecial(tea.KeyEsc))
	mm := m2.(manageTagsModel)

	if mm.phase != manageTagsPhaseContext {
		t.Errorf("esc from browse: want context phase, got %d", mm.phase)
	}
}

func TestManageTagsModel_ContextSelect_EnterLoadsTags(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })

	_, _ = db.FindOrCreateTag(d, "dinner", models.TagContextCourses)

	m := newManageTagsModel(d)
	m.phase = manageTagsPhaseContext
	m.contextCursor = 0 // courses

	m2, _ := m.Update(keySpecial(tea.KeyEnter))
	mm := m2.(manageTagsModel)

	if mm.phase != manageTagsPhaseBrowse {
		t.Errorf("enter on context: want browse phase, got %d", mm.phase)
	}
	if len(mm.tags) != 1 || mm.tags[0].Name != "dinner" {
		t.Errorf("expected [dinner] loaded, got %v", mm.tags)
	}
}

func TestManageTagsModel_EditPhase_SavesRename(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })

	tagID, _ := db.FindOrCreateTag(d, "supper", models.TagContextCourses)

	m := newManageTagsModel(d)
	m.selectedContext = models.TagContextCourses
	m.tags = []db.TagWithCount{{ID: tagID, Name: "supper", Context: models.TagContextCourses}}
	m.tagCursor = 0
	m.phase = manageTagsPhaseEdit
	m.editInput.SetValue("dinner")

	m2, _ := m.Update(keySpecial(tea.KeyEnter))
	mm := m2.(manageTagsModel)

	if mm.phase != manageTagsPhaseResult {
		t.Errorf("enter in edit: want result phase, got %d", mm.phase)
	}
	if mm.resultErr {
		t.Errorf("unexpected error: %q", mm.resultMsg)
	}
	// Verify DB was updated.
	rows, _ := db.ListTagsByContext(d, models.TagContextCourses)
	if len(rows) != 1 || rows[0].Name != "dinner" {
		t.Errorf("DB: expected [dinner] after rename, got %v", rows)
	}
}

func TestManageTagsModel_DeleteConfirm_DeletesTag(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })

	tagID, _ := db.FindOrCreateTag(d, "lunch", models.TagContextCourses)

	m := newManageTagsModel(d)
	m.selectedContext = models.TagContextCourses
	m.tags = []db.TagWithCount{{ID: tagID, Name: "lunch"}}
	m.tagCursor = 0
	m.confirmName = "lunch"
	m.phase = manageTagsPhaseConfirm

	m2, _ := m.Update(keyMsg("y"))
	mm := m2.(manageTagsModel)

	if mm.phase != manageTagsPhaseResult {
		t.Errorf("y in delete confirm: want result phase, got %d", mm.phase)
	}
	rows, _ := db.ListTagsByContext(d, models.TagContextCourses)
	if len(rows) != 0 {
		t.Errorf("tag should be deleted, got %v", rows)
	}
}
