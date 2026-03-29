// Design: docs/architecture/config/yang-config-design.md — editor mode switching
// Overview: model.go — editor model and update loop
// Detail: completer_command.go — command mode operational completion
// Related: model_render.go — mode-aware prompt rendering

package cli

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"codeberg.org/thomas-mangin/ze/internal/component/command"
)

// EditorMode represents the current editor mode.
type EditorMode int

const (
	// ModeEdit is the config editing mode (default).
	ModeEdit EditorMode = iota
	// ModeCommand is the operational command mode.
	ModeCommand
)

// Mode name constants.
const (
	modeNameEdit    = "edit"
	modeNameCommand = "command"
)

// String returns the mode name.
func (m EditorMode) String() string {
	if m == ModeCommand {
		return modeNameCommand
	}
	return modeNameEdit
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

// SwitchMode switches the editor to the given mode, saving and restoring screen state.
func (m *Model) SwitchMode(target EditorMode) {
	if m.mode == target {
		m.statusMessage = "already in " + target.String() + " mode"
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
	if target == ModeCommand && m.commandExecutor == nil {
		m.statusMessage = "no daemon connection — completions available, but commands will not execute"
	}
}

// editModeCommands lists config commands that trigger a switch from command mode to edit mode.
var editModeCommands = map[string]bool{
	cmdSet: true, cmdDelete: true, cmdShow: true, cmdOption: true, cmdEdit: true,
	cmdDeactivate: true, cmdActivate: true,
	cmdCommit: true, cmdSave: true, cmdDiscard: true, cmdCompare: true,
	cmdRollback: true, cmdHistory: true, cmdLoad: true,
	cmdErrors: true, cmdTop: true, cmdUp: true,
	cmdWho: true, cmdDisconnect: true,
}

// isEditCommand returns true if the input starts with a config editing command.
func isEditCommand(input string) bool {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return false
	}
	return editModeCommands[fields[0]]
}

// isEditCommandWithArgs returns true if the input starts with a config editing command
// followed by arguments or a trailing space. Used by updateCompletions to decide when
// to switch from merged completions to YANG-only completions.
func isEditCommandWithArgs(input string) bool {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return false
	}
	if !editModeCommands[fields[0]] {
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
				err: fmt.Errorf("no daemon connection (command mode requires a running daemon)"),
			}
		}
		cmdStr, formatFn := command.ProcessPipesDefaultTable(input)
		output, err := executor(cmdStr)
		if err != nil {
			return commandResultMsg{err: err}
		}
		return commandResultMsg{result: commandResult{output: formatFn(output)}}
	}
}
