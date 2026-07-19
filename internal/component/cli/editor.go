// Design: docs/architecture/config/yang-config-design.md — config editor
// Detail: editor_draft.go — write-through draft protocol (sub-hub for commit/walk)
// Detail: editor_session.go — session identity for concurrent editing
// Detail: editor_annotated.go — annotated view and show column preferences
//
// Package editor provides an interactive configuration editor.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/archive"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/helpfmt"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// ReloadNotifier is called after a successful save to notify the running daemon.
// Returns nil on success, or an error if the daemon could not be reached.
type ReloadNotifier func() error

// Editor manages an editing session for a configuration file.
// The tree is the canonical in-memory representation when treeValid is true.
// WorkingContent() returns Serialize(tree) when tree is valid, otherwise falls
// back to stored raw text for configs that can't be parsed.
type Editor struct {
	originalPath      string
	store             storage.Storage // Storage backend (filesystem or blob)
	originalContent   string
	workingContent    string         // Fallback when tree can't parse
	tree              *config.Tree   // Parsed config tree (canonical when treeValid)
	schema            *config.Schema // YANG schema for Serialize
	treeValid         bool           // True when tree was parsed successfully
	dirty             atomic.Bool
	hasPendingEdit    bool                         // true if .edit file exists
	session           *EditSession                 // Optional: concurrent editing session
	meta              *config.MetaTree             // Optional: metadata tree for write-through
	draftMtime        time.Time                    // Last known draft file mtime (for polling)
	onReload          ReloadNotifier               // Optional: called after successful save
	onArchive         archive.Notifier             // Optional: called after successful save to archive config
	preCommitValidate func(candidate string) error // Optional: validate candidate config before writing
	showColumns       map[string]bool              // In-memory show column preferences (sticky per session)
	diffGutter        bool                         // Whether diff gutter (+/-) markers are shown (default true)
	draftSaved        bool                         // True when changes have been persisted to draft (reset on new edits)
	stdoutSink        io.Writer                    // Non-nil for a stdin-sourced ("-") editor: Save emits here instead of writing a file
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
	return newEditor(store, configPath, string(data), true)
}

// NewEditorFromContent builds an editor from in-memory content instead of a
// stored path, for a config supplied on stdin ("-"). identity is a synthetic
// name (typically "-") used only for display and error context; no on-disk
// ".edit" sibling is probed, since a stdin source has none. A stdin-sourced
// editor is shown or emitted to stdout (see SetStdoutSink), never written back
// to a path derived from identity.
func NewEditorFromContent(content []byte, identity string) (*Editor, error) {
	return newEditor(storage.NewFilesystem(), identity, string(content), false)
}

// newEditor parses content into the tree/meta and assembles the Editor.
// checkPending probes the on-disk ".edit" sibling (skipped for content-sourced
// editors, which have no sibling).
func newEditor(store storage.Storage, identity, content string, checkPending bool) (*Editor, error) {
	// Parse config into tree using YANG-derived schema
	schema, err := config.YANGSchema()
	if err != nil {
		return nil, fmt.Errorf("YANG schema: %w", err)
	}
	tree, meta, err := parseConfigWithFormat(content, schema)
	if err != nil {
		// Retry with lenient parsing: skip unknown fields so the editor
		// can display a tree view even when stale fields exist on disk.
		tree, meta, err = parseConfigLenient(content, schema)
		if err != nil {
			tree = config.NewTree()
			meta = nil
		}
	}
	_ = meta // metadata used later by SetSession

	// Check for existing edit file (only for path-backed editors).
	hasPending := false
	if checkPending {
		var tb textbuf.Buffer
		editPath := tb.Str(identity).Str(".edit").String()
		hasPending = store.Exists(editPath)
	}

	// Parse succeeded if tree has content (not the empty fallback)
	treeValid := err == nil

	return &Editor{
		originalPath:    identity,
		store:           store,
		originalContent: content,
		workingContent:  content,
		tree:            tree,
		schema:          schema,
		treeValid:       treeValid,
		hasPendingEdit:  hasPending,
		showColumns:     newShowColumnDefaults(),
		diffGutter:      true,
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
	var tb textbuf.Buffer
	editPath := tb.Str(e.originalPath).Str(".edit").String()
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
	var tb textbuf.Buffer
	editPath := tb.Str(e.originalPath).Str(".edit").String()
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

	// Route prompt output through a RenderWriter: if stdout is broken there is no
	// way to interact, so a write error aborts the prompt as a quit.
	rw := helpfmt.NewRenderWriter(os.Stdout)
	var tb textbuf.Buffer
	rw.Str(tb.Str("\nFound uncommitted changes from ").Str(timeStr).Str(".\n").String())
	rw.Line("  [c] Continue editing")
	rw.Line("  [d] Discard and start fresh")
	rw.Line("  [v] View changes first")
	rw.Line("  [q] Quit")

	reader := bufio.NewReader(os.Stdin)
	for {
		rw.Str("Choice: ")
		if rw.Err() != nil {
			return PendingEditQuit
		}
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
				rw.Line("\nNo differences found.")
			} else {
				rw.Line("\nChanges:")
				rw.Line(diff)
			}
			// After viewing, prompt again
			rw.Line("  [c] Continue editing")
			rw.Line("  [d] Discard and start fresh")
			rw.Line("  [q] Quit")
		case "q":
			return PendingEditQuit
		default:
			rw.Line("Invalid choice. Enter c, d, v, or q.")
		}
	}
}

// LoadPendingEdit loads the content from the .edit file.
func (e *Editor) LoadPendingEdit() error {
	var tb textbuf.Buffer
	editPath := tb.Str(e.originalPath).Str(".edit").String()
	data, err := e.store.ReadFile(editPath)
	if err != nil {
		return fmt.Errorf("cannot read edit file: %w", err)
	}

	e.workingContent = string(data)
	e.dirty.Store(true)
	e.hasPendingEdit = false // Loaded, no longer "pending"
	return nil
}

// SetStdoutSink makes Save emit the working config to w instead of writing it
// back to a stored path. It is set for a config supplied on stdin ("-"), which
// turns set/deactivate/activate into pipeline stages (read stdin, modify, emit
// to stdout). A real-path editor leaves this nil and writes the file as before.
func (e *Editor) SetStdoutSink(w io.Writer) {
	e.stdoutSink = w
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

// SetPreCommitValidate sets a function called during SaveDraft to validate
// the candidate config before writing. If the function returns an error,
// the save is rejected and the draft is not written.
func (e *Editor) SetPreCommitValidate(fn func(candidate string) error) {
	e.preCommitValidate = fn
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

// OriginalContentAtPath returns the serialized content from the original (on-disk)
// config at the given context path in tree format. Re-parses originalContent on each
// call (config files are small, and this is only called on user interaction, not hot path).
// Always returns tree format to match ContentAtPath, regardless of the stored format.
// Returns empty string if path doesn't resolve in the original config.
func (e *Editor) OriginalContentAtPath(path []string) string {
	if e.schema == nil {
		if len(path) == 0 {
			return e.originalContent
		}
		return ""
	}
	origTree, _, err := parseConfigWithFormat(e.originalContent, e.schema)
	if err != nil {
		return e.originalContent
	}
	if len(path) == 0 {
		return config.Serialize(origTree, e.schema)
	}
	subtree, schemaNode := e.walkPathWithSchemaFrom(origTree, path)
	if subtree == nil || schemaNode == nil {
		return ""
	}
	return config.SerializeSubtree(subtree, schemaNode)
}

// ContentAtPath returns the serialized content at the given context path in tree format.
// Always returns tree (junos-style) format for display, regardless of session state.
// If the path doesn't resolve, falls back to full tree content.
func (e *Editor) ContentAtPath(path []string) string {
	if len(path) == 0 {
		if e.treeValid && e.tree != nil && e.schema != nil {
			return config.Serialize(e.tree, e.schema)
		}
		return e.workingContent
	}
	if !e.treeValid || e.tree == nil || e.schema == nil {
		return e.workingContent
	}

	subtree, schemaNode := e.walkPathWithSchema(path)
	if subtree == nil || schemaNode == nil {
		if e.treeValid && e.tree != nil && e.schema != nil {
			return config.Serialize(e.tree, e.schema)
		}
		return e.workingContent
	}
	return config.SerializeSubtree(subtree, schemaNode)
}

// ActiveContentAtPath returns config text showing only active nodes (inactive pruned).
func (e *Editor) ActiveContentAtPath(path []string) string {
	if !e.treeValid || e.tree == nil || e.schema == nil {
		return e.workingContent
	}
	clone := e.tree.Clone()
	config.PruneInactive(clone, e.schema)
	config.MaskBcryptInPlace(clone, e.schema) // display-only; hides ze:bcrypt hashes
	if len(path) == 0 {
		return config.Serialize(clone, e.schema)
	}
	subtree, schemaNode := e.walkPathWithSchemaFrom(clone, path)
	if subtree == nil || schemaNode == nil {
		return config.Serialize(clone, e.schema)
	}
	return config.SerializeSubtree(subtree, schemaNode)
}

// InactiveContentAtPath returns config text showing only inactive nodes.
// Active nodes are pruned; inactive nodes are shown with their full subtree.
func (e *Editor) InactiveContentAtPath(path []string) string {
	if !e.treeValid || e.tree == nil || e.schema == nil {
		return ""
	}
	clone := e.tree.Clone()
	config.PruneActive(clone, e.schema)
	config.MaskBcryptInPlace(clone, e.schema) // display-only; hides ze:bcrypt hashes
	if len(path) == 0 {
		return config.Serialize(clone, e.schema)
	}
	subtree, schemaNode := e.walkPathWithSchemaFrom(clone, path)
	if subtree == nil || schemaNode == nil {
		return config.Serialize(clone, e.schema)
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

// WalkPathWithSchema is the exported form of walkPathWithSchema. Returns
// the (sub-tree, schema node) pair at the given path. Halts at structural
// nodes (containers, lists) -- for terminal-leaf lookups use
// LookupSchemaNode instead.
func (e *Editor) WalkPathWithSchema(path []string) (*config.Tree, config.Node) {
	return e.walkPathWithSchema(path)
}

// LookupSchemaNode returns the schema node at the terminus of path.
// Unlike WalkPathWithSchema, this walks the schema only (no tree walk),
// so it resolves leaves as well as containers and list entries. Used by
// one-shot CLI verbs that need to dispatch deactivate/activate based on
// the node kind regardless of whether the value is currently set.
//
// List keys interleaved in the path are skipped (a list child consumes
// two tokens: its name and its key value).
func (e *Editor) LookupSchemaNode(path []string) config.Node {
	if e.schema == nil || len(path) == 0 {
		return nil
	}
	var current schemaGetter = e.schema
	var last config.Node
	i := 0
	for i < len(path) {
		name := path[i]
		node := current.Get(name)
		if node == nil {
			return nil
		}
		last = node
		i++
		// Step over a list key, if this list child has one.
		if _, isList := node.(*config.ListNode); isList && i < len(path) {
			i++
		}
		if i < len(path) {
			sg, ok := node.(schemaGetter)
			if !ok {
				return nil // path continues past a leaf -- invalid
			}
			current = sg
		}
	}
	return last
}

// ResolveLeafListValue checks whether fullPath terminates inside a
// leaf-list value (e.g. `bgp filter import no-self-as`). Returns the
// tree-level parent path (to the container holding the leaf-list), the
// leaf-list field name, and true if so. Handles list keys interleaved
// in the path (e.g. `bgp peer peer1 filter import value`).
//
// The Model's TUI dispatch and the one-shot CLI verbs both call this
// to route deactivate/activate to DeactivateLeafListValue.
func (e *Editor) ResolveLeafListValue(fullPath []string) (parentPath []string, leafListName string, ok bool) {
	if e.schema == nil || len(fullPath) < 2 {
		return nil, "", false
	}
	var current schemaGetter = e.schema
	i := 0
	for i < len(fullPath)-1 {
		name := fullPath[i]
		node := current.Get(name)
		if node == nil {
			return nil, "", false
		}
		if i == len(fullPath)-2 {
			switch node.(type) {
			case *config.ValueOrArrayNode, *config.BracketLeafListNode:
				return fullPath[:i], name, true
			}
			return nil, "", false
		}
		sg, canNavigate := node.(schemaGetter)
		if !canNavigate {
			return nil, "", false
		}
		current = sg
		i++
		if _, isList := node.(*config.ListNode); isList {
			i++ // skip list key
		}
	}
	return nil, "", false
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

// HasSession returns true if a concurrent editing session is active.
func (e *Editor) HasSession() bool {
	return e.session != nil
}

// HasPendingSessionChanges returns true if this session has pending changes in the draft.
func (e *Editor) HasPendingSessionChanges() bool {
	if e.session == nil || e.meta == nil {
		return false
	}
	if e.draftSaved {
		return false // Changes persisted to draft, safe to exit
	}
	return len(e.PendingChanges(e.session.ID)) > 0
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

// SensitiveKeys returns the set of leaf names marked ze:sensitive in the schema.
func (e *Editor) SensitiveKeys() map[string]bool {
	if e.schema == nil {
		return nil
	}
	return config.SensitiveKeys(e.schema)
}

// BcryptKeys returns the set of leaf names marked ze:bcrypt in the schema.
func (e *Editor) BcryptKeys() map[string]bool {
	return config.BcryptKeys(e.schema)
}

// SessionChanges returns the changes for a specific session, or all sessions.
// If sessionID is empty, returns changes for all sessions (scanning change files).
func (e *Editor) SessionChanges(sessionID string) []config.SessionEntry {
	seen := make(map[string]bool)
	var all []config.SessionEntry

	addEntries := func(entries []config.SessionEntry) {
		for _, entry := range entries {
			var tb textbuf.Buffer
			// Member distinguishes per-member leaf-list entries that share a path.
			key := tb.Str(entry.Entry.SessionKey()).Byte('|').Str(entry.Path).Byte('|').Str(entry.Entry.Member).String()
			if seen[key] {
				continue
			}
			seen[key] = true
			all = append(all, entry)
		}
	}

	if e.meta != nil {
		if sessionID != "" {
			addEntries(e.meta.SessionEntries(sessionID))
		} else {
			for _, sid := range e.meta.AllSessions() {
				addEntries(e.meta.SessionEntries(sid))
			}
		}
	}

	for _, cf := range e.listChangeFiles() {
		meta, _, err := e.readChangeFileContent(cf)
		if err != nil {
			continue
		}
		if sessionID != "" {
			addEntries(meta.SessionEntries(sessionID))
			continue
		}
		for _, sid := range meta.AllSessions() {
			addEntries(meta.SessionEntries(sid))
		}
	}

	return all
}

// PendingChanges returns the unified pending-change view for a specific session,
// or all sessions when sessionID is empty.
func (e *Editor) PendingChanges(sessionID string) []config.PendingChange {
	seen := make(map[string]bool)
	var changes []config.PendingChange

	addChange := func(change config.PendingChange) {
		key := pendingChangeKey(change)
		if seen[key] {
			return
		}
		seen[key] = true
		changes = append(changes, change)
	}

	if e.meta != nil {
		if sessionID != "" {
			for _, entry := range e.meta.SessionEntries(sessionID) {
				addChange(config.PendingChangeFromSessionEntry(entry))
			}
		} else {
			for _, sid := range e.meta.AllSessions() {
				for _, entry := range e.meta.SessionEntries(sid) {
					addChange(config.PendingChangeFromSessionEntry(entry))
				}
			}
		}
	}

	for _, cf := range e.listChangeFiles() {
		meta, ops, err := e.readChangeFileContent(cf)
		if err != nil {
			continue
		}
		if sessionID != "" {
			for _, entry := range meta.SessionEntries(sessionID) {
				addChange(config.PendingChangeFromSessionEntry(entry))
			}
			for i := range ops {
				if ops[i].SessionKey() == sessionID {
					addChange(ops[i].PendingChange())
				}
			}
			continue
		}
		for _, sid := range meta.AllSessions() {
			for _, entry := range meta.SessionEntries(sid) {
				addChange(config.PendingChangeFromSessionEntry(entry))
			}
		}
		for i := range ops {
			addChange(ops[i].PendingChange())
		}
	}

	config.SortPendingChanges(changes)
	return changes
}

// ActiveSessions returns all session IDs with pending changes.
// Scans change files to include other users' sessions.
func (e *Editor) ActiveSessions() []string {
	seen := make(map[string]bool)
	var sessions []string
	for _, change := range e.PendingChanges("") {
		if change.SessionID == "" || seen[change.SessionID] {
			continue
		}
		seen[change.SessionID] = true
		sessions = append(sessions, change.SessionID)
	}
	sort.Strings(sessions)
	return sessions
}

// listChangeFiles returns all per-user change file paths for this config.
func (e *Editor) listChangeFiles() []string {
	dir := filepath.Dir(e.originalPath)
	files, err := e.store.List(dir)
	if err != nil {
		return nil
	}

	prefix := ChangePrefix(e.originalPath)

	var result []string
	for _, f := range files {
		base := filepath.Base(f)
		if !strings.HasPrefix(base, prefix) {
			continue
		}
		result = append(result, f)
	}
	return result
}

func (e *Editor) readChangeFileContent(path string) (*config.MetaTree, []config.StructuralOp, error) {
	data, err := e.store.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	_, meta, ops, parseErr := config.ParseChangeFile(string(data), config.NewSetParser(e.schema))
	return meta, ops, parseErr
}

func pendingChangeKey(change config.PendingChange) string {
	var tb textbuf.Buffer
	switch change.Kind {
	case config.PendingChangeRename:
		return tb.Str(change.SessionID).Str("|rename|").Str(change.OldPath).Byte('|').Str(change.NewPath).String()
	default:
		tb.Str(change.SessionID).Byte('|').Str(string(change.Kind)).Byte('|').Str(change.Path)
		if change.Member != "" {
			// Each leaf-list member is its own pending change; without the
			// member in the key, a second member would dedupe into the first.
			tb.Byte('|').Str(change.Member)
		}
		return tb.String()
	}
}

// SetSession sets the concurrent editing session identity.
// When set, SetValue and DeleteValue use write-through to the draft file.
func (e *Editor) SetSession(session *EditSession) {
	e.session = session
	if e.meta == nil {
		e.meta = config.NewMetaTree()
	}
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

	var diff textbuf.Buffer

	// Removed lines
	for _, line := range originalLines {
		if strings.TrimSpace(line) != "" && !modifiedSet[line] {
			diff.Str("- ").Str(line).Byte('\n')
		}
	}

	// Added lines
	for _, line := range modifiedLines {
		if strings.TrimSpace(line) != "" && !originalSet[line] {
			diff.Str("+ ").Str(line).Byte('\n')
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

// createBackup creates a dated version of the given content.
// When guard is non-nil (inside a lock), writes through the guard to avoid deadlock.
// When guard is nil (outside a lock), writes through e.store.
func (e *Editor) createBackup(content string, guard storage.WriteGuard) error {
	now := time.Now()
	if guard != nil {
		return guard.WriteVersion(e.originalPath, []byte(content), now)
	}
	return e.store.WriteVersion(e.originalPath, []byte(content), now)
}

// ListBackups returns available backup files, sorted by timestamp descending.
func (e *Editor) ListBackups() ([]BackupInfo, error) {
	versions, err := e.store.ListVersions(e.originalPath)
	if err != nil {
		return nil, err
	}
	backups := make([]BackupInfo, len(versions))
	for i, v := range versions {
		backups[i] = BackupInfo{Path: v.Path, Timestamp: v.Date}
	}
	return backups, nil
}

// ReadBackupContent reads the content of a backup by its path.
func (e *Editor) ReadBackupContent(path string) ([]byte, error) {
	return e.store.ReadFile(path)
}

// HasDraft returns true if a draft file exists for this config.
func (e *Editor) HasDraft() bool {
	return e.store.Exists(DraftPath(e.originalPath))
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

	// Read current committed config from storage (not cache) for accurate backup.
	currentData, readErr := e.store.ReadFile(e.originalPath)
	if readErr != nil {
		return fmt.Errorf("cannot read current config for backup: %w", readErr)
	}
	if err := e.createBackup(string(currentData), nil); err != nil {
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
