package db_test

import (
	"testing"

	"github.com/djcp/enplace/internal/db"
	"github.com/djcp/enplace/internal/models"
)

func fptr(f float64) *float64 { return &f }

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestCreateAndGetRecipe(t *testing.T) {
	d := openTestDB(t)

	r := &models.Recipe{
		Name:       "Test Pasta",
		Status:     models.StatusDraft,
		SourceURL:  "https://example.com/pasta",
		SourceText: "",
	}

	id, err := db.CreateRecipe(d, r)
	if err != nil {
		t.Fatalf("CreateRecipe() error: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}

	got, err := db.GetRecipe(d, id)
	if err != nil {
		t.Fatalf("GetRecipe() error: %v", err)
	}

	if got.Name != r.Name {
		t.Errorf("name: got %q, want %q", got.Name, r.Name)
	}
	if got.Status != r.Status {
		t.Errorf("status: got %q, want %q", got.Status, r.Status)
	}
	if got.SourceURL != r.SourceURL {
		t.Errorf("source_url: got %q, want %q", got.SourceURL, r.SourceURL)
	}
}

func TestGetRecipeByURL_Found(t *testing.T) {
	d := openTestDB(t)

	url := "https://example.com/pasta"
	id, err := db.CreateRecipe(d, &models.Recipe{Name: "Pasta", Status: models.StatusPublished, SourceURL: url})
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.GetRecipeByURL(d, url)
	if err != nil {
		t.Fatalf("GetRecipeByURL() error: %v", err)
	}
	if got == nil {
		t.Fatal("expected recipe, got nil")
	}
	if got.ID != id {
		t.Errorf("ID: got %d, want %d", got.ID, id)
	}
}

func TestGetRecipeByURL_CaseInsensitive(t *testing.T) {
	d := openTestDB(t)

	id, err := db.CreateRecipe(d, &models.Recipe{Name: "Pasta", Status: models.StatusPublished, SourceURL: "https://EXAMPLE.COM/pasta"})
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.GetRecipeByURL(d, "https://example.com/pasta")
	if err != nil {
		t.Fatalf("GetRecipeByURL() error: %v", err)
	}
	if got == nil {
		t.Fatal("expected recipe for case-insensitive match, got nil")
	}
	if got.ID != id {
		t.Errorf("ID: got %d, want %d", got.ID, id)
	}
}

func TestGetRecipeByURL_NotFound(t *testing.T) {
	d := openTestDB(t)

	got, err := db.GetRecipeByURL(d, "https://example.com/nonexistent")
	if err != nil {
		t.Fatalf("GetRecipeByURL() error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for missing URL, got recipe ID %d", got.ID)
	}
}

func TestCreateRecipe_DuplicateURL_Rejected(t *testing.T) {
	d := openTestDB(t)

	url := "https://example.com/pasta"
	_, err := db.CreateRecipe(d, &models.Recipe{Name: "Pasta", Status: models.StatusPublished, SourceURL: url})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateRecipe(d, &models.Recipe{Name: "Pasta 2", Status: models.StatusPublished, SourceURL: url})
	if err == nil {
		t.Error("expected error for duplicate URL, got nil")
	}
}

func TestCreateRecipe_DuplicateURL_CaseInsensitive(t *testing.T) {
	d := openTestDB(t)

	_, err := db.CreateRecipe(d, &models.Recipe{Name: "Pasta", Status: models.StatusPublished, SourceURL: "https://EXAMPLE.COM/pasta"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateRecipe(d, &models.Recipe{Name: "Pasta 2", Status: models.StatusPublished, SourceURL: "https://example.com/pasta"})
	if err == nil {
		t.Error("expected error for case-insensitive duplicate URL, got nil")
	}
}

func TestCreateRecipe_EmptyURL_AllowsDuplicates(t *testing.T) {
	d := openTestDB(t)

	// Paste-mode and manual recipes have empty source_url — must be allowed in multiples.
	_, err := db.CreateRecipe(d, &models.Recipe{Name: "Paste 1", Status: models.StatusPublished, SourceURL: ""})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateRecipe(d, &models.Recipe{Name: "Paste 2", Status: models.StatusPublished, SourceURL: ""})
	if err != nil {
		t.Errorf("expected nil for second empty-URL recipe, got: %v", err)
	}
}

func TestGetRecipe_NotFound(t *testing.T) {
	d := openTestDB(t)
	_, err := db.GetRecipe(d, 99999)
	if err == nil {
		t.Error("expected error for missing recipe, got nil")
	}
}

func TestUpdateRecipeStatus(t *testing.T) {
	d := openTestDB(t)

	id, err := db.CreateRecipe(d, &models.Recipe{Name: "Test", Status: models.StatusDraft})
	if err != nil {
		t.Fatal(err)
	}

	if err := db.UpdateRecipeStatus(d, id, models.StatusProcessing); err != nil {
		t.Fatalf("UpdateRecipeStatus() error: %v", err)
	}

	r, err := db.GetRecipe(d, id)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != models.StatusProcessing {
		t.Errorf("status: got %q, want %q", r.Status, models.StatusProcessing)
	}
}

func TestFindOrCreateIngredient_Idempotent(t *testing.T) {
	d := openTestDB(t)

	id1, err := db.FindOrCreateIngredient(d, "garlic")
	if err != nil {
		t.Fatal(err)
	}

	id2, err := db.FindOrCreateIngredient(d, "garlic")
	if err != nil {
		t.Fatal(err)
	}

	if id1 != id2 {
		t.Errorf("expected same ID for duplicate ingredient: %d != %d", id1, id2)
	}
}

func TestFindOrCreateIngredient_Normalizes(t *testing.T) {
	d := openTestDB(t)

	id1, err := db.FindOrCreateIngredient(d, "Garlic")
	if err != nil {
		t.Fatal(err)
	}
	id2, err := db.FindOrCreateIngredient(d, "  garlic  ")
	if err != nil {
		t.Fatal(err)
	}

	if id1 != id2 {
		t.Errorf("expected same ID for case/space normalized ingredient: %d != %d", id1, id2)
	}
}

func TestRecipeIngredients_InsertAndLoad(t *testing.T) {
	d := openTestDB(t)

	recipeID, err := db.CreateRecipe(d, &models.Recipe{Name: "Test", Status: models.StatusDraft})
	if err != nil {
		t.Fatal(err)
	}

	ingID, err := db.FindOrCreateIngredient(d, "flour")
	if err != nil {
		t.Fatal(err)
	}

	ri := &models.RecipeIngredient{
		RecipeID:     recipeID,
		IngredientID: ingID,
		Quantity:     "2",
		Unit:         "cup",
		Descriptor:   "sifted",
		Section:      "Crust",
		Position:     0,
	}
	if err := db.InsertRecipeIngredient(d, ri); err != nil {
		t.Fatalf("InsertRecipeIngredient() error: %v", err)
	}

	ris, err := db.GetRecipeIngredients(d, recipeID)
	if err != nil {
		t.Fatal(err)
	}

	if len(ris) != 1 {
		t.Fatalf("expected 1 ingredient, got %d", len(ris))
	}
	got := ris[0]
	if got.IngredientName != "flour" {
		t.Errorf("ingredient name: got %q, want %q", got.IngredientName, "flour")
	}
	if got.Quantity != "2" {
		t.Errorf("quantity: got %q, want %q", got.Quantity, "2")
	}
	if got.Unit != "cup" {
		t.Errorf("unit: got %q, want %q", got.Unit, "cup")
	}
	if got.Descriptor != "sifted" {
		t.Errorf("descriptor: got %q, want %q", got.Descriptor, "sifted")
	}
	if got.Section != "Crust" {
		t.Errorf("section: got %q, want %q", got.Section, "Crust")
	}
}

func TestDeleteRecipeIngredients(t *testing.T) {
	d := openTestDB(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "Test", Status: models.StatusDraft})
	ingID, _ := db.FindOrCreateIngredient(d, "salt")
	_ = db.InsertRecipeIngredient(d, &models.RecipeIngredient{RecipeID: recipeID, IngredientID: ingID})

	if err := db.DeleteRecipeIngredients(d, recipeID); err != nil {
		t.Fatal(err)
	}

	ris, err := db.GetRecipeIngredients(d, recipeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ris) != 0 {
		t.Errorf("expected 0 ingredients after delete, got %d", len(ris))
	}
}

func TestFindOrCreateTag_Idempotent(t *testing.T) {
	d := openTestDB(t)

	id1, err := db.FindOrCreateTag(d, "bake", models.TagContextCookingMethods)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := db.FindOrCreateTag(d, "bake", models.TagContextCookingMethods)
	if err != nil {
		t.Fatal(err)
	}

	if id1 != id2 {
		t.Errorf("expected same tag ID: %d != %d", id1, id2)
	}
}

func TestFindOrCreateTag_SameNameDifferentContext(t *testing.T) {
	d := openTestDB(t)

	id1, _ := db.FindOrCreateTag(d, "roast", models.TagContextCookingMethods)
	id2, _ := db.FindOrCreateTag(d, "roast", models.TagContextCourses)

	if id1 == id2 {
		t.Error("expected different IDs for same name in different contexts")
	}
}

func TestAttachTag_Idempotent(t *testing.T) {
	d := openTestDB(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "Test", Status: models.StatusDraft})
	tagID, _ := db.FindOrCreateTag(d, "italian", models.TagContextCulturalInfluences)

	if err := db.AttachTag(d, recipeID, tagID); err != nil {
		t.Fatal(err)
	}
	// Attach again — should not error (INSERT OR IGNORE).
	if err := db.AttachTag(d, recipeID, tagID); err != nil {
		t.Fatalf("second AttachTag() error: %v", err)
	}
}

func TestGetRecipeTags(t *testing.T) {
	d := openTestDB(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "Test", Status: models.StatusDraft})
	tag1, _ := db.FindOrCreateTag(d, "italian", models.TagContextCulturalInfluences)
	tag2, _ := db.FindOrCreateTag(d, "dinner", models.TagContextCourses)
	_ = db.AttachTag(d, recipeID, tag1)
	_ = db.AttachTag(d, recipeID, tag2)

	tags, err := db.GetRecipeTags(d, recipeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags))
	}
}

func TestDeleteRecipe(t *testing.T) {
	d := openTestDB(t)

	id, err := db.CreateRecipe(d, &models.Recipe{Name: "To Delete", Status: models.StatusPublished})
	if err != nil {
		t.Fatal(err)
	}

	if err := db.DeleteRecipe(d, id); err != nil {
		t.Fatalf("DeleteRecipe() error: %v", err)
	}

	_, err = db.GetRecipe(d, id)
	if err == nil {
		t.Error("expected error after deleting recipe, got nil")
	}
}

func TestDeleteRecipe_CascadesIngredients(t *testing.T) {
	d := openTestDB(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "Test", Status: models.StatusDraft})
	ingID, _ := db.FindOrCreateIngredient(d, "butter")
	_ = db.InsertRecipeIngredient(d, &models.RecipeIngredient{RecipeID: recipeID, IngredientID: ingID})

	if err := db.DeleteRecipe(d, recipeID); err != nil {
		t.Fatal(err)
	}

	ris, err := db.GetRecipeIngredients(d, recipeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ris) != 0 {
		t.Errorf("expected 0 ingredients after recipe delete, got %d", len(ris))
	}
}

func TestListRecipes_AllStatuses(t *testing.T) {
	d := openTestDB(t)

	_, _ = db.CreateRecipe(d, &models.Recipe{Name: "Draft", Status: models.StatusDraft})
	_, _ = db.CreateRecipe(d, &models.Recipe{Name: "Published", Status: models.StatusPublished})

	recipes, err := db.ListRecipes(d, db.RecipeFilter{})
	if err != nil {
		t.Fatal(err)
	}

	if len(recipes) != 2 {
		t.Errorf("expected 2 recipes (all statuses), got %d", len(recipes))
	}
}

func TestListRecipes_QueryFilter(t *testing.T) {
	d := openTestDB(t)

	id1, _ := db.CreateRecipe(d, &models.Recipe{Name: "Spaghetti Carbonara", Status: models.StatusPublished})
	_, _ = db.CreateRecipe(d, &models.Recipe{Name: "Chocolate Cake", Status: models.StatusPublished})

	recipes, err := db.ListRecipes(d, db.RecipeFilter{Query: "carbonara"})
	if err != nil {
		t.Fatal(err)
	}

	if len(recipes) != 1 {
		t.Errorf("expected 1 match, got %d", len(recipes))
	}
	if recipes[0].ID != id1 {
		t.Errorf("wrong recipe returned: got ID %d", recipes[0].ID)
	}
}

func TestListRecipes_TagFilter(t *testing.T) {
	d := openTestDB(t)

	id1, _ := db.CreateRecipe(d, &models.Recipe{Name: "Pizza", Status: models.StatusPublished})
	id2, _ := db.CreateRecipe(d, &models.Recipe{Name: "Sushi", Status: models.StatusPublished})

	italianTag, _ := db.FindOrCreateTag(d, "italian", models.TagContextCulturalInfluences)
	japaneseTag, _ := db.FindOrCreateTag(d, "japanese", models.TagContextCulturalInfluences)
	_ = db.AttachTag(d, id1, italianTag)
	_ = db.AttachTag(d, id2, japaneseTag)

	recipes, err := db.ListRecipes(d, db.RecipeFilter{
		CulturalInfluences: []string{"italian"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(recipes) != 1 {
		t.Fatalf("expected 1 recipe, got %d", len(recipes))
	}
	if recipes[0].ID != id1 {
		t.Errorf("wrong recipe: got %d, want %d", recipes[0].ID, id1)
	}
}

func TestAllIngredientNames(t *testing.T) {
	d := openTestDB(t)

	_, _ = db.FindOrCreateIngredient(d, "zucchini")
	_, _ = db.FindOrCreateIngredient(d, "apple")
	_, _ = db.FindOrCreateIngredient(d, "basil")

	names, err := db.AllIngredientNames(d)
	if err != nil {
		t.Fatalf("AllIngredientNames() error: %v", err)
	}
	if len(names) != 3 {
		t.Errorf("expected 3 names, got %d", len(names))
	}
	// Should be alphabetical.
	if names[0] != "apple" || names[1] != "basil" || names[2] != "zucchini" {
		t.Errorf("unexpected order: %v", names)
	}
}

func TestAllUnits(t *testing.T) {
	d := openTestDB(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "Test", Status: models.StatusDraft})
	ingID1, _ := db.FindOrCreateIngredient(d, "flour")
	ingID2, _ := db.FindOrCreateIngredient(d, "milk")
	ingID3, _ := db.FindOrCreateIngredient(d, "salt")

	_ = db.InsertRecipeIngredient(d, &models.RecipeIngredient{RecipeID: recipeID, IngredientID: ingID1, Unit: "cup"})
	_ = db.InsertRecipeIngredient(d, &models.RecipeIngredient{RecipeID: recipeID, IngredientID: ingID2, Unit: "tbsp"})
	_ = db.InsertRecipeIngredient(d, &models.RecipeIngredient{RecipeID: recipeID, IngredientID: ingID3, Unit: ""}) // empty unit excluded

	units, err := db.AllUnits(d)
	if err != nil {
		t.Fatalf("AllUnits() error: %v", err)
	}
	if len(units) != 2 {
		t.Errorf("expected 2 units (empty excluded), got %d: %v", len(units), units)
	}
}

func TestSaveRecipe_Create(t *testing.T) {
	d := openTestDB(t)

	r := &models.Recipe{
		Name:   "New Recipe",
		Status: models.StatusDraft,
	}
	r.Ingredients = []models.RecipeIngredient{
		{IngredientName: "butter", Quantity: "2", Unit: "tbsp"},
		{IngredientName: "flour", Quantity: "1", Unit: "cup"},
	}
	tagNames := map[string][]string{
		models.TagContextCourses:        {"dessert"},
		models.TagContextCookingMethods: {"bake"},
	}

	if err := db.SaveRecipe(d, r, tagNames); err != nil {
		t.Fatalf("SaveRecipe() error: %v", err)
	}
	if r.ID == 0 {
		t.Error("expected recipe ID to be set after save")
	}

	got, err := db.GetRecipe(d, r.ID)
	if err != nil {
		t.Fatalf("GetRecipe() error: %v", err)
	}
	if got.Name != "New Recipe" {
		t.Errorf("name: got %q, want %q", got.Name, "New Recipe")
	}
	if len(got.Ingredients) != 2 {
		t.Errorf("expected 2 ingredients, got %d", len(got.Ingredients))
	}
	if len(got.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(got.Tags))
	}
}

func TestSaveRecipe_Update(t *testing.T) {
	d := openTestDB(t)

	id, err := db.CreateRecipe(d, &models.Recipe{Name: "Original", Status: models.StatusDraft})
	if err != nil {
		t.Fatal(err)
	}
	// Attach an initial tag.
	tagID, _ := db.FindOrCreateTag(d, "lunch", models.TagContextCourses)
	_ = db.AttachTag(d, id, tagID)

	// Now update via SaveRecipe.
	r := &models.Recipe{
		ID:     id,
		Name:   "Updated",
		Status: models.StatusPublished,
	}
	r.Ingredients = []models.RecipeIngredient{
		{IngredientName: "garlic", Quantity: "3", Unit: "cloves"},
	}
	tagNames := map[string][]string{
		models.TagContextCourses: {"dinner"}, // replaces "lunch"
	}

	if err := db.SaveRecipe(d, r, tagNames); err != nil {
		t.Fatalf("SaveRecipe() update error: %v", err)
	}

	got, err := db.GetRecipe(d, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Updated" {
		t.Errorf("name: got %q, want %q", got.Name, "Updated")
	}
	if got.Status != models.StatusPublished {
		t.Errorf("status: got %q, want %q", got.Status, models.StatusPublished)
	}
	if len(got.Ingredients) != 1 {
		t.Errorf("expected 1 ingredient, got %d", len(got.Ingredients))
	}
	// Tags should be replaced: "lunch" gone, "dinner" present.
	courses := got.TagsByContext(models.TagContextCourses)
	if len(courses) != 1 || courses[0] != "dinner" {
		t.Errorf("expected [dinner] courses, got %v", courses)
	}
}

func TestCreateAIRun(t *testing.T) {
	d := openTestDB(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "Test", Status: models.StatusDraft})
	run := &models.AIClassifierRun{
		RecipeID:     &recipeID,
		ServiceClass: "TextExtractor",
		Adapter:      "anthropic",
		AIModel:      "claude-haiku",
		UserPrompt:   "https://example.com",
	}

	runID, err := db.CreateAIRun(d, run)
	if err != nil {
		t.Fatalf("CreateAIRun() error: %v", err)
	}
	if runID <= 0 {
		t.Errorf("expected positive run ID, got %d", runID)
	}
}

func TestCompleteAIRun(t *testing.T) {
	d := openTestDB(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "Test", Status: models.StatusDraft})
	runID, _ := db.CreateAIRun(d, &models.AIClassifierRun{
		RecipeID:     &recipeID,
		ServiceClass: "AIExtractor",
		Adapter:      "anthropic",
	})

	if err := db.CompleteAIRun(d, runID, `{"name":"Test"}`); err != nil {
		t.Fatalf("CompleteAIRun() error: %v", err)
	}
}

func TestFailAIRun(t *testing.T) {
	d := openTestDB(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "Test", Status: models.StatusDraft})
	runID, _ := db.CreateAIRun(d, &models.AIClassifierRun{
		RecipeID:     &recipeID,
		ServiceClass: "TextExtractor",
		Adapter:      "anthropic",
	})

	if err := db.FailAIRun(d, runID, "net.Error", "connection refused"); err != nil {
		t.Fatalf("FailAIRun() error: %v", err)
	}
}

// ---- IsBread round-trip -------------------------------------------------------

func TestCreateRecipe_IsBread_RoundTrip(t *testing.T) {
	d := openTestDB(t)

	id, err := db.CreateRecipe(d, &models.Recipe{
		Name:    "Sourdough Loaf",
		Status:  models.StatusDraft,
		IsBread: true,
	})
	if err != nil {
		t.Fatalf("CreateRecipe() error: %v", err)
	}
	got, err := db.GetRecipe(d, id)
	if err != nil {
		t.Fatalf("GetRecipe() error: %v", err)
	}
	if !got.IsBread {
		t.Error("IsBread: got false, want true")
	}
}

func TestCreateRecipe_IsBread_DefaultFalse(t *testing.T) {
	d := openTestDB(t)

	id, err := db.CreateRecipe(d, &models.Recipe{Name: "Pasta", Status: models.StatusDraft})
	if err != nil {
		t.Fatalf("CreateRecipe() error: %v", err)
	}
	got, err := db.GetRecipe(d, id)
	if err != nil {
		t.Fatalf("GetRecipe() error: %v", err)
	}
	if got.IsBread {
		t.Error("IsBread: got true, want false for non-bread recipe")
	}
}

func TestUpdateRecipeFields_IsBread_Persists(t *testing.T) {
	d := openTestDB(t)

	id, err := db.CreateRecipe(d, &models.Recipe{Name: "Focaccia", Status: models.StatusDraft})
	if err != nil {
		t.Fatalf("CreateRecipe() error: %v", err)
	}
	if err := db.UpdateRecipeFields(d, &models.Recipe{
		ID:      id,
		Name:    "Focaccia",
		Status:  models.StatusReview,
		IsBread: true,
	}); err != nil {
		t.Fatalf("UpdateRecipeFields() error: %v", err)
	}
	got, err := db.GetRecipe(d, id)
	if err != nil {
		t.Fatalf("GetRecipe() error: %v", err)
	}
	if !got.IsBread {
		t.Error("IsBread: got false after UpdateRecipeFields with IsBread=true")
	}
}

func TestSaveRecipe_IsBread_RoundTrip(t *testing.T) {
	d := openTestDB(t)

	r := &models.Recipe{Name: "Pizza Dough", Status: models.StatusDraft, IsBread: true}
	if err := db.SaveRecipe(d, r, nil); err != nil {
		t.Fatalf("SaveRecipe() error: %v", err)
	}
	got, err := db.GetRecipe(d, r.ID)
	if err != nil {
		t.Fatalf("GetRecipe() error: %v", err)
	}
	if !got.IsBread {
		t.Error("IsBread: got false after SaveRecipe with IsBread=true")
	}
}

// ---- QuantityNumeric round-trip -----------------------------------------------

func TestRecipeIngredients_QuantityNumeric_RoundTrip(t *testing.T) {
	d := openTestDB(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "Test", Status: models.StatusDraft})
	ingID, _ := db.FindOrCreateIngredient(d, "bread flour")

	v := 500.0
	if err := db.InsertRecipeIngredient(d, &models.RecipeIngredient{
		RecipeID:        recipeID,
		IngredientID:    ingID,
		Quantity:        "500",
		QuantityNumeric: &v,
		Unit:            "g",
	}); err != nil {
		t.Fatalf("InsertRecipeIngredient() error: %v", err)
	}

	ris, err := db.GetRecipeIngredients(d, recipeID)
	if err != nil {
		t.Fatalf("GetRecipeIngredients() error: %v", err)
	}
	if len(ris) != 1 {
		t.Fatalf("expected 1 ingredient, got %d", len(ris))
	}
	if ris[0].QuantityNumeric == nil {
		t.Fatal("QuantityNumeric: got nil, want 500")
	}
	if *ris[0].QuantityNumeric != 500 {
		t.Errorf("QuantityNumeric: got %v, want 500", *ris[0].QuantityNumeric)
	}
}

func TestRecipeIngredients_QuantityNumeric_NilPreserved(t *testing.T) {
	d := openTestDB(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "Test", Status: models.StatusDraft})
	ingID, _ := db.FindOrCreateIngredient(d, "salt")

	if err := db.InsertRecipeIngredient(d, &models.RecipeIngredient{
		RecipeID:     recipeID,
		IngredientID: ingID,
		Quantity:     "to taste",
		Unit:         "",
	}); err != nil {
		t.Fatalf("InsertRecipeIngredient() error: %v", err)
	}

	ris, err := db.GetRecipeIngredients(d, recipeID)
	if err != nil {
		t.Fatalf("GetRecipeIngredients() error: %v", err)
	}
	if ris[0].QuantityNumeric != nil {
		t.Errorf("QuantityNumeric: got %v, want nil", *ris[0].QuantityNumeric)
	}
}

// ---- IngredientType round-trip ------------------------------------------------

func TestSetIngredientType_AllValues(t *testing.T) {
	for _, typ := range []string{"wet", "dry", "starter", ""} {
		t.Run(typ, func(t *testing.T) {
			d := openTestDB(t)

			recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "R", Status: models.StatusDraft})
			ingID, _ := db.FindOrCreateIngredient(d, "test ingredient")

			if typ != "" {
				if err := db.SetIngredientType(d, ingID, typ); err != nil {
					t.Fatalf("SetIngredientType(%q) error: %v", typ, err)
				}
			}
			if err := db.InsertRecipeIngredient(d, &models.RecipeIngredient{
				RecipeID: recipeID, IngredientID: ingID, Quantity: "1", Unit: "g",
			}); err != nil {
				t.Fatalf("InsertRecipeIngredient() error: %v", err)
			}

			ris, err := db.GetRecipeIngredients(d, recipeID)
			if err != nil {
				t.Fatalf("GetRecipeIngredients() error: %v", err)
			}
			if ris[0].IngredientType != typ {
				t.Errorf("IngredientType: got %q, want %q", ris[0].IngredientType, typ)
			}
		})
	}
}

// ---- BackfillQuantityNumeric --------------------------------------------------

func TestBackfillQuantityNumeric_PopulatesNullRows(t *testing.T) {
	d := openTestDB(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "Old Recipe", Status: models.StatusDraft})
	ingID, _ := db.FindOrCreateIngredient(d, "flour")

	// Insert without quantity_numeric to simulate a pre-backfill row.
	if _, err := d.Exec(
		`INSERT INTO recipe_ingredients (recipe_id, ingredient_id, quantity, unit, position) VALUES (?, ?, ?, ?, ?)`,
		recipeID, ingID, "2", "cup", 0,
	); err != nil {
		t.Fatalf("raw insert error: %v", err)
	}

	if err := db.BackfillQuantityNumeric(d); err != nil {
		t.Fatalf("BackfillQuantityNumeric() error: %v", err)
	}

	ris, err := db.GetRecipeIngredients(d, recipeID)
	if err != nil {
		t.Fatalf("GetRecipeIngredients() error: %v", err)
	}
	if ris[0].QuantityNumeric == nil {
		t.Fatal("QuantityNumeric: still nil after backfill")
	}
	if *ris[0].QuantityNumeric != 2.0 {
		t.Errorf("QuantityNumeric: got %v, want 2.0", *ris[0].QuantityNumeric)
	}
}

func TestBackfillQuantityNumeric_SkipsNonNumeric(t *testing.T) {
	d := openTestDB(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "R", Status: models.StatusDraft})
	ingID, _ := db.FindOrCreateIngredient(d, "salt")

	if _, err := d.Exec(
		`INSERT INTO recipe_ingredients (recipe_id, ingredient_id, quantity, unit, position) VALUES (?, ?, ?, ?, ?)`,
		recipeID, ingID, "to taste", "", 0,
	); err != nil {
		t.Fatalf("raw insert error: %v", err)
	}

	if err := db.BackfillQuantityNumeric(d); err != nil {
		t.Fatalf("BackfillQuantityNumeric() error: %v", err)
	}

	ris, err := db.GetRecipeIngredients(d, recipeID)
	if err != nil {
		t.Fatalf("GetRecipeIngredients() error: %v", err)
	}
	if ris[0].QuantityNumeric != nil {
		t.Errorf("QuantityNumeric: got %v, want nil for non-numeric quantity", *ris[0].QuantityNumeric)
	}
}

func TestBackfillQuantityNumeric_IsIdempotent(t *testing.T) {
	d := openTestDB(t)

	recipeID, _ := db.CreateRecipe(d, &models.Recipe{Name: "R", Status: models.StatusDraft})
	ingID, _ := db.FindOrCreateIngredient(d, "water")

	if _, err := d.Exec(
		`INSERT INTO recipe_ingredients (recipe_id, ingredient_id, quantity, unit, position) VALUES (?, ?, ?, ?, ?)`,
		recipeID, ingID, "300", "g", 0,
	); err != nil {
		t.Fatalf("raw insert error: %v", err)
	}

	if err := db.BackfillQuantityNumeric(d); err != nil {
		t.Fatalf("first BackfillQuantityNumeric() error: %v", err)
	}
	if err := db.BackfillQuantityNumeric(d); err != nil {
		t.Fatalf("second BackfillQuantityNumeric() error: %v", err)
	}

	ris, _ := db.GetRecipeIngredients(d, recipeID)
	if *ris[0].QuantityNumeric != 300.0 {
		t.Errorf("QuantityNumeric after second backfill: got %v, want 300", *ris[0].QuantityNumeric)
	}
}
