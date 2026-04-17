// Package scaling provides ingredient quantity parsing, formatting, and
// scaling math for recipe scaling and bread-baking hydration calculations.
package scaling

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"

	"github.com/djcp/enplace/internal/models"
)

// debugHydration logs a detailed hydration calculation trace to the application
// log via slog.Default(). Remove this function and its call in BreadMetrics when done.
func debugHydration(ingredients []models.RecipeIngredient, res BreadMetricsResult) {
	log := slog.Default()

	// Per-ingredient breakdown.
	for _, ing := range ingredients {
		qty := "nil"
		if ing.QuantityNumeric != nil {
			qty = fmt.Sprintf("%.4f", *ing.QuantityNumeric)
		}
		unitWt := "nil"
		if ing.UnitWeightG != nil {
			unitWt = fmt.Sprintf("%.2f", *ing.UnitWeightG)
		}
		g, ok := effectiveWeightGrams(ing)
		gramsStr := "n/a"
		if ok {
			gramsStr = fmt.Sprintf("%.2f", g)
		}
		ingType := ing.IngredientType
		if ingType == "" {
			ingType = "(blank)"
		}

		var dryContrib, wetContrib float64
		if ok {
			switch ing.IngredientType {
			case "flour", "dry":
				dryContrib = g
			case "wet":
				wetContrib = g
			case "starter":
				dryContrib = g / 2
				wetContrib = g / 2
			}
		}

		log.Debug("hydration ingredient",
			"name", ing.IngredientName,
			"type", ingType,
			"quantity", qty,
			"unit", ing.Unit,
			"unit_weight_g", unitWt,
			"grams", gramsStr,
			"dry_contrib", fmt.Sprintf("%.2f", dryContrib),
			"wet_contrib", fmt.Sprintf("%.2f", wetContrib),
		)
	}

	// Totals and result.
	log.Debug("hydration totals",
		"flour_g", fmt.Sprintf("%.2f", res.TotalFlourGrams),
		"dry_g", fmt.Sprintf("%.2f", res.TotalDryGrams),
		"wet_g", fmt.Sprintf("%.2f", res.TotalWetGrams),
		"fat_g", fmt.Sprintf("%.2f", res.TotalFatGrams),
		"starter_count", res.StarterCount,
		"excluded_count", res.ExcludedCount,
		"hydration_pct", fmt.Sprintf("%.2f", res.HydrationPct),
		"total_dough_g", fmt.Sprintf("%.2f", res.TotalDryGrams+res.TotalWetGrams+res.TotalFatGrams),
	)

	// Baker's percentages.
	for _, p := range res.PerIngredient {
		log.Debug("hydration bakers_pct",
			"name", p.Name,
			"type", p.Type,
			"weight_g", fmt.Sprintf("%.2f", p.WeightGrams),
			"pct_of_flour", fmt.Sprintf("%.1f", p.Percentage),
		)
	}
}

// weightUnitsToGrams maps lower-cased unit strings to their gram multiplier.
var weightUnitsToGrams = map[string]float64{
	"g":  1.0,
	"kg": 1000.0,
	"oz": 28.3495,
	"lb": 453.592,
}

// IsWeightUnit reports whether unit is a known weight unit (g, kg, oz, lb).
func IsWeightUnit(unit string) bool {
	_, ok := weightUnitsToGrams[strings.ToLower(strings.TrimSpace(unit))]
	return ok
}

// toGrams converts qty in the given unit to grams.
// Returns (0, false) when unit is not a known weight unit.
func toGrams(qty float64, unit string) (float64, bool) {
	factor, ok := weightUnitsToGrams[strings.ToLower(strings.TrimSpace(unit))]
	if !ok {
		return 0, false
	}
	return qty * factor, true
}

// effectiveWeightGrams converts an ingredient to its gram weight.
// It first tries standard weight-unit conversion (g, kg, oz, lb). If the unit
// is not a weight unit but UnitWeightG is set (e.g. eggs with a per-unit gram
// weight), it multiplies QuantityNumeric by UnitWeightG instead.
// Returns (0, false) when the ingredient cannot be converted.
func effectiveWeightGrams(ing models.RecipeIngredient) (float64, bool) {
	if ing.QuantityNumeric == nil {
		return 0, false
	}
	qty := *ing.QuantityNumeric
	if g, ok := toGrams(qty, ing.Unit); ok {
		return g, true
	}
	if ing.UnitWeightG != nil && *ing.UnitWeightG > 0 {
		return qty * *ing.UnitWeightG, true
	}
	return 0, false
}

// unicodeFractions maps Unicode vulgar-fraction runes to their float64 value.
var unicodeFractions = map[rune]float64{
	'½': 1.0 / 2,
	'¼': 1.0 / 4,
	'¾': 3.0 / 4,
	'⅓': 1.0 / 3,
	'⅔': 2.0 / 3,
	'⅛': 1.0 / 8,
	'⅜': 3.0 / 8,
	'⅝': 5.0 / 8,
	'⅞': 7.0 / 8,
	'⅙': 1.0 / 6,
	'⅚': 5.0 / 6,
	'⅕': 1.0 / 5,
	'⅖': 2.0 / 5,
	'⅗': 3.0 / 5,
	'⅘': 4.0 / 5,
}

// ParseQuantity parses a recipe quantity string into a float64.
// Returns (0, false) for non-numeric values such as "to taste" or "as needed".
//
// Handled forms:
//   - integers: "2", "500"
//   - simple fractions: "1/2", "3/4"
//   - mixed numbers: "1 1/2", "2 3/4" (any amount of internal whitespace)
//   - Unicode vulgar fractions: "½", "¼", "¾", "⅓", "⅔", "⅛", …
//   - mixed with Unicode fractions: "1½", "2¼"
//   - ranges with hyphen or en-dash: "2-3", "2–3" → midpoint (2.5)
//   - zero is treated as non-numeric (returns false)
func ParseQuantity(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}

	// Normalise en-dash and minus sign to ASCII hyphen for range detection.
	s = strings.ReplaceAll(s, "\u2013", "-") // en-dash
	s = strings.ReplaceAll(s, "\u2212", "-") // minus sign

	// Range: "2-3" → midpoint. Only treat as range when the hyphen is
	// surrounded by valid quantity tokens (idx > 0 guards a bare "-").
	if idx := strings.Index(s, "-"); idx > 0 {
		lo, okLo := ParseQuantity(s[:idx])
		hi, okHi := ParseQuantity(s[idx+1:])
		if okLo && okHi && hi > lo {
			return (lo + hi) / 2, true
		}
	}

	// Scan for Unicode vulgar fractions mixed in with ASCII digits.
	var asciiBuilder strings.Builder
	var unicodeFrac float64
	var hasUnicode bool
	for _, r := range s {
		if fv, ok := unicodeFractions[r]; ok {
			unicodeFrac = fv
			hasUnicode = true
		} else {
			asciiBuilder.WriteRune(r)
		}
	}

	if hasUnicode {
		ascii := strings.TrimSpace(asciiBuilder.String())
		if ascii == "" {
			// Pure unicode fraction: "½"
			if unicodeFrac == 0 {
				return 0, false
			}
			return unicodeFrac, true
		}
		// Mixed: "1½" → integer part + fraction
		whole, ok := parseInteger(ascii)
		if !ok {
			return 0, false
		}
		result := whole + unicodeFrac
		if result == 0 {
			return 0, false
		}
		return result, true
	}

	// Split on whitespace and handle the resulting tokens.
	parts := strings.Fields(s)
	switch len(parts) {
	case 1:
		v, ok := parseToken(parts[0])
		if !ok || v == 0 {
			return 0, false
		}
		return v, true
	case 2:
		// "1 1/2": whole number followed by a fraction.
		whole, ok1 := parseInteger(parts[0])
		frac, ok2 := parseToken(parts[1])
		if !ok1 || !ok2 || frac <= 0 || frac >= 1 {
			return 0, false
		}
		result := whole + frac
		if result == 0 {
			return 0, false
		}
		return result, true
	}

	return 0, false
}

// parseToken parses a single quantity token: plain integer, decimal, or fraction.
func parseToken(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if slash := strings.Index(s, "/"); slash > 0 {
		num, ok1 := parseInteger(s[:slash])
		den, ok2 := parseInteger(s[slash+1:])
		if !ok1 || !ok2 || den == 0 {
			return 0, false
		}
		return num / den, true
	}
	return parseDecimal(s)
}

// parseInteger parses a non-negative integer string.
func parseInteger(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, false
	}
	return float64(n), true
}

// parseDecimal parses a non-negative decimal string.
func parseDecimal(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 0, false
	}
	return f, true
}

// namedFraction pairs a float value with its display string.
type namedFraction struct {
	val float64
	str string
}

// volumeFractions is the set of common fractions used for volume/unitless display.
// The sentinel {1.0, ""} means "round up to the next whole number".
var volumeFractions = []namedFraction{
	{0, ""},
	{1.0 / 8, "1/8"},
	{1.0 / 4, "1/4"},
	{1.0 / 3, "1/3"},
	{3.0 / 8, "3/8"},
	{1.0 / 2, "1/2"},
	{5.0 / 8, "5/8"},
	{2.0 / 3, "2/3"},
	{3.0 / 4, "3/4"},
	{7.0 / 8, "7/8"},
	{1.0, ""},
}

// FormatQuantity converts a scaled float64 quantity back to a display string.
//
//   - Weight units (g, oz, lb): round to nearest integer.
//   - kg: one decimal place (e.g. 1.5, 2.0 → "2").
//   - Volume and unitless quantities: round to the nearest common fraction
//     (halves, thirds, quarters, eighths).
//
// Returns "" for non-positive values.
func FormatQuantity(f float64, unit string) string {
	if f <= 0 {
		return ""
	}
	u := strings.ToLower(strings.TrimSpace(unit))

	if IsWeightUnit(u) {
		switch u {
		case "kg":
			rounded := math.Round(f*10) / 10
			if rounded == math.Trunc(rounded) {
				return strconv.Itoa(int(rounded))
			}
			return strconv.FormatFloat(rounded, 'f', 1, 64)
		default: // g, oz, lb
			return strconv.Itoa(int(math.Round(f)))
		}
	}

	// Volume / unitless: find the nearest common fraction.
	whole := math.Floor(f)
	frac := f - whole

	// Find nearest candidate in volumeFractions (including 0 and the 1-sentinel).
	bestIdx := 0
	bestDist := math.Abs(frac - volumeFractions[0].val)
	for i, cf := range volumeFractions[1:] {
		d := math.Abs(frac - cf.val)
		if d < bestDist {
			bestDist = d
			bestIdx = i + 1
		}
	}

	best := volumeFractions[bestIdx]
	if best.val == 1.0 {
		// Round up to next whole number.
		whole++
		return strconv.Itoa(int(whole))
	}

	wholeInt := int(whole)
	if best.str == "" {
		// Fraction rounded to zero — just the whole number.
		if wholeInt == 0 {
			return "1/8" // never display "0"; minimum useful quantity
		}
		return strconv.Itoa(wholeInt)
	}
	if wholeInt == 0 {
		return best.str
	}
	return fmt.Sprintf("%d %s", wholeInt, best.str)
}

// ScaleIngredients returns a copy of ingredients with quantities multiplied by
// factor. Ingredients with nil QuantityNumeric are left unchanged (their
// display string is preserved as-is). The returned slice shares no backing
// arrays with the input.
func ScaleIngredients(ingredients []models.RecipeIngredient, factor float64) []models.RecipeIngredient {
	out := make([]models.RecipeIngredient, len(ingredients))
	for i, ing := range ingredients {
		out[i] = ing
		if ing.QuantityNumeric == nil {
			continue
		}
		scaled := *ing.QuantityNumeric * factor
		out[i].QuantityNumeric = &scaled
		out[i].Quantity = FormatQuantity(scaled, ing.Unit)
	}
	return out
}

// TotalWeightGrams returns the total gram weight of all classified (wet, dry,
// or starter) ingredients. The second return value is true only when every
// classified ingredient was successfully converted to grams; false means at
// least one classified ingredient used a non-weight unit and was skipped.
func TotalWeightGrams(ingredients []models.RecipeIngredient) (float64, bool) {
	total := 0.0
	allConverted := true
	for _, ing := range ingredients {
		if ing.IngredientType == "" {
			continue // salt, yeast, spices — excluded by design
		}
		g, ok := effectiveWeightGrams(ing)
		if !ok {
			allConverted = false
			continue
		}
		total += g
	}
	return total, allConverted
}

// IngredientBakerPct holds the baker's-percentage result for one ingredient.
type IngredientBakerPct struct {
	Name        string
	WeightGrams float64
	Percentage  float64 // of total flour weight
	Type        string  // "flour", "dry", "wet", "starter", or "fat"
}

// BreadMetricsResult holds the computed bread hydration and per-ingredient
// baker's percentages.
//
// Hydration is computed as TotalWetGrams / TotalDryGrams where TotalDryGrams
// includes flour, all other dry ingredients (oats, seeds, salt, yeast, etc.),
// and the dry half of any starters. Fats (butter, lard, etc.) are excluded from
// the hydration ratio but are included in PerIngredient and TotalFatGrams so
// callers can display total dough weight.
//
// Baker's percentages use TotalFlourGrams as the 100% base. PerIngredient is
// only populated when at least one "flour" ingredient is present.
type BreadMetricsResult struct {
	HydrationPct    float64
	TotalFlourGrams float64 // flour only — baker's percentage base (= 100%)
	TotalDryGrams   float64 // flour + other dry + starter dry half — hydration denominator
	TotalWetGrams   float64 // wet ingredients + starter wet half — hydration numerator
	TotalFatGrams   float64 // excluded fats (butter, lard, etc.) — for total dough weight
	StarterCount    int     // number of starter ingredients (assumed 100% hydration)
	PerIngredient   []IngredientBakerPct
	ExcludedCount   int // typed ingredients skipped due to non-convertible units
}

// BreadMetrics computes hydration percentage and baker's percentages for a
// bread recipe.
//
// Ingredient type semantics:
//   - "flour": any flour (AP, bread, whole wheat, rye, etc.). Contributes to
//     TotalFlourGrams and TotalDryGrams. Used as the 100% base for baker's %.
//   - "dry": non-flour dry ingredients (oats, seeds, sugar, salt, yeast, etc.).
//     Contributes to TotalDryGrams.
//   - "wet": liquids (water, milk, eggs, oil, honey, etc.).
//     Contributes to TotalWetGrams.
//   - "starter": pre-ferments assumed to be 100% hydration. Half contributes to
//     TotalDryGrams, half to TotalWetGrams. StarterCount is incremented.
//   - "fat": saturated fats (butter, lard, margarine, etc.) excluded from the
//     hydration ratio. Contributes to TotalFatGrams only.
//   - "": truly unweighable items (herb sprigs, whole spices). Silently skipped.
//
// Hydration = TotalWetGrams / TotalDryGrams × 100.
//
// Baker's percentages use TotalFlourGrams as the 100% base. PerIngredient
// covers all typed ingredients (flour, dry, wet, starter, fat) expressed as a
// percentage of flour weight. PerIngredient is empty when no flour is present.
//
// Ingredients whose units cannot be converted to grams are counted in
// ExcludedCount and otherwise skipped — they do not cause an error.
//
// Returns an error when TotalDryGrams is zero (nothing to compute hydration from).
func BreadMetrics(ingredients []models.RecipeIngredient) (BreadMetricsResult, error) {
	var res BreadMetricsResult

	for _, ing := range ingredients {
		if ing.IngredientType == "" {
			continue
		}
		g, ok := effectiveWeightGrams(ing)
		if !ok {
			res.ExcludedCount++
			continue
		}

		switch ing.IngredientType {
		case "flour":
			res.TotalFlourGrams += g
			res.TotalDryGrams += g
		case "dry":
			res.TotalDryGrams += g
		case "wet":
			res.TotalWetGrams += g
		case "starter":
			// Assume 100% hydration: 50% flour equivalent, 50% water equivalent.
			half := g / 2
			res.TotalDryGrams += half
			res.TotalWetGrams += half
			res.StarterCount++
		case "fat":
			res.TotalFatGrams += g
		default:
			res.ExcludedCount++
		}
	}

	if res.TotalDryGrams == 0 {
		return res, fmt.Errorf("no flour or dry ingredients with weight units found")
	}

	res.HydrationPct = (res.TotalWetGrams / res.TotalDryGrams) * 100

	// Baker's percentages require flour as the 100% base.
	if res.TotalFlourGrams > 0 {
		for _, ing := range ingredients {
			if ing.IngredientType == "" {
				continue
			}
			g, ok := effectiveWeightGrams(ing)
			if !ok {
				continue
			}
			res.PerIngredient = append(res.PerIngredient, IngredientBakerPct{
				Name:        ing.IngredientName,
				WeightGrams: g,
				Percentage:  (g / res.TotalFlourGrams) * 100,
				Type:        ing.IngredientType,
			})
		}
	}

	debugHydration(ingredients, res) // TODO: remove when debugging is done
	return res, nil
}
