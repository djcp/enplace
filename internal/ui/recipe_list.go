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
	recipes  []models.Recipe
	filtered []models.Recipe
	cursor   int
	query    string
	typing   bool
	width    int
	height   int
	offset   int // scroll offset

	// Set to > 0 when the user pressed Enter to view a recipe.
	selectedID int64
	quitting   bool
}

// NewListModel creates a ListModel from a slice of recipes.
func NewListModel(recipes []models.Recipe) ListModel {
	m := ListModel{
		recipes:  recipes,
		filtered: recipes,
		width:    80,
		height:   24,
	}
	return m
}

// SelectedID returns the recipe ID the user selected (0 if none).
func (m ListModel) SelectedID() int64 { return m.selectedID }

func (m ListModel) Init() tea.Cmd { return nil }

func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
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
		m.filtered = m.recipes
		m.cursor = 0
		m.offset = 0
	case tea.KeyEnter:
		m.typing = false
	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.query) > 0 {
			m.query = m.query[:len(m.query)-1]
			m.applyFilter()
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.query += string(msg.Runes)
			m.applyFilter()
		}
	}
	return m, nil
}

func (m ListModel) handleNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit
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
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			visible := m.visibleRows()
			if m.cursor >= m.offset+visible {
				m.offset = m.cursor - visible + 1
			}
		}
	case "enter", " ":
		if len(m.filtered) > 0 {
			m.selectedID = m.filtered[m.cursor].ID
			return m, tea.Quit
		}
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *ListModel) applyFilter() {
	if m.query == "" {
		m.filtered = m.recipes
	} else {
		q := strings.ToLower(m.query)
		var out []models.Recipe
		for _, r := range m.recipes {
			if strings.Contains(strings.ToLower(r.Name), q) {
				out = append(out, r)
				continue
			}
			// Search ingredients.
			for _, ing := range r.Ingredients {
				if strings.Contains(strings.ToLower(ing.IngredientName), q) {
					out = append(out, r)
					break
				}
			}
		}
		m.filtered = out
	}
	m.cursor = 0
	m.offset = 0
}

func (m ListModel) visibleRows() int {
	// Header (3) + search bar (2) + footer (2) = 7 overhead lines.
	v := m.height - 9
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

	// Search bar.
	sb.WriteString(renderSearchBar(m.query, m.typing, m.width))
	sb.WriteString("\n\n")

	if len(m.filtered) == 0 {
		sb.WriteString(MutedStyle.Render("  No recipes found."))
		sb.WriteString("\n")
	} else {
		visible := m.visibleRows()
		end := m.offset + visible
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for i := m.offset; i < end; i++ {
			r := m.filtered[i]
			selected := i == m.cursor
			sb.WriteString(renderRecipeRow(r, selected, m.width))
			sb.WriteString("\n")
		}

		// Scroll hint.
		if len(m.filtered) > visible {
			total := len(m.filtered)
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
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(1, 2).
		Render("🍳  gorecipes")

	border := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(title)

	return border
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
		"↑/↓ navigate",
		"/ search",
		"enter view",
		"q quit",
	}
	line := "  " + strings.Join(keys, "   ")
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(line)
}

func truncate(s string, max int) string {
	if len([]rune(s)) <= max {
		return s
	}
	runes := []rune(s)
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

// RunListUI runs the interactive recipe browser and returns the selected recipe ID (or 0).
func RunListUI(recipes []models.Recipe) (int64, error) {
	m := NewListModel(recipes)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return 0, err
	}
	finalModel := final.(ListModel)
	return finalModel.SelectedID(), nil
}
