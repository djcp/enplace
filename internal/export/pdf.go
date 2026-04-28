package export

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/djcp/enplace/internal/models"
	"github.com/djcp/enplace/internal/scaling"
	"github.com/go-pdf/fpdf"
)

// ToPDF renders a recipe as a PDF document (Letter size).
func ToPDF(r *models.Recipe, opts Options) ([]byte, error) {
	ren := newPDFRenderer()
	if _, err := RenderRecipe(r, opts, ren); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := ren.f.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type pdfRenderer struct {
	f  *fpdf.Fpdf
	tr func(string) string
	pw float64
}

func newPDFRenderer() *pdfRenderer {
	f := fpdf.New("P", "mm", "Letter", "")
	// tr converts UTF-8 strings to cp1252 (the encoding used by core PDF fonts
	// like Helvetica). Without this, multi-byte UTF-8 sequences for characters
	// such as • (U+2022) and ° (U+00B0) are misread as two Latin-1 characters,
	// producing mojibake like "â€¢" and "Â°".
	tr := f.UnicodeTranslatorFromDescriptor("")
	f.SetMargins(20, 25, 20)
	f.AddPage()
	totalW, _ := f.GetPageSize()
	return &pdfRenderer{f: f, tr: tr, pw: totalW - 40}
}

func (r *pdfRenderer) Title(name string) {
	r.f.SetFont("Helvetica", "B", 18)
	r.f.SetTextColor(201, 100, 66) // terracotta
	r.f.MultiCell(r.pw, 10, r.tr(name), "", "L", false)
	r.f.Ln(2)
}

func (r *pdfRenderer) Meta(timingSummary string, _, _ *int, servings *int, servingUnits string) {
	r.f.SetFont("Helvetica", "", 10)
	r.f.SetTextColor(142, 129, 120) // warm gray
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
		r.f.MultiCell(r.pw, 6, r.tr(strings.Join(parts, "  \u00b7  ")), "", "L", false)
	}
}

func (r *pdfRenderer) Hydration(pct float64, totalGrams int, starterAssumed bool) {
	r.f.SetFont("Helvetica", "B", 11)
	r.f.SetTextColor(201, 100, 66) // terracotta
	line := fmt.Sprintf("Hydration: %.1f%%  \u00b7  %dg total", pct, totalGrams)
	if starterAssumed {
		line += "  (100% hydration starter assumed)"
	}
	r.f.MultiCell(r.pw, 6, r.tr(line), "", "L", false)
	r.f.SetTextColor(50, 50, 50)
}

func (r *pdfRenderer) Description(text string) {
	r.f.SetFont("Helvetica", "I", 11)
	r.f.SetTextColor(92, 74, 60) // dark warm brown
	r.f.MultiCell(r.pw, 6, r.tr(text), "", "L", false)
	r.f.Ln(4)
}

func (r *pdfRenderer) TagLine(ctxLabel, joined string) {
	r.f.SetFont("Helvetica", "", 9)
	r.f.SetTextColor(142, 129, 120)
	r.f.MultiCell(r.pw, 5, r.tr(ctxLabel+": "+joined), "", "L", false)
}

func (r *pdfRenderer) IngredientsHeader() {
	r.f.Ln(4)
	r.renderPDFSection("Ingredients")
	r.f.SetFont("Helvetica", "", 11)
	r.f.SetTextColor(50, 50, 50)
}

func (r *pdfRenderer) IngredientSection(section string) {
	r.f.Ln(2)
	r.f.SetFont("Helvetica", "B", 11)
	r.f.MultiCell(r.pw, 6, r.tr(section), "", "L", false)
	r.f.SetFont("Helvetica", "", 11)
}

func (r *pdfRenderer) Ingredient(display string) {
	r.f.MultiCell(r.pw, 6, r.tr("  \u2022  "+display), "", "L", false)
}

func (r *pdfRenderer) DirectionsHeader() {
	r.f.Ln(4)
	r.renderPDFSection("Directions")
	r.f.SetFont("Helvetica", "", 11)
	r.f.SetTextColor(50, 50, 50)
}

func (r *pdfRenderer) Directions(text string) {
	r.f.MultiCell(r.pw, 6, r.tr(text), "", "L", false)
}

func (r *pdfRenderer) SourceURL(url string) {
	r.f.Ln(6)
	r.f.SetFont("Helvetica", "", 9)
	r.f.SetTextColor(142, 129, 120)
	r.f.MultiCell(r.pw, 5, r.tr("Source: "+url), "", "L", false)
}

func (r *pdfRenderer) Rating(rating int) {
	// Core PDF fonts (Helvetica) use cp1252 which lacks ★/☆; use numeric form.
	r.f.Ln(4)
	r.f.SetFont("Helvetica", "B", 11)
	r.f.SetTextColor(201, 100, 66) // terracotta
	r.f.MultiCell(r.pw, 6, r.tr(fmt.Sprintf("Rating: %d/5", rating)), "", "L", false)
	r.f.SetTextColor(50, 50, 50)
}

func (r *pdfRenderer) BreadMetricsTable(perIngredient []scaling.IngredientBakerPct, hydrationPct float64, starterAssumed bool) {
	if len(perIngredient) == 0 {
		return
	}
	r.f.Ln(4)
	r.renderPDFSection("Baker's Percentages")
	r.f.SetFont("Helvetica", "", 10)
	r.f.SetTextColor(50, 50, 50)

	// Three columns: name (fills remaining width), weight (20mm), percent (20mm).
	wt := 20.0
	pct := 20.0
	nameW := r.pw - wt - pct

	// Header row.
	r.f.SetFont("Helvetica", "B", 9)
	r.f.SetTextColor(142, 129, 120)
	r.f.CellFormat(nameW, 6, r.tr("Ingredient"), "", 0, "L", false, 0, "")
	r.f.CellFormat(wt, 6, r.tr("Weight"), "", 0, "R", false, 0, "")
	r.f.CellFormat(pct, 6, r.tr("Baker's %"), "", 1, "R", false, 0, "")

	// Data rows.
	r.f.SetFont("Helvetica", "", 10)
	r.f.SetTextColor(50, 50, 50)
	for _, ing := range perIngredient {
		name := ing.Name
		if ing.Type == "starter" {
			name += "*"
		}
		grams := int(ing.WeightGrams + 0.5)
		r.f.CellFormat(nameW, 6, r.tr(name), "", 0, "L", false, 0, "")
		r.f.CellFormat(wt, 6, r.tr(fmt.Sprintf("%dg", grams)), "", 0, "R", false, 0, "")
		r.f.CellFormat(pct, 6, r.tr(fmt.Sprintf("%.1f%%", ing.Percentage)), "", 1, "R", false, 0, "")
	}

	// Hydration summary.
	totalG := 0
	for _, ing := range perIngredient {
		totalG += int(ing.WeightGrams + 0.5)
	}
	r.f.Ln(3)
	r.f.SetFont("Helvetica", "B", 11)
	r.f.SetTextColor(201, 100, 66) // terracotta
	r.f.MultiCell(r.pw, 6, r.tr(fmt.Sprintf("Hydration: %.1f%%  \u00b7  %dg total", hydrationPct, totalG)), "", "L", false)
	if starterAssumed {
		r.f.SetFont("Helvetica", "I", 9)
		r.f.SetTextColor(142, 129, 120)
		r.f.MultiCell(r.pw, 5, r.tr("* 100% hydration starter assumed"), "", "L", false)
	}
	r.f.SetTextColor(50, 50, 50)
}

func (r *pdfRenderer) Footer(credits, versionStr string) {
	r.f.Ln(8)
	r.f.SetDrawColor(200, 200, 200)
	r.f.SetLineWidth(0.2)
	r.f.Line(r.f.GetX(), r.f.GetY(), r.f.GetX()+r.pw, r.f.GetY())
	r.f.Ln(3)
	r.f.SetFont("Helvetica", "I", 8)
	r.f.SetTextColor(128, 128, 128)
	if credits != "" {
		halfW := r.pw / 2
		r.f.CellFormat(halfW, 5, r.tr(credits), "", 0, "L", false, 0, "")
		r.f.CellFormat(halfW, 5, r.tr(versionStr), "", 1, "R", false, 0, "")
	} else {
		r.f.MultiCell(r.pw, 5, r.tr(versionStr), "", "R", false)
	}
}

func (r *pdfRenderer) Result() ([]byte, error) {
	// Actual output happens in ToPDF after calling RenderRecipe.
	return nil, nil
}

func (r *pdfRenderer) renderPDFSection(title string) {
	r.f.SetFont("Helvetica", "B", 13)
	r.f.SetTextColor(124, 158, 110) // sage green
	r.f.MultiCell(r.pw, 8, r.tr(title), "", "L", false)
	// Thin horizontal rule
	r.f.SetDrawColor(220, 213, 204) // light warm gray
	r.f.SetLineWidth(0.3)
	x := r.f.GetX()
	y := r.f.GetY()
	r.f.Line(x, y, x+r.pw, y)
	r.f.Ln(3)
}
