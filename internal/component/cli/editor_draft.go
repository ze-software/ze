// Design: docs/architecture/config/yang-config-design.md — write-through draft protocol
// Overview: editor.go — config editor (calls write-through from SetValue/DeleteValue)
// Detail: editor_walk.go — schema-aware tree/meta walking
// Detail: editor_commit.go — commit/discard/disconnect protocol
// Related: editor_session.go — session identity for concurrent editing

package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// ConflictType identifies whether a conflict is a live disagreement or stale previous.
type ConflictType int

const (
	// ConflictLive means another active session has a pending change at the same path with a different value.
	ConflictLive ConflictType = iota
	// ConflictStale means the committed value changed since this session's edit.
	ConflictStale
)

// Conflict describes a single conflict detected during commit.
type Conflict struct {
	Path          string       // YANG path (space-separated).
	Type          ConflictType // Live disagreement or stale previous.
	MyValue       string       // Value this session wants to set.
	OtherValue    string       // Conflicting value (committed or other session).
	OtherUser     string       // Who made the conflicting change.
	PreviousValue string       // What config.conf had when this session edited.
}

// CommitResult holds the outcome of a CommitSession attempt.
type CommitResult struct {
	Conflicts        []Conflict // Non-empty if commit was blocked.
	Applied          int        // Number of changes applied (0 if conflicts).
	MigrationWarning string     // Non-empty if tree structure migration failed (format conversion still applied).
}

// writeThroughSet implements the write-through protocol for set commands.
// Writes to the per-user change file (not shared draft). The change file
// contains only changed entries (sparse tree), not a full config dump.
func (e *Editor) writeThroughSet(path []string, key, value string) error {
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	// Validate the path against the schema before mutating anything.
	// Use a temporary tree to check walkOrCreateIn succeeds.
	if _, walkErr := e.walkOrCreateIn(e.tree.Clone(), path); walkErr != nil {
		return fmt.Errorf("write-through set path: %w", walkErr)
	}

	// Read change file (sparse tree of this user's changes).
	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, err := e.readChangeFile(guard, changePath)
	if err != nil {
		return fmt.Errorf("write-through read change: %w", err)
	}

	// Apply the set to the change tree.
	changeTarget, err := e.walkOrCreateIn(changeTree, path)
	if err != nil {
		return fmt.Errorf("write-through set change path: %w", err)
	}
	changeTarget.Set(key, value)

	// Build the YANG path for metadata recording.
	metaPath := append(path, key) //nolint:gocritic // intentional new slice

	// Read committed value for Previous field.
	previous := ""
	if committedTree := e.readCommittedTree(guard); committedTree != nil {
		previous = getValueAtPath(committedTree, e.schema, metaPath)
	}

	// Record metadata in change file.
	entry := config.MetaEntry{
		User:     e.session.User,
		Source:   e.session.Origin,
		Time:     e.session.StartTime,
		Previous: previous,
		Value:    value,
	}
	changeMetaTarget := walkOrCreateMeta(changeMeta, e.schema, path)
	changeMetaTarget.SetEntry(key, entry)

	// Serialize and write change file.
	output := config.SerializeSetWithMeta(changeTree, changeMeta, e.schema)
	if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
	}

	// Update in-memory tree directly (base + own changes).
	target, _ := e.walkOrCreateIn(e.tree, path)
	target.Set(key, value)
	metaTarget := walkOrCreateMeta(e.meta, e.schema, path)
	metaTarget.SetEntry(key, entry)

	e.dirty.Store(true)
	return nil
}

// writeThroughDelete implements the write-through protocol for delete commands.
// Writes to the per-user change file (not shared draft).
func (e *Editor) writeThroughDelete(path []string, key string) error {
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	// Verify path exists in in-memory tree before mutating.
	target := walkPath(e.tree, e.schema, path)
	if target == nil {
		return fmt.Errorf("path not found")
	}

	// Read change file.
	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, err := e.readChangeFile(guard, changePath)
	if err != nil {
		return fmt.Errorf("write-through read change: %w", err)
	}

	// Read committed value for Previous field.
	metaPath := append(path, key) //nolint:gocritic // intentional new slice
	previous := ""
	if committedTree := e.readCommittedTree(guard); committedTree != nil {
		previous = getValueAtPath(committedTree, e.schema, metaPath)
	}

	// Create the parent path in the change tree so the serializer can navigate
	// to the metadata node. The leaf itself is NOT set (it's a delete).
	if _, walkErr := e.walkOrCreateIn(changeTree, path); walkErr != nil {
		return fmt.Errorf("write-through delete change path: %w", walkErr)
	}

	// Record delete metadata in change file. The serializer emits "delete" lines
	// for metadata entries without corresponding tree values.
	entry := config.MetaEntry{
		User:     e.session.User,
		Source:   e.session.Origin,
		Time:     e.session.StartTime,
		Previous: previous,
	}
	changeMetaTarget := walkOrCreateMeta(changeMeta, e.schema, path)
	changeMetaTarget.SetEntry(key, entry)

	// Serialize and write change file.
	output := config.SerializeSetWithMeta(changeTree, changeMeta, e.schema)
	if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
	}

	// Update in-memory tree.
	target.Delete(key)
	metaTarget := walkOrCreateMeta(e.meta, e.schema, path)
	metaTarget.SetEntry(key, entry)

	e.dirty.Store(true)
	return nil
}

// readChangeFile reads and parses a per-user change file.
// Returns empty tree and meta if the file does not exist.
func (e *Editor) readChangeFile(guard storage.WriteGuard, changePath string) (*config.Tree, *config.MetaTree, error) {
	data, readErr := guard.ReadFile(changePath)
	if readErr != nil {
		// No change file: start with empty sparse tree.
		return config.NewTree(), config.NewMetaTree(), nil //nolint:nilerr // file-not-found is expected, start fresh
	}
	parser := config.NewSetParser(e.schema)
	tree, meta, parseErr := parser.ParseWithMeta(string(data))
	if parseErr != nil {
		return nil, nil, fmt.Errorf("parse change file: %w", parseErr)
	}
	return tree, meta, nil
}

// SaveDraft applies changes from the per-user change file to config.conf.draft.
// Creates a new draft (base + own changes), then deletes the change file.
func (e *Editor) SaveDraft() error {
	if e.session == nil {
		return fmt.Errorf("no session set")
	}

	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("save lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock

	// Read the change file.
	changePath := ChangePath(e.originalPath, e.session.User)
	_, changeMeta, err := e.readChangeFile(guard, changePath)
	if err != nil {
		return fmt.Errorf("save read change: %w", err)
	}

	myEntries := changeMeta.SessionEntries(e.session.ID)
	if len(myEntries) == 0 {
		return nil // Nothing to save.
	}

	// Read base (draft if exists, else committed).
	draftPath := DraftPath(e.originalPath)
	baseTree, baseMeta, err := e.readDraftOrConfig(guard, draftPath)
	if err != nil {
		return fmt.Errorf("save read base: %w", err)
	}

	// Apply changes to base.
	for _, se := range myEntries {
		pathParts := strings.Fields(se.Path)
		if len(pathParts) == 0 {
			continue
		}
		leafName := pathParts[len(pathParts)-1]
		parentPath := pathParts[:len(pathParts)-1]

		if se.Entry.Value != "" {
			target, walkErr := e.walkOrCreateIn(baseTree, parentPath)
			if walkErr != nil {
				return fmt.Errorf("save apply %s: %w", se.Path, walkErr)
			}
			target.Set(leafName, se.Entry.Value)
		} else {
			target := walkPath(baseTree, e.schema, parentPath)
			if target != nil {
				target.Delete(leafName)
			}
		}

		// Record metadata in draft.
		metaTarget := walkOrCreateMeta(baseMeta, e.schema, parentPath)
		metaTarget.SetEntry(leafName, se.Entry)
	}

	// Write draft, tagging the modifier so CheckDraftChanged can identify the author.
	guard.SetModifier(e.session.ID)
	draftOutput := config.SerializeSetWithMeta(baseTree, baseMeta, e.schema)
	if err := guard.WriteFile(draftPath, []byte(draftOutput), 0o600); err != nil {
		return fmt.Errorf("save write draft: %w", err)
	}

	// Delete change file. If removal fails, return error to prevent in-memory
	// state update — an orphaned change file would cause duplicate application.
	if err := guard.Remove(changePath); err != nil {
		return fmt.Errorf("save remove change file: %w", err)
	}

	// Update in-memory state to draft.
	e.tree = baseTree
	e.meta = baseMeta
	e.draftMtime = time.Now()
	return nil
}

// DetectConflicts scans all change files for other users and returns conflicts
// where another user has a pending change at the same path with a different value.
func (e *Editor) DetectConflicts() []Conflict {
	if e.session == nil || e.meta == nil {
		return nil
	}

	// List files in config directory.
	dir := filepath.Dir(e.originalPath)
	files, err := e.store.List(dir)
	if err != nil {
		return nil
	}

	prefix := ChangePrefix(e.originalPath)
	myUser := e.session.User

	// Collect this session's entries from in-memory meta.
	myEntries := e.meta.SessionEntries(e.session.ID)
	if len(myEntries) == 0 {
		return nil
	}

	// Build lookup of my entries by path.
	myByPath := make(map[string]string, len(myEntries))
	for _, se := range myEntries {
		myByPath[se.Path] = se.Entry.Value
	}

	var conflicts []Conflict

	for _, f := range files {
		base := filepath.Base(f)
		if !strings.HasPrefix(base, prefix) {
			continue
		}
		otherUser := strings.TrimPrefix(base, prefix)
		if otherUser == myUser {
			continue
		}

		// Read and parse the other user's change file.
		data, readErr := e.store.ReadFile(f)
		if readErr != nil {
			continue
		}
		parser := config.NewSetParser(e.schema)
		_, otherMeta, parseErr := parser.ParseWithMeta(string(data))
		if parseErr != nil {
			continue
		}

		// Check for overlapping paths with different values.
		for _, sid := range otherMeta.AllSessions() {
			for _, otherEntry := range otherMeta.SessionEntries(sid) {
				myValue, overlap := myByPath[otherEntry.Path]
				if !overlap {
					continue
				}
				if myValue != otherEntry.Entry.Value {
					conflicts = append(conflicts, Conflict{
						Path:       otherEntry.Path,
						Type:       ConflictLive,
						MyValue:    myValue,
						OtherValue: otherEntry.Entry.Value,
						OtherUser:  otherUser,
					})
				}
			}
		}
	}

	return conflicts
}

// readDraftOrConfig reads and parses the draft file, falling back to config.conf.
// Returns the parsed tree and metadata. Uses guard for I/O (called within locked sections).
func (e *Editor) readDraftOrConfig(guard storage.WriteGuard, draftPath string) (*config.Tree, *config.MetaTree, error) {
	parser := config.NewSetParser(e.schema)

	data, err := guard.ReadFile(draftPath)
	if err == nil {
		tree, meta, parseErr := parser.ParseWithMeta(string(data))
		if parseErr != nil {
			return nil, nil, fmt.Errorf("parse draft: %w", parseErr)
		}
		return tree, meta, nil
	}

	// No draft exists: clone the in-memory tree (already parsed from any format)
	// and start with empty metadata.
	return e.tree.Clone(), config.NewMetaTree(), nil
}

// readCommittedTree reads and parses config.conf under lock.
// Re-reads each time to capture external commits between write-through calls.
// Returns nil if the file cannot be read or parsed.
func (e *Editor) readCommittedTree(guard storage.WriteGuard) *config.Tree {
	data, err := guard.ReadFile(e.originalPath)
	if err != nil {
		return nil
	}
	tree, _, err := parseConfigWithFormat(string(data), e.schema)
	if err != nil {
		return nil
	}
	return tree
}

// AdoptSession rewrites all entries belonging to oldSessionID to the current session.
// Used when a user reconnects and wants to take over an orphaned session's changes.
func (e *Editor) AdoptSession(oldSessionID string) error {
	if e.session == nil {
		return fmt.Errorf("no session set")
	}

	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("adopt lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock

	draftPath := DraftPath(e.originalPath)
	draftData, err := guard.ReadFile(draftPath)
	if err != nil {
		return fmt.Errorf("read draft: %w", err)
	}

	setParser := config.NewSetParser(e.schema)
	tree, meta, err := setParser.ParseWithMeta(string(draftData))
	if err != nil {
		return fmt.Errorf("parse draft: %w", err)
	}

	// Find all entries for the old session and rewrite to current session.
	oldEntries := meta.SessionEntries(oldSessionID)
	if len(oldEntries) == 0 {
		return nil // Nothing to adopt.
	}

	for _, se := range oldEntries {
		pathParts := strings.Fields(se.Path)
		if len(pathParts) == 0 {
			continue
		}
		leafName := pathParts[len(pathParts)-1]
		parentPath := pathParts[:len(pathParts)-1]

		metaTarget := walkOrCreateMeta(meta, e.schema, parentPath)
		metaTarget.RemoveSessionEntry(leafName, oldSessionID)
		metaTarget.SetEntry(leafName, config.MetaEntry{
			User:     e.session.User,
			Source:   e.session.Origin,
			Time:     e.session.StartTime,
			Previous: se.Entry.Previous,
			Value:    se.Entry.Value,
		})
	}

	// Serialize and write updated draft.
	output := config.SerializeSetWithMeta(tree, meta, e.schema)
	if err := guard.WriteFile(draftPath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write draft: %w", err)
	}

	// Update in-memory state.
	e.tree = tree
	e.meta = meta
	e.dirty.Store(true)
	return nil
}

// CheckDraftChanged checks if the draft file has been modified by another session.
// Uses Storage.Stat for both filesystem (OS mtime) and blob (tracked mtime).
// Returns true if the draft mtime is newer than the last known mtime.
// Also re-reads and re-parses the draft on change to update in-memory state.
func (e *Editor) CheckDraftChanged() (changed bool, notification string) {
	if e.session == nil {
		return false, ""
	}

	draftPath := DraftPath(e.originalPath)
	meta, err := e.store.Stat(draftPath)
	if err != nil || meta.ModTime.IsZero() {
		return false, ""
	}

	if e.draftMtime.IsZero() {
		e.draftMtime = meta.ModTime
		return false, ""
	}

	if !meta.ModTime.After(e.draftMtime) {
		return false, ""
	}

	e.draftMtime = meta.ModTime

	// If the current session made the change, silently update mtime and skip notification.
	if meta.ModifiedBy != "" && meta.ModifiedBy == e.session.ID {
		return false, ""
	}

	// Re-read and re-parse the draft to update in-memory state.
	data, readErr := e.store.ReadFile(draftPath)
	if readErr != nil {
		return false, ""
	}
	tree, draftMeta, parseErr := parseConfigWithFormat(string(data), e.schema)
	if parseErr != nil {
		return false, ""
	}
	e.tree = tree
	e.meta = draftMeta

	msg := "Draft updated by another session"
	if meta.ModifiedBy != "" {
		msg += " (" + meta.ModifiedBy + ")"
	}
	return true, msg
}

// parseConfigWithFormat reads config content using format auto-detection.
// Returns the tree and any existing metadata (nil for hierarchical format).
func parseConfigWithFormat(content string, schema *config.Schema) (*config.Tree, *config.MetaTree, error) {
	format := config.DetectFormat(content)

	switch format {
	case config.FormatSetMeta:
		return config.NewSetParser(schema).ParseWithMeta(content)
	case config.FormatSet:
		tree, err := config.NewSetParser(schema).Parse(content)
		return tree, config.NewMetaTree(), err
	case config.FormatHierarchical:
		tree, err := config.NewParser(schema).Parse(content)
		return tree, config.NewMetaTree(), err
	}

	tree, err := config.NewParser(schema).Parse(content)
	return tree, config.NewMetaTree(), err
}
