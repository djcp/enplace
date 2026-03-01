package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/gorecipes/internal/models"
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
// initialQuery pre-fills the search bar with an active filter when the list re-opens.
func NewListModel(recipes []models.Recipe, initialQuery string) ListModel {
	m := ListModel{
		recipes: recipes,
		width:   80,
		height:  24,
		query:   initialQuery,
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

// Query returns the current search query.
func (m ListModel) Query() string { return m.query }

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

func (m ListModel) handleTypingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.typing = false
		m.query = ""
		m.cursor = 0
		m.offset = 0
	case tea.KeyEnter:
		m.typing = false
		m.searchConfirmed = true
		return m, tea.Quit
	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.query) > 0 {
			m.query = m.query[:len(m.query)-1]
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.query += string(msg.Runes)
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
		if m.query != "" {
			// Clear active filter and return to the full list.
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
		m.typing = true
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
	// Banner (4) + blank-before-footer (1) + footer (2) = 7 fixed overhead.
	// When typing the bordered search bar adds 4 more lines (3-line box + 1 blank).
	v := m.height - 7
	if m.typing {
		v -= 4
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
	if len(m.recipes) == 0 && m.query == "" && !m.typing {
		sb.WriteString(m.viewEmpty())
		used := strings.Count(sb.String(), "\n")
		if fill := m.height - used - 3; fill > 0 {
			sb.WriteString(strings.Repeat("\n", fill))
		}
		sb.WriteString("\n")
		sb.WriteString(renderFooter(m.width))
		return sb.String()
	}

	// Search bar — only visible while the user is actively typing.
	if m.typing {
		sb.WriteString(renderSearchBar(m.query, m.typing, m.width))
		sb.WriteString("\n\n")
	}

	visible := m.visibleRows()
	if len(m.recipes) == 0 {
		// A filter/search was active but returned nothing.
		sb.WriteString(MutedStyle.Render(fmt.Sprintf(`  No recipes match "%s".`, m.query)))
		sb.WriteString("\n")
		for i := 1; i < visible; i++ {
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

func renderSearchBar(query string, typing bool, width int) string {
	prefix := MutedStyle.Render("  / ")
	var content string
	if typing {
		content = query + lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(lipgloss.Color("#FFFFFF")).
			Render(" ")
	} else if query != "" {
		content = lipgloss.NewStyle().Foreground(ColorPrimary).Render(query)
	} else {
		content = MutedStyle.Render("search recipes...")
	}

	bar := lipgloss.NewStyle().
		Width(width-2).
		Render(prefix + content)

	if typing {
		bar = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorPrimary).
			Width(width - 4).
			Padding(0, 1).
			Render(prefix + content)
	}

	return bar
}

func renderRecipeRow(r models.Recipe, selected bool, width int) string {
	nameWidth := width - 40
	if nameWidth < 20 {
		nameWidth = 20
	}

	name := truncate(r.Name, nameWidth)
	courses := truncate(strings.Join(r.TagsByContext(models.TagContextCourses), ", "), 14)
	methods := truncate(strings.Join(r.TagsByContext(models.TagContextCookingMethods), ", "), 12)
	status := StatusBadge(r.Status)

	dateStr := r.CreatedAt.Format("Jan 2")

	row := fmt.Sprintf("  %-*s  %-14s  %-12s  %-6s  %s",
		nameWidth, name, courses, methods, dateStr, status)

	if selected {
		return HighlightStyle.Width(width).Render(row)
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
// Returns the selected recipe ID (or 0), navigation signals, the search query,
// the recipe ID confirmed for deletion (or 0), the recipe ID to edit (or 0),
// whether the user pressed "m" to open manage, and any error.
func RunListUI(recipes []models.Recipe, initialQuery string) (selectedID int64, goAdd bool, goHome bool, searchConfirmed bool, searchQuery string, deleteID int64, editID int64, goManage bool, err error) {
	m := NewListModel(recipes, initialQuery)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, runErr := p.Run()
	if runErr != nil {
		return 0, false, false, false, "", 0, 0, false, runErr
	}
	fm := final.(ListModel)
	return fm.SelectedID(), fm.GoAdd(), fm.GoHome(), fm.SearchConfirmed(), fm.Query(), fm.DeleteTargetID(), fm.EditID(), fm.GoManage(), nil
}
