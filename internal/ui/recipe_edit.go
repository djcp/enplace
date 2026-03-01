package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/gorecipes/internal/models"
)

// EditData carries autocomplete options loaded from the database.
type EditData struct {
	TagsByContext   map[string][]string
	IngredientNames []string
	Units           []string
}

type editFocus int

const (
	efName editFocus = iota
	efStatus
	efDescription
	efPrepTime
	efCookTime
	efServings
	efServingUnits
	efTagCourses
	efTagCooking
	efTagCultural
	efTagDietary
	efIngredients
	efDirections
	efCount
)

// tagContextForFocus maps tag focus values to their model context string.
var tagContextForFocus = map[editFocus]string{
	efTagCourses:  models.TagContextCourses,
	efTagCooking:  models.TagContextCookingMethods,
	efTagCultural: models.TagContextCulturalInfluences,
	efTagDietary:  models.TagContextDietaryRestrictions,
}

var editStatusOptions = []string{
	models.StatusDraft,
	models.StatusReview,
	models.StatusPublished,
}

type ingredientRow struct {
	qty        textinput.Model
	unit       textinput.Model
	name       textinput.Model
	descriptor textinput.Model
	section    textinput.Model
}

func newIngredientRow() ingredientRow {
	qty := textinput.New()
	qty.Placeholder = "qty"
	qty.Width = 6

	unit := textinput.New()
	unit.Placeholder = "unit"
	unit.Width = 10

	name := textinput.New()
	name.Placeholder = "ingredient"
	name.Width = 18

	desc := textinput.New()
	desc.Placeholder = "descriptor"
	desc.Width = 12

	sect := textinput.New()
	sect.Placeholder = "section"
	sect.Width = 10

	return ingredientRow{qty: qty, unit: unit, name: name, descriptor: desc, section: sect}
}

func populateIngredientRow(ri models.RecipeIngredient) ingredientRow {
	row := newIngredientRow()
	row.qty.SetValue(ri.Quantity)
	row.unit.SetValue(ri.Unit)
	row.name.SetValue(ri.IngredientName)
	row.descriptor.SetValue(ri.Descriptor)
	row.section.SetValue(ri.Section)
	return row
}

// EditModel is a Bubbletea model for the recipe edit / create form.
type EditModel struct {
	isNew  bool
	recipe *models.Recipe // nil if new

	nameInput         textinput.Model
	statusIdx         int
	descInput         textarea.Model
	prepInput         textinput.Model
	cookInput         textinput.Model
	servingsInput     textinput.Model
	servingUnitsInput textinput.Model
	sourceURL         string // read-only; preserved as-is on save
	directionsInput   textarea.Model

	// context → selected pills
	tagValues map[string][]string
	// context → live text input
	tagInputs map[string]textinput.Model

	ingRows      []ingredientRow
	ingRowCursor int
	ingColCursor int // 0–4

	allIngNames []string
	allUnits    []string
	allTags     map[string][]string

	focused editFocus
	width   int
	height  int
	scroll  int

	saved  bool
	goHome bool
	errMsg string
}

func newEditModel(recipe *models.Recipe, data EditData) EditModel {
	m := EditModel{
		isNew:       recipe == nil,
		recipe:      recipe,
		allIngNames: data.IngredientNames,
		allUnits:    data.Units,
		allTags:     data.TagsByContext,
		width:       80,
		height:      24,
		tagValues:   make(map[string][]string),
		tagInputs:   make(map[string]textinput.Model),
	}
	if m.allTags == nil {
		m.allTags = make(map[string][]string)
	}

	// Build top-level text inputs.
	m.nameInput = textinput.New()
	m.nameInput.Placeholder = "Recipe name"
	m.nameInput.Width = 40

	m.prepInput = textinput.New()
	m.prepInput.Placeholder = "0"
	m.prepInput.Width = 6

	m.cookInput = textinput.New()
	m.cookInput.Placeholder = "0"
	m.cookInput.Width = 6

	m.servingsInput = textinput.New()
	m.servingsInput.Placeholder = "0"
	m.servingsInput.Width = 6

	m.servingUnitsInput = textinput.New()
	m.servingUnitsInput.Placeholder = "servings"
	m.servingUnitsInput.Width = 12

	// Build textarea inputs.
	m.descInput = textarea.New()
	m.descInput.Placeholder = "Short description..."
	m.descInput.ShowLineNumbers = false
	m.descInput.SetHeight(3)

	m.directionsInput = textarea.New()
	m.directionsInput.Placeholder = "Step-by-step directions..."
	m.directionsInput.ShowLineNumbers = false
	m.directionsInput.SetHeight(6)

	// Build tag inputs for each context.
	for _, ctx := range models.AllTagContexts {
		ti := textinput.New()
		ti.Placeholder = "add tag..."
		ti.Width = 18
		suggestions := m.allTags[ctx]
		if len(suggestions) > 0 {
			ti.SetSuggestions(suggestions)
			ti.ShowSuggestions = true
		}
		m.tagInputs[ctx] = ti
		m.tagValues[ctx] = nil
	}

	// Populate from existing recipe.
	if recipe != nil {
		m.nameInput.SetValue(recipe.Name)
		m.statusIdx = statusIndex(recipe.Status)
		m.descInput.SetValue(recipe.Description)
		if recipe.PreparationTime != nil {
			m.prepInput.SetValue(strconv.Itoa(*recipe.PreparationTime))
		}
		if recipe.CookingTime != nil {
			m.cookInput.SetValue(strconv.Itoa(*recipe.CookingTime))
		}
		if recipe.Servings != nil {
			m.servingsInput.SetValue(strconv.Itoa(*recipe.Servings))
		}
		m.servingUnitsInput.SetValue(recipe.ServingUnits)
		m.sourceURL = recipe.SourceURL
		m.directionsInput.SetValue(recipe.Directions)

		// Load tag pills.
		for _, ctx := range models.AllTagContexts {
			m.tagValues[ctx] = recipe.TagsByContext(ctx)
		}

		// Load ingredient rows.
		for _, ri := range recipe.Ingredients {
			m.ingRows = append(m.ingRows, populateIngredientRow(ri))
		}
	}

	// Always have at least one ingredient row.
	if len(m.ingRows) == 0 {
		m.ingRows = append(m.ingRows, newIngredientRow())
	}

	// Set ingredient suggestions.
	for i := range m.ingRows {
		m.ingRows[i].name.SetSuggestions(m.allIngNames)
		m.ingRows[i].name.ShowSuggestions = true
		m.ingRows[i].unit.SetSuggestions(m.allUnits)
		m.ingRows[i].unit.ShowSuggestions = true
	}

	// Start focus on name.
	m.nameInput.Focus()
	return m
}

func statusIndex(status string) int {
	for i, s := range editStatusOptions {
		if s == status {
			return i
		}
	}
	return 0
}

func (m EditModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m EditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Resize text inputs to fit.
		formWidth := m.formWidth()
		m.nameInput.Width = formWidth - 14
		m.descInput.SetWidth(formWidth - 4)
		m.directionsInput.SetWidth(formWidth - 4)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward messages to focused textarea for cursor blink etc.
	switch m.focused {
	case efDescription:
		var cmd tea.Cmd
		m.descInput, cmd = m.descInput.Update(msg)
		return m, cmd
	case efDirections:
		var cmd tea.Cmd
		m.directionsInput, cmd = m.directionsInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m EditModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys.
	switch msg.String() {
	case "ctrl+s":
		if strings.TrimSpace(m.nameInput.Value()) == "" {
			m.errMsg = "Recipe name is required"
			return m, nil
		}
		m.errMsg = ""
		m.saved = true
		return m, tea.Quit
	case "esc", "ctrl+c":
		m.goHome = true
		return m, tea.Quit
	}

	switch m.focused {
	case efName:
		var cmd tea.Cmd
		m, m.nameInput, cmd = m.handleTextInput(msg, m.nameInput)
		return m, cmd

	case efStatus:
		switch msg.String() {
		case "left", "h":
			if m.statusIdx > 0 {
				m.statusIdx--
			}
		case "right", "l":
			if m.statusIdx < len(editStatusOptions)-1 {
				m.statusIdx++
			}
		case "tab":
			m.advanceFocus()
		case "shift+tab":
			m.retreatFocus()
		}
		return m, nil

	case efDescription:
		if msg.String() == "tab" {
			m.advanceFocus()
			return m, nil
		}
		if msg.String() == "shift+tab" {
			m.retreatFocus()
			return m, nil
		}
		var cmd tea.Cmd
		m.descInput, cmd = m.descInput.Update(msg)
		return m, cmd

	case efPrepTime:
		var cmd tea.Cmd
		m, m.prepInput, cmd = m.handleTextInput(msg, m.prepInput)
		return m, cmd
	case efCookTime:
		var cmd tea.Cmd
		m, m.cookInput, cmd = m.handleTextInput(msg, m.cookInput)
		return m, cmd
	case efServings:
		var cmd tea.Cmd
		m, m.servingsInput, cmd = m.handleTextInput(msg, m.servingsInput)
		return m, cmd
	case efServingUnits:
		var cmd tea.Cmd
		m, m.servingUnitsInput, cmd = m.handleTextInput(msg, m.servingUnitsInput)
		return m, cmd

	case efTagCourses, efTagCooking, efTagCultural, efTagDietary:
		return m.handleTagKey(msg)

	case efIngredients:
		return m.handleIngredientKey(msg)

	case efDirections:
		if msg.String() == "tab" {
			m.advanceFocus()
			return m, nil
		}
		if msg.String() == "shift+tab" {
			m.retreatFocus()
			return m, nil
		}
		var cmd tea.Cmd
		m.directionsInput, cmd = m.directionsInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleTextInput processes a key for a textinput with Tab disambiguation.
// Returns the updated model, the updated input, and any command.
// The caller must assign the returned input back to the appropriate field.
func (m EditModel) handleTextInput(msg tea.KeyMsg, inp textinput.Model) (EditModel, textinput.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		oldVal := inp.Value()
		newInp, cmd := inp.Update(msg)
		if newInp.Value() != oldVal {
			// Tab accepted a suggestion — stay on this field.
			return m, newInp, cmd
		}
		m.advanceFocus()
		return m, inp, nil
	case "shift+tab":
		m.retreatFocus()
		return m, inp, nil
	default:
		newInp, cmd := inp.Update(msg)
		return m, newInp, cmd
	}
}

func (m EditModel) handleTagKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ctx := tagContextForFocus[m.focused]
	ti := m.tagInputs[ctx]

	switch msg.String() {
	case "enter":
		val := strings.ToLower(strings.TrimSpace(ti.Value()))
		if val != "" {
			m.tagValues[ctx] = append(m.tagValues[ctx], val)
		}
		ti.SetValue("")
		m.tagInputs[ctx] = ti
		return m, nil

	case "backspace":
		if ti.Value() == "" && len(m.tagValues[ctx]) > 0 {
			m.tagValues[ctx] = m.tagValues[ctx][:len(m.tagValues[ctx])-1]
			m.tagInputs[ctx] = ti
			return m, nil
		}
		// Fall through to textinput handler.

	case "tab":
		oldVal := ti.Value()
		newTi, cmd := ti.Update(msg)
		if newTi.Value() != oldVal {
			m.tagInputs[ctx] = newTi
			return m, cmd
		}
		m.advanceFocus()
		return m, nil

	case "shift+tab":
		m.tagInputs[ctx] = ti
		m.retreatFocus()
		return m, nil
	}

	var cmd tea.Cmd
	ti, cmd = ti.Update(msg)
	m.tagInputs[ctx] = ti
	return m, cmd
}

func (m EditModel) handleIngredientKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if m.ingRowCursor > 0 {
			m.ingRows[m.ingRowCursor] = m.blurIngRow(m.ingRows[m.ingRowCursor])
			m.ingRowCursor--
			m.ingRows[m.ingRowCursor] = m.focusIngCol(m.ingRows[m.ingRowCursor], m.ingColCursor)
		} else {
			// Exit ingredient section upward.
			m.ingRows[m.ingRowCursor] = m.blurIngRow(m.ingRows[m.ingRowCursor])
			m.retreatFocus()
		}
		return m, nil

	case "down":
		if m.ingRowCursor < len(m.ingRows)-1 {
			m.ingRows[m.ingRowCursor] = m.blurIngRow(m.ingRows[m.ingRowCursor])
			m.ingRowCursor++
			m.ingRows[m.ingRowCursor] = m.focusIngCol(m.ingRows[m.ingRowCursor], m.ingColCursor)
		} else {
			// Exit ingredient section downward.
			m.ingRows[m.ingRowCursor] = m.blurIngRow(m.ingRows[m.ingRowCursor])
			m.advanceFocus()
		}
		return m, nil

	case "ctrl+a":
		// Append new empty row.
		newRow := newIngredientRow()
		newRow.name.SetSuggestions(m.allIngNames)
		newRow.name.ShowSuggestions = true
		newRow.unit.SetSuggestions(m.allUnits)
		newRow.unit.ShowSuggestions = true
		m.ingRows[m.ingRowCursor] = m.blurIngRow(m.ingRows[m.ingRowCursor])
		m.ingRows = append(m.ingRows, newRow)
		m.ingRowCursor = len(m.ingRows) - 1
		m.ingColCursor = 0
		m.ingRows[m.ingRowCursor] = m.focusIngCol(m.ingRows[m.ingRowCursor], 0)
		return m, nil

	case "ctrl+d":
		if len(m.ingRows) > 1 {
			m.ingRows = append(m.ingRows[:m.ingRowCursor], m.ingRows[m.ingRowCursor+1:]...)
			if m.ingRowCursor >= len(m.ingRows) {
				m.ingRowCursor = len(m.ingRows) - 1
			}
			m.ingRows[m.ingRowCursor] = m.focusIngCol(m.ingRows[m.ingRowCursor], m.ingColCursor)
		}
		return m, nil

	case "tab":
		return m.handleIngredientTab(msg)

	case "shift+tab":
		return m.handleIngredientShiftTab(msg)

	default:
		// Forward to focused column.
		return m.updateIngCell(msg)
	}
}

func (m EditModel) handleIngredientTab(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	row := m.ingRows[m.ingRowCursor]
	inp := m.ingColInput(&row, m.ingColCursor)

	oldVal := inp.Value()
	newInp, cmd := inp.Update(msg)
	if newInp.Value() != oldVal {
		m.setIngColInput(&row, m.ingColCursor, newInp)
		m.ingRows[m.ingRowCursor] = row
		return m, cmd
	}

	// Advance column.
	if m.ingColCursor < 4 {
		m.setIngColInput(&row, m.ingColCursor, m.blurInput(newInp))
		m.ingColCursor++
		row = m.focusIngCol(row, m.ingColCursor)
		m.ingRows[m.ingRowCursor] = row
	} else {
		// Past last column — advance to next row or exit.
		m.ingRows[m.ingRowCursor] = m.blurIngRow(row)
		if m.ingRowCursor < len(m.ingRows)-1 {
			m.ingRowCursor++
			m.ingColCursor = 0
			m.ingRows[m.ingRowCursor] = m.focusIngCol(m.ingRows[m.ingRowCursor], 0)
		} else {
			m.advanceFocus()
		}
	}
	return m, nil
}

func (m EditModel) handleIngredientShiftTab(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	row := m.ingRows[m.ingRowCursor]
	if m.ingColCursor > 0 {
		row = m.blurIngRow(row)
		m.ingColCursor--
		row = m.focusIngCol(row, m.ingColCursor)
		m.ingRows[m.ingRowCursor] = row
	} else if m.ingRowCursor > 0 {
		m.ingRows[m.ingRowCursor] = m.blurIngRow(row)
		m.ingRowCursor--
		m.ingColCursor = 4
		m.ingRows[m.ingRowCursor] = m.focusIngCol(m.ingRows[m.ingRowCursor], 4)
	} else {
		m.ingRows[m.ingRowCursor] = m.blurIngRow(row)
		m.retreatFocus()
	}
	return m, nil
}

func (m EditModel) updateIngCell(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	row := m.ingRows[m.ingRowCursor]
	inp := m.ingColInput(&row, m.ingColCursor)
	var cmd tea.Cmd
	newInp, cmd := inp.Update(msg)
	m.setIngColInput(&row, m.ingColCursor, newInp)
	m.ingRows[m.ingRowCursor] = row
	return m, cmd
}

func (m EditModel) ingColInput(row *ingredientRow, col int) textinput.Model {
	switch col {
	case 0:
		return row.qty
	case 1:
		return row.unit
	case 2:
		return row.name
	case 3:
		return row.descriptor
	default:
		return row.section
	}
}

func (m EditModel) setIngColInput(row *ingredientRow, col int, inp textinput.Model) {
	switch col {
	case 0:
		row.qty = inp
	case 1:
		row.unit = inp
	case 2:
		row.name = inp
	case 3:
		row.descriptor = inp
	default:
		row.section = inp
	}
}

func (m EditModel) blurIngRow(row ingredientRow) ingredientRow {
	row.qty.Blur()
	row.unit.Blur()
	row.name.Blur()
	row.descriptor.Blur()
	row.section.Blur()
	return row
}

func (m EditModel) focusIngCol(row ingredientRow, col int) ingredientRow {
	row = m.blurIngRow(row)
	switch col {
	case 0:
		row.qty.Focus()
	case 1:
		row.unit.Focus()
	case 2:
		row.name.Focus()
	case 3:
		row.descriptor.Focus()
	default:
		row.section.Focus()
	}
	return row
}

func (m EditModel) blurInput(inp textinput.Model) textinput.Model {
	inp.Blur()
	return inp
}

func (m *EditModel) advanceFocus() {
	m.blurCurrent()
	m.focused = (m.focused + 1) % efCount
	m.focusCurrent()
	m.ensureFocusVisible()
}

func (m *EditModel) retreatFocus() {
	m.blurCurrent()
	if m.focused == 0 {
		m.focused = efCount - 1
	} else {
		m.focused--
	}
	m.focusCurrent()
	m.ensureFocusVisible()
}

// estimateFocusLine returns the approximate formLines index of the focused
// field, computed from the known per-section heights in buildForm.
//
// Each section's height (in newlines) depends on whether it is currently
// focused (bordered box, taller) or not (compact single-line / base height).
// The helper h(f, compact, expanded) returns compact when f != m.focused.
func (m EditModel) estimateFocusLine() int {
	h := func(f editFocus, compact, expanded int) int {
		if m.focused == f {
			return expanded
		}
		return compact
	}

	line := 1 // after opening \n in buildForm

	if m.focused == efName {
		return line
	}
	line += h(efName, 1, 3) // single-line field (+\n) or 3-line border box (+\n)

	if m.focused == efStatus {
		return line
	}
	line += h(efStatus, 2, 4) // single-line (+\n\n) or 3-line box (+\n\n)

	// "Description:" label occupies one line before the textarea.
	if m.focused == efDescription {
		return line + 1 // skip the label line to show the input
	}
	line += 1 + h(efDescription, 4, 6) // label\n + 3-line textarea+\n\n (or 5-line when focused)

	if m.focused == efPrepTime || m.focused == efCookTime {
		return line
	}
	line++ // "Prep: X  Cook: Y" inline row

	if m.focused == efServings || m.focused == efServingUnits {
		return line
	}
	line++ // "Servings: N units" inline row

	if m.sourceURL != "" {
		line += 2 // "Source URL: …\n\n"
	}

	if m.focused == efTagCourses {
		return line
	}
	line += h(efTagCourses, 1, 3)

	if m.focused == efTagCooking {
		return line
	}
	line += h(efTagCooking, 1, 3)

	if m.focused == efTagCultural {
		return line
	}
	line += h(efTagCultural, 1, 3)

	if m.focused == efTagDietary {
		return line
	}
	line += h(efTagDietary, 1, 3) + 1 // +1 for blank line after tag block

	if m.focused == efIngredients {
		return line
	}
	// Ingredients header (1) + col-header (1) + N rows + hint (1) + \n\n (2)
	line += 2 + len(m.ingRows) + 3

	if m.focused == efDirections {
		return line + 1 // skip "Directions:" label to show the textarea
	}

	return line
}

// ensureFocusVisible adjusts m.scroll so the focused field is within the viewport.
func (m *EditModel) ensureFocusVisible() {
	target := m.estimateFocusLine()
	vh := m.viewportHeight()
	if target < m.scroll {
		m.scroll = target
	} else if target >= m.scroll+vh {
		m.scroll = target - vh + 2
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m *EditModel) blurCurrent() {
	switch m.focused {
	case efName:
		m.nameInput.Blur()
	case efDescription:
		m.descInput.Blur()
	case efPrepTime:
		m.prepInput.Blur()
	case efCookTime:
		m.cookInput.Blur()
	case efServings:
		m.servingsInput.Blur()
	case efServingUnits:
		m.servingUnitsInput.Blur()
	case efTagCourses, efTagCooking, efTagCultural, efTagDietary:
		ctx := tagContextForFocus[m.focused]
		ti := m.tagInputs[ctx]
		ti.Blur()
		m.tagInputs[ctx] = ti
	case efIngredients:
		if m.ingRowCursor < len(m.ingRows) {
			m.ingRows[m.ingRowCursor] = m.blurIngRow(m.ingRows[m.ingRowCursor])
		}
	case efDirections:
		m.directionsInput.Blur()
	}
}

func (m *EditModel) focusCurrent() {
	switch m.focused {
	case efName:
		m.nameInput.Focus()
	case efDescription:
		m.descInput.Focus()
	case efPrepTime:
		m.prepInput.Focus()
	case efCookTime:
		m.cookInput.Focus()
	case efServings:
		m.servingsInput.Focus()
	case efServingUnits:
		m.servingUnitsInput.Focus()
	case efTagCourses, efTagCooking, efTagCultural, efTagDietary:
		ctx := tagContextForFocus[m.focused]
		ti := m.tagInputs[ctx]
		ti.Focus()
		m.tagInputs[ctx] = ti
	case efIngredients:
		if m.ingRowCursor < len(m.ingRows) {
			m.ingRows[m.ingRowCursor] = m.focusIngCol(m.ingRows[m.ingRowCursor], m.ingColCursor)
		}
	case efDirections:
		m.directionsInput.Focus()
	}
}

// formWidth returns the usable form content width.
func (m EditModel) formWidth() int {
	w := m.width - 4
	if w > 100 {
		w = 100
	}
	if w < 40 {
		w = 40
	}
	return w
}

// viewportHeight is the scrollable area height.
func (m EditModel) viewportHeight() int {
	// banner (4) + footer (2) + error line (1 if present)
	overhead := 7
	if m.errMsg != "" {
		overhead++
	}
	v := m.height - overhead
	if v < 4 {
		v = 4
	}
	return v
}

func (m EditModel) View() string {
	var sb strings.Builder

	// Banner.
	if m.isNew {
		sb.WriteString(renderEditBanner("New Recipe", m.width))
	} else if m.recipe != nil {
		sb.WriteString(renderEditBanner(m.recipe.Name, m.width))
	} else {
		sb.WriteString(renderEditBanner("Edit Recipe", m.width))
	}
	sb.WriteString("\n")

	// Build the full form as lines, then apply scroll.
	formLines := strings.Split(m.buildForm(), "\n")
	vh := m.viewportHeight()

	// Clamp scroll.
	maxScroll := len(formLines) - vh
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}
	if m.scroll < 0 {
		m.scroll = 0
	}

	end := m.scroll + vh
	if end > len(formLines) {
		end = len(formLines)
	}
	for i := m.scroll; i < end; i++ {
		sb.WriteString(formLines[i])
		sb.WriteString("\n")
	}
	// Pad remaining viewport.
	for i := end - m.scroll; i < vh; i++ {
		sb.WriteString("\n")
	}

	// Error message.
	if m.errMsg != "" {
		sb.WriteString(ErrorStyle.Render("  " + m.errMsg))
		sb.WriteString("\n")
	}

	// Footer.
	sb.WriteString(renderEditFooter(m.width))

	return sb.String()
}

// buildForm renders the complete form as a single string.
func (m EditModel) buildForm() string {
	var sb strings.Builder
	w := m.formWidth()

	focused := func(f editFocus) bool { return m.focused == f }

	// renderField renders a labelled text input.
	// When focused: label + input are rendered inside a bordered box with
	// MarginLeft(2) so every line of the box is indented consistently.
	// When unfocused: single-line "  label  input" (no multi-line risk).
	renderField := func(label string, inp textinput.Model, focus bool) string {
		lbl := MutedStyle.Width(14).Render(label)
		if focus {
			return lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1).
				Width(w - 6).
				MarginLeft(2).
				Render(lbl + inp.View())
		}
		return "  " + lbl + inp.View()
	}

	// renderInlineField highlights a short numeric/text field without adding a
	// border.  A border would make it multi-line and break the surrounding
	// single-line "Prep: X min  Cook: Y min" layout.
	renderInlineField := func(inp textinput.Model, focus bool) string {
		if focus {
			return lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(inp.View())
		}
		return inp.View()
	}

	sb.WriteString("\n")

	// Name.
	sb.WriteString(renderField("Name:", m.nameInput, focused(efName)))
	sb.WriteString("\n")

	// Status.
	left := MutedStyle.Render("◄")
	right := MutedStyle.Render("►")
	statusVal := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(editStatusOptions[m.statusIdx])
	statusLbl := MutedStyle.Width(14).Render("Status:")
	statusContent := statusLbl + left + " " + statusVal + " " + right
	if focused(efStatus) {
		sb.WriteString(lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1).
			MarginLeft(2).
			Render(statusContent))
	} else {
		sb.WriteString("  " + statusContent)
	}
	sb.WriteString("\n\n")

	// Description — already uses MarginLeft(2) so all lines are indented.
	sb.WriteString(MutedStyle.Render("  Description:"))
	sb.WriteString("\n")
	descBlock := lipgloss.NewStyle().
		MarginLeft(2).
		Width(w - 4)
	if focused(efDescription) {
		descBlock = descBlock.Border(lipgloss.NormalBorder()).BorderForeground(ColorPrimary)
	}
	sb.WriteString(descBlock.Render(m.descInput.View()))
	sb.WriteString("\n\n")

	// Prep / Cook — inline fields; use bold+color for focus to stay single-line.
	prepLbl := MutedStyle.Render("Prep: ")
	cookLbl := MutedStyle.Render("  Cook: ")
	minLbl := MutedStyle.Render(" min")
	sb.WriteString("  " + prepLbl +
		renderInlineField(m.prepInput, focused(efPrepTime)) + minLbl +
		cookLbl +
		renderInlineField(m.cookInput, focused(efCookTime)) + minLbl)
	sb.WriteString("\n")

	// Servings — same inline approach.
	servLbl := MutedStyle.Render("Servings: ")
	sb.WriteString("  " + servLbl +
		renderInlineField(m.servingsInput, focused(efServings)) +
		"  " +
		renderInlineField(m.servingUnitsInput, focused(efServingUnits)))
	sb.WriteString("\n")

	// Source URL — read-only; displayed only when present.
	if m.sourceURL != "" {
		lbl := MutedStyle.Width(14).Render("Source URL:")
		url := lipgloss.NewStyle().Foreground(ColorPrimary).Render(truncate(m.sourceURL, w-20))
		sb.WriteString("  " + lbl + url)
		sb.WriteString("\n\n")
	}

	// Tag sections — when focused, embed label + pills inside the bordered box
	// and use MarginLeft(2) so all border lines are indented consistently.
	tagFocuses := []struct {
		f   editFocus
		ctx string
		lbl string
	}{
		{efTagCourses, models.TagContextCourses, "Courses:"},
		{efTagCooking, models.TagContextCookingMethods, "Cooking:"},
		{efTagCultural, models.TagContextCulturalInfluences, "Cultural:"},
		{efTagDietary, models.TagContextDietaryRestrictions, "Dietary:"},
	}
	for _, tf := range tagFocuses {
		lbl := MutedStyle.Width(14).Render(tf.lbl)
		pills := m.renderTagPills(tf.ctx)
		ti := m.tagInputs[tf.ctx]
		if focused(tf.f) {
			sb.WriteString(lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1).
				MarginLeft(2).
				Render(lbl + pills + ti.View()))
		} else {
			sb.WriteString("  " + lbl + pills + ti.View())
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Ingredients section header.
	sepLine := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Render(strings.Repeat("─", w-4))
	sb.WriteString("  " + MutedStyle.Bold(true).Render("Ingredients") + " " + sepLine)
	sb.WriteString("\n")
	sb.WriteString(MutedStyle.Render(fmt.Sprintf(
		"  %-6s  %-10s  %-18s  %-12s  %-10s",
		"Qty", "Unit", "Name", "Descriptor", "Section",
	)))
	sb.WriteString("\n")

	for i, row := range m.ingRows {
		isRowFocused := m.focused == efIngredients && i == m.ingRowCursor
		sb.WriteString(m.renderIngRow(row, isRowFocused, i))
		sb.WriteString("\n")
	}
	sb.WriteString(MutedStyle.Render("  ctrl+a  add row   ctrl+d  remove row"))
	sb.WriteString("\n\n")

	// Directions — already uses MarginLeft(2).
	sb.WriteString(MutedStyle.Render("  Directions:"))
	sb.WriteString("\n")
	dirBlock := lipgloss.NewStyle().
		MarginLeft(2).
		Width(w - 4)
	if focused(efDirections) {
		dirBlock = dirBlock.Border(lipgloss.NormalBorder()).BorderForeground(ColorPrimary)
	}
	sb.WriteString(dirBlock.Render(m.directionsInput.View()))
	sb.WriteString("\n")

	return sb.String()
}

func (m EditModel) renderTagPills(ctx string) string {
	var sb strings.Builder
	for _, name := range m.tagValues[ctx] {
		sb.WriteString(TagStyle(ctx).Render(name))
	}
	return sb.String()
}

func (m EditModel) renderIngRow(row ingredientRow, rowFocused bool, rowIdx int) string {
	renderCol := func(inp textinput.Model, colIdx int, width int) string {
		isFocused := rowFocused && m.ingColCursor == colIdx
		v := inp.View()
		if isFocused {
			return lipgloss.NewStyle().
				Background(ColorHighlight).
				Foreground(lipgloss.Color("#2D1810")).
				Width(width).
				Render(v)
		}
		return lipgloss.NewStyle().Width(width).Render(v)
	}

	qty := renderCol(row.qty, 0, 6)
	unit := renderCol(row.unit, 1, 10)
	name := renderCol(row.name, 2, 18)
	desc := renderCol(row.descriptor, 3, 12)
	sect := renderCol(row.section, 4, 10)

	return "  " + qty + "  " + unit + "  " + name + "  " + desc + "  " + sect
}

// assembleRecipe reads all form inputs into a *models.Recipe.
func (m EditModel) assembleRecipe() (*models.Recipe, map[string][]string) {
	r := &models.Recipe{}
	if m.recipe != nil {
		r.ID = m.recipe.ID
		r.SourceText = m.recipe.SourceText
	}
	r.Name = strings.TrimSpace(m.nameInput.Value())
	r.Status = editStatusOptions[m.statusIdx]
	r.Description = strings.TrimSpace(m.descInput.Value())
	r.Directions = strings.TrimSpace(m.directionsInput.Value())
	r.SourceURL = m.sourceURL
	r.ServingUnits = strings.TrimSpace(m.servingUnitsInput.Value())

	if v, err := strconv.Atoi(strings.TrimSpace(m.prepInput.Value())); err == nil && v > 0 {
		r.PreparationTime = &v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(m.cookInput.Value())); err == nil && v > 0 {
		r.CookingTime = &v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(m.servingsInput.Value())); err == nil && v > 0 {
		r.Servings = &v
	}

	for i, row := range m.ingRows {
		name := strings.TrimSpace(row.name.Value())
		if name == "" {
			continue
		}
		r.Ingredients = append(r.Ingredients, models.RecipeIngredient{
			IngredientName: name,
			Quantity:       strings.TrimSpace(row.qty.Value()),
			Unit:           strings.TrimSpace(row.unit.Value()),
			Descriptor:     strings.TrimSpace(row.descriptor.Value()),
			Section:        strings.TrimSpace(row.section.Value()),
			Position:       i,
		})
	}

	tagNames := make(map[string][]string)
	for _, ctx := range models.AllTagContexts {
		tagNames[ctx] = m.tagValues[ctx]
	}

	return r, tagNames
}

// renderEditBanner renders the banner with "🍳  gorecipes  /  [name] / Edit".
func renderEditBanner(name string, width int) string {
	breadcrumb := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Render(
			"🍳  gorecipes  " +
				MutedStyle.Render("/") +
				"  " +
				lipgloss.NewStyle().
					Bold(false).
					Foreground(lipgloss.Color("#5C4A3C")).
					Render(truncate(name, width-30)+" / Edit"),
		)

	contentWidth := width - 6
	gap := contentWidth - lipgloss.Width(breadcrumb)
	if gap < 1 {
		gap = 1
	}

	title := lipgloss.NewStyle().
		Padding(1, 2).
		Render(breadcrumb + strings.Repeat(" ", gap))

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(title)
}

func renderEditFooter(width int) string {
	keys := []string{
		"⇥ tab next",
		"⇤ shift+tab back",
		"💾 ctrl+s save",
		"✖ esc cancel",
	}
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(footerLine(keys, width-2))
}

// RunEditUI runs the edit form. recipe=nil → blank new-recipe form.
// Returns toSave (non-nil when Ctrl+S pressed), tagNames, goHome, and error.
// The caller must call db.SaveRecipe(toSave, tagNames) when toSave != nil.
func RunEditUI(recipe *models.Recipe, data EditData) (
	toSave *models.Recipe,
	tagNames map[string][]string,
	goHome bool,
	err error,
) {
	m := newEditModel(recipe, data)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, runErr := p.Run()
	if runErr != nil {
		return nil, nil, false, runErr
	}
	fm := final.(EditModel)
	if fm.saved {
		r, tags := fm.assembleRecipe()
		return r, tags, false, nil
	}
	return nil, nil, fm.goHome, nil
}
