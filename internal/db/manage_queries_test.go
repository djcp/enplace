package db_test

import (
	"testing"
	"time"

	"github.com/djcp/enplace/internal/db"
	"github.com/djcp/enplace/internal/models"
)

// --- Tag management ---

func TestListTagsByContext_CountsAndOrder(t *testing.T) {
	d := openTestDB(t)

	r1, _ := db.CreateRecipe(d, &models.Recipe{Name: "R1", Status: models.StatusPublished})
	r2, _ := db.CreateRecipe(d, &models.Recipe{Name: "R2", Status: models.StatusPublished})

	breakfast, _ := db.FindOrCreateTag(d, "breakfast", models.TagContextCourses)
	dinner, _ := db.FindOrCreateTag(d, "dinner", models.TagContextCourses)
	// Tag in a different context — must not appear in courses list.
	_, _ = db.FindOrCreateTag(d, "bake", models.TagContextCookingMethods)

	_ = db.AttachTag(d, r1, breakfast)
	_ = db.AttachTag(d, r2, breakfast) // breakfast used by 2 recipes
	_ = db.AttachTag(d, r1, dinner)    // dinner used by 1 recipe

	rows, err := db.ListTagsByContext(d, models.TagContextCourses)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(rows))
	}
	// Results must be alphabetical.
	if rows[0].Name != "breakfast" {
		t.Errorf("row[0]: want breakfast, got %q", rows[0].Name)
	}
	if rows[0].Count != 2 {
		t.Errorf("breakfast count: want 2, got %d", rows[0].Count)
	}
	if rows[1].Name != "dinner" {
		t.Errorf("row[1]: want dinner, got %q", rows[1].Name)
	}
	if rows[1].Count != 1 {
		t.Errorf("dinner count: want 1, got %d", rows[1].Count)
	}
}

func TestListTagsByContext_ExcludesOtherContexts(t *testing.T) {
	d := openTestDB(t)
	_, _ = db.FindOrCreateTag(d, "bake", models.TagContextCookingMethods)

	rows, err := db.ListTagsByContext(d, models.TagContextCourses)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 course tags, got %d", len(rows))
	}
}

func TestRenameTag(t *testing.T) {
	d := openTestDB(t)
	id, _ := db.FindOrCreateTag(d, "supper", models.TagContextCourses)

	if err := db.RenameTag(d, id, "dinner"); err != nil {
		t.Fatal(err)
	}

	rows, _ := db.ListTagsByContext(d, models.TagContextCourses)
	if len(rows) != 1 || rows[0].Name != "dinner" {
		t.Errorf("expected [dinner], got %v", rows)
	}
}

func TestMergeTag_MovesRecipeTags(t *testing.T) {
	d := openTestDB(t)

	r1, _ := db.CreateRecipe(d, &models.Recipe{Name: "R1", Status: models.StatusPublished})
	r2, _ := db.CreateRecipe(d, &models.Recipe{Name: "R2", Status: models.StatusPublished})

	src, _ := db.FindOrCreateTag(d, "brunch", models.TagContextCourses)
	dst, _ := db.FindOrCreateTag(d, "breakfast", models.TagContextCourses)
	_ = db.AttachTag(d, r1, src) // r1 → brunch
	_ = db.AttachTag(d, r2, dst) // r2 → breakfast

	if err := db.MergeTag(d, src, dst); err != nil {
		t.Fatal(err)
	}

	// Source tag must be gone.
	rows, _ := db.ListTagsByContext(d, models.TagContextCourses)
	if len(rows) != 1 || rows[0].Name != "breakfast" {
		t.Fatalf("expected only breakfast after merge, got %v", rows)
	}
	// Both recipes must now be tagged breakfast.
	if rows[0].Count != 2 {
		t.Errorf("expected count 2 after merge, got %d", rows[0].Count)
	}

	tags1, _ := db.GetRecipeTags(d, r1)
	if len(tags1) != 1 || tags1[0].ID != dst {
		t.Errorf("r1 should carry dst tag after merge, got %v", tags1)
	}
}

func TestMergeTag_NoDuplicates(t *testing.T) {
	// A recipe already carries both tags; after merge it should have exactly one.
	d := openTestDB(t)

	r, _ := db.CreateRecipe(d, &models.Recipe{Name: "R", Status: models.StatusPublished})
	src, _ := db.FindOrCreateTag(d, "brunch", models.TagContextCourses)
	dst, _ := db.FindOrCreateTag(d, "breakfast", models.TagContextCourses)
	_ = db.AttachTag(d, r, src)
	_ = db.AttachTag(d, r, dst)

	if err := db.MergeTag(d, src, dst); err != nil {
		t.Fatal(err)
	}

	tags, _ := db.GetRecipeTags(d, r)
	if len(tags) != 1 {
		t.Errorf("expected 1 tag after dedup-merge, got %d", len(tags))
	}
}

func TestDeleteTag_CascadesRecipeTags(t *testing.T) {
	d := openTestDB(t)

	r, _ := db.CreateRecipe(d, &models.Recipe{Name: "R", Status: models.StatusPublished})
	id, _ := db.FindOrCreateTag(d, "lunch", models.TagContextCourses)
	_ = db.AttachTag(d, r, id)

	if err := db.DeleteTag(d, id); err != nil {
		t.Fatal(err)
	}

	rows, _ := db.ListTagsByContext(d, models.TagContextCourses)
	if len(rows) != 0 {
		t.Errorf("expected tag gone, got %d rows", len(rows))
	}
	// recipe_tags must cascade.
	tags, _ := db.GetRecipeTags(d, r)
	if len(tags) != 0 {
		t.Errorf("expected recipe_tags cascade delete, got %d", len(tags))
	}
}

// --- Ingredient management ---

func TestListIngredientsWithCount(t *testing.T) {
	d := openTestDB(t)

	r1, _ := db.CreateRecipe(d, &models.Recipe{Name: "R1", Status: models.StatusDraft})
	r2, _ := db.CreateRecipe(d, &models.Recipe{Name: "R2", Status: models.StatusDraft})
	butter, _ := db.FindOrCreateIngredient(d, "butter")
	flour, _ := db.FindOrCreateIngredient(d, "flour")

	_ = db.InsertRecipeIngredient(d, &models.RecipeIngredient{RecipeID: r1, IngredientID: butter})
	_ = db.InsertRecipeIngredient(d, &models.RecipeIngredient{RecipeID: r2, IngredientID: butter})
	_ = db.InsertRecipeIngredient(d, &models.RecipeIngredient{RecipeID: r1, IngredientID: flour})

	rows, err := db.ListIngredientsWithCount(d, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 ingredients, got %d", len(rows))
	}
	// Alphabetical: butter first.
	if rows[0].Name != "butter" || rows[0].Count != 2 {
		t.Errorf("butter: want count 2, got name=%q count=%d", rows[0].Name, rows[0].Count)
	}
	if rows[1].Name != "flour" || rows[1].Count != 1 {
		t.Errorf("flour: want count 1, got name=%q count=%d", rows[1].Name, rows[1].Count)
	}
}

func TestListIngredientsWithCount_Search(t *testing.T) {
	d := openTestDB(t)

	db.FindOrCreateIngredient(d, "all-purpose flour")
	db.FindOrCreateIngredient(d, "almond flour")
	db.FindOrCreateIngredient(d, "butter")

	rows, err := db.ListIngredientsWithCount(d, "flour")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("search 'flour': want 2 results, got %d", len(rows))
	}

	rows, err = db.ListIngredientsWithCount(d, "FLOUR")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("search 'FLOUR' (case-insensitive): want 2 results, got %d", len(rows))
	}

	rows, err = db.ListIngredientsWithCount(d, "zzzz")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("search 'zzzz': want 0 results, got %d", len(rows))
	}
}

func TestRenameIngredient(t *testing.T) {
	d := openTestDB(t)
	id, _ := db.FindOrCreateIngredient(d, "creme fraiche")

	if err := db.RenameIngredient(d, id, "crème fraîche"); err != nil {
		t.Fatal(err)
	}

	rows, _ := db.ListIngredientsWithCount(d, "")
	if len(rows) != 1 || rows[0].Name != "crème fraîche" {
		t.Errorf("expected renamed ingredient, got %v", rows)
	}
}

func TestMergeIngredient_ReassignsAndDeletesSource(t *testing.T) {
	d := openTestDB(t)

	r, _ := db.CreateRecipe(d, &models.Recipe{Name: "R", Status: models.StatusDraft})
	src, _ := db.FindOrCreateIngredient(d, "heavy cream")
	dst, _ := db.FindOrCreateIngredient(d, "double cream")
	_ = db.InsertRecipeIngredient(d, &models.RecipeIngredient{RecipeID: r, IngredientID: src})

	if err := db.MergeIngredient(d, src, dst); err != nil {
		t.Fatal(err)
	}

	rows, _ := db.ListIngredientsWithCount(d, "")
	if len(rows) != 1 || rows[0].Name != "double cream" {
		t.Fatalf("expected only double cream, got %v", rows)
	}
	if rows[0].Count != 1 {
		t.Errorf("expected 1 usage on dst, got %d", rows[0].Count)
	}
}

// --- Unit management ---

func TestListUnitsWithCount(t *testing.T) {
	d := openTestDB(t)

	r, _ := db.CreateRecipe(d, &models.Recipe{Name: "R", Status: models.StatusDraft})
	i1, _ := db.FindOrCreateIngredient(d, "flour")
	i2, _ := db.FindOrCreateIngredient(d, "milk")
	i3, _ := db.FindOrCreateIngredient(d, "eggs")

	_ = db.InsertRecipeIngredient(d, &models.RecipeIngredient{RecipeID: r, IngredientID: i1, Unit: "cup"})
	_ = db.InsertRecipeIngredient(d, &models.RecipeIngredient{RecipeID: r, IngredientID: i2, Unit: "cup"})
	_ = db.InsertRecipeIngredient(d, &models.RecipeIngredient{RecipeID: r, IngredientID: i3, Unit: ""}) // excluded

	rows, err := db.ListUnitsWithCount(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 distinct non-empty unit, got %d: %v", len(rows), rows)
	}
	if rows[0].Name != "cup" || rows[0].Count != 2 {
		t.Errorf("expected cup×2, got %+v", rows[0])
	}
}

func TestRenameUnit(t *testing.T) {
	d := openTestDB(t)

	r, _ := db.CreateRecipe(d, &models.Recipe{Name: "R", Status: models.StatusDraft})
	i, _ := db.FindOrCreateIngredient(d, "flour")
	_ = db.InsertRecipeIngredient(d, &models.RecipeIngredient{RecipeID: r, IngredientID: i, Unit: "c"})

	if err := db.RenameUnit(d, "c", "cup"); err != nil {
		t.Fatal(err)
	}

	units, _ := db.ListUnitsWithCount(d)
	if len(units) != 1 || units[0].Name != "cup" {
		t.Errorf("expected unit renamed to cup, got %v", units)
	}
}

func TestMergeUnit_ConsolidatesRows(t *testing.T) {
	d := openTestDB(t)

	r, _ := db.CreateRecipe(d, &models.Recipe{Name: "R", Status: models.StatusDraft})
	i1, _ := db.FindOrCreateIngredient(d, "flour")
	i2, _ := db.FindOrCreateIngredient(d, "sugar")
	_ = db.InsertRecipeIngredient(d, &models.RecipeIngredient{RecipeID: r, IngredientID: i1, Unit: "c"})
	_ = db.InsertRecipeIngredient(d, &models.RecipeIngredient{RecipeID: r, IngredientID: i2, Unit: "cup"})

	if err := db.MergeUnit(d, "c", "cup"); err != nil {
		t.Fatal(err)
	}

	units, _ := db.ListUnitsWithCount(d)
	if len(units) != 1 || units[0].Name != "cup" || units[0].Count != 2 {
		t.Errorf("expected cup×2, got %v", units)
	}
}

// --- AI run management ---

func TestListAIRunSummaries_BasicFields(t *testing.T) {
	d := openTestDB(t)

	r, _ := db.CreateRecipe(d, &models.Recipe{Name: "Pasta", Status: models.StatusPublished})
	runID, _ := db.CreateAIRun(d, &models.AIClassifierRun{
		RecipeID:     &r,
		ServiceClass: "RecipeScraper",
		AIModel:      "claude-haiku",
		Adapter:      "anthropic",
	})
	_ = db.CompleteAIRun(d, runID, "{}")

	rows, err := db.ListAIRunSummaries(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(rows))
	}
	row := rows[0]
	if row.RecipeName != "Pasta" {
		t.Errorf("recipe name: want Pasta, got %q", row.RecipeName)
	}
	if row.ServiceClass != "RecipeScraper" {
		t.Errorf("service class: got %q", row.ServiceClass)
	}
	if !row.Success {
		t.Error("expected Success=true after CompleteAIRun")
	}
	if row.DurationMS < 0 {
		t.Errorf("expected non-negative DurationMS, got %d", row.DurationMS)
	}
}

func TestListAIRunSummaries_DeletedRecipeShowsEmpty(t *testing.T) {
	d := openTestDB(t)

	r, _ := db.CreateRecipe(d, &models.Recipe{Name: "Temp", Status: models.StatusDraft})
	runID, _ := db.CreateAIRun(d, &models.AIClassifierRun{RecipeID: &r, ServiceClass: "S", Adapter: "a"})
	_ = db.CompleteAIRun(d, runID, "{}")
	_ = db.DeleteRecipe(d, r)

	rows, err := db.ListAIRunSummaries(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("run should survive recipe deletion, got %d rows", len(rows))
	}
	if rows[0].RecipeName != "" {
		t.Errorf("expected empty recipe name for deleted recipe, got %q", rows[0].RecipeName)
	}
}

func TestListAIRunSummaries_NewestFirst(t *testing.T) {
	d := openTestDB(t)

	id1, _ := db.CreateAIRun(d, &models.AIClassifierRun{ServiceClass: "First", Adapter: "a"})
	id2, _ := db.CreateAIRun(d, &models.AIClassifierRun{ServiceClass: "Second", Adapter: "a"})

	rows, _ := db.ListAIRunSummaries(d)
	if len(rows) < 2 {
		t.Fatalf("expected ≥2 rows, got %d", len(rows))
	}
	// Second was created later, so it should be rows[0].
	if rows[0].ID != id2 || rows[1].ID != id1 {
		t.Errorf("expected newest-first order: got IDs %d, %d", rows[0].ID, rows[1].ID)
	}
}

func TestGetAIRun_FullFields(t *testing.T) {
	d := openTestDB(t)

	runID, _ := db.CreateAIRun(d, &models.AIClassifierRun{
		ServiceClass: "TagClassifier",
		Adapter:      "anthropic",
		SystemPrompt: "you are a tagger",
		UserPrompt:   "classify this",
	})

	run, err := db.GetAIRun(d, runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.SystemPrompt != "you are a tagger" {
		t.Errorf("SystemPrompt: got %q", run.SystemPrompt)
	}
	if run.UserPrompt != "classify this" {
		t.Errorf("UserPrompt: got %q", run.UserPrompt)
	}
	if run.ServiceClass != "TagClassifier" {
		t.Errorf("ServiceClass: got %q", run.ServiceClass)
	}
}

func TestDeleteAIRun(t *testing.T) {
	d := openTestDB(t)

	runID, _ := db.CreateAIRun(d, &models.AIClassifierRun{ServiceClass: "S", Adapter: "a"})

	if err := db.DeleteAIRun(d, runID); err != nil {
		t.Fatal(err)
	}

	_, err := db.GetAIRun(d, runID)
	if err == nil {
		t.Error("expected error after deleting AI run, got nil")
	}
}

func TestDeleteAIRunsOlderThan_PrunesOldOnly(t *testing.T) {
	d := openTestDB(t)

	oldID, _ := db.CreateAIRun(d, &models.AIClassifierRun{ServiceClass: "Old", Adapter: "a"})
	// Back-date the old run to 31 days ago.
	_, _ = d.Exec(
		`UPDATE ai_classifier_runs SET created_at = ? WHERE id = ?`,
		time.Now().Add(-31*24*time.Hour), oldID,
	)

	_, _ = db.CreateAIRun(d, &models.AIClassifierRun{ServiceClass: "New", Adapter: "a"})

	count, err := db.DeleteAIRunsOlderThan(d, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 run pruned, got %d", count)
	}

	remaining, _ := db.ListAIRunSummaries(d)
	if len(remaining) != 1 || remaining[0].ServiceClass != "New" {
		t.Errorf("expected only New run to remain, got %v", remaining)
	}
}

func TestDeleteAIRunsOlderThan_NothingToDelete(t *testing.T) {
	d := openTestDB(t)
	_, _ = db.CreateAIRun(d, &models.AIClassifierRun{ServiceClass: "Recent", Adapter: "a"})

	count, err := db.DeleteAIRunsOlderThan(d, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 runs pruned, got %d", count)
	}
}
