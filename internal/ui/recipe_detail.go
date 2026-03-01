package ui

import (
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/gorecipes/internal/models"
	"github.com/muesli/termenv"
)

var (
	glamourStyleOnce sync.Once
	glamourStyleName string
)

// detectedGlamourStyle queries the terminal background colour exactly once and
// returns the matching glamour style name ("dark" or "light"). Subsequent calls
// return the cached result instantly, avoiding the OSC round-trip on every render.
func detectedGlamourStyle() string {
	glamourStyleOnce.Do(func() {
		if termenv.HasDarkBackground() {
			glamourStyleName = "dark"
		} else {
			glamourStyleName = "light"
		}
	})
	return glamourStyleName
}

type detailFocus int

const (
	detailFocusContent detailFocus = iota
	detailFocusHeader              // search bar active
	detailFocusFooter              // home / navigation active
)

// DetailModel is a full-screen interactive recipe detail viewer that mirrors
// the visual structure of ListModel: banner, search bar, scrollable content, footer.
type DetailModel struct {
	recipe *models.Recipe
	lines  []string // pre-rendered content split into terminal lines
	scroll int
	width  int
	height int

	focus detailFocus
	query string // search bar text (carried back to the list on "home")

	goHome          bool
	goAdd           bool
	goEdit          bool
	returnQuery     string
	confirmingDelete bool
	deleteConfirmed  bool
}

// NewDetailModel creates a DetailModel for the given recipe.
// It detects the terminal background colour and pre-renders content before
// the TUI starts so the first frame and any resize redraws are instant.
func NewDetailModel(recipe *models.Recipe) DetailModel {
	detectedGlamourStyle() // warm up the cache before entering the event loop
	m := DetailModel{recipe: recipe, width: 80, height: 24}
	m.lines = m.buildLines()
	return m
}

// GoHome returns true when the user selected "home".
func (m DetailModel) GoHome() bool { return m.goHome }

// GoAdd returns true when the user pressed "a" to add a new recipe.
func (m DetailModel) GoAdd() bool { return m.goAdd }

// GoEdit returns true when the user pressed "e" to edit the recipe.
func (m DetailModel) GoEdit() bool { return m.goEdit }

// DeleteConfirmed returns true when the user confirmed deletion of the recipe.
func (m DetailModel) DeleteConfirmed() bool { return m.deleteConfirmed }

// ReturnQuery returns any search text typed before leaving.
func (m DetailModel) ReturnQuery() string { return m.returnQuery }

func (m DetailModel) Init() tea.Cmd { return nil }

// viewportHeight mirrors the list model's formula so the two views feel identical.
// When the search bar is visible (header focus) it adds 4 lines of overhead.
func (m DetailModel) viewportHeight() int {
	v := m.height - 7
	if m.focus == detailFocusHeader {
		v -= 4
	}
	if v < 1 {
		v = 1
	}
	return v
}

func (m DetailModel) maxScroll() int {
	ms := len(m.lines) - m.viewportHeight()
	if ms < 0 {
		return 0
	}
	return ms
}

func (m DetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.lines = m.buildLines()
		if m.scroll > m.maxScroll() {
			m.scroll = m.maxScroll()
		}

	case tea.KeyMsg:
		if m.confirmingDelete {
			return m.handleConfirmKey(msg)
		}
		// All keypresses when header has focus are routed to the search handler.
		if m.focus == detailFocusHeader {
			return m.handleHeaderKey(msg)
		}
		return m.handleNavKey(msg)
	}

	return m, nil
}

// handleConfirmKey processes keys while the delete confirmation overlay is shown.
func (m DetailModel) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		m.deleteConfirmed = true
		return m, tea.Quit
	case "n", "esc", "ctrl+c":
		m.confirmingDelete = false
	}
	return m, nil
}

// handleHeaderKey processes keys while the search bar has focus.
func (m DetailModel) handleHeaderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		// Cancel search: clear query and return focus to content.
		m.query = ""
		m.focus = detailFocusContent

	case tea.KeyEnter:
		// Confirm search: go home with the current query.
		m.goHome = true
		m.returnQuery = m.query
		return m, tea.Quit

	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.query) > 0 {
			runes := []rune(m.query)
			m.query = string(runes[:len(runes)-1])
		}

	case tea.KeyDown:
		// Dismiss search bar, return to content at top.
		m.focus = detailFocusContent
		m.scroll = 0

	default:
		if msg.Type == tea.KeyRunes {
			m.query += string(msg.Runes)
		}
	}
	return m, nil
}

// handleNavKey processes keys while content or footer has focus.
func (m DetailModel) handleNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "q", "esc":
		switch m.focus {
		case detailFocusFooter:
			m.focus = detailFocusContent
		default:
			return m, tea.Quit
		}

	case "h":
		m.goHome = true
		m.returnQuery = m.query
		return m, tea.Quit

	case "a":
		m.goAdd = true
		return m, tea.Quit

	case "e":
		m.goEdit = true
		return m, tea.Quit

	case "d":
		m.confirmingDelete = true

	case "/":
		m.focus = detailFocusHeader

	case "enter":
		switch m.focus {
		case detailFocusFooter:
			m.goHome = true
			m.returnQuery = m.query
			return m, tea.Quit
		}

	case "up", "k":
		switch m.focus {
		case detailFocusContent:
			if m.scroll > 0 {
				m.scroll--
			}
		case detailFocusFooter:
			// Return to content, positioned at the bottom.
			m.focus = detailFocusContent
		}

	case "down", "j":
		switch m.focus {
		case detailFocusContent:
			if m.scroll < m.maxScroll() {
				m.scroll++
			} else {
				// At bottom: shift focus to the footer.
				m.focus = detailFocusFooter
			}
		}

	case "pgup":
		if m.focus == detailFocusContent {
			m.scroll -= m.viewportHeight()
			if m.scroll < 0 {
				m.scroll = 0
			}
		}

	case "pgdown":
		if m.focus == detailFocusContent {
			m.scroll += m.viewportHeight()
			if m.scroll >= m.maxScroll() {
				m.scroll = m.maxScroll()
				m.focus = detailFocusFooter
			}
		}
	}

	return m, nil
}

func (m DetailModel) View() string {
	if m.width == 0 {
		return ""
	}
	lines := m.lines
	if len(lines) == 0 {
		lines = m.buildLines()
	}

	var sb strings.Builder

	// Banner — same structure as list view, with recipe name as breadcrumb.
	sb.WriteString(renderDetailBanner(m.recipe.Name, m.width))
	sb.WriteString("\n")

	// Delete confirmation overlay — replaces content and footer.
	if m.confirmingDelete {
		confirmContent := m.viewConfirm()
		sb.WriteString(confirmContent)
		used := strings.Count(sb.String(), "\n")
		if fill := m.height - used - 3; fill > 0 {
			sb.WriteString(strings.Repeat("\n", fill))
		}
		sb.WriteString("\n")
		sb.WriteString(renderConfirmFooter(m.width))
		return sb.String()
	}

	// Search bar — only visible when the user pressed "/" to initiate a search.
	if m.focus == detailFocusHeader {
		sb.WriteString(renderSearchBar(m.query, true, m.width))
		sb.WriteString("\n\n")
	}

	// Scrollable content viewport.
	vh := m.viewportHeight()
	start := m.scroll
	end := start + vh
	if end > len(lines) {
		end = len(lines)
	}
	for i := start; i < end; i++ {
		sb.WriteString(lines[i])
		sb.WriteString("\n")
	}
	// Pad any remaining viewport rows so the footer stays pinned to the bottom.
	for i := end - start; i < vh; i++ {
		sb.WriteString("\n")
	}

	// Footer.
	sb.WriteString("\n")
	sb.WriteString(renderDetailFooter(m.focus, m.width))

	return sb.String()
}

func (m DetailModel) viewConfirm() string {
	var sb strings.Builder
	sb.WriteString("\n\n")

	name := truncate(m.recipe.Name, m.width-20)
	inner := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Foreground(ColorError).Render("Delete recipe?"),
		"",
		lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(name),
		"",
		MutedStyle.Render("This cannot be undone."),
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorError).
		Padding(1, 3).
		Render(inner)

	sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, box))
	sb.WriteString("\n")
	return sb.String()
}

// buildLines renders the recipe body at the current terminal width and splits
// the result into individual terminal lines for viewport scrolling.
func (m DetailModel) buildLines() []string {
	contentWidth := m.width - 4
	if contentWidth > 100 {
		contentWidth = 100
	}
	if contentWidth < 20 {
		contentWidth = 20
	}
	raw := buildRecipeBlock(m.recipe, contentWidth)
	return strings.Split(raw, "\n")
}

// buildRecipeBlock assembles the full styled recipe body as a single string.
func buildRecipeBlock(r *models.Recipe, width int) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Width(width).
		Render(r.Name))
	sb.WriteString("\n")

	// Timing & servings.
	var meta []string
	if t := r.TimingSummary(); t != "" {
		meta = append(meta, MutedStyle.Render(t))
	}
	if r.Servings != nil && *r.Servings > 0 {
		units := r.ServingUnits
		if units == "" {
			units = "servings"
		}
		meta = append(meta, MutedStyle.Render(fmt.Sprintf("Serves %d %s", *r.Servings, units)))
	}
	if len(meta) > 0 {
		sb.WriteString(strings.Join(meta, MutedStyle.Render("  ·  ")))
		sb.WriteString("\n")
	}

	// Tag pills.
	if tags := buildTagPills(r); tags != "" {
		sb.WriteString(tags)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Description.
	if r.Description != "" {
		sb.WriteString(lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("#5C4A3C")).
			Width(width).
			Render(r.Description))
		sb.WriteString("\n\n")
	}

	// Ingredients.
	if len(r.Ingredients) > 0 {
		sb.WriteString(SectionLabelStyle.Render("Ingredients"))
		sb.WriteString("\n")
		sb.WriteString(buildIngredientLines(r.Ingredients))
		sb.WriteString("\n")
	}

	// Directions.
	if r.Directions != "" {
		sb.WriteString(SectionLabelStyle.Render("Directions"))
		sb.WriteString("\n")
		sb.WriteString(renderMarkdown(r.Directions, width))
	}

	// Source URL.
	if r.SourceURL != "" {
		sb.WriteString("\n")
		sb.WriteString(MutedStyle.Render("Source: " + r.SourceURL))
		sb.WriteString("\n")
	}

	return sb.String()
}

func buildTagPills(r *models.Recipe) string {
	var pills []string
	for _, ctx := range models.AllTagContexts {
		for _, name := range r.TagsByContext(ctx) {
			pills = append(pills, TagStyle(ctx).Render(name))
		}
	}
	return strings.Join(pills, "")
}

func buildIngredientLines(ings []models.RecipeIngredient) string {
	var sb strings.Builder
	currentSection := ""
	for _, ing := range ings {
		if ing.Section != currentSection && ing.Section != "" {
			if currentSection != "" {
				sb.WriteString("\n")
			}
			sb.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorMuted).
				Render("  " + ing.Section))
			sb.WriteString("\n")
			currentSection = ing.Section
		}
		sb.WriteString(MutedStyle.Render("  · ") + ing.DisplayString())
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderMarkdown(text string, width int) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(detectedGlamourStyle()),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return text
	}
	out, err := r.Render(text)
	if err != nil {
		return text
	}
	return out
}

// renderDetailBanner renders the banner with a "gorecipes / Recipe Name" breadcrumb
// and a right-aligned "a  add" hint.
func renderDetailBanner(name string, width int) string {
	addHint := MutedStyle.Render("a  add")
	addHintWidth := lipgloss.Width(addHint)

	// Reserve space for breadcrumb: "🍳  gorecipes  /  " (~18 cols) plus the add hint and gap.
	maxNameLen := width - 26 - addHintWidth - 2
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
					Foreground(lipgloss.Color("#5C4A3C")).
					Render(truncate(name, maxNameLen)),
		)

	// contentWidth is the space inside the border minus left+right padding (2 each).
	contentWidth := width - 6
	gap := contentWidth - lipgloss.Width(breadcrumb) - addHintWidth
	if gap < 1 {
		gap = 1
	}

	title := lipgloss.NewStyle().
		Padding(1, 2).
		Render(breadcrumb + strings.Repeat(" ", gap) + addHint)

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(title)
}

// renderDetailFooter renders the footer; "h home" and the border are highlighted
// when footer has focus, signalling the user can press enter or h.
func renderDetailFooter(focus detailFocus, width int) string {
	homeStyle := MutedStyle
	borderColor := ColorBorder

	if focus == detailFocusFooter {
		homeStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
		borderColor = ColorPrimary
	}

	keys := []string{
		"📜 ↑/↓ scroll",
		"🔍 / search",
		homeStyle.Render("🏠 h home"),
		MutedStyle.Render("✏️ e edit"),
		MutedStyle.Render("➕ a add"),
		MutedStyle.Render("🗑 d delete"),
		"🚪 q quit",
	}
	line := "  " + strings.Join(keys, "   ")

	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(borderColor).
		Width(width - 2).
		Render(line)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RunDetailUI runs the interactive recipe detail TUI.
// Returns goHome, goAdd, goEdit, deleteConfirmed booleans, the search query, and any error.
func RunDetailUI(recipe *models.Recipe) (goHome bool, goAdd bool, goEdit bool, deleteConfirmed bool, searchQuery string, err error) {
	m := NewDetailModel(recipe)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, runErr := p.Run()
	if runErr != nil {
		return false, false, false, false, "", runErr
	}
	fm := final.(DetailModel)
	return fm.GoHome(), fm.GoAdd(), fm.GoEdit(), fm.DeleteConfirmed(), fm.ReturnQuery(), nil
}
