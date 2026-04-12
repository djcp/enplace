package scaling

import (
	"math"
	"testing"

	"github.com/djcp/enplace/internal/models"
)

// ---- ParseQuantity --------------------------------------------------------

func TestParseQuantity(t *testing.T) {
	tests := []struct {
		input  string
		want   float64
		wantOK bool
	}{
		// Integers
		{"1", 1.0, true},
		{"2", 2.0, true},
		{"500", 500.0, true},
		{"1000", 1000.0, true},

		// Simple fractions
		{"1/2", 0.5, true},
		{"3/4", 0.75, true},
		{"1/4", 0.25, true},
		{"1/3", 1.0 / 3, true},
		{"2/3", 2.0 / 3, true},
		{"1/8", 0.125, true},
		{"3/8", 0.375, true},

		// Mixed numbers
		{"1 1/2", 1.5, true},
		{"2 3/4", 2.75, true},
		{"1 1/4", 1.25, true},
		{"3 1/3", 3 + 1.0/3, true},
		{"1  1/2", 1.5, true}, // double space

		// Unicode vulgar fractions — standalone
		{"½", 0.5, true},
		{"¼", 0.25, true},
		{"¾", 0.75, true},
		{"⅓", 1.0 / 3, true},
		{"⅔", 2.0 / 3, true},
		{"⅛", 0.125, true},
		{"⅜", 0.375, true},
		{"⅝", 0.625, true},
		{"⅞", 0.875, true},
		{"⅙", 1.0 / 6, true},
		{"⅚", 5.0 / 6, true},

		// Mixed integer + Unicode fraction
		{"1½", 1.5, true},
		{"2¼", 2.25, true},
		{"3¾", 3.75, true},
		{"1⅓", 1 + 1.0/3, true},
		{"2⅔", 2 + 2.0/3, true},

		// Ranges — hyphen (midpoint)
		{"2-3", 2.5, true},
		{"1-2", 1.5, true},
		{"1/4-1/2", 0.375, true},

		// Ranges — en-dash (U+2013)
		{"2–3", 2.5, true},
		{"1–2", 1.5, true},

		// Leading/trailing whitespace
		{"  1  ", 1.0, true},
		{"  1/2  ", 0.5, true},
		{"  1 1/2  ", 1.5, true},

		// Non-numeric — should return false
		{"to taste", 0, false},
		{"as needed", 0, false},
		{"pinch", 0, false},
		{"handful", 0, false},
		{"", 0, false},
		{"some", 0, false},
		{"a few", 0, false},

		// Zero — treated as non-numeric
		{"0", 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, ok := ParseQuantity(tc.input)
			if ok != tc.wantOK {
				t.Errorf("ParseQuantity(%q) ok=%v want %v", tc.input, ok, tc.wantOK)
				return
			}
			if !tc.wantOK {
				return
			}
			if math.Abs(got-tc.want) > 1e-9 {
				t.Errorf("ParseQuantity(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ---- FormatQuantity --------------------------------------------------------

func TestFormatQuantity_WeightUnits(t *testing.T) {
	tests := []struct {
		f    float64
		unit string
		want string
	}{
		{120.0, "g", "120"},
		{562.5, "g", "563"},
		{500.0, "g", "500"},
		{1.5, "kg", "1.5"},
		{2.0, "kg", "2"},
		{1.0, "kg", "1"},
		{0.75, "kg", "0.8"},
		{28.0, "oz", "28"},
		{28.35, "oz", "28"},
		{1.0, "lb", "1"},
		{0.5, "lb", "1"}, // rounds to nearest integer
		// Case insensitive
		{100.0, "G", "100"},
		{1.0, "KG", "1"},
	}
	for _, tc := range tests {
		got := FormatQuantity(tc.f, tc.unit)
		if got != tc.want {
			t.Errorf("FormatQuantity(%v, %q) = %q, want %q", tc.f, tc.unit, got, tc.want)
		}
	}
}

func TestFormatQuantity_VolumeAndUnitless(t *testing.T) {
	tests := []struct {
		f    float64
		unit string
		want string
	}{
		{1.5, "cup", "1 1/2"},
		{0.5, "cup", "1/2"},
		{0.333, "cup", "1/3"},
		{0.667, "cup", "2/3"},
		{0.25, "tbsp", "1/4"},
		{0.75, "tsp", "3/4"},
		{2.5, "", "2 1/2"},
		{3.0, "cup", "3"},
		{1.0, "cup", "1"},
		{0.125, "cup", "1/8"},
		{0.375, "cup", "3/8"},
		{0.625, "cup", "5/8"},
		{0.875, "cup", "7/8"},
		// Rounds to nearest fraction
		{0.34, "cup", "1/3"},
		{0.66, "cup", "2/3"},
		{0.74, "cup", "3/4"},
		{2.0, "", "2"},
		// Non-positive → ""
		{0, "cup", ""},
		{-1.0, "cup", ""},
	}
	for _, tc := range tests {
		got := FormatQuantity(tc.f, tc.unit)
		if got != tc.want {
			t.Errorf("FormatQuantity(%v, %q) = %q, want %q", tc.f, tc.unit, got, tc.want)
		}
	}
}

// ---- ScaleIngredients ------------------------------------------------------

func fptr(f float64) *float64 { return &f }

func TestScaleIngredients(t *testing.T) {
	ings := []models.RecipeIngredient{
		{Quantity: "500", QuantityNumeric: fptr(500), Unit: "g"},
		{Quantity: "325", QuantityNumeric: fptr(325), Unit: "g"},
		{Quantity: "10", QuantityNumeric: fptr(10), Unit: "g"},
		{Quantity: "to taste", QuantityNumeric: nil, Unit: ""},
	}

	scaled := ScaleIngredients(ings, 2.0)

	if len(scaled) != len(ings) {
		t.Fatalf("len=%d want %d", len(scaled), len(ings))
	}
	if *scaled[0].QuantityNumeric != 1000 {
		t.Errorf("scaled[0].QuantityNumeric = %v, want 1000", *scaled[0].QuantityNumeric)
	}
	if scaled[0].Quantity != "1000" {
		t.Errorf("scaled[0].Quantity = %q, want %q", scaled[0].Quantity, "1000")
	}
	if *scaled[1].QuantityNumeric != 650 {
		t.Errorf("scaled[1].QuantityNumeric = %v, want 650", *scaled[1].QuantityNumeric)
	}
	// nil quantity_numeric — unchanged
	if scaled[3].QuantityNumeric != nil {
		t.Errorf("scaled[3].QuantityNumeric should be nil")
	}
	if scaled[3].Quantity != "to taste" {
		t.Errorf("scaled[3].Quantity = %q, want %q", scaled[3].Quantity, "to taste")
	}
	// Original slice not mutated
	if *ings[0].QuantityNumeric != 500 {
		t.Error("original slice was mutated")
	}
}

// ---- TotalWeightGrams ------------------------------------------------------

func TestTotalWeightGrams(t *testing.T) {
	ings := []models.RecipeIngredient{
		{IngredientType: "flour", QuantityNumeric: fptr(500), Unit: "g"},
		{IngredientType: "wet", QuantityNumeric: fptr(325), Unit: "g"},
		{IngredientType: "starter", QuantityNumeric: fptr(100), Unit: "g"},
		{IngredientType: "fat", QuantityNumeric: fptr(50), Unit: "g"},
		{IngredientType: "", QuantityNumeric: fptr(10), Unit: "g"}, // unweighable — excluded
	}
	total, ok := TotalWeightGrams(ings)
	if !ok {
		t.Error("expected ok=true")
	}
	if total != 975 {
		t.Errorf("total = %v, want 975", total)
	}
}

func TestTotalWeightGrams_PartialConversion(t *testing.T) {
	ings := []models.RecipeIngredient{
		{IngredientType: "flour", QuantityNumeric: fptr(500), Unit: "g"},
		{IngredientType: "wet", QuantityNumeric: fptr(2), Unit: "cup"}, // volume — can't convert
	}
	_, ok := TotalWeightGrams(ings)
	if ok {
		t.Error("expected ok=false when a classified ingredient has a volume unit")
	}
}

func TestTotalWeightGrams_NilQuantity(t *testing.T) {
	ings := []models.RecipeIngredient{
		{IngredientType: "flour", QuantityNumeric: fptr(500), Unit: "g"},
		{IngredientType: "wet", QuantityNumeric: nil, Unit: "g"},
	}
	_, ok := TotalWeightGrams(ings)
	if ok {
		t.Error("expected ok=false when a classified ingredient has nil quantity_numeric")
	}
}

func TestTotalWeightGrams_UnitWeightG(t *testing.T) {
	uwg := 50.0
	qty := 2.0
	ings := []models.RecipeIngredient{
		{IngredientType: "flour", QuantityNumeric: fptr(400), Unit: "g"},
		{IngredientType: "wet", QuantityNumeric: &qty, Unit: "large", UnitWeightG: &uwg},
	}
	total, ok := TotalWeightGrams(ings)
	if !ok {
		t.Error("expected ok=true")
	}
	if total != 500 {
		t.Errorf("total = %v, want 500", total)
	}
}

// ---- BreadMetrics ----------------------------------------------------------

func TestBreadMetrics(t *testing.T) {
	// Salt and yeast are "dry" and included in the hydration denominator.
	// dry total = 500 (flour) + 10 (salt) + 3 (yeast) = 513
	// hydration = 325 / 513 * 100
	// flour = 100% baker's % base
	ings := []models.RecipeIngredient{
		{IngredientName: "bread flour", IngredientType: "flour", QuantityNumeric: fptr(500), Unit: "g"},
		{IngredientName: "water", IngredientType: "wet", QuantityNumeric: fptr(325), Unit: "g"},
		{IngredientName: "salt", IngredientType: "dry", QuantityNumeric: fptr(10), Unit: "g"},
		{IngredientName: "instant yeast", IngredientType: "dry", QuantityNumeric: fptr(3), Unit: "g"},
	}

	res, err := BreadMetrics(ings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantDry := 500.0 + 10.0 + 3.0
	if math.Abs(res.TotalDryGrams-wantDry) > 0.001 {
		t.Errorf("TotalDryGrams = %v, want %v", res.TotalDryGrams, wantDry)
	}
	if res.TotalFlourGrams != 500 {
		t.Errorf("TotalFlourGrams = %v, want 500", res.TotalFlourGrams)
	}
	if res.TotalWetGrams != 325 {
		t.Errorf("TotalWetGrams = %v, want 325", res.TotalWetGrams)
	}
	wantHydration := (325.0 / wantDry) * 100
	if math.Abs(res.HydrationPct-wantHydration) > 0.001 {
		t.Errorf("HydrationPct = %v, want %v", res.HydrationPct, wantHydration)
	}
	if res.StarterCount != 0 {
		t.Errorf("StarterCount = %v, want 0", res.StarterCount)
	}
	if res.ExcludedCount != 0 {
		t.Errorf("ExcludedCount = %v, want 0", res.ExcludedCount)
	}
	// All 4 typed ingredients appear in PerIngredient.
	if len(res.PerIngredient) != 4 {
		t.Errorf("len(PerIngredient) = %v, want 4", len(res.PerIngredient))
	}
	// Flour must be exactly 100%.
	for _, bp := range res.PerIngredient {
		if bp.Name == "bread flour" && math.Abs(bp.Percentage-100.0) > 0.001 {
			t.Errorf("flour baker's %% = %v, want 100", bp.Percentage)
		}
	}
}

func TestBreadMetrics_FlourIs100Pct(t *testing.T) {
	// Multiple flour types combined equal the 100% base.
	ings := []models.RecipeIngredient{
		{IngredientName: "bread flour", IngredientType: "flour", QuantityNumeric: fptr(400), Unit: "g"},
		{IngredientName: "whole wheat flour", IngredientType: "flour", QuantityNumeric: fptr(100), Unit: "g"},
		{IngredientName: "water", IngredientType: "wet", QuantityNumeric: fptr(325), Unit: "g"},
		{IngredientName: "salt", IngredientType: "dry", QuantityNumeric: fptr(10), Unit: "g"},
	}
	res, err := BreadMetrics(ings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TotalFlourGrams != 500 {
		t.Errorf("TotalFlourGrams = %v, want 500", res.TotalFlourGrams)
	}
	// TotalDryGrams = 500 (both flours) + 10 (salt)
	if res.TotalDryGrams != 510 {
		t.Errorf("TotalDryGrams = %v, want 510", res.TotalDryGrams)
	}
	// Combined flour baker's % = 100%.
	totalFlourPct := 0.0
	for _, bp := range res.PerIngredient {
		if bp.Type == "flour" {
			totalFlourPct += bp.Percentage
		}
	}
	if math.Abs(totalFlourPct-100.0) > 0.001 {
		t.Errorf("total flour baker's %% = %v, want 100", totalFlourPct)
	}
}

func TestBreadMetrics_FatExcludedFromHydration(t *testing.T) {
	// Butter (fat) does not affect hydration but appears in PerIngredient
	// and contributes to total dough weight via TotalFatGrams.
	ings := []models.RecipeIngredient{
		{IngredientName: "bread flour", IngredientType: "flour", QuantityNumeric: fptr(500), Unit: "g"},
		{IngredientName: "water", IngredientType: "wet", QuantityNumeric: fptr(325), Unit: "g"},
		{IngredientName: "butter", IngredientType: "fat", QuantityNumeric: fptr(50), Unit: "g"},
		{IngredientName: "salt", IngredientType: "dry", QuantityNumeric: fptr(10), Unit: "g"},
	}
	res, err := BreadMetrics(ings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Hydration uses only wet / (flour + other dry).
	wantDry := 500.0 + 10.0
	wantHydration := (325.0 / wantDry) * 100
	if math.Abs(res.HydrationPct-wantHydration) > 0.001 {
		t.Errorf("HydrationPct = %v, want %v", res.HydrationPct, wantHydration)
	}
	if res.TotalFatGrams != 50 {
		t.Errorf("TotalFatGrams = %v, want 50", res.TotalFatGrams)
	}
	// Butter appears in PerIngredient at 10% of flour (50/500).
	found := false
	for _, bp := range res.PerIngredient {
		if bp.Name == "butter" {
			found = true
			if bp.Type != "fat" {
				t.Errorf("butter Type = %q, want fat", bp.Type)
			}
			if math.Abs(bp.Percentage-10.0) > 0.001 {
				t.Errorf("butter baker's %% = %v, want 10", bp.Percentage)
			}
		}
	}
	if !found {
		t.Error("butter not found in PerIngredient")
	}
}

func TestBreadMetrics_WithStarter(t *testing.T) {
	// Pullman sourdough with corrected ingredient types.
	// flour: 482 + 113 = 595g
	// dry: 595 (flour) + 42 (potato flakes) + 17 (salt) + 7 (yeast) + 70 (starter dry half) = 731g
	// wet: 424 + 42 + 70 (starter wet half) = 536g
	// fat: 27 (butter) — excluded from hydration
	// hydration: 536 / 731 ≈ 73.3%
	ings := []models.RecipeIngredient{
		{IngredientName: "sourdough starter", IngredientType: "starter", QuantityNumeric: fptr(140), Unit: "g"},
		{IngredientName: "water", IngredientType: "wet", QuantityNumeric: fptr(424), Unit: "g"},
		{IngredientName: "bread flour", IngredientType: "flour", QuantityNumeric: fptr(482), Unit: "g"},
		{IngredientName: "whole wheat flour", IngredientType: "flour", QuantityNumeric: fptr(113), Unit: "g"},
		{IngredientName: "potato flakes", IngredientType: "dry", QuantityNumeric: fptr(42), Unit: "g"},
		{IngredientName: "honey", IngredientType: "wet", QuantityNumeric: fptr(42), Unit: "g"},
		{IngredientName: "butter", IngredientType: "fat", QuantityNumeric: fptr(27), Unit: "g"},
		{IngredientName: "salt", IngredientType: "dry", QuantityNumeric: fptr(17), Unit: "g"},
		{IngredientName: "yeast", IngredientType: "dry", QuantityNumeric: fptr(7), Unit: "g"},
	}

	res, err := BreadMetrics(ings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantFlour := 482.0 + 113.0
	wantDry := wantFlour + 42.0 + 17.0 + 7.0 + 70.0
	wantWet := 424.0 + 42.0 + 70.0
	wantHydration := (wantWet / wantDry) * 100

	if math.Abs(res.TotalFlourGrams-wantFlour) > 0.001 {
		t.Errorf("TotalFlourGrams = %v, want %v", res.TotalFlourGrams, wantFlour)
	}
	if math.Abs(res.TotalDryGrams-wantDry) > 0.001 {
		t.Errorf("TotalDryGrams = %v, want %v", res.TotalDryGrams, wantDry)
	}
	if math.Abs(res.TotalWetGrams-wantWet) > 0.001 {
		t.Errorf("TotalWetGrams = %v, want %v", res.TotalWetGrams, wantWet)
	}
	if res.TotalFatGrams != 27 {
		t.Errorf("TotalFatGrams = %v, want 27", res.TotalFatGrams)
	}
	if math.Abs(res.HydrationPct-wantHydration) > 0.01 {
		t.Errorf("HydrationPct = %.4f, want %.4f", res.HydrationPct, wantHydration)
	}
	if res.StarterCount != 1 {
		t.Errorf("StarterCount = %v, want 1", res.StarterCount)
	}
	if res.ExcludedCount != 0 {
		t.Errorf("ExcludedCount = %v, want 0", res.ExcludedCount)
	}
	// All 9 typed ingredients appear in PerIngredient.
	if len(res.PerIngredient) != 9 {
		t.Errorf("len(PerIngredient) = %v, want 9", len(res.PerIngredient))
	}
	// Starter appears with correct type.
	found := false
	for _, bp := range res.PerIngredient {
		if bp.Name == "sourdough starter" {
			found = true
			if bp.Type != "starter" {
				t.Errorf("starter Type = %q, want starter", bp.Type)
			}
		}
	}
	if !found {
		t.Error("sourdough starter not found in PerIngredient")
	}
}

func TestBreadMetrics_StarterOnly(t *testing.T) {
	// Starter alone keeps TotalDryGrams non-zero so hydration can be computed.
	// No flour present → PerIngredient is empty (no baker's % base).
	ings := []models.RecipeIngredient{
		{IngredientName: "levain", IngredientType: "starter", QuantityNumeric: fptr(200), Unit: "g"},
	}
	res, err := BreadMetrics(ings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TotalDryGrams != 100 {
		t.Errorf("TotalDryGrams = %v, want 100", res.TotalDryGrams)
	}
	if res.TotalWetGrams != 100 {
		t.Errorf("TotalWetGrams = %v, want 100", res.TotalWetGrams)
	}
	if math.Abs(res.HydrationPct-100.0) > 0.001 {
		t.Errorf("HydrationPct = %v, want 100", res.HydrationPct)
	}
	if res.StarterCount != 1 {
		t.Errorf("StarterCount = %v, want 1", res.StarterCount)
	}
	if len(res.PerIngredient) != 0 {
		t.Errorf("PerIngredient should be empty without flour, got %d entries", len(res.PerIngredient))
	}
}

func TestBreadMetrics_WithExclusions(t *testing.T) {
	// One wet ingredient uses a volume unit — excluded gracefully.
	ings := []models.RecipeIngredient{
		{IngredientName: "bread flour", IngredientType: "flour", QuantityNumeric: fptr(500), Unit: "g"},
		{IngredientName: "water", IngredientType: "wet", QuantityNumeric: fptr(325), Unit: "g"},
		{IngredientName: "honey", IngredientType: "wet", QuantityNumeric: fptr(2), Unit: "tbsp"},
	}
	res, err := BreadMetrics(ings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExcludedCount != 1 {
		t.Errorf("ExcludedCount = %v, want 1", res.ExcludedCount)
	}
	// Hydration only counts the water (honey in tbsp excluded).
	wantHydration := (325.0 / 500.0) * 100
	if math.Abs(res.HydrationPct-wantHydration) > 0.001 {
		t.Errorf("HydrationPct = %v, want %v", res.HydrationPct, wantHydration)
	}
}

func TestBreadMetrics_NoDryIngredients(t *testing.T) {
	ings := []models.RecipeIngredient{
		{IngredientName: "water", IngredientType: "wet", QuantityNumeric: fptr(325), Unit: "g"},
	}
	_, err := BreadMetrics(ings)
	if err == nil {
		t.Error("expected error when no flour or dry ingredients found")
	}
}

func TestBreadMetrics_UnitConversion(t *testing.T) {
	// 1 lb flour ≈ 453.592g, 16 oz water ≈ 453.592g → ~100% hydration
	ings := []models.RecipeIngredient{
		{IngredientName: "flour", IngredientType: "flour", QuantityNumeric: fptr(1), Unit: "lb"},
		{IngredientName: "water", IngredientType: "wet", QuantityNumeric: fptr(16), Unit: "oz"},
	}
	res, err := BreadMetrics(ings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(res.HydrationPct-100.0) > 0.1 {
		t.Errorf("HydrationPct = %v, want ~100", res.HydrationPct)
	}
}

func TestBreadMetrics_UnitWeightG(t *testing.T) {
	// 2 large eggs via unit_weight_g (50g each = 100g wet) + 400g flour.
	// hydration = 100 / 400 * 100 = 25%; egg baker's % = 25%
	uwg := 50.0
	qty := 2.0
	ings := []models.RecipeIngredient{
		{IngredientName: "bread flour", IngredientType: "flour", QuantityNumeric: fptr(400), Unit: "g"},
		{IngredientName: "egg", IngredientType: "wet", QuantityNumeric: &qty, Unit: "large", UnitWeightG: &uwg},
	}
	res, err := BreadMetrics(ings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TotalWetGrams != 100 {
		t.Errorf("TotalWetGrams = %v, want 100", res.TotalWetGrams)
	}
	wantHydration := (100.0 / 400.0) * 100
	if math.Abs(res.HydrationPct-wantHydration) > 0.001 {
		t.Errorf("HydrationPct = %v, want %v", res.HydrationPct, wantHydration)
	}
	if res.ExcludedCount != 0 {
		t.Errorf("ExcludedCount = %v, want 0", res.ExcludedCount)
	}
	// Egg appears in PerIngredient at 25% of flour.
	found := false
	for _, bp := range res.PerIngredient {
		if bp.Name == "egg" {
			found = true
			if bp.WeightGrams != 100 {
				t.Errorf("egg WeightGrams = %v, want 100", bp.WeightGrams)
			}
			if math.Abs(bp.Percentage-25.0) > 0.001 {
				t.Errorf("egg baker's %% = %v, want 25", bp.Percentage)
			}
		}
	}
	if !found {
		t.Error("egg not found in PerIngredient")
	}
}

func TestBreadMetrics_UnitWeightG_NoWeight(t *testing.T) {
	// An egg without unit_weight_g set is excluded gracefully.
	qty := 2.0
	ings := []models.RecipeIngredient{
		{IngredientName: "bread flour", IngredientType: "flour", QuantityNumeric: fptr(400), Unit: "g"},
		{IngredientName: "egg", IngredientType: "wet", QuantityNumeric: &qty, Unit: "large", UnitWeightG: nil},
	}
	res, err := BreadMetrics(ings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExcludedCount != 1 {
		t.Errorf("ExcludedCount = %v, want 1", res.ExcludedCount)
	}
	if res.TotalWetGrams != 0 {
		t.Errorf("TotalWetGrams = %v, want 0", res.TotalWetGrams)
	}
}
