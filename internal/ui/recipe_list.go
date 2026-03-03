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

// enterTypingMode saves the current filter state and activates typing mode.
func (m ListModel) enterTypingMode() ListModel {
	m.savedQuery = m.query
	m.savedCourses = append([]string(nil), m.filterCourses...)
	m.savedInfluences = append([]string(nil), m.filterInfluences...)
	m.savedStatus = m.filterStatus
	m.typing = true
	return m
}

func (m ListModel) handleTypingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.typing = false
		m.query = m.savedQuery
		m.filterCourses = m.savedCourses
		m.filterInfluences = m.savedInfluences
		m.filterStatus = m.savedStatus
		m.savedQuery, m.savedCourses, m.savedInfluences, m.savedStatus = "", nil, nil, ""
		m.courseBuffer, m.influenceBuffer = "", ""

	case tea.KeyEnter:
		if m.filterFocus == ffCourses && m.courseBuffer != "" {
			m.filterCourses = append(m.filterCourses, resolveMatch(m.courseBuffer, m.allCourses))
			m.courseBuffer = ""
			return m, nil
		}
		if m.filterFocus == ffInfluences && m.influenceBuffer != "" {
			m.filterInfluences = append(m.filterInfluences, resolveMatch(m.influenceBuffer, m.allInfluences))
			m.influenceBuffer = ""
			return m, nil
		}
		m.typing = false
		m.searchConfirmed = true
		return m, tea.Quit

	case tea.KeyTab:
		m.courseBuffer, m.influenceBuffer = "", ""
		m.filterFocus = (m.filterFocus + 1) % 4

	case tea.KeyShiftTab:
		m.courseBuffer, m.influenceBuffer = "", ""
		m.filterFocus = (m.filterFocus - 1 + 4) % 4

	case tea.KeyLeft:
		if m.filterFocus == ffStatus {
			m.filterStatus = prevStatus(m.filterStatus)
		}

	case tea.KeyRight:
		if m.filterFocus == ffStatus {
			m.filterStatus = nextStatus(m.filterStatus)
		}

	case tea.KeyBackspace, tea.KeyDelete:
		switch m.filterFocus {
		case ffText:
			runes := []rune(m.query)
			if len(runes) > 0 {
				m.query = string(runes[:len(runes)-1])
			}
		case ffCourses:
			runes := []rune(m.courseBuffer)
			if len(runes) > 0 {
				m.courseBuffer = string(runes[:len(runes)-1])
			} else if len(m.filterCourses) > 0 {
				m.filterCourses = m.filterCourses[:len(m.filterCourses)-1]
			}
		case ffInfluences:
			runes := []rune(m.influenceBuffer)
			if len(runes) > 0 {
				m.influenceBuffer = string(runes[:len(runes)-1])
			} else if len(m.filterInfluences) > 0 {
				m.filterInfluences = m.filterInfluences[:len(m.filterInfluences)-1]
			}
		}

	default:
		if msg.Type == tea.KeyRunes {
			switch m.filterFocus {
			case ffText:
				m.query += string(msg.Runes)
			case ffCourses:
				m.courseBuffer += string(msg.Runes)
			case ffInfluences:
				m.influenceBuffer += string(msg.Runes)
			}
		}
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
	case "/":
		m = m.enterTypingMode()
		m.filterFocus = ffText
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		} else {
			// Already at the top row — move focus up into the search panel.
			m = m.enterTypingMode()
			m.filterFocus = ffText
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
	// Banner (4) + col header (1) + blank-before-footer (1) + footer (2) = 8 fixed overhead.
	// Selected row expands to 2 lines, so we reserve 1 extra = 9 total overhead.
	// Inactive filter bar adds 1 line.
	// Active filter panel is a 7-line bordered box (5 content rows + 2 border lines)
	// plus 2 blank lines after = 9 total lines taken, minus the base 1 = net 8 extra.
	v := m.height - 9
	if m.typing {
		v -= 8
	} else if m.hasActiveFilters() {
		v -= 1
	}
	if v < 1 {
		v = 1
	}
	return v
}

func (m ListModel) View() string {
	var sb strings.Builder

	// Banner.
	sb.WriteString(renderBanner(m.width))
	sb.WriteString("\n")

	// Delete confirmation overlay — replaces list content and footer.
	if m.confirmingDelete {
		confirmContent := m.viewConfirm()
		sb.WriteString(confirmContent)
		// Fill remaining height so the footer stays pinned.
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

	// Filter panel — expanded when typing, compact summary line when filters active.
	if m.typing || m.hasActiveFilters() {
		if m.typing {
			sb.WriteString(m.renderSearchPanel())
			sb.WriteString("\n\n")
		} else {
			sb.WriteString(m.renderInactiveFilter())
			sb.WriteString("\n")
		}
	}

	// Column headers.
	sb.WriteString(renderColumnHeaders(m.width))
	sb.WriteString("\n")

	visible := m.visibleRows()
	if len(m.recipes) == 0 {
		// A filter/search was active but returned nothing.
		sb.WriteString(MutedStyle.Render("  No recipes match the current filters."))
		sb.WriteString("\n")
		for i := 1; i <= visible; i++ {
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
			sb.WriteString(renderRecipeRow(r, selected, m.width))
			sb.WriteString("\n")
		}

		// Fill remaining viewport rows so the footer stays pinned.
		for i := end - m.offset; i < visible; i++ {
			sb.WriteString("\n")
		}

		// Scroll hint — only shown when there are more recipes than fit.
		if len(m.recipes) > visible {
			total := len(m.recipes)
			shown := fmt.Sprintf("  %d–%d of %d", m.offset+1, end, total)
			sb.WriteString("\n")
			sb.WriteString(MutedStyle.Render(shown))
			sb.WriteString("\n")
		}
	}

	// Footer keybindings.
	sb.WriteString("\n")
	sb.WriteString(renderFooter(m.width))

	return sb.String()
}

// renderSearchPanel renders the expanded filter panel shown when m.typing is true.
func (m ListModel) renderSearchPanel() string {
	rows := []string{
		renderTextSearchRow(m.query, m.filterFocus == ffText),
		MutedStyle.Render("  tab: next field    enter: search    esc: cancel"),
		renderTagFilterRow("courses:", models.TagContextCourses, m.filterCourses, m.courseBuffer, m.allCourses, m.filterFocus == ffCourses),
		renderTagFilterRow("influences:", models.TagContextCulturalInfluences, m.filterInfluences, m.influenceBuffer, m.allInfluences, m.filterFocus == ffInfluences),
		renderStatusRow(m.filterStatus, m.filterFocus == ffStatus),
	}
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorPrimary).
		Width(m.width-4).
		Padding(0, 1).
		Render(strings.Join(rows, "\n"))
}

// renderInactiveFilter renders the compact summary line when filters are active but panel is closed.
func (m ListModel) renderInactiveFilter() string {
	prefix := MutedStyle.Render("  / ")
	var parts []string
	if m.query != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(ColorPrimary).Render(m.query))
	}
	if len(m.filterCourses) > 0 {
		parts = append(parts, MutedStyle.Render("courses: ")+strings.Join(m.filterCourses, ", "))
	}
	if len(m.filterInfluences) > 0 {
		parts = append(parts, MutedStyle.Render("influences: ")+strings.Join(m.filterInfluences, ", "))
	}
	if m.filterStatus != "" {
		parts = append(parts, StatusBadge(m.filterStatus))
	}
	sep := MutedStyle.Render("  ·  ")
	return prefix + strings.Join(parts, sep)
}

// renderSearchBar renders the single-row bordered search bar used by the detail view.
func renderSearchBar(query string, typing bool, width int) string {
	prefix := MutedStyle.Render("  / ")
	cursor := lipgloss.NewStyle().
		Background(ColorPrimary).
		Foreground(lipgloss.Color("#FFFFFF")).
		Render(" ")

	var content string
	if typing {
		if query == "" {
			content = MutedStyle.Render("search by title or ingredient...") + cursor
		} else {
			content = query + cursor
		}
	} else if query != "" {
		content = lipgloss.NewStyle().Foreground(ColorPrimary).Render(query)
	} else {
		content = MutedStyle.Render("search by title or ingredient...")
	}

	bar := lipgloss.NewStyle().
		Width(width - 2).
		Render(prefix + content)

	if typing {
		bar = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorPrimary).
			Width(width-4).
			Padding(0, 1).
			Render(prefix + content)
	}

	return bar
}

func renderTextSearchRow(query string, focused bool) string {
	prefix := MutedStyle.Render("  / ")
	cursor := lipgloss.NewStyle().
		Background(ColorPrimary).
		Foreground(lipgloss.Color("#FFFFFF")).
		Render(" ")

	var content string
	if focused {
		if query == "" {
			content = MutedStyle.Render("search by title or ingredient...") + cursor
		} else {
			content = query + cursor
		}
	} else if query != "" {
		content = lipgloss.NewStyle().Foreground(ColorPrimary).Render(query)
	} else {
		content = MutedStyle.Render("search by title or ingredient...")
	}

	return prefix + content
}

func renderTagFilterRow(label, ctx string, pills []string, buffer string, suggestions []string, focused bool) string {
	var sb strings.Builder
	sb.WriteString(MutedStyle.Render(fmt.Sprintf("  %-12s", label)))

	pillStyle := lipgloss.NewStyle().
		Background(TagStyle(ctx).GetBackground()).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1)

	for _, p := range pills {
		sb.WriteString(pillStyle.Render(p))
		sb.WriteString(" ")
	}

	if focused {
		cursor := lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(lipgloss.Color("#FFFFFF")).
			Render(" ")
		match := findFirstMatch(buffer, suggestions)
		if buffer != "" {
			sb.WriteString(buffer)
			if match != "" {
				sb.WriteString(cursor)
				sb.WriteString(MutedStyle.Render(match[len(buffer):]))
			} else {
				sb.WriteString(cursor)
			}
		} else {
			if len(pills) == 0 {
				sb.WriteString(MutedStyle.Render("type to filter... "))
			}
			sb.WriteString(cursor)
		}
	} else if len(pills) == 0 {
		sb.WriteString(MutedStyle.Render("any"))
	}

	return sb.String()
}

func renderStatusRow(status string, focused bool) string {
	var sb strings.Builder
	sb.WriteString(MutedStyle.Render("  status:     "))

	if focused {
		sb.WriteString(MutedStyle.Render("◀ "))
		if status == "" {
			sb.WriteString("all")
		} else {
			sb.WriteString(StatusBadge(status))
		}
		sb.WriteString(MutedStyle.Render(" ▶"))
	} else {
		if status == "" {
			sb.WriteString(MutedStyle.Render("all"))
		} else {
			sb.WriteString(StatusBadge(status))
		}
	}

	return sb.String()
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

	if selected {
		desc := truncate(r.Description, width-4)
		if desc == "" {
			desc = MutedStyle.Render("  no description")
		} else {
			desc = MutedStyle.Render("  " + desc)
		}
		return HighlightStyle.Width(width).Render(row + "\n" + desc)
	}
	return row
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
