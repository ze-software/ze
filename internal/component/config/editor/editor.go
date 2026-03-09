// Design: docs/architecture/config/yang-config-design.md — config editor
//
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
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// ReloadNotifier is called after a successful save to notify the running daemon.
// Returns nil on success, or an error if the daemon could not be reached.
type ReloadNotifier func() error

// Editor manages an editing session for a configuration file.
// The tree is the canonical in-memory representation when treeValid is true.
// WorkingContent() returns Serialize(tree) when tree is valid, otherwise falls
// back to stored raw text for configs that can't be parsed.
type Editor struct {
	originalPath    string
	originalContent string
	workingContent  string         // Fallback when tree can't parse
	tree            *config.Tree   // Parsed config tree (canonical when treeValid)
	schema          *config.Schema // YANG schema for Serialize
	treeValid       bool           // True when tree was parsed successfully
	dirty           atomic.Bool
	hasPendingEdit  bool           // true if .edit file exists
	onReload        ReloadNotifier // Optional: called after successful save
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

	// Parse succeeded if tree has content (not the empty fallback)
	treeValid := err == nil

	return &Editor{
		originalPath:    configPath,
		originalContent: content,
		workingContent:  content,
		tree:            tree,
		schema:          schema,
		treeValid:       treeValid,
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
	return e.dirty.Load()
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
	e.dirty.Store(true)
	e.hasPendingEdit = false // Loaded, no longer "pending"
	return nil
}

// SaveEditState saves the current working content to the .edit file.
func (e *Editor) SaveEditState() error {
	if !e.dirty.Load() {
		return nil // Nothing to save
	}

	editPath := e.originalPath + ".edit"
	if err := os.WriteFile(editPath, []byte(e.workingContent), 0o600); err != nil {
		return fmt.Errorf("failed to write edit file: %w", err)
	}
	return nil
}

// deleteEditFile removes the .edit file if it exists.
func (e *Editor) deleteEditFile() {
	editPath := e.originalPath + ".edit"
	_ = os.Remove(editPath) // Ignore error if doesn't exist
}

// SetReloadNotifier sets an optional function to notify the daemon after save.
// When set, commit will call this after writing config to disk.
// When nil (standalone mode), no notification is attempted.
func (e *Editor) SetReloadNotifier(fn ReloadNotifier) {
	e.onReload = fn
}

// HasReloadNotifier returns true if a reload notifier is configured.
// Use this to distinguish "no daemon" from "reload succeeded".
func (e *Editor) HasReloadNotifier() bool {
	return e.onReload != nil
}

// NotifyReload calls the reload notifier if one is configured.
// Returns nil if no notifier is set or if notification succeeds.
func (e *Editor) NotifyReload() error {
	if e.onReload == nil {
		return nil
	}
	return e.onReload()
}

// MarkDirty marks the editor as having unsaved changes.
func (e *Editor) MarkDirty() {
	e.dirty.Store(true)
}

// OriginalContent returns the original file content.
func (e *Editor) OriginalContent() string {
	return e.originalContent
}

// WorkingContent returns the current working content.
// When tree is valid, returns Serialize(tree, schema); otherwise raw text.
func (e *Editor) WorkingContent() string {
	if e.treeValid && e.tree != nil && e.schema != nil {
		return config.Serialize(e.tree, e.schema)
	}
	return e.workingContent
}

// SetWorkingContent sets the working content and parses it into the tree.
// If parsing fails, falls back to raw text mode (treeValid = false).
func (e *Editor) SetWorkingContent(content string) {
	e.workingContent = content
	if e.schema != nil {
		parser := config.NewParser(e.schema)
		tree, err := parser.Parse(content)
		if err == nil {
			e.tree = tree
			e.treeValid = true
		} else {
			e.treeValid = false
		}
	}
}

// ContentAtPath returns the serialized content at the given context path.
// If path is empty, returns the full WorkingContent().
// If the path doesn't resolve, falls back to full content.
func (e *Editor) ContentAtPath(path []string) string {
	if len(path) == 0 {
		return e.WorkingContent()
	}
	if !e.treeValid || e.tree == nil || e.schema == nil {
		return e.workingContent
	}

	subtree, schemaNode := e.walkPathWithSchema(path)
	if subtree == nil || schemaNode == nil {
		return e.WorkingContent()
	}
	return config.SerializeSubtree(subtree, schemaNode)
}

// schemaGetter is any schema node that can look up children by name.
// Satisfied by *config.Schema, *config.ContainerNode, *config.ListNode, *config.FlexNode.
type schemaGetter interface {
	Get(name string) config.Node
}

// walkPathWithSchema navigates tree and schema in parallel, returning both
// the subtree and the schema node at the destination.
func (e *Editor) walkPathWithSchema(path []string) (*config.Tree, config.Node) {
	if e.tree == nil || e.schema == nil || len(path) == 0 {
		return nil, nil
	}

	currentTree := e.tree
	var currentSchema schemaGetter = e.schema

	i := 0
	for i < len(path) {
		name := path[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			return nil, nil
		}

		navigable, next, step := walkSchemaNode(schemaNode, currentTree, name, path, i)
		if !navigable || next == nil {
			return nil, nil
		}
		currentTree = next

		switch n := schemaNode.(type) {
		case *config.ContainerNode:
			currentSchema = n
		case *config.ListNode:
			currentSchema = n
		case *config.FlexNode:
			currentSchema = n
		}
		i += step
	}

	// Return the tree and the last schema node we navigated through
	node, ok := currentSchema.(config.Node)
	if !ok {
		return nil, nil
	}
	return currentTree, node
}

// WalkPath navigates the tree using the schema to distinguish containers from list keys.
// Returns the subtree at the given path, or nil if the path doesn't resolve.
func (e *Editor) WalkPath(path []string) *config.Tree {
	if e.tree == nil || e.schema == nil || len(path) == 0 {
		return nil
	}

	currentTree := e.tree
	var currentSchema schemaGetter = e.schema

	i := 0
	for i < len(path) {
		name := path[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			return nil
		}

		navigable, next, step := walkSchemaNode(schemaNode, currentTree, name, path, i)
		if !navigable || next == nil {
			return nil
		}
		currentTree = next

		switch n := schemaNode.(type) {
		case *config.ContainerNode:
			currentSchema = n
		case *config.ListNode:
			currentSchema = n
		case *config.FlexNode:
			currentSchema = n
		}
		i += step
	}

	return currentTree
}

// walkSchemaNode resolves one step of tree navigation based on the schema node type.
// Returns (navigable, subtree, step). step is how many path elements were consumed
// (2 for keyed list entries, 1 for containers and anonymous list entries).
func walkSchemaNode(schemaNode config.Node, tree *config.Tree, name string, path []string, i int) (bool, *config.Tree, int) {
	switch n := schemaNode.(type) {
	case *config.ContainerNode:
		return true, tree.GetContainer(name), 1
	case *config.ListNode:
		entries := tree.GetList(name)
		// Determine if next path element is a key or an anonymous entry.
		// Anonymous: no next element, or next element is a schema child of the list.
		if i+1 >= len(path) || n.Get(path[i+1]) != nil {
			// Anonymous list entry — use KeyDefault
			if entries == nil {
				return true, nil, 1
			}
			return true, entries[config.KeyDefault], 1
		}
		// Keyed list entry — next path element is the key
		key := path[i+1]
		if entries == nil {
			return true, nil, 2
		}
		// Resolve #N positional index to actual key
		if entry := resolveListKey(tree, name, key); entry != nil {
			return true, entry, 2
		}
		return true, entries[key], 2
	case *config.FlexNode:
		return true, tree.GetContainer(name), 1
	case *config.LeafNode, *config.FreeformNode,
		*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode,
		*config.InlineListNode:
		return false, nil, 0 // Leaf-like nodes — can't navigate deeper
	}
	return false, nil, 0 // Unknown node type
}

// AutoSelectListEntry checks if the path ends at a list node with exactly one entry.
// If so, it returns the expanded path with the single entry's key appended.
// Otherwise returns the original path unchanged.
func (e *Editor) AutoSelectListEntry(path []string) []string {
	if e.schema == nil || e.tree == nil || len(path) == 0 {
		return path
	}

	// Navigate to the parent and check if the last element is a list
	lastElem := path[len(path)-1]
	var parentSchema schemaGetter = e.schema
	parentTree := e.tree

	// Walk to the parent of the last element
	for i := 0; i < len(path)-1; i++ {
		name := path[i]
		schemaNode := parentSchema.Get(name)
		if schemaNode == nil {
			return path
		}
		_, next, step := walkSchemaNode(schemaNode, parentTree, name, path, i)
		if next == nil {
			return path
		}
		parentTree = next
		switch n := schemaNode.(type) {
		case *config.ContainerNode:
			parentSchema = n
		case *config.ListNode:
			parentSchema = n
		case *config.FlexNode:
			parentSchema = n
		}
		i += step - 1 // -1 because the for loop increments
	}

	schemaNode := parentSchema.Get(lastElem)
	if _, ok := schemaNode.(*config.ListNode); !ok {
		return path
	}

	entries := parentTree.GetListOrdered(lastElem)
	if len(entries) != 1 {
		return path
	}

	// Single entry — expand path with the entry's key
	return append(path, entries[0].Key)
}

// resolveListKey resolves a #N positional index to the actual list entry.
// Returns nil if the key is not a positional index or the index is out of range.
func resolveListKey(tree *config.Tree, listName, key string) *config.Tree {
	if !strings.HasPrefix(key, "#") {
		return nil
	}
	idx, err := strconv.Atoi(key[1:])
	if err != nil || idx < 1 {
		return nil
	}
	ordered := tree.GetListOrdered(listName)
	if idx > len(ordered) {
		return nil
	}
	return ordered[idx-1].Value
}

// walkOrCreate navigates the tree, creating containers along the way.
func (e *Editor) walkOrCreate(path []string) (*config.Tree, error) {
	if e.tree == nil || e.schema == nil {
		return nil, fmt.Errorf("tree or schema not available")
	}
	if len(path) == 0 {
		return e.tree, nil
	}

	currentTree := e.tree
	var currentSchema schemaGetter = e.schema

	i := 0
	for i < len(path) {
		name := path[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			return nil, fmt.Errorf("unknown path element: %s", name)
		}

		switch n := schemaNode.(type) {
		case *config.ContainerNode:
			currentTree = currentTree.GetOrCreateContainer(name)
			currentSchema = n
			i++
		case *config.ListNode:
			// Determine anonymous vs keyed: anonymous if no next element
			// or next element is a schema child of the list.
			var key string
			var step int
			if i+1 >= len(path) || n.Get(path[i+1]) != nil {
				key = config.KeyDefault
				step = 1
			} else {
				key = path[i+1]
				step = 2
			}
			entries := currentTree.GetList(name)
			if entries == nil || entries[key] == nil {
				entry := config.NewTree()
				currentTree.AddListEntry(name, key, entry)
				currentTree = entry
			} else {
				currentTree = entries[key]
			}
			currentSchema = n
			i += step
		case *config.FlexNode:
			currentTree = currentTree.GetOrCreateContainer(name)
			currentSchema = n
			i++
		case *config.LeafNode, *config.FreeformNode,
			*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode,
			*config.InlineListNode:
			return nil, fmt.Errorf("cannot navigate into %s (leaf node)", name)
		}
	}

	return currentTree, nil
}

// SetValue sets a leaf value at the given path in the tree.
func (e *Editor) SetValue(path []string, key, value string) error {
	target, err := e.walkOrCreate(path)
	if err != nil {
		return err
	}
	target.Set(key, value)
	e.dirty.Store(true)
	return nil
}

// DeleteValue removes a leaf value at the given path in the tree.
func (e *Editor) DeleteValue(path []string, key string) error {
	target := e.WalkPath(path)
	if target == nil {
		return fmt.Errorf("path not found")
	}
	target.Delete(key)
	e.dirty.Store(true)
	return nil
}

// DeleteContainer removes a container at the given path in the tree.
func (e *Editor) DeleteContainer(path []string, name string) error {
	var target *config.Tree
	if len(path) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(path)
	}
	if target == nil {
		return fmt.Errorf("path not found")
	}
	target.DeleteContainer(name)
	e.dirty.Store(true)
	return nil
}

// DeleteByPath deletes the element at the given absolute path using schema awareness.
// It determines whether the target is a leaf, container, or list entry and calls
// the appropriate delete method.
func (e *Editor) DeleteByPath(fullPath []string) error {
	if len(fullPath) == 0 {
		return fmt.Errorf("empty path")
	}
	if e.schema == nil {
		return fmt.Errorf("schema not available")
	}

	// Walk the schema to find what the second-to-last element is.
	// If it's a ListNode, the last element is a key → DeleteListEntry.
	// Otherwise, last element is a leaf or container name.
	if len(fullPath) >= 2 {
		possibleListName := fullPath[len(fullPath)-2]
		possibleKey := fullPath[len(fullPath)-1]
		parentPath := fullPath[:len(fullPath)-2]

		// Walk schema to the parent of possibleListName
		parentSchema := e.walkSchema(parentPath)
		if parentSchema != nil {
			schemaNode := parentSchema.Get(possibleListName)
			if _, isList := schemaNode.(*config.ListNode); isList {
				return e.DeleteListEntry(parentPath, possibleListName, possibleKey)
			}
		}
	}

	// Not a list entry: try leaf delete, then container delete
	target := fullPath[len(fullPath)-1]
	parentPath := fullPath[:len(fullPath)-1]

	if err := e.DeleteValue(parentPath, target); err != nil {
		if errC := e.DeleteContainer(parentPath, target); errC != nil {
			return fmt.Errorf("not found: %s", strings.Join(fullPath, " "))
		}
	}
	return nil
}

// walkSchema walks the schema tree along the given path, returning the schema node
// at the end of the path (or nil if any element is not found or not navigable).
func (e *Editor) walkSchema(path []string) schemaGetter {
	var current schemaGetter = e.schema
	for _, name := range path {
		node := current.Get(name)
		if node == nil {
			return nil
		}
		switch n := node.(type) {
		case *config.ContainerNode:
			current = n
		case *config.ListNode:
			current = n
		case *config.FlexNode:
			current = n
		case *config.LeafNode, *config.FreeformNode,
			*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode,
			*config.InlineListNode:
			return nil // Can't navigate into leaf nodes
		}
	}
	return current
}

// DeleteListEntry removes a list entry at the given path in the tree.
func (e *Editor) DeleteListEntry(path []string, listName, key string) error {
	var target *config.Tree
	if len(path) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(path)
	}
	if target == nil {
		return fmt.Errorf("path not found")
	}
	target.RemoveListEntry(listName, key)
	e.dirty.Store(true)
	return nil
}

// Save commits changes: creates backup of original, writes serialized tree.
func (e *Editor) Save() error {
	if !e.dirty.Load() {
		return nil
	}

	// Create backup of original
	if _, err := e.createBackup(); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Write serialized tree (or raw text fallback) to original path
	content := e.WorkingContent()
	if err := os.WriteFile(e.originalPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Update original to match saved
	e.originalContent = content
	e.dirty.Store(false)

	// Delete edit file on successful commit
	e.deleteEditFile()

	return nil
}

// Discard reverts working content and tree to original state.
func (e *Editor) Discard() error {
	e.workingContent = e.originalContent
	e.dirty.Store(false)

	// Re-parse original content into tree
	if e.schema != nil {
		parser := config.NewParser(e.schema)
		tree, err := parser.Parse(e.originalContent)
		if err == nil {
			e.tree = tree
			e.treeValid = true
		}
	}

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
	if err := os.WriteFile(backupPath, []byte(e.originalContent), 0o600); err != nil {
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

// LivePath returns the path to the .live.conf file.
// This file holds the trial config during a "commit confirmed" window.
func (e *Editor) LivePath() string {
	dir := filepath.Dir(e.originalPath)
	base := filepath.Base(e.originalPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, name+".live"+ext)
}

// SaveLive writes the current working content to the .live.conf file.
// Used by "commit confirmed" to create the trial config.
func (e *Editor) SaveLive() error {
	content := e.WorkingContent()
	if err := os.WriteFile(e.LivePath(), []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to write live config: %w", err)
	}
	return nil
}

// HasPendingLive returns true if a .live.conf file exists.
// This indicates an unconfirmed "commit confirmed" from a previous session.
func (e *Editor) HasPendingLive() bool {
	_, err := os.Stat(e.LivePath())
	return err == nil
}

// DeleteLive removes the .live.conf file if it exists.
// Errors are ignored because the file may not exist.
func (e *Editor) DeleteLive() {
	livePath := e.LivePath()
	if err := os.Remove(livePath); err != nil && !os.IsNotExist(err) {
		// Best-effort removal — log but don't fail
		return
	}
}

// Rollback restores the configuration from a backup file.
func (e *Editor) Rollback(backupPath string) error {
	// Read backup content
	data, err := os.ReadFile(backupPath) //nolint:gosec // Backup path from ListBackups
	if err != nil {
		return fmt.Errorf("cannot read backup: %w", err)
	}

	// Write to original path
	if err := os.WriteFile(e.originalPath, data, 0o600); err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}

	// Update editor state and re-parse into tree
	content := string(data)
	e.originalContent = content
	e.workingContent = content
	e.dirty.Store(false)

	// Re-parse the restored content into the tree
	if e.schema != nil {
		parser := config.NewParser(e.schema)
		tree, err := parser.Parse(content)
		if err == nil {
			e.tree = tree
			e.treeValid = true
		} else {
			e.treeValid = false
		}
	}

	return nil
}
