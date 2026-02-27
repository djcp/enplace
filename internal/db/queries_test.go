package db_test

import (
	"testing"

	"github.com/djcp/gorecipes/internal/db"
	"github.com/djcp/gorecipes/internal/models"
	"github.com/jmoiron/sqlx"
)

func openTestDB(t *testing.T) *sqlx.DB {
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

func TestListRecipes_PublishedOnly(t *testing.T) {
	d := openTestDB(t)

	_, _ = db.CreateRecipe(d, &models.Recipe{Name: "Draft", Status: models.StatusDraft})
	pubID, _ := db.CreateRecipe(d, &models.Recipe{Name: "Published", Status: models.StatusPublished})

	recipes, err := db.ListRecipes(d, db.RecipeFilter{})
	if err != nil {
		t.Fatal(err)
	}

	if len(recipes) != 1 {
		t.Errorf("expected 1 published recipe, got %d", len(recipes))
	}
	if recipes[0].ID != pubID {
		t.Errorf("expected recipe ID %d, got %d", pubID, recipes[0].ID)
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
