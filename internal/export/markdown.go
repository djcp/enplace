package export

import (
	"fmt"
	"strings"

	"github.com/djcp/enplace/internal/models"
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

func (r *markdownRenderer) Hydration(pct float64, starterAssumed bool) {
	r.sb.WriteString(fmt.Sprintf("**Hydration:** %.1f%%", pct))
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
