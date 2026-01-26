// Package editor provides an interactive configuration editor.
package editor

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/config"
)

// Editor manages an editing session for a configuration file.
type Editor struct {
	originalPath    string
	originalContent string
	workingContent  string
	tree            *config.Tree // Parsed config tree
	dirty           bool
	hasPendingEdit  bool // true if .edit file exists
}

// BackupInfo describes a backup file.
type BackupInfo struct {
	Path      string
	Timestamp time.Time
	Number    int
}

// NewEditor creates a new editor for the given configuration file.
func NewEditor(configPath string) (*Editor, error) {
	// Read original file
	data, err := os.ReadFile(configPath) //nolint:gosec // Config path is user-provided
	if err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}

	content := string(data)

	// Parse config into tree using YANG-derived schema
	schema := config.YANGSchema()
	if schema == nil {
		return nil, fmt.Errorf("failed to load YANG schema")
	}
	parser := config.NewParser(schema)
	tree, err := parser.Parse(content)
	if err != nil {
		// Non-fatal: allow editing invalid configs
		tree = config.NewTree()
	}

	// Check for existing edit file
	editPath := configPath + ".edit"
	hasPending := false
	if _, err := os.Stat(editPath); err == nil {
		hasPending = true
	}

	return &Editor{
		originalPath:    configPath,
		originalContent: content,
		workingContent:  content,
		tree:            tree,
		dirty:           false,
		hasPendingEdit:  hasPending,
	}, nil
}

// Tree returns the parsed configuration tree.
func (e *Editor) Tree() *config.Tree {
	return e.tree
}

// ListKeys returns the keys for a list at the given path (e.g., "neighbor").
func (e *Editor) ListKeys(listName string) []string {
	if e.tree == nil {
		return nil
	}
	return e.tree.ListKeys(listName)
}

// Close cleans up any resources.
func (e *Editor) Close() error {
	return nil
}

// OriginalPath returns the path to the original configuration file.
func (e *Editor) OriginalPath() string {
	return e.originalPath
}

// Dirty returns true if there are unsaved changes.
func (e *Editor) Dirty() bool {
	return e.dirty
}

// HasPendingEdit returns true if an edit file exists from a previous session.
func (e *Editor) HasPendingEdit() bool {
	return e.hasPendingEdit
}

// PendingEditTime returns the modification time of the .edit file.
// Returns zero time if no edit file exists.
func (e *Editor) PendingEditTime() time.Time {
	editPath := e.originalPath + ".edit"
	info, err := os.Stat(editPath)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// PendingEditDiff returns the diff between original and pending edit content.
// Returns empty string if no edit file exists.
func (e *Editor) PendingEditDiff() string {
	editPath := e.originalPath + ".edit"
	data, err := os.ReadFile(editPath) //nolint:gosec // Edit path derived from original
	if err != nil {
		return ""
	}
	return computeDiff(e.originalContent, string(data))
}

// PendingEditAction represents user's choice for pending edit file.
type PendingEditAction int

const (
	// PendingEditContinue - continue editing from pending file.
	PendingEditContinue PendingEditAction = iota
	// PendingEditDiscard - discard pending file, start fresh.
	PendingEditDiscard
	// PendingEditQuit - quit without editing.
	PendingEditQuit
)

// PromptPendingEdit prompts user about existing uncommitted changes.
// Reads from stdin, writes to stdout.
func (e *Editor) PromptPendingEdit() PendingEditAction {
	modTime := e.PendingEditTime()
	timeStr := modTime.Format("2006-01-02 15:04")

	fmt.Printf("\nFound uncommitted changes from %s.\n", timeStr)
	fmt.Println("  [c] Continue editing")
	fmt.Println("  [d] Discard and start fresh")
	fmt.Println("  [v] View changes first")
	fmt.Println("  [q] Quit")

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Choice: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return PendingEditQuit
		}

		choice := strings.ToLower(strings.TrimSpace(input))
		switch choice {
		case "c":
			return PendingEditContinue
		case "d":
			return PendingEditDiscard
		case "v":
			diff := e.PendingEditDiff()
			if diff == "" {
				fmt.Println("\nNo differences found.")
			} else {
				fmt.Println("\nChanges:")
				fmt.Println(diff)
			}
			// After viewing, prompt again
			fmt.Println("  [c] Continue editing")
			fmt.Println("  [d] Discard and start fresh")
			fmt.Println("  [q] Quit")
		case "q":
			return PendingEditQuit
		default:
			fmt.Println("Invalid choice. Enter c, d, v, or q.")
		}
	}
}

// LoadPendingEdit loads the content from the .edit file.
func (e *Editor) LoadPendingEdit() error {
	editPath := e.originalPath + ".edit"
	data, err := os.ReadFile(editPath) //nolint:gosec // Edit path derived from original
	if err != nil {
		return fmt.Errorf("cannot read edit file: %w", err)
	}

	e.workingContent = string(data)
	e.dirty = true
	e.hasPendingEdit = false // Loaded, no longer "pending"
	return nil
}

// SaveEditState saves the current working content to the .edit file.
func (e *Editor) SaveEditState() error {
	if !e.dirty {
		return nil // Nothing to save
	}

	editPath := e.originalPath + ".edit"
	if err := os.WriteFile(editPath, []byte(e.workingContent), 0600); err != nil {
		return fmt.Errorf("failed to write edit file: %w", err)
	}
	return nil
}

// deleteEditFile removes the .edit file if it exists.
func (e *Editor) deleteEditFile() {
	editPath := e.originalPath + ".edit"
	_ = os.Remove(editPath) // Ignore error if doesn't exist
}

// MarkDirty marks the editor as having unsaved changes.
func (e *Editor) MarkDirty() {
	e.dirty = true
}

// OriginalContent returns the original file content.
func (e *Editor) OriginalContent() string {
	return e.originalContent
}

// WorkingContent returns the current working content.
func (e *Editor) WorkingContent() string {
	return e.workingContent
}

// SetWorkingContent sets the working content.
func (e *Editor) SetWorkingContent(content string) {
	e.workingContent = content
}

// Save commits changes: creates backup of original, writes working content.
func (e *Editor) Save() error {
	if !e.dirty {
		return nil
	}

	// Create backup of original
	if _, err := e.createBackup(); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Write working content to original path
	if err := os.WriteFile(e.originalPath, []byte(e.workingContent), 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Update original to match working
	e.originalContent = e.workingContent
	e.dirty = false

	// Delete edit file on successful commit
	e.deleteEditFile()

	return nil
}

// Discard reverts working content to original.
func (e *Editor) Discard() error {
	e.workingContent = e.originalContent
	e.dirty = false

	// Delete edit file on discard
	e.deleteEditFile()

	return nil
}

// Diff returns a simple diff between original and working content.
func (e *Editor) Diff() string {
	return computeDiff(e.originalContent, e.workingContent)
}

// computeDiff computes a simple line-based diff between two strings.
func computeDiff(original, modified string) string {
	if original == modified {
		return ""
	}

	originalLines := strings.Split(original, "\n")
	modifiedLines := strings.Split(modified, "\n")

	originalSet := make(map[string]bool)
	for _, line := range originalLines {
		if strings.TrimSpace(line) != "" {
			originalSet[line] = true
		}
	}

	modifiedSet := make(map[string]bool)
	for _, line := range modifiedLines {
		if strings.TrimSpace(line) != "" {
			modifiedSet[line] = true
		}
	}

	var diff strings.Builder

	// Removed lines
	for _, line := range originalLines {
		if strings.TrimSpace(line) != "" && !modifiedSet[line] {
			diff.WriteString("- ")
			diff.WriteString(line)
			diff.WriteString("\n")
		}
	}

	// Added lines
	for _, line := range modifiedLines {
		if strings.TrimSpace(line) != "" && !originalSet[line] {
			diff.WriteString("+ ")
			diff.WriteString(line)
			diff.WriteString("\n")
		}
	}

	return diff.String()
}

// createBackup creates a backup of the original file.
func (e *Editor) createBackup() (string, error) {
	dir := filepath.Dir(e.originalPath)
	base := filepath.Base(e.originalPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	today := time.Now().Format("2006-01-02")

	// Find next number for today
	num := e.nextBackupNumber(dir, name, today)

	backupPath := filepath.Join(dir, fmt.Sprintf("%s-%s-%d.conf", name, today, num))

	// Copy original content to backup
	if err := os.WriteFile(backupPath, []byte(e.originalContent), 0600); err != nil {
		return "", err
	}

	return backupPath, nil
}

// nextBackupNumber finds the next backup number for the given date.
func (e *Editor) nextBackupNumber(dir, name, date string) int {
	pattern := filepath.Join(dir, fmt.Sprintf("%s-%s-*.conf", name, date))
	matches, _ := filepath.Glob(pattern)

	maxNum := 0
	re := regexp.MustCompile(`-(\d+)\.conf$`)

	for _, match := range matches {
		if m := re.FindStringSubmatch(match); len(m) > 1 {
			if n, err := strconv.Atoi(m[1]); err == nil && n > maxNum {
				maxNum = n
			}
		}
	}

	return maxNum + 1
}

// ListBackups returns available backup files, sorted by date descending.
func (e *Editor) ListBackups() ([]BackupInfo, error) {
	dir := filepath.Dir(e.originalPath)
	base := filepath.Base(e.originalPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// Pattern: name-YYYY-MM-DD-N.conf
	pattern := filepath.Join(dir, fmt.Sprintf("%s-????-??-??-*.conf", name))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	backups := make([]BackupInfo, 0, len(matches))
	re := regexp.MustCompile(`-(\d{4}-\d{2}-\d{2})-(\d+)\.conf$`)

	for _, path := range matches {
		m := re.FindStringSubmatch(path)
		if len(m) < 3 {
			continue
		}

		dateStr := m[1]
		numStr := m[2]

		ts, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		num, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}

		backups = append(backups, BackupInfo{
			Path:      path,
			Timestamp: ts,
			Number:    num,
		})
	}

	// Sort by timestamp descending, then number descending
	sort.Slice(backups, func(i, j int) bool {
		if backups[i].Timestamp.Equal(backups[j].Timestamp) {
			return backups[i].Number > backups[j].Number
		}
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
}

// Rollback restores the configuration from a backup file.
func (e *Editor) Rollback(backupPath string) error {
	// Read backup content
	data, err := os.ReadFile(backupPath) //nolint:gosec // Backup path from ListBackups
	if err != nil {
		return fmt.Errorf("cannot read backup: %w", err)
	}

	// Write to original path
	if err := os.WriteFile(e.originalPath, data, 0600); err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}

	// Update editor state
	content := string(data)
	e.originalContent = content
	e.workingContent = content
	e.dirty = false

	return nil
}
