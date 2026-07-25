// Design: docs/architecture/config/yang-config-design.md — config editor

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errTimeoutMustBeAtLeast1           = errors.New("timeout must be at least 1 second")
	errTimeoutMustBeAtMost3600         = errors.New("timeout must be at most 3600 seconds (1 hour)")
	errCommitSucceededButNoBackupFound = errors.New("commit succeeded but no backup found for rollback")
	errNoPendingCommitToConfirm        = errors.New("no pending commit to confirm")
	errNoPendingCommitToAbort          = errors.New("no pending commit to abort")
	errUsageLoadFile                   = errors.New("usage: load <file>")
	errUsageLoadMergeFile              = errors.New("usage: load merge <file>")
	errNoCommandBeforePipe             = errors.New("no command before pipe")
)

// --- Commit Confirmed (VyOS-style safe commit with auto-revert) ---
//
// Three-file convention:
//   <config>.edit.conf — working copy being edited
//   <config>.live.conf — trial config applied during confirm window
//   <config>.conf      — stable config, restored on timeout/abort
//
// Flow:
//   commit confirmed <N> → backup .conf, write .live.conf, overwrite .conf, start timer
//   confirm              → delete .live.conf (already permanent in .conf)
//   timeout/abort        → rollback .conf from backup, delete .live.conf, reload daemon

// cmdCommitConfirmed commits with auto-rollback if not confirmed within timeout.
// Writes the trial config to .live.conf for audit, then overwrites .conf so the
// daemon picks it up on reload. The original .conf is preserved in a dated backup.
// When force is true, warnings are skipped but errors still block.
func (m *Model) cmdCommitConfirmed(seconds int, force bool) (commandResult, error) {
	// Boundary validation: 1-3600 seconds
	if seconds < 1 {
		return commandResult{}, errTimeoutMustBeAtLeast1
	}
	if seconds > 3600 {
		return commandResult{}, errTimeoutMustBeAtMost3600
	}

	// Validate before commit.
	result := m.validator.Validate(m.editor.WorkingContent())
	if force {
		// Force mode: only errors block, warnings are skipped.
		if len(result.Errors) > 0 {
			return commandResult{}, fmt.Errorf("cannot commit: %s", formatValidationErrors(result.Errors))
		}
	} else {
		// Normal mode: both errors and warnings block.
		issues := make([]ConfigValidationError, 0, len(result.Errors)+len(result.Warnings))
		issues = append(issues, result.Errors...)
		issues = append(issues, result.Warnings...)
		if len(issues) > 0 {
			return commandResult{}, fmt.Errorf("cannot commit: %s", formatValidationErrors(issues))
		}
	}

	// Write trial config to .live.conf (audit trail + pending indicator)
	if err := m.editor.SaveLive(); err != nil {
		return commandResult{}, err
	}

	// Save to .conf: use CommitSession() in session mode (writes set+meta format),
	// fall back to Save() for non-session mode (raw text / hierarchical).
	if m.editor.HasSession() {
		commitResult, err := m.editor.CommitSession()
		if err != nil {
			m.editor.DeleteLive()
			return commandResult{}, err
		}
		if len(commitResult.Conflicts) > 0 {
			m.editor.DeleteLive()
			var b textbuf.Buffer
			b.Str("Commit blocked by conflicts:\n")
			for _, c := range commitResult.Conflicts {
				b.Str("  ").Str(c.Path).Str(": ").Str(c.MyValue).Byte('\n')
			}
			return commandResult{output: b.String()}, nil
		}
	} else {
		if err := m.editor.Save(); err != nil {
			m.editor.DeleteLive()
			return commandResult{}, err
		}
	}
	m.searchCache = "" // tree changed, invalidate cached set-view

	// Notify daemon immediately so it runs the new config during the confirm window.
	var reloadWarning string
	if m.editor.HasReloadNotifier() {
		reloadWarning = m.tryReload()
	}

	// Get the most recent backup path for potential rollback
	backups, err := m.editor.ListBackups()
	if err != nil || len(backups) == 0 {
		return commandResult{}, errCommitSucceededButNoBackupFound
	}

	return commandResult{
		statusMessage: func() string {
			var tb textbuf.Buffer
			return tb.Str("Committed").Str(reloadWarning).Str(". Confirm within ").Int(int64(seconds)).Str("s or auto-revert. Use 'confirm' or 'confirm abort'.").String()
		}(),
		refreshConfig:         true,
		revalidate:            true,
		setConfirmTimer:       true,
		confirmTimerValue:     true,
		confirmBackupPath:     backups[0].Path,
		startConfirmCountdown: seconds,
	}, nil
}

// cmdConfirm confirms a pending commit, making the trial config permanent.
// The .conf already has the new content. We just clean up .live.conf and stop the timer.
func (m *Model) cmdConfirm() (commandResult, error) {
	if !m.confirmTimerActive {
		return commandResult{}, errNoPendingCommitToConfirm
	}

	// Clean up .live.conf — .conf already has the confirmed content
	m.editor.DeleteLive()
	m.searchCache = "" // tree finalized, invalidate cached set-view

	msg := "Configuration confirmed and saved permanently."
	if m.editor.HasReloadNotifier() {
		msg += m.tryReload()
	}

	return commandResult{
		statusMessage:     msg,
		refreshConfig:     true,
		setConfirmTimer:   true,
		confirmTimerValue: false,
		confirmBackupPath: "",
	}, nil
}

// cmdAbort aborts a pending commit and rolls back to previous state.
// Restores .conf from backup, deletes .live.conf, notifies daemon.
func (m *Model) cmdAbort() (commandResult, error) {
	if !m.confirmTimerActive {
		return commandResult{}, errNoPendingCommitToAbort
	}

	return m.rollbackConfirmed()
}

// rollbackConfirmed performs the actual rollback for both abort and timeout.
func (m *Model) rollbackConfirmed() (commandResult, error) {
	// Rollback .conf from backup
	if m.confirmBackupPath != "" {
		if err := m.editor.Rollback(m.confirmBackupPath); err != nil {
			return commandResult{
				setConfirmTimer:   true,
				confirmTimerValue: false,
				confirmBackupPath: "",
			}, fmt.Errorf("rollback failed: %w", err)
		}
	}

	// Clean up .live.conf
	m.editor.DeleteLive()
	m.searchCache = "" // tree changed, invalidate cached set-view

	msg := "Changes rolled back to previous configuration."
	if m.editor.HasReloadNotifier() {
		msg += m.tryReload()
	}

	return commandResult{
		statusMessage:     msg,
		configView:        m.configViewAtPath(m.contextPath),
		revalidate:        true,
		setConfirmTimer:   true,
		confirmTimerValue: false,
		confirmBackupPath: "",
	}, nil
}

// handleConfirmCountdown processes a countdown tick during a commit confirmed window.
// Decrements the counter each second, triggers rollback at zero.
func (m Model) handleConfirmCountdown() (tea.Model, tea.Cmd) {
	if !m.confirmTimerActive || m.editor == nil {
		return m, nil
	}

	m.confirmSecondsLeft--
	if m.confirmSecondsLeft <= 0 {
		// Timeout — auto-revert
		result, err := m.rollbackConfirmed()
		if err != nil {
			m.err = err
		}
		m.applyResult(result)
		m.statusMessage = "Timeout: configuration automatically rolled back."
		return m, nil
	}

	// Update countdown display
	m.statusMessage = textbuf.StrIntStr("Confirm within ", int64(m.confirmSecondsLeft), "s or auto-revert. Use 'confirm' or 'confirm abort'.")
	return m, tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return confirmCountdownMsg{}
	})
}

// cmdLoad loads configuration from a file, replacing current content.
func (m *Model) cmdLoad(args []string) (commandResult, error) {
	if len(args) < 1 {
		return commandResult{}, errUsageLoadFile
	}

	loadPath := m.resolveConfigPath(args[0])

	data, err := readFile(loadPath)
	if err != nil {
		return commandResult{}, fmt.Errorf("cannot read %s: %w", args[0], err)
	}

	m.editor.SetWorkingContent(string(data))
	m.editor.MarkDirty()

	var tb textbuf.Buffer
	return commandResult{
		statusMessage: tb.Str("Configuration loaded from ").Str(args[0]).String(),
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// cmdLoadMerge loads configuration from a file and merges with current content.
func (m *Model) cmdLoadMerge(args []string) (commandResult, error) {
	if len(args) < 1 {
		return commandResult{}, errUsageLoadMergeFile
	}

	loadPath := m.resolveConfigPath(args[0])

	data, err := readFile(loadPath)
	if err != nil {
		return commandResult{}, fmt.Errorf("cannot read %s: %w", args[0], err)
	}

	// Merge needs full content (not subtree)
	currentContent := m.editor.WorkingContent()
	mergeContent := string(data)

	merged := mergeConfigs(currentContent, mergeContent)

	m.editor.SetWorkingContent(merged)
	m.editor.MarkDirty()

	var tb textbuf.Buffer
	return commandResult{
		statusMessage: tb.Str("Configuration merged from ").Str(args[0]).String(),
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// resolveConfigPath resolves a path relative to the config file directory.
func (m *Model) resolveConfigPath(path string) string {
	if isAbsPath(path) {
		return path
	}
	configDir := getDir(m.editor.OriginalPath())
	return joinPath(configDir, path)
}

// parseLoadArgs parses the new load command syntax: load <source> <location> <action> [file]
// Returns (source, location, action, path, error).
// source: "file" or "terminal"
// location: "absolute" or "relative"
// action: "replace" or "merge"
// path: required when source="file", empty for "terminal"
func parseLoadArgs(args []string) (source, location, action, path string, err error) {
	const usage = "usage: load file|terminal absolute|relative replace|merge [path]"

	if len(args) < 1 {
		return "", "", "", "", fmt.Errorf("missing source (file|terminal). %s", usage)
	}

	source = args[0]
	if source != "file" && source != "terminal" {
		return "", "", "", "", fmt.Errorf("invalid source %q (must be file|terminal). %s", source, usage)
	}

	if len(args) < 2 {
		return "", "", "", "", fmt.Errorf("missing location (absolute|relative). %s", usage)
	}

	location = args[1]
	if location != loadLocationAbsolute && location != loadLocationRelative {
		return "", "", "", "", fmt.Errorf("invalid location %q (must be absolute|relative). %s", location, usage)
	}

	if len(args) < 3 {
		return "", "", "", "", fmt.Errorf("missing action (replace|merge). %s", usage)
	}

	action = args[2]
	if action != loadActionReplace && action != loadActionMerge {
		return "", "", "", "", fmt.Errorf("invalid action %q (must be replace|merge). %s", action, usage)
	}

	if source == "file" {
		if len(args) < 4 {
			return "", "", "", "", fmt.Errorf("missing path for 'load file'. %s", usage)
		}
		path = args[3]
	}

	return source, location, action, path, nil
}

// cmdLoadNew handles the redesigned load command syntax.
// Syntax: load <source> <location> <action> [file].
func (m *Model) cmdLoadNew(args []string) (commandResult, error) {
	if m.editor.HasSession() {
		return commandResult{}, errLoadNotSupportedInSessionMode
	}

	source, location, action, path, err := parseLoadArgs(args)
	if err != nil {
		return commandResult{}, err
	}

	// Terminal source enters paste mode
	if source == "terminal" {
		return commandResult{
			statusMessage:     "[Paste mode - Ctrl-D to finish]",
			enterPasteMode:    true,
			pasteModeLocation: location,
			pasteModeAction:   action,
		}, nil
	}

	// File source - read and apply
	loadPath := m.resolveConfigPath(path)
	data, err := readFile(loadPath)
	if err != nil {
		return commandResult{}, fmt.Errorf("cannot read %s: %w", path, err)
	}

	if location == loadLocationAbsolute {
		return m.applyLoadAbsolute(action, string(data), path)
	}
	return m.applyLoadRelative(action, string(data), path)
}

// applyLoadAbsolute applies loaded content at root level.
func (m *Model) applyLoadAbsolute(action, content, path string) (commandResult, error) {
	var tb textbuf.Buffer
	if action == loadActionReplace {
		m.editor.SetWorkingContent(content)
		m.editor.MarkDirty()
		return commandResult{
			statusMessage: tb.Str("Configuration loaded from ").Str(path).String(),
			configView:    m.configViewAtPath(m.contextPath),
			revalidate:    true,
		}, nil
	}

	// action == "merge"
	currentContent := m.editor.WorkingContent()
	merged := mergeConfigs(currentContent, content)
	m.editor.SetWorkingContent(merged)
	m.editor.MarkDirty()
	return commandResult{
		statusMessage: tb.Reset().Str("Configuration merged from ").Str(path).String(),
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// applyLoadRelative applies loaded content at current context position.
func (m *Model) applyLoadRelative(action, content, path string) (commandResult, error) {
	if len(m.contextPath) == 0 {
		// At root level, relative == absolute
		return m.applyLoadAbsolute(action, content, path)
	}

	// Apply at context position
	currentContent := m.editor.WorkingContent()
	var newContent string

	if action == loadActionReplace {
		newContent = replaceAtContext(currentContent, m.contextPath, content)
	} else {
		newContent = mergeAtContext(currentContent, m.contextPath, content)
	}

	m.editor.SetWorkingContent(newContent)
	m.editor.MarkDirty()

	verb := "loaded"
	if action == loadActionMerge {
		verb = "merged"
	}

	var tb textbuf.Buffer
	return commandResult{
		statusMessage: tb.Str("Configuration ").Str(verb).Str(" from ").Str(path).Str(" at ").Join(m.contextPath, " ").String(),
		configView:    m.configViewAtPath(m.contextPath),
		revalidate:    true,
	}, nil
}

// replaceAtContext replaces the content at the given context path with new content.
func replaceAtContext(fullConfig string, contextPath []string, newContent string) string {
	if len(contextPath) == 0 {
		return fullConfig // nothing to replace
	}

	var result textbuf.Buffer

	// Build the pattern to match (e.g., "peer 1.1.1.1" or just "bgp")
	var targetPattern string
	if len(contextPath) == 1 {
		targetPattern = contextPath[0]
	} else {
		var tb textbuf.Buffer
		targetPattern = tb.Str(contextPath[len(contextPath)-2]).Byte(' ').Str(contextPath[len(contextPath)-1]).String()
	}

	inTarget := false
	targetDepth := 0
	currentDepth := 0

	for line := range strings.SplitSeq(fullConfig, "\n") {
		trimmed := strings.TrimSpace(line)
		openBraces := strings.Count(trimmed, "{")
		closeBraces := strings.Count(trimmed, "}")

		if !inTarget {
			// Looking for target block
			if strings.Contains(trimmed, "{") {
				blockPart := strings.TrimSuffix(trimmed, "{")
				blockPart = strings.TrimSpace(blockPart)

				if blockPart == targetPattern {
					// Found target - write opening line and new content
					result.Str(line).Byte('\n')
					inTarget = true
					targetDepth = currentDepth + openBraces

					// Write indented new content
					indent := strings.Repeat("  ", targetDepth)
					for newLine := range strings.SplitSeq(strings.TrimSpace(newContent), "\n") {
						result.Str(indent).Str(newLine).Byte('\n')
					}
					currentDepth += openBraces - closeBraces
					continue
				}
			}
			result.Str(line).Byte('\n')
		} else {
			// Inside target - skip old content until closing brace
			newDepth := currentDepth + openBraces - closeBraces
			if newDepth < targetDepth {
				// Found closing brace - write it
				result.Str(line).Byte('\n')
				inTarget = false
			}
			// Skip old content lines
		}

		currentDepth += openBraces - closeBraces
	}

	return strings.TrimSuffix(result.String(), "\n")
}

// mergeAtContext merges new content into the block at the given context path.
func mergeAtContext(fullConfig string, contextPath []string, newContent string) string {
	if len(contextPath) == 0 {
		return fullConfig // nothing to merge into
	}

	var result textbuf.Buffer

	// Build the pattern to match (e.g., "peer 1.1.1.1" or just "bgp")
	var targetPattern string
	if len(contextPath) == 1 {
		targetPattern = contextPath[0]
	} else {
		var tb textbuf.Buffer
		targetPattern = tb.Str(contextPath[len(contextPath)-2]).Byte(' ').Str(contextPath[len(contextPath)-1]).String()
	}

	inTarget := false
	targetDepth := 0
	currentDepth := 0
	contentInserted := false

	for line := range strings.SplitSeq(fullConfig, "\n") {
		trimmed := strings.TrimSpace(line)
		openBraces := strings.Count(trimmed, "{")
		closeBraces := strings.Count(trimmed, "}")

		if !inTarget {
			if strings.Contains(trimmed, "{") {
				blockPart := strings.TrimSuffix(trimmed, "{")
				blockPart = strings.TrimSpace(blockPart)

				if blockPart == targetPattern {
					inTarget = true
					targetDepth = currentDepth + openBraces
				}
			} else if !contentInserted {
				var tb textbuf.Buffer
				targetWithSpace := tb.Str(targetPattern).Byte(' ').String()
				if inlineContent, ok := strings.CutPrefix(trimmed, targetWithSpace); ok {
					leadingIndent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
					childIndent := tb.Reset().Str(leadingIndent).Byte('\t').String()
					result.Str(leadingIndent).Str(targetPattern).Str(" {\n")
					result.Str(childIndent).Str(inlineContent).Byte('\n')
					for newLine := range strings.SplitSeq(strings.TrimSpace(newContent), "\n") {
						result.Str(childIndent).Str(newLine).Byte('\n')
					}
					result.Str(leadingIndent).Str("}\n")
					contentInserted = true
					currentDepth += openBraces - closeBraces
					continue
				}
			}
			result.Str(line).Byte('\n')
		} else {
			newDepth := currentDepth + openBraces - closeBraces
			if newDepth < targetDepth && !contentInserted {
				// Insert merged content before closing brace
				indent := strings.Repeat("  ", targetDepth)
				for newLine := range strings.SplitSeq(strings.TrimSpace(newContent), "\n") {
					result.Str(indent).Str(newLine).Byte('\n')
				}
				contentInserted = true
				inTarget = false
			}
			result.Str(line).Byte('\n')
		}

		currentDepth += openBraces - closeBraces
	}

	return strings.TrimSuffix(result.String(), "\n")
}

// cmdShowPipe executes show with pipe filters.
// Recognizes show-specific pipes (format, compare) and delegates to cmdShowDisplay,
// then applies text filters (grep, head, tail) to the result.
func (m *Model) cmdShowPipe(args []string, filters []PipeFilter) (commandResult, error) {
	opts, err := ClassifyShowPipes(filters)
	if err != nil {
		return commandResult{}, err
	}

	source := ""
	pathArgs := args
	if len(pathArgs) > 0 && (pathArgs[0] == srcConfirmed || pathArgs[0] == srcSaved) {
		source = pathArgs[0]
		pathArgs = pathArgs[1:]
	}

	savedPath := m.contextPath
	if len(pathArgs) > 0 {
		m.contextPath = append(append([]string{}, m.contextPath...), pathArgs...)
		defer func() { m.contextPath = savedPath }()
	}

	if opts.TreeFilter != "" {
		return m.cmdShowFiltered(opts.TreeFilter, opts.TextFilters)
	}

	if opts.Blame {
		if !m.editor.HasSession() {
			return commandResult{}, fmt.Errorf("show | blame requires an active editing session")
		}
		r, e := m.cmdShowBlame()
		return m.applyTextFilters(r, e, opts.TextFilters)
	}

	if opts.Changes {
		if !m.editor.HasSession() {
			return commandResult{}, fmt.Errorf("show | changes requires an active editing session")
		}
		var cargs []string
		if opts.ChangesAll {
			cargs = []string{cmdAll}
		}
		r, e := m.cmdShowChanges(cargs)
		return m.applyTextFilters(r, e, opts.TextFilters)
	}

	if opts.Errors {
		r, e := m.cmdErrors(nil)
		return m.applyTextFilters(r, e, opts.TextFilters)
	}

	if opts.History {
		r, e := m.cmdHistory()
		return m.applyTextFilters(r, e, opts.TextFilters)
	}

	result, err := m.cmdShowDisplayWithSource(opts.Format, opts.CompareTarget, source)
	if err != nil {
		return result, err
	}

	if len(opts.TextFilters) == 0 {
		return result, nil
	}

	// Text filters flatten configView into plain text since grep/head/tail
	// break the line-by-line diff correspondence.
	output := result.output
	if output == "" && result.configView != nil {
		output = result.configView.content
		result.configView = nil
	}
	for _, f := range opts.TextFilters {
		output, err = ApplyPipeFilter(output, f)
		if err != nil {
			return commandResult{}, err
		}
	}
	result.output = output

	return result, nil
}

func (m *Model) applyTextFilters(result commandResult, err error, filters []PipeFilter) (commandResult, error) {
	if err != nil || len(filters) == 0 {
		return result, err
	}
	output := result.output
	if output == "" && result.configView != nil {
		output = result.configView.content
		result.configView = nil
	}
	for _, f := range filters {
		output, err = ApplyPipeFilter(output, f)
		if err != nil {
			return commandResult{}, err
		}
	}
	result.output = output
	return result, nil
}

// ApplyPipeFilter applies a single pipe filter to content.
// Returns error for unknown filter types.
func ApplyPipeFilter(content string, filter PipeFilter) (string, error) {
	switch filter.Type {
	case cmdMatch:
		var result textbuf.Buffer
		first := true
		for line := range strings.SplitSeq(content, "\n") {
			if strings.Contains(line, filter.Arg) {
				if !first {
					result.Byte('\n')
				}
				result.Str(line)
				first = false
			}
		}
		return result.String(), nil

	case cmdHead:
		n := 10 // default
		if filter.Arg != "" {
			if parsed, err := parseIntArg(filter.Arg); err == nil && parsed > 0 {
				n = parsed
			}
		}
		var result textbuf.Buffer
		written := 0
		for line := range strings.SplitSeq(content, "\n") {
			if written >= n {
				break
			}
			if written > 0 {
				result.Byte('\n')
			}
			result.Str(line)
			written++
		}
		return result.String(), nil

	case cmdTail:
		n := 10 // default
		if filter.Arg != "" {
			if parsed, err := parseIntArg(filter.Arg); err == nil && parsed > 0 {
				n = parsed
			}
		}
		ring := make([]string, n)
		count := 0
		for line := range strings.SplitSeq(content, "\n") {
			ring[count%n] = line
			count++
		}
		take := min(n, count)
		start := count - take
		var result textbuf.Buffer
		for i := range take {
			if i > 0 {
				result.Byte('\n')
			}
			result.Str(ring[(start+i)%n])
		}
		return result.String(), nil

	case cmdCompare:
		var result textbuf.Buffer
		first := true
		for line := range strings.SplitSeq(content, "\n") {
			if strings.TrimSpace(line) != "" {
				if !first {
					result.Byte('\n')
				}
				result.Str("+ ").Str(line)
				first = false
			}
		}
		if first {
			return "(no changes)", nil
		}
		return result.String(), nil
	}

	return "", fmt.Errorf("unknown pipe filter: %s", filter.Type)
}

// mergeConfigs merges two configuration strings.
// Simple strategy: use current as base, add non-duplicate blocks/keys from merge.
// Existing keys in current are preserved (merge file's duplicates are skipped).
func mergeConfigs(current, merge string) string {
	currentLines := strings.Split(current, "\n")
	mergeLines := strings.Split(merge, "\n")

	// Extract existing keys from current config at depth 1 (inside main block)
	existingKeys := make(map[string]bool)
	depth := 0
	for _, line := range currentLines {
		trimmed := strings.TrimSpace(line)
		openBraces := strings.Count(trimmed, "{")
		closeBraces := strings.Count(trimmed, "}")

		// At depth 1, extract keys
		if depth == 1 && trimmed != "" && trimmed != "}" {
			key := extractConfigKey(trimmed)
			if key != "" {
				existingKeys[key] = true
			}
		}

		depth += openBraces - closeBraces
	}

	// Find the closing brace of the main block in current and insert merge content before it
	result := make([]string, 0, len(currentLines)+len(mergeLines))
	depth = 0
	inserted := false
	mergeDepth := 0
	skipUntilClose := false

	for i, line := range currentLines {
		trimmed := strings.TrimSpace(line)
		depth += strings.Count(trimmed, "{")
		depth -= strings.Count(trimmed, "}")

		// If we're about to close the main block and haven't inserted yet
		if depth == 0 && strings.Contains(trimmed, "}") && !inserted {
			// Insert merge content, skipping duplicates
			for _, mergeLine := range mergeLines {
				mergeTrimmed := strings.TrimSpace(mergeLine)

				// Track depth in merge content
				mergeOpenBraces := strings.Count(mergeTrimmed, "{")
				mergeCloseBraces := strings.Count(mergeTrimmed, "}")

				// Skip top-level block markers
				if mergeTrimmed == "" || mergeTrimmed == "bgp {" || mergeTrimmed == "}" {
					mergeDepth += mergeOpenBraces - mergeCloseBraces
					continue
				}

				// If we're skipping a duplicate block, continue until it closes
				if skipUntilClose {
					mergeDepth += mergeOpenBraces - mergeCloseBraces
					if mergeDepth <= 1 {
						skipUntilClose = false
					}
					continue
				}

				// At depth 1 in merge, check if key already exists
				if mergeDepth == 1 {
					key := extractConfigKey(mergeTrimmed)
					if key != "" && existingKeys[key] {
						// Skip this key/block - it already exists in current
						if mergeOpenBraces > 0 {
							skipUntilClose = true
						}
						mergeDepth += mergeOpenBraces - mergeCloseBraces
						continue
					}
				}

				mergeDepth += mergeOpenBraces - mergeCloseBraces
				result = append(result, mergeLine)
			}
			inserted = true
		}

		result = append(result, currentLines[i])
	}

	return textbuf.Join(result, "\n")
}

// extractConfigKey extracts the key from a config line.
// For "router-id 1.2.3.4;" returns "router-id".
// For "peer 1.1.1.1 {" returns "peer 1.1.1.1".
// For "local-as 65000;" returns "local-as".
func extractConfigKey(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(line, "{")
	line = strings.TrimSuffix(line, ";")
	line = strings.TrimSpace(line)

	// Split into words
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return ""
	}

	// For leaf values like "router-id 1.2.3.4", the key is "router-id"
	// For blocks like "peer 1.1.1.1", the key is "peer 1.1.1.1"
	// Heuristic: if there are 2 parts and first is a known block keyword, use both
	if len(parts) >= 2 {
		first := parts[0]
		// Known block keywords that take a key value
		blockKeywords := map[string]bool{
			"peer": true, "template": true, "plugin": true, "process": true, "group": true,
		}
		if blockKeywords[first] {
			var tb textbuf.Buffer
			return tb.Str(first).Byte(' ').Str(parts[1]).String()
		}
	}

	// Default: just use the first word as the key
	return parts[0]
}

// findPipeIndex returns the index of "|" in tokens, or -1 if not found.
func FindPipeIndex(tokens []string) int {
	for i, t := range tokens {
		if t == "|" {
			return i
		}
	}
	return -1
}

// dispatchWithPipe handles commands with pipe filters.
func (m *Model) dispatchWithPipe(cmdTokens, pipeTokens []string) (commandResult, error) {
	if len(cmdTokens) == 0 {
		return commandResult{}, errNoCommandBeforePipe
	}

	// Parse pipe filters
	filters := ParsePipeFilters(pipeTokens)

	cmd := cmdTokens[0]
	if cmd == cmdShow {
		return m.cmdShowPipe(cmdTokens[1:], filters)
	}

	return commandResult{}, fmt.Errorf("command '%s' does not support piping", cmd)
}

// ParsePipeFilters parses pipe filter tokens into PipeFilter structs.
func ParsePipeFilters(tokens []string) []PipeFilter {
	var filters []PipeFilter
	i := 0

	for i < len(tokens) {
		if tokens[i] == "|" {
			i++
			continue
		}

		filter := PipeFilter{Type: tokens[i]}
		i++

		// Get argument if present
		if i < len(tokens) && tokens[i] != "|" {
			filter.Arg = tokens[i]
			i++
			// "compare rollback N" needs two args: combine "rollback" + "N".
			if filter.Type == cmdCompare && filter.Arg == cmdRollback && i < len(tokens) && tokens[i] != "|" {
				var tb textbuf.Buffer
				filter.Arg = tb.Str(filter.Arg).Byte(' ').Str(tokens[i]).String()
				i++
			}
		}

		filters = append(filters, filter)
	}

	return filters
}

// ShowPipeOpts holds the classified result of show-specific pipe filters.
type ShowPipeOpts struct {
	Format        string       // "tree" (default) or "config"
	CompareTarget string       // "", "confirmed", "saved", "rollback N"
	TreeFilter    string       // "", "active", "inactive"
	Blame         bool         // show | blame
	Changes       bool         // show | changes
	ChangesAll    bool         // show | changes all
	Errors        bool         // show | errors
	History       bool         // show | history
	TextFilters   []PipeFilter // remaining text filters (match, head, tail)
}

// ClassifyShowPipes separates show-specific pipes (format, compare, active/inactive)
// from text filters (match, head, tail). Both the SSH and web CLI call this so the
// classification logic lives in one place.
func ClassifyShowPipes(filters []PipeFilter) (ShowPipeOpts, error) {
	opts := ShowPipeOpts{Format: fmtTree}

	for _, f := range filters {
		switch f.Type {
		case cmdFormat:
			if f.Arg != fmtTree && f.Arg != fmtConfig {
				return ShowPipeOpts{}, fmt.Errorf("unknown format: %s (use tree or config)", f.Arg)
			}
			opts.Format = f.Arg
		case cmdCompare:
			opts.CompareTarget = f.Arg
			if opts.CompareTarget == "" {
				opts.CompareTarget = srcConfirmed
			}
		case cmdActive, cmdInactive:
			opts.TreeFilter = f.Type
		case cmdBlame:
			opts.Blame = true
		case cmdChanges:
			opts.Changes = true
			opts.ChangesAll = f.Arg == cmdAll
		case cmdErrors:
			opts.Errors = true
		case cmdHistory:
			opts.History = true
		default:
			opts.TextFilters = append(opts.TextFilters, f)
		}
	}

	return opts, nil
}

// Helper functions wrapping standard library calls
// These use os, filepath, strconv packages.

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path) //nolint:gosec // Path comes from user input in editor context
}

func isAbsPath(path string) bool {
	return filepath.IsAbs(path)
}

func getDir(path string) string {
	return filepath.Dir(path)
}

func joinPath(base, path string) string {
	return filepath.Join(base, path)
}

func parseIntArg(s string) (int, error) {
	return strconv.Atoi(s)
}
