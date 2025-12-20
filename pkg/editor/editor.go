// Package editor provides an interactive configuration editor.
package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/exa-networks/zebgp/pkg/config"
)

// Editor manages an editing session for a configuration file.
type Editor struct {
	originalPath    string
	originalContent string
	workingContent  string
	schema          *config.Schema
	tree            *config.Tree // Parsed config tree
	dirty           bool
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
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}

	content := string(data)
	schema := config.BGPSchema()

	// Parse config into tree
	parser := config.NewParser(schema)
	tree, err := parser.Parse(content)
	if err != nil {
		// Non-fatal: allow editing invalid configs
		tree = config.NewTree()
	}

	return &Editor{
		originalPath:    configPath,
		originalContent: content,
		workingContent:  content,
		schema:          schema,
		tree:            tree,
		dirty:           false,
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

// MarkDirty marks the editor as having unsaved changes.
func (e *Editor) MarkDirty() {
	e.dirty = true
}

// Schema returns the configuration schema.
func (e *Editor) Schema() *config.Schema {
	return e.schema
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
	if err := os.WriteFile(e.originalPath, []byte(e.workingContent), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Update original to match working
	e.originalContent = e.workingContent
	e.dirty = false

	return nil
}

// Discard reverts working content to original.
func (e *Editor) Discard() error {
	e.workingContent = e.originalContent
	e.dirty = false
	return nil
}

// Diff returns a simple diff between original and working content.
func (e *Editor) Diff() string {
	if e.originalContent == e.workingContent {
		return ""
	}

	originalLines := strings.Split(e.originalContent, "\n")
	workingLines := strings.Split(e.workingContent, "\n")

	originalSet := make(map[string]bool)
	for _, line := range originalLines {
		if strings.TrimSpace(line) != "" {
			originalSet[line] = true
		}
	}

	workingSet := make(map[string]bool)
	for _, line := range workingLines {
		if strings.TrimSpace(line) != "" {
			workingSet[line] = true
		}
	}

	var diff strings.Builder

	// Removed lines
	for _, line := range originalLines {
		if strings.TrimSpace(line) != "" && !workingSet[line] {
			diff.WriteString("- ")
			diff.WriteString(line)
			diff.WriteString("\n")
		}
	}

	// Added lines
	for _, line := range workingLines {
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
	if err := os.WriteFile(backupPath, []byte(e.originalContent), 0644); err != nil {
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

	var backups []BackupInfo
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
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("cannot read backup: %w", err)
	}

	// Write to original path
	if err := os.WriteFile(e.originalPath, data, 0644); err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}

	// Update editor state
	content := string(data)
	e.originalContent = content
	e.workingContent = content
	e.dirty = false

	return nil
}
