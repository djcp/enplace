package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// stepState tracks the status of one pipeline step.
type stepState int

const (
	stepPending  stepState = iota
	stepActive
	stepDone
	stepFailed
)

// progressStep is one row in the progress display.
type progressStep struct {
	label  string
	state  stepState
	detail string
}

// ProgressModel is a Bubbletea model for the 3-step add pipeline UI.
type ProgressModel struct {
	steps       []progressStep
	currentStep int
	done        bool
	err         error
	tick        int
	width       int

	// Channel over which pipeline sends step updates.
	stepCh <-chan StepUpdate
	doneCh <-chan error
}

// StepUpdate is sent by the pipeline goroutine to advance the progress UI.
type StepUpdate struct {
	Step  int
	Label string
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewProgressModel creates a progress model with the given step labels.
func NewProgressModel(stepLabels []string, stepCh <-chan StepUpdate, doneCh <-chan error) ProgressModel {
	steps := make([]progressStep, len(stepLabels))
	for i, label := range stepLabels {
		steps[i] = progressStep{label: label, state: stepPending}
	}
	return ProgressModel{
		steps:  steps,
		stepCh: stepCh,
		doneCh: doneCh,
		width:  60,
	}
}

type tickMsg struct{}
type stepMsg StepUpdate
type doneMsg struct{ err error }

func (m ProgressModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		waitForStep(m.stepCh),
		waitForDone(m.doneCh),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func waitForStep(ch <-chan StepUpdate) tea.Cmd {
	return func() tea.Msg {
		update, ok := <-ch
		if !ok {
			return nil
		}
		return stepMsg(update)
	}
}

func waitForDone(ch <-chan error) tea.Cmd {
	return func() tea.Msg {
		err := <-ch
		return doneMsg{err: err}
	}
}

func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tickMsg:
		m.tick++
		if !m.done {
			return m, tickCmd()
		}

	case stepMsg:
		step := msg.Step - 1 // convert to 0-indexed
		if step >= 0 && step < len(m.steps) {
			// Mark previous step done.
			if step > 0 {
				m.steps[step-1].state = stepDone
			}
			m.steps[step].state = stepActive
			m.currentStep = step
		}
		return m, waitForStep(m.stepCh)

	case doneMsg:
		m.done = true
		m.err = msg.err
		if msg.err == nil {
			// Mark all as done.
			for i := range m.steps {
				m.steps[i].state = stepDone
			}
		} else {
			// Mark current as failed.
			if m.currentStep < len(m.steps) {
				m.steps[m.currentStep].state = stepFailed
			}
		}
		return m, tea.Quit

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m ProgressModel) View() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(TitleStyle.Render("  Adding Recipe"))
	sb.WriteString("\n\n")

	for _, step := range m.steps {
		sb.WriteString(renderStep(step, m.tick))
		sb.WriteString("\n")
	}

	if m.done {
		sb.WriteString("\n")
		if m.err != nil {
			sb.WriteString(ErrorStyle.Render("  ✗ " + m.err.Error()))
		} else {
			sb.WriteString(SuccessStyle.Render("  ✓ Recipe saved!"))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderStep(step progressStep, tick int) string {
	var icon, labelStyle string

	switch step.state {
	case stepPending:
		icon = MutedStyle.Render("  ○")
		labelStyle = MutedStyle.Render(step.label)
	case stepActive:
		frame := spinnerFrames[tick%len(spinnerFrames)]
		icon = lipgloss.NewStyle().Foreground(ColorPrimary).Render("  " + frame)
		labelStyle = BoldStyle.Render(step.label)
	case stepDone:
		icon = SuccessStyle.Render("  ✓")
		labelStyle = step.label
	case stepFailed:
		icon = ErrorStyle.Render("  ✗")
		labelStyle = ErrorStyle.Render(step.label)
	}

	return fmt.Sprintf("%s  %s", icon, labelStyle)
}

// RunProgressUI runs the progress Bubbletea program and returns any error.
func RunProgressUI(stepLabels []string, stepCh <-chan StepUpdate, doneCh <-chan error) error {
	m := NewProgressModel(stepLabels, stepCh, doneCh)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
