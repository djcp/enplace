package ui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/enplace/internal/db"
	"github.com/djcp/enplace/internal/services"
)

type queryPhase int

const (
	queryPhaseInput queryPhase = iota
	queryPhaseLoading
	queryPhaseResults
	queryPhaseError
)

type queryMode int

const (
	queryModeNL  queryMode = iota // natural language
	queryModeRaw                  // raw SQL
)

type queryFocus int

const (
	queryFocusEditor queryFocus = iota
	queryFocusResults
	queryFocusSchema
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

// QueryModel is the three-pane query TUI: results (upper left),
// schema browser (upper right), and SQL editor (bottom).
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

	// Results.
	columns   []string
	rows      [][]string
	duration  time.Duration
	resultMsg string
	resultErr bool
	scrollY   int

	// Schema browser.
	schema      []querySchemaEntry
	schemaCur   int
	schemaLines []string // flattened renderable lines

	// Error.
	errMsg string

	// Return signal.
	done bool
}

// queryEditorSchema parses the schema text into a list of tables with columns.
func querySchemaParse(schemaText string) []querySchemaEntry {
	var tables []querySchemaEntry
	var current *querySchemaEntry

	for _, line := range strings.Split(schemaText, "\n") {
		line = strings.TrimSpace(line)

		// Detect CREATE TABLE.
		if strings.HasPrefix(strings.ToUpper(line), "CREATE TABLE") {
			// Extract table name: CREATE TABLE IF NOT EXISTS foo (
			name := line
			name = strings.ReplaceAll(name, "CREATE TABLE IF NOT EXISTS ", "")
			name = strings.ReplaceAll(name, "CREATE TABLE ", "")
			name = strings.TrimSpace(name)
			name = strings.TrimRight(name, "(")
			name = strings.TrimSpace(name)
			current = &querySchemaEntry{name: name}
			tables = append(tables, *current)
			// Point to the last element so we can modify it.
			current = &tables[len(tables)-1]
			continue
		}

		if current != nil {
			// Detect column definition: starts with a word, followed by a type.
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
				// Skip constraints like PRIMARY, NOT, DEFAULT, REFERENCES, UNIQUE.
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

// querySchemaLines flattens the schema entries into renderable lines.
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
	}

	// Forward to editor for cursor blink etc.
	if m.focus == queryFocusEditor {
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m QueryModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.done = true
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

	case "up", "k":
		if m.focus == queryFocusResults {
			if m.scrollY > 0 {
				m.scrollY--
			}
		} else if m.focus == queryFocusSchema {
			if m.schemaCur > 0 {
				m.schemaCur--
			}
		}
	case "down", "j":
		if m.focus == queryFocusResults {
			maxScroll := len(m.rows) - m.resultsVisibleRows()
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.scrollY < maxScroll {
				m.scrollY++
			}
		} else if m.focus == queryFocusSchema {
			if m.schemaCur < len(m.schemaLines)-1 {
				m.schemaCur++
			}
		}
	case "pgup":
		if m.focus == queryFocusResults {
			m.scrollY -= m.resultsVisibleRows()
			if m.scrollY < 0 {
				m.scrollY = 0
			}
		} else if m.focus == queryFocusSchema {
			m.schemaCur -= m.resultsVisibleRows()
			if m.schemaCur < 0 {
				m.schemaCur = 0
			}
		}
	case "pgdown":
		if m.focus == queryFocusResults {
			m.scrollY += m.resultsVisibleRows()
			maxScroll := len(m.rows) - m.resultsVisibleRows()
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.scrollY > maxScroll {
				m.scrollY = maxScroll
			}
		} else if m.focus == queryFocusSchema {
			m.schemaCur += m.resultsVisibleRows()
			if m.schemaCur >= len(m.schemaLines) {
				m.schemaCur = len(m.schemaLines) - 1
			}
		}
	case "c":
		if m.focus == queryFocusResults {
			m.columns = nil
			m.rows = nil
			m.resultMsg = ""
			m.phase = queryPhaseInput
			m.scrollY = 0
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

// executeQuery runs the SQL in the editor (generating it from NL if needed).
func (m QueryModel) executeQuery() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.editor.Value())
	if text == "" {
		return m, nil
	}

	var sql string
	if m.mode == queryModeNL {
		m.phase = queryPhaseLoading
		return m, m.generateSQLCmd(text)
	}

	// Raw mode: validate and execute directly.
	if err := services.ValidateReadOnly(text); err != nil {
		m.errMsg = err.Error()
		m.phase = queryPhaseError
		return m, nil
	}
	sql = text

	m.phase = queryPhaseLoading
	return m, m.executeSQLCmd(sql)
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
		ctx := context.Background()
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

func (m QueryModel) resultsVisibleRows() int {
	v := m.height - 12 // banner(1) + top divider(1) + editor area(~5) + footer(2) + padding(3)
	if v < 1 {
		v = 1
	}
	return v
}

func (m QueryModel) View() string {
	if m.width == 0 {
		return ""
	}

	var sb strings.Builder

	// Banner.
	sb.WriteString(renderQueryBanner(m.width))
	sb.WriteString("\n\n")

	// Top panes: results (left) + schema (right).
	topHeight := m.height - 10 // banner(1) + divider(1) + editor(~5) + footer(2) + padding(1)
	if topHeight < 3 {
		topHeight = 3
	}

	leftW := m.width * 2 / 3
	rightW := m.width - leftW - 1 // -1 for divider

	resultsPane := m.renderResultsPane(leftW, topHeight)
	schemaPane := m.renderSchemaPane(rightW, topHeight)

	// Join panes side by side.
	topPanes := lipgloss.JoinHorizontal(lipgloss.Top, resultsPane, schemaPane)
	sb.WriteString(topPanes)
	sb.WriteString("\n")

	// Divider.
	sb.WriteString(strings.Repeat("─", m.width-2))
	sb.WriteString("\n")

	// Editor pane.
	sb.WriteString(m.renderEditorPane(m.width, 5))
	sb.WriteString("\n")

	// Footer.
	sb.WriteString(renderQueryFooter(m.width))

	return sb.String()
}

func (m QueryModel) renderResultsPane(width, height int) string {
	var content string

	switch m.phase {
	case queryPhaseLoading:
		content = MutedStyle.Render("  Generating SQL...")
	case queryPhaseError:
		content = ErrorStyle.Render("  " + m.errMsg)
	case queryPhaseResults:
		if len(m.columns) == 0 {
			content = MutedStyle.Render("  No results.")
		} else {
			content = m.renderResultsTable(width - 4)
		}
	default:
		if m.editor.Value() == "" {
			content = MutedStyle.Render("  Press ctrl+e to execute a query.")
		} else {
			content = MutedStyle.Render("  Press ctrl+e to execute.")
		}
	}

	title := "Results"
	if len(m.rows) > 0 {
		title = "Results (" + itoa(len(m.rows)) + " rows, " + m.duration.Round(time.Microsecond).String() + ")"
	}

	borderColor := ColorBorder
	if m.focus == queryFocusResults {
		borderColor = ColorPrimary
	}

	return framePanel(content, width, height-2, title, "", "", borderColor, ColorPrimary)
}

func (m QueryModel) renderResultsTable(innerWidth int) string {
	if len(m.columns) == 0 || len(m.rows) == 0 {
		return ""
	}

	// Compute column widths.
	widths := make([]int, len(m.columns))
	for i, col := range m.columns {
		w := len(col)
		if w > 30 {
			w = 30
		}
		widths[i] = w
	}
	for _, row := range m.rows {
		for i, cell := range row {
			if i < len(widths) {
				w := len(cell)
				if w > widths[i] {
					if w > 30 {
						w = 30
					}
					widths[i] = w
				}
			}
		}
	}

	// Header.
	var sb strings.Builder
	for i, col := range m.columns {
		if i < len(widths) {
			sb.WriteString(truncate(col, widths[i]))
			sb.WriteString("  ")
		}
	}
	sb.WriteString("\n")

	// Separator.
	for _, w := range widths {
		sb.WriteString(strings.Repeat("─", w))
		sb.WriteString("  ")
	}
	sb.WriteString("\n")

	// Rows.
	maxRows := m.resultsVisibleRows() - 2
	if maxRows < 1 {
		maxRows = 1
	}
	end := m.scrollY + maxRows
	if end > len(m.rows) {
		end = len(m.rows)
	}
	for _, row := range m.rows[m.scrollY:end] {
		for i, cell := range row {
			if i < len(widths) {
				c := cell
				if len(c) > 30 {
					c = c[:27] + "..."
				}
				sb.WriteString(truncate(c, widths[i]))
				sb.WriteString("  ")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
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

	return framePanel(content, width, height-2, "Schema", "", "", borderColor, ColorPrimary)
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

func renderQueryFooter(width int) string {
	keys := []string{
		keyHint("ctrl+e", "execute"),
		keyHint("ctrl+n", "mode"),
		keyHint("tab", "focus"),
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

// RunQueryUI runs the interactive query screen and returns when the user exits.
func RunQueryUI(sqlDB *db.DB, client services.AIClient, model string) error {
	dialect := "sqlite"
	if sqlDB.Driver() == "postgres" {
		dialect = "postgres"
	}

	m := newQueryModel(sqlDB, client, model, dialect)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
