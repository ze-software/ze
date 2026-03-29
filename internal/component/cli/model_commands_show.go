// Design: docs/architecture/config/yang-config-design.md — show command display
// Overview: model_commands.go — command dispatch
// Related: model_commands_option.go — display settings (option command)

package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// cmdShow displays configuration content.
// "show" renders the full tree; "show confirmed" shows committed config; "show saved" shows draft.
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

	// Source selection: show confirmed | show saved | show (= show edit).
	source := ""
	if len(args) > 0 && (args[0] == srcConfirmed || args[0] == srcSaved) {
		source = args[0]
	}

	return m.cmdShowDisplayWithSource(fmtTree, "", source)
}

// cmdShowDisplay renders the working config with the specified format and optional compare baseline.
// Shorthand for cmdShowDisplayWithSource with empty source (= working config).
func (m *Model) cmdShowDisplay(format, compareTarget string) (commandResult, error) {
	return m.cmdShowDisplayWithSource(format, compareTarget, "")
}

// cmdShowDisplayWithSource renders config from the selected source with format and compare options.
// Source: "" or "edit" = working config, "confirmed" = on-disk original, "saved" = draft file.
func (m *Model) cmdShowDisplayWithSource(format, compareTarget, source string) (commandResult, error) {
	// For alternate sources, render that source's content directly.
	if source == srcConfirmed {
		return m.showAlternateSource(m.editor.OriginalContentAtPath(m.contextPath), compareTarget)
	}
	if source == srcSaved {
		draft := m.editor.SavedDraftContent()
		if draft == "" {
			return commandResult{output: "(no saved draft)"}, nil
		}
		return m.showAlternateSource(m.resolveSourceContent(draft), compareTarget)
	}

	// Default: working config.
	if m.editor.ContentAtPath(m.contextPath) == "" {
		return commandResult{output: "(empty configuration)"}, nil
	}

	columns := m.showColumns()

	// Compare mode: use diff gutter (original vs current) to show +/- markers.
	// This works independently of metadata columns -- it compares content, not metadata.
	if compareTarget != "" {
		original, err := m.resolveCompareBaseline(compareTarget)
		if err != nil {
			return commandResult{}, err
		}
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

// cmdShowFiltered renders config with a tree-level filter (active or inactive).
// The filter clones the tree and prunes it before serialization, then applies text filters.
func (m *Model) cmdShowFiltered(filter string, textFilters []PipeFilter) (commandResult, error) {
	content := m.editor.ActiveContentAtPath(m.contextPath)
	if filter == cmdInactive {
		content = m.editor.InactiveContentAtPath(m.contextPath)
	}

	if content == "" {
		return commandResult{output: fmt.Sprintf("(no %s configuration)", filter)}, nil
	}

	if len(textFilters) == 0 {
		return commandResult{output: content}, nil
	}

	var err error
	for _, f := range textFilters {
		content, err = applyPipeFilter(content, f)
		if err != nil {
			return commandResult{}, err
		}
	}
	return commandResult{output: content}, nil
}

// showAlternateSource displays pre-rendered content from a non-working source (confirmed/saved).
// Supports compare and format pipes applied to the alternate content.
func (m *Model) showAlternateSource(content, compareTarget string) (commandResult, error) {
	if content == "" {
		return commandResult{output: "(empty configuration)"}, nil
	}
	if compareTarget != "" {
		original, err := m.resolveCompareBaseline(compareTarget)
		if err != nil {
			return commandResult{}, err
		}
		return commandResult{configView: &viewportData{
			content:         content,
			originalContent: original,
			hasOriginal:     true,
		}}, nil
	}
	return commandResult{configView: &viewportData{content: content}}, nil
}

// resolveSourceContent parses raw content (e.g., draft file) through the schema
// to produce hierarchical tree output. Returns raw content as-is on parse failure.
func (m *Model) resolveSourceContent(raw string) string {
	if m.editor == nil || m.editor.schema == nil {
		return raw
	}
	tree, _, err := parseConfigWithFormat(raw, m.editor.schema)
	if err != nil {
		return raw
	}
	return config.Serialize(tree, m.editor.schema)
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
// Handles: "committed", "saved", "rollback N". Returns error for invalid targets.
func (m *Model) resolveCompareBaseline(target string) (string, error) {
	if target == srcSaved {
		return m.editor.SavedDraftContent(), nil
	}

	if strings.HasPrefix(target, "rollback ") {
		return m.resolveRollbackBaseline(target[len("rollback "):])
	}

	if target == srcConfirmed {
		return m.editor.OriginalContent(), nil
	}

	// Treat as username: build baseline by reverting that user's changes.
	baseline := m.editor.ContentWithoutUser(target)
	if baseline == "" {
		// No metadata or no changes by that user -- fall back to committed.
		return m.editor.OriginalContent(), nil
	}
	return baseline, nil
}

// resolveRollbackBaseline reads the Nth backup file content.
// N is 1-based (rollback 1 = most recent backup).
func (m *Model) resolveRollbackBaseline(nStr string) (string, error) {
	n, err := strconv.Atoi(strings.TrimSpace(nStr))
	if err != nil {
		return "", fmt.Errorf("invalid rollback number: %s", nStr)
	}

	if n < 1 {
		return "", fmt.Errorf("rollback number must be >= 1, got %d", n)
	}

	backups, err := m.editor.ListBackups()
	if err != nil {
		return "", fmt.Errorf("cannot list backups: %w", err)
	}

	if n > len(backups) {
		return "", fmt.Errorf("backup %d not found (have %d backups)", n, len(backups))
	}

	data, err := os.ReadFile(backups[n-1].Path)
	if err != nil {
		return "", fmt.Errorf("cannot read backup %d: %w", n, err)
	}

	return string(data), nil
}
