// Design: docs/architecture/config/yang-config-design.md — show command display
// Overview: model_commands.go — command dispatch
// Related: model_commands_option.go — display settings (option command)

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// cmdShow displays configuration content.
// "show" renders the full tree; "show confirmed" shows committed config; "show saved" shows draft.
func (m *Model) cmdShow(args []string) (commandResult, error) {
	if m.editor == nil {
		return commandResult{}, fmt.Errorf("command %q requires config mode (no config file loaded)", cmdShow)
	}

	// Reject subcommands that moved to pipe filters or option.
	if len(args) > 0 {
		switch args[0] {
		case cmdBlame, cmdChanges, cmdErrors, cmdHistory, cmdCompare:
			return commandResult{}, fmt.Errorf("use 'show | %s' instead", args[0])
		case cmdAll, cmdNone:
			return commandResult{}, fmt.Errorf("use 'option %s' instead", args[0])
		}
		if isOptionColumn(args[0]) {
			return commandResult{}, fmt.Errorf("use 'option %s' instead", args[0])
		}
	}

	// Source selection: show confirmed | show saved | show (= show edit).
	source := ""
	if len(args) > 0 && (args[0] == srcConfirmed || args[0] == srcSaved) {
		source = args[0]
		return m.cmdShowDisplayWithSource(fmtTree, "", source)
	}

	// Path arguments: "show bgp" navigates temporarily into bgp for display.
	if len(args) > 0 {
		saved := m.contextPath
		m.contextPath = append(append([]string{}, m.contextPath...), args...)
		result, err := m.cmdShowDisplayWithSource(fmtTree, "", "")
		m.contextPath = saved
		return result, err
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
	// Compare is ONE path for every source, and it runs before the per-source
	// rendering below because it needs TREES to prune against, not the pre-rendered
	// text that rendering produces. A separate compare branch per source is exactly
	// how `show saved | compare` kept dumping the whole config after the working
	// config was fixed.
	if compareTarget != "" {
		return m.compareView(format, compareTarget, source)
	}

	// For alternate sources, render that source's content directly.
	if source == srcConfirmed {
		return m.showAlternateSource(m.renderRawSourceAtPath(m.editor.OriginalContent(), m.contextPath, format))
	}
	if source == srcSaved {
		draft := m.editor.savedDraftContent()
		if draft == "" {
			return commandResult{output: "(no saved draft)"}, nil
		}
		return m.showAlternateSource(m.renderRawSourceAtPath(draft, m.contextPath, format))
	}

	// Default: working config.
	if m.editor.ContentAtPath(m.contextPath) == "" {
		return commandResult{output: "(empty configuration)"}, nil
	}

	columns := m.showColumns()

	if !columns.AnyEnabled() {
		// No columns enabled: use bare serializers
		if format == fmtConfig {
			return commandResult{output: m.renderTreeAtPath(m.editor.tree, m.contextPath, fmtConfig)}, nil
		}
		return commandResult{configView: m.configViewAtPath(m.contextPath)}, nil
	}

	// Annotated view with enabled columns
	content := m.editor.annotatedView(m.contextPath, columns, format == fmtConfig)
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
		var tb textbuf.Buffer
		return commandResult{output: tb.Str("(no ").Str(filter).Str(" configuration)").String()}, nil
	}

	if len(textFilters) == 0 {
		return commandResult{output: content}, nil
	}

	var err error
	for _, f := range textFilters {
		content, err = ApplyPipeFilter(content, f)
		if err != nil {
			return commandResult{}, err
		}
	}
	return commandResult{output: content}, nil
}

// showAlternateSource displays pre-rendered content from a non-working source
// (confirmed/saved). Compare is NOT handled here: it needs trees, and lives in
// compareView, which serves every source.
func (m *Model) showAlternateSource(content string) (commandResult, error) {
	if content == "" {
		return commandResult{output: "(empty configuration)"}, nil
	}
	return commandResult{configView: &viewportData{content: content}}, nil
}

// renderRawSourceAtPath parses raw source content, then serializes only the
// requested path in the requested format. Returns raw root content on parse
// failure so legacy files remain visible instead of blank.
func (m *Model) renderRawSourceAtPath(raw string, path []string, format string) string {
	if m.editor == nil || m.editor.schema == nil {
		return raw
	}
	tree, _, err := parseConfigWithFormat(raw, m.editor.schema)
	if err != nil {
		if len(path) > 0 {
			return ""
		}
		return raw
	}
	return m.renderTreeAtPath(tree, path, format)
}

// renderShowContent produces display content using the appropriate serializer
// based on enabled columns and format preference.
func (m *Model) renderShowContent(columns config.ShowColumns, format string) string {
	if columns.AnyEnabled() {
		return m.editor.annotatedView(m.contextPath, columns, format == fmtConfig)
	}
	if format == fmtConfig {
		return m.renderTreeAtPath(m.editor.tree, m.contextPath, fmtConfig)
	}
	return m.editor.DisplayContentAtPath(m.contextPath)
}

func (m *Model) renderTreeAtPath(tree *config.Tree, path []string, format string) string {
	if tree == nil || m.editor == nil || m.editor.schema == nil {
		return ""
	}
	// Mask ze:bcrypt leaf values for display. MaskBcrypt clones, so the editor's
	// working tree keeps the real hash for validation and persistence. Masking is
	// value-only and line-preserving, so validation line numbers still align.
	tree = config.MaskBcrypt(tree, m.editor.schema)
	if format == fmtConfig {
		return config.FilterSetByPath(config.SerializeSet(tree, m.editor.schema), path)
	}
	if len(path) == 0 {
		return config.Serialize(tree, m.editor.schema)
	}
	subtree, schemaNode := m.editor.walkPathWithSchemaFrom(tree, path)
	if subtree == nil || schemaNode == nil {
		return ""
	}
	return config.SerializeSubtree(subtree, schemaNode)
}

// showColumns returns the ShowColumns based on current DB preferences.
func (m *Model) showColumns() config.ShowColumns {
	return config.ShowColumns{
		Author:  m.editor.showColumnEnabled(colAuthor),
		Date:    m.editor.showColumnEnabled(colDate),
		Source:  m.editor.showColumnEnabled(colSource),
		Changes: m.editor.showColumnEnabled(colChanges),
	}
}

// compareView renders `show [confirmed|saved] | compare <target>` for EVERY source:
// the displayed source and the baseline, each pruned to what differs, then serialized
// by the format pipe. compare selects the data, format presents it (ClassifyShowPipes
// draws the same line).
//
// forceChanges: the markers ARE the output of compare, so they cannot depend on the
// changes column the way a plain `show` does (setViewportData gates on it).
func (m *Model) compareView(format, compareTarget, source string) (commandResult, error) {
	baseline, err := m.resolveCompareBaselineTree(compareTarget)
	if err != nil {
		return commandResult{}, err
	}

	displayed, msg := m.sourceTree(source)
	if msg != "" {
		return commandResult{output: msg}, nil
	}

	// Metadata columns annotate the working config only: the alternate sources were
	// always rendered through renderRawSourceAtPath, which ignores them.
	var columns config.ShowColumns
	if source == "" {
		columns = m.showColumns()
	}

	// Without both trees there is nothing to prune against. Fall back to the
	// unpruned text views so a legacy or corrupt file stays VISIBLE rather than
	// blank -- renderRawSourceAtPath returns raw content on parse failure for
	// exactly this reason. Isolation is lost here; visibility is not.
	if displayed == nil || baseline == nil || m.editor.schema == nil {
		return m.compareViewUnpruned(format, compareTarget, source, columns)
	}

	content, original := m.comparePrunedViews(displayed, baseline, columns, format)
	return commandResult{configView: &viewportData{
		content:         content,
		originalContent: original,
		hasOriginal:     true,
		forceChanges:    true,
		// The pruned view is not the validated string, so it cannot position
		// validation errors. Only this branch opts out: compareViewUnpruned below
		// renders the full content, where the line numbers still line up.
		noValidationHighlight: true,
	}}, nil
}

// compareViewUnpruned is the pre-prune compare rendering, kept for a source or
// baseline the parser cannot turn into a tree.
func (m *Model) compareViewUnpruned(format, compareTarget, source string, columns config.ShowColumns) (commandResult, error) {
	original, err := m.resolveCompareBaseline(compareTarget, format)
	if err != nil {
		return commandResult{}, err
	}
	content := m.sourceContent(source, columns, format)
	if content == "" {
		return commandResult{output: "(empty configuration)"}, nil
	}
	return commandResult{configView: &viewportData{
		content:         content,
		originalContent: original,
		hasOriginal:     true,
		forceChanges:    true,
	}}, nil
}

// sourceTree returns the parsed tree for a show source, or a message when the source
// has nothing to show. A nil tree with an empty message means the source could not be
// parsed: the caller falls back to the unpruned text views.
func (m *Model) sourceTree(source string) (*config.Tree, string) {
	switch source {
	case srcConfirmed:
		return m.parseSourceTree(m.editor.OriginalContent()), ""
	case srcSaved:
		draft := m.editor.savedDraftContent()
		if draft == "" {
			return nil, "(no saved draft)"
		}
		return m.parseSourceTree(draft), ""
	default:
		if !m.editor.treeValid {
			return nil, ""
		}
		return m.editor.tree, ""
	}
}

// sourceContent renders a show source as text, the way it was rendered before
// tree-level pruning existed.
func (m *Model) sourceContent(source string, columns config.ShowColumns, format string) string {
	switch source {
	case srcConfirmed:
		return m.renderRawSourceAtPath(m.editor.OriginalContent(), m.contextPath, format)
	case srcSaved:
		return m.renderRawSourceAtPath(m.editor.savedDraftContent(), m.contextPath, format)
	default:
		return m.renderShowContent(columns, format)
	}
}

// comparePrunedViews renders both sides of a compare, each pruned to what differs.
//
// Both directions are pruned because they answer different questions: the displayed
// tree pruned against the baseline yields additions and modifications, while the
// baseline pruned against the displayed tree yields removals. A deleted node exists
// only in the baseline, so nothing but the second direction can show it.
//
// The two sides keep the rendering they had before pruning: content honors the
// metadata columns (as renderShowContent does), original does not (as
// renderRawSourceAtPath did). Pruning changes WHICH nodes are rendered, not how.
func (m *Model) comparePrunedViews(displayed, baseline *config.Tree, columns config.ShowColumns, format string) (content, original string) {
	displayedPruned := displayed.Clone()
	config.PruneUnchanged(displayedPruned, baseline, m.editor.schema)

	baselinePruned := baseline.Clone()
	config.PruneUnchanged(baselinePruned, displayed, m.editor.schema)

	return m.renderPrunedContent(displayedPruned, columns, format),
		m.renderTreeAtPath(baselinePruned, m.contextPath, format)
}

// renderPrunedContent mirrors renderShowContent over an explicitly supplied tree.
func (m *Model) renderPrunedContent(tree *config.Tree, columns config.ShowColumns, format string) string {
	if columns.AnyEnabled() {
		return m.editor.annotatedViewOf(tree, m.contextPath, columns, format == fmtConfig)
	}
	return m.renderTreeAtPath(tree, m.contextPath, format)
}

// resolveCompareBaselineTree is resolveCompareBaseline returning the parsed tree
// instead of rendered text, so the caller can prune against it.
// Handles: "confirmed"/"committed", "saved", "rollback N", or a username.
// Returns nil (not an error) when the source cannot be parsed -- the caller falls
// back to unpruned views.
func (m *Model) resolveCompareBaselineTree(target string) (*config.Tree, error) {
	normalized := NormalizeCompareTarget(target)
	if normalized == srcSaved {
		return m.parseSourceTree(m.editor.savedDraftContent()), nil
	}

	if normalized == cmdRollback {
		raw, err := m.resolveRollbackBaseline(strings.TrimPrefix(strings.TrimSpace(target), "rollback "))
		if err != nil {
			return nil, err
		}
		return m.parseSourceTree(raw), nil
	}

	if normalized == srcConfirmed {
		return m.parseSourceTree(m.editor.OriginalContent()), nil
	}

	// Treat as username: build baseline by reverting that user's changes.
	baseline := m.editor.contentWithoutUser(target)
	if baseline == "" {
		// No metadata or no changes by that user -- fall back to committed.
		return m.parseSourceTree(m.editor.OriginalContent()), nil
	}
	return m.parseSourceTree(baseline), nil
}

// parseSourceTree parses raw config text into a tree, or nil if it cannot be parsed.
func (m *Model) parseSourceTree(raw string) *config.Tree {
	if m.editor == nil || m.editor.schema == nil || raw == "" {
		return nil
	}
	tree, _, err := parseConfigWithFormat(raw, m.editor.schema)
	if err != nil {
		return nil
	}
	return tree
}

// resolveCompareBaseline returns the content for a compare target.
// Handles: "confirmed"/"committed", "saved", "rollback N", or a username.
func (m *Model) resolveCompareBaseline(target, format string) (string, error) {
	normalized := NormalizeCompareTarget(target)
	if normalized == srcSaved {
		return m.renderRawSourceAtPath(m.editor.savedDraftContent(), m.contextPath, format), nil
	}

	if normalized == cmdRollback {
		raw, err := m.resolveRollbackBaseline(strings.TrimPrefix(strings.TrimSpace(target), "rollback "))
		if err != nil {
			return "", err
		}
		return m.renderRawSourceAtPath(raw, m.contextPath, format), nil
	}

	if normalized == srcConfirmed {
		return m.renderRawSourceAtPath(m.editor.OriginalContent(), m.contextPath, format), nil
	}

	// Treat as username: build baseline by reverting that user's changes.
	baseline := m.editor.contentWithoutUser(target)
	if baseline == "" {
		// No metadata or no changes by that user -- fall back to committed.
		return m.renderRawSourceAtPath(m.editor.OriginalContent(), m.contextPath, format), nil
	}
	return m.renderRawSourceAtPath(baseline, m.contextPath, format), nil
}

// NormalizeCompareTarget maps compare aliases to the shared target names used
// by SSH and web CLI compare handling.
func NormalizeCompareTarget(target string) string {
	trimmed := strings.TrimSpace(target)
	switch trimmed {
	case "", srcConfirmed, "committed", "commit":
		return srcConfirmed
	case srcSaved:
		return srcSaved
	}
	if strings.HasPrefix(trimmed, "rollback ") {
		return cmdRollback
	}
	return trimmed
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

	data, err := m.editor.ReadBackupContent(backups[n-1].Path)
	if err != nil {
		return "", fmt.Errorf("cannot read backup %d: %w", n, err)
	}

	return string(data), nil
}
