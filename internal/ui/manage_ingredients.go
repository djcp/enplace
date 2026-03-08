package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/enplace/internal/db"
	"github.com/jmoiron/sqlx"
)

type manageIngPhase int

const (
	manageIngPhaseBrowse manageIngPhase = iota
	manageIngPhaseEdit
	manageIngPhaseMerge
	manageIngPhaseMergeConfirm
	manageIngPhaseResult
)

// manageIngredientsModel is the TUI model for ingredient management.
type manageIngredientsModel struct {
	sqlDB *sqlx.DB

	phase manageIngPhase

	// Displayed list (already filtered by current search term via SQL).
	filtered []db.IngredientWithCount
	cursor         int
	offset         int

	// Search.
	searchInput textinput.Model
	searching   bool

	// Edit.
	editInput textinput.Model

	// Merge.
	mergeList       []db.IngredientWithCount
	mergeCursor     int
	mergeSourceName string
	mergeTargetName string
	mergeTargetID   int64

	// Result.
	resultMsg    string
	resultErr    bool
	restoreIngID int64 // ID to seek to when returning to browse; 0 = clamp by cursor

	width  int
	height int
}

func newManageIngredientsModel(sqlDB *sqlx.DB) manageIngredientsModel {
	si := textinput.New()
	si.Placeholder = "search ingredients..."
	si.Width = 30

	return manageIngredientsModel{
		sqlDB:       sqlDB,
		searchInput: si,
		width:       80,
		height:      24,
	}
}

func (m *manageIngredientsModel) loadIngredients() error {
	search := strings.TrimSpace(m.searchInput.Value())
	ings, err := db.ListIngredientsWithCount(m.sqlDB, search)
	if err != nil {
		return err
	}
	m.filtered = ings
	return nil
}

func (m manageIngredientsModel) Init() tea.Cmd { return textinput.Blink }

func (m manageIngredientsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	if m.searching || m.phase == manageIngPhaseEdit {
		var cmd tea.Cmd
		if m.searching {
			prev := m.searchInput.Value()
			m.searchInput, cmd = m.searchInput.Update(msg)
			if m.searchInput.Value() != prev {
				_ = m.loadIngredients()
				m.cursor = 0
				m.offset = 0
			}
		} else {
			m.editInput, cmd = m.editInput.Update(msg)
		}
		return m, cmd
	}
	return m, nil
}

func (m manageIngredientsModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.phase {
	case manageIngPhaseBrowse:
		return m.handleBrowseKey(msg)
	case manageIngPhaseEdit:
		return m.handleEditKey(msg)
	case manageIngPhaseMerge:
		return m.handleMergeKey(msg)
	case manageIngPhaseMergeConfirm:
		return m.handleMergeConfirmKey(msg)
	case manageIngPhaseResult:
		prevCursor := m.cursor
		if err := m.loadIngredients(); err != nil {
			m.resultMsg = "Error reloading: " + err.Error()
			m.resultErr = true
			return m, nil
		}
		m.cursor, m.offset = restoredCursorByID(m.restoreIngID, prevCursor, len(m.filtered),
			func(i int) int64 { return m.filtered[i].ID }, m.visibleRows())
		m.restoreIngID = 0
		m.phase = manageIngPhaseBrowse
		return m, nil
	}
	return m, nil
}

func (m manageIngredientsModel) visibleRows() int {
	// Banner(4) + search(2) + header(1) + footer(2) = 9
	v := m.height - 9
	if v < 1 {
		v = 1
	}
	return v
}

func (m manageIngredientsModel) handleBrowseKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.searching {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.searching = false
			m.searchInput.Blur()
			m.searchInput.SetValue("")
			_ = m.loadIngredients()
			m.cursor = 0
			m.offset = 0
			return m, nil
		case "enter":
			m.searching = false
			m.searchInput.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		prev := m.searchInput.Value()
		m.searchInput, cmd = m.searchInput.Update(msg)
		if m.searchInput.Value() != prev {
			_ = m.loadIngredients()
			m.cursor = 0
			m.offset = 0
		}
		return m, cmd
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		return m, tea.Quit
	case "/":
		m.searching = true
		m.searchInput.Focus()
		return m, textinput.Blink
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}
	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			visible := m.visibleRows()
			if m.cursor >= m.offset+visible {
				m.offset = m.cursor - visible + 1
			}
		}
	case "e":
		if len(m.filtered) == 0 {
			return m, nil
		}
		ing := m.filtered[m.cursor]
		ti := textinput.New()
		ti.SetValue(ing.Name)
		ti.Width = m.width - 16
		ti.Focus()
		m.editInput = ti
		m.phase = manageIngPhaseEdit
		return m, textinput.Blink
	case "m":
		if len(m.filtered) < 2 {
			return m, nil
		}
		source := m.filtered[m.cursor]
		mergeList := make([]db.IngredientWithCount, 0, len(m.filtered)-1)
		for _, ing := range m.filtered {
			if ing.ID != source.ID {
				mergeList = append(mergeList, ing)
			}
		}
		m.mergeList = mergeList
		m.mergeCursor = 0
		m.mergeSourceName = source.Name
		m.phase = manageIngPhaseMerge
	}
	return m, nil
}

func (m manageIngredientsModel) handleEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.phase = manageIngPhaseBrowse
		return m, nil
	case "enter":
		newName := strings.TrimSpace(m.editInput.Value())
		if newName == "" {
			m.phase = manageIngPhaseBrowse
			return m, nil
		}
		ing := m.filtered[m.cursor]
		if err := db.RenameIngredient(m.sqlDB, ing.ID, newName); err != nil {
			m.resultMsg = "Error renaming: " + err.Error()
			m.resultErr = true
		} else {
			m.resultMsg = fmt.Sprintf("Renamed '%s' → '%s'", ing.Name, newName)
			m.resultErr = false
			m.restoreIngID = ing.ID
		}
		m.phase = manageIngPhaseResult
		return m, nil
	}
	var cmd tea.Cmd
	m.editInput, cmd = m.editInput.Update(msg)
	return m, cmd
}

func (m manageIngredientsModel) handleMergeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.phase = manageIngPhaseBrowse
		return m, nil
	case "up", "k":
		if m.mergeCursor > 0 {
			m.mergeCursor--
		}
	case "down", "j":
		if m.mergeCursor < len(m.mergeList)-1 {
			m.mergeCursor++
		}
	case "enter", " ":
		target := m.mergeList[m.mergeCursor]
		m.mergeTargetName = target.Name
		m.mergeTargetID = target.ID
		m.phase = manageIngPhaseMergeConfirm
	}
	return m, nil
}

func (m manageIngredientsModel) handleMergeConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "n":
		m.phase = manageIngPhaseMerge
		return m, nil
	case "y", "enter":
		source := m.filtered[m.cursor]
		if err := db.MergeIngredient(m.sqlDB, source.ID, m.mergeTargetID); err != nil {
			m.resultMsg = "Error merging: " + err.Error()
			m.resultErr = true
		} else {
			m.resultMsg = fmt.Sprintf("Merged '%s' into '%s'", m.mergeSourceName, m.mergeTargetName)
			m.resultErr = false
			m.restoreIngID = m.mergeTargetID
		}
		m.phase = manageIngPhaseResult
	}
	return m, nil
}

func (m manageIngredientsModel) View() string {
	if m.width == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(renderManageBanner("ingredients", m.width))
	sb.WriteString("\n")

	switch m.phase {
	case manageIngPhaseBrowse:
		sb.WriteString(m.viewBrowse())
	case manageIngPhaseEdit:
		sb.WriteString(m.viewEdit())
	case manageIngPhaseMerge:
		sb.WriteString(m.viewMerge())
	case manageIngPhaseMergeConfirm:
		sb.WriteString(m.viewMergeConfirm())
	case manageIngPhaseResult:
		sb.WriteString(m.viewResult())
	}

	return sb.String()
}

func (m manageIngredientsModel) viewBrowse() string {
	var sb strings.Builder

	// Search bar.
	searchBorder := ColorBorder
	if m.searching {
		searchBorder = ColorPrimary
	}
	searchBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(searchBorder).
		Padding(0, 1).
		MarginLeft(2).
		Render(MutedStyle.Render("/ ") + m.searchInput.View())
	sb.WriteString("\n")
	sb.WriteString(searchBox)
	sb.WriteString("\n\n")

	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	if len(m.filtered) == 0 {
		sb.WriteString(MutedStyle.Render("  No ingredients found."))
		sb.WriteString("\n")
	} else {
		for i := m.offset; i < end; i++ {
			ing := m.filtered[i]
			selected := i == m.cursor
			count := fmt.Sprintf("%d", ing.Count)
			row := fmt.Sprintf("  %-35s  %s", truncate(ing.Name, 35), MutedStyle.Render(count))
			if selected {
				sb.WriteString(HighlightStyle.Width(m.width - 2).Render(row))
			} else {
				sb.WriteString(row)
			}
			sb.WriteString("\n")
		}
	}

	used := strings.Count(sb.String(), "\n")
	if fill := m.height - used - 3; fill > 0 {
		sb.WriteString(strings.Repeat("\n", fill))
	}
	sb.WriteString("\n")
	sb.WriteString(renderManageFooter([]string{"↑/↓ navigate", "/ search", "e edit", "m merge", "esc back"}, m.width))
	return sb.String()
}

func (m manageIngredientsModel) viewEdit() string {
	var sb strings.Builder
	ing := m.filtered[m.cursor]
	sb.WriteString("\n")
	sb.WriteString(MutedStyle.Render(fmt.Sprintf("  Rename ingredient '%s':", ing.Name)))
	sb.WriteString("\n\n")
	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1).
		MarginLeft(4).
		Render(m.editInput.View())
	sb.WriteString(box)
	sb.WriteString("\n")
	used := strings.Count(sb.String(), "\n")
	if fill := m.height - used - 3; fill > 0 {
		sb.WriteString(strings.Repeat("\n", fill))
	}
	sb.WriteString("\n")
	sb.WriteString(renderManageFooter([]string{"enter save", "esc cancel"}, m.width))
	return sb.String()
}

func (m manageIngredientsModel) viewMerge() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(MutedStyle.Render(fmt.Sprintf("  Merge '%s' into…", m.mergeSourceName)))
	sb.WriteString("\n\n")

	visible := m.height - 8
	if visible < 1 {
		visible = 1
	}
	offset := 0
	if m.mergeCursor >= visible {
		offset = m.mergeCursor - visible + 1
	}
	end := offset + visible
	if end > len(m.mergeList) {
		end = len(m.mergeList)
	}

	for i := offset; i < end; i++ {
		ing := m.mergeList[i]
		selected := i == m.mergeCursor
		count := fmt.Sprintf("%d", ing.Count)
		row := fmt.Sprintf("  %-35s  %s", truncate(ing.Name, 35), MutedStyle.Render(count))
		if selected {
			sb.WriteString(HighlightStyle.Width(m.width - 2).Render(row))
		} else {
			sb.WriteString(row)
		}
		sb.WriteString("\n")
	}

	used := strings.Count(sb.String(), "\n")
	if fill := m.height - used - 3; fill > 0 {
		sb.WriteString(strings.Repeat("\n", fill))
	}
	sb.WriteString("\n")
	sb.WriteString(renderManageFooter([]string{"↑/↓ select target", "enter confirm", "esc cancel"}, m.width))
	return sb.String()
}

func (m manageIngredientsModel) viewMergeConfirm() string {
	return buildCenteredBox(
		"Merge ingredients?", ColorWarning, ColorWarning,
		[]string{
			MutedStyle.Render(fmt.Sprintf("Merge '%s' into '%s'?", m.mergeSourceName, m.mergeTargetName)),
			MutedStyle.Render("The source ingredient will be deleted."),
		},
		m.width, m.height,
		renderManageConfirmFooter("y confirm", ColorWarning, m.width),
	)
}

func (m manageIngredientsModel) viewResult() string {
	return viewManageResult(
		m.resultMsg, m.resultErr,
		m.width, m.height,
		renderManageFooter([]string{"any key continue"}, m.width),
	)
}

// RunManageIngredientsUI runs the ingredients management TUI.
func RunManageIngredientsUI(sqlDB *sqlx.DB) error {
	m := newManageIngredientsModel(sqlDB)
	if err := m.loadIngredients(); err != nil {
		return err
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
