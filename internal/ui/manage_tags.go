package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/enplace/internal/db"
	"github.com/djcp/enplace/internal/models"
)

type manageTagsPhase int

const (
	manageTagsPhaseContext      manageTagsPhase = iota // pick context
	manageTagsPhaseBrowse                              // browse tags in context
	manageTagsPhaseEdit                                // rename overlay
	manageTagsPhaseMerge                               // pick merge target
	manageTagsPhaseMergeConfirm                        // confirm merge
	manageTagsPhaseConfirm                             // confirm delete
	manageTagsPhaseResult                              // show result, any-key continues
)

// manageTagsModel is the TUI model for the tags management screen.
type manageTagsModel struct {
	sqlDB *db.DB

	phase manageTagsPhase

	// Context selection.
	contextCursor int

	// Tag browse.
	selectedContext string
	tags            []db.TagWithCount
	tagCursor       int
	tagOffset       int

	// Edit.
	editInput textinput.Model

	// Merge — secondary list.
	mergeList   []db.TagWithCount
	mergeCursor int

	// Confirm delete.
	confirmName  string
	confirmCount int

	// Merge confirm.
	mergeSourceName string
	mergeTargetName string
	mergeTargetID   int64
	mergeCount      int

	// Result.
	resultMsg    string
	resultErr    bool
	restoreTagID int64 // ID to seek to when returning to browse; 0 = clamp by cursor

	width  int
	height int
}

func newManageTagsModel(sqlDB *db.DB) manageTagsModel {
	return manageTagsModel{sqlDB: sqlDB, width: 80, height: 24}
}

var tagContextLabels = []struct{ key, label string }{
	{models.TagContextCourses, "Courses"},
	{models.TagContextCookingMethods, "Cooking Methods"},
	{models.TagContextCulturalInfluences, "Cultural Influences"},
	{models.TagContextDietaryRestrictions, "Dietary Restrictions"},
}

func (m manageTagsModel) Init() tea.Cmd { return textinput.Blink }

func (m manageTagsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	if m.phase == manageTagsPhaseEdit {
		var cmd tea.Cmd
		m.editInput, cmd = m.editInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m manageTagsModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.phase {
	case manageTagsPhaseContext:
		return m.handleContextKey(msg)
	case manageTagsPhaseBrowse:
		return m.handleBrowseKey(msg)
	case manageTagsPhaseEdit:
		return m.handleEditKey(msg)
	case manageTagsPhaseMerge:
		return m.handleMergeKey(msg)
	case manageTagsPhaseMergeConfirm:
		return m.handleMergeConfirmKey(msg)
	case manageTagsPhaseConfirm:
		return m.handleConfirmDeleteKey(msg)
	case manageTagsPhaseResult:
		prevCursor := m.tagCursor
		tags, _ := db.ListTagsByContext(m.sqlDB, m.selectedContext)
		m.tags = tags
		m.tagCursor, m.tagOffset = restoredCursorByID(m.restoreTagID, prevCursor, len(tags),
			func(i int) int64 { return tags[i].ID }, m.browseVisibleRows())
		m.restoreTagID = 0
		m.phase = manageTagsPhaseBrowse
		return m, nil
	}
	return m, nil
}

func (m manageTagsModel) handleContextKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.contextCursor > 0 {
			m.contextCursor--
		}
	case "down", "j":
		if m.contextCursor < len(tagContextLabels)-1 {
			m.contextCursor++
		}
	case "enter", " ":
		ctx := tagContextLabels[m.contextCursor].key
		tags, err := db.ListTagsByContext(m.sqlDB, ctx)
		if err != nil {
			m.resultMsg = "Error loading tags: " + err.Error()
			m.resultErr = true
			m.phase = manageTagsPhaseResult
			return m, nil
		}
		m.selectedContext = ctx
		m.tags = tags
		m.tagCursor = 0
		m.tagOffset = 0
		m.phase = manageTagsPhaseBrowse
	}
	return m, nil
}

func (m manageTagsModel) browseVisibleRows() int {
	// Banner(4) + header(2) + footer(2) = 8 overhead
	v := m.height - 8
	if v < 1 {
		v = 1
	}
	return v
}

func (m manageTagsModel) handleBrowseKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.phase = manageTagsPhaseContext
		return m, nil
	case "up", "k":
		if m.tagCursor > 0 {
			m.tagCursor--
			if m.tagCursor < m.tagOffset {
				m.tagOffset = m.tagCursor
			}
		}
	case "down", "j":
		if m.tagCursor < len(m.tags)-1 {
			m.tagCursor++
			visible := m.browseVisibleRows()
			if m.tagCursor >= m.tagOffset+visible {
				m.tagOffset = m.tagCursor - visible + 1
			}
		}
	case "e":
		if len(m.tags) == 0 {
			return m, nil
		}
		tag := m.tags[m.tagCursor]
		ti := textinput.New()
		ti.SetValue(tag.Name)
		ti.Width = m.width - 16
		ti.Focus()
		m.editInput = ti
		m.phase = manageTagsPhaseEdit
		return m, textinput.Blink
	case "m":
		if len(m.tags) < 2 {
			return m, nil
		}
		// Build merge list excluding current tag.
		source := m.tags[m.tagCursor]
		mergeList := make([]db.TagWithCount, 0, len(m.tags)-1)
		for _, t := range m.tags {
			if t.ID != source.ID {
				mergeList = append(mergeList, t)
			}
		}
		m.mergeList = mergeList
		m.mergeCursor = 0
		m.mergeSourceName = source.Name
		m.phase = manageTagsPhaseMerge
	case "d":
		if len(m.tags) == 0 {
			return m, nil
		}
		tag := m.tags[m.tagCursor]
		m.confirmName = tag.Name
		m.confirmCount = tag.Count
		m.phase = manageTagsPhaseConfirm
	}
	return m, nil
}

func (m manageTagsModel) handleEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.phase = manageTagsPhaseBrowse
		return m, nil
	case "enter":
		newName := strings.TrimSpace(m.editInput.Value())
		if newName == "" {
			m.phase = manageTagsPhaseBrowse
			return m, nil
		}
		tag := m.tags[m.tagCursor]
		if err := db.RenameTag(m.sqlDB, tag.ID, newName); err != nil {
			m.resultMsg = "Error renaming tag: " + err.Error()
			m.resultErr = true
		} else {
			m.resultMsg = fmt.Sprintf("Renamed '%s' → '%s'", tag.Name, newName)
			m.resultErr = false
			m.restoreTagID = tag.ID
		}
		m.phase = manageTagsPhaseResult
		return m, nil
	}
	var cmd tea.Cmd
	m.editInput, cmd = m.editInput.Update(msg)
	return m, cmd
}

func (m manageTagsModel) handleMergeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.phase = manageTagsPhaseBrowse
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
		m.mergeCount = m.tags[m.tagCursor].Count
		m.phase = manageTagsPhaseMergeConfirm
	}
	return m, nil
}

func (m manageTagsModel) handleMergeConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "n":
		m.phase = manageTagsPhaseMerge
		return m, nil
	case "y", "enter":
		source := m.tags[m.tagCursor]
		if err := db.MergeTag(m.sqlDB, source.ID, m.mergeTargetID); err != nil {
			m.resultMsg = "Error merging tag: " + err.Error()
			m.resultErr = true
		} else {
			m.resultMsg = fmt.Sprintf("Merged '%s' into '%s'", m.mergeSourceName, m.mergeTargetName)
			m.resultErr = false
			m.restoreTagID = m.mergeTargetID
		}
		m.phase = manageTagsPhaseResult
	}
	return m, nil
}

func (m manageTagsModel) handleConfirmDeleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "n":
		m.phase = manageTagsPhaseBrowse
		return m, nil
	case "y", "enter":
		tag := m.tags[m.tagCursor]
		if err := db.DeleteTag(m.sqlDB, tag.ID); err != nil {
			m.resultMsg = "Error deleting tag: " + err.Error()
			m.resultErr = true
		} else {
			m.resultMsg = fmt.Sprintf("Deleted tag '%s'", tag.Name)
			m.resultErr = false
		}
		m.phase = manageTagsPhaseResult
	}
	return m, nil
}

func (m manageTagsModel) View() string {
	if m.width == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(renderManageBanner("tags", m.width))
	sb.WriteString("\n")

	switch m.phase {
	case manageTagsPhaseContext:
		sb.WriteString(m.viewContextSelect())
	case manageTagsPhaseBrowse:
		sb.WriteString(m.viewBrowse())
	case manageTagsPhaseEdit:
		sb.WriteString(m.viewEdit())
	case manageTagsPhaseMerge:
		sb.WriteString(m.viewMerge())
	case manageTagsPhaseMergeConfirm:
		sb.WriteString(m.viewMergeConfirm())
	case manageTagsPhaseConfirm:
		sb.WriteString(m.viewConfirmDelete())
	case manageTagsPhaseResult:
		sb.WriteString(m.viewResult())
	}

	return sb.String()
}

func (m manageTagsModel) viewContextSelect() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(MutedStyle.Render("  Select a tag context:"))
	sb.WriteString("\n\n")

	for i, ctx := range tagContextLabels {
		selected := i == m.contextCursor
		if selected {
			sb.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				Render("  ▶ " + ctx.label))
		} else {
			sb.WriteString(MutedStyle.Render("    " + ctx.label))
		}
		sb.WriteString("\n")
	}

	used := strings.Count(sb.String(), "\n")
	if fill := m.height - used - 4; fill > 0 {
		sb.WriteString(strings.Repeat("\n", fill))
	}
	sb.WriteString("\n")
	sb.WriteString(renderManageFooter([]string{"↑/↓ select", "enter open", "esc back"}, m.width))
	return sb.String()
}

func (m manageTagsModel) viewBrowse() string {
	var sb strings.Builder

	ctxLabel := tagContextLabels[m.contextCursor].label
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Render(fmt.Sprintf("  %s  (%d tags)", ctxLabel, len(m.tags)))
	sb.WriteString("\n")
	sb.WriteString(header)
	sb.WriteString("\n\n")

	visible := m.browseVisibleRows() - 3 // account for header lines
	if visible < 1 {
		visible = 1
	}
	end := m.tagOffset + visible
	if end > len(m.tags) {
		end = len(m.tags)
	}

	if len(m.tags) == 0 {
		sb.WriteString(MutedStyle.Render("  No tags in this context."))
		sb.WriteString("\n")
	} else {
		for i := m.tagOffset; i < end; i++ {
			t := m.tags[i]
			selected := i == m.tagCursor
			count := fmt.Sprintf("%d recipe", t.Count)
			if t.Count != 1 {
				count += "s"
			}
			row := fmt.Sprintf("  %-30s  %s", t.Name, MutedStyle.Render(count))
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
	sb.WriteString(renderManageFooter([]string{"↑/↓ navigate", "e edit", "m merge", "d delete", "esc back"}, m.width))
	return sb.String()
}

func (m manageTagsModel) viewEdit() string {
	var sb strings.Builder
	tag := m.tags[m.tagCursor]

	sb.WriteString("\n")
	sb.WriteString(MutedStyle.Render(fmt.Sprintf("  Rename tag '%s':", tag.Name)))
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

func (m manageTagsModel) viewMerge() string {
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
		t := m.mergeList[i]
		selected := i == m.mergeCursor
		count := fmt.Sprintf("%d recipe", t.Count)
		if t.Count != 1 {
			count += "s"
		}
		row := fmt.Sprintf("  %-30s  %s", t.Name, MutedStyle.Render(count))
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

func (m manageTagsModel) viewMergeConfirm() string {
	return buildCenteredBox(
		"Merge tags?", ColorWarning, ColorWarning,
		[]string{
			MutedStyle.Render(fmt.Sprintf("Merge '%s' into '%s'?", m.mergeSourceName, m.mergeTargetName)),
			MutedStyle.Render(fmt.Sprintf("%d recipe(s) will be updated.", m.mergeCount)),
			"",
			MutedStyle.Render("The source tag will be deleted."),
		},
		m.width, m.height,
		renderManageConfirmFooter("y confirm", ColorWarning, m.width),
	)
}

func (m manageTagsModel) viewConfirmDelete() string {
	count := fmt.Sprintf("Used by %d recipe", m.confirmCount)
	if m.confirmCount != 1 {
		count += "s"
	}
	count += "."
	return buildCenteredBox(
		"Delete tag?", ColorError, ColorError,
		[]string{
			lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("'" + m.confirmName + "'"),
			"",
			MutedStyle.Render(count),
			MutedStyle.Render("This cannot be undone."),
		},
		m.width, m.height,
		renderManageConfirmFooter("y confirm", ColorError, m.width),
	)
}

func (m manageTagsModel) viewResult() string {
	return viewManageResult(
		m.resultMsg, m.resultErr,
		m.width, m.height,
		renderManageFooter([]string{"any key continue"}, m.width),
	)
}

// RunManageTagsUI runs the tags management TUI.
func RunManageTagsUI(sqlDB *db.DB) error {
	m := newManageTagsModel(sqlDB)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
