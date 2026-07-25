// Design: docs/architecture/config/yang-config-design.md — display option settings
// Overview: model_commands.go — command dispatch
// Related: model_commands_show.go — content display (show command)

package cli

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var errUsageOption = errors.New("usage: option <author|date|source|changes|all|none|errors> [enable|disable|hints|hide]")

// isOptionColumn returns true if the name is a valid display column.
var optionColumnNames = []string{colAuthor, colDate, colSource, colChanges}

func isOptionColumn(name string) bool {
	return slices.Contains(optionColumnNames, name)
}

// cmdOption manages display settings: blame, changes, column toggles, all/none.
func (m *Model) cmdOption(args []string) (commandResult, error) {
	if m.editor == nil {
		return commandResult{}, fmt.Errorf("command %q requires config mode (no config file loaded)", cmdOption)
	}

	if len(args) == 0 {
		return commandResult{}, errUsageOption
	}

	// Column toggles: option <column> enable|disable
	if len(args) >= 2 && isOptionColumn(args[0]) && (args[1] == cmdEnable || args[1] == cmdDisable) {
		return m.cmdOptionColumnToggle(args)
	}

	// Moved to pipe filter: redirect users.
	if args[0] == cmdBlame {
		return commandResult{}, fmt.Errorf("use 'show | %s' instead", args[0])
	}

	// Error display toggles: option errors hints / option errors hide
	if args[0] == cmdErrors {
		return m.cmdOptionErrors(args[1:])
	}

	// Column query: bare "option <column>" reports current state.
	if isOptionColumn(args[0]) {
		return m.cmdOptionColumnToggle(args)
	}

	// Bulk toggles: option all / option none
	if args[0] == cmdAll {
		return m.cmdOptionAllColumns(true)
	}
	if args[0] == cmdNone {
		return m.cmdOptionAllColumns(false)
	}

	return commandResult{}, fmt.Errorf("unknown option: %s", args[0])
}

// cmdOptionColumnToggle handles "option <column> enable|disable".
// After toggling, re-renders the viewport with updated column settings.
func (m *Model) cmdOptionColumnToggle(args []string) (commandResult, error) {
	if len(args) < 2 {
		// Just "option <column>" -- report current state and refresh viewport.
		var enabled bool
		if args[0] == colChanges {
			enabled = m.editor.diffGutterEnabled()
		} else {
			enabled = m.editor.showColumnEnabled(args[0])
		}
		state := cmdDisable
		if enabled {
			state = cmdEnable
		}
		result, err := m.cmdShowDisplay(fmtTree, "")
		if err != nil {
			return result, err
		}
		var tb textbuf.Buffer
		result.statusMessage = tb.Str(args[0]).Str(": ").Str(state).Byte('d').String()
		return result, nil
	}

	switch args[1] {
	case cmdEnable:
		m.editor.setShowColumn(args[0], true)
		if args[0] == colChanges {
			m.editor.setDiffGutter(true)
		}
	case cmdDisable:
		m.editor.setShowColumn(args[0], false)
		if args[0] == colChanges {
			m.editor.setDiffGutter(false)
		}
	default: // reject unknown action
		return commandResult{}, fmt.Errorf("usage: option %s enable|disable", args[0])
	}

	// Re-render viewport with the new column setting
	result, err := m.cmdShowDisplay(fmtTree, "")
	if err != nil {
		return result, err
	}
	var tb2 textbuf.Buffer
	result.statusMessage = tb2.Str(args[0]).Str(" column ").Str(args[1]).Byte('d').String()
	return result, nil
}

// cmdOptionAllColumns enables or disables all four display columns and refreshes the viewport.
func (m *Model) cmdOptionAllColumns(enable bool) (commandResult, error) {
	for _, col := range optionColumnNames {
		m.editor.setShowColumn(col, enable)
	}
	m.editor.setDiffGutter(enable)
	result, err := m.cmdShowDisplay(fmtTree, "")
	if err != nil {
		return result, err
	}
	if enable {
		result.statusMessage = "All columns enabled"
	} else {
		result.statusMessage = "All columns disabled"
	}
	return result, nil
}

// cmdOptionErrors handles error display toggles: option errors hints / option errors hide.
func (m *Model) cmdOptionErrors(args []string) (commandResult, error) {
	if len(args) == 0 {
		state := "disabled"
		if m.showHints {
			state = "enabled"
		}
		var tb3 textbuf.Buffer
		return commandResult{statusMessage: tb3.Str("error hints: ").Str(state).String()}, nil
	}

	switch args[0] {
	case "hints":
		m.showHints = !m.showHints
		msg := "Inline hints disabled"
		if m.showHints {
			msg = "Inline hints enabled"
		}
		return commandResult{
			statusMessage: msg,
			configView:    m.configViewAtPath(m.contextPath),
		}, nil
	case "hide":
		return commandResult{
			statusMessage: "Errors hidden",
			configView:    m.configViewAtPath(m.contextPath),
		}, nil
	}

	return commandResult{}, fmt.Errorf("unknown option errors subcommand: %s (use hints or hide)", args[0])
}
