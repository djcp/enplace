package export

import (
	"fmt"
	"strings"

	"github.com/djcp/enplace/internal/models"
	"github.com/djcp/enplace/internal/scaling"
)

// ToMarkdown renders a recipe as a Markdown document.
func ToMarkdown(r *models.Recipe, opts Options) string {
	ren := &markdownRenderer{}
	b, _ := RenderRecipe(r, opts, ren)
	return string(b)
}

type markdownRenderer struct {
	sb strings.Builder
}

func (r *markdownRenderer) Title(name string) {
	r.sb.WriteString("# " + name + "\n\n")
}

func (r *markdownRenderer) Meta(_ string, prepMins, cookMins, servings *int, servingUnits string) {
	var parts []string
	if prepMins != nil && *prepMins > 0 {
		parts = append(parts, fmt.Sprintf("**Prep:** %s", FormatMins(*prepMins)))
	}
	if cookMins != nil && *cookMins > 0 {
		parts = append(parts, fmt.Sprintf("**Cook:** %s", FormatMins(*cookMins)))
	}
	if servings != nil && *servings > 0 {
		units := servingUnits
		if units == "" {
			units = "servings"
		}
		parts = append(parts, fmt.Sprintf("**Serves:** %d %s", *servings, units))
	}
	if len(parts) > 0 {
		r.sb.WriteString(strings.Join(parts, " | ") + "\n\n")
	}
}

func (r *markdownRenderer) Hydration(pct float64, totalGrams int, starterAssumed bool) {
	r.sb.WriteString(fmt.Sprintf("**Hydration:** %.1f%%  ·  %dg total", pct, totalGrams))
	if starterAssumed {
		r.sb.WriteString("  *(100% hydration starter assumed)*")
	}
	r.sb.WriteString("\n\n")
}

func (r *markdownRenderer) Description(text string) {
	r.sb.WriteString("> " + text + "\n\n")
}

func (r *markdownRenderer) TagLine(ctxLabel, joined string) {
	r.sb.WriteString("> **" + ctxLabel + ":** " + joined + "\n")
}

func (r *markdownRenderer) IngredientsHeader() {
	r.sb.WriteString("\n## Ingredients\n\n")
}

func (r *markdownRenderer) IngredientSection(section string) {
	r.sb.WriteString("\n### " + section + "\n\n")
}

func (r *markdownRenderer) Ingredient(display string) {
	r.sb.WriteString("- " + display + "\n")
}

func (r *markdownRenderer) DirectionsHeader() {
	r.sb.WriteString("\n## Directions\n\n")
}

func (r *markdownRenderer) Directions(text string) {
	r.sb.WriteString(text + "\n")
}

func (r *markdownRenderer) SourceURL(url string) {
	r.sb.WriteString("\n---\n\nSource: " + url + "\n")
}

func (r *markdownRenderer) BreadMetricsTable(perIngredient []scaling.IngredientBakerPct, hydrationPct float64, starterAssumed bool) {
	if len(perIngredient) == 0 {
		return
	}
	r.sb.WriteString("\n## Baker's Percentages\n\n")
	r.sb.WriteString("| Ingredient | Weight | Baker's % |\n")
	r.sb.WriteString("|---|---|---|\n")
	for _, ing := range perIngredient {
		name := ing.Name
		if ing.Type == "starter" {
			name += " \\*"
		}
		grams := int(ing.WeightGrams + 0.5)
		r.sb.WriteString(fmt.Sprintf("| %s | %dg | %.1f%% |\n", name, grams, ing.Percentage))
	}
	totalG := 0
	for _, ing := range perIngredient {
		totalG += int(ing.WeightGrams + 0.5)
	}
	r.sb.WriteString(fmt.Sprintf("\n**Hydration:** %.1f%%  ·  %dg total", hydrationPct, totalG))
	if starterAssumed {
		r.sb.WriteString("  \n\\* 100% hydration starter assumed")
	}
	r.sb.WriteString("\n")
}

func (r *markdownRenderer) Footer(credits, versionStr string) {
	if credits != "" {
		r.sb.WriteString("\n<table width=\"100%\"><tr>")
		r.sb.WriteString("<td><sub>" + credits + "</sub></td>")
		r.sb.WriteString("<td align=\"right\"><sub>" + versionStr + "</sub></td>")
		r.sb.WriteString("</tr></table>\n")
	} else {
		r.sb.WriteString("\n<p align=\"right\"><sub>" + versionStr + "</sub></p>\n")
	}
}

func (r *markdownRenderer) Result() ([]byte, error) {
	return []byte(r.sb.String()), nil
}
