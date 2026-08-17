// Design: docs/architecture/config/yang-config-design.md — editor mode switching
// Overview: model.go — editor model and update loop
// Detail: completer_command.go — operational mode command completion
// Related: model_render.go — mode-aware prompt rendering

package cli

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errNoDaemonConnectionOperationalModeRequires = errors.New("no daemon connection (operational mode requires a running daemon)")

// EditorMode represents the current editor mode.
type EditorMode int

const (
	// ModeConfig is the config editing mode (default when editor loaded).
	ModeConfig EditorMode = iota
	// ModeOperational is the operational command mode.
	ModeOperational
)

// Mode name constants.
const (
	modeNameConfig      = "config"
	modeNameOperational = "operational"
)

// String returns the mode name.
func (m EditorMode) String() string {
	if m == ModeOperational {
		return modeNameOperational
	}
	return modeNameConfig
}

// modeState saves the screen state for a mode.
type modeState struct {
	viewportContent string          // Content displayed in viewport
	viewportYOffset int             // Vertical scroll position
	showViewport    bool            // Whether viewport was active
	statusMessage   string          // Status message at time of switch
	histSnap        historySnapshot // Command history snapshot for this mode
}

// Mode returns the current editor mode.
func (m Model) Mode() EditorMode {
	return m.mode
}

// switchMode switches the editor to the given mode, saving and restoring screen state.
func (m *Model) switchMode(target EditorMode) {
	if m.mode == target {
		var tb textbuf.Buffer
		m.statusMessage = tb.Str("already in ").Str(target.String()).Str(" mode").String()
		return
	}

	// Save current mode's state
	m.modeStates[m.mode] = modeState{
		viewportContent: m.viewportContent,
		viewportYOffset: m.viewport.YOffset(),
		showViewport:    m.showViewport,
		statusMessage:   m.statusMessage,
		histSnap:        m.history.snapshot(),
	}

	// Switch mode
	m.mode = target

	// Restore target mode's state
	saved := m.modeStates[target]
	m.viewportContent = saved.viewportContent
	m.showViewport = saved.showViewport
	m.statusMessage = saved.statusMessage
	m.history.restore(saved.histSnap)

	m.viewport.SetContent(saved.viewportContent)
	m.viewport.SetYOffset(saved.viewportYOffset)

	// Warn when entering command mode without a daemon connection
	if target == ModeOperational && m.commandExecutor == nil {
		m.statusMessage = "no daemon connection — completions available, but commands will not execute"
	}
}

// configModeCommands lists config commands that trigger a switch from operational mode to config mode.
var configModeCommands = map[string]bool{
	cmdSet: true, cmdDelete: true, cmdShow: true, cmdOption: true, cmdEdit: true,
	cmdDeactivate: true, cmdActivate: true,
	cmdCommit: true, cmdSave: true, cmdDiscard: true,
	cmdRollback: true, cmdLoad: true,
	cmdTop: true, cmdUp: true,
	cmdWho: true, cmdDisconnect: true,
}

// isOperationalVerb returns true if the input starts with a verb that works
// in both config mode (config viewer) and operational mode (operational dispatch).
// When no editor is loaded, these fall through to operational dispatch instead
// of showing "config mode not available".
func isOperationalVerb(input string) bool {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case cmdShow, cmdWho:
		return true
	}
	return false
}

// isConfigCommand returns true if the input starts with a config editing command.
func isConfigCommand(input string) bool {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return false
	}
	return configModeCommands[fields[0]]
}

// isConfigCommandWithArgs returns true if the input starts with a config editing command
// followed by arguments or a trailing space. Used by updateCompletions to decide when
// to switch from merged completions to YANG-only completions.
func isConfigCommandWithArgs(input string) bool {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return false
	}
	if !configModeCommands[fields[0]] {
		return false
	}
	return len(fields) > 1 || strings.HasSuffix(input, " ")
}

// executeOperationalCommand sends a command to the daemon via the injected executor.
// Pipe operators (| table, | json, | match, etc.) are processed here so all
// entry points (ze cli, SSH, plugin CLI) get pipe support automatically.
// Returns a tea.Cmd that produces a commandResultMsg with the response.
func (m Model) executeOperationalCommand(input string) tea.Cmd {
	executor := m.commandExecutor
	return func() tea.Msg {
		if executor == nil {
			return commandResultMsg{
				err: errNoDaemonConnectionOperationalModeRequires,
			}
		}
		cmdStr, formatFn, pipeErr := command.ProcessPipesDefaultFormatChecked(input, m.cliFormat)
		if pipeErr != "" {
			var tb2 textbuf.Buffer
			return commandResultMsg{err: errors.New(tb2.Str("pipe error: ").Str(pipeErr).String())}
		}
		output, err := executor(cmdStr)
		result := commandResult{
			output:            formatFn(output.Text),
			transportComplete: output.TransportComplete,
		}
		if err != nil {
			return commandResultMsg{result: result, err: err}
		}
		return commandResultMsg{result: result}
	}
}
