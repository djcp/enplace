package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/gorecipes/internal/config"
)

type configFocus int

const (
	cfCredits configFocus = iota
	cfAPIKey
	cfModel
	cfCount
)

var modelOptions = []string{
	"claude-haiku-4-5-20251001",
	"claude-sonnet-4-6",
	"claude-opus-4-6",
}

// ConfigModel is a full-screen interactive configuration editor.
type ConfigModel struct {
	cfg        *config.Config
	configPath string
	width      int
	height     int

	focus configFocus

	creditsInput textinput.Model
	apiKeyInput  textinput.Model
	modelIdx     int // index into modelOptions

	saved bool
}

func newConfigModel(cfg *config.Config, configPath string) ConfigModel {
	m := ConfigModel{
		cfg:        cfg,
		configPath: configPath,
		width:      80,
		height:     24,
	}

	ci := textinput.New()
	ci.Placeholder = "e.g. Chef Jane Smith · myrecipeblog.com"
	ci.SetValue(cfg.Credits)
	ci.Focus()
	m.creditsInput = ci

	ai := textinput.New()
	ai.Placeholder = "sk-ant-..."
	ai.EchoMode = textinput.EchoPassword
	ai.SetValue(cfg.AnthropicAPIKey)
	m.apiKeyInput = ai

	for i, opt := range modelOptions {
		if opt == cfg.AnthropicModel {
			m.modelIdx = i
			break
		}
	}

	m.updateInputWidths()
	return m
}

func (m *ConfigModel) updateInputWidths() {
	w := m.width - 12
	if w > 68 {
		w = 68
	}
	if w < 20 {
		w = 20
	}
	m.creditsInput.Width = w
	m.apiKeyInput.Width = w
}

// Saved returns true if the user pressed ctrl+s to save changes.
func (m ConfigModel) Saved() bool { return m.saved }

func (m ConfigModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m ConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateInputWidths()
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward non-key messages (e.g. cursor blink) to the active input.
	var cmd tea.Cmd
	switch m.focus {
	case cfCredits:
		m.creditsInput, cmd = m.creditsInput.Update(msg)
	case cfAPIKey:
		m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
	}
	return m, cmd
}

func (m ConfigModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit

	case "ctrl+s":
		return m.doSave()

	case "tab", "down":
		m = m.advanceFocus(1)
		return m, textinput.Blink

	case "shift+tab", "up":
		m = m.advanceFocus(-1)
		return m, textinput.Blink

	case "left":
		if m.focus == cfModel {
			if m.modelIdx > 0 {
				m.modelIdx--
			}
			return m, nil
		}

	case "right":
		if m.focus == cfModel {
			if m.modelIdx < len(modelOptions)-1 {
				m.modelIdx++
			}
			return m, nil
		}
	}

	// Forward all other keys to the active textinput.
	var cmd tea.Cmd
	switch m.focus {
	case cfCredits:
		m.creditsInput, cmd = m.creditsInput.Update(msg)
	case cfAPIKey:
		m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
	}
	return m, cmd
}

func (m ConfigModel) advanceFocus(dir int) ConfigModel {
	switch m.focus {
	case cfCredits:
		m.creditsInput.Blur()
	case cfAPIKey:
		m.apiKeyInput.Blur()
	}

	m.focus = configFocus((int(m.focus) + dir + int(cfCount)) % int(cfCount))

	switch m.focus {
	case cfCredits:
		m.creditsInput.Focus()
	case cfAPIKey:
		m.apiKeyInput.Focus()
	}
	return m
}

func (m ConfigModel) doSave() (tea.Model, tea.Cmd) {
	m.cfg.Credits = strings.TrimSpace(m.creditsInput.Value())
	m.cfg.AnthropicAPIKey = strings.TrimSpace(m.apiKeyInput.Value())
	m.cfg.AnthropicModel = modelOptions[m.modelIdx]
	m.saved = true
	return m, tea.Quit
}

func (m ConfigModel) View() string {
	if m.width == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(renderConfigBanner(m.width))
	sb.WriteString("\n\n")

	inputBoxStyle := func(focused bool) lipgloss.Style {
		bc := ColorBorder
		if focused {
			bc = ColorPrimary
		}
		return lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(bc).
			Padding(0, 1).
			MarginLeft(4)
	}

	fieldLabel := func(label string, focused bool) string {
		if focused {
			return lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(label)
		}
		return MutedStyle.Render(label)
	}

	// Credits
	sb.WriteString("    " + fieldLabel("Credits", m.focus == cfCredits) + "\n")
	sb.WriteString(inputBoxStyle(m.focus == cfCredits).Render(m.creditsInput.View()) + "\n")
	sb.WriteString("    " + MutedStyle.Render("Claim recipe authorship — included in the footer of exported files.") + "\n")
	sb.WriteString("\n")

	// API Key
	sb.WriteString("    " + fieldLabel("API Key", m.focus == cfAPIKey) + "\n")
	sb.WriteString(inputBoxStyle(m.focus == cfAPIKey).Render(m.apiKeyInput.View()) + "\n")
	sb.WriteString("\n")

	// Model
	sb.WriteString("    " + fieldLabel("AI Model", m.focus == cfModel) + "\n")
	var modelRow string
	if m.focus == cfModel {
		modelRow = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render("◀ " + modelOptions[m.modelIdx] + " ▶")
	} else {
		modelRow = MutedStyle.Render("◀ " + modelOptions[m.modelIdx] + " ▶")
	}
	sb.WriteString("    " + modelRow + "\n")
	sb.WriteString("\n")

	// Read-only info
	labelW := 12
	infoRow := func(label, value string) string {
		l := lipgloss.NewStyle().Foreground(ColorMuted).Width(labelW).Render(label)
		v := lipgloss.NewStyle().Foreground(ColorFaint).Render(value)
		return "    " + l + "  " + v
	}
	sb.WriteString(infoRow("Config file", m.configPath) + "\n")
	sb.WriteString(infoRow("Database", m.cfg.DBPath) + "\n")

	// Fill remaining rows so the footer stays pinned.
	used := strings.Count(sb.String(), "\n")
	if fill := m.height - used - 3; fill > 0 {
		sb.WriteString(strings.Repeat("\n", fill))
	}
	sb.WriteString("\n")
	sb.WriteString(renderConfigFooter(m.width))

	return sb.String()
}

func renderConfigBanner(width int) string {
	breadcrumb := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Render(
			"🍳  gorecipes  " +
				MutedStyle.Render("/") +
				"  " +
				lipgloss.NewStyle().
					Bold(false).
					Foreground(ColorSubtle).
					Render("configure"),
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

func renderConfigFooter(width int) string {
	keys := []string{
		MutedStyle.Render("tab next"),
		MutedStyle.Render("shift+tab prev"),
		MutedStyle.Render("◀/▶ change model"),
		lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Render("ctrl+s save"),
		MutedStyle.Render("esc cancel"),
	}
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorBorder).
		Width(width - 2).
		Render(footerLine(keys, width-2))
}

// RunConfigUI opens the interactive configuration editor.
// If the user saves, cfg's fields are updated in-place and saved=true is returned;
// the caller should call cfg.Save() to persist the changes to disk.
func RunConfigUI(cfg *config.Config, configPath string) (saved bool, err error) {
	m := newConfigModel(cfg, configPath)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, runErr := p.Run()
	if runErr != nil {
		return false, runErr
	}
	fm := final.(ConfigModel)
	return fm.Saved(), nil
}
