package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/djcp/enplace/internal/db"
	"github.com/djcp/enplace/internal/models"
)

type manageAIRunsPhase int

const (
	manageAIRunsPhaseList manageAIRunsPhase = iota
	manageAIRunsPhaseDetail
	manageAIRunsPhaseDeleteConfirm
	manageAIRunsPhaseRetryConfirm
	manageAIRunsPhasePruneConfirm
	manageAIRunsPhasePruneResult
)

const pruneAge = 30 * 24 * time.Hour

// manageAIRunsModel is the TUI model for AI runs browsing.
type manageAIRunsModel struct {
	sqlDB *db.DB

	phase manageAIRunsPhase

	// List view.
	runs   []db.AIRunSummary
	cursor int
	offset int

	// Detail view.
	fullRun      *models.AIClassifierRun
	detailLines  []string
	detailScroll int

	// Delete single run.
	deleteTargetID int64

	// Retry failed run.
	retryTargetRecipeID int64
	retryTargetName     string
	retryRecipeID       int64 // set on confirm; returned to caller via RunManageAIRunsUI

	// listNotice is shown inline on the list view after a delete (cleared on next delete/prune).
	listNotice    string
	listNoticeErr bool

	// Result message (prune full-page result).
	resultMsg string
	resultErr bool

	width  int
	height int
}

func newManageAIRunsModel(sqlDB *db.DB) manageAIRunsModel {
	return manageAIRunsModel{sqlDB: sqlDB, width: 80, height: 24}
}

func (m *manageAIRunsModel) loadRuns() error {
	runs, err := db.ListAIRunSummaries(m.sqlDB)
	if err != nil {
		return err
	}
	m.runs = runs
	return nil
}

func (m manageAIRunsModel) Init() tea.Cmd { return nil }

func (m manageAIRunsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.phase == manageAIRunsPhaseDetail {
			m.detailLines = m.buildDetailLines()
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m manageAIRunsModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.phase {
	case manageAIRunsPhaseList:
		return m.handleListKey(msg)
	case manageAIRunsPhaseDetail:
		return m.handleDetailKey(msg)
	case manageAIRunsPhaseDeleteConfirm:
		return m.handleDeleteConfirmKey(msg)
	case manageAIRunsPhaseRetryConfirm:
		return m.handleRetryConfirmKey(msg)
	case manageAIRunsPhasePruneConfirm:
		return m.handlePruneConfirmKey(msg)
	case manageAIRunsPhasePruneResult:
		// Any key → back to list.
		if err := m.loadRuns(); err != nil {
			m.resultMsg = "Error reloading: " + err.Error()
			m.resultErr = true
			return m, nil
		}
		m.cursor = 0
		m.offset = 0
		m.phase = manageAIRunsPhaseList
		return m, nil
	}
	return m, nil
}

func (m manageAIRunsModel) listVisibleRows() int {
	// Banner rule(1) + blank-before-footer(1) + footer(2) = 4
	v := m.height - 4
	if v < 1 {
		v = 1
	}
	return v
}

// listDataRows returns the number of AI run data rows the list view can display.
// It accounts for the leading blank line and the lipgloss table header row
// within the content area returned by listVisibleRows.
//
// Use this value — not listVisibleRows — wherever the rendered window size matters:
// both in viewList (to slice m.runs) and in handleListKey (scroll threshold).
// Using the same value in both places guarantees the selected row is always visible.
func (m manageAIRunsModel) listDataRows() int {
	// Subtract 1 for the leading blank line at the top of viewList,
	// and 1 for the lipgloss table header row.
	v := m.listVisibleRows() - 2
	if m.listNotice != "" {
		// The inline notice consumes two lines (blank + notice) above the
		// footer; shrink the data window so the screen height stays exact.
		v -= 2
	}
	if v < 1 {
		v = 1
	}
	return v
}

func (m manageAIRunsModel) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}
	case "down", "j":
		if m.cursor < len(m.runs)-1 {
			m.cursor++
			dataRows := m.listDataRows()
			if m.cursor >= m.offset+dataRows {
				m.offset = m.cursor - dataRows + 1
			}
		}
	case "enter", " ":
		if len(m.runs) == 0 {
			return m, nil
		}
		run, err := db.GetAIRun(m.sqlDB, m.runs[m.cursor].ID)
		if err != nil {
			m.resultMsg = "Error loading run: " + err.Error()
			m.resultErr = true
			m.phase = manageAIRunsPhasePruneResult
			return m, nil
		}
		m.fullRun = run
		m.detailScroll = 0
		m.detailLines = m.buildDetailLines()
		m.phase = manageAIRunsPhaseDetail
	case "d":
		if len(m.runs) == 0 {
			return m, nil
		}
		m.deleteTargetID = m.runs[m.cursor].ID
		m.phase = manageAIRunsPhaseDeleteConfirm
	case "p":
		m.phase = manageAIRunsPhasePruneConfirm
	}
	return m, nil
}

func (m manageAIRunsModel) handleDeleteConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "n":
		m.phase = manageAIRunsPhaseList
		return m, nil
	case "y", "enter":
		// Build the notice message before deleting (run won't be in list after reload).
		notice := fmt.Sprintf("Run #%d deleted.", m.deleteTargetID)
		noticeErr := false
		for _, r := range m.runs {
			if r.ID == m.deleteTargetID {
				name := r.RecipeName
				if name == "" {
					name = "deleted recipe"
				}
				notice = fmt.Sprintf("Run #%d removed — %s for \"%s\".", r.ID, r.ServiceClass, name)
				break
			}
		}
		if err := db.DeleteAIRun(m.sqlDB, m.deleteTargetID); err != nil {
			notice = "Error deleting run: " + err.Error()
			noticeErr = true
		}
		_ = m.loadRuns()
		if m.cursor >= len(m.runs) && m.cursor > 0 {
			m.cursor = len(m.runs) - 1
		}
		m.listNotice = notice
		m.listNoticeErr = noticeErr
		m.phase = manageAIRunsPhaseList
	}
	return m, nil
}

func (m manageAIRunsModel) detailViewportHeight() int {
	// Banner rule(1) + summary header(3) + blank-before-footer(1) + footer(2) = 6
	v := m.height - 6
	if v < 1 {
		v = 1
	}
	return v
}

func (m manageAIRunsModel) maxDetailScroll() int {
	ms := len(m.detailLines) - m.detailViewportHeight()
	if ms < 0 {
		return 0
	}
	return ms
}

func (m manageAIRunsModel) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.phase = manageAIRunsPhaseList
		return m, nil
	case "up", "k":
		if m.detailScroll > 0 {
			m.detailScroll--
		}
	case "down", "j":
		if m.detailScroll < m.maxDetailScroll() {
			m.detailScroll++
		}
	case "pgup":
		m.detailScroll -= m.detailViewportHeight()
		if m.detailScroll < 0 {
			m.detailScroll = 0
		}
	case "pgdown":
		m.detailScroll += m.detailViewportHeight()
		if m.detailScroll > m.maxDetailScroll() {
			m.detailScroll = m.maxDetailScroll()
		}
	case "r":
		if m.fullRun != nil && m.fullRun.RecipeID != nil {
			m.retryTargetRecipeID = *m.fullRun.RecipeID
			if len(m.runs) > 0 && m.cursor < len(m.runs) {
				m.retryTargetName = m.runs[m.cursor].RecipeName
			}
			m.phase = manageAIRunsPhaseRetryConfirm
		}
	}
	return m, nil
}

func (m manageAIRunsModel) handleRetryConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "n":
		m.phase = manageAIRunsPhaseDetail
		return m, nil
	case "y", "enter":
		m.retryRecipeID = m.retryTargetRecipeID
		return m, tea.Quit
	}
	return m, nil
}

func (m manageAIRunsModel) handlePruneConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "n":
		m.phase = manageAIRunsPhaseList
		return m, nil
	case "y", "enter":
		count, err := db.DeleteAIRunsOlderThan(m.sqlDB, pruneAge)
		if err != nil {
			m.resultMsg = "Error pruning: " + err.Error()
			m.resultErr = true
		} else {
			m.resultMsg = fmt.Sprintf("Pruned %d run(s) older than 30 days.", count)
			m.resultErr = false
		}
		m.phase = manageAIRunsPhasePruneResult
	}
	return m, nil
}

func (m manageAIRunsModel) View() string {
	if m.width == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(renderManageBanner("AI runs", m.width))
	sb.WriteString("\n")

	switch m.phase {
	case manageAIRunsPhaseList:
		sb.WriteString(m.viewList())
	case manageAIRunsPhaseDetail:
		sb.WriteString(m.viewDetail())
	case manageAIRunsPhaseDeleteConfirm:
		sb.WriteString(m.viewDeleteConfirm())
	case manageAIRunsPhaseRetryConfirm:
		sb.WriteString(m.viewRetryConfirm())
	case manageAIRunsPhasePruneConfirm:
		sb.WriteString(m.viewPruneConfirm())
	case manageAIRunsPhasePruneResult:
		sb.WriteString(m.viewPruneResult())
	}

	return sb.String()
}

func (m manageAIRunsModel) viewList() string {
	var sb strings.Builder
	sb.WriteString("\n")

	dataRows := m.listDataRows()
	end := m.offset + dataRows
	if end > len(m.runs) {
		end = len(m.runs)
	}

	tableWidth := m.width - 2
	if len(m.runs) == 0 {
		sb.WriteString(buildAIRunsTable(nil, -1, tableWidth))
		sb.WriteString("\n")
		sb.WriteString(MutedStyle.Render("  No AI runs recorded."))
		sb.WriteString("\n")
	} else {
		selectedIdx := m.cursor - m.offset
		rendered := end - m.offset
		if selectedIdx < 0 || selectedIdx >= rendered {
			selectedIdx = -1
		}
		sb.WriteString(buildAIRunsTable(m.runs[m.offset:end], selectedIdx, tableWidth))
		sb.WriteString("\n")
	}

	if m.listNotice != "" {
		noticeStyle := SuccessStyle
		if m.listNoticeErr {
			noticeStyle = ErrorStyle
		}
		used := strings.Count(sb.String(), "\n")
		if fill := m.height - used - 6; fill > 0 {
			sb.WriteString(strings.Repeat("\n", fill))
		}
		sb.WriteString("\n")
		sb.WriteString("  " + noticeStyle.Render(m.listNotice))
		sb.WriteString("\n")
	} else {
		used := strings.Count(sb.String(), "\n")
		if fill := m.height - used - 4; fill > 0 {
			sb.WriteString(strings.Repeat("\n", fill))
		}
	}
	sb.WriteString("\n")
	sb.WriteString(renderManageFooter([]string{"↑/↓ navigate", "enter view", "d delete", "p prune (30d)", "esc back"}, m.width))
	return sb.String()
}

// buildAIRunsTable renders a header row and the given AI run rows as a
// borderless lipgloss table. runs is the visible window; selectedIdx is the
// 0-based index within that window of the selected row (-1 if none).
// width is the available content width (typically m.width - 2).
//
// Rows are single-line (Wrap false) so the date, service, model, success,
// duration, and recipe columns are each truncated to their allocated width.
func buildAIRunsTable(runs []db.AIRunSummary, selectedIdx, width int) string {
	const (
		dateColWidth    = 10 + listColPad // 12
		serviceColWidth = 18 + listColPad // 20
		modelColWidth   = 26 + listColPad // 28
		successColWidth = 1 + listColPad  // 3
		durColWidth     = 8 + listColPad  // 10
	)
	const fixedCols = dateColWidth + serviceColWidth + modelColWidth + successColWidth + durColWidth // 73

	recipeColWidth := width - fixedCols
	if recipeColWidth < 8 {
		recipeColWidth = 8
	}

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
			return base.PaddingLeft(listColPad).Width(dateColWidth)
		case 1:
			return base.PaddingLeft(listColPad).Width(serviceColWidth)
		case 2:
			return base.PaddingLeft(listColPad).Width(modelColWidth)
		case 3:
			return base.PaddingLeft(listColPad).Width(successColWidth)
		case 4:
			return base.PaddingLeft(listColPad).Width(durColWidth)
		default: // recipe name
			return base.PaddingLeft(listColPad).Width(recipeColWidth)
		}
	}

	t := table.New().
		Headers("Date", "Service", "Model", "OK", "Duration", "Recipe").
		StyleFunc(styleFunc).
		Width(width).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderHeader(false).
		BorderColumn(false).
		BorderRow(false).
		Wrap(false)

	for _, r := range runs {
		dateStr := r.CreatedAt.Format("2006-01-02")
		successMark := lipgloss.NewStyle().Foreground(ColorSuccess).Render("✓")
		if !r.Success {
			successMark = lipgloss.NewStyle().Foreground(ColorError).Render("✗")
		}
		durStr := ""
		if r.DurationMS >= 0 {
			durStr = fmt.Sprintf("%dms", r.DurationMS)
		}
		recipeName := r.RecipeName
		if recipeName == "" {
			recipeName = MutedStyle.Render("(deleted)")
		}
		t.Row(dateStr, r.ServiceClass, r.AIModel, successMark, durStr, recipeName)
	}

	return t.String()
}

func (m manageAIRunsModel) buildDetailLines() []string {
	if m.fullRun == nil {
		return nil
	}
	r := m.fullRun
	contentWidth := m.width - 4
	if contentWidth > 100 {
		contentWidth = 100
	}
	if contentWidth < 20 {
		contentWidth = 20
	}

	var sb strings.Builder

	// Header info.
	successStr := lipgloss.NewStyle().Foreground(ColorSuccess).Render("succeeded")
	if !r.Success {
		successStr = lipgloss.NewStyle().Foreground(ColorError).Render("failed")
	}
	durStr := ""
	if r.DurationMS() >= 0 {
		durStr = fmt.Sprintf("  Duration: %dms", r.DurationMS())
	}

	sb.WriteString(fmt.Sprintf("  ID: %d   Service: %s   Model: %s\n", r.ID, r.ServiceClass, r.AIModel))
	sb.WriteString(fmt.Sprintf("  Status: %s%s\n", successStr, durStr))
	sb.WriteString(fmt.Sprintf("  Created:   %s\n", r.CreatedAt.Format("Jan 2, 2006  3:04:05 PM MST")))
	if r.StartedAt != nil {
		sb.WriteString(fmt.Sprintf("  Started:   %s\n", r.StartedAt.Format("Jan 2, 2006  3:04:05 PM MST")))
	}
	if r.CompletedAt != nil {
		sb.WriteString(fmt.Sprintf("  Completed: %s\n", r.CompletedAt.Format("Jan 2, 2006  3:04:05 PM MST")))
	}
	if r.ErrorMessage != "" {
		sb.WriteString(fmt.Sprintf("  Error: %s — %s\n", r.ErrorClass, r.ErrorMessage))
	}
	sb.WriteString("\n")

	// Section helper.
	writeSectionHeader := func(label string) {
		sb.WriteString(SectionLabelStyle.Render("  " + label))
		sb.WriteString("\n")
		sb.WriteString(MutedStyle.Render(strings.Repeat("─", contentWidth)))
		sb.WriteString("\n")
	}

	writeWrapped := func(text string) {
		if text == "" {
			sb.WriteString(MutedStyle.Render("  (empty)"))
			sb.WriteString("\n")
			return
		}
		for _, line := range strings.Split(text, "\n") {
			// Wrap long lines.
			for len([]rune(line)) > contentWidth {
				chunk := string([]rune(line)[:contentWidth])
				sb.WriteString("  " + chunk + "\n")
				line = string([]rune(line)[contentWidth:])
			}
			sb.WriteString("  " + line + "\n")
		}
	}

	writeSectionHeader("SYSTEM PROMPT")
	writeWrapped(r.SystemPrompt)
	sb.WriteString("\n")

	writeSectionHeader("USER PROMPT")
	writeWrapped(r.UserPrompt)
	sb.WriteString("\n")

	writeSectionHeader("RAW RESPONSE")
	writeWrapped(r.RawResponse)

	return strings.Split(sb.String(), "\n")
}

func (m manageAIRunsModel) viewDetail() string {
	var sb strings.Builder

	// Sub-header.
	summaryLine := ""
	if len(m.runs) > 0 && m.cursor < len(m.runs) {
		r := m.runs[m.cursor]
		recipeName := r.RecipeName
		if recipeName == "" {
			recipeName = "(deleted)"
		}
		summaryLine = fmt.Sprintf("  %s", truncate(recipeName, m.width-10))
	}
	sb.WriteString("\n")
	sb.WriteString(MutedStyle.Render(summaryLine))
	sb.WriteString("\n")

	vh := m.detailViewportHeight()
	start := m.detailScroll
	end := start + vh
	if end > len(m.detailLines) {
		end = len(m.detailLines)
	}

	for i := start; i < end; i++ {
		sb.WriteString(m.detailLines[i])
		sb.WriteString("\n")
	}
	for i := end - start; i < vh; i++ {
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	footerHints := []string{"↑/↓/pgup/pgdown scroll", "esc back"}
	if m.fullRun != nil && m.fullRun.RecipeID != nil {
		footerHints = append(footerHints, "r retry")
	}
	sb.WriteString(renderManageFooter(footerHints, m.width))
	return sb.String()
}

func (m manageAIRunsModel) viewDeleteConfirm() string {
	runLabel := fmt.Sprintf("run #%d", m.deleteTargetID)
	for _, r := range m.runs {
		if r.ID == m.deleteTargetID {
			name := r.RecipeName
			if name == "" {
				name = "(deleted recipe)"
			}
			runLabel = fmt.Sprintf("run #%d — %s / %s", r.ID, r.ServiceClass, name)
			break
		}
	}
	return buildCenteredBox(
		"Delete AI run?", ColorError, ColorError,
		[]string{
			MutedStyle.Render(truncate(runLabel, m.width-12)),
			MutedStyle.Render("This cannot be undone."),
		},
		m.width, m.height,
		renderManageConfirmFooter("y delete", ColorError, m.width),
	)
}

func (m manageAIRunsModel) viewRetryConfirm() string {
	name := m.retryTargetName
	if name == "" {
		name = "(unknown recipe)"
	}
	return buildCenteredBox(
		"Retry extraction?", ColorWarning, ColorWarning,
		[]string{
			MutedStyle.Render(truncate(name, m.width-12)),
			MutedStyle.Render("Re-run AI extraction for this recipe."),
		},
		m.width, m.height,
		renderManageConfirmFooter("y retry", ColorWarning, m.width),
	)
}

func (m manageAIRunsModel) viewPruneConfirm() string {
	return buildCenteredBox(
		"Prune old runs?", ColorWarning, ColorWarning,
		[]string{
			MutedStyle.Render("Delete AI runs older than 30 days?"),
			MutedStyle.Render("This cannot be undone."),
		},
		m.width, m.height,
		renderManageConfirmFooter("y prune", ColorWarning, m.width),
	)
}

func (m manageAIRunsModel) viewPruneResult() string {
	return viewManageResult(
		m.resultMsg, m.resultErr,
		m.width, m.height,
		renderManageFooter([]string{"any key continue"}, m.width),
	)
}

// RunManageAIRunsUI runs the AI runs management TUI.
// Returns retryRecipeID > 0 if the user confirmed a retry, 0 for normal exit.
func RunManageAIRunsUI(sqlDB *db.DB) (retryRecipeID int64, err error) {
	m := newManageAIRunsModel(sqlDB)
	if err := m.loadRuns(); err != nil {
		return 0, err
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return 0, err
	}
	fm := final.(manageAIRunsModel)
	return fm.retryRecipeID, nil
}
