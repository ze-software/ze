// Design: docs/architecture/config/yang-config-design.md — write-through draft protocol
// Overview: editor.go — config editor (calls write-through from SetValue/DeleteValue)
// Detail: editor_walk.go — schema-aware tree/meta walking
// Detail: editor_commit.go — commit/discard/disconnect protocol
// Related: editor_session.go — session identity for concurrent editing

package cli

import (
	"fmt"
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
// Steps: lock -> read draft -> parse -> apply set -> record metadata -> serialize -> write draft -> unlock.
func (e *Editor) writeThroughSet(path []string, key, value string) error {
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths

	// Read draft (or config.conf if no draft exists).
	draftPath := DraftPath(e.originalPath)
	tree, meta, err := e.readDraftOrConfig(guard, draftPath)
	if err != nil {
		return fmt.Errorf("write-through read: %w", err)
	}

	// Apply the set to the draft tree.
	target, err := e.walkOrCreateIn(tree, path)
	if err != nil {
		return err
	}
	target.Set(key, value)

	// Build the YANG path for metadata recording.
	metaPath := append(path, key) //nolint:gocritic // intentional new slice

	// Read committed value for Previous field (re-read under lock
	// to capture external commits; reuses getValueAtPath for navigation).
	previous := ""
	if committedTree := e.readCommittedTree(guard); committedTree != nil {
		previous = getValueAtPath(committedTree, e.schema, metaPath)
	}

	// Record metadata at the leaf.
	now := time.Now()
	entry := config.MetaEntry{
		User:     e.session.UserAtOrigin(),
		Time:     now,
		Session:  e.session.ID,
		Previous: previous,
		Value:    value,
	}
	metaTarget := walkOrCreateMeta(meta, e.schema, path)
	metaTarget.SetEntry(key, entry)

	// Serialize and write draft through the guard.
	output := config.SerializeSetWithMeta(tree, meta, e.schema)
	if err := guard.WriteFile(draftPath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
	}

	// Update in-memory state.
	e.tree = tree
	e.meta = meta
	e.dirty.Store(true)
	return nil
}

// writeThroughDelete implements the write-through protocol for delete commands.
// Steps: lock -> read draft -> parse -> apply delete -> serialize -> write draft -> unlock.
func (e *Editor) writeThroughDelete(path []string, key string) error {
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths

	// Read draft (or config.conf if no draft exists).
	draftPath := DraftPath(e.originalPath)
	tree, meta, err := e.readDraftOrConfig(guard, draftPath)
	if err != nil {
		return fmt.Errorf("write-through read: %w", err)
	}

	// Apply the delete to the draft tree.
	target := walkPath(tree, e.schema, path)
	if target == nil {
		return fmt.Errorf("path not found")
	}
	target.Delete(key)

	// Read committed value for Previous field (re-read under lock
	// to capture external commits, matching writeThroughSet behavior).
	metaPath := append(path, key) //nolint:gocritic // intentional new slice
	previous := ""
	if committedTree := e.readCommittedTree(guard); committedTree != nil {
		previous = getValueAtPath(committedTree, e.schema, metaPath)
	}

	// Record metadata for the delete. The serializer emits "delete" lines
	// for metadata entries without corresponding tree values, so Previous
	// survives the serialize->parse round-trip.
	now := time.Now()
	entry := config.MetaEntry{
		User:     e.session.UserAtOrigin(),
		Time:     now,
		Session:  e.session.ID,
		Previous: previous,
	}
	metaTarget := walkOrCreateMeta(meta, e.schema, path)
	metaTarget.SetEntry(key, entry)

	// Serialize and write draft through the guard.
	output := config.SerializeSetWithMeta(tree, meta, e.schema)
	if err := guard.WriteFile(draftPath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
	}

	// Update in-memory state.
	e.tree = tree
	e.meta = meta
	e.dirty.Store(true)
	return nil
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
