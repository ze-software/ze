// Design: docs/architecture/config/yang-config-design.md — show command display
// Overview: model_commands.go — command dispatch

package cli

import (
	"fmt"
	"slices"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

func (m *Model) cmdShow(args []string) (commandResult, error) {
	if m.editor == nil {
		return commandResult{}, fmt.Errorf("command %q requires edit mode (no config file loaded)", cmdShow)
	}

	if len(args) > 0 {
		// Column toggles: show <column> enable|disable
		// Only matches when the second arg is explicitly "enable" or "disable",
		// so "show changes all" still routes to the changes subcommand.
		if len(args) >= 2 && isShowColumn(args[0]) && (args[1] == cmdEnable || args[1] == cmdDisable) {
			return m.cmdShowColumnToggle(args)
		}

		// Preserved subcommands: blame, changes require an active session.
		if args[0] == cmdBlame || args[0] == cmdChanges {
			if !m.editor.HasSession() {
				return commandResult{}, fmt.Errorf("show %s requires an active editing session", args[0])
			}
			if args[0] == cmdBlame {
				return m.cmdShowBlame()
			}
			return m.cmdShowChanges(args[1:])
		}

		// Column query: bare "show <column>" reports current state.
		if isShowColumn(args[0]) {
			return m.cmdShowColumnToggle(args)
		}

		// Bulk toggles: show all / show none
		if args[0] == cmdAll {
			return m.cmdShowAllColumns(true)
		}
		if args[0] == cmdNone {
			return m.cmdShowAllColumns(false)
		}

	}

	// Default: display config in tree format with enabled columns.
	return m.cmdShowDisplay(fmtTree, "")
}

// isShowColumn returns true if the name is a valid show column.
func isShowColumn(name string) bool {
	return slices.Contains(showColumnNames, name)
}

// cmdShowColumnToggle handles "show <column> enable|disable".
// After toggling, re-renders the viewport with updated column settings.
func (m *Model) cmdShowColumnToggle(args []string) (commandResult, error) {
	if len(args) < 2 {
		// Just "show <column>" -- report current state and refresh viewport.
		// For changes column, report diff gutter state (the user-visible effect).
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
		return commandResult{}, fmt.Errorf("usage: show %s enable|disable", args[0])
	}

	// Re-render viewport with the new column setting
	result, err := m.cmdShowDisplay(fmtTree, "")
	if err != nil {
		return result, err
	}
	result.statusMessage = fmt.Sprintf("%s column %sd", args[0], args[1])
	return result, nil
}

// cmdShowAllColumns enables or disables all four display columns and refreshes the viewport.
func (m *Model) cmdShowAllColumns(enable bool) (commandResult, error) {
	for _, col := range showColumnNames {
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

// cmdShowDisplay renders the config with the specified format and optional compare baseline.
func (m *Model) cmdShowDisplay(format, compareTarget string) (commandResult, error) {
	if m.editor.ContentAtPath(m.contextPath) == "" {
		return commandResult{output: "(empty configuration)"}, nil
	}

	columns := m.showColumns()

	// Compare mode: use diff gutter (original vs current) to show +/- markers.
	// This works independently of metadata columns -- it compares content, not metadata.
	if compareTarget != "" {
		original := m.resolveCompareBaseline(compareTarget)
		content := m.renderShowContent(columns, format)
		return commandResult{configView: &viewportData{
			content:         content,
			originalContent: original,
			hasOriginal:     true,
		}}, nil
	}

	if !columns.AnyEnabled() {
		// No columns enabled: use bare serializers
		if format == fmtConfig {
			return commandResult{output: m.editor.SetView()}, nil
		}
		return commandResult{configView: m.configViewAtPath(m.contextPath)}, nil
	}

	// Annotated view with enabled columns
	content := m.editor.AnnotatedView(m.contextPath, columns, format == fmtConfig)
	return commandResult{output: content}, nil
}

// renderShowContent produces display content using the appropriate serializer
// based on enabled columns and format preference.
func (m *Model) renderShowContent(columns config.ShowColumns, format string) string {
	if columns.AnyEnabled() {
		return m.editor.AnnotatedView(m.contextPath, columns, format == fmtConfig)
	}
	if format == fmtConfig {
		return m.editor.SetView()
	}
	return m.editor.ContentAtPath(m.contextPath)
}

// showColumns returns the ShowColumns based on current DB preferences.
func (m *Model) showColumns() config.ShowColumns {
	return config.ShowColumns{
		Author:  m.editor.ShowColumnEnabled(colAuthor),
		Date:    m.editor.ShowColumnEnabled(colDate),
		Source:  m.editor.ShowColumnEnabled(colSource),
		Changes: m.editor.ShowColumnEnabled(colChanges),
	}
}

// resolveCompareBaseline returns the content for a compare target.
// Handles: "committed" and "saved". Other targets fall back to committed.
func (m *Model) resolveCompareBaseline(target string) string {
	if target == srcSaved {
		return m.editor.SavedDraftContent()
	}
	// "committed" and all others: use the committed (original) content.
	return m.editor.OriginalContent()
}
