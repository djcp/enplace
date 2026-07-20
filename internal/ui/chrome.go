package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// chrome.go — btop-inspired chrome primitives: titled panel frames, flat
// titled rules, key hints, gradient gauges, and bar charts. All rendering
// keeps the warm earth palette defined in styles.go.

// isDarkBG reports whether the terminal has a dark background, reusing the
// cached glamour style detection so the OSC round-trip happens only once.
func isDarkBG() bool { return detectedGlamourStyle() == "dark" }

// bracketTitleStyled renders "┫ title ┣" around an already-styled title
// segment — heavy-joint brackets in the border colour.
func bracketTitleStyled(styled string, borderColor lipgloss.TerminalColor) string {
	bs := lipgloss.NewStyle().Foreground(borderColor)
	return bs.Render("┫ ") + styled + bs.Render(" ┣")
}

// bracketTitle renders "┫ title ┣" — heavy-joint brackets in the border
// colour with the title text in bold accent.
func bracketTitle(title string, borderColor, titleColor lipgloss.TerminalColor) string {
	ts := lipgloss.NewStyle().Bold(true).Foreground(titleColor)
	return bracketTitleStyled(ts.Render(title), borderColor)
}

// hRule returns n light horizontal border dashes in the border colour.
func hRule(n int, borderColor lipgloss.TerminalColor) string {
	if n < 0 {
		n = 0
	}
	return lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat("─", n))
}

// panelTop builds the top border of a titled panel:
//
//	╭─┫ left ┣──────────────┫ right ┣─╮
//
// left/right may be "" to omit. The line spans exactly width display columns.
func panelTop(width int, left, right string, borderColor, titleColor lipgloss.TerminalColor) string {
	bs := lipgloss.NewStyle().Foreground(borderColor)

	var leftSeg, rightSeg string
	if left != "" {
		leftSeg = bracketTitle(left, borderColor, titleColor)
	}
	if right != "" {
		rightSeg = bracketTitle(right, borderColor, titleColor)
	}

	// corners (2) + one dash after ╭ and one before ╮ when segments exist.
	fill := width - 2 - lipgloss.Width(leftSeg) - lipgloss.Width(rightSeg)
	if leftSeg != "" {
		fill -= 1
	}
	if rightSeg != "" {
		fill -= 1
	}
	if fill < 0 {
		// Too narrow for titles — degrade to a plain border line.
		return bs.Render("╭" + strings.Repeat("─", max(width-2, 0)) + "╮")
	}

	var sb strings.Builder
	sb.WriteString(bs.Render("╭"))
	if leftSeg != "" {
		sb.WriteString(bs.Render("─"))
		sb.WriteString(leftSeg)
	}
	sb.WriteString(hRule(fill, borderColor))
	if rightSeg != "" {
		sb.WriteString(rightSeg)
		sb.WriteString(bs.Render("─"))
	}
	sb.WriteString(bs.Render("╮"))
	return sb.String()
}

// panelBottom builds the bottom border of a titled panel:
//
//	╰──────────────────────┫ title ┣─╯
//
// title may be "" for a plain bottom border.
func panelBottom(width int, title string, borderColor, titleColor lipgloss.TerminalColor) string {
	bs := lipgloss.NewStyle().Foreground(borderColor)
	if title == "" {
		return bs.Render("╰" + strings.Repeat("─", max(width-2, 0)) + "╯")
	}
	seg := bracketTitle(title, borderColor, titleColor)
	fill := width - 2 - lipgloss.Width(seg) - 1
	if fill < 0 {
		return bs.Render("╰" + strings.Repeat("─", max(width-2, 0)) + "╯")
	}
	return bs.Render("╰") + hRule(fill, borderColor) + seg + bs.Render("─") + bs.Render("╯")
}

// framePanel wraps content in a full titled panel frame. width is the total
// panel width including borders; innerHeight is the number of content lines
// (content is padded or truncated to exactly that many lines). topLeft,
// topRight, and bottom are optional border titles.
func framePanel(content string, width, innerHeight int, topLeft, topRight, bottom string, borderColor, titleColor lipgloss.TerminalColor) string {
	bs := lipgloss.NewStyle().Foreground(borderColor)
	innerW := width - 2
	if innerW < 1 {
		innerW = 1
	}

	lines := strings.Split(content, "\n")
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}

	clip := lipgloss.NewStyle().MaxWidth(innerW)
	var sb strings.Builder
	sb.WriteString(panelTop(width, topLeft, topRight, borderColor, titleColor))
	sb.WriteString("\n")
	for i := 0; i < innerHeight; i++ {
		line := ""
		if i < len(lines) {
			line = clip.Render(lines[i])
		}
		pad := innerW - lipgloss.Width(line)
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(bs.Render("│"))
		sb.WriteString(line)
		sb.WriteString(strings.Repeat(" ", pad))
		sb.WriteString(bs.Render("│"))
		sb.WriteString("\n")
	}
	sb.WriteString(panelBottom(width, bottom, borderColor, titleColor))
	return sb.String()
}

// flatRule builds a single-line titled rule (used as a screen banner):
//
//	─┫ title ┣──────────────────┫ right ┣──
//
// It spans exactly width display columns and has no corners, so it reads as
// a header bar rather than an open box.
func flatRule(width int, title, right string, borderColor, titleColor lipgloss.TerminalColor) string {
	var styledTitle string
	if title != "" {
		styledTitle = lipgloss.NewStyle().Bold(true).Foreground(titleColor).Render(title)
	}
	return flatRuleStyled(width, styledTitle, right, borderColor, titleColor)
}

// flatRuleStyled is flatRule with a pre-styled left title segment, allowing
// mixed-colour breadcrumbs ("🍳 enplace / Recipe Name") inside the brackets.
func flatRuleStyled(width int, styledTitle, right string, borderColor, titleColor lipgloss.TerminalColor) string {
	var leftSeg, rightSeg string
	if styledTitle != "" {
		leftSeg = bracketTitleStyled(styledTitle, borderColor)
	}
	if right != "" {
		rightSeg = bracketTitle(right, borderColor, titleColor)
	}
	fill := width - 1 - lipgloss.Width(leftSeg) - lipgloss.Width(rightSeg)
	if rightSeg != "" {
		fill -= 2
	}
	if fill < 0 {
		return hRule(max(width, 0), borderColor)
	}
	var sb strings.Builder
	sb.WriteString(hRule(1, borderColor))
	sb.WriteString(leftSeg)
	sb.WriteString(hRule(fill, borderColor))
	if rightSeg != "" {
		sb.WriteString(rightSeg)
		sb.WriteString(hRule(2, borderColor))
	}
	return sb.String()
}

// sectionRule renders a light titled rule for reading-view section headers:
//
//	─┤ Ingredients ├──────────────
//
// Deliberately quieter than panel borders — light joints, sage label — so
// prose-heavy views keep their calm.
func sectionRule(width int, label string) string {
	bs := lipgloss.NewStyle().Foreground(ColorBorder)
	ts := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary)
	seg := bs.Render("─┤ ") + ts.Render(label) + bs.Render(" ├")
	fill := width - lipgloss.Width(seg)
	if fill < 0 {
		fill = 0
	}
	return seg + bs.Render(strings.Repeat("─", fill))
}

// keyHint renders a footer key hint: the key in bold accent, the label muted.
func keyHint(key, label string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(key) +
		" " + MutedStyle.Render(label)
}

// ── gauges and bars ──────────────────────────────────────────────────────

// gauge partial-fill glyphs, 1/8 through 7/8.
var eighthBlocks = []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉"}

// gaugeRamp colour stops, dry → wet, chosen per background. The ramp stays
// in the warm family: sage → gold → terracotta → red.
var (
	gaugeRampDark  = [4][3]int{{0x90, 0xB8, 0x82}, {0xD4, 0x98, 0x3A}, {0xE0, 0x78, 0x56}, {0xD0, 0x50, 0x50}}
	gaugeRampLight = [4][3]int{{0x7C, 0x9E, 0x6E}, {0xB8, 0x83, 0x2A}, {0xC9, 0x64, 0x42}, {0xB8, 0x40, 0x40}}
)

// gaugeColor returns the ramp colour at position t ∈ [0,1], linearly
// interpolated between the ramp stops.
func gaugeColor(t float64) lipgloss.Color {
	ramp := gaugeRampDark
	if !isDarkBG() {
		ramp = gaugeRampLight
	}
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	seg := t * float64(len(ramp)-1)
	i := int(seg)
	if i >= len(ramp)-1 {
		i = len(ramp) - 2
	}
	f := seg - float64(i)
	a, b := ramp[i], ramp[i+1]
	r := int(float64(a[0]) + f*float64(b[0]-a[0]))
	g := int(float64(a[1]) + f*float64(b[1]-a[1]))
	bl := int(float64(a[2]) + f*float64(b[2]-a[2]))
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", r, g, bl))
}

// renderGauge renders a horizontal gradient gauge filled to frac ∈ [0,1] of
// width cells. Filled cells are coloured by their absolute position along the
// gauge (btop-style), the unfilled remainder is faint shade blocks.
func renderGauge(frac float64, width int) string {
	if width < 4 {
		width = 4
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	cells := frac * float64(width)
	full := int(cells)
	rem := cells - float64(full)

	var sb strings.Builder
	for i := 0; i < full; i++ {
		t := float64(i) / float64(width-1)
		sb.WriteString(lipgloss.NewStyle().Foreground(gaugeColor(t)).Render("█"))
	}
	used := full
	if full < width {
		if idx := int(rem * 8); idx > 0 {
			t := float64(full) / float64(width-1)
			sb.WriteString(lipgloss.NewStyle().Foreground(gaugeColor(t)).Render(eighthBlocks[idx-1]))
			used++
		}
	}
	if used < width {
		sb.WriteString(lipgloss.NewStyle().Foreground(ColorFaint).Render(strings.Repeat("░", width-used)))
	}
	return sb.String()
}

// renderBar renders a single solid-colour horizontal bar filled to
// frac ∈ [0,1] of width cells, with an eighth-block partial end. The
// unfilled remainder is left as spaces so bars align in a chart.
func renderBar(frac float64, width int, color lipgloss.TerminalColor) string {
	if width < 1 {
		width = 1
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	cells := frac * float64(width)
	full := int(cells)
	rem := cells - float64(full)

	st := lipgloss.NewStyle().Foreground(color)
	var sb strings.Builder
	if full > 0 {
		sb.WriteString(st.Render(strings.Repeat("█", full)))
	}
	used := full
	if full < width {
		if idx := int(rem * 8); idx > 0 {
			sb.WriteString(st.Render(eighthBlocks[idx-1]))
			used++
		} else if full == 0 {
			// Never render a fully empty bar for a non-zero value.
			if frac > 0 {
				sb.WriteString(st.Render("▏"))
				used++
			}
		}
	}
	if used < width {
		sb.WriteString(strings.Repeat(" ", width-used))
	}
	return sb.String()
}

// ingredientTypeColor maps an ingredient_type to its chart colour.
func ingredientTypeColor(t string) lipgloss.TerminalColor {
	switch t {
	case "flour":
		return ColorPrimary
	case "wet":
		return ColorTeal
	case "dry":
		return ColorWarning
	case "starter":
		return ColorPurple
	case "fat":
		return ColorMuted
	}
	return ColorMuted
}

// hydrationGaugeFrac maps a hydration percentage onto gauge fill. The gauge
// spans 50%–105% hydration — stiff bagel doughs sit low, slack ciabattas
// near the top — so mid-range doughs read visually mid-gauge.
func hydrationGaugeFrac(pct float64) float64 {
	return (pct - 50) / (105 - 50)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
