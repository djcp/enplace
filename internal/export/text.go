package export

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/djcp/enplace/internal/models"
	"github.com/djcp/enplace/internal/scaling"
)

// ToText renders a recipe as plain text.
func ToText(r *models.Recipe, opts Options) string {
	ren := &textRenderer{}
	b, _ := RenderRecipe(r, opts, ren)
	return string(b)
}

type textRenderer struct {
	sb strings.Builder
}

func (r *textRenderer) Title(name string) {
	r.sb.WriteString(name + "\n")
	r.sb.WriteString(strings.Repeat("=", len([]rune(name))) + "\n")
}

func (r *textRenderer) Meta(timingSummary string, _, _ *int, servings *int, servingUnits string) {
	var parts []string
	if timingSummary != "" {
		parts = append(parts, timingSummary)
	}
	if servings != nil && *servings > 0 {
		units := servingUnits
		if units == "" {
			units = "servings"
		}
		parts = append(parts, formatServings(*servings, units))
	}
	if len(parts) > 0 {
		r.sb.WriteString(strings.Join(parts, "  ·  ") + "\n")
	}
}

func (r *textRenderer) Hydration(pct float64, totalGrams int, starterAssumed bool) {
	r.sb.WriteString(fmt.Sprintf("Hydration: %.1f%%  ·  %dg total", pct, totalGrams))
	if starterAssumed {
		r.sb.WriteString("  (100% hydration starter assumed)")
	}
	r.sb.WriteString("\n")
}

func (r *textRenderer) Description(text string) {
	r.sb.WriteString("\n" + text + "\n")
}

func (r *textRenderer) TagLine(ctxLabel, joined string) {
	r.sb.WriteString(ctxLabel + ": " + joined + "\n")
}

func (r *textRenderer) IngredientsHeader() {
	r.sb.WriteString("\nINGREDIENTS\n")
	r.sb.WriteString("-----------\n")
}

func (r *textRenderer) IngredientSection(section string) {
	r.sb.WriteString("\n  " + section + "\n")
}

func (r *textRenderer) Ingredient(display string) {
	r.sb.WriteString("  " + display + "\n")
}

func (r *textRenderer) DirectionsHeader() {
	r.sb.WriteString("\nDIRECTIONS\n")
	r.sb.WriteString("----------\n")
}

func (r *textRenderer) Directions(text string) {
	r.sb.WriteString(text + "\n")
}

func (r *textRenderer) SourceURL(url string) {
	r.sb.WriteString("\nSource: " + url + "\n")
}

func (r *textRenderer) Rating(rating int) {
	r.sb.WriteString(fmt.Sprintf("\nRating: %s  (%d/5)\n", ratingGlyphStr(rating), rating))
}

func (r *textRenderer) BreadMetricsTable(perIngredient []scaling.IngredientBakerPct, hydrationPct float64, starterAssumed bool) {
	if len(perIngredient) == 0 {
		return
	}
	r.sb.WriteString("\nBAKER'S PERCENTAGES\n")
	r.sb.WriteString("-------------------\n")

	// Compute name column width.
	maxName := 0
	for _, ing := range perIngredient {
		n := len([]rune(ing.Name))
		if ing.Type == "starter" {
			n++ // room for asterisk
		}
		if n > maxName {
			maxName = n
		}
	}
	if maxName < 10 {
		maxName = 10
	}

	for _, ing := range perIngredient {
		name := ing.Name
		if ing.Type == "starter" {
			name += "*"
		}
		grams := int(ing.WeightGrams + 0.5)
		r.sb.WriteString(fmt.Sprintf("  %-*s  %5dg  %6.1f%%\n", maxName, name, grams, ing.Percentage))
	}

	totalG := 0
	for _, ing := range perIngredient {
		totalG += int(ing.WeightGrams + 0.5)
	}
	r.sb.WriteString(fmt.Sprintf("\n  Hydration: %.1f%%  ·  %dg total\n", hydrationPct, totalG))
	if starterAssumed {
		r.sb.WriteString("  * 100% hydration starter assumed\n")
	}
}

func (r *textRenderer) Footer(credits, versionStr string) {
	if credits != "" {
		gap := 80 - len([]rune(credits)) - len([]rune(versionStr))
		if gap < 2 {
			gap = 2
		}
		r.sb.WriteString("\n" + credits + strings.Repeat(" ", gap) + versionStr + "\n")
	} else {
		r.sb.WriteString(fmt.Sprintf("\n%80s\n", versionStr))
	}
}

func (r *textRenderer) Result() ([]byte, error) {
	return []byte(r.sb.String()), nil
}

func formatServings(n int, units string) string {
	if n == 1 {
		return "Makes 1 " + units
	}
	return "Makes " + strconv.Itoa(n) + " " + units
}
