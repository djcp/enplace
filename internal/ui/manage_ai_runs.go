package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/gorecipes/internal/db"
	"github.com/djcp/gorecipes/internal/models"
	"github.com/jmoiron/sqlx"
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
	sqlDB *sqlx.DB

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

func newManageAIRunsModel(sqlDB *sqlx.DB) manageAIRunsModel {
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
	// Banner(4) + footer(2) = 6
	v := m.height - 6
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
			visible := m.listVisibleRows()
			if m.cursor >= m.offset+visible {
				m.offset = m.cursor - visible + 1
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
	// Banner(4) + header(2) + footer(2) = 8
	v := m.height - 8
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

	visible := m.listVisibleRows() - 1
	if visible < 1 {
		visible = 1
	}
	end := m.offset + visible
	if end > len(m.runs) {
		end = len(m.runs)
	}

	if len(m.runs) == 0 {
		sb.WriteString(MutedStyle.Render("  No AI runs recorded."))
		sb.WriteString("\n")
	} else {
		for i := m.offset; i < end; i++ {
			r := m.runs[i]
			selected := i == m.cursor

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

			nameWidth := m.width - 90
			if nameWidth < 1 {
				nameWidth = 1
			}
			row := fmt.Sprintf("  %s  %-18s  %-26s  %s  %-8s  %s",
				dateStr,
				truncate(r.ServiceClass, 18),
				truncate(r.AIModel, 26),
				successMark,
				durStr,
				truncate(recipeName, nameWidth),
			)

			if selected {
				sb.WriteString(HighlightStyle.Width(m.width - 2).Render(row))
			} else {
				sb.WriteString(row)
			}
			sb.WriteString("\n")
		}
	}

	if m.listNotice != "" {
		noticeStyle := SuccessStyle
		if m.listNoticeErr {
			noticeStyle = ErrorStyle
		}
		used := strings.Count(sb.String(), "\n")
		if fill := m.height - used - 5; fill > 0 {
			sb.WriteString(strings.Repeat("\n", fill))
		}
		sb.WriteString("\n")
		sb.WriteString("  " + noticeStyle.Render(m.listNotice))
		sb.WriteString("\n")
	} else {
		used := strings.Count(sb.String(), "\n")
		if fill := m.height - used - 3; fill > 0 {
			sb.WriteString(strings.Repeat("\n", fill))
		}
	}
	sb.WriteString("\n")
	sb.WriteString(renderManageFooter([]string{"↑/↓ navigate", "enter view", "d delete", "p prune (30d)", "esc back"}, m.width))
	return sb.String()
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
func RunManageAIRunsUI(sqlDB *sqlx.DB) (retryRecipeID int64, err error) {
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
