package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/enplace/internal/models"
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
	efIsBread
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
	qty  textinput.Model
	unit textinput.Model
	name textinput.Model
	// descriptor, section, and ingType are edited via the detail overlay (enter key).
	descriptor textinput.Model
	section    textinput.Model
	ingType    textinput.Model // "wet", "dry", "starter", or ""
}

func newIngredientRow() ingredientRow {
	qty := textinput.New()
	qty.Placeholder = "qty"
	qty.Width = 8

	unit := textinput.New()
	unit.Placeholder = "unit"
	unit.Width = 13

	name := textinput.New()
	name.Placeholder = "ingredient"
	name.Width = 23

	desc := textinput.New()
	desc.Placeholder = "descriptor"
	desc.Width = 28

	sect := textinput.New()
	sect.Placeholder = "section"
	sect.Width = 20

	typ := textinput.New()
	typ.Placeholder = "wet / dry / starter / (blank)"
	typ.Width = 20

	return ingredientRow{qty: qty, unit: unit, name: name, descriptor: desc, section: sect, ingType: typ}
}

func populateIngredientRow(ri models.RecipeIngredient) ingredientRow {
	row := newIngredientRow()
	row.qty.SetValue(ri.Quantity)
	row.qty.CursorStart()
	row.unit.SetValue(ri.Unit)
	row.unit.CursorStart()
	row.name.SetValue(ri.IngredientName)
	row.name.CursorStart()
	row.descriptor.SetValue(ri.Descriptor)
	row.descriptor.CursorStart()
	row.section.SetValue(ri.Section)
	row.section.CursorStart()
	row.ingType.SetValue(ri.IngredientType)
	row.ingType.CursorStart()
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

	isBread bool

	ingRows      []ingredientRow
	ingRowCursor int
	ingColCursor int // 0–2 (qty, unit, name); descriptor/section/type via overlay

	// Ingredient detail overlay (opened with enter on any ingredient row).
	ingDetailOpen  bool
	ingDetailFocus int // 0=descriptor, 1=section, 2=type
	ingDetailErr   string

	allIngNames []string
	allUnits    []string
	allTags     map[string][]string

	focused editFocus
	width   int
	height  int

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
		m.isBread = recipe.IsBread
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
	// Ingredient detail overlay captures all keys when open.
	if m.ingDetailOpen {
		return m.handleIngDetailKey(msg)
	}

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
	case "esc":
		m.goHome = true
		return m, tea.Quit
	case "ctrl+c":
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
		case "tab", "down":
			m.advanceFocus()
		case "shift+tab", "up":
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

	case efIsBread:
		switch msg.String() {
		case "left", "right", "h", "l", " ":
			m.isBread = !m.isBread
		case "tab", "down":
			m.advanceFocus()
		case "shift+tab", "up":
			m.retreatFocus()
		}
		return m, nil

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
	case "tab", "down":
		oldVal := inp.Value()
		newInp, cmd := inp.Update(msg)
		if msg.String() == "tab" && newInp.Value() != oldVal {
			// Tab accepted a suggestion — stay on this field.
			return m, newInp, cmd
		}
		m.advanceFocus()
		return m, inp, nil
	case "shift+tab", "up":
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

	case "enter":
		// Open the detail overlay for descriptor, section, and type.
		m.ingDetailOpen = true
		m.ingDetailFocus = 0
		m.ingDetailErr = ""
		row := m.ingRows[m.ingRowCursor]
		row = m.focusIngDetailField(row, 0)
		m.ingRows[m.ingRowCursor] = row
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

	// Advance column (main row has 3 columns: qty=0, unit=1, name=2).
	if m.ingColCursor < 2 {
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

func (m EditModel) handleIngredientShiftTab(_ tea.KeyMsg) (tea.Model, tea.Cmd) {
	row := m.ingRows[m.ingRowCursor]
	if m.ingColCursor > 0 {
		row = m.blurIngRow(row)
		m.ingColCursor--
		row = m.focusIngCol(row, m.ingColCursor)
		m.ingRows[m.ingRowCursor] = row
	} else if m.ingRowCursor > 0 {
		m.ingRows[m.ingRowCursor] = m.blurIngRow(row)
		m.ingRowCursor--
		m.ingColCursor = 2
		m.ingRows[m.ingRowCursor] = m.focusIngCol(m.ingRows[m.ingRowCursor], 2)
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

// ingColInput returns the input for the given main-row column (0=qty, 1=unit, 2=name).
func (m EditModel) ingColInput(row *ingredientRow, col int) textinput.Model {
	switch col {
	case 0:
		return row.qty
	case 1:
		return row.unit
	default:
		return row.name
	}
}

// setIngColInput sets the input for the given main-row column.
func (m EditModel) setIngColInput(row *ingredientRow, col int, inp textinput.Model) {
	switch col {
	case 0:
		row.qty = inp
	case 1:
		row.unit = inp
	default:
		row.name = inp
	}
}

func (m EditModel) blurIngRow(row ingredientRow) ingredientRow {
	row.qty.Blur()
	row.unit.Blur()
	row.name.Blur()
	row.descriptor.Blur()
	row.section.Blur()
	row.ingType.Blur()
	return row
}

func (m EditModel) focusIngCol(row ingredientRow, col int) ingredientRow {
	row = m.blurIngRow(row)
	switch col {
	case 0:
		row.qty.Focus()
	case 1:
		row.unit.Focus()
	default:
		row.name.Focus()
	}
	return row
}

func (m EditModel) blurInput(inp textinput.Model) textinput.Model {
	inp.Blur()
	return inp
}

// handleIngDetailKey processes keys while the ingredient detail overlay is open.
func (m EditModel) handleIngDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	row := m.ingRows[m.ingRowCursor]

	switch msg.String() {
	case "esc":
		// Close without error — values already live in row inputs.
		m.ingDetailOpen = false
		m.ingDetailErr = ""
		row = m.blurIngRow(row)
		row = m.focusIngCol(row, m.ingColCursor)
		m.ingRows[m.ingRowCursor] = row
		return m, nil

	case "tab", "down":
		if m.ingDetailFocus < 2 {
			m.ingDetailFocus++
			row = m.focusIngDetailField(row, m.ingDetailFocus)
			m.ingRows[m.ingRowCursor] = row
		} else {
			// Last field — close the overlay.
			if err := m.validateIngDetailType(row); err != "" {
				m.ingDetailErr = err
				m.ingRows[m.ingRowCursor] = row
				return m, nil
			}
			m.ingDetailOpen = false
			m.ingDetailErr = ""
			row = m.blurIngRow(row)
			row = m.focusIngCol(row, m.ingColCursor)
			m.ingRows[m.ingRowCursor] = row
		}
		return m, nil

	case "shift+tab", "up":
		if m.ingDetailFocus > 0 {
			m.ingDetailFocus--
			row = m.focusIngDetailField(row, m.ingDetailFocus)
			m.ingRows[m.ingRowCursor] = row
		}
		return m, nil

	case "enter":
		if err := m.validateIngDetailType(row); err != "" {
			m.ingDetailErr = err
			m.ingRows[m.ingRowCursor] = row
			return m, nil
		}
		m.ingDetailOpen = false
		m.ingDetailErr = ""
		row = m.blurIngRow(row)
		row = m.focusIngCol(row, m.ingColCursor)
		m.ingRows[m.ingRowCursor] = row
		return m, nil
	}

	// Forward to the focused detail input.
	var cmd tea.Cmd
	switch m.ingDetailFocus {
	case 0:
		row.descriptor, cmd = row.descriptor.Update(msg)
	case 1:
		row.section, cmd = row.section.Update(msg)
	case 2:
		row.ingType, cmd = row.ingType.Update(msg)
		m.ingDetailErr = "" // clear error on edit
	}
	m.ingRows[m.ingRowCursor] = row
	return m, cmd
}

func (m EditModel) focusIngDetailField(row ingredientRow, field int) ingredientRow {
	row.descriptor.Blur()
	row.section.Blur()
	row.ingType.Blur()
	switch field {
	case 0:
		row.descriptor.Focus()
	case 1:
		row.section.Focus()
	case 2:
		row.ingType.Focus()
	}
	return row
}

func (m EditModel) validateIngDetailType(row ingredientRow) string {
	v := strings.TrimSpace(row.ingType.Value())
	if v != "" && v != "wet" && v != "dry" && v != "starter" {
		return `type must be "wet", "dry", "starter", or blank`
	}
	return ""
}

// renderIngDetailOverlay renders the ingredient detail overlay dialog.
func (m EditModel) renderIngDetailOverlay() string {
	row := m.ingRows[m.ingRowCursor]

	renderDetailField := func(label string, inp textinput.Model, focused bool) string {
		lbl := MutedStyle.Width(12).Render(label)
		if focused {
			return lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1).
				Width(46).
				Render(lbl + inp.View())
		}
		return "  " + lbl + inp.View()
	}

	var sb strings.Builder
	sb.WriteString(renderDetailField("Descriptor:", row.descriptor, m.ingDetailFocus == 0))
	sb.WriteString("\n")
	sb.WriteString(renderDetailField("Section:", row.section, m.ingDetailFocus == 1))
	sb.WriteString("\n")
	sb.WriteString(renderDetailField("Type:", row.ingType, m.ingDetailFocus == 2))

	inner := sb.String()
	if m.ingDetailErr != "" {
		inner += "\n" + ErrorStyle.Render("  "+m.ingDetailErr)
	}

	name := ""
	if m.ingRowCursor < len(m.ingRows) {
		name = strings.TrimSpace(m.ingRows[m.ingRowCursor].name.Value())
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("Ingredient Detail")
	if name != "" {
		title += MutedStyle.Render(" — " + name)
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Render(title + "\n\n" + inner + "\n\n" +
			MutedStyle.Render("tab/↓ next   shift+tab/↑ prev   enter save   esc cancel"))

	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, box)
}

func (m *EditModel) advanceFocus() {
	m.blurCurrent()
	m.focused = (m.focused + 1) % efCount
	m.focusCurrent()
}

func (m *EditModel) retreatFocus() {
	m.blurCurrent()
	if m.focused == 0 {
		m.focused = efCount - 1
	} else {
		m.focused--
	}
	m.focusCurrent()
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

	// Ingredient detail overlay — rendered on top of the form.
	if m.ingDetailOpen {
		sb.WriteString("\n")
		overlay := m.renderIngDetailOverlay()
		overlayLines := strings.Count(overlay, "\n") + 1
		sb.WriteString(overlay)
		// Pad remaining space then show footer.
		used := 2 + overlayLines // banner(~4 lines) + \n + overlay
		if fill := m.height - used - 3; fill > 0 {
			sb.WriteString(strings.Repeat("\n", fill))
		}
		sb.WriteString("\n")
		sb.WriteString(renderEditFooter(m.width))
		return sb.String()
	}
	sb.WriteString("\n")

	// Build the full form as lines, tracking exactly where the focused field is.
	form, focusLine := m.buildForm()
	formLines := strings.Split(form, "\n")
	vh := m.viewportHeight()

	// Compute scroll so the focused field lands roughly one-third from the top.
	scroll := focusLine - vh/3
	if scroll < 0 {
		scroll = 0
	}
	maxScroll := len(formLines) - vh
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}

	end := scroll + vh
	if end > len(formLines) {
		end = len(formLines)
	}
	for i := scroll; i < end; i++ {
		sb.WriteString(formLines[i])
		sb.WriteString("\n")
	}
	// Pad remaining viewport.
	for i := end - scroll; i < vh; i++ {
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

// buildForm renders the complete form as a single string and returns the line
// index where the focused field begins. Line tracking is exact: every \n
// written through the write() helper increments the counter, so focusLine
// always points to the actual rendered position of the active field.
func (m EditModel) buildForm() (string, int) {
	var sb strings.Builder
	w := m.formWidth()
	focused := func(f editFocus) bool { return m.focused == f }

	lineCount := 0
	focusLine := 0

	// write appends s to the builder and counts its newlines.
	write := func(s string) {
		sb.WriteString(s)
		lineCount += strings.Count(s, "\n")
	}

	// markFocus records the current line as the start of field f's content,
	// but only when f is the field that is currently focused.
	markFocus := func(f editFocus) {
		if m.focused == f {
			focusLine = lineCount
		}
	}

	// renderField renders a labelled text input.
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

	// renderInlineField highlights a short field without a border.
	renderInlineField := func(inp textinput.Model, focus bool) string {
		if focus {
			return lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(inp.View())
		}
		return inp.View()
	}

	write("\n")

	// Name.
	markFocus(efName)
	write(renderField("Name:", m.nameInput, focused(efName)))
	write("\n")

	// Status.
	left := MutedStyle.Render("◄")
	right := MutedStyle.Render("►")
	statusVal := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(editStatusOptions[m.statusIdx])
	statusLbl := MutedStyle.Width(14).Render("Status:")
	statusContent := statusLbl + left + " " + statusVal + " " + right
	markFocus(efStatus)
	if focused(efStatus) {
		write(lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1).
			MarginLeft(2).
			Render(statusContent))
	} else {
		write("  " + statusContent)
	}
	write("\n\n")

	// Description — label on its own line, then the textarea.
	write(MutedStyle.Render("  Description:") + "\n")
	markFocus(efDescription) // mark after label so focus shows the input
	descBlock := lipgloss.NewStyle().
		MarginLeft(2).
		Width(w - 4)
	if focused(efDescription) {
		descBlock = descBlock.Border(lipgloss.NormalBorder()).BorderForeground(ColorPrimary)
	}
	write(descBlock.Render(m.descInput.View()))
	write("\n\n")

	// Source URL — read-only; displayed only when present.
	if m.sourceURL != "" {
		lbl := MutedStyle.Width(14).Render("Source URL:")
		url := lipgloss.NewStyle().Foreground(ColorPrimary).Render(truncate(m.sourceURL, w-20))
		write("  " + lbl + url + "\n\n")
	}

	// Prep / Cook — inline fields; use bold+color for focus to stay single-line.
	prepLbl := MutedStyle.Render("Prep: ")
	cookLbl := MutedStyle.Render("  Cook: ")
	minLbl := MutedStyle.Render(" min")
	markFocus(efPrepTime)
	markFocus(efCookTime) // same rendered line
	write("  " + prepLbl +
		renderInlineField(m.prepInput, focused(efPrepTime)) + minLbl +
		cookLbl +
		renderInlineField(m.cookInput, focused(efCookTime)) + minLbl)
	write("\n")

	// Servings — same inline approach.
	servLbl := MutedStyle.Render("Servings: ")
	markFocus(efServings)
	markFocus(efServingUnits) // same rendered line
	write("  " + servLbl +
		renderInlineField(m.servingsInput, focused(efServings)) +
		"  " +
		renderInlineField(m.servingUnitsInput, focused(efServingUnits)))
	write("\n")

	// Bread / dough toggle.
	isBreadLbl := MutedStyle.Width(14).Render("Bread/dough:")
	isBreadVal := "no"
	if m.isBread {
		isBreadVal = "yes"
	}
	isBreadValStr := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(isBreadVal)
	isBreadContent := "  " + isBreadLbl + left + " " + isBreadValStr + " " + right
	markFocus(efIsBread)
	if focused(efIsBread) {
		write(lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1).
			Render(isBreadContent))
	} else {
		write(isBreadContent)
	}
	write("\n")

	// Tag sections.
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
		markFocus(tf.f)
		if focused(tf.f) {
			write(lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1).
				MarginLeft(2).
				Render(lbl + pills + ti.View()))
		} else {
			write("  " + lbl + pills + ti.View())
		}
		write("\n")
	}
	write("\n")

	// Ingredients section header.
	sepLine := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Render(strings.Repeat("─", w-4))
	write("  " + MutedStyle.Bold(true).Render("Ingredients") + " " + sepLine + "\n")
	write(MutedStyle.Render(fmt.Sprintf(
		"  %-8s  %-13s  %-23s  %s",
		"Qty", "Unit", "Name", "Details (enter to edit)",
	)) + "\n")

	for i, row := range m.ingRows {
		isRowFocused := m.focused == efIngredients && i == m.ingRowCursor
		// For the ingredients section, track the exact cursor row instead of
		// the section header so scroll brings the active row into view.
		if m.focused == efIngredients && i == m.ingRowCursor {
			focusLine = lineCount
		}
		write(m.renderIngRow(row, isRowFocused, i) + "\n")
	}
	write(MutedStyle.Render("  ctrl+a  add row   ctrl+d  remove row") + "\n\n")

	// Directions — label on its own line, then the textarea.
	write(MutedStyle.Render("  Directions:") + "\n")
	markFocus(efDirections) // mark after label
	dirBlock := lipgloss.NewStyle().
		MarginLeft(2).
		Width(w - 4)
	if focused(efDirections) {
		dirBlock = dirBlock.Border(lipgloss.NormalBorder()).BorderForeground(ColorPrimary)
	}
	write(dirBlock.Render(m.directionsInput.View()))
	write("\n")

	return sb.String(), focusLine
}

func (m EditModel) renderTagPills(ctx string) string {
	var sb strings.Builder
	for _, name := range m.tagValues[ctx] {
		sb.WriteString(TagStyle(ctx).Render(name))
	}
	return sb.String()
}

func (m EditModel) renderIngRow(row ingredientRow, rowFocused bool, _ int) string {
	renderCol := func(inp textinput.Model, colIdx int, width int) string {
		isFocused := rowFocused && m.ingColCursor == colIdx
		v := inp.View()
		if isFocused {
			return lipgloss.NewStyle().
				Background(ColorHighlight).
				Foreground(ColorHighlightFg).
				Width(width).
				Render(v)
		}
		return lipgloss.NewStyle().Width(width).Render(v)
	}

	qty := renderCol(row.qty, 0, 8)
	unit := renderCol(row.unit, 1, 13)
	name := renderCol(row.name, 2, 23)

	// Read-only detail summary: show any set values from the overlay fields.
	var details []string
	if v := strings.TrimSpace(row.descriptor.Value()); v != "" {
		details = append(details, v)
	}
	if v := strings.TrimSpace(row.section.Value()); v != "" {
		details = append(details, "§"+v)
	}
	if v := strings.TrimSpace(row.ingType.Value()); v != "" {
		details = append(details, "["+v+"]")
	}
	detailStr := ""
	if len(details) > 0 {
		detailStr = MutedStyle.Render(strings.Join(details, "  "))
	}

	return "  " + qty + "  " + unit + "  " + name + "  " + detailStr
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
	r.IsBread = m.isBread

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
			IngredientType: strings.TrimSpace(row.ingType.Value()),
			Position:       i,
		})
	}

	tagNames := make(map[string][]string)
	for _, ctx := range models.AllTagContexts {
		tagNames[ctx] = m.tagValues[ctx]
	}

	return r, tagNames
}

// renderEditBanner renders the banner with "🍳  enplace  /  [name] / Edit".
func renderEditBanner(name string, width int) string {
	breadcrumb := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Render(
			"🍳  enplace  " +
				MutedStyle.Render("/") +
				"  " +
				lipgloss.NewStyle().
					Bold(false).
					Foreground(ColorSubtle).
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
		"↑↓/⇥ tab next",
		"↓↑/⇤ shift+tab back",
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
