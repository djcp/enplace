// Package scaling provides ingredient quantity parsing, formatting, and
// scaling math for recipe scaling and bread-baking hydration calculations.
package scaling

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/djcp/enplace/internal/models"
)

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
		if ing.QuantityNumeric == nil {
			allConverted = false
			continue
		}
		g, ok := toGrams(*ing.QuantityNumeric, ing.Unit)
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
	Percentage  float64 // of total dry weight
	Type        string  // "wet", "dry", or "starter"
}

// BreadMetricsResult holds the computed bread hydration and per-ingredient
// baker's percentages.
type BreadMetricsResult struct {
	HydrationPct  float64
	TotalDryGrams float64
	TotalWetGrams float64
	StarterCount  int // number of starter ingredients (assumed 100% hydration)
	PerIngredient []IngredientBakerPct
	ExcludedCount int // classified ingredients skipped due to non-weight unit
}

// BreadMetrics computes hydration percentage and baker's percentages for a
// bread recipe.
//
//   - "dry" ingredients contribute their full weight to dry.
//   - "wet" ingredients contribute their full weight to wet.
//   - "starter" ingredients are assumed to be 100% hydration: half their weight
//     is counted as dry (flour) and half as wet (water). StarterCount is
//     incremented for each starter ingredient so callers can surface the
//     assumption to the user.
//   - Ingredients with "" type (salt, yeast, spices) are silently skipped.
//   - Classified ingredients using non-weight units are counted in
//     ExcludedCount but do not cause an error.
//   - Returns an error when total dry weight is zero.
func BreadMetrics(ingredients []models.RecipeIngredient) (BreadMetricsResult, error) {
	var res BreadMetricsResult

	for _, ing := range ingredients {
		if ing.IngredientType == "" {
			continue
		}
		if ing.QuantityNumeric == nil {
			res.ExcludedCount++
			continue
		}
		g, ok := toGrams(*ing.QuantityNumeric, ing.Unit)
		if !ok {
			res.ExcludedCount++
			continue
		}

		switch ing.IngredientType {
		case "dry":
			res.TotalDryGrams += g
		case "wet":
			res.TotalWetGrams += g
		case "starter":
			// Assume 100% hydration: 50% flour, 50% water.
			half := g / 2
			res.TotalDryGrams += half
			res.TotalWetGrams += half
			res.StarterCount++
		}
	}

	if res.TotalDryGrams == 0 {
		return res, fmt.Errorf("no dry ingredients with weight units found")
	}

	res.HydrationPct = (res.TotalWetGrams / res.TotalDryGrams) * 100

	for _, ing := range ingredients {
		if ing.IngredientType == "" || ing.QuantityNumeric == nil {
			continue
		}
		g, ok := toGrams(*ing.QuantityNumeric, ing.Unit)
		if !ok {
			continue
		}
		res.PerIngredient = append(res.PerIngredient, IngredientBakerPct{
			Name:        ing.IngredientName,
			WeightGrams: g,
			Percentage:  (g / res.TotalDryGrams) * 100,
			Type:        ing.IngredientType,
		})
	}

	return res, nil
}
