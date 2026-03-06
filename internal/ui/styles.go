package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/gorecipes/internal/version"
)

// Palette — warm, food-inspired earth tones. Each color uses AdaptiveColor so
// values are chosen for both light and dark terminal backgrounds.
var (
	ColorPrimary   = lipgloss.AdaptiveColor{Light: "#C96442", Dark: "#E07856"} // terracotta
	ColorSecondary = lipgloss.AdaptiveColor{Light: "#7C9E6E", Dark: "#90B882"} // sage green
	ColorMuted     = lipgloss.AdaptiveColor{Light: "#8E8178", Dark: "#A89888"} // warm gray
	ColorFaint     = lipgloss.AdaptiveColor{Light: "#B8B0A8", Dark: "#685E56"} // subtle — version tags
	ColorBorder    = lipgloss.AdaptiveColor{Light: "#DDD5CC", Dark: "#3C3028"} // border
	ColorBg        = lipgloss.AdaptiveColor{Light: "#FDF8F3", Dark: "#1A1614"} // background
	ColorSuccess   = lipgloss.AdaptiveColor{Light: "#5A8A5A", Dark: "#70A870"} // muted green
	ColorWarning   = lipgloss.AdaptiveColor{Light: "#B8832A", Dark: "#D4983A"} // amber
	ColorError     = lipgloss.AdaptiveColor{Light: "#B84040", Dark: "#D05050"} // muted red
	ColorHighlight = lipgloss.AdaptiveColor{Light: "#E8D5C4", Dark: "#3D2A1E"} // selected row bg

	// ColorSubtle is a warm brown used for secondary/breadcrumb text.
	ColorSubtle = lipgloss.AdaptiveColor{Light: "#5C4A3C", Dark: "#BCA898"}
	// ColorHighlightFg is the foreground text colour used on highlighted (selected) rows.
	ColorHighlightFg = lipgloss.AdaptiveColor{Light: "#2D1810", Dark: "#F5EAE0"}

	// Status badge colors.
	StatusColors = map[string]lipgloss.TerminalColor{
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
			Foreground(ColorHighlightFg)
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
	var color lipgloss.TerminalColor = ColorMuted
	switch context {
	case "courses":
		color = ColorPrimary
	case "cooking_methods":
		color = ColorSecondary
	case "cultural_influences":
		color = lipgloss.AdaptiveColor{Light: "#7A6E9E", Dark: "#9A8EC0"} // muted purple
	case "dietary_restrictions":
		color = lipgloss.AdaptiveColor{Light: "#4A8A8A", Dark: "#5AACAC"} // teal
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

// restoredCursorByID returns (cursor, offset) after reloading a list. If seekID is
// non-zero it scans idAt(i) for a match; otherwise it clamps prevCursor to the new
// list length. visible is the number of rows that fit on screen.
func restoredCursorByID(seekID int64, prevCursor, listLen int, idAt func(int) int64, visible int) (cursor, offset int) {
	if seekID != 0 {
		for i := 0; i < listLen; i++ {
			if idAt(i) == seekID {
				cursor = i
				break
			}
		}
	} else {
		cursor = prevCursor
		if cursor >= listLen && listLen > 0 {
			cursor = listLen - 1
		}
	}
	if cursor >= offset+visible {
		offset = cursor - visible + 1
	}
	return cursor, offset
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
					Foreground(ColorSubtle).
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
func renderManageConfirmFooter(yLabel string, accent lipgloss.TerminalColor, width int) string {
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
func buildCenteredBox(title string, titleColor, borderColor lipgloss.TerminalColor, bodyLines []string, width, height int, footerStr string) string {
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
