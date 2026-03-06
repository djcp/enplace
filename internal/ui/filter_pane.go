package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/gorecipes/internal/models"
)

// filterState holds all state for the shared filter pane.
// It is used by both ListModel and DetailModel.
type filterState struct {
	query           string
	focus           filterFocus
	courses         []string
	influences      []string
	status          string
	courseBuffer    string
	influenceBuffer string
	allCourses      []string
	allInfluences   []string

	// Saved state — restored on Esc.
	savedQuery      string
	savedCourses    []string
	savedInfluences []string
	savedStatus     string

	// active = the pane is currently open / the user is typing in it.
	active bool
}

// newFilterState constructs a filterState from an external FilterState and SearchData.
func newFilterState(initial FilterState, sd SearchData) filterState {
	return filterState{
		query:         initial.Query,
		courses:       initial.Courses,
		influences:    initial.Influences,
		status:        initial.Status,
		allCourses:    sd.Courses,
		allInfluences: sd.Influences,
	}
}

// toPublicFilter converts the filterState back to the external FilterState type.
func (fs filterState) toPublicFilter() FilterState {
	return FilterState{
		Query:      fs.query,
		Courses:    fs.courses,
		Influences: fs.influences,
		Status:     fs.status,
	}
}

// enter saves the current filter values and activates the pane.
func (fs filterState) enter() filterState {
	fs.savedQuery = fs.query
	fs.savedCourses = append([]string(nil), fs.courses...)
	fs.savedInfluences = append([]string(nil), fs.influences...)
	fs.savedStatus = fs.status
	fs.active = true
	return fs
}

// cancel restores the saved filter values and deactivates the pane.
func (fs filterState) cancel() filterState {
	fs.query = fs.savedQuery
	fs.courses = fs.savedCourses
	fs.influences = fs.savedInfluences
	fs.status = fs.savedStatus
	fs.savedQuery, fs.savedCourses, fs.savedInfluences, fs.savedStatus = "", nil, nil, ""
	fs.courseBuffer, fs.influenceBuffer = "", ""
	fs.active = false
	return fs
}

// hasActiveFilters returns true when any filter field is non-empty.
func (fs filterState) hasActiveFilters() bool {
	return fs.query != "" || len(fs.courses) > 0 || len(fs.influences) > 0 || fs.status != ""
}

// handleFilterKey processes a keypress in the filter pane.
// Returns the updated filterState and a confirmed bool.
// confirmed=true means the user pressed Enter/Search to apply the search.
func handleFilterKey(fs filterState, msg tea.KeyMsg) (filterState, bool) {
	switch msg.Type {
	case tea.KeyEsc:
		fs = fs.cancel()

	case tea.KeyEnter:
		if fs.focus == ffCourses && fs.courseBuffer != "" {
			fs.courses = append(fs.courses, resolveMatch(fs.courseBuffer, fs.allCourses))
			fs.courseBuffer = ""
			return fs, false
		}
		if fs.focus == ffInfluences && fs.influenceBuffer != "" {
			fs.influences = append(fs.influences, resolveMatch(fs.influenceBuffer, fs.allInfluences))
			fs.influenceBuffer = ""
			return fs, false
		}
		fs.active = false
		return fs, true

	case tea.KeyTab, tea.KeyDown:
		fs.courseBuffer, fs.influenceBuffer = "", ""
		fs.focus = (fs.focus + 1) % ffCount

	case tea.KeyShiftTab, tea.KeyUp:
		fs.courseBuffer, fs.influenceBuffer = "", ""
		fs.focus = (fs.focus - 1 + ffCount) % ffCount

	case tea.KeyLeft:
		if fs.focus == ffStatus {
			fs.status = prevStatus(fs.status)
		}

	case tea.KeyRight:
		if fs.focus == ffStatus {
			fs.status = nextStatus(fs.status)
		}

	case tea.KeyBackspace, tea.KeyDelete:
		switch fs.focus {
		case ffText:
			runes := []rune(fs.query)
			if len(runes) > 0 {
				fs.query = string(runes[:len(runes)-1])
			}
		case ffCourses:
			runes := []rune(fs.courseBuffer)
			if len(runes) > 0 {
				fs.courseBuffer = string(runes[:len(runes)-1])
			} else if len(fs.courses) > 0 {
				fs.courses = fs.courses[:len(fs.courses)-1]
			}
		case ffInfluences:
			runes := []rune(fs.influenceBuffer)
			if len(runes) > 0 {
				fs.influenceBuffer = string(runes[:len(runes)-1])
			} else if len(fs.influences) > 0 {
				fs.influences = fs.influences[:len(fs.influences)-1]
			}
		}

	default:
		var ch string
		if msg.Type == tea.KeyRunes {
			ch = string(msg.Runes)
		} else if msg.Type == tea.KeySpace {
			ch = " "
		}
		if ch != "" {
			switch fs.focus {
			case ffText:
				fs.query += ch
			case ffCourses:
				fs.courseBuffer += ch
			case ffInfluences:
				fs.influenceBuffer += ch
			}
		}
	}
	return fs, false
}

// renderFilterPane renders the filter pane with a left border separator.
// scrollHint is an optional string shown at the bottom (e.g. "5–18 of 42"); pass "" to omit.
func renderFilterPane(fs filterState, width, height int, scrollHint string) string {
	var sb strings.Builder

	titleText := " Filters"
	if fs.active {
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(titleText))
	} else if fs.hasActiveFilters() {
		sb.WriteString(lipgloss.NewStyle().Foreground(ColorPrimary).Render(titleText))
	} else {
		sb.WriteString(MutedStyle.Render(titleText))
	}
	sb.WriteString("\n")

	dividerW := width - 2
	if dividerW < 1 {
		dividerW = 1
	}
	sb.WriteString(MutedStyle.Render(" " + strings.Repeat("─", dividerW)))
	sb.WriteString("\n\n")

	sb.WriteString(renderFilterPaneSearch(fs.query, fs.active && fs.focus == ffText))
	sb.WriteString("\n\n")

	sb.WriteString(renderFilterPaneTagSection(
		"courses", models.TagContextCourses,
		fs.courses, fs.courseBuffer, fs.allCourses,
		fs.active && fs.focus == ffCourses,
	))
	sb.WriteString("\n\n")

	sb.WriteString(renderFilterPaneTagSection(
		"influences", models.TagContextCulturalInfluences,
		fs.influences, fs.influenceBuffer, fs.allInfluences,
		fs.active && fs.focus == ffInfluences,
	))
	sb.WriteString("\n\n")

	sb.WriteString(renderFilterPaneStatus(fs.status, fs.active && fs.focus == ffStatus))
	sb.WriteString("\n\n")

	if fs.active {
		sb.WriteString(MutedStyle.Render(" ↑↓/tab navigate · esc cancel"))
	} else {
		sb.WriteString(MutedStyle.Render(" → or / to filter"))
	}
	sb.WriteString("\n\n")

	sb.WriteString(renderFilterPaneSearchButton(fs.active && fs.focus == ffSearch))
	sb.WriteString("\n")

	if scrollHint != "" {
		sb.WriteString("\n")
		sb.WriteString(MutedStyle.Render(" " + scrollHint))
		sb.WriteString("\n")
	}

	content := sb.String()
	// Left border acts as the visual separator between the two panes.
	// Width(width-1) + left border (1 char) = total filterWidth.
	return lipgloss.NewStyle().
		Background(lipgloss.AdaptiveColor{Light: "#F5EDE6", Dark: "#211D1A"}).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(ColorSecondary).
		Height(height).
		Width(width - 1).
		Render(content)
}

// renderFilterPaneSearch renders the text-search row for the filter pane.
func renderFilterPaneSearch(query string, focused bool) string {
	prefix := MutedStyle.Render(" / ")
	cursor := lipgloss.NewStyle().
		Background(ColorPrimary).
		Foreground(lipgloss.Color("#FFFFFF")).
		Render(" ")

	var content string
	if focused {
		if query == "" {
			content = MutedStyle.Render("search...") + cursor
		} else {
			content = query + cursor
		}
	} else if query != "" {
		content = lipgloss.NewStyle().Foreground(ColorPrimary).Render(query)
	} else {
		content = MutedStyle.Render("search...")
	}
	return prefix + content
}

// renderFilterPaneTagSection renders a 2-line tag filter block (label + pills/input).
func renderFilterPaneTagSection(label, ctx string, pills []string, buffer string, suggestions []string, focused bool) string {
	var sb strings.Builder
	sb.WriteString(MutedStyle.Render(" " + label + ":"))
	sb.WriteString("\n ")

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
				sb.WriteString(MutedStyle.Render("any "))
			}
			sb.WriteString(cursor)
		}
	} else if len(pills) == 0 {
		sb.WriteString(MutedStyle.Render("any"))
	}

	return sb.String()
}

// renderFilterPaneStatus renders the status selector row for the filter pane.
func renderFilterPaneStatus(status string, focused bool) string {
	var sb strings.Builder
	sb.WriteString(MutedStyle.Render(" status: "))
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

// renderFilterPaneSearchButton renders the "search" action button at the bottom of the filter pane.
func renderFilterPaneSearchButton(focused bool) string {
	if focused {
		return " " + lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 2).
			Render("search")
	}
	return MutedStyle.Render(" [ search ]")
}
