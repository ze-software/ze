// Design: docs/architecture/config/yang-config-design.md — config editor
// Detail: editor_draft.go — write-through draft protocol (sub-hub for commit/walk)
// Detail: editor_session.go — session identity for concurrent editing
// Detail: editor_annotated.go — annotated view and show column preferences
//
// Package editor provides an interactive configuration editor.
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/archive"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
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
	store             storage.Storage // Shared store, or history-only for a loose-file editor.
	looseFile         bool            // Explicit file authority; store, if present, holds history only.
	viewScope         editorViewScope // Immutable construction choice: shared draft or own candidate.
	ownsStore         bool            // Close releases an offline editor's lifetime ownership.
	originalContent   string
	workingContent    string         // Fallback when tree can't parse
	tree              *config.Tree   // Parsed config tree (canonical when treeValid)
	schema            *config.Schema // YANG schema for Serialize
	treeValid         bool           // True when tree was parsed successfully
	dirty             atomic.Bool
	hasPendingEdit    bool                                 // true if .edit file exists
	session           *EditSession                         // Optional: concurrent editing session
	meta              *config.MetaTree                     // Optional: metadata tree for write-through
	draftMtime        time.Time                            // Last known draft file mtime (for polling)
	commitWriter      func(expected, content []byte) error // Daemon-owned source publication.
	onReload          ReloadNotifier                       // Optional: called after successful save
	confirmWindow     func() *confirm.Window               // Optional: the daemon's confirmed-commit window (model_commit_window.go)
	offlinePromotion  bool                                 // onReload promotes in the editor's own store; no daemon.
	onArchive         archive.Notifier                     // Optional: called after successful save to archive config
	preCommitValidate func(candidate string) error         // Optional: validate candidate config before writing
	showColumns       map[string]bool                      // In-memory show column preferences (sticky per session)
	diffGutter        bool                                 // Whether diff gutter (+/-) markers are shown (default true)
	draftSaved        bool                                 // True when changes have been persisted to draft (reset on new edits)
	// loadStage is non-nil only while a session load runs: every write-through
	// step then edits the one parsed copy of the change file it holds, under
	// the one lock the load holds, and the load writes the file once at the end.
	loadStage *loadStage
	// overridden holds the other users' LIVE-conflicting changes a forced
	// candidate commit overrode, until MarkCommittedContent discards them. Each
	// candidate commit resets it, so a failed reload discards nothing.
	overridden []liveOverlap
	// discardNoticeSeen is the stamp of the last discard notice line this
	// session showed (AckDiscardNotice), set when the session starts past the
	// lines another session already showed (startDiscardNotice).
	discardNoticeSeen int64
	stdoutSink        io.Writer // Non-nil for a stdin-sourced ("-") editor: Save emits here instead of writing a file
	// now supplies the wall clock for version stamps. A field rather than a
	// direct time.Now() call so a test can pin it: the same-millisecond backup
	// collision this guards is otherwise unreproducible on demand, and a test
	// that merely hopes two calls land in one millisecond gates nothing.
	now func() time.Time
	// lastBackupStamp is the millisecond-truncated stamp of the most recent
	// backup this editor took, so the next one can be forced strictly later.
	// See nextBackupStamp.
	lastBackupStamp time.Time
}

// BackupInfo describes a backup file.
type BackupInfo struct {
	Path      string
	Timestamp time.Time
}

// NewLooseFileEditor opens an explicit config file. A supplied store records
// versions; nil leaves the file usable without creating a store.
// Unparseable loose files keep raw-text inspection so operators can repair
// broken syntax interactively; stored editors use newEditor and fail closed.
// The caller MUST close any supplied store after closing the editor.
func NewLooseFileEditor(store storage.Storage, configPath string) (*Editor, error) {
	data, err := cliio.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}
	ed, err := newRawFallbackEditor(store, configPath, string(data), false)
	if err != nil {
		return nil, err
	}
	ed.looseFile = true
	_, err = os.Stat(configPath + ".edit")
	ed.hasPendingEdit = err == nil
	return ed, nil
}

// NewEditorWithStorage creates a new editor backed by the given storage.
// Committed content follows its verified active pointer; draft, change-file,
// backup and lock operations remain in the same storage key space.
func NewEditorWithStorage(store storage.Storage, configPath string) (*Editor, error) {
	if store == nil {
		return nil, fmt.Errorf("config editor: %w; run ze init", storage.ErrNoStore)
	}
	data, err := storage.ReadActiveConfig(store, configPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}
	return newEditor(store, configPath, string(data), true)
}

// NewIsolatedEditorWithStorage opens an editor whose visible candidate contains
// committed configuration and only its assigned session's pending intent. The
// caller MUST assign SetSession and finish ResumeSession before publishing it.
// The shared draft and its complete metadata still serve replay and conflicts.
func NewIsolatedEditorWithStorage(store storage.Storage, configPath string) (*Editor, error) {
	editor, err := NewEditorWithStorage(store, configPath)
	if err != nil {
		return nil, err
	}
	editor.viewScope = editorViewIsolated
	return editor, nil
}

// NewOfflineSessionEditor opens a session editor over a store that no daemon
// owns, which is a backup artifact opened by `ze config edit --backup`. It
// carries what a daemon's session editor carries (session identity, the saved
// draft, version history in the store) and replaces the daemon's reload with
// SetOfflinePromotion, so a commit writes the version, the pointers and the
// file/active mirror into the store itself.
//
// origin is "local": the session is typed at this terminal. The caller keeps
// the store's close obligation, or passes it with OwnStore.
func NewOfflineSessionEditor(store storage.Storage, configPath, user string) (*Editor, error) {
	if err := ValidateUser(user); err != nil {
		return nil, fmt.Errorf("invalid username: %w", err)
	}
	ed, err := NewEditorWithStorage(store, configPath)
	if err != nil {
		return nil, err
	}
	ed.SetSession(NewEditSession(user, "local"))
	if ed.HasDraft() {
		if !ed.LoadDraft() {
			return nil, fmt.Errorf("load saved draft for %s", configPath)
		}
	}
	ed.SetOfflinePromotion()
	return ed, nil
}

// NewEditorFromContent builds an editor from in-memory content instead of a
// stored path, for a config supplied on stdin ("-") or one-shot file display.
// identity is a synthetic name (typically "-") used only for display and error
// context; no on-disk ".edit" sibling is probed, since a stdin source has none.
// Unparseable content retains raw-text display so local inspection (`ze config
// show`) can print a broken file for repair while tree paths fail closed. A
// stdin-sourced editor is shown or emitted to stdout (see SetStdoutSink), never
// written back to a path derived from identity.
func NewEditorFromContent(content []byte, identity string) (*Editor, error) {
	return newRawFallbackEditor(nil, identity, string(content), false)
}

// newEditor parses content into the tree/meta and assembles the Editor.
// It fails closed on parse errors rather than substituting an empty tree, so
// stored editors never mistake corrupt configuration for an empty one.
// checkPending probes the ".edit" sibling in store.
func newEditor(store storage.Storage, identity, content string, checkPending bool) (*Editor, error) {
	schema, _, err := config.SchemaWithDeclaredPlugins(config.ConfigSource{Text: content, Path: identity})
	if err != nil {
		return nil, fmt.Errorf("YANG schema: %w", err)
	}
	tree, meta, err := parseEditingConfig(content, schema)
	if err != nil {
		return nil, fmt.Errorf("parse configuration %s: %w", identity, err)
	}
	_ = meta // metadata used later by SetSession
	return buildEditor(store, identity, content, schema, tree, true, checkPending), nil
}

// newRawFallbackEditor constructs an Editor that retains raw text with
// treeValid=false when content cannot be parsed. Callers MUST use this only
// for explicit raw-text inspection or loose-file repair paths where displaying
// the broken source is required.
func newRawFallbackEditor(store storage.Storage, identity, content string, checkPending bool) (*Editor, error) {
	// Parse config into tree using the YANG-derived schema of this process's
	// module set (config.SchemaWithDeclaredPlugins), which starts no program.
	// With no daemon that is the compiled-in set, so a section of an external
	// plugin is refused here; inside a daemon it is the set the daemon joined
	// from its plugins when it started, and a plugin declared during the
	// session is known only after a restart.
	schema, _, err := config.SchemaWithDeclaredPlugins(config.ConfigSource{Text: content, Path: identity})
	if err != nil {
		return nil, fmt.Errorf("YANG schema: %w", err)
	}
	tree, meta, err := parseEditingConfig(content, schema)
	if err != nil {
		tree = config.NewTree()
		meta = nil
	}
	_ = meta // metadata used later by SetSession
	return buildEditor(store, identity, content, schema, tree, err == nil, checkPending), nil
}

// buildEditor assembles an Editor from a resolved schema and tree.
func buildEditor(store storage.Storage, identity, content string, schema *config.Schema, tree *config.Tree, treeValid, checkPending bool) *Editor {
	// Check for existing edit file (only for path-backed editors).
	hasPending := false
	if checkPending && store != nil {
		var tb textbuf.Buffer
		editPath := tb.Str(identity).Str(".edit").String()
		hasPending = store.Exists(editPath)
	}

	return &Editor{
		originalPath:    identity,
		viewScope:       editorViewShared,
		store:           store,
		originalContent: content,
		workingContent:  content,
		tree:            tree,
		schema:          schema,
		treeValid:       treeValid,
		hasPendingEdit:  hasPending,
		showColumns:     newShowColumnDefaults(),
		diffGutter:      true,
		now:             time.Now,
	}
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

// Close releases the store acquired by an offline editor. Callers MUST call
// Close when finished; daemon-owned stores are never closed here.
func (e *Editor) Close() error {
	if e.ownsStore {
		e.ownsStore = false
		return e.store.Close()
	}
	return nil
}

// OwnStore transfers an offline store's close obligation to this editor.
// The caller MUST call Close after transferring ownership.
func (e *Editor) OwnStore() {
	e.ownsStore = true
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

// pendingEditTime returns the modification time of the .edit file.
// Returns zero time when the source cannot supply a timestamp.
func (e *Editor) pendingEditTime() time.Time {
	var tb textbuf.Buffer
	editPath := tb.Str(e.originalPath).Str(".edit").String()
	if e.looseFile {
		if info, err := os.Stat(editPath); err == nil {
			return info.ModTime()
		}
	} else if e.store != nil {
		if info, err := e.store.Stat(editPath); err == nil {
			return info.ModTime
		}
	}
	return time.Time{}
}

// pendingEditDiff returns the diff between original and pending edit content.
// Returns empty string if no edit file exists.
func (e *Editor) pendingEditDiff() string {
	var tb textbuf.Buffer
	editPath := tb.Str(e.originalPath).Str(".edit").String()
	data, err := e.readFile(editPath)
	if err != nil {
		return ""
	}
	return diffOrderedLines(e.originalContent, string(data))
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
	modTime := e.pendingEditTime()
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
			diff := e.pendingEditDiff()
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
	data, err := e.readFile(editPath)
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

// SetOfflinePromotion makes a commit publish its staged candidate in the
// editor's own store, for a store no daemon owns (`ze config edit --backup`).
// A daemon accepts a candidate by reloading it and then promotes it; an
// offline artifact has nobody to accept it, so acceptance IS the promotion:
// the active and rollback pointers and the file/active mirror move in one
// guarded step (storage.PromoteCandidate). A commit that wrote only the
// file/active mirror would leave the active pointer on the old version, and
// every reader that follows the pointer (ReadActiveConfig, restore) would
// still answer the old config.
//
// It replaces any reload notifier. The caller MUST NOT set one afterwards.
func (e *Editor) SetOfflinePromotion() {
	e.onReload = func() error { return storage.PromoteCandidate(e.store, e.originalPath) }
	e.offlinePromotion = true
}

// acceptedVerb names what the acceptance step after a staged candidate did,
// for the commit status line: a daemon reloaded it, an offline store
// published it.
func (e *Editor) acceptedVerb() string {
	if e.offlinePromotion {
		return "published"
	}
	return "reloaded"
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

// SetCommitWriter selects the owning source's publication transaction.
// The callback MUST publish both configuration and history before returning nil.
// An editor of a versioned stored source MUST have this callback before Save.
func (e *Editor) SetCommitWriter(fn func(expected, content []byte) error) {
	e.commitWriter = fn
}

// SetArchiveNotifier sets an optional function to archive config after save.
// When set, commit will call this after writing config to disk.
// When nil (no archive locations configured), no archival is attempted.
func (e *Editor) SetArchiveNotifier(fn archive.Notifier) {
	e.onArchive = fn
}

// SetPreCommitValidate sets a function that validates a candidate config
// before it is written. It runs at each of the two writes, over the tree that
// write is about to persist: in SaveDraft over the draft tree, and in
// CommitSession and CommitSessionCandidate over the committed tree
// (validateStagedTree). An error rejects that write and nothing is persisted.
func (e *Editor) SetPreCommitValidate(fn func(candidate string) error) {
	e.preCommitValidate = fn
}

// hasArchiveNotifier returns true if an archive notifier is configured.
func (e *Editor) hasArchiveNotifier() bool {
	return e.onArchive != nil
}

// notifyArchive calls the archive notifier if one is configured.
// Returns nil if no notifier is set or if archival succeeds.
func (e *Editor) notifyArchive(content []byte) []error {
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

// liveCommittedContent returns the current committed configuration.
// Store-backed editors read the verified active version from storage so
// external commits are reflected immediately; loose-file and content-only
// editors return their loaded original content.
func (e *Editor) liveCommittedContent() (string, error) {
	if e.store != nil && !e.looseFile {
		data, err := storage.ReadActiveConfig(e.store, e.originalPath)
		if err != nil {
			return "", fmt.Errorf("read committed configuration: %w", err)
		}
		return string(data), nil
	}
	return e.originalContent, nil
}

// WorkingContent returns the current working content.
// When a session is active with metadata, returns set+meta format (matching
// what CommitSession writes). Otherwise returns hierarchical format.
// Falls back to raw text if tree is not valid.
func (e *Editor) WorkingContent() string {
	if e.treeValid && e.tree != nil && e.schema != nil {
		if e.session != nil && e.meta != nil {
			return config.SerializeSetWithMeta(e.tree, e.workingViewMetadata(), e.schema)
		}
		return config.Serialize(e.tree, e.schema)
	}
	return e.workingContent
}

// stampedWorkingContent is WorkingContent headed by this binary's schema
// stamp, the form a commit writes. The hierarchical form replaces the stamp
// the tree kept from its file (config.SerializeStamped) rather than adding a
// second one; the set forms print no comment, so they carry none to replace.
func (e *Editor) stampedWorkingContent() string {
	if e.treeValid && e.tree != nil && e.schema != nil {
		if e.session != nil && e.meta != nil {
			return config.FormatSchemaStamp() + config.SerializeSetWithMeta(e.tree, e.workingViewMetadata(), e.schema)
		}
		return config.SerializeStamped(e.tree, e.schema)
	}
	return config.FormatSchemaStamp() + e.workingContent
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
	if subtree != nil && schemaNode != nil {
		return config.SerializeSubtree(subtree, schemaNode)
	}
	if content, ok := e.serializeBareListAtPath(origTree, path); ok {
		return content
	}
	return ""
}

// ContentAtPath returns the serialized content at the given context path in tree format.
// Always returns tree (junos-style) format for display, regardless of session state.
// Bare keyed list paths serialize only that list's entries (or empty when none exist).
// If an unrecognized path doesn't resolve, falls back to full tree content.
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
	if subtree != nil && schemaNode != nil {
		return config.SerializeSubtree(subtree, schemaNode)
	}
	if content, ok := e.serializeBareListAtPath(e.tree, path); ok {
		return content
	}
	return config.Serialize(e.tree, e.schema)
}

// activeContentAtPath returns config text showing only active nodes (inactive pruned).
func (e *Editor) activeContentAtPath(path []string) string {
	return e.prunedContentAtPath(path, config.PruneInactive, e.workingContent)
}

// inactiveContentAtPath returns config text showing only inactive nodes.
// Active nodes are pruned; inactive nodes are shown with their full subtree.
func (e *Editor) inactiveContentAtPath(path []string) string {
	return e.prunedContentAtPath(path, config.PruneActive, "")
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

// LookupSchemaNode returns the schema node at the terminus of path, read
// against this editor's schema. Unlike WalkPathWithSchema, it walks the schema
// only (no tree walk), so it resolves leaves as well as containers and list
// entries. One-shot CLI verbs dispatch deactivate and activate on the node kind
// it answers, whether or not the value is currently set.
//
// Schema.LookupTokenPath owns the walk, because the display mask reads the same
// question and two walks over one path would answer it two ways.
func (e *Editor) LookupSchemaNode(path []string) config.Node {
	return e.schema.LookupTokenPath(path)
}

// DisplayValueAtPath answers the text a command may echo back for the value the
// operator supplied at path, read against this editor's schema. A secret reads
// as config.SecretDataPlaceholder.
func (e *Editor) DisplayValueAtPath(path []string, value string) string {
	return config.DisplayValueAtPath(e.schema, path, value)
}

// DisplayMessageAtPath answers the refusal text a command may publish about the
// value it rejected at path, read against this editor's schema.
func (e *Editor) DisplayMessageAtPath(path []string, message, value string) string {
	return config.DisplayMessageAtPath(e.schema, path, message, value)
}

// PendingChangeSummary answers the one-line form of a pending change, read
// against this editor's schema. The adoption prompt of `ze config edit` writes
// one per change, and a set summary carries the value the operator typed.
func (e *Editor) PendingChangeSummary(change config.PendingChange) string {
	return change.Summary(e.schema)
}

// ResolveLeafListValue checks whether fullPath terminates inside a
// leaf-list value (e.g. `bgp filter import filters no-self-as`). Returns the
// tree-level parent path (to the container holding the leaf-list), the
// leaf-list field name, and true if so. Handles list keys interleaved
// in the path (e.g. `bgp peer peer1 filter import filters value`).
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
		if listNode, isList := node.(*config.ListNode); isList && i < len(fullPath)-2 {
			i += config.ListAddressLen(listNode, fullPath[i:len(fullPath)-2]) // skip the entry address
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
		case *config.InlineListNode:
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

// walkBareListFrom resolves a path that stops at a list schema node without a
// key (for example ["bgp", "peer"]). It returns the parent tree (which may be
// nil when the parent container has no entries yet), the list name, the list
// schema node, and true. Anonymous lists holding KeyDefault are handled by
// walkPathWithSchemaFrom and return false here.
func (e *Editor) walkBareListFrom(tree *config.Tree, path []string) (*config.Tree, string, config.Node, bool) {
	if e.schema == nil {
		return nil, "", nil, false
	}
	if len(path) == 0 {
		return nil, "", nil, false
	}

	currentTree := tree
	var currentSchema schemaGetter = e.schema

	for i := 0; i < len(path); {
		name := path[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			return nil, "", nil, false
		}

		if i+1 == len(path) {
			switch schemaNode.(type) {
			case *config.ListNode, *config.InlineListNode:
				if currentTree != nil {
					if entries := currentTree.GetList(name); entries[config.KeyDefault] != nil {
						return nil, "", nil, false
					}
				}
				return currentTree, name, schemaNode, true
			default:
				return nil, "", nil, false
			}
		}

		switch n := schemaNode.(type) {
		case *config.ContainerNode:
			if currentTree != nil {
				currentTree = currentTree.GetContainer(name)
			}
			currentSchema = n
			i++
		case *config.FlexNode:
			if currentTree != nil {
				currentTree = currentTree.GetContainer(name)
			}
			currentSchema = n
			i++
		case *config.ListNode:
			if n.Get(path[i+1]) != nil {
				if currentTree != nil {
					currentTree = currentTree.GetList(name)[config.KeyDefault]
				}
				currentSchema = n
				i++
				continue
			}
			addr, err := config.ListPathStep(name, n, path[i+1:])
			if err != nil {
				return nil, "", nil, false
			}
			step := 1 + addr.Tokens
			if i+step >= len(path) {
				return nil, "", nil, false
			}
			if currentTree != nil {
				if entry := resolveListKey(currentTree, name, addr.Key); entry != nil {
					currentTree = entry
				} else {
					currentTree = currentTree.GetList(name)[addr.Key]
				}
			}
			currentSchema = n
			i += step
		case *config.InlineListNode:
			if n.Get(path[i+1]) != nil {
				if currentTree != nil {
					currentTree = currentTree.GetList(name)[config.KeyDefault]
				}
				currentSchema = n
				i++
				continue
			}
			if i+2 >= len(path) {
				return nil, "", nil, false
			}
			if currentTree != nil {
				currentTree = currentTree.GetList(name)[path[i+1]]
			}
			currentSchema = n
			i += 2
		default:
			return nil, "", nil, false
		}
	}

	return nil, "", nil, false
}

// serializeBareListAtPath serializes every entry of a bare list path in
// hierarchical tree format. It returns ("", true) when the path names a valid
// bare list that currently holds no entries, and ("", false) when the path does
// not end at a bare list.
func (e *Editor) serializeBareListAtPath(tree *config.Tree, path []string) (string, bool) {
	parent, name, node, ok := e.walkBareListFrom(tree, path)
	if !ok {
		return "", false
	}
	if parent == nil {
		return "", true
	}
	entries := parent.GetList(name)
	if len(entries) == 0 {
		return "", true
	}

	ordered := parent.GetListOrdered(name)
	order := make([]string, len(ordered))
	for i, item := range ordered {
		order[i] = item.Key
	}
	keys := config.ListEntryKeys(entries, order)

	var b textbuf.Buffer
	switch n := node.(type) {
	case *config.ListNode:
		if config.ListUsesMultiBlock(entries, n) {
			b.Str(name).Str(" {\n")
			for _, key := range keys {
				if entry := entries[key]; entry != nil {
					config.WritePositionalListEntry(&b, entry, key, n, "\t")
				}
			}
			b.Str("}\n")
			return b.String(), true
		}
		for _, key := range keys {
			if entry := entries[key]; entry != nil {
				config.WriteListEntry(&b, entry, name, key, n, "", "")
			}
		}
	case *config.InlineListNode:
		for _, key := range keys {
			if entry := entries[key]; entry != nil {
				config.WriteInlineListEntry(&b, entry, name, key, n, "")
			}
		}
	}
	return b.String(), true
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
		case *config.InlineListNode:
			currentSchema = n
		}
		i += step
	}

	return currentTree
}

// walkSchemaNode resolves one step of tree navigation based on the schema node type.
// Returns (navigable, subtree, step). step is how many path elements were consumed
// (the list name plus its entry address for keyed list entries, 1 for
// containers and anonymous list entries).
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
		// Keyed list entry: the address after the list name is the key.
		addr, err := config.ListPathStep(name, n, path[i+1:])
		if err != nil {
			return false, nil, 0
		}
		step := 1 + addr.Tokens
		if entries == nil {
			return true, nil, step
		}
		// Resolve #N positional index to actual key
		if entry := resolveListKey(tree, name, addr.Key); entry != nil {
			return true, entry, step
		}
		return true, entries[addr.Key], step
	case *config.FlexNode:
		return true, tree.GetContainer(name), 1
	case *config.FreeformNode:
		return true, tree.GetContainer(name), 1
	case *config.InlineListNode:
		if i+1 < len(path) && n.Get(path[i+1]) == nil {
			return true, tree.GetList(name)[path[i+1]], 2
		}
		return true, tree.GetList(name)[config.KeyDefault], 1
	case *config.LeafNode,
		*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
		return false, nil, 0 // Leaf-like nodes — can't navigate deeper
	}
	return false, nil, 0 // Unknown node type
}

// autoSelectListEntry checks if the path ends at a list node with exactly one entry.
// If so, it returns the expanded path with the single entry's key appended.
// Otherwise returns the original path unchanged.
func (e *Editor) autoSelectListEntry(path []string) []string {
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
	listNode, ok := schemaNode.(*config.ListNode)
	if !ok {
		return path
	}

	entries := parentTree.GetListOrdered(lastElem)
	if len(entries) != 1 {
		return path
	}

	// Single entry: expand the path with the entry's address, a token per
	// word, so a multi-leaf key reads back as the same address.
	return append(path, config.ListKeyTokens(listNode, entries[0].Key)...)
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

// hasPendingSessionChanges returns true if this session has pending changes in the draft.
func (e *Editor) hasPendingSessionChanges() bool {
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

// blameView returns a blame-annotated view of the configuration.
// When no metadata exists, uses an empty MetaTree to produce a consistent
// hierarchical tree format with empty blame gutters.
func (e *Editor) blameView() string {
	meta := e.meta
	if meta == nil {
		meta = config.NewMetaTree()
	}
	// A blame gutter is a display path, so every secret leaf is masked.
	// MaskSecrets clones, and the metadata tree is keyed by node rather than by
	// value, so each line still names its author.
	return config.SerializeBlame(config.MaskSecrets(e.tree, e.schema), meta, e.schema)
}

// displaySetView returns the flat set-format view of the configuration, with
// every secret leaf masked. Config search reads it, and a search result is text
// the operator sees. The format stays bare set commands with no metadata
// (AC-15: exportable format).
//
// Masking the TREE here is why search needs no secret predicate of its own. It
// used to carry one, keyed on the leaf NAME, and that was a third answer to a
// question config.LeafHoldsSecret already answers.
func (e *Editor) displaySetView() string {
	return config.SerializeSet(config.MaskSecrets(e.tree, e.schema), e.schema)
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
			key := tb.Str(entry.Entry.SessionKey()).Byte('|').Str(entry.Path.String()).Byte('|').Str(entry.Entry.Member).String()
			if seen[key] {
				continue
			}
			seen[key] = true
			all = append(all, entry)
		}
	}

	if e.meta != nil {
		if sessionID == "" {
			for _, sid := range e.meta.AllSessions() {
				addEntries(e.meta.SessionEntries(e.schema, sid))
			}
		} else {
			addEntries(e.meta.SessionEntries(e.schema, sessionID))
		}
	}

	for _, cf := range e.listChangeFiles(e.store) {
		meta, _, err := e.readChangeFileContent(e.store, cf)
		if err != nil {
			continue
		}
		if sessionID != "" {
			addEntries(meta.SessionEntries(e.schema, sessionID))
			continue
		}
		for _, sid := range meta.AllSessions() {
			addEntries(meta.SessionEntries(e.schema, sid))
		}
	}

	return all
}

// PendingChanges returns the best-effort pending inventory for a specific
// session, or all sessions when sessionID is empty. An unreadable durable file
// contributes no records; PendingReview instead reports that read failure.
func (e *Editor) PendingChanges(sessionID string) []config.PendingChange {
	return e.pendingChanges(e.store, sessionID)
}

type pendingChangeReader interface {
	ReadFile(string) ([]byte, error)
	List(string) ([]string, error)
}

func (e *Editor) pendingChanges(reader pendingChangeReader, sessionID string) []config.PendingChange {
	// Empty selects every owner, independently of the active session.
	if sessionID != "" {
		if e.session != nil {
			if sessionID == e.session.ID {
				if reader == nil {
					return e.pendingSessionChanges(nil, nil, sessionID)
				}
				// The current session already knows its account's change path.
				// A directory census adds a failure unrelated to reading that file.
				meta, ops, err := e.readChangeFileContent(reader, ChangePath(e.originalPath, e.session.User))
				if err != nil {
					// Preserve this inventory API's existing best-effort behavior:
					// readable memory survives an unavailable durable source.
					return e.pendingSessionChanges(nil, nil, sessionID)
				}
				return e.pendingSessionChanges(meta, ops, sessionID)
			}
		}
	}

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
		if sessionID == "" {
			for _, sid := range e.meta.AllSessions() {
				for _, entry := range e.meta.SessionEntries(e.schema, sid) {
					addChange(config.PendingChangeFromSessionEntry(entry))
				}
			}
		} else {
			for _, entry := range e.meta.SessionEntries(e.schema, sessionID) {
				addChange(config.PendingChangeFromSessionEntry(entry))
			}
		}
	}

	for _, cf := range e.listChangeFiles(reader) {
		meta, ops, err := e.readChangeFileContent(reader, cf)
		if err != nil {
			continue
		}
		if sessionID != "" {
			for _, entry := range meta.SessionEntries(e.schema, sessionID) {
				addChange(config.PendingChangeFromSessionEntry(entry))
			}
			for i := range ops {
				if ops[i].SessionKey() == sessionID {
					addChange(ops[i].PendingChange(e.schema))
				}
			}
			continue
		}
		for _, sid := range meta.AllSessions() {
			for _, entry := range meta.SessionEntries(e.schema, sid) {
				addChange(config.PendingChangeFromSessionEntry(entry))
			}
		}
		for i := range ops {
			addChange(ops[i].PendingChange(e.schema))
		}
	}

	config.SortPendingChanges(changes)
	return changes
}

// pendingSessionChanges is the single logical record collector for the active
// session's inventory and review. The metadata and recorded operations bound
// every loop. It changes no ownership or conflict baseline; presentation may
// replace Previous on the returned copies after collecting them.
func (e *Editor) pendingSessionChanges(meta *config.MetaTree, ops []config.StructuralOp, sessionID string) []config.PendingChange {
	seen := make(map[string]bool)
	var changes []config.PendingChange
	add := func(change config.PendingChange) {
		key := pendingChangeKey(change)
		if seen[key] {
			return
		}
		seen[key] = true
		changes = append(changes, change)
	}
	for _, source := range []*config.MetaTree{e.meta, meta} {
		if source == nil {
			continue
		}
		for _, entry := range source.SessionEntries(e.schema, sessionID) {
			add(config.PendingChangeFromSessionEntry(entry))
		}
	}
	for i := range ops {
		if ops[i].SessionKey() != sessionID {
			continue
		}
		add(ops[i].PendingChange(e.schema))
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
	slices.Sort(sessions)
	return sessions
}

// listChangeFiles returns all per-user change file paths for this config.
func (e *Editor) listChangeFiles(reader pendingChangeReader) []string {
	if reader == nil {
		return nil
	}
	dir := filepath.Dir(e.originalPath)
	files, err := reader.List(dir)
	if err != nil {
		return nil
	}

	return e.selectChangeFiles(files)
}

// selectChangeFiles applies the same config-specific selection to permissive
// display reads and to the strict materialized-draft replay listing.
func (e *Editor) selectChangeFiles(files []string) []string {
	prefix := changePrefix(e.originalPath)

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

func (e *Editor) readChangeFileContent(reader pendingChangeReader, path string) (*config.MetaTree, []config.StructuralOp, error) {
	data, err := reader.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	_, meta, ops, parseErr := config.ParseCandidateChangeFile(string(data), e.schema)
	return meta, ops, parseErr
}

func pendingChangeKey(change config.PendingChange) string {
	var tb textbuf.Buffer
	// Rename and copy identity needs both paths; other kinds use the generic key.
	//exhaustive:ignore
	switch change.Kind {
	case config.PendingChangeRename, config.PendingChangeCopy:
		return tb.Str(change.SessionID).Byte('|').Str(string(change.Kind)).Byte('|').Str(change.OldPath).Byte('|').Str(change.NewPath).String()
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
	if session != nil {
		e.startDiscardNotice()
	}
}

// Diff returns a path-qualified line diff between the committed config and the
// working config, with every secret leaf masked.
//
// In an active storage-backed editing session, Diff delegates to PendingReview
// (the same RenderPendingChanges renderer web EditorManager.Diff uses), so
// every line names its full configuration path and identical leaf lines under
// distinct sibling blocks never collapse. Non-session editors with parsed trees
// compare masked set-format lines via ordered LCS and append changed-secret
// notices; unparsed raw fallback editors compare ordered display lines.
//
// Four sinks read Diff:
//
//   - the commit and discard audit records (model_commands_commit.go)
//   - the REST and gRPC session diff (internal/component/api/config_session.go)
//   - the hub (cmd/ze/hub/editor_adapter.go)
//   - the --dry-run form of `ze config set` and `ze config deactivate`
func (e *Editor) Diff() string {
	if e.session != nil && e.store != nil && !e.looseFile {
		review, err := e.PendingReview(nil)
		if err == nil {
			return review
		}
		draftLogger.Warn("session pending review failed during diff",
			"code", diagnostic.CodeDoctorStoreIntegrity, "error", err)
	}

	var b textbuf.Buffer
	if e.schema != nil && e.treeValid && e.tree != nil {
		if origTree := e.committedTree(); origTree != nil {
			origSet := stripSetPrefixes(config.SerializeSet(config.MaskSecrets(origTree, e.schema), e.schema))
			workSet := stripSetPrefixes(config.SerializeSet(config.MaskSecrets(e.tree, e.schema), e.schema))
			b.Str(diffOrderedLines(origSet, workSet))
			for _, leafPath := range e.changedSecretsAt(nil) {
				b.Str(secretChangeLine(leafPath)).Byte('\n')
			}
			return b.String()
		}
	}

	b.Str(diffOrderedLines(e.DisplayOriginalContentAtPath(nil), e.DisplayContentAtPath(nil)))
	for _, leafPath := range e.changedSecretsAt(nil) {
		b.Str(secretChangeLine(leafPath)).Byte('\n')
	}
	return b.String()
}

// stripSetPrefixes removes the leading "set " verb from SerializeSet lines so
// non-session path-qualified diffs match RenderPendingChanges formatting.
func stripSetPrefixes(content string) string {
	if content == "" {
		return ""
	}
	var b textbuf.Buffer
	for _, line := range splitDiffLines(content) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.Str(strings.TrimPrefix(line, "set ")).Byte('\n')
	}
	return b.String()
}

// diffOrderedLines computes an ordered LCS line diff emitting separate removed
// ("- ") and added ("+ ") lines without collapsing duplicate lines across
// distinct blocks.
func diffOrderedLines(original, modified string) string {
	if original == modified {
		return ""
	}

	origLines := splitDiffLines(original)
	modLines := splitDiffLines(modified)
	dp := lcsTable(origLines, modLines)
	raw := backtrackDiff(dp, origLines, modLines)

	var diff textbuf.Buffer
	for _, dl := range raw {
		if strings.TrimSpace(dl.Text) == "" {
			continue
		}
		switch dl.Marker {
		case diffRemoved:
			diff.Str("- ").Str(dl.Text).Byte('\n')
		case diffAdded:
			diff.Str("+ ").Str(dl.Text).Byte('\n')
		case diffUnchanged, diffModified:
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
//
// The stamp IS the version's key and has millisecond resolution, so two backups
// taken inside one millisecond would write the same key and the second would
// replace the first -- one fewer point to roll back to, reported nowhere. This
// caller derives its stamp from the clock and does not depend on its exact value
// (unlike pointer.go, which names the stamp in a pointer it writes), so it is the
// right place to step to a free slot.
func (e *Editor) createBackup(content string, guard storage.WriteGuard) error {
	if e.store == nil {
		return nil
	}
	now := e.nextBackupStamp()
	if guard != nil {
		return guard.WriteVersion(e.originalPath, []byte(content), now)
	}
	return e.store.WriteVersion(e.originalPath, []byte(content), now)
}

// nextBackupStamp returns a stamp strictly later, at MILLISECOND resolution,
// than the previous backup this editor took.
//
// Millisecond is the resolution that matters because the stamp IS the version's
// storage key (storage.FormatVersionStamp), so two backups inside one
// millisecond write one key and the second replaces the first.
//
// Deliberately derived from in-memory state, never from the store. createBackup
// is called both inside and outside a lock, and a guarded section must not reach
// for an unlocked Storage method: those re-acquire the store mutex against the
// write lock the guard already holds, which is exactly what WriteGuard.Has
// exists to avoid. Listing versions here to find a free slot deadlocked that
// way, hanging the cli and hub package tests until their 600s timeout.
//
// Truncation is load-bearing: comparing untruncated times would accept two
// stamps microseconds apart, which format to the same key and collide anyway.
func (e *Editor) nextBackupStamp() time.Time {
	now := e.now().Truncate(time.Millisecond)
	if !now.After(e.lastBackupStamp) {
		now = e.lastBackupStamp.Add(time.Millisecond)
	}
	e.lastBackupStamp = now
	return now
}

// HasHistory reports whether commits can record persistent rollback versions.
func (e *Editor) HasHistory() bool {
	return e.store != nil
}

// ListBackups returns available backup files, sorted by timestamp descending.
func (e *Editor) ListBackups() ([]BackupInfo, error) {
	if e.store == nil {
		return nil, fmt.Errorf("config history: %w; run ze init", storage.ErrNoStore)
	}
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

// readBackupContent reads the content of a backup by its history entry key,
// through the entry's object with the hash verified.
func (e *Editor) readBackupContent(path string) ([]byte, error) {
	return storage.ReadVersionEntry(e.store, e.originalPath, path)
}

// HasDraft returns true if a draft file exists for this config.
func (e *Editor) HasDraft() bool {
	if e.store == nil {
		return false
	}
	return e.fileExists(DraftPath(e.originalPath))
}

// livePath returns the path to the .live.conf file.
// This file holds the trial config during a "commit confirmed" window.
func (e *Editor) livePath() string {
	dir := filepath.Dir(e.originalPath)
	base := filepath.Base(e.originalPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, name+".live"+ext)
}

// saveLive writes the current working content to the .live.conf file.
// Used by "commit confirmed" to create the trial config.
func (e *Editor) saveLive() error {
	content := e.WorkingContent()
	if err := e.writeFile(e.livePath(), []byte(content)); err != nil {
		return fmt.Errorf("failed to write live config: %w", err)
	}
	return nil
}

// HasPendingLive returns true if a .live.conf file exists.
// This indicates an unconfirmed "commit confirmed" from a previous session.
func (e *Editor) HasPendingLive() bool {
	return e.fileExists(e.livePath())
}

// deleteLive removes the .live.conf file if it exists.
// Errors are ignored because the file may not exist.
func (e *Editor) deleteLive() {
	_ = e.removeFile(e.livePath()) // Ignore error if doesn't exist
}

// Rollback restores history into a session's candidate for review and commit.
// Without a session it backs up the current configuration and restores directly.
// Both forms are refused while the daemon's confirmed-commit window is open:
// its revert could invalidate the restored view or overwrite a direct restore.
// The window's refusal names its owner; that owner's refusal also points to
// `commit abort`, which ends the window.
func (e *Editor) Rollback(backupPath string) error {
	err := confirm.WriteOutside(e.daemonWindow(), func() error {
		return e.rollback(backupPath)
	})
	var other *confirm.OtherUserError
	if !errors.As(err, &other) {
		return err
	}
	if e.session == nil {
		return err
	}
	if other.Owner != e.session.User {
		return err
	}
	return fmt.Errorf("rollback refused while your confirmed commit is pending: run '%s' to revert it, then roll back (%w)",
		contract.CommitCommand(contract.CommitAbort), err)
}

// rollback is Rollback once the window allows it.
func (e *Editor) rollback(backupPath string) error {
	// Read backup content through its object, hash verified.
	data, err := e.readBackupContent(backupPath)
	if err != nil {
		return fmt.Errorf("cannot read backup: %w", err)
	}
	if e.session != nil {
		// A session restores history into its candidate, the same atomic load
		// as an operator-pasted replacement. Publishing here would bypass the
		// session's review, conflict checks and daemon apply transaction.
		loaded, err := e.ParseLoad(string(data), nil)
		if err != nil {
			return fmt.Errorf("cannot load saved configuration: %w", err)
		}
		return e.LoadReplace(nil, loaded)
	}

	// Read current committed config from storage (not cache) for accurate backup.
	currentData, readErr := e.readFile(e.originalPath)
	if readErr != nil {
		return fmt.Errorf("cannot read current config for backup: %w", readErr)
	}
	if err := e.createBackup(string(currentData), nil); err != nil {
		return fmt.Errorf("cannot backup current config before rollback: %w", err)
	}

	// Write to original path
	if err := e.writeFile(e.originalPath, data); err != nil {
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
