package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/djcp/enplace/internal/models"
)

// FilterState is the complete search/filter state passed in and out of RunListUI.
type FilterState struct {
	Query      string
	Courses    []string
	Influences []string
	Status     string // "" = all; else "draft", "review", "published"
	IsBread    bool   // false = all recipes; true = bread/dough only
	MinRating  int    // 0 = any; 1–5 = minimum rating
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
	ffIsBread
	ffMinRating
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
	filterIsBread    bool   // false = all; true = bread/dough only
	filterMinRating  int    // 0 = any; 1–5 = minimum
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
	savedIsBread    bool
	savedMinRating  int

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
		filterIsBread:    initial.IsBread,
		filterMinRating:  initial.MinRating,
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
		IsBread:    m.filterIsBread,
		MinRating:  m.filterMinRating,
	}
}

func (m ListModel) hasActiveFilters() bool {
	return m.query != "" || len(m.filterCourses) > 0 ||
		len(m.filterInfluences) > 0 || m.filterStatus != "" || m.filterIsBread ||
		m.filterMinRating > 0
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
		isBread:         m.filterIsBread,
		minRating:       m.filterMinRating,
		courseBuffer:    m.courseBuffer,
		influenceBuffer: m.influenceBuffer,
		allCourses:      m.allCourses,
		allInfluences:   m.allInfluences,
		savedQuery:      m.savedQuery,
		savedCourses:    m.savedCourses,
		savedInfluences: m.savedInfluences,
		savedStatus:     m.savedStatus,
		savedIsBread:    m.savedIsBread,
		savedMinRating:  m.savedMinRating,
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
	m.filterIsBread = fs.isBread
	m.filterMinRating = fs.minRating
	m.courseBuffer = fs.courseBuffer
	m.influenceBuffer = fs.influenceBuffer
	m.savedQuery = fs.savedQuery
	m.savedCourses = fs.savedCourses
	m.savedInfluences = fs.savedInfluences
	m.savedStatus = fs.savedStatus
	m.savedIsBread = fs.savedIsBread
	m.savedMinRating = fs.savedMinRating
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
	visible := m.visibleRows()

	if len(m.recipes) == 0 {
		// Header row only, then no-match message, then fill to full height.
		// lipgloss's MaxHeight in Render strips the table's trailing \n; add it back.
		sb.WriteString(buildRecipeTable(nil, -1, width))
		sb.WriteString("\n")
		sb.WriteString(MutedStyle.Render("  No recipes match the current filters."))
		sb.WriteString("\n")
		for i := 1; i < 2*visible; i++ {
			sb.WriteString("\n")
		}
		return sb.String()
	}

	end := m.offset + visible
	if end > len(m.recipes) {
		end = len(m.recipes)
	}
	rendered := end - m.offset
	selectedIdx := m.cursor - m.offset
	if selectedIdx < 0 || selectedIdx >= rendered {
		selectedIdx = -1
	}

	// lipgloss's MaxHeight in Render strips the table's trailing \n; add it back.
	sb.WriteString(buildRecipeTable(m.recipes[m.offset:end], selectedIdx, width))
	sb.WriteString("\n")
	// Fill remaining viewport slots — each slot is 2 terminal lines.
	for i := rendered; i < visible; i++ {
		sb.WriteString("\n\n")
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
		Render("🍳  enplace")

	hints := MutedStyle.Render("🔍 / search") + "   " + MutedStyle.Render("⚙ m manage") + "   " + MutedStyle.Render("🏠 h home") + "   " + MutedStyle.Render("🚪 q quit")
	innerWidth := width - 6 // border(2) + padding(2+2)
	gap := innerWidth - lipgloss.Width(appName) - lipgloss.Width(hints)
	if gap < 1 {
		gap = 1
	}

	title := lipgloss.NewStyle().
		Padding(1, 2).
		Render(appName + strings.Repeat(" ", gap) + hints)

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(title)
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

// listColPad is the left padding applied to every table column.
const listColPad = 2

// truncateW truncates s to fit within maxCols terminal display columns,
// using display-width measurement so wide characters (emoji, CJK) are
// counted correctly. Appends "…" when content is cut, unless maxCols ≤ 2.
func truncateW(s string, maxCols int) string {
	if maxCols <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxCols {
		return s
	}
	if maxCols <= 2 {
		var out []rune
		used := 0
		for _, r := range s {
			rw := lipgloss.Width(string(r))
			if used+rw > maxCols {
				break
			}
			out = append(out, r)
			used += rw
		}
		return string(out)
	}
	var out []rune
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > maxCols-1 {
			break
		}
		out = append(out, r)
		used += rw
	}
	return string(out) + "…"
}

// buildRecipeTable renders a header row and the given recipe rows as a
// borderless lipgloss table. recipes is the visible window; selectedIdx is
// the 0-based index within that window of the selected row (-1 if none).
// width is the available pane width.
//
// The name column takes 60% of width; courses and time are fixed;
// status expands to fill the remainder. All rows are exactly 2 terminal
// lines: the selected row shows the recipe description on line 2, all
// others keep a blank second line for layout stability.
func buildRecipeTable(recipes []models.Recipe, selectedIdx, width int) string {
	// Fixed widths for courses, time, and status (content + listColPad).
	// Status minimum = 12 content cols, enough for the widest badge "⠋ processing".
	const (
		coursesColWidth = 14 + listColPad // 16
		timeColWidth    = 8 + listColPad  // 10
		statusMinWidth  = 12 + listColPad // 14
	)

	// Name takes 60% of width. If that leaves the status column below its
	// minimum, reduce name until status fits.
	//
	// All columns are fixed so their widths normally sum exactly to width,
	// preventing any cell from wrapping and keeping every row at exactly 2
	// terminal lines. Below ~60 columns the fixed columns alone exceed width
	// and the table will overflow the pane; that is accepted since the list
	// pane is 66% of the terminal and this only occurs below ~45 columns.
	nameColWidth := width * 60 / 100
	if nameColWidth < 20 {
		nameColWidth = 20
	}
	statusColWidth := width - nameColWidth - coursesColWidth - timeColWidth
	if statusColWidth < statusMinWidth {
		statusColWidth = statusMinWidth
		nameColWidth = width - coursesColWidth - timeColWidth - statusColWidth
		if nameColWidth < 20 {
			nameColWidth = 20
		}
	}
	nameContentWidth := nameColWidth - listColPad

	styleFunc := func(row, col int) lipgloss.Style {
		var base lipgloss.Style
		switch {
		case row == table.HeaderRow:
			base = MutedStyle
		case row == selectedIdx:
			base = HighlightStyle
		default:
			base = lipgloss.NewStyle()
		}
		switch col {
		case 0:
			return base.PaddingLeft(listColPad).Width(nameColWidth)
		case 1:
			return base.PaddingLeft(listColPad).Width(coursesColWidth)
		case 2:
			return base.PaddingLeft(listColPad).Width(timeColWidth)
		default: // status
			return base.PaddingLeft(listColPad).Width(statusColWidth)
		}
	}

	t := table.New().
		Headers("Name", "Courses", "Time", "Status").
		StyleFunc(styleFunc).
		Width(width).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderHeader(false).
		BorderColumn(false).
		BorderRow(false).
		Wrap(true)

	for i, r := range recipes {
		nameStr := r.Name
		if r.IsBread {
			nameStr = "🍞 " + r.Name
		}
		suffix := ""
		if g := r.RatingGlyphs(); g != "" {
			suffix = " " + g
		}
		if r.Notes != "" {
			suffix += " 📝"
		}
		nameAvail := nameContentWidth - lipgloss.Width(suffix)
		if nameAvail < 1 {
			nameAvail = 1
		}
		nameLine := truncateW(nameStr, nameAvail) + suffix

		// Name cell is always 2 lines for layout stability.
		var nameCell string
		if i == selectedIdx {
			desc := r.Description
			if desc == "" {
				desc = "no description"
			}
			descLine := lipgloss.NewStyle().Foreground(ColorMuted).Render(
				truncateW(desc, nameContentWidth),
			)
			nameCell = nameLine + "\n" + descLine
		} else {
			nameCell = nameLine + "\n"
		}

		courses := truncateW(strings.Join(r.TagsByContext(models.TagContextCourses), ", "), 14)
		timeStr := totalTimeStr(r.PreparationTime, r.CookingTime)
		status := StatusBadge(r.Status)

		t.Row(nameCell, courses, timeStr, status)
	}

	return t.String()
}

func renderFooter(width int) string {
	keys := []string{
		"📜 ↑/↓ scroll",
		"👁 enter view",
		"✏️ e edit",
		"🗑 d delete",
		"➕ a add",
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
