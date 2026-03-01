package ui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PipelineLaunchFn is called by AddModel when the user submits the form.
// sourceURL and sourceText are mutually exclusive (one will be empty).
// onStep is invoked at the start of each pipeline step with its 1-based index and label.
// Returns the newly created recipe ID and any pipeline error.
type PipelineLaunchFn func(
	ctx context.Context,
	sourceURL, sourceText string,
	onStep func(step int, label string),
) (recipeID int64, err error)

type addPhase int

const (
	addPhaseMode     addPhase = iota // mode selection: URL / paste / manual
	addPhaseInput                    // user entering URL or paste text
	addPhaseProgress                 // pipeline running
	addPhaseResult                   // pipeline done (error); waiting for nav key
)

// pipelineResult carries the pipeline outcome back to the bubbletea event loop.
type pipelineResult struct {
	recipeID int64
	err      error
}

type addDoneMsg pipelineResult

func waitForAddDone(ch <-chan pipelineResult) tea.Cmd {
	return func() tea.Msg { return addDoneMsg(<-ch) }
}

// AddModel is a Bubbletea model for the full-screen add recipe flow.
// It shares the same banner / footer chrome as the list and detail views.
type AddModel struct {
	pasteMode bool
	launch    PipelineLaunchFn
	phase     addPhase

	// Mode selection cursor (addPhaseMode).
	modeIdx int

	// URL input (single-line, hand-rolled to match the search bar style).
	urlInput string
	inputErr string

	// Paste input.
	textarea textarea.Model

	// Progress steps, reusing the shared progressStep / renderStep helpers.
	steps       []progressStep
	currentStep int
	tick        int

	stepCh <-chan StepUpdate
	doneCh <-chan pipelineResult

	width  int
	height int

	// Outcome signals returned to the caller via RunAddUI.
	recipeID int64
	pipeErr  error
	goHome   bool
	goAdd    bool
	goManual bool
}

func newAddSteps(pasteMode bool) []progressStep {
	steps := []progressStep{
		{label: "Fetching recipe content", state: stepPending},
		{label: "Extracting with AI", state: stepPending},
		{label: "Saving to database", state: stepPending},
	}
	if pasteMode {
		steps[0].label = "Preparing text"
	}
	return steps
}

// NewAddModel constructs an AddModel.  If initialURL is non-empty the model
// skips the mode screen and immediately launches the pipeline.
// pasteMode=true skips the mode screen and goes to paste input.
func NewAddModel(pasteMode bool, initialURL string, fn PipelineLaunchFn) AddModel {
	m := AddModel{
		pasteMode: pasteMode,
		launch:    fn,
		steps:     newAddSteps(pasteMode),
		width:     80,
		height:    24,
	}
	if initialURL != "" {
		// URL provided on CLI — skip mode selection and go straight to pipeline.
		m.urlInput = initialURL
		m = m.startPipeline(initialURL, "")
	} else if pasteMode {
		// --paste flag — skip mode selection and go straight to paste input.
		ta := textarea.New()
		ta.Placeholder = "Paste the full recipe text here..."
		ta.ShowLineNumbers = false
		ta.Focus()
		m.textarea = ta
		m.phase = addPhaseInput
	} else {
		// No URL or paste mode — show mode selection screen.
		m.phase = addPhaseMode
	}
	return m
}

func (m AddModel) RecipeID() int64  { return m.recipeID }
func (m AddModel) GoHome() bool     { return m.goHome }
func (m AddModel) GoAdd() bool      { return m.goAdd }
func (m AddModel) GoManual() bool   { return m.goManual }
func (m AddModel) PipeErr() error   { return m.pipeErr }

func (m AddModel) Init() tea.Cmd {
	if m.phase == addPhaseProgress {
		return tea.Batch(waitForStep(m.stepCh), waitForAddDone(m.doneCh), tickCmd())
	}
	if m.phase == addPhaseInput && m.pasteMode {
		return textarea.Blink
	}
	return nil
}

// startPipeline sets up channels, launches the goroutine, and returns the
// model transitioned to addPhaseProgress.
func (m AddModel) startPipeline(sourceURL, sourceText string) AddModel {
	stepCh := make(chan StepUpdate, 8)
	doneCh := make(chan pipelineResult, 1)
	m.stepCh = stepCh
	m.doneCh = doneCh
	m.phase = addPhaseProgress

	firstLabel := m.steps[0].label
	launch := m.launch

	go func() {
		// Pre-signal step 1 so the UI shows activity immediately.
		// For URL: the pipeline will also signal it (idempotent).
		// For paste: the pipeline skips step 1, so this is the only signal.
		stepCh <- StepUpdate{Step: 1, Label: firstLabel}

		id, err := launch(context.Background(), sourceURL, sourceText, func(step int, label string) {
			stepCh <- StepUpdate{Step: step, Label: label}
		})
		close(stepCh)
		doneCh <- pipelineResult{recipeID: id, err: err}
	}()

	return m
}

func (m AddModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.pasteMode && m.phase == addPhaseInput {
			m.textarea.SetWidth(m.textareaWidth())
			m.textarea.SetHeight(m.textareaHeight())
		}

	case tickMsg:
		m.tick++
		if m.phase == addPhaseProgress {
			return m, tickCmd()
		}

	case stepMsg:
		step := msg.Step - 1 // convert to 0-indexed
		if step >= 0 && step < len(m.steps) {
			if step > 0 {
				m.steps[step-1].state = stepDone
			}
			m.steps[step].state = stepActive
			m.currentStep = step
		}
		return m, waitForStep(m.stepCh)

	case addDoneMsg:
		m.recipeID = msg.recipeID
		m.pipeErr = msg.err
		if msg.err == nil {
			for i := range m.steps {
				m.steps[i].state = stepDone
			}
			return m, tea.Quit // auto-quit on success; caller shows detail view
		}
		if m.currentStep < len(m.steps) {
			m.steps[m.currentStep].state = stepFailed
		}
		m.phase = addPhaseResult // stay alive so the user can navigate

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward non-key messages to the textarea (e.g. cursor blink).
	if m.phase == addPhaseInput && m.pasteMode {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m AddModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// During pipeline execution only ctrl+c is honoured (avoid orphaned goroutines).
	if m.phase == addPhaseProgress {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

	// After a pipeline failure: allow navigation.
	if m.phase == addPhaseResult {
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			m.goHome = true
			return m, tea.Quit
		case "h":
			m.goHome = true
			return m, tea.Quit
		case "a":
			m.goAdd = true
			return m, tea.Quit
		}
		return m, nil
	}

	// Mode selection screen.
	if m.phase == addPhaseMode {
		const modeCount = 3
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			m.goHome = true
			return m, tea.Quit
		case "up", "k":
			if m.modeIdx > 0 {
				m.modeIdx--
			}
		case "down", "j":
			if m.modeIdx < modeCount-1 {
				m.modeIdx++
			}
		case "enter", " ":
			switch m.modeIdx {
			case 0: // From a URL
				m.phase = addPhaseInput
				m.pasteMode = false
			case 1: // Paste recipe text
				ta := textarea.New()
				ta.Placeholder = "Paste the full recipe text here..."
				ta.ShowLineNumbers = false
				ta.Focus()
				m.textarea = ta
				m.pasteMode = true
				m.phase = addPhaseInput
				return m, textarea.Blink
			case 2: // Enter manually
				m.goManual = true
				return m, tea.Quit
			}
		}
		return m, nil
	}

	// Input phase — paste mode.
	if m.pasteMode {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+d":
			text := strings.TrimSpace(m.textarea.Value())
			if text == "" {
				m.inputErr = "Recipe text cannot be empty"
				return m, nil
			}
			m.inputErr = ""
			m = m.startPipeline("", text)
			return m, tea.Batch(waitForStep(m.stepCh), waitForAddDone(m.doneCh), tickCmd())
		case "esc":
			// Back to mode selection.
			m.phase = addPhaseMode
			return m, nil
		}
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}

	// Input phase — URL mode.
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		// Back to mode selection.
		m.phase = addPhaseMode
		return m, nil
	case "enter":
		url := strings.TrimSpace(m.urlInput)
		if url == "" {
			m.inputErr = "URL is required"
			return m, nil
		}
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			m.inputErr = "Must be a valid http:// or https:// URL"
			return m, nil
		}
		m.inputErr = ""
		m = m.startPipeline(url, "")
		return m, tea.Batch(waitForStep(m.stepCh), waitForAddDone(m.doneCh), tickCmd())
	case "backspace":
		if len(m.urlInput) > 0 {
			runes := []rune(m.urlInput)
			m.urlInput = string(runes[:len(runes)-1])
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.urlInput += string(msg.Runes)
		}
	}
	return m, nil
}

func (m AddModel) textareaWidth() int {
	w := m.width - 8
	if w < 20 {
		return 20
	}
	return w
}

func (m AddModel) textareaHeight() int {
	// Banner(4) + "\n"(1) + label(1) + "\n"(1) + "\n"(1) + footer(2) + slack(2) = 12
	h := m.height - 12
	if h < 3 {
		return 3
	}
	return h
}

func (m AddModel) View() string {
	if m.width == 0 {
		return ""
	}
	var sb strings.Builder

	sb.WriteString(renderAddBanner(m.width))
	sb.WriteString("\n")

	contentHeight := m.height - 9
	if contentHeight < 3 {
		contentHeight = 3
	}

	switch m.phase {
	case addPhaseProgress, addPhaseResult:
		sb.WriteString(m.viewProgress(contentHeight))
	case addPhaseMode:
		sb.WriteString(m.viewModeSelect(contentHeight))
	default:
		sb.WriteString(m.viewInput(contentHeight))
	}

	sb.WriteString("\n")
	sb.WriteString(renderAddFooter(m.pasteMode, m.phase, m.width))

	return sb.String()
}

var modeOptions = []string{
	"From a URL",
	"Paste recipe text",
	"Enter manually",
}

func (m AddModel) viewModeSelect(contentHeight int) string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(MutedStyle.Render("  How would you like to add this recipe?"))
	sb.WriteString("\n\n")

	for i, opt := range modeOptions {
		if i == m.modeIdx {
			sb.WriteString(lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true).
				Render("  ▶ " + opt))
		} else {
			sb.WriteString(MutedStyle.Render("    " + opt))
		}
		sb.WriteString("\n")
	}

	used := strings.Count(sb.String(), "\n")
	for i := used; i < contentHeight; i++ {
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m AddModel) viewInput(contentHeight int) string {
	var sb strings.Builder
	sb.WriteString("\n")

	if m.pasteMode {
		sb.WriteString(MutedStyle.Render("  Paste the recipe text below (ctrl+d to submit):"))
		sb.WriteString("\n\n")
		for _, line := range strings.Split(m.textarea.View(), "\n") {
			sb.WriteString("  ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	} else {
		prefix := MutedStyle.Render("  URL  ")
		cursor := lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(lipgloss.Color("#FFFFFF")).
			Render(" ")

		var inputContent string
		if m.urlInput != "" {
			inputContent = lipgloss.NewStyle().Foreground(ColorPrimary).Render(m.urlInput) + cursor
		} else {
			inputContent = MutedStyle.Render("https://...") + cursor
		}

		bar := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorPrimary).
			Width(m.width - 6).
			Padding(0, 1).
			MarginLeft(2).
			Render(prefix + inputContent)

		sb.WriteString(bar)
		sb.WriteString("\n")
	}

	if m.inputErr != "" {
		sb.WriteString("\n")
		sb.WriteString(ErrorStyle.Render("  " + m.inputErr))
		sb.WriteString("\n")
	}

	// Pad to fill the content area so the footer stays pinned.
	used := strings.Count(sb.String(), "\n")
	for i := used; i < contentHeight; i++ {
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m AddModel) viewProgress(contentHeight int) string {
	var sb strings.Builder
	sb.WriteString("\n")

	for _, step := range m.steps {
		sb.WriteString("  ")
		sb.WriteString(renderStep(step, m.tick))
		sb.WriteString("\n")
	}

	if m.phase == addPhaseResult && m.pipeErr != nil {
		sb.WriteString("\n")
		sb.WriteString(ErrorStyle.Render("  ✗ " + m.pipeErr.Error()))
		sb.WriteString("\n")
	}

	used := strings.Count(sb.String(), "\n")
	for i := used; i < contentHeight; i++ {
		sb.WriteString("\n")
	}
	return sb.String()
}

// renderAddBanner renders a "🍳  gorecipes / Add Recipe" banner.
func renderAddBanner(width int) string {
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
					Render("Add Recipe"),
		)

	title := lipgloss.NewStyle().
		Padding(1, 2).
		Render(breadcrumb)

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(title)
}

// renderAddFooter renders keybinding hints appropriate for the current phase.
func renderAddFooter(pasteMode bool, phase addPhase, width int) string {
	var keys []string
	switch phase {
	case addPhaseProgress:
		keys = []string{"ctrl+c quit"}
	case addPhaseResult:
		keys = []string{"a add another", "h home", "q quit"}
	case addPhaseMode:
		keys = []string{"↑/↓ select", "enter confirm", "esc back"}
	default:
		if pasteMode {
			keys = []string{"ctrl+d submit", "esc back"}
		} else {
			keys = []string{"enter submit", "esc back"}
		}
	}
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(footerLine(keys, width-2))
}

// RunAddUI runs the full-screen add recipe TUI.
// Returns the new recipe ID (>0 on pipeline success), navigation signals, and any pipeline error.
// A non-nil pipeErr means the error was already shown in the TUI.
func RunAddUI(pasteMode bool, initialURL string, fn PipelineLaunchFn) (recipeID int64, goHome bool, goAdd bool, goManual bool, pipeErr error) {
	m := NewAddModel(pasteMode, initialURL, fn)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return 0, false, false, false, err
	}
	fm := final.(AddModel)
	return fm.RecipeID(), fm.GoHome(), fm.GoAdd(), fm.GoManual(), fm.PipeErr()
}
