package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// --- Phase 2: New Editor Features ---

// cmdCommitConfirm commits with auto-rollback if not confirmed within timeout.
// RFC 4271 Section 4.2 doesn't specify this, but it's a standard network CLI feature.
func (m *Model) cmdCommitConfirm(seconds int) (commandResult, error) {
	// Boundary validation: 1-3600 seconds
	if seconds < 1 {
		return commandResult{}, fmt.Errorf("timeout must be at least 1 second")
	}
	if seconds > 3600 {
		return commandResult{}, fmt.Errorf("timeout must be at most 3600 seconds (1 hour)")
	}

	// Validate before commit
	result := m.validator.Validate(m.editor.WorkingContent())
	if len(result.Errors) > 0 {
		return commandResult{}, fmt.Errorf("cannot commit: %d validation error(s). Use 'errors' to see details", len(result.Errors))
	}

	// Save changes (this creates a backup)
	if err := m.editor.Save(); err != nil {
		return commandResult{}, err
	}

	// Notify daemon immediately so it runs the new config during the confirm window
	var reloadWarning string
	if m.editor.HasReloadNotifier() {
		if err := m.editor.NotifyReload(); err != nil {
			reloadWarning = fmt.Sprintf(" (reload failed: %v)", err)
		}
	}

	// Get the most recent backup path for potential rollback
	backups, err := m.editor.ListBackups()
	if err != nil || len(backups) == 0 {
		return commandResult{}, fmt.Errorf("commit succeeded but no backup found for rollback")
	}

	return commandResult{
		statusMessage:     fmt.Sprintf("Committed%s. Confirm within %d seconds or changes will be rolled back. Use 'confirm' or 'abort'.", reloadWarning, seconds),
		setConfirmTimer:   true,
		confirmTimerValue: true,
		confirmBackupPath: backups[0].Path,
	}, nil
}

// cmdConfirm confirms a pending commit, making changes permanent.
// Triggers daemon reload so the confirmed config takes effect.
func (m *Model) cmdConfirm() (commandResult, error) {
	if !m.confirmTimerActive {
		return commandResult{}, fmt.Errorf("no pending commit to confirm")
	}

	msg := "Configuration confirmed and saved permanently."
	if m.editor.HasReloadNotifier() {
		if err := m.editor.NotifyReload(); err != nil {
			msg = fmt.Sprintf("Configuration confirmed (reload failed: %v)", err)
		}
	}

	return commandResult{
		statusMessage:     msg,
		setConfirmTimer:   true,
		confirmTimerValue: false,
		confirmBackupPath: "",
	}, nil
}

// cmdAbort aborts a pending commit and rolls back to previous state.
// Triggers daemon reload so the daemon reverts to the rolled-back config.
func (m *Model) cmdAbort() (commandResult, error) {
	if !m.confirmTimerActive {
		return commandResult{}, fmt.Errorf("no pending commit to abort")
	}

	// Rollback to backup
	if m.confirmBackupPath != "" {
		if err := m.editor.Rollback(m.confirmBackupPath); err != nil {
			return commandResult{
				setConfirmTimer:   true,
				confirmTimerValue: false,
				confirmBackupPath: "",
			}, fmt.Errorf("rollback failed: %w", err)
		}
	}

	msg := "Changes rolled back to previous state."
	if m.editor.HasReloadNotifier() {
		if err := m.editor.NotifyReload(); err != nil {
			msg = fmt.Sprintf("Changes rolled back (reload failed: %v)", err)
		}
	}

	content := m.editor.ContentAtPath(m.contextPath)
	return commandResult{
		statusMessage:     msg,
		configView:        &viewportData{content: content, lineMapping: nil},
		revalidate:        true,
		setConfirmTimer:   true,
		confirmTimerValue: false,
		confirmBackupPath: "",
	}, nil
}

// cmdLoad loads configuration from a file, replacing current content.
func (m *Model) cmdLoad(args []string) (commandResult, error) {
	if len(args) < 1 {
		return commandResult{}, fmt.Errorf("usage: load <file>")
	}

	loadPath := m.resolveConfigPath(args[0])

	data, err := readFile(loadPath)
	if err != nil {
		return commandResult{}, fmt.Errorf("cannot read %s: %w", args[0], err)
	}

	m.editor.SetWorkingContent(string(data))
	m.editor.MarkDirty()

	content := m.editor.ContentAtPath(m.contextPath)
	return commandResult{
		statusMessage: fmt.Sprintf("Configuration loaded from %s", args[0]),
		configView:    &viewportData{content: content, lineMapping: nil},
		revalidate:    true,
	}, nil
}

// cmdLoadMerge loads configuration from a file and merges with current content.
func (m *Model) cmdLoadMerge(args []string) (commandResult, error) {
	if len(args) < 1 {
		return commandResult{}, fmt.Errorf("usage: load merge <file>")
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

	content := m.editor.ContentAtPath(m.contextPath)
	return commandResult{
		statusMessage: fmt.Sprintf("Configuration merged from %s", args[0]),
		configView:    &viewportData{content: content, lineMapping: nil},
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
	if action == loadActionReplace {
		m.editor.SetWorkingContent(content)
		m.editor.MarkDirty()
		return commandResult{
			statusMessage: fmt.Sprintf("Configuration loaded from %s", path),
			configView:    &viewportData{content: m.editor.ContentAtPath(m.contextPath), lineMapping: nil},
			revalidate:    true,
		}, nil
	}

	// action == "merge"
	currentContent := m.editor.WorkingContent()
	merged := mergeConfigs(currentContent, content)
	m.editor.SetWorkingContent(merged)
	m.editor.MarkDirty()
	return commandResult{
		statusMessage: fmt.Sprintf("Configuration merged from %s", path),
		configView:    &viewportData{content: m.editor.ContentAtPath(m.contextPath), lineMapping: nil},
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

	return commandResult{
		statusMessage: fmt.Sprintf("Configuration %s from %s at %s", verb, path, strings.Join(m.contextPath, " ")),
		configView:    &viewportData{content: m.editor.ContentAtPath(m.contextPath), lineMapping: nil},
		revalidate:    true,
	}, nil
}

// replaceAtContext replaces the content at the given context path with new content.
func replaceAtContext(fullConfig string, contextPath []string, newContent string) string {
	if len(contextPath) == 0 {
		return fullConfig // nothing to replace
	}

	lines := strings.Split(fullConfig, "\n")
	var result strings.Builder

	// Build the pattern to match (e.g., "peer 1.1.1.1" or just "bgp")
	var targetPattern string
	if len(contextPath) == 1 {
		targetPattern = contextPath[0]
	} else {
		// len >= 2: combine last two elements (e.g., "peer" + "1.1.1.1")
		targetPattern = contextPath[len(contextPath)-2] + " " + contextPath[len(contextPath)-1]
	}

	inTarget := false
	targetDepth := 0
	currentDepth := 0

	for _, line := range lines {
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
					result.WriteString(line)
					result.WriteString("\n")
					inTarget = true
					targetDepth = currentDepth + openBraces

					// Write indented new content
					indent := strings.Repeat("  ", targetDepth)
					for newLine := range strings.SplitSeq(strings.TrimSpace(newContent), "\n") {
						result.WriteString(indent)
						result.WriteString(newLine)
						result.WriteString("\n")
					}
					currentDepth += openBraces - closeBraces
					continue
				}
			}
			result.WriteString(line)
			result.WriteString("\n")
		} else {
			// Inside target - skip old content until closing brace
			newDepth := currentDepth + openBraces - closeBraces
			if newDepth < targetDepth {
				// Found closing brace - write it
				result.WriteString(line)
				result.WriteString("\n")
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

	lines := strings.Split(fullConfig, "\n")
	var result strings.Builder

	// Build the pattern to match (e.g., "peer 1.1.1.1" or just "bgp")
	var targetPattern string
	if len(contextPath) == 1 {
		targetPattern = contextPath[0]
	} else {
		// len >= 2: combine last two elements (e.g., "peer" + "1.1.1.1")
		targetPattern = contextPath[len(contextPath)-2] + " " + contextPath[len(contextPath)-1]
	}

	inTarget := false
	targetDepth := 0
	currentDepth := 0
	contentInserted := false

	for _, line := range lines {
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
			}
			result.WriteString(line)
			result.WriteString("\n")
		} else {
			newDepth := currentDepth + openBraces - closeBraces
			if newDepth < targetDepth && !contentInserted {
				// Insert merged content before closing brace
				indent := strings.Repeat("  ", targetDepth)
				for newLine := range strings.SplitSeq(strings.TrimSpace(newContent), "\n") {
					result.WriteString(indent)
					result.WriteString(newLine)
					result.WriteString("\n")
				}
				contentInserted = true
				inTarget = false
			}
			result.WriteString(line)
			result.WriteString("\n")
		}

		currentDepth += openBraces - closeBraces
	}

	return strings.TrimSuffix(result.String(), "\n")
}

// cmdShowPipe executes show with pipe filters.
func (m *Model) cmdShowPipe(_ []string, filters []PipeFilter) (commandResult, error) {
	content := m.editor.ContentAtPath(m.contextPath)
	if content == "" {
		return commandResult{output: "(empty configuration)"}, nil
	}

	// Apply pipe filters
	output := content
	for _, filter := range filters {
		var err error
		output, err = applyPipeFilter(output, filter)
		if err != nil {
			return commandResult{}, err
		}
	}

	return commandResult{output: output}, nil
}

// applyPipeFilter applies a single pipe filter to content.
// Returns error for unknown filter types.
func applyPipeFilter(content string, filter PipeFilter) (string, error) {
	lines := strings.Split(content, "\n")

	switch filter.Type {
	case "grep":
		var matched []string
		for _, line := range lines {
			if strings.Contains(line, filter.Arg) {
				matched = append(matched, line)
			}
		}
		return strings.Join(matched, "\n"), nil

	case "head":
		n := 10 // default
		if filter.Arg != "" {
			if parsed, err := parseIntArg(filter.Arg); err == nil && parsed > 0 {
				n = parsed
			}
		}
		if n > len(lines) {
			n = len(lines)
		}
		return strings.Join(lines[:n], "\n"), nil

	case "tail":
		n := 10 // default
		if filter.Arg != "" {
			if parsed, err := parseIntArg(filter.Arg); err == nil && parsed > 0 {
				n = parsed
			}
		}
		if n > len(lines) {
			n = len(lines)
		}
		return strings.Join(lines[len(lines)-n:], "\n"), nil

	case "compare":
		// Compare filter marks each line with + or - based on content
		// This is a simplified version - it just prefixes lines to indicate changes
		// A proper implementation would need the original content to compute a real diff
		var result []string
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				result = append(result, "+ "+line)
			}
		}
		if len(result) == 0 {
			return "(no changes)", nil
		}
		return strings.Join(result, "\n"), nil
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

	return strings.Join(result, "\n")
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
			return first + " " + parts[1]
		}
	}

	// Default: just use the first word as the key
	return parts[0]
}

// findPipeIndex returns the index of "|" in tokens, or -1 if not found.
func findPipeIndex(tokens []string) int {
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
		return commandResult{}, fmt.Errorf("no command before pipe")
	}

	// Parse pipe filters
	filters := parsePipeFilters(pipeTokens)

	// Only show supports piping currently
	cmd := cmdTokens[0]
	switch cmd {
	case "show":
		return m.cmdShowPipe(cmdTokens[1:], filters)
	case "errors":
		result, err := m.cmdErrors()
		if err != nil {
			return result, err
		}
		// Apply filters to errors output
		for _, f := range filters {
			result.output, err = applyPipeFilter(result.output, f)
			if err != nil {
				return commandResult{}, err
			}
		}
		return result, nil
	}

	return commandResult{}, fmt.Errorf("command '%s' does not support piping", cmd)
}

// parsePipeFilters parses pipe filter tokens into PipeFilter structs.
func parsePipeFilters(tokens []string) []PipeFilter {
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
		}

		filters = append(filters, filter)
	}

	return filters
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
