package ui

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/gorecipes/internal/export"
	"github.com/djcp/gorecipes/internal/models"
)

type printPhase int

const (
	printPhasePreview printPhase = iota
	printPhaseFormatSelect
	printPhaseResult
)

var exportFormats = []struct {
	label string
	ext   string
}{
	{"PDF (.pdf)", "pdf"},
	{"Rich Text (.rtf)", "rtf"},
	{"Markdown (.md)", "md"},
	{"Plain Text (.txt)", "txt"},
	{"Print to printer", ""},
}

// PrintModel is a full-screen print preview with integrated export/print dialog.
type PrintModel struct {
	recipe    *models.Recipe
	opts      export.Options
	phase     printPhase
	lines     []string // pre-rendered text preview lines
	scroll    int
	width     int
	height    int
	cursor    int // format selection cursor
	resultMsg string
	isError   bool
	goBack    bool
	quitting  bool // true when user pressed q/ctrl+c (vs esc to go back)
}

// Quitting returns true when the user explicitly pressed q or ctrl+c to exit.
func (m PrintModel) Quitting() bool { return m.quitting }

func newPrintModel(recipe *models.Recipe, opts export.Options) PrintModel {
	m := PrintModel{recipe: recipe, opts: opts, width: 80, height: 24}
	m.lines = m.buildPreviewLines()
	return m
}

func (m PrintModel) Init() tea.Cmd { return nil }

func (m PrintModel) viewportHeight() int {
	v := m.height - 7
	if v < 1 {
		v = 1
	}
	return v
}

func (m PrintModel) maxScroll() int {
	ms := len(m.lines) - m.viewportHeight()
	if ms < 0 {
		return 0
	}
	return ms
}

func (m PrintModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.lines = m.buildPreviewLines()
		if m.scroll > m.maxScroll() {
			m.scroll = m.maxScroll()
		}
	case tea.KeyMsg:
		switch m.phase {
		case printPhasePreview:
			return m.handlePreviewKey(msg)
		case printPhaseFormatSelect:
			return m.handleFormatKey(msg)
		case printPhaseResult:
			m.goBack = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m PrintModel) handlePreviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.goBack = true
		return m, tea.Quit
	case "up", "k":
		if m.scroll > 0 {
			m.scroll--
		}
	case "down", "j":
		if m.scroll < m.maxScroll() {
			m.scroll++
		}
	case "pgup":
		m.scroll -= m.viewportHeight()
		if m.scroll < 0 {
			m.scroll = 0
		}
	case "pgdown":
		m.scroll += m.viewportHeight()
		if m.scroll > m.maxScroll() {
			m.scroll = m.maxScroll()
		}
	case "p":
		m.phase = printPhaseFormatSelect
	}
	return m, nil
}

func (m PrintModel) handleFormatKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.phase = printPhasePreview
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(exportFormats)-1 {
			m.cursor++
		}
	case "enter":
		m = m.execute()
	}
	return m, nil
}

func (m PrintModel) execute() PrintModel {
	f := exportFormats[m.cursor]
	if f.ext == "" {
		// Print to system printer
		err := printToPrinter(m.recipe.Name, export.ToText(m.recipe, m.opts))
		if err != nil {
			m.isError = true
			m.resultMsg = "Error: " + err.Error()
		} else {
			m.resultMsg = "Sent to printer"
		}
	} else {
		dir, err := export.DownloadsDir()
		if err != nil {
			m.isError = true
			m.resultMsg = "Error: " + err.Error()
			m.phase = printPhaseResult
			return m
		}
		path := export.UniqueFilePath(dir, export.SafeFilename(m.recipe.Name), f.ext)

		var data []byte
		switch f.ext {
		case "txt":
			data = []byte(export.ToText(m.recipe, m.opts))
		case "md":
			data = []byte(export.ToMarkdown(m.recipe, m.opts))
		case "rtf":
			data = []byte(export.ToRTF(m.recipe, m.opts))
		case "pdf":
			data, err = export.ToPDF(m.recipe, m.opts)
			if err != nil {
				m.isError = true
				m.resultMsg = "Error: " + err.Error()
				m.phase = printPhaseResult
				return m
			}
		}

		if err := os.WriteFile(path, data, 0o644); err != nil {
			m.isError = true
			m.resultMsg = "Error: " + err.Error()
		} else {
			m.resultMsg = "Saved to " + path
		}
	}
	m.phase = printPhaseResult
	return m
}

func printToPrinter(name, text string) error {
	lp, err := exec.LookPath("lp")
	if err != nil {
		lp, err = exec.LookPath("lpr")
		if err != nil {
			return errors.New("no printer found (lp/lpr not available)")
		}
	}
	cmd := exec.Command(lp, "-t", name)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (m PrintModel) View() string {
	if m.width == 0 {
		return ""
	}

	var sb strings.Builder

	// Banner
	sb.WriteString(renderPrintBanner(m.recipe.Name, m.width))
	sb.WriteString("\n")

	switch m.phase {
	case printPhasePreview:
		m.renderPreview(&sb)

	case printPhaseFormatSelect:
		m.renderFormatSelect(&sb)

	case printPhaseResult:
		m.renderResult(&sb)
	}

	return sb.String()
}

func (m PrintModel) renderPreview(sb *strings.Builder) {
	vh := m.viewportHeight()
	start := m.scroll
	end := start + vh
	if end > len(m.lines) {
		end = len(m.lines)
	}
	for i := start; i < end; i++ {
		sb.WriteString(m.lines[i])
		sb.WriteString("\n")
	}
	// Pad remaining viewport rows so footer stays pinned.
	for i := end - start; i < vh; i++ {
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(renderPrintPreviewFooter(m.width))
}

func (m PrintModel) renderFormatSelect(sb *strings.Builder) {
	// Build format option list
	var optLines []string
	for i, f := range exportFormats {
		label := f.label
		if i == m.cursor {
			optLines = append(optLines, lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("▶ "+label))
		} else {
			optLines = append(optLines, MutedStyle.Render("  "+label))
		}
	}

	inner := lipgloss.JoinVertical(lipgloss.Left,
		append([]string{
			lipgloss.NewStyle().Bold(true).Render("Export / Print"),
			"",
		}, optLines...)...,
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 3).
		Render(inner)

	sb.WriteString("\n\n")
	sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, box))
	sb.WriteString("\n")

	// Fill remaining vertical space
	used := strings.Count(sb.String(), "\n")
	if fill := m.height - used - 3; fill > 0 {
		sb.WriteString(strings.Repeat("\n", fill))
	}
	sb.WriteString("\n")
	sb.WriteString(renderFormatSelectFooter(m.width))
}

func (m PrintModel) renderResult(sb *strings.Builder) {
	var msgStyle lipgloss.Style
	if m.isError {
		msgStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorError)
	} else {
		msgStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess)
	}

	inner := lipgloss.JoinVertical(lipgloss.Left,
		msgStyle.Render(m.resultMsg),
		"",
		MutedStyle.Render("Press any key to return"),
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 3).
		Render(inner)

	sb.WriteString("\n\n")
	sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, box))
}

// buildPreviewLines produces a []string of terminal lines for the preview
// viewport, with recipe name and section headers highlighted.
func (m PrintModel) buildPreviewLines() []string {
	raw := export.ToText(m.recipe, m.opts)
	rawLines := strings.Split(raw, "\n")
	lines := make([]string, 0, len(rawLines))
	for i, line := range rawLines {
		var rendered string
		switch {
		case i == 0:
			rendered = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(line)
		case line == "INGREDIENTS" || line == "DIRECTIONS":
			rendered = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Render(line)
		default:
			rendered = line
		}
		lines = append(lines, rendered)
	}
	return lines
}

func renderPrintBanner(name string, width int) string {
	hint := MutedStyle.Render("💾 export preview")
	hintWidth := lipgloss.Width(hint)

	maxNameLen := width - 26 - hintWidth - 2
	if maxNameLen < 8 {
		maxNameLen = 8
	}

	breadcrumb := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Render(
			"🍳  gorecipes  " +
				MutedStyle.Render("/") +
				"  " +
				lipgloss.NewStyle().
					Bold(false).
					Foreground(ColorSubtle).
					Render(truncate(name, maxNameLen)),
		)

	contentWidth := width - 6
	gap := contentWidth - lipgloss.Width(breadcrumb) - hintWidth
	if gap < 1 {
		gap = 1
	}

	title := lipgloss.NewStyle().
		Padding(1, 2).
		Render(breadcrumb + strings.Repeat(" ", gap) + hint)

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(title)
}

func renderPrintPreviewFooter(width int) string {
	keys := []string{
		MutedStyle.Render("📜 ↑/↓ scroll"),
		MutedStyle.Render("💾 p export"),
		MutedStyle.Render("✖ esc back"),
	}
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(footerLine(keys, width-2))
}

func renderFormatSelectFooter(width int) string {
	keys := []string{
		MutedStyle.Render("📜 ↑/↓ navigate"),
		MutedStyle.Render("👁 enter select"),
		MutedStyle.Render("✖ esc back"),
	}
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(footerLine(keys, width-2))
}

// RunPrintUI runs the interactive print preview TUI for the given recipe.
// opts controls export behaviour, notably Credits for file footer attribution.
func RunPrintUI(recipe *models.Recipe, opts export.Options) (bool, error) {
	m := newPrintModel(recipe, opts)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return false, err
	}
	pm, _ := final.(PrintModel)
	return pm.Quitting(), nil
}
