package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/gorecipes/internal/models"
)

// FilterState is the complete search/filter state passed in and out of RunListUI.
type FilterState struct {
	Query      string
	Courses    []string
	Influences []string
	Status     string // "" = all; else "draft", "review", "published"
}

// SearchData holds autocomplete suggestions for the filter panel.
type SearchData struct {
	Courses    []string
	Influences []string
}

type filterFocus int

const (
	ffText filterFocus = iota
	ffCourses
	ffInfluences
	ffStatus
	ffSearch
	ffCount // total number of filter fields
)

// ListModel is a Bubbletea model for the interactive recipe browser.
type ListModel struct {
	recipes []models.Recipe
	cursor  int
	query   string
	typing  bool
	width   int
	height  int
	offset  int // scroll offset

	// Filter panel state.
	filterFocus      filterFocus
	filterCourses    []string
	filterInfluences []string
	filterStatus     string // "" = all
	courseBuffer     string // currently-being-typed for courses row
	influenceBuffer  string // currently-being-typed for influences row

	// Autocomplete suggestions (loaded once from DB).
	allCourses    []string
	allInfluences []string

	// Saved filter state — restored on Esc.
	savedQuery      string
	savedCourses    []string
	savedInfluences []string
	savedStatus     string

	// Set to > 0 when the user pressed Enter to view a recipe.
	selectedID      int64
	quitting        bool
	goAdd           bool
	goHome          bool
	goManage        bool
	searchConfirmed bool
	editID          int64

	// Delete confirmation state.
	confirmingDelete bool
	deleteTargetID   int64
	deleteTargetName string
}

// NewListModel creates a ListModel from a slice of recipes.
func NewListModel(recipes []models.Recipe, initial FilterState, sd SearchData) ListModel {
	m := ListModel{
		recipes:          recipes,
		width:            80,
		height:           24,
		query:            initial.Query,
		filterCourses:    initial.Courses,
		filterInfluences: initial.Influences,
		filterStatus:     initial.Status,
		allCourses:       sd.Courses,
		allInfluences:    sd.Influences,
	}
	return m
}

// SelectedID returns the recipe ID the user selected (0 if none).
func (m ListModel) SelectedID() int64 { return m.selectedID }

// GoAdd returns true when the user pressed "a" to add a new recipe.
func (m ListModel) GoAdd() bool { return m.goAdd }

// GoHome returns true when the user pressed "h" to go home (clear filter).
func (m ListModel) GoHome() bool { return m.goHome }

// SearchConfirmed returns true when the user pressed Enter to confirm a search.
func (m ListModel) SearchConfirmed() bool { return m.searchConfirmed }

// DeleteTargetID returns the recipe ID the user confirmed for deletion (0 if none).
func (m ListModel) DeleteTargetID() int64 { return m.deleteTargetID }

// EditID returns the recipe ID the user wants to edit (0 if none).
func (m ListModel) EditID() int64 { return m.editID }

// GoManage returns true when the user pressed "m" to open the manage screen.
func (m ListModel) GoManage() bool { return m.goManage }

// Query returns the current text search query.
func (m ListModel) Query() string { return m.query }

// Filter returns the current FilterState.
func (m ListModel) Filter() FilterState {
	return FilterState{
		Query:      m.query,
		Courses:    m.filterCourses,
		Influences: m.filterInfluences,
		Status:     m.filterStatus,
	}
}

func (m ListModel) hasActiveFilters() bool {
	return m.query != "" || len(m.filterCourses) > 0 ||
		len(m.filterInfluences) > 0 || m.filterStatus != ""
}

func (m ListModel) Init() tea.Cmd { return nil }

func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.confirmingDelete {
			return m.handleConfirmKey(msg)
		}
		if m.typing {
			return m.handleTypingKey(msg)
		}
		return m.handleNavKey(msg)
	}
	return m, nil
}

// toFilterState converts the ListModel's filter fields into a shared filterState.
func (m ListModel) toFilterState() filterState {
	return filterState{
		query:           m.query,
		focus:           m.filterFocus,
		courses:         m.filterCourses,
		influences:      m.filterInfluences,
		status:          m.filterStatus,
		courseBuffer:    m.courseBuffer,
		influenceBuffer: m.influenceBuffer,
		allCourses:      m.allCourses,
		allInfluences:   m.allInfluences,
		savedQuery:      m.savedQuery,
		savedCourses:    m.savedCourses,
		savedInfluences: m.savedInfluences,
		savedStatus:     m.savedStatus,
		active:          m.typing,
	}
}

// applyFilterState copies a shared filterState back into the ListModel's filter fields.
func (m ListModel) applyFilterState(fs filterState) ListModel {
	m.query = fs.query
	m.filterFocus = fs.focus
	m.filterCourses = fs.courses
	m.filterInfluences = fs.influences
	m.filterStatus = fs.status
	m.courseBuffer = fs.courseBuffer
	m.influenceBuffer = fs.influenceBuffer
	m.savedQuery = fs.savedQuery
	m.savedCourses = fs.savedCourses
	m.savedInfluences = fs.savedInfluences
	m.savedStatus = fs.savedStatus
	m.typing = fs.active
	return m
}

// enterTypingMode saves the current filter state and activates typing mode.
func (m ListModel) enterTypingMode() ListModel {
	return m.applyFilterState(m.toFilterState().enter())
}

func (m ListModel) handleTypingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fs, confirmed := handleFilterKey(m.toFilterState(), msg)
	m = m.applyFilterState(fs)
	if confirmed {
		m.searchConfirmed = true
		return m, tea.Quit
	}
	return m, nil
}

func (m ListModel) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		// deleteTargetID is already set; quitting signals confirmed deletion.
		m.confirmingDelete = false
		return m, tea.Quit
	case "n", "esc", "ctrl+c":
		m.confirmingDelete = false
		m.deleteTargetID = 0
		m.deleteTargetName = ""
	}
	return m, nil
}

func (m ListModel) handleNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		if m.hasActiveFilters() {
			// Clear active filters and return to the full list.
			m.goHome = true
			return m, tea.Quit
		}
		// Nothing to go back to — do nothing.
	case "a":
		m.goAdd = true
		return m, tea.Quit
	case "m":
		m.goManage = true
		return m, tea.Quit
	case "h":
		m.goHome = true
		return m, tea.Quit
	case "e":
		if len(m.recipes) > 0 {
			m.editID = m.recipes[m.cursor].ID
			return m, tea.Quit
		}
	case "d":
		if len(m.recipes) > 0 {
			m.confirmingDelete = true
			m.deleteTargetID = m.recipes[m.cursor].ID
			m.deleteTargetName = m.recipes[m.cursor].Name
		}
	case "/", "right":
		m = m.enterTypingMode()
		m.filterFocus = ffText
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}
	case "down", "j":
		if m.cursor < len(m.recipes)-1 {
			m.cursor++
			visible := m.visibleRows()
			if m.cursor >= m.offset+visible {
				m.offset = m.cursor - visible + 1
			}
		}
	case "enter", " ":
		if len(m.recipes) > 0 {
			m.selectedID = m.recipes[m.cursor].ID
			return m, tea.Quit
		}
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m ListModel) visibleRows() int {
	// Banner (4) + col header (1) + blank-before-footer (1) + footer (2) = 8 fixed overhead,
	// plus 1 for the terminal line between banner and content = 9 total.
	// Each recipe row is always 2 terminal lines (name/status + description), so divide by 2.
	v := (m.height - 9) / 2
	if v < 1 {
		v = 1
	}
	return v
}

func (m ListModel) View() string {
	var sb strings.Builder

	// Banner — full width.
	sb.WriteString(renderBanner(m.width))
	sb.WriteString("\n")

	// Delete confirmation overlay — replaces split content and footer.
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

	// Empty DB — show a centered info box with fill so the footer stays pinned.
	if len(m.recipes) == 0 && !m.hasActiveFilters() && !m.typing {
		sb.WriteString(m.viewEmpty())
		used := strings.Count(sb.String(), "\n")
		if fill := m.height - used - 3; fill > 0 {
			sb.WriteString(strings.Repeat("\n", fill))
		}
		sb.WriteString("\n")
		sb.WriteString(renderFooter(m.width))
		return sb.String()
	}

	// Split layout: list pane (66%) on the left, filter pane (33%) on the right.
	listWidth := (m.width * 2) / 3
	filterWidth := m.width - listWidth
	contentH := 2*m.visibleRows() + 1 // col header (1) + visible rows × 2 lines each

	leftPane := m.renderListPane(listWidth)
	rightPane := m.renderFilterPane(filterWidth, contentH)

	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane))
	sb.WriteString("\n")
	sb.WriteString(renderFooter(m.width))

	return sb.String()
}

// renderListPane renders the left pane (column headers + recipe rows + fill).
func (m ListModel) renderListPane(width int) string {
	var sb strings.Builder

	sb.WriteString(renderColumnHeaders(width))
	sb.WriteString("\n")

	visible := m.visibleRows()
	if len(m.recipes) == 0 {
		sb.WriteString(MutedStyle.Render("  No recipes match the current filters."))
		sb.WriteString("\n")
		// Fill remaining 2-line slots (minus the 1 line used by the message above).
		for i := 1; i < 2*visible; i++ {
			sb.WriteString("\n")
		}
	} else {
		end := m.offset + visible
		if end > len(m.recipes) {
			end = len(m.recipes)
		}
		for i := m.offset; i < end; i++ {
			r := m.recipes[i]
			selected := i == m.cursor
			sb.WriteString(renderRecipeRow(r, selected, width))
			sb.WriteString("\n")
		}
		// Fill remaining viewport slots — each slot is 2 terminal lines.
		for i := end - m.offset; i < visible; i++ {
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}

// renderFilterPane renders the right pane (filter inputs + scroll info) with a left border separator.
func (m ListModel) renderFilterPane(width, height int) string {
	var scrollHint string
	visible := m.visibleRows()
	if len(m.recipes) > visible {
		end := m.offset + visible
		if end > len(m.recipes) {
			end = len(m.recipes)
		}
		scrollHint = fmt.Sprintf("%d–%d of %d", m.offset+1, end, len(m.recipes))
	}
	return renderFilterPane(m.toFilterState(), width, height, scrollHint)
}

var statusCycle = []string{"", "draft", "review", "published"}

func nextStatus(s string) string {
	for i, v := range statusCycle {
		if v == s {
			return statusCycle[(i+1)%len(statusCycle)]
		}
	}
	return ""
}

func prevStatus(s string) string {
	for i, v := range statusCycle {
		if v == s {
			return statusCycle[(i-1+len(statusCycle))%len(statusCycle)]
		}
	}
	return ""
}

func findFirstMatch(buffer string, suggestions []string) string {
	if buffer == "" {
		return ""
	}
	lower := strings.ToLower(buffer)
	for _, s := range suggestions {
		if strings.HasPrefix(strings.ToLower(s), lower) {
			return s
		}
	}
	return ""
}

func resolveMatch(buffer string, suggestions []string) string {
	if match := findFirstMatch(buffer, suggestions); match != "" {
		return match
	}
	return buffer
}

func renderBanner(width int) string {
	appName := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Render("🍳  gorecipes")

	title := lipgloss.NewStyle().
		Padding(1, 2).
		Render(appName)

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(title)
}

func listNameWidth(width int) int {
	nw := width - 40
	if nw < 20 {
		nw = 20
	}
	return nw
}

func totalTimeStr(prepMins, cookMins *int) string {
	total := 0
	if prepMins != nil {
		total += *prepMins
	}
	if cookMins != nil {
		total += *cookMins
	}
	if total == 0 {
		return "—"
	}
	h := total / 60
	m := total % 60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

func renderColumnHeaders(width int) string {
	nw := listNameWidth(width)
	header := fmt.Sprintf("  %-*s  %-14s  %-8s  %s", nw, "Name", "Courses", "Time", "Status")
	return MutedStyle.Render(header)
}

func renderRecipeRow(r models.Recipe, selected bool, width int) string {
	nw := listNameWidth(width)

	name := truncate(r.Name, nw)
	courses := truncate(strings.Join(r.TagsByContext(models.TagContextCourses), ", "), 14)
	timeStr := totalTimeStr(r.PreparationTime, r.CookingTime)
	status := StatusBadge(r.Status)

	row := fmt.Sprintf("  %-*s  %-14s  %-8s  %s", nw, name, courses, timeStr, status)

	desc := truncate(r.Description, width-4)
	if selected {
		descLine := MutedStyle.Render("  " + desc)
		if desc == "" {
			descLine = MutedStyle.Render("  no description")
		}
		return HighlightStyle.Width(width).Render(row + "\n" + descLine)
	}
	// Non-selected: keep the second line blank so the layout never jumps.
	return row + "\n"
}

func renderFooter(width int) string {
	keys := []string{
		"🧭 ↑/↓ navigate",
		"🔍 / search",
		"👁 enter view",
		"✏️ e edit",
		"🗑 d delete",
		"➕ a add",
		"🏠 h home",
		"⚙ m manage",
		"🚪 q quit",
	}
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(footerLine(keys, width-2))
}

func (m ListModel) viewEmpty() string {
	inner := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("No recipes yet"),
		"",
		MutedStyle.Render("Press a to add your first — from a URL,"),
		MutedStyle.Render("pasted text, or entered manually."),
	)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 3).
		Render(inner)

	var sb strings.Builder
	sb.WriteString("\n\n")
	sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, box))
	sb.WriteString("\n")
	return sb.String()
}

func (m ListModel) viewConfirm() string {
	var sb strings.Builder
	sb.WriteString("\n\n")

	name := truncate(m.deleteTargetName, m.width-20)
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

func renderConfirmFooter(width int) string {
	yKey := lipgloss.NewStyle().Bold(true).Foreground(ColorError).Render("🗑 y delete")
	line := "  " + yKey + "   " + MutedStyle.Render("✖ esc cancel")
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorError).
		Width(width - 2).
		Render(line)
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len([]rune(s)) <= max {
		return s
	}
	runes := []rune(s)
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

// RunListUI runs the interactive recipe browser.
// Returns the selected recipe ID (or 0), navigation signals, the active filter state,
// the recipe ID confirmed for deletion (or 0), the recipe ID to edit (or 0),
// whether the user pressed "m" to open manage, and any error.
func RunListUI(
	recipes []models.Recipe,
	initial FilterState,
	sd SearchData,
) (selectedID int64, goAdd bool, goHome bool, searchConfirmed bool,
	filter FilterState, deleteID int64, editID int64, goManage bool, err error) {
	m := NewListModel(recipes, initial, sd)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, runErr := p.Run()
	if runErr != nil {
		return 0, false, false, false, FilterState{}, 0, 0, false, runErr
	}
	fm := final.(ListModel)
	return fm.SelectedID(), fm.GoAdd(), fm.GoHome(), fm.SearchConfirmed(),
		fm.Filter(), fm.DeleteTargetID(), fm.EditID(), fm.GoManage(), nil
}
