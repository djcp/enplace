package ui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/enplace/internal/models"
	"github.com/djcp/enplace/internal/scaling"
	"github.com/muesli/termenv"
)

var (
	glamourStyleOnce sync.Once
	glamourStyleName string
)

// detectedGlamourStyle queries the terminal background colour exactly once and
// returns the matching glamour style name ("dark" or "light"). Subsequent calls
// return the cached result instantly, avoiding the OSC round-trip on every render.
func detectedGlamourStyle() string {
	glamourStyleOnce.Do(func() {
		if termenv.HasDarkBackground() {
			glamourStyleName = "dark"
		} else {
			glamourStyleName = "light"
		}
	})
	return glamourStyleName
}

type detailFocus int

const (
	detailFocusContent detailFocus = iota
	detailFocusHeader              // search bar active
)

// DetailModel is a full-screen interactive recipe detail viewer that mirrors
// the visual structure of ListModel: banner, scrollable content, footer.
// When the user presses "/" the right pane opens with the same filter panel as the list view.
type DetailModel struct {
	recipe *models.Recipe
	lines  []string // pre-rendered content split into terminal lines
	scroll int
	width  int
	height int

	focus  detailFocus
	filter filterState // search/filter pane state (shared with list view)

	// Rating overlay — huh.Select form launched on r.
	ratingMode    bool
	ratingPending *int // heap-allocated int shared with the form binding
	ratingForm    *huh.Form

	// Notes overlay — press N to open a full textarea editor.
	notesMode  bool
	notesInput textarea.Model

	// Return signals for the caller to persist.
	updateRating bool
	newRating    *int
	updateNotes  bool
	newNotes     string

	goHome           bool
	goAdd            bool
	goEdit           bool
	goPrint          bool
	goScale          bool
	goManage         bool
	goRetry          bool
	confirmingDelete bool
	deleteConfirmed  bool
}

// NewDetailModel creates a DetailModel for the given recipe.
// initial carries any active filter from the calling context (e.g. list view).
// sd provides autocomplete suggestions for the filter pane.
// It detects the terminal background colour and pre-renders content before
// the TUI starts so the first frame and any resize redraws are instant.
func NewDetailModel(recipe *models.Recipe, initial FilterState, sd SearchData) DetailModel {
	detectedGlamourStyle() // warm up the cache before entering the event loop
	ni := textarea.New()
	ni.ShowLineNumbers = false
	ni.SetHeight(8)
	ni.Placeholder = "Personal notes..."
	m := DetailModel{
		recipe:     recipe,
		width:      80,
		height:     24,
		filter:     newFilterState(initial, sd),
		notesInput: ni,
	}
	m.lines = m.buildLines()
	return m
}

// GoHome returns true when the user selected "home".
func (m DetailModel) GoHome() bool { return m.goHome }

// GoAdd returns true when the user pressed "a" to add a new recipe.
func (m DetailModel) GoAdd() bool { return m.goAdd }

// GoEdit returns true when the user pressed "e" to edit the recipe.
func (m DetailModel) GoEdit() bool { return m.goEdit }

// GoPrint returns true when the user pressed "p" to open print preview.
func (m DetailModel) GoPrint() bool { return m.goPrint }

// GoScale returns true when the user pressed "s" to open the scale screen.
func (m DetailModel) GoScale() bool { return m.goScale }

// GoManage returns true when the user pressed "m" to open the manage screen.
func (m DetailModel) GoManage() bool { return m.goManage }

// GoRetry returns true when the user pressed "r" to retry a failed extraction.
func (m DetailModel) GoRetry() bool { return m.goRetry }

// DeleteConfirmed returns true when the user confirmed deletion of the recipe.
func (m DetailModel) DeleteConfirmed() bool { return m.deleteConfirmed }

// UpdateRating returns true when the user set or cleared a rating.
func (m DetailModel) UpdateRating() bool { return m.updateRating }

// NewRating returns the pending rating value (nil = clear).
func (m DetailModel) NewRating() *int { return m.newRating }

// UpdateNotes returns true when the user saved new notes text.
func (m DetailModel) UpdateNotes() bool { return m.updateNotes }

// NewNotes returns the saved notes text.
func (m DetailModel) NewNotes() string { return m.newNotes }

// ReturnFilter returns the filter state the user had when leaving (for passing back to the list).
func (m DetailModel) ReturnFilter() FilterState { return m.filter.toPublicFilter() }

func (m DetailModel) Init() tea.Cmd { return nil }

// viewportHeight returns the number of content lines visible in the viewport.
// Overhead: banner rule (1) + blank-before-footer (1) + footer (2) = 4.
func (m DetailModel) viewportHeight() int {
	v := m.height - 4
	if v < 1 {
		v = 1
	}
	return v
}

func (m DetailModel) maxScroll() int {
	ms := len(m.lines) - m.viewportHeight()
	if ms < 0 {
		return 0
	}
	return ms
}

func (m DetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Always track terminal size.
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		m.height = ws.Height
		m.lines = m.buildLines()
		if m.scroll > m.maxScroll() {
			m.scroll = m.maxScroll()
		}
		if m.notesMode {
			m.notesInput.SetWidth(m.width - 12)
		}
	}

	// Rating overlay: all messages are delegated to the huh form.
	if m.ratingMode {
		return m.handleRatingMsg(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, nil // already handled above

	case tea.KeyMsg:
		if m.confirmingDelete {
			return m.handleConfirmKey(msg)
		}
		if m.notesMode {
			return m.handleNotesKey(msg)
		}
		// All keypresses when header has focus are routed to the search handler.
		if m.focus == detailFocusHeader {
			return m.handleHeaderKey(msg)
		}
		return m.handleNavKey(msg)
	}

	// Forward non-key messages to the textarea when in notes mode (e.g. blink).
	if m.notesMode {
		var cmd tea.Cmd
		m.notesInput, cmd = m.notesInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleConfirmKey processes keys while the delete confirmation overlay is shown.
func (m DetailModel) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		m.deleteConfirmed = true
		return m, tea.Quit
	case "n", "esc", "ctrl+c":
		m.confirmingDelete = false
	}
	return m, nil
}

// handleHeaderKey processes keys while the filter pane has focus.
func (m DetailModel) handleHeaderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	wasActive := m.filter.active
	fs, confirmed := handleFilterKey(m.filter, msg)
	m.filter = fs

	if confirmed {
		// User pressed Enter/Search — go home with the selected filters.
		m.goHome = true
		m.focus = detailFocusContent
		m.lines = m.buildLines()
		return m, tea.Quit
	}

	if wasActive && !fs.active {
		// Esc was pressed — close the filter pane and restore full-width content.
		m.focus = detailFocusContent
		m.lines = m.buildLines()
		if m.scroll > m.maxScroll() {
			m.scroll = m.maxScroll()
		}
	}

	return m, nil
}

// handleRatingMsg delegates all messages to the embedded huh.Select form.
// On completion the chosen value is committed; on abort the overlay closes.
func (m DetailModel) handleRatingMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.ratingForm.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.ratingForm = f
	}
	switch m.ratingForm.State {
	case huh.StateCompleted:
		if m.ratingPending != nil {
			if *m.ratingPending == 0 {
				m.updateRating = true
				m.newRating = nil
			} else {
				m.updateRating = true
				v := *m.ratingPending
				m.newRating = &v
			}
		}
		m.ratingMode = false
		return m, tea.Quit
	case huh.StateAborted:
		m.ratingMode = false
		return m, nil
	}
	return m, cmd
}

func buildRatingForm(value *int, termWidth int) *huh.Form {
	formWidth := termWidth - 4
	if formWidth > 52 {
		formWidth = 52
	}
	if formWidth < 24 {
		formWidth = 24
	}
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("Rate this recipe").
				Options(
					huh.NewOption("★★★★★  5 stars", 5),
					huh.NewOption("★★★★☆  4 stars", 4),
					huh.NewOption("★★★☆☆  3 stars", 3),
					huh.NewOption("★★☆☆☆  2 stars", 2),
					huh.NewOption("★☆☆☆☆  1 star", 1),
					huh.NewOption("(no rating)", 0),
				).
				Value(value),
		),
	).WithWidth(formWidth)
}

// handleNotesKey processes keys while the notes overlay is open.
// ctrl+s saves and quits; Esc discards and closes the overlay.
func (m DetailModel) handleNotesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+s":
		m.updateNotes = true
		m.newNotes = m.notesInput.Value()
		m.notesMode = false
		return m, tea.Quit
	case "esc":
		m.notesMode = false
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	default:
		var cmd tea.Cmd
		m.notesInput, cmd = m.notesInput.Update(msg)
		return m, cmd
	}
}

// handleNavKey processes keys while content or footer has focus.
func (m DetailModel) handleNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "q":
		return m, tea.Quit

	case "esc":
		// Go back to the list (preserving any active filter).
		m.goHome = true
		return m, tea.Quit

	case "h":
		m.goHome = true
		return m, tea.Quit

	case "a":
		m.goAdd = true
		return m, tea.Quit

	case "e":
		m.goEdit = true
		return m, tea.Quit

	case "p":
		m.goPrint = true
		return m, tea.Quit

	case "s":
		m.goScale = true
		return m, tea.Quit

	case "m":
		m.goManage = true
		return m, tea.Quit

	case "r":
		if m.recipe.IsFailed() {
			m.goRetry = true
			return m, tea.Quit
		}
		v := 0
		if m.recipe.Rating != nil {
			v = *m.recipe.Rating
		}
		m.ratingPending = &v
		m.ratingForm = buildRatingForm(m.ratingPending, m.width)
		m.ratingMode = true
		return m, m.ratingForm.Init()

	case "N":
		m.notesMode = true
		m.notesInput.SetValue(m.recipe.Notes)
		m.notesInput.SetWidth(m.width - 12)
		return m, m.notesInput.Focus()

	case "d":
		m.confirmingDelete = true

	case "/", "right":
		m.filter = m.filter.enter()
		m.filter.focus = ffText
		m.focus = detailFocusHeader
		m.lines = m.buildLines()
		if m.scroll > m.maxScroll() {
			m.scroll = m.maxScroll()
		}

	case "up", "k":
		if m.scroll > 0 {
			m.scroll--
		}

	case "down", "j":
		if m.scroll < m.maxScroll() {
			m.scroll++
		}

	case "pgup":
		m.scroll -= m.viewportHeight()
		if m.scroll < 0 {
			m.scroll = 0
		}

	case "pgdown":
		m.scroll += m.viewportHeight()
		if m.scroll > m.maxScroll() {
			m.scroll = m.maxScroll()
		}
	}

	return m, nil
}

func (m DetailModel) View() string {
	if m.width == 0 {
		return ""
	}
	lines := m.lines
	if len(lines) == 0 {
		lines = m.buildLines()
	}

	var sb strings.Builder

	// Banner — same structure as list view, with recipe name as breadcrumb.
	sb.WriteString(renderDetailBanner(m.recipe.Name, m.recipe.IsBread, m.width))
	sb.WriteString("\n")

	// Delete confirmation overlay — replaces content and footer.
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

	// Rating overlay — huh form takes over the content area.
	if m.ratingMode && m.ratingForm != nil {
		sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, m.ratingForm.View()))
		return sb.String()
	}

	// Notes overlay — replaces content and footer.
	if m.notesMode {
		sb.WriteString(m.viewNotesOverlay())
		used := strings.Count(sb.String(), "\n")
		if fill := m.height - used - 3; fill > 0 {
			sb.WriteString(strings.Repeat("\n", fill))
		}
		sb.WriteString("\n")
		sb.WriteString(renderNotesFooter(m.width))
		return sb.String()
	}

	vh := m.viewportHeight()
	start := m.scroll
	end := start + vh
	if end > len(lines) {
		end = len(lines)
	}

	if m.focus == detailFocusHeader {
		// Split layout: content on left (66%), filter pane on right (33%).
		listWidth := (m.width * 2) / 3
		filterWidth := m.width - listWidth

		var lsb strings.Builder
		for i := start; i < end; i++ {
			lsb.WriteString(lines[i])
			lsb.WriteString("\n")
		}
		for i := end - start; i < vh; i++ {
			lsb.WriteString("\n")
		}

		leftPane := lipgloss.NewStyle().Width(listWidth).Render(lsb.String())
		filterInnerH := vh - 2
		if filterInnerH < 3 {
			filterInnerH = 3
		}
		rightPane := renderFilterPanel(m.filter, filterWidth, filterInnerH)
		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane))
		sb.WriteString("\n\n")
	} else {
		// Single-column: full-width scrollable content viewport.
		for i := start; i < end; i++ {
			sb.WriteString(lines[i])
			sb.WriteString("\n")
		}
		// Pad remaining viewport rows so the footer stays pinned to the bottom.
		for i := end - start; i < vh; i++ {
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Footer.
	sb.WriteString(renderDetailFooter(m.recipe.IsFailed(), m.width))

	return sb.String()
}

func (m DetailModel) viewConfirm() string {
	var sb strings.Builder
	sb.WriteString("\n\n")

	name := truncate(m.recipe.Name, m.width-20)
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

func (m DetailModel) viewNotesOverlay() string {
	var sb strings.Builder
	sb.WriteString("\n\n")

	inner := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("Notes"),
		"",
		m.notesInput.View(),
	)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Render(inner)

	sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, box))
	sb.WriteString("\n")
	return sb.String()
}

func renderNotesFooter(width int) string {
	keys := []string{
		lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess).Render("ctrl+s save"),
		MutedStyle.Render("Esc cancel"),
	}
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(footerLine(keys, width-2))
}

// buildLines renders the recipe body at the current terminal width and splits
// the result into individual terminal lines for viewport scrolling.
// When the filter pane is open the content is constrained to the left 66% of the terminal.
func (m DetailModel) buildLines() []string {
	contentWidth := m.width - 4
	if m.focus == detailFocusHeader {
		contentWidth = (m.width * 2 / 3) - 4
	}
	if contentWidth > 100 {
		contentWidth = 100
	}
	if contentWidth < 20 {
		contentWidth = 20
	}
	raw := buildRecipeBlock(m.recipe, contentWidth)
	return strings.Split(raw, "\n")
}

// buildRecipeBlock assembles the full styled recipe body as a single string.
func buildRecipeBlock(r *models.Recipe, width int) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Width(width).
		Render(r.Name))
	sb.WriteString("\n")

	// Timing & servings.
	var meta []string
	if t := r.TimingSummary(); t != "" {
		meta = append(meta, MutedStyle.Render(t))
	}
	if r.Servings != nil && *r.Servings > 0 {
		units := r.ServingUnits
		if units == "" {
			units = "servings"
		}
		meta = append(meta, MutedStyle.Render(fmt.Sprintf("Serves %d %s", *r.Servings, units)))
	}
	if len(meta) > 0 {
		sb.WriteString(strings.Join(meta, MutedStyle.Render("  ·  ")))
		sb.WriteString("\n")
	}
	if r.Rating != nil {
		sb.WriteString(MutedStyle.Render("Rating: "))
		sb.WriteString(lipgloss.NewStyle().Foreground(ColorPrimary).Render(r.RatingGlyphs()))
		sb.WriteString("\n")
	}
	if r.IsBread {
		if bm, err := scaling.BreadMetrics(r.Ingredients); err == nil {
			sb.WriteString(renderHydrationGauge(bm, width))
		}
	}

	// Tag pills.
	if tags := buildTagPills(r); tags != "" {
		sb.WriteString(tags)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Description.
	if r.Description != "" {
		sb.WriteString(lipgloss.NewStyle().
			Italic(true).
			Foreground(ColorSubtle).
			Width(width).
			Render(r.Description))
		sb.WriteString("\n\n")
	}

	// Ingredients.
	if len(r.Ingredients) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionRule(width, "Ingredients"))
		sb.WriteString("\n")
		sb.WriteString(buildIngredientLines(r.Ingredients))
		sb.WriteString("\n")
	}

	// Directions.
	if r.Directions != "" {
		sb.WriteString("\n")
		sb.WriteString(sectionRule(width, "Directions"))
		sb.WriteString("\n")
		sb.WriteString(renderMarkdown(r.Directions, width))
	}

	// Source URL.
	if r.SourceURL != "" {
		sb.WriteString("\n")
		sb.WriteString(MutedStyle.Render("Source: " + r.SourceURL))
		sb.WriteString("\n")
	}

	// Notes.
	if r.Notes != "" {
		sb.WriteString("\n")
		sb.WriteString(sectionRule(width, "Notes"))
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().
			Foreground(ColorMuted).
			Width(width).
			Render(r.Notes))
		sb.WriteString("\n")
	}

	// Baker's percentages chart — bread/dough recipes only.
	if r.IsBread {
		if bm, err := scaling.BreadMetrics(r.Ingredients); err == nil && len(bm.PerIngredient) > 0 {
			sb.WriteString("\n")
			sb.WriteString(sectionRule(width, "Baker's Percentages"))
			sb.WriteString("\n")
			sb.WriteString(renderBakerBars(bm, width))
		}
	}

	return sb.String()
}

// renderHydrationGauge renders the headline hydration line for a bread
// recipe: a colour-ramped gauge, the percentage, and total dough weight.
//
//	Hydration ███████████▉░░░░░░ 72.4%  ·  1864g dough
func renderHydrationGauge(bm scaling.BreadMetricsResult, width int) string {
	totalG := int(bm.TotalDryGrams + bm.TotalWetGrams + bm.TotalFatGrams + 0.5)
	label := MutedStyle.Render("Hydration ")
	pctStr := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).
		Render(fmt.Sprintf(" %.1f%%", bm.HydrationPct))
	doughStr := MutedStyle.Render(fmt.Sprintf("  ·  %dg dough", totalG))

	gaugeW := width - lipgloss.Width(label) - lipgloss.Width(pctStr) - lipgloss.Width(doughStr)
	if gaugeW > 32 {
		gaugeW = 32
	}
	if gaugeW < 8 {
		gaugeW = 8
	}

	var sb strings.Builder
	sb.WriteString(label)
	sb.WriteString(renderGauge(hydrationGaugeFrac(bm.HydrationPct), gaugeW))
	sb.WriteString(pctStr)
	sb.WriteString(doughStr)
	sb.WriteString("\n")
	if bm.StarterCount > 0 {
		sb.WriteString(MutedStyle.Render("(100% hydration starter assumed)"))
		sb.WriteString("\n")
	}
	return sb.String()
}

// renderBakerBars renders the baker's percentages as a horizontal bar chart,
// one bar per ingredient, coloured by ingredient type:
//
//	bread flour   500g ████████████████████ 100.0%
//	water         360g ██████████████▍       72.0%
func renderBakerBars(bm scaling.BreadMetricsResult, width int) string {
	var sb strings.Builder

	// Column widths.
	maxName := 0
	maxPct := 100.0
	for _, ing := range bm.PerIngredient {
		n := len([]rune(ing.Name))
		if ing.Type == "starter" {
			n++
		}
		if n > maxName {
			maxName = n
		}
		if ing.Percentage > maxPct {
			maxPct = ing.Percentage
		}
	}
	if maxName > 24 {
		maxName = 24
	}

	// "  name  1234g " + bar + " 100.0%"
	barW := width - maxName - 18
	if barW > 30 {
		barW = 30
	}
	if barW < 8 {
		barW = 8
	}

	for _, ing := range bm.PerIngredient {
		name := ing.Name
		if ing.Type == "starter" {
			name += "*"
		}
		name = truncate(name, maxName)
		grams := int(ing.WeightGrams + 0.5)
		sb.WriteString(MutedStyle.Render(fmt.Sprintf("  %-*s %5dg ", maxName, name, grams)))
		sb.WriteString(renderBar(ing.Percentage/maxPct, barW, ingredientTypeColor(ing.Type)))
		sb.WriteString(lipgloss.NewStyle().Foreground(ColorSubtle).Render(fmt.Sprintf(" %5.1f%%", ing.Percentage)))
		sb.WriteString("\n")
	}

	if bm.StarterCount > 0 {
		sb.WriteString(MutedStyle.Render("  * 100% hydration starter assumed"))
		sb.WriteString("\n")
	}
	return sb.String()
}

func buildTagPills(r *models.Recipe) string {
	var pills []string
	if r.IsBread {
		pills = append(pills, BreadPill)
	}
	for _, ctx := range models.AllTagContexts {
		for _, name := range r.TagsByContext(ctx) {
			pills = append(pills, TagStyle(ctx).Render(name))
		}
	}
	return strings.Join(pills, "")
}

func buildIngredientLines(ings []models.RecipeIngredient) string {
	var sb strings.Builder
	currentSection := ""
	for _, ing := range ings {
		if ing.Section != currentSection && ing.Section != "" {
			if currentSection != "" {
				sb.WriteString("\n")
			}
			sb.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorMuted).
				Render("  " + ing.Section))
			sb.WriteString("\n")
			currentSection = ing.Section
		}
		sb.WriteString(MutedStyle.Render("  · ") + ing.DisplayString())
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderMarkdown(text string, width int) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(detectedGlamourStyle()),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return text
	}
	out, err := r.Render(text)
	if err != nil {
		return text
	}
	return out
}

// breadcrumbTitle builds the styled "🍳 enplace / trail" banner segment.
func breadcrumbTitle(trail string) string {
	app := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("🍳 enplace")
	if trail == "" {
		return app
	}
	return app + MutedStyle.Render(" / ") +
		lipgloss.NewStyle().Foreground(ColorSubtle).Render(trail)
}

// renderDetailBanner renders the one-line titled banner rule with an
// "enplace / Recipe Name" breadcrumb.
func renderDetailBanner(name string, isBread bool, width int) string {
	maxNameLen := width - 30
	if maxNameLen < 8 {
		maxNameLen = 8
	}
	displayName := truncate(name, maxNameLen)
	if isBread {
		displayName += " 🍞"
	}
	return flatRuleStyled(width, breadcrumbTitle(displayName), "", ColorBorder, ColorPrimary)
}

// renderDetailFooter renders the key-hint footer for the recipe detail view.
func renderDetailFooter(isFailed bool, width int) string {
	keys := []string{
		keyHint("↑/↓", "scroll"),
		keyHint("h", "home"),
		keyHint("e", "edit"),
		keyHint("s", "scale"),
		keyHint("p", "export"),
		keyHint("d", "delete"),
	}
	if isFailed {
		keys = append(keys, keyHint("r", "retry"))
	} else {
		keys = append(keys, keyHint("r", "rate"))
		keys = append(keys, keyHint("N", "notes"))
	}

	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(footerLine(keys, width-2))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RunDetailUI runs the interactive recipe detail TUI.
// initial carries the active filter from the calling context; sd provides autocomplete suggestions.
// Returns navigation signals, rating/notes updates, the return filter state, and any error.
func RunDetailUI(recipe *models.Recipe, initial FilterState, sd SearchData) (goHome bool, goAdd bool, goEdit bool, goPrint bool, goScale bool, goManage bool, goRetry bool, deleteConfirmed bool, updateRating bool, newRating *int, updateNotes bool, newNotes string, returnFilter FilterState, err error) {
	m := NewDetailModel(recipe, initial, sd)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, runErr := p.Run()
	if runErr != nil {
		return false, false, false, false, false, false, false, false, false, nil, false, "", FilterState{}, runErr
	}
	fm := final.(DetailModel)
	return fm.GoHome(), fm.GoAdd(), fm.GoEdit(), fm.GoPrint(), fm.GoScale(), fm.GoManage(), fm.GoRetry(), fm.DeleteConfirmed(), fm.UpdateRating(), fm.NewRating(), fm.UpdateNotes(), fm.NewNotes(), fm.ReturnFilter(), nil
}
