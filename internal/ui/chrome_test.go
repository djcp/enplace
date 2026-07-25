package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// ── panelTop / panelBottom ───────────────────────────────────────────────────

func TestPanelTop_ExactWidth(t *testing.T) {
	for _, w := range []int{20, 40, 80, 120} {
		got := panelTop(w, "title", "right", ColorBorder, ColorPrimary)
		if lw := lipgloss.Width(got); lw != w {
			t.Errorf("panelTop width %d: got display width %d", w, lw)
		}
	}
}

func TestPanelTop_EmojiTitleWidth(t *testing.T) {
	got := panelTop(60, "🍳 enplace", "", ColorBorder, ColorPrimary)
	if lw := lipgloss.Width(got); lw != 60 {
		t.Errorf("emoji title: want width 60, got %d", lw)
	}
}

func TestPanelTop_TitlesPresent(t *testing.T) {
	got := stripANSI(panelTop(60, "left", "right", ColorBorder, ColorPrimary))
	for _, want := range []string{"╭", "╮", "┫ left ┣", "┫ right ┣"} {
		if !strings.Contains(got, want) {
			t.Errorf("panelTop missing %q in %q", want, got)
		}
	}
}

func TestPanelTop_NarrowDegradesToPlainBorder(t *testing.T) {
	got := stripANSI(panelTop(6, "a very long title", "", ColorBorder, ColorPrimary))
	if lw := lipgloss.Width(got); lw != 6 {
		t.Errorf("narrow panelTop: want width 6, got %d", lw)
	}
	if strings.Contains(got, "┫") {
		t.Errorf("narrow panelTop should drop the title, got %q", got)
	}
}

func TestPanelBottom_ExactWidth(t *testing.T) {
	for _, title := range []string{"", "1–20 of 27"} {
		got := panelBottom(50, title, ColorBorder, ColorSecondary)
		if lw := lipgloss.Width(got); lw != 50 {
			t.Errorf("panelBottom(title=%q): want width 50, got %d", title, lw)
		}
	}
	got := stripANSI(panelBottom(50, "5 recipes", ColorBorder, ColorSecondary))
	for _, want := range []string{"╰", "╯", "┫ 5 recipes ┣"} {
		if !strings.Contains(got, want) {
			t.Errorf("panelBottom missing %q in %q", want, got)
		}
	}
}

// ── framePanel ───────────────────────────────────────────────────────────────

func TestFramePanel_Geometry(t *testing.T) {
	content := "line one\nline two"
	got := framePanel(content, 40, 5, "top", "", "bottom", ColorBorder, ColorPrimary)
	lines := strings.Split(got, "\n")

	if len(lines) != 7 { // top border + 5 content lines + bottom border
		t.Fatalf("framePanel: want 7 lines, got %d", len(lines))
	}
	for i, line := range lines {
		if lw := lipgloss.Width(line); lw != 40 {
			t.Errorf("framePanel line %d: want width 40, got %d (%q)", i, lw, stripANSI(line))
		}
	}
}

func TestFramePanel_PadsShortContent(t *testing.T) {
	got := framePanel("only", 20, 4, "", "", "", ColorBorder, ColorPrimary)
	lines := strings.Split(got, "\n")
	if len(lines) != 6 {
		t.Fatalf("want 6 lines, got %d", len(lines))
	}
	// Line 4 (index 3) is padding — border + spaces + border.
	plain := stripANSI(lines[3])
	if strings.TrimSpace(strings.Trim(plain, "│")) != "" {
		t.Errorf("padding line should be blank inside borders, got %q", plain)
	}
}

func TestFramePanel_ClipsOverflow(t *testing.T) {
	// More content lines than innerHeight, and a line wider than the panel.
	content := strings.Repeat("x", 100) + "\nb\nc\nd\ne"
	got := framePanel(content, 30, 2, "", "", "", ColorBorder, ColorPrimary)
	lines := strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Fatalf("want 4 lines (2 borders + 2 content), got %d", len(lines))
	}
	for i, line := range lines {
		if lw := lipgloss.Width(line); lw != 30 {
			t.Errorf("line %d: want width 30, got %d", i, lw)
		}
	}
}

// ── flatRule / sectionRule ───────────────────────────────────────────────────

func TestFlatRule_ExactWidth(t *testing.T) {
	cases := []struct{ title, right string }{
		{"", ""},
		{"🍳 enplace", ""},
		{"🍳 enplace / manage / tags", ""},
		{"scale", "×2.5"},
	}
	for _, c := range cases {
		got := flatRule(80, c.title, c.right, ColorBorder, ColorPrimary)
		if lw := lipgloss.Width(got); lw != 80 {
			t.Errorf("flatRule(%q, %q): want width 80, got %d", c.title, c.right, lw)
		}
	}
}

func TestFlatRule_NarrowDegradesToPlainRule(t *testing.T) {
	got := flatRule(5, "much too long a title", "", ColorBorder, ColorPrimary)
	if lw := lipgloss.Width(got); lw != 5 {
		t.Errorf("narrow flatRule: want width 5, got %d", lw)
	}
}

func TestFlatRuleStyled_PreStyledTitleWidth(t *testing.T) {
	styled := breadcrumbTitle("Some Recipe / edit")
	got := flatRuleStyled(90, styled, "", ColorBorder, ColorPrimary)
	if lw := lipgloss.Width(got); lw != 90 {
		t.Errorf("flatRuleStyled: want width 90, got %d", lw)
	}
	plain := stripANSI(got)
	if !strings.Contains(plain, "enplace / Some Recipe / edit") {
		t.Errorf("breadcrumb missing from rule: %q", plain)
	}
}

func TestSectionRule_ExactWidthAndLabel(t *testing.T) {
	got := sectionRule(60, "Ingredients")
	if lw := lipgloss.Width(got); lw != 60 {
		t.Errorf("sectionRule: want width 60, got %d", lw)
	}
	plain := stripANSI(got)
	if !strings.Contains(plain, "┤ Ingredients ├") {
		t.Errorf("sectionRule missing label brackets: %q", plain)
	}
}

// ── keyHint ──────────────────────────────────────────────────────────────────

func TestKeyHint_ContainsKeyAndLabel(t *testing.T) {
	plain := stripANSI(keyHint("enter", "view"))
	if plain != "enter view" {
		t.Errorf("keyHint: want %q, got %q", "enter view", plain)
	}
}

// ── renderGauge ──────────────────────────────────────────────────────────────

func TestRenderGauge_ExactWidth(t *testing.T) {
	for _, frac := range []float64{-1, 0, 0.25, 0.5, 0.999, 1, 2} {
		got := renderGauge(frac, 24)
		if lw := lipgloss.Width(got); lw != 24 {
			t.Errorf("renderGauge(frac=%v): want width 24, got %d", frac, lw)
		}
	}
}

func TestRenderGauge_FillProportions(t *testing.T) {
	empty := stripANSI(renderGauge(0, 20))
	if strings.Contains(empty, "█") {
		t.Errorf("frac=0 gauge should have no full cells: %q", empty)
	}
	full := stripANSI(renderGauge(1, 20))
	if got := strings.Count(full, "█"); got != 20 {
		t.Errorf("frac=1 gauge: want 20 full cells, got %d (%q)", got, full)
	}
	half := stripANSI(renderGauge(0.5, 20))
	if got := strings.Count(half, "█"); got != 10 {
		t.Errorf("frac=0.5 gauge: want 10 full cells, got %d (%q)", got, half)
	}
	if !strings.Contains(half, "░") {
		t.Errorf("partial gauge should shade the remainder: %q", half)
	}
}

func TestRenderGauge_MinimumWidth(t *testing.T) {
	got := renderGauge(0.5, 1)
	if lw := lipgloss.Width(got); lw != 4 {
		t.Errorf("gauge width clamps to 4, got %d", lw)
	}
}

// ── renderBar ────────────────────────────────────────────────────────────────

func TestRenderBar_ExactWidth(t *testing.T) {
	for _, frac := range []float64{-0.5, 0, 0.01, 0.37, 1, 1.5} {
		got := renderBar(frac, 30, ColorPrimary)
		if lw := lipgloss.Width(got); lw != 30 {
			t.Errorf("renderBar(frac=%v): want width 30, got %d", frac, lw)
		}
	}
}

func TestRenderBar_TinyValueStillVisible(t *testing.T) {
	got := stripANSI(renderBar(0.001, 30, ColorPrimary))
	if strings.TrimSpace(got) == "" {
		t.Error("non-zero value must render a visible sliver, got blank bar")
	}
}

func TestRenderBar_ZeroIsBlank(t *testing.T) {
	got := stripANSI(renderBar(0, 30, ColorPrimary))
	if strings.TrimSpace(got) != "" {
		t.Errorf("frac=0 should render a blank bar, got %q", got)
	}
}

// ── gauge colour ramp ────────────────────────────────────────────────────────

var hexColorRe = regexp.MustCompile(`^#[0-9A-F]{6}$`)

func TestGaugeColor_ValidHexAndClamped(t *testing.T) {
	for _, tt := range []float64{-1, 0, 0.33, 0.66, 1, 2} {
		c := string(gaugeColor(tt))
		if !hexColorRe.MatchString(c) {
			t.Errorf("gaugeColor(%v): not a #RRGGBB colour: %q", tt, c)
		}
	}
	if gaugeColor(-5) != gaugeColor(0) {
		t.Error("t < 0 should clamp to the first ramp stop")
	}
	if gaugeColor(5) != gaugeColor(1) {
		t.Error("t > 1 should clamp to the last ramp stop")
	}
}

// ── hydrationGaugeFrac ───────────────────────────────────────────────────────

func TestHydrationGaugeFrac_Mapping(t *testing.T) {
	cases := []struct {
		pct  float64
		want float64
	}{
		{50, 0},
		{105, 1},
		{77.5, 0.5},
	}
	for _, c := range cases {
		if got := hydrationGaugeFrac(c.pct); got != c.want {
			t.Errorf("hydrationGaugeFrac(%v): want %v, got %v", c.pct, c.want, got)
		}
	}
}

// ── ingredientTypeColor ──────────────────────────────────────────────────────

func TestIngredientTypeColor_Mapping(t *testing.T) {
	cases := map[string]lipgloss.TerminalColor{
		"flour":   ColorPrimary,
		"wet":     ColorTeal,
		"dry":     ColorWarning,
		"starter": ColorPurple,
		"fat":     ColorMuted,
		"":        ColorMuted,
		"bogus":   ColorMuted,
	}
	for typ, want := range cases {
		if got := ingredientTypeColor(typ); got != want {
			t.Errorf("ingredientTypeColor(%q): want %v, got %v", typ, want, got)
		}
	}
}
