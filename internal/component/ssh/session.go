// Design: (none -- new SSH server component)
// Overview: ssh.go -- SSH server lifecycle

package ssh

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SessionModel is a minimal Bubble Tea model for SSH sessions.
// It provides a read-only command prompt with optional daemon command execution.
// Uses pointer receivers to avoid copying the large embedded bubble components.
type SessionModel struct {
	textInput textinput.Model
	viewport  viewport.Model
	width     int
	height    int
	quitting  bool
	output    string
	history   []string
	histIdx   int
	histTmp   string

	// Command execution — injected directly by the daemon, no socket needed.
	commandExecutor CommandExecutor
}

// NewSessionModel creates a new session model for SSH.
// The model starts in a command-mode-like interface.
// If executor is non-nil, operational commands are dispatched through it.
func NewSessionModel(executor CommandExecutor) *SessionModel {
	ti := textinput.New()
	ti.Placeholder = "type command (exit to quit)"
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = 120

	vp := viewport.New(120, 20)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	return &SessionModel{
		textInput:       ti,
		viewport:        vp,
		output:          "SSH session ready. Type 'help' for available commands, 'exit' to disconnect.",
		histIdx:         -1,
		commandExecutor: executor,
	}
}

// Init implements tea.Model.
func (m *SessionModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m *SessionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.Width = msg.Width - 4
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = max(msg.Height-6, 5)
		m.viewport.SetContent(m.output)
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m *SessionModel) View() string {
	if m.quitting {
		return "Goodbye.\n"
	}

	var b strings.Builder
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	b.WriteString(m.textInput.View())
	b.WriteString("\n")
	return b.String()
}

func (m *SessionModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type { //nolint:exhaustive // only handle specific keys
	case tea.KeyCtrlC, tea.KeyEsc:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyEnter:
		return m.handleEnter()

	case tea.KeyUp:
		if len(m.history) == 0 {
			return m, nil
		}
		if m.histIdx == -1 {
			m.histTmp = m.textInput.Value()
			m.histIdx = len(m.history) - 1
		} else if m.histIdx > 0 {
			m.histIdx--
		}
		m.textInput.SetValue(m.history[m.histIdx])
		m.textInput.CursorEnd()
		return m, nil

	case tea.KeyDown:
		if m.histIdx == -1 {
			return m, nil
		}
		if m.histIdx < len(m.history)-1 {
			m.histIdx++
			m.textInput.SetValue(m.history[m.histIdx])
		} else {
			m.histIdx = -1
			m.textInput.SetValue(m.histTmp)
			m.histTmp = ""
		}
		m.textInput.CursorEnd()
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *SessionModel) handleEnter() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textInput.Value())
	if input == "" {
		return m, nil
	}

	// Save to history
	if len(m.history) == 0 || m.history[len(m.history)-1] != input {
		m.history = append(m.history, input)
	}
	m.histIdx = -1
	m.histTmp = ""

	m.textInput.SetValue("")

	if input == "exit" || input == "quit" {
		m.quitting = true
		return m, tea.Quit
	}

	if input == "help" {
		help := "SSH session\n\nBuilt-in commands:\n  help    Show this help\n  exit    Disconnect\n  quit    Disconnect"
		if m.commandExecutor != nil {
			help += "\n\nDaemon connected. Operational commands available (e.g., peer list, rib summary)."
		} else {
			help += "\n\nNo command executor. Only built-in commands available."
		}
		m.output = help
		m.viewport.SetContent(m.output)
		m.viewport.GotoTop()
		return m, nil
	}

	// Try command executor if available.
	if m.commandExecutor != nil {
		result, err := m.commandExecutor(input)
		if err != nil {
			m.output = fmt.Sprintf("error: %v", err)
		} else {
			m.output = result
		}
	} else {
		m.output = fmt.Sprintf("no command executor: cannot execute '%s'\nType 'help' for available commands.", input)
	}

	m.viewport.SetContent(m.output)
	m.viewport.GotoTop()
	return m, nil
}
