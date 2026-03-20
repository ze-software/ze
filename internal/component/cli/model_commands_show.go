// Design: docs/architecture/config/yang-config-design.md — show command display
// Overview: model_commands.go — command dispatch
// Related: model_commands_option.go — display settings (option command)

package cli

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// cmdShow displays configuration content.
// "show" renders the full tree; future: "show peer" filters to the peer subtree.
func (m *Model) cmdShow(args []string) (commandResult, error) {
	if m.editor == nil {
		return commandResult{}, fmt.Errorf("command %q requires edit mode (no config file loaded)", cmdShow)
	}

	// Reject old show subcommands that moved to "option".
	if len(args) > 0 {
		if args[0] == cmdBlame || args[0] == cmdChanges || isOptionColumn(args[0]) ||
			args[0] == cmdAll || args[0] == cmdNone {
			return commandResult{}, fmt.Errorf("display settings moved: use 'option %s' instead", strings.Join(args, " "))
		}
	}

	// Default: display config in tree format with enabled columns.
	return m.cmdShowDisplay(fmtTree, "")
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
