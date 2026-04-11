package export

import (
	"strings"

	"github.com/djcp/enplace/internal/models"
	"github.com/djcp/enplace/internal/scaling"
	"github.com/djcp/enplace/internal/version"
)

// Renderer is implemented by each export format. RenderRecipe calls its
// methods in document order and then calls Result to obtain the output.
type Renderer interface {
	// Title is called first with the recipe name.
	Title(name string)

	// Meta is called when at least one timing/servings field is present.
	// timingSummary is the pre-formatted "Prep X · Cook X" string (empty if
	// neither timing field is set). prepMins and cookMins may be nil.
	Meta(timingSummary string, prepMins, cookMins, servings *int, servingUnits string)

	// Hydration is called for bread/dough recipes immediately after Meta.
	// pct is the computed hydration percentage (e.g. 65.0 for 65%).
	// starterAssumed is true when one or more starter ingredients were present
	// and counted as 100% hydration (50% flour / 50% water).
	Hydration(pct float64, starterAssumed bool)

	// TagLine is called once per non-empty tag context.
	TagLine(ctxLabel, joined string)

	// Description is called when the recipe has a description.
	Description(text string)

	// IngredientsHeader is called before the first ingredient.
	IngredientsHeader()

	// IngredientSection is called when a new non-empty section header begins.
	IngredientSection(section string)

	// Ingredient is called for each ingredient row.
	Ingredient(display string)

	// DirectionsHeader is called before the directions text.
	DirectionsHeader()

	// Directions is called when the recipe has directions.
	Directions(text string)

	// SourceURL is called when the recipe has a source URL.
	SourceURL(url string)

	// Footer is called last with the credits string (may be empty) and the
	// application version attribution string.
	Footer(credits, versionStr string)

	// Result returns the final rendered output.
	Result() ([]byte, error)
}

// RenderRecipe traverses r in document order, dispatching to ren at each
// section, then returns ren.Result().
func RenderRecipe(r *models.Recipe, opts Options, ren Renderer) ([]byte, error) {
	ren.Title(r.Name)

	timingSummary := r.TimingSummary()
	hasPrep := r.PreparationTime != nil && *r.PreparationTime > 0
	hasCook := r.CookingTime != nil && *r.CookingTime > 0
	hasServ := r.Servings != nil && *r.Servings > 0
	if timingSummary != "" || hasPrep || hasCook || hasServ {
		ren.Meta(timingSummary, r.PreparationTime, r.CookingTime, r.Servings, r.ServingUnits)
	}

	if r.IsBread {
		if bm, err := scaling.BreadMetrics(r.Ingredients); err == nil {
			ren.Hydration(bm.HydrationPct, bm.StarterCount > 0)
		}
	}

	if r.Description != "" {
		ren.Description(r.Description)
	}

	for _, ctx := range models.AllTagContexts {
		tags := r.TagsByContext(ctx)
		if len(tags) > 0 {
			ren.TagLine(TagContextLabel(ctx), strings.Join(tags, ", "))
		}
	}

	if len(r.Ingredients) > 0 {
		ren.IngredientsHeader()
		currentSection := ""
		for _, ing := range r.Ingredients {
			if ing.Section != currentSection && ing.Section != "" {
				ren.IngredientSection(ing.Section)
				currentSection = ing.Section
			}
			ren.Ingredient(ing.DisplayString())
		}
	}

	if r.Directions != "" {
		ren.DirectionsHeader()
		ren.Directions(r.Directions)
	}

	if r.SourceURL != "" {
		ren.SourceURL(r.SourceURL)
	}

	ren.Footer(opts.Credits, "exported from enplace "+version.Version)
	return ren.Result()
}
