package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/gorecipes/internal/version"
)

// Palette — warm, food-inspired earth tones.
var (
	ColorPrimary   = lipgloss.Color("#C96442") // terracotta
	ColorSecondary = lipgloss.Color("#7C9E6E") // sage green
	ColorMuted     = lipgloss.Color("#8E8178") // warm gray
	ColorFaint     = lipgloss.Color("#B8B0A8") // very light warm gray — for version tags
	ColorBorder    = lipgloss.Color("#DDD5CC") // light warm gray
	ColorBg        = lipgloss.Color("#FDF8F3") // off-white cream
	ColorSuccess   = lipgloss.Color("#5A8A5A") // muted green
	ColorWarning   = lipgloss.Color("#B8832A") // amber
	ColorError     = lipgloss.Color("#B84040") // muted red
	ColorHighlight = lipgloss.Color("#E8D5C4") // peach

	// Status badge colors.
	StatusColors = map[string]lipgloss.Color{
		"published":         ColorSuccess,
		"review":            ColorWarning,
		"processing":        ColorPrimary,
		"processing_failed": ColorError,
		"draft":             ColorMuted,
		"rejected":          ColorError,
	}
)

// Text styles.
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)

	BoldStyle = lipgloss.NewStyle().Bold(true)

	MutedStyle = lipgloss.NewStyle().Foreground(ColorMuted)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	HighlightStyle = lipgloss.NewStyle().
			Background(ColorHighlight).
			Foreground(lipgloss.Color("#2D1810"))
)

// Layout styles.
var (
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(ColorBorder).
			MarginBottom(1).
			PaddingBottom(0)

	SectionLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary).
				MarginTop(1)
)

// Tag pill style.
func TagStyle(context string) lipgloss.Style {
	color := ColorMuted
	switch context {
	case "courses":
		color = ColorPrimary
	case "cooking_methods":
		color = ColorSecondary
	case "cultural_influences":
		color = lipgloss.Color("#7A6E9E") // muted purple
	case "dietary_restrictions":
		color = lipgloss.Color("#4A8A8A") // teal
	}
	return lipgloss.NewStyle().
		Background(color).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1).
		Margin(0, 1, 0, 0)
}

// StatusBadge renders a colored status label.
func StatusBadge(status string) string {
	color, ok := StatusColors[status]
	if !ok {
		color = ColorMuted
	}
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true).
		Render(statusLabel(status))
}

func statusLabel(status string) string {
	switch status {
	case "published":
		return "✓"
	case "review":
		return "⌛ review"
	case "processing":
		return "⠋ processing"
	case "processing_failed":
		return "✗ failed"
	case "draft":
		return "· draft"
	case "rejected":
		return "✗ rejected"
	default:
		return status
	}
}

// footerLine builds a footer content string with keybinding hints on the left
// and the application version tag right-aligned within the given inner width.
// innerWidth is the content width of the rendered footer block (Width() value).
func footerLine(keys []string, innerWidth int) string {
	left := "  " + strings.Join(keys, "   ")
	right := lipgloss.NewStyle().Foreground(ColorFaint).Render("gorecipes " + version.Version)
	gap := innerWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// renderManageBanner renders the breadcrumb banner shared by all manage sub-screens.
// pageName is the current section, e.g. "tags", "ingredients", "serving units".
func renderManageBanner(pageName string, width int) string {
	breadcrumb := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Render(
			"🍳  gorecipes  " +
				MutedStyle.Render("/") +
				"  manage  " +
				MutedStyle.Render("/") +
				"  " +
				lipgloss.NewStyle().
					Bold(false).
					Foreground(lipgloss.Color("#5C4A3C")).
					Render(pageName),
		)
	title := lipgloss.NewStyle().Padding(1, 2).Render(breadcrumb)
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(title)
}

// renderManageFooter renders the standard manage-screen key-hint footer.
func renderManageFooter(keys []string, width int) string {
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(footerLine(keys, width-2))
}

// renderManageConfirmFooter renders a yes/no footer: a bold coloured "y <action>"
// key and a muted "n / esc cancel" hint. accent sets both the key and border colour.
func renderManageConfirmFooter(yLabel string, accent lipgloss.Color, width int) string {
	yKey := lipgloss.NewStyle().Bold(true).Foreground(accent).Render(yLabel)
	line := "  " + yKey + "   " + MutedStyle.Render("n / esc cancel")
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(accent).
		Width(width - 2).
		Render(line)
}

// viewManageResult renders the result phase shared by all manage sub-screens:
// a centred success or error box with vertical fill and the given footer.
func viewManageResult(msg string, isErr bool, width, height int, footerStr string) string {
	var sb strings.Builder
	sb.WriteString("\n\n")
	style := SuccessStyle
	if isErr {
		style = ErrorStyle
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 3).
		Render(style.Render(msg))
	sb.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, box))
	sb.WriteString("\n")
	used := strings.Count(sb.String(), "\n")
	if fill := height - used - 3; fill > 0 {
		sb.WriteString(strings.Repeat("\n", fill))
	}
	sb.WriteString("\n")
	sb.WriteString(footerStr)
	return sb.String()
}

// buildCenteredBox renders a centred rounded-border dialog with a bold title,
// body lines, vertical fill, and footerStr below. titleColor tints the title;
// borderColor sets the box border.
func buildCenteredBox(title string, titleColor, borderColor lipgloss.Color, bodyLines []string, width, height int, footerStr string) string {
	var sb strings.Builder
	sb.WriteString("\n\n")
	parts := make([]string, 0, len(bodyLines)+2)
	parts = append(parts, lipgloss.NewStyle().Bold(true).Foreground(titleColor).Render(title), "")
	parts = append(parts, bodyLines...)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 3).
		Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
	sb.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, box))
	sb.WriteString("\n")
	used := strings.Count(sb.String(), "\n")
	if fill := height - used - 3; fill > 0 {
		sb.WriteString(strings.Repeat("\n", fill))
	}
	sb.WriteString("\n")
	sb.WriteString(footerStr)
	return sb.String()
}
