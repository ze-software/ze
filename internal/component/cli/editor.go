// Design: docs/architecture/config/yang-config-design.md — config editor
// Detail: editor_draft.go — write-through draft protocol
// Detail: editor_commit.go — commit/discard/disconnect protocol
// Detail: editor_walk.go — schema-aware tree/meta walking
// Detail: editor_session.go — session identity for concurrent editing
//
// Package editor provides an interactive configuration editor.
package cli

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
	"codeberg.org/thomas-mangin/ze/internal/component/config/archive"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
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
	store           storage.Storage // Storage backend (filesystem or blob)
	originalContent string
	workingContent  string         // Fallback when tree can't parse
	tree            *config.Tree   // Parsed config tree (canonical when treeValid)
	schema          *config.Schema // YANG schema for Serialize
	treeValid       bool           // True when tree was parsed successfully
	dirty           atomic.Bool
	hasPendingEdit  bool             // true if .edit file exists
	session         *EditSession     // Optional: concurrent editing session
	meta            *config.MetaTree // Optional: metadata tree for write-through
	draftMtime      time.Time        // Last known draft file mtime (for polling)
	onReload        ReloadNotifier   // Optional: called after successful save
	onArchive       archive.Notifier // Optional: called after successful save to archive config
}

// BackupInfo describes a backup file.
type BackupInfo struct {
	Path      string
	Timestamp time.Time
}

// NewEditor creates a new editor for the given configuration file.
// Uses filesystem storage by default. For blob storage, use NewEditorWithStorage.
func NewEditor(configPath string) (*Editor, error) {
	return NewEditorWithStorage(storage.NewFilesystem(), configPath)
}

// NewEditorWithStorage creates a new editor backed by the given storage.
// All file I/O (config, draft, backup, lock) goes through the storage interface.
func NewEditorWithStorage(store storage.Storage, configPath string) (*Editor, error) {
	// Read original file
	data, err := store.ReadFile(configPath)
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
	hasPending := store.Exists(editPath)

	// Parse succeeded if tree has content (not the empty fallback)
	treeValid := err == nil

	return &Editor{
		originalPath:    configPath,
		store:           store,
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
// Returns zero time if no edit file exists. For blob storage, mod time is
// unavailable so this returns zero time even when the edit exists; callers
// handle zero time gracefully in the prompt.
func (e *Editor) PendingEditTime() time.Time {
	editPath := e.originalPath + ".edit"
	if !e.store.Exists(editPath) {
		return time.Time{}
	}
	// Best-effort: filesystem stat for mod time (blob returns zero time).
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
	data, err := e.store.ReadFile(editPath)
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
	data, err := e.store.ReadFile(editPath)
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
	if err := e.store.WriteFile(editPath, []byte(e.workingContent), 0o600); err != nil {
		return fmt.Errorf("failed to write edit file: %w", err)
	}
	return nil
}

// deleteEditFile removes the .edit file if it exists.
func (e *Editor) deleteEditFile() {
	editPath := e.originalPath + ".edit"
	_ = e.store.Remove(editPath) // Ignore error if doesn't exist
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

// SetArchiveNotifier sets an optional function to archive config after save.
// When set, commit will call this after writing config to disk.
// When nil (no archive locations configured), no archival is attempted.
func (e *Editor) SetArchiveNotifier(fn archive.Notifier) {
	e.onArchive = fn
}

// HasArchiveNotifier returns true if an archive notifier is configured.
func (e *Editor) HasArchiveNotifier() bool {
	return e.onArchive != nil
}

// NotifyArchive calls the archive notifier if one is configured.
// Returns nil if no notifier is set or if archival succeeds.
func (e *Editor) NotifyArchive(content []byte) []error {
	if e.onArchive == nil {
		return nil
	}
	return e.onArchive(content)
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
// When a session is active with metadata, returns set+meta format (matching
// what CommitSession writes). Otherwise returns hierarchical format.
// Falls back to raw text if tree is not valid.
func (e *Editor) WorkingContent() string {
	if e.treeValid && e.tree != nil && e.schema != nil {
		if e.session != nil && e.meta != nil {
			return config.SerializeSetWithMeta(e.tree, e.meta, e.schema)
		}
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

// OriginalContentAtPath returns the serialized content from the original (on-disk)
// config at the given context path. Re-parses originalContent on each call (config
// files are small, and this is only called on user interaction, not hot path).
// Returns empty string if path doesn't resolve in the original config.
func (e *Editor) OriginalContentAtPath(path []string) string {
	if len(path) == 0 {
		return e.originalContent
	}
	if e.schema == nil {
		return ""
	}
	parser := config.NewParser(e.schema)
	origTree, err := parser.Parse(e.originalContent)
	if err != nil {
		return ""
	}
	subtree, schemaNode := e.walkPathWithSchemaFrom(origTree, path)
	if subtree == nil || schemaNode == nil {
		return ""
	}
	return config.SerializeSubtree(subtree, schemaNode)
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

// walkPathWithSchema navigates the working tree and schema in parallel.
func (e *Editor) walkPathWithSchema(path []string) (*config.Tree, config.Node) {
	return e.walkPathWithSchemaFrom(e.tree, path)
}

// walkPathWithSchemaFrom navigates an arbitrary tree and schema in parallel,
// returning both the subtree and the schema node at the destination.
// Used by ContentAtPath (working tree) and OriginalContentAtPath (re-parsed original tree).
func (e *Editor) walkPathWithSchemaFrom(tree *config.Tree, path []string) (*config.Tree, config.Node) {
	if tree == nil || e.schema == nil || len(path) == 0 {
		return nil, nil
	}

	currentTree := tree
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
// Supports anonymous list entries (KeyDefault) for interactive editing.
// See walkOrCreateIn in editor_draft.go for the write-through variant
// that requires explicit list keys (used with full set-command paths).
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
		case *config.InlineListNode:
			// Inline lists use the same key navigation as regular lists.
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
		case *config.LeafNode, *config.FreeformNode,
			*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
			return nil, fmt.Errorf("cannot navigate into %s (leaf node)", name)
		}
	}

	return currentTree, nil
}

// HasSession returns true if a concurrent editing session is active.
func (e *Editor) HasSession() bool {
	return e.session != nil
}

// HasPendingSessionChanges returns true if this session has pending changes in the draft.
func (e *Editor) HasPendingSessionChanges() bool {
	if e.session == nil || e.meta == nil {
		return false
	}
	return len(e.meta.SessionEntries(e.session.ID)) > 0
}

// SessionID returns the current session's ID, or empty string if no session.
func (e *Editor) SessionID() string {
	if e.session == nil {
		return ""
	}
	return e.session.ID
}

// BlameView returns a blame-annotated view of the configuration.
// When no metadata exists, uses an empty MetaTree to produce a consistent
// hierarchical tree format with empty blame gutters.
func (e *Editor) BlameView() string {
	meta := e.meta
	if meta == nil {
		meta = config.NewMetaTree()
	}
	return config.SerializeBlame(e.tree, meta, e.schema)
}

// SetView returns the flat set-format view of the configuration.
// Always emits bare set commands without metadata (AC-15: exportable format).
func (e *Editor) SetView() string {
	return config.SerializeSet(e.tree, e.schema)
}

// SessionChanges returns the changes for a specific session, or all sessions.
// If sessionID is empty, returns changes for all sessions.
func (e *Editor) SessionChanges(sessionID string) []config.SessionEntry {
	if e.meta == nil {
		return nil
	}
	if sessionID == "" {
		// All sessions: collect from all known sessions.
		var all []config.SessionEntry
		for _, sid := range e.meta.AllSessions() {
			all = append(all, e.meta.SessionEntries(sid)...)
		}
		return all
	}
	return e.meta.SessionEntries(sessionID)
}

// ActiveSessions returns all session IDs with pending changes.
func (e *Editor) ActiveSessions() []string {
	if e.meta == nil {
		return nil
	}
	return e.meta.AllSessions()
}

// SetSession sets the concurrent editing session identity.
// When set, SetValue and DeleteValue use write-through to the draft file.
func (e *Editor) SetSession(session *EditSession) {
	e.session = session
	if e.meta == nil {
		e.meta = config.NewMetaTree()
	}
}

// SetValue sets a leaf value at the given path in the tree.
func (e *Editor) SetValue(path []string, key, value string) error {
	if e.session != nil {
		return e.writeThroughSet(path, key, value)
	}
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
	if e.session != nil {
		return e.writeThroughDelete(path, key)
	}
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
// Returns an error when a session is active -- use CommitSession() instead.
func (e *Editor) Save() error {
	if e.session != nil {
		return fmt.Errorf("Save() not allowed with active session; use CommitSession()")
	}
	if !e.dirty.Load() {
		return nil
	}

	// Create backup of original
	if err := e.createBackup(e.originalContent, nil); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Write serialized tree (or raw text fallback) to original path
	content := e.WorkingContent()
	if err := e.store.WriteFile(e.originalPath, []byte(content), 0o600); err != nil {
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

// atomicWriteFile writes data to a file atomically: write to a temp file in the
// same directory, then rename. On POSIX, rename is atomic — the target path is
// either the old content or the new content, never a partial write.
func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ze-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	cleanup := func() {
		tmp.Close()        //nolint:errcheck // best effort cleanup on error path
		os.Remove(tmpName) //nolint:errcheck // best effort cleanup on error path
	}

	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName) //nolint:errcheck // best effort cleanup on error path
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName) //nolint:errcheck // best effort cleanup on error path
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// rollbackDir returns the rollback subdirectory for storing config snapshots.
// Junos-style: backups live in a dedicated rollback/ folder alongside the config.
func (e *Editor) rollbackDir() string {
	return filepath.Join(filepath.Dir(e.originalPath), "rollback")
}

// createBackup creates a backup of the given content in the rollback/ subdirectory.
// Filename uses a full timestamp (YYYYMMDD-HHMMSS.mmm) for natural date ordering.
// The content parameter is what was on disk before the overwrite -- callers pass
// freshly-read data to avoid backing up stale cached content.
// When guard is non-nil (inside a lock), writes through the guard to avoid deadlock.
// When guard is nil (outside a lock), writes through e.store.
func (e *Editor) createBackup(content string, guard storage.WriteGuard) error {
	base := filepath.Base(e.originalPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	rollback := e.rollbackDir()

	now := time.Now()
	stamp := fmt.Sprintf("%s.%03d", now.Format("20060102-150405"), now.Nanosecond()/1e6)
	backupPath := filepath.Join(rollback, fmt.Sprintf("%s-%s.conf", name, stamp))

	if guard != nil {
		return guard.WriteFile(backupPath, []byte(content), 0o600)
	}
	return e.store.WriteFile(backupPath, []byte(content), 0o600)
}

// ListBackups returns available backup files, sorted by timestamp descending.
// Looks in the rollback/ subdirectory for date-stamped config snapshots.
func (e *Editor) ListBackups() ([]BackupInfo, error) {
	base := filepath.Base(e.originalPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	rollback := e.rollbackDir()

	// List all files in rollback directory, then filter by name pattern.
	matches, err := e.store.List(rollback)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No rollback directory = no backups
		}
		return nil, err
	}

	// Filter to files matching this config's backup pattern.
	prefix := name + "-"
	filtered := matches[:0]
	for _, m := range matches {
		if b := filepath.Base(m); strings.HasPrefix(b, prefix) {
			filtered = append(filtered, m)
		}
	}
	matches = filtered

	backups := make([]BackupInfo, 0, len(matches))
	re := regexp.MustCompile(`-(\d{8}-\d{6})\.(\d{3})\.conf$`)

	for _, path := range matches {
		m := re.FindStringSubmatch(path)
		if len(m) < 3 {
			continue
		}

		ts, err := time.ParseInLocation("20060102-150405", m[1], time.Local)
		if err != nil {
			continue
		}

		// Add milliseconds back to timestamp
		ms, _ := strconv.Atoi(m[2])
		ts = ts.Add(time.Duration(ms) * time.Millisecond)

		backups = append(backups, BackupInfo{
			Path:      path,
			Timestamp: ts,
		})
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(backups, func(i, j int) bool {
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
	if err := e.store.WriteFile(e.LivePath(), []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to write live config: %w", err)
	}
	return nil
}

// HasPendingLive returns true if a .live.conf file exists.
// This indicates an unconfirmed "commit confirmed" from a previous session.
func (e *Editor) HasPendingLive() bool {
	return e.store.Exists(e.LivePath())
}

// DeleteLive removes the .live.conf file if it exists.
// Errors are ignored because the file may not exist.
func (e *Editor) DeleteLive() {
	_ = e.store.Remove(e.LivePath()) // Ignore error if doesn't exist
}

// Rollback restores the configuration from a backup file.
// Creates a backup of the current config first, so the rollback itself can be undone.
func (e *Editor) Rollback(backupPath string) error {
	// Read backup content
	data, err := e.store.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("cannot read backup: %w", err)
	}

	// Backup current config before overwriting -- rollback is itself reversible
	if err := e.createBackup(e.originalContent, nil); err != nil {
		return fmt.Errorf("cannot backup current config before rollback: %w", err)
	}

	// Write to original path
	if err := e.store.WriteFile(e.originalPath, data, 0o600); err != nil {
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
