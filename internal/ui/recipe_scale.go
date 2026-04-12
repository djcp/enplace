package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/enplace/internal/export"
	"github.com/djcp/enplace/internal/models"
	"github.com/djcp/enplace/internal/scaling"
)

type scalePhase int

const (
	scalePhaseInput scalePhase = iota
	scalePhaseView
)

// ScaleModel is a two-phase TUI for scaling recipe ingredients.
//
//   - scalePhaseInput: user enters a scale factor or target dough weight.
//   - scalePhaseView: the full recipe rendered with scaled quantities (same
//     layout as the detail view), plus bread metrics when available.
type ScaleModel struct {
	recipe *models.Recipe
	opts   export.Options
	phase  scalePhase

	// Input phase.
	factorInput  textinput.Model
	weightInput  textinput.Model
	inputFocus   int // 0=factor, 1=weight
	inputErr     string
	canUseWeight bool // true when all classified ings have weight units

	// View phase.
	scaledRecipe *models.Recipe // full recipe copy: scaled ings + directions note
	scaleFactor  float64
	lines        []string // pre-rendered content lines for scrolling
	scroll       int

	width   int
	height  int
	goBack  bool
	goPrint bool
}

func newScaleModel(recipe *models.Recipe, opts export.Options) ScaleModel {
	fi := textinput.New()
	fi.Placeholder = "e.g. 2  or  0.5"
	fi.Width = 18
	fi.Focus()

	wi := textinput.New()
	wi.Placeholder = "grams"
	wi.Width = 18

	canUseWeight := false
	if recipe.IsBread {
		_, canUseWeight = scaling.TotalWeightGrams(recipe.Ingredients)
	}

	return ScaleModel{
		recipe:       recipe,
		opts:         opts,
		phase:        scalePhaseInput,
		factorInput:  fi,
		weightInput:  wi,
		canUseWeight: canUseWeight,
		width:        80,
		height:       24,
	}
}

// GoBack returns true when the user dismissed the scale screen.
func (m ScaleModel) GoBack() bool { return m.goBack }

// GoPrint returns true when the user pressed p to print/export the scaled recipe.
func (m ScaleModel) GoPrint() bool { return m.goPrint }

func (m ScaleModel) Init() tea.Cmd { return textinput.Blink }

func (m ScaleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.phase == scalePhaseView && m.scaledRecipe != nil {
			m.lines = m.buildScaledLines()
		}
		return m, nil
	case tea.KeyMsg:
		if m.phase == scalePhaseInput {
			return m.handleInputKey(msg)
		}
		return m.handleViewKey(msg)
	}

	// Forward blink to active input.
	if m.phase == scalePhaseInput {
		var cmd tea.Cmd
		if m.inputFocus == 0 {
			m.factorInput, cmd = m.factorInput.Update(msg)
		} else {
			m.weightInput, cmd = m.weightInput.Update(msg)
		}
		return m, cmd
	}
	return m, nil
}

func (m ScaleModel) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.goBack = true
		return m, tea.Quit
	case "tab":
		if m.canUseWeight {
			m.inputFocus = 1 - m.inputFocus
			if m.inputFocus == 0 {
				m.factorInput.Focus()
				m.weightInput.Blur()
			} else {
				m.weightInput.Focus()
				m.factorInput.Blur()
			}
		}
		return m, nil
	case "enter":
		factor, errMsg := m.computeFactor()
		if errMsg != "" {
			m.inputErr = errMsg
			return m, nil
		}
		m.scaleFactor = factor
		scaled := scaling.ScaleIngredients(m.recipe.Ingredients, factor)
		m.scaledRecipe = makeScaledRecipeCopy(m.recipe, scaled, factor)
		m.phase = scalePhaseView
		m.scroll = 0
		m.lines = m.buildScaledLines()
		return m, nil
	}

	var cmd tea.Cmd
	if m.inputFocus == 0 {
		m.factorInput, cmd = m.factorInput.Update(msg)
	} else {
		m.weightInput, cmd = m.weightInput.Update(msg)
	}
	m.inputErr = ""
	return m, cmd
}

func (m ScaleModel) handleViewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.phase = scalePhaseInput
		m.scaledRecipe = nil
		m.lines = nil
		return m, nil
	case "p":
		m.goPrint = true
		return m, tea.Quit
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

// computeFactor derives the scale multiplier from whichever input is active.
// Returns (factor, "") on success or (0, errMsg) on failure.
func (m ScaleModel) computeFactor() (float64, string) {
	if m.inputFocus == 0 {
		raw := strings.TrimSpace(m.factorInput.Value())
		if raw == "" {
			return 0, "enter a scale factor (e.g. 2 or 0.5)"
		}
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil || f <= 0 {
			return 0, "scale factor must be a positive number (e.g. 2 or 0.5)"
		}
		return f, ""
	}

	// Target dough weight path.
	raw := strings.TrimSpace(m.weightInput.Value())
	if raw == "" {
		return 0, "enter a target dough weight in grams"
	}
	target, err := strconv.ParseFloat(raw, 64)
	if err != nil || target <= 0 {
		return 0, "target weight must be a positive number of grams"
	}
	current, ok := scaling.TotalWeightGrams(m.recipe.Ingredients)
	if !ok || current == 0 {
		return 0, "cannot compute: some classified ingredients lack weight units"
	}
	return target / current, ""
}

func (m ScaleModel) viewportHeight() int {
	v := m.height - 8
	if v < 4 {
		v = 4
	}
	return v
}

func (m ScaleModel) maxScroll() int {
	ms := len(m.lines) - m.viewportHeight()
	if ms < 0 {
		return 0
	}
	return ms
}

// buildScaledLines renders the view-phase content: an optional bread-metrics
// summary block followed by the full recipe block (same renderer as detail view).
func (m ScaleModel) buildScaledLines() []string {
	contentWidth := m.width - 4
	if contentWidth > 100 {
		contentWidth = 100
	}
	if contentWidth < 20 {
		contentWidth = 20
	}

	var sb strings.Builder

	// Full recipe rendered identically to the detail view.
	sb.WriteString(buildRecipeBlock(m.scaledRecipe, contentWidth))

	return strings.Split(sb.String(), "\n")
}

func (m ScaleModel) View() string {
	if m.width == 0 {
		return ""
	}

	var sb strings.Builder

	if m.phase == scalePhaseInput {
		sb.WriteString(renderScaleInputBanner(m.recipe.Name, m.recipe.IsBread, m.width))
		sb.WriteString("\n")
		sb.WriteString(m.renderInputPhase())
		sb.WriteString(renderScaleInputFooter(m.canUseWeight, m.width))
		return sb.String()
	}

	// View phase: banner shows scale factor.
	sb.WriteString(renderScaleViewBanner(m.recipe.Name, m.recipe.IsBread, m.scaleFactor, m.width))
	sb.WriteString("\n")

	lines := m.lines
	vh := m.viewportHeight()
	start := m.scroll
	end := start + vh
	if end > len(lines) {
		end = len(lines)
	}
	for i := start; i < end; i++ {
		sb.WriteString(lines[i])
		sb.WriteString("\n")
	}
	for i := end - start; i < vh; i++ {
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(renderScaleViewFooter(m.width))
	return sb.String()
}

func (m ScaleModel) renderInputPhase() string {
	var sb strings.Builder
	w := m.width - 8
	if w > 60 {
		w = 60
	}

	renderInput := func(label string, inp textinput.Model, focused bool, note string) string {
		lbl := MutedStyle.Width(26).Render(label)
		inner := lbl + inp.View()
		if note != "" {
			inner += "  " + MutedStyle.Render(note)
		}
		if focused {
			return lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1).
				MarginLeft(2).
				Render(inner)
		}
		return "  " + inner
	}

	sb.WriteString("\n")
	sb.WriteString(renderInput("Scale factor:", m.factorInput, m.inputFocus == 0, ""))
	sb.WriteString("\n\n")

	if m.recipe.IsBread {
		weightLabel := "Target dough weight (g):"
		if m.canUseWeight {
			sb.WriteString(renderInput(weightLabel, m.weightInput, m.inputFocus == 1, ""))
			sb.WriteString("\n")
			current, _ := scaling.TotalWeightGrams(m.recipe.Ingredients)
			sb.WriteString(MutedStyle.Render(fmt.Sprintf("  Current dough weight: %.0fg", current)))
			sb.WriteString("\n")
		} else {
			note := "(requires all wet/dry ingredients in weight units)"
			sb.WriteString(MutedStyle.Render("  " + MutedStyle.Width(26).Render(weightLabel) + "—  " + note))
			sb.WriteString("\n")
		}
	}

	if m.inputErr != "" {
		sb.WriteString("\n")
		sb.WriteString(ErrorStyle.Render("  " + m.inputErr))
		sb.WriteString("\n")
	}

	content := sb.String()
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center,
		lipgloss.NewStyle().Width(w+8).Render(content))
}

// formatFactor returns a clean display string for a scale factor.
func formatFactor(f float64) string {
	if f == float64(int(f)) {
		return strconv.Itoa(int(f))
	}
	// Up to 4 significant digits, no trailing zeros.
	return strconv.FormatFloat(f, 'g', 4, 64)
}

func renderScaleInputBanner(recipeName string, isBread bool, width int) string {
	displayName := truncate(recipeName, width-30)
	if isBread {
		displayName += "  🍞"
	}
	breadcrumb := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(
		"🍳  enplace  " + MutedStyle.Render("/") + "  " +
			lipgloss.NewStyle().Bold(false).Foreground(ColorSubtle).
				Render(displayName+" / Scale"),
	)
	title := lipgloss.NewStyle().Padding(1, 2).Render(breadcrumb)
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(title)
}

func renderScaleViewBanner(recipeName string, isBread bool, factor float64, width int) string {
	factorLabel := "×" + formatFactor(factor)
	displayName := truncate(recipeName, width-30)
	if isBread {
		displayName += "  🍞"
	}
	subtitle := displayName + " / Scale " + factorLabel

	breadcrumb := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(
		"🍳  enplace  " + MutedStyle.Render("/") + "  " +
			lipgloss.NewStyle().Bold(false).Foreground(ColorSubtle).Render(subtitle),
	)
	title := lipgloss.NewStyle().Padding(1, 2).Render(breadcrumb)
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(title)
}

func renderScaleInputFooter(canWeight bool, width int) string {
	keys := []string{
		MutedStyle.Render("enter confirm"),
		MutedStyle.Render("esc back"),
	}
	if canWeight {
		keys = append([]string{MutedStyle.Render("tab switch field")}, keys...)
	}
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(footerLine(keys, width-2))
}

func renderScaleViewFooter(width int) string {
	keys := []string{
		MutedStyle.Render("↑/↓ scroll"),
		MutedStyle.Render("p print/export"),
		MutedStyle.Render("esc change factor"),
		MutedStyle.Render("q quit"),
	}
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(footerLine(keys, width-2))
}

// makeScaledRecipeCopy returns a copy of r with:
//   - Ingredients replaced by the scaled slice.
//   - A scaling notice prepended to Directions so it appears in all output
//     formats (TUI, print preview, PDF, plain text, etc.).
func makeScaledRecipeCopy(r *models.Recipe, scaled []models.RecipeIngredient, factor float64) *models.Recipe {
	cp := *r
	cp.Ingredients = scaled
	if r.Directions != "" {
		cp.Directions = scaledDirectionsNote(factor) + r.Directions
	}
	return &cp
}

// scaledDirectionsNote returns a Markdown blockquote notice that appears at the
// top of the directions for a scaled recipe.
func scaledDirectionsNote(factor float64) string {
	return fmt.Sprintf(
		"> **Scaled ×%s** — Ingredient amounts above have been scaled. "+
			"Step amounts in these directions reference the original recipe quantities "+
			"and have not been adjusted.\n\n",
		formatFactor(factor),
	)
}

// RunScaleUI runs the interactive scale TUI for the given recipe.
// Returns (goPrint, scaledRecipe, error). When goPrint is true, scaledRecipe
// is a fully prepared copy (scaled ingredients + directions note) ready to
// pass directly to RunPrintUI.
func RunScaleUI(recipe *models.Recipe, opts export.Options) (goPrint bool, scaledRecipe *models.Recipe, err error) {
	m := newScaleModel(recipe, opts)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, runErr := p.Run()
	if runErr != nil {
		return false, nil, runErr
	}
	fm := final.(ScaleModel)
	if fm.GoPrint() {
		return true, fm.scaledRecipe, nil
	}
	return false, nil, nil
}
