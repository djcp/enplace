package export

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/djcp/enplace/internal/models"
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

func (r *textRenderer) Hydration(pct float64, starterAssumed bool) {
	r.sb.WriteString(fmt.Sprintf("Hydration: %.1f%%", pct))
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
