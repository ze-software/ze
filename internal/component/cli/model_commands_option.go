// Design: docs/architecture/config/yang-config-design.md — display option settings
// Overview: model_commands.go — command dispatch
// Related: model_commands_show.go — content display (show command)

package cli

import (
	"fmt"
	"slices"
)

// isOptionColumn returns true if the name is a valid display column.
var optionColumnNames = []string{colAuthor, colDate, colSource, colChanges}

func isOptionColumn(name string) bool {
	return slices.Contains(optionColumnNames, name)
}

// cmdOption manages display settings: blame, changes, column toggles, all/none.
func (m *Model) cmdOption(args []string) (commandResult, error) {
	if m.editor == nil {
		return commandResult{}, fmt.Errorf("command %q requires edit mode (no config file loaded)", cmdOption)
	}

	if len(args) == 0 {
		return commandResult{}, fmt.Errorf("usage: option <blame|changes|author|date|source|all|none> [enable|disable]")
	}

	// Column toggles: option <column> enable|disable
	if len(args) >= 2 && isOptionColumn(args[0]) && (args[1] == cmdEnable || args[1] == cmdDisable) {
		return m.cmdOptionColumnToggle(args)
	}

	// View modes: blame, changes require an active session.
	if args[0] == cmdBlame || args[0] == cmdChanges {
		if !m.editor.HasSession() {
			return commandResult{}, fmt.Errorf("option %s requires an active editing session", args[0])
		}
		if args[0] == cmdBlame {
			return m.cmdShowBlame()
		}
		return m.cmdShowChanges(args[1:])
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
			enabled = m.editor.DiffGutterEnabled()
		} else {
			enabled = m.editor.ShowColumnEnabled(args[0])
		}
		state := cmdDisable + "d"
		if enabled {
			state = cmdEnable + "d"
		}
		result, err := m.cmdShowDisplay(fmtTree, "")
		if err != nil {
			return result, err
		}
		result.statusMessage = fmt.Sprintf("%s: %s", args[0], state)
		return result, nil
	}

	switch args[1] {
	case cmdEnable:
		m.editor.SetShowColumn(args[0], true)
		if args[0] == colChanges {
			m.editor.SetDiffGutter(true)
		}
	case cmdDisable:
		m.editor.SetShowColumn(args[0], false)
		if args[0] == colChanges {
			m.editor.SetDiffGutter(false)
		}
	default: // reject unknown action
		return commandResult{}, fmt.Errorf("usage: option %s enable|disable", args[0])
	}

	// Re-render viewport with the new column setting
	result, err := m.cmdShowDisplay(fmtTree, "")
	if err != nil {
		return result, err
	}
	result.statusMessage = fmt.Sprintf("%s column %sd", args[0], args[1])
	return result, nil
}

// cmdOptionAllColumns enables or disables all four display columns and refreshes the viewport.
func (m *Model) cmdOptionAllColumns(enable bool) (commandResult, error) {
	for _, col := range optionColumnNames {
		m.editor.SetShowColumn(col, enable)
	}
	m.editor.SetDiffGutter(enable)
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

// cmdOptionPipe executes option subcommands with pipe filters (grep, head, tail).
func (m *Model) cmdOptionPipe(args []string, filters []PipeFilter) (commandResult, error) {
	var result commandResult
	var err error

	if len(args) > 0 && (args[0] == cmdBlame || args[0] == cmdChanges) {
		if !m.editor.HasSession() {
			return commandResult{}, fmt.Errorf("option %s requires an active editing session", args[0])
		}
		if args[0] == cmdBlame {
			result, err = m.cmdShowBlame()
		} else {
			result, err = m.cmdShowChanges(args[1:])
		}
	} else {
		// Column toggles don't produce pipeable output; just run the option command.
		return m.cmdOption(args)
	}
	if err != nil {
		return result, err
	}

	if len(filters) == 0 {
		return result, nil
	}

	// Apply text filters to the output.
	output := result.output
	for _, f := range filters {
		output, err = ApplyPipeFilter(output, f)
		if err != nil {
			return commandResult{}, err
		}
	}
	result.output = output
	return result, nil
}
