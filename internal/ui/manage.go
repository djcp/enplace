package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ManageSection identifies which section the user selected on the manage landing page.
type ManageSection int

const (
	ManageSectionBack ManageSection = iota
	ManageSectionConfig
	ManageSectionTags
	ManageSectionIngredients
	ManageSectionUnits
	ManageSectionAIRuns
)

var manageOptions = []struct{ label, desc string }{
	{"Configure", "API key, AI model, and author credits on exported recipes"},
	{"Tags", "Edit, merge, and delete tags by context"},
	{"Ingredients", "Edit and merge ingredient names"},
	{"Serving Units", "Edit and merge serving units"},
	{"AI Classifier Runs", "Browse AI extraction history"},
}

// ManageModel is the landing page for the manage section.
type ManageModel struct {
	cursor  int
	width   int
	height  int
	section ManageSection
	done    bool
}

func newManageModel() ManageModel {
	return ManageModel{width: 80, height: 24}
}

func (m ManageModel) Init() tea.Cmd { return nil }

func (m ManageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m ManageModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.done = true
		m.section = ManageSectionBack
		return m, tea.Quit
	case "esc":
		m.section = ManageSectionBack
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(manageOptions)-1 {
			m.cursor++
		}
	case "enter", " ":
		m.section = ManageSection(m.cursor + 1) // +1 because 0 = Back
		return m, tea.Quit
	}
	return m, nil
}

func (m ManageModel) View() string {
	if m.width == 0 {
		return ""
	}
	var sb strings.Builder

	sb.WriteString(renderManageLandingBanner(m.width))
	sb.WriteString("\n\n")

	// Label column width.
	labelW := 20
	for _, opt := range manageOptions {
		if len([]rune(opt.label)) > labelW {
			labelW = len([]rune(opt.label))
		}
	}

	for i, opt := range manageOptions {
		selected := i == m.cursor
		var line string
		if selected {
			label := lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				Render("  ▶ " + padRight(opt.label, labelW) + "  ")
			desc := lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Render(opt.desc)
			line = label + desc
		} else {
			label := MutedStyle.Render("    " + padRight(opt.label, labelW) + "  ")
			desc := lipgloss.NewStyle().
				Foreground(ColorFaint).
				Render(opt.desc)
			line = label + desc
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Fill remaining space so footer is pinned.
	used := strings.Count(sb.String(), "\n")
	if fill := m.height - used - 3; fill > 0 {
		sb.WriteString(strings.Repeat("\n", fill))
	}
	sb.WriteString("\n")
	sb.WriteString(renderManageLandingFooter(m.width))

	return sb.String()
}

func padRight(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(runes))
}

func renderManageLandingBanner(width int) string {
	breadcrumb := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Render(
			"🍳  gorecipes  " +
				MutedStyle.Render("/") +
				"  " +
				lipgloss.NewStyle().
					Bold(false).
					Foreground(lipgloss.Color("#5C4A3C")).
					Render("manage"),
		)

	title := lipgloss.NewStyle().
		Padding(1, 2).
		Render(breadcrumb)

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(title)
}

func renderManageLandingFooter(width int) string {
	keys := []string{
		MutedStyle.Render("↑/↓ navigate"),
		MutedStyle.Render("enter open"),
		MutedStyle.Render("esc back"),
	}
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(footerLine(keys, width-2))
}

// RunManageUI runs the manage landing page TUI and returns which section was selected.
func RunManageUI() (ManageSection, error) {
	m := newManageModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return ManageSectionBack, err
	}
	fm := final.(ManageModel)
	return fm.section, nil
}
