package ui

import "github.com/charmbracelet/lipgloss"

// Palette — warm, food-inspired earth tones.
var (
	ColorPrimary   = lipgloss.Color("#C96442") // terracotta
	ColorSecondary = lipgloss.Color("#7C9E6E") // sage green
	ColorMuted     = lipgloss.Color("#8E8178") // warm gray
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
