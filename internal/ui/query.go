package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/enplace/internal/db"
	"github.com/djcp/enplace/internal/services"
)

const queryAILimit = 60 * time.Second

type queryPhase int

const (
	queryPhaseInput queryPhase = iota
	queryPhaseLoading
	queryPhaseResults
	queryPhaseError
)

type queryMode int

const (
	queryModeNL queryMode = iota
	queryModeRaw
)

type queryFocus int

const (
	queryFocusSchema queryFocus = iota
	queryFocusMessages
	queryFocusEditor
)

// QueryResult is returned by RunQueryUI when the query screen closes.
type QueryResult struct {
	RecipeIDs []int64
	QueryText string // the NL question (empty for raw SQL)
	SQL       string // the generated/executed SQL
	Err       error
}

// Package-level state to remember the last query across invocations.
var (
	lastQueryText string
	lastQueryMode queryMode
	lastEditorVal string
)

// querySchemaEntry represents a table in the schema browser.
type querySchemaEntry struct {
	name    string
	columns []queryColumn
}

type queryColumn struct {
	name string
	typ  string
}

// QueryModel is the two-column query TUI:
//
//	Left 66%:  Messages pane (top) + Editor pane (bottom)
//	Right 33%: Schema pane (full height)
type QueryModel struct {
	sqlDB   *db.DB
	client  services.AIClient
	model   string
	dialect string

	width, height int
	phase         queryPhase
	focus         queryFocus
	mode          queryMode

	// Editor.
	editor textarea.Model

	// Messages pane (left, top).
	msgLines  []string
	msgScroll int

	// Schema browser (right, full height).
	schema      []querySchemaEntry
	schemaCur   int
	schemaLines []string

	// Last executed SQL (for display in messages pane).
	lastSQL string

	// Query text (NL question or raw SQL).
	queryText string

	// Columns/rows from last execution (for ID extraction).
	columns []string
	rows    [][]string

	// Error message.
	errMsg string

	// Result to return.
	result QueryResult
}

// querySchemaParse parses the schema text into a list of tables with columns.
func querySchemaParse(schemaText string) []querySchemaEntry {
	var tables []querySchemaEntry
	var current *querySchemaEntry

	for _, line := range strings.Split(schemaText, "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(strings.ToUpper(line), "CREATE TABLE") {
			name := line
			name = strings.ReplaceAll(name, "CREATE TABLE IF NOT EXISTS ", "")
			name = strings.ReplaceAll(name, "CREATE TABLE ", "")
			name = strings.TrimSpace(name)
			name = strings.TrimRight(name, "(")
			name = strings.TrimSpace(name)
			tables = append(tables, querySchemaEntry{name: name})
			current = &tables[len(tables)-1]
			continue
		}

		if current != nil {
			trimmed := strings.TrimLeft(line, " \t")
			if trimmed == "" || trimmed == ");" || strings.HasPrefix(strings.ToUpper(trimmed), "CREATE ") ||
				strings.HasPrefix(strings.ToUpper(trimmed), "UNIQUE ") {
				if trimmed == ");" || strings.HasPrefix(strings.ToUpper(trimmed), "CREATE ") || strings.HasPrefix(strings.ToUpper(trimmed), "UNIQUE ") {
					current = nil
				}
				continue
			}
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				colName := parts[0]
				colType := parts[1]
				upper := strings.ToUpper(colName)
				if upper == "PRIMARY" || upper == "NOT" || upper == "DEFAULT" || upper == "REFERENCES" ||
					upper == "UNIQUE" || upper == "CHECK" || upper == "ON" || upper == "CONSTRAINT" ||
					upper == ")" || upper == "" {
					continue
				}
				current.columns = append(current.columns, queryColumn{name: colName, typ: colType})
			}
		}
	}
	return tables
}

// querySchemaFlatten flattens the schema entries into renderable lines.
func querySchemaFlatten(entries []querySchemaEntry) []string {
	var lines []string
	for _, t := range entries {
		lines = append(lines, "tables")
		lines = append(lines, "├─ "+t.name+" ("+itoa(len(t.columns))+")")
		for _, c := range t.columns {
			lines = append(lines, "│  ├─ "+c.name+": "+c.typ)
		}
	}
	return lines
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// newQueryModel creates the query TUI model.
func newQueryModel(sqlDB *db.DB, client services.AIClient, model, dialect string) QueryModel {
	ed := textarea.New()
	ed.Placeholder = "Ask a question about your recipes..."
	ed.ShowLineNumbers = false
	ed.SetHeight(3)
	ed.Focus()

	// Restore last query state.
	if lastEditorVal != "" {
		ed.SetValue(lastEditorVal)
	}
	mode := lastQueryMode
	if mode == queryModeNL {
		ed.Placeholder = "Ask a question about your recipes..."
	} else {
		ed.Placeholder = "Enter SQL query..."
	}

	schemaText := services.SQLiteSchema
	if dialect == "postgres" {
		schemaText = services.PostgresSchema
	}
	entries := querySchemaParse(schemaText)

	m := QueryModel{
		sqlDB:   sqlDB,
		client:  client,
		model:   model,
		dialect: dialect,
		focus:   queryFocusEditor,
		mode:    mode,
		editor:  ed,
		schema:  entries,
	}
	m.schemaLines = querySchemaFlatten(entries)
	return m
}

func (m QueryModel) Init() tea.Cmd { return textarea.Blink }

func (m QueryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case querySQLGeneratedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.setMsgError(m.errMsg)
			m.phase = queryPhaseError
			return m, nil
		}
		m.lastSQL = msg.result.SQL
		m.setMsgSQL(msg.result.SQL)
		m.phase = queryPhaseLoading
		return m, m.executeSQLCmd(msg.result.SQL)

	case querySQLExecutedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.setMsgError(m.errMsg)
			m.phase = queryPhaseError
			return m, nil
		}
		m.columns = msg.columns
		m.rows = msg.rows
		m.phase = queryPhaseResults
		m.msgScroll = 0
		m.buildMsgResultLines(msg.duration)

		// Extract recipe IDs and auto-close.
		ids := extractIDs(msg.columns, msg.rows)
		if len(ids) == 0 {
			m.errMsg = "Query did not return recipe IDs — try asking about specific recipes."
			m.setMsgError(m.errMsg)
			m.phase = queryPhaseError
			return m, nil
		}
		m.result = QueryResult{
			RecipeIDs: ids,
			QueryText: m.queryText,
			SQL:       m.lastSQL,
		}
		return m, tea.Quit
	}

	// Forward to editor for cursor blink etc.
	if m.focus == queryFocusEditor {
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd
	}
	return m, nil
}

// extractIDs scans the query result columns for an "id" column and returns
// the values as int64 slices. Returns nil if no id column is found.
func extractIDs(columns []string, rows [][]string) []int64 {
	idIdx := -1
	for i, col := range columns {
		if strings.EqualFold(col, "id") {
			idIdx = i
			break
		}
	}
	if idIdx < 0 {
		return nil
	}

	var ids []int64
	for _, row := range rows {
		if idIdx >= len(row) {
			continue
		}
		id, err := strconv.ParseInt(row[idIdx], 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func (m QueryModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit

	case "tab":
		m.focus = (m.focus + 1) % 3
		if m.focus == queryFocusEditor {
			return m, m.editor.Focus()
		}
		return m, nil

	case "ctrl+n":
		if m.focus == queryFocusEditor {
			if m.mode == queryModeNL {
				m.mode = queryModeRaw
				m.editor.Placeholder = "Enter SQL query..."
			} else {
				m.mode = queryModeNL
				m.editor.Placeholder = "Ask a question about your recipes..."
			}
		}
		return m, nil

	case "ctrl+e", "ctrl+j":
		if m.focus == queryFocusEditor && m.phase != queryPhaseLoading {
			return m.executeQuery()
		}
		return m, nil

	case "up", "k":
		if m.focus == queryFocusMessages {
			m.moveMsgScroll(-1)
		} else if m.focus == queryFocusSchema {
			if m.schemaCur > 0 {
				m.schemaCur--
			}
		}
	case "down", "j":
		if m.focus == queryFocusMessages {
			m.moveMsgScroll(1)
		} else if m.focus == queryFocusSchema {
			if m.schemaCur < len(m.schemaLines)-1 {
				m.schemaCur++
			}
		}
	case "pgup":
		if m.focus == queryFocusMessages {
			m.moveMsgScroll(-m.msgVisibleRows())
		} else if m.focus == queryFocusSchema {
			m.schemaCur -= m.leftColHeight()
			if m.schemaCur < 0 {
				m.schemaCur = 0
			}
		}
	case "pgdown":
		if m.focus == queryFocusMessages {
			m.moveMsgScroll(m.msgVisibleRows())
		} else if m.focus == queryFocusSchema {
			m.schemaCur += m.leftColHeight()
			if m.schemaCur >= len(m.schemaLines) {
				m.schemaCur = len(m.schemaLines) - 1
			}
		}
	case "g":
		if m.focus == queryFocusMessages {
			m.msgScroll = 0
		} else if m.focus == queryFocusSchema {
			m.schemaCur = 0
		}
	case "G":
		if m.focus == queryFocusMessages {
			maxScroll := len(m.msgLines) - m.msgVisibleRows()
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.msgScroll = maxScroll
		} else if m.focus == queryFocusSchema {
			m.schemaCur = len(m.schemaLines) - 1
			if m.schemaCur < 0 {
				m.schemaCur = 0
			}
		}
	case "c":
		if m.focus == queryFocusMessages {
			m.msgLines = nil
			m.msgScroll = 0
			m.lastSQL = ""
			m.columns = nil
			m.rows = nil
			m.phase = queryPhaseInput
		}
	}

	// Forward to editor when focused.
	if m.focus == queryFocusEditor {
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd
	}
	return m, nil
}

// msgVisibleRows returns how many content lines fit in the messages pane.
func (m QueryModel) msgVisibleRows() int {
	return m.leftColMsgInnerH()
}

// leftColHeight returns the height of the left column (messages + editor).
func (m QueryModel) leftColHeight() int {
	v := m.height - 3 // banner(1) + footer(2)
	if v < 3 {
		v = 3
	}
	return v
}

// moveMsgScroll adjusts msgScroll by delta and clamps.
func (m *QueryModel) moveMsgScroll(delta int) {
	if len(m.msgLines) == 0 {
		return
	}
	m.msgScroll += delta
	if m.msgScroll < 0 {
		m.msgScroll = 0
	}
	maxScroll := len(m.msgLines) - m.msgVisibleRows()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.msgScroll > maxScroll {
		m.msgScroll = maxScroll
	}
}

// setMsgError sets the messages lines to display an error.
func (m *QueryModel) setMsgError(errMsg string) {
	m.msgLines = []string{
		ErrorStyle.Render("  " + errMsg),
	}
	m.msgScroll = 0
}

// setMsgSQL sets the messages lines to display the generated SQL.
func (m *QueryModel) setMsgSQL(sql string) {
	m.msgLines = []string{
		MutedStyle.Render("  Generated SQL:"),
		MutedStyle.Render(""),
	}
	for _, line := range strings.Split(sql, "\n") {
		m.msgLines = append(m.msgLines, "  "+line)
	}
	m.msgScroll = 0
}

// buildMsgResultLines builds messages lines after successful execution.
func (m *QueryModel) buildMsgResultLines(duration time.Duration) {
	var lines []string

	// Show the executed SQL.
	if m.lastSQL != "" {
		lines = append(lines, MutedStyle.Render("  Query:"))
		lines = append(lines, MutedStyle.Render(""))
		for _, l := range strings.Split(m.lastSQL, "\n") {
			lines = append(lines, "  "+l)
		}
		lines = append(lines, MutedStyle.Render(""))
	}

	lines = append(lines, MutedStyle.Render("  Completed in "+fmt.Sprintf("%.0f", float64(duration.Microseconds())/1000)+"ms"))

	m.msgLines = lines
	m.msgScroll = 0
}

// executeQuery runs the SQL in the editor (generating it from NL if needed).
func (m QueryModel) executeQuery() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.editor.Value())
	if text == "" {
		return m, nil
	}

	// Remember the query for next time.
	lastEditorVal = text
	lastQueryMode = m.mode

	if m.mode == queryModeNL {
		m.queryText = text
		m.phase = queryPhaseLoading
		m.msgLines = []string{MutedStyle.Render("  Generating SQL...")}
		m.msgScroll = 0
		return m, m.generateSQLCmd(text)
	}

	// Raw mode: validate and execute directly.
	if err := services.ValidateReadOnly(text); err != nil {
		m.errMsg = err.Error()
		m.setMsgError(m.errMsg)
		m.phase = queryPhaseError
		return m, nil
	}

	m.queryText = "" // raw SQL — no NL question
	m.phase = queryPhaseLoading
	m.lastSQL = text
	m.msgLines = []string{MutedStyle.Render("  Executing...")}
	m.msgScroll = 0
	return m, m.executeSQLCmd(text)
}

type querySQLGeneratedMsg struct {
	result *services.GenerateSQLResult
	err    error
}

type querySQLExecutedMsg struct {
	columns  []string
	rows     [][]string
	duration time.Duration
	err      error
}

func (m QueryModel) generateSQLCmd(question string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), queryAILimit)
		defer cancel()
		result, err := services.GenerateSQL(ctx, m.client, m.model, m.dialect, question)
		return querySQLGeneratedMsg{result: result, err: err}
	}
}

func (m QueryModel) executeSQLCmd(sql string) tea.Cmd {
	return func() tea.Msg {
		columns, rows, duration, err := services.ExecuteQuery(m.sqlDB, sql)
		return querySQLExecutedMsg{columns: columns, rows: rows, duration: duration, err: err}
	}
}

func (m QueryModel) View() string {
	if m.width == 0 {
		return ""
	}

	var sb strings.Builder

	// Banner.
	sb.WriteString(renderQueryBanner(m.width))
	sb.WriteString("\n\n")

	// Two-column layout: left 66% (messages + editor), right 33% (schema).
	leftW := m.width * 66 / 100
	rightW := m.width - leftW
	colH := m.leftColHeight()

	leftPane := m.renderLeftPane(leftW, colH)
	rightPane := m.renderSchemaPane(rightW, colH)
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane))

	// Footer.
	sb.WriteString(m.renderQueryFooter(m.width))

	return sb.String()
}

func (m QueryModel) renderLeftPane(width, height int) string {
	msgInnerH := m.leftColMsgInnerH()
	editorInnerH := height - 3 - msgInnerH
	if editorInnerH < 3 {
		editorInnerH = 3
	}

	var sb strings.Builder
	sb.WriteString(m.renderMessagesPane(width, msgInnerH))
	sb.WriteString("\n") // framePanel has no trailing \n; separator needed before next panel
	sb.WriteString(m.renderEditorPane(width, editorInnerH))
	return sb.String()
}

// leftColMsgInnerH returns the messages pane content height.
// Two framePanels stacked vertically must total the same rendered height
// as the single schema framePanel at the same colH.
// Each framePanel adds 2 border lines (top + bottom), plus 1 separator
// newline between them:
//
//	(msgInnerH + 2) + 1 + (editorInnerH + 2) = colH + 2
//	→ msgInnerH + editorInnerH = colH - 3
//
// Messages gets ~20% (it only shows the executed query, errors, or a
// placeholder), editor fills the rest.
func (m QueryModel) leftColMsgInnerH() int {
	colH := m.leftColHeight()
	v := colH * 20 / 100
	if v < 3 {
		v = 3
	}
	return v
}

func (m QueryModel) renderMessagesPane(width, height int) string {
	var content string

	switch {
	case m.phase == queryPhaseLoading && len(m.msgLines) == 0:
		content = MutedStyle.Render("  Generating SQL...")
	case len(m.msgLines) == 0:
		if m.editor.Value() == "" {
			if m.mode == queryModeNL {
				content = MutedStyle.Render("  Ask a question about your recipes in natural language.")
			} else {
				content = MutedStyle.Render("  Create an SQL query about your recipes.")
			}
		} else {
			content = MutedStyle.Render("  Press ctrl+e to execute.")
		}
	default:
		visible := m.msgVisibleRows()
		end := m.msgScroll + visible
		if end > len(m.msgLines) {
			end = len(m.msgLines)
		}
		var sb strings.Builder
		for _, line := range m.msgLines[m.msgScroll:end] {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		content = sb.String()
	}

	title := "Messages"
	borderColor := ColorBorder
	if m.focus == queryFocusMessages {
		borderColor = ColorPrimary
	}

	return framePanel(content, width, height, title, "", "", borderColor, ColorPrimary)
}

func (m QueryModel) renderSchemaPane(width, height int) string {
	var sb strings.Builder
	startLine := m.schemaCur
	if startLine < 0 {
		startLine = 0
	}
	endLine := startLine + height - 4
	if endLine > len(m.schemaLines) {
		endLine = len(m.schemaLines)
	}
	for _, line := range m.schemaLines[startLine:endLine] {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	content := sb.String()
	if content == "" {
		content = MutedStyle.Render("  No schema.")
	}

	borderColor := ColorBorder
	if m.focus == queryFocusSchema {
		borderColor = ColorPrimary
	}

	return framePanel(content, width, height, "Schema", "", "", borderColor, ColorPrimary)
}

func (m QueryModel) renderEditorPane(width, height int) string {
	borderColor := ColorBorder
	if m.focus == queryFocusEditor {
		borderColor = ColorPrimary
	}

	modeLabel := "natural language"
	if m.mode == queryModeRaw {
		modeLabel = "raw sql"
	}

	title := "Editor [" + modeLabel + "]"
	return framePanel(m.editor.View(), width, height, title, "", "", borderColor, ColorPrimary)
}

func renderQueryBanner(width int) string {
	return flatRuleStyled(width, breadcrumbTitle("manage / query"), "", ColorBorder, ColorPrimary)
}

func (m QueryModel) renderQueryFooter(width int) string {
	modeLabel := "natural language"
	if m.mode == queryModeRaw {
		modeLabel = "raw SQL"
	}
	keys := []string{
		keyHint("ctrl+e", "execute"),
		keyHint("ctrl+n", "mode ("+modeLabel+")"),
		keyHint("tab", "focus"),
		keyHint("j/k", "scroll"),
		keyHint("c", "clear"),
		keyHint("esc", "back"),
	}
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(footerLine(keys, width-2))
}

// RunQueryUI runs the interactive query screen and returns the result.
func RunQueryUI(sqlDB *db.DB, client services.AIClient, model string) QueryResult {
	dialect := "sqlite"
	if sqlDB.Driver() == "postgres" {
		dialect = "postgres"
	}

	m := newQueryModel(sqlDB, client, model, dialect)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return QueryResult{Err: fmt.Errorf("query UI: %w", err)}
	}

	if qm, ok := finalModel.(QueryModel); ok {
		return qm.result
	}
	return QueryResult{}
}
