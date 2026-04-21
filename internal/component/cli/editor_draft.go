// Design: docs/architecture/config/yang-config-design.md — write-through draft protocol
// Overview: editor.go — config editor (calls write-through from SetValue/DeleteValue)
// Detail: editor_walk.go — schema-aware tree/meta walking
// Detail: editor_commit.go — commit/discard/disconnect protocol
// Related: editor_session.go — session identity for concurrent editing

package cli

import (
	"fmt"

	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/cli/contract"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

var draftLogger = slogutil.Logger("cli.editor.draft")

// ConflictType is a type alias of contract.ConflictType.
type ConflictType = contract.ConflictType

// Re-export contract conflict constants for backward compatibility.
var (
	ConflictLive  = contract.ConflictLive
	ConflictStale = contract.ConflictStale
)

// Conflict describes a single conflict detected during commit.
// Conflict is a type alias of contract.Conflict.
type Conflict = contract.Conflict

// CommitResult holds the outcome of a CommitSession attempt.
// CommitResult is a type alias of contract.CommitResult.
type CommitResult = contract.CommitResult

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
	changeTree, changeMeta, changeOps := e.readChangeFile(guard, changePath)

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
	output := config.SerializeChangeFile(changeTree, changeMeta, changeOps, e.schema)
	if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
	}

	// Update in-memory tree directly (base + own changes).
	target, _ := e.walkOrCreateIn(e.tree, path)
	target.Set(key, value)
	metaTarget := walkOrCreateMeta(e.meta, e.schema, path)
	metaTarget.SetEntry(key, entry)

	e.dirty.Store(true)
	e.draftSaved = false // New edit after save means unsaved changes
	return nil
}

// writeThroughCreate implements the write-through protocol for creating empty list entries.
// It ensures the path exists in both the change file and in-memory tree without setting any leaf.
func (e *Editor) writeThroughCreate(path []string) error {
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	// Validate the path against the schema.
	if _, walkErr := e.walkOrCreateIn(e.tree.Clone(), path); walkErr != nil {
		return fmt.Errorf("write-through create path: %w", walkErr)
	}

	// Read change file.
	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps := e.readChangeFile(guard, changePath)

	// Create the path in the change tree (no leaf set).
	if _, walkErr := e.walkOrCreateIn(changeTree, path); walkErr != nil {
		return fmt.Errorf("write-through create change path: %w", walkErr)
	}

	// Serialize and write change file.
	output := config.SerializeChangeFile(changeTree, changeMeta, changeOps, e.schema)
	if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
	}

	// Update in-memory tree.
	if _, walkErr := e.walkOrCreateIn(e.tree, path); walkErr != nil {
		return fmt.Errorf("write-through create in-memory: %w", walkErr)
	}

	e.dirty.Store(true)
	e.draftSaved = false
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
	changeTree, changeMeta, changeOps := e.readChangeFile(guard, changePath)

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
	output := config.SerializeChangeFile(changeTree, changeMeta, changeOps, e.schema)
	if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
	}

	// Update in-memory tree.
	target.Delete(key)
	metaTarget := walkOrCreateMeta(e.meta, e.schema, path)
	metaTarget.SetEntry(key, entry)

	e.dirty.Store(true)
	e.draftSaved = false // New edit after save means unsaved changes
	return nil
}

// writeThroughRename records a structural rename in the per-user change file
// and immediately rebases any pending subtree edits onto the new key.
func (e *Editor) writeThroughRename(parentPath []string, listName, oldKey, newKey string) error {
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	if oldKey == newKey {
		return fmt.Errorf("new key must differ from current key")
	}

	working := e.tree.Clone()
	var validateTarget *config.Tree
	if len(parentPath) == 0 {
		validateTarget = working
	} else {
		validateTarget = walkPath(working, e.schema, parentPath)
	}
	if validateTarget == nil {
		return fmt.Errorf("path not found")
	}
	if err := validateTarget.RenameListEntry(listName, oldKey, newKey); err != nil {
		return err
	}

	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps := e.readChangeFile(guard, changePath)
	proposedOp := config.StructuralOp{
		Type:       config.StructuralOpRename,
		User:       e.session.User,
		Source:     e.session.Origin,
		Time:       e.session.StartTime,
		ParentPath: strings.Join(parentPath, " "),
		ListName:   listName,
		OldKey:     oldKey,
		NewKey:     newKey,
	}
	if err := e.validateRenameLiveConflict(guard, proposedOp.PendingChange()); err != nil {
		return err
	}

	var changeTarget *config.Tree
	if len(parentPath) == 0 {
		changeTarget = changeTree
	} else {
		changeTarget = walkPath(changeTree, e.schema, parentPath)
	}
	if changeTarget != nil {
		if err := renameTreeListEntry(changeTarget, listName, oldKey, newKey); err != nil {
			return err
		}
	}

	changeMetaTarget := walkMetaReadOnly(changeMeta, e.schema, parentPath)
	if changeMetaTarget != nil {
		if err := changeMetaTarget.RenameListEntry(listName, oldKey, newKey); err != nil {
			return err
		}
	}

	changeOps = append(changeOps, proposedOp)
	changeOps = config.CoalesceRenameOps(changeOps)

	output := config.SerializeChangeFile(changeTree, changeMeta, changeOps, e.schema)
	if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
	}

	var target *config.Tree
	if len(parentPath) == 0 {
		target = e.tree
	} else {
		target = walkPath(e.tree, e.schema, parentPath)
	}
	if target == nil {
		return fmt.Errorf("path not found")
	}
	if err := target.RenameListEntry(listName, oldKey, newKey); err != nil {
		return err
	}

	metaTarget := walkMetaReadOnly(e.meta, e.schema, parentPath)
	if metaTarget != nil {
		if err := metaTarget.RenameListEntry(listName, oldKey, newKey); err != nil {
			return err
		}
	}

	e.dirty.Store(true)
	e.draftSaved = false
	return nil
}

// readChangeFile reads and parses a per-user change file.
// Returns empty tree/meta/op collections if the file does not exist or is corrupt.
func (e *Editor) readChangeFile(guard storage.WriteGuard, changePath string) (*config.Tree, *config.MetaTree, []config.StructuralOp) {
	data, readErr := guard.ReadFile(changePath)
	if readErr != nil {
		// No change file: start with empty sparse tree.
		return config.NewTree(), config.NewMetaTree(), nil
	}
	parser := config.NewSetParser(e.schema)
	tree, meta, ops, parseErr := config.ParseChangeFile(string(data), parser)
	if parseErr != nil {
		// Corrupt change file (e.g., from a previous bug). Log and start fresh
		// rather than blocking all future edits.
		draftLogger.Warn("discarding corrupt change file", "path", changePath, "error", parseErr)
		return config.NewTree(), config.NewMetaTree(), nil
	}
	return tree, meta, ops
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
	_, changeMeta, changeOps := e.readChangeFile(guard, changePath)

	myEntries := changeMeta.SessionEntries(e.session.ID)
	myOps := filterStructuralOps(changeOps, e.session.ID)
	if len(myEntries) == 0 && len(myOps) == 0 {
		return nil // Nothing to save.
	}

	// Read base (draft if exists, else committed).
	draftPath := DraftPath(e.originalPath)
	baseTree, baseMeta := e.readDraftOrConfig(guard, draftPath)

	if err := applyStructuralOps(baseTree, e.schema, myOps, true); err != nil {
		return fmt.Errorf("save apply rename: %w", err)
	}
	if err := applyStructuralOpsToMeta(baseMeta, e.schema, myOps, true); err != nil {
		return fmt.Errorf("save apply rename meta: %w", err)
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

	if len(myOps) > 0 {
		output := config.SerializeChangeFile(config.NewTree(), config.NewMetaTree(), myOps, e.schema)
		if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
			return fmt.Errorf("save preserve rename ops: %w", err)
		}
	} else if err := guard.Remove(changePath); err != nil {
		return fmt.Errorf("save remove change file: %w", err)
	}

	// Update in-memory state to draft.
	e.tree = baseTree
	e.meta = baseMeta
	e.draftMtime = time.Now()
	e.draftSaved = true
	return nil
}

func renameTreeListEntry(target *config.Tree, listName, oldKey, newKey string) error {
	entries := target.GetList(listName)
	if entries == nil || entries[oldKey] == nil {
		return nil
	}
	return target.RenameListEntry(listName, oldKey, newKey)
}

func filterStructuralOps(ops []config.StructuralOp, sessionID string) []config.StructuralOp {
	if sessionID == "" {
		return append([]config.StructuralOp(nil), ops...)
	}
	filtered := make([]config.StructuralOp, 0, len(ops))
	for i := range ops {
		if ops[i].SessionKey() == sessionID {
			filtered = append(filtered, ops[i])
		}
	}
	return filtered
}

func applyStructuralOps(tree *config.Tree, schema *config.Schema, ops []config.StructuralOp, allowAlreadyApplied bool) error {
	for i := range ops {
		if ops[i].Type != config.StructuralOpRename {
			return fmt.Errorf("unsupported structural op %q", ops[i].Type)
		}
		parentPath := strings.Fields(ops[i].ParentPath)
		target := walkPath(tree, schema, parentPath)
		if target == nil {
			return fmt.Errorf("path not found: %s", ops[i].ParentPath)
		}
		if err := target.RenameListEntry(ops[i].ListName, ops[i].OldKey, ops[i].NewKey); err != nil {
			if allowAlreadyApplied && renameAlreadyApplied(target, ops[i].ListName, ops[i].OldKey, ops[i].NewKey) {
				continue
			}
			return err
		}
	}
	return nil
}

func applyStructuralOpsToMeta(meta *config.MetaTree, schema *config.Schema, ops []config.StructuralOp, allowAlreadyApplied bool) error {
	if meta == nil {
		return nil
	}
	for i := range ops {
		if ops[i].Type != config.StructuralOpRename {
			return fmt.Errorf("unsupported structural op %q", ops[i].Type)
		}
		parentPath := strings.Fields(ops[i].ParentPath)
		target := walkMetaReadOnly(meta, schema, parentPath)
		if target == nil {
			continue
		}
		if err := target.RenameListEntry(ops[i].ListName, ops[i].OldKey, ops[i].NewKey); err != nil {
			if allowAlreadyApplied && renameMetaAlreadyApplied(target, ops[i].ListName, ops[i].OldKey, ops[i].NewKey) {
				continue
			}
			return err
		}
	}
	return nil
}

func renameAlreadyApplied(target *config.Tree, listName, oldKey, newKey string) bool {
	entries := target.GetList(listName)
	if entries == nil {
		return false
	}
	if entries[oldKey] != nil {
		return false
	}
	return entries[newKey] != nil
}

func renameMetaAlreadyApplied(target *config.MetaTree, listName, oldKey, newKey string) bool {
	listMeta := target.GetContainer(listName)
	if listMeta == nil {
		return false
	}
	if listMeta.GetListEntry(oldKey) != nil {
		return false
	}
	return listMeta.GetListEntry(newKey) != nil
}

// LoadDraft reads the draft file and loads its content into the editor's in-memory tree.
// Called on startup to restore previously saved work. Returns false if no draft exists.
func (e *Editor) LoadDraft() bool {
	draftPath := DraftPath(e.originalPath)
	data, err := e.store.ReadFile(draftPath)
	if err != nil {
		return false
	}
	parser := config.NewSetParser(e.schema)
	tree, meta, err := parser.ParseWithMeta(string(data))
	if err != nil {
		return false
	}
	e.tree = tree
	e.meta = meta
	e.treeValid = true
	e.draftSaved = true

	// Set draftMtime so CheckDraftChanged doesn't false-trigger.
	if fi, statErr := e.store.Stat(draftPath); statErr == nil {
		e.draftMtime = fi.ModTime
	}
	return true
}

// DetectConflicts scans pending changes from other sessions and reports live overlaps.
func (e *Editor) DetectConflicts() []Conflict {
	if e.session == nil || e.meta == nil {
		return nil
	}
	myChanges := e.PendingChanges(e.session.ID)
	if len(myChanges) == 0 {
		return nil
	}

	var conflicts []Conflict
	for _, sid := range e.ActiveSessions() {
		if sid == e.session.ID {
			continue
		}
		otherUser, _, _ := strings.Cut(sid, "@")
		for _, mine := range myChanges {
			for _, other := range e.PendingChanges(sid) {
				if !pendingChangesConflict(mine, other) {
					continue
				}
				conflicts = append(conflicts, Conflict{
					Path:       conflictPath(mine, other),
					Type:       ConflictLive,
					MyValue:    pendingConflictValue(mine),
					OtherValue: pendingConflictValue(other),
					OtherUser:  otherUser,
				})
			}
		}
	}

	return conflicts
}

func pendingChangesConflict(a, b config.PendingChange) bool {
	if a.Kind != config.PendingChangeRename && b.Kind != config.PendingChangeRename {
		return a.Path == b.Path && a.Value != b.Value
	}
	for _, aPath := range a.ConflictPaths() {
		for _, bPath := range b.ConflictPaths() {
			if pathOverlaps(aPath, bPath) {
				return true
			}
		}
	}
	return false
}

func (e *Editor) validateRenameLiveConflict(guard storage.WriteGuard, proposed config.PendingChange) error {
	if e.session == nil {
		return nil
	}
	seen := make(map[string]bool)
	for _, other := range e.PendingChanges("") {
		if other.SessionID == "" || other.SessionID == e.session.ID {
			continue
		}
		seen[pendingChangeKey(other)] = true
		if !pendingChangesConflict(proposed, other) {
			continue
		}
		return fmt.Errorf("pending change conflict with %s at %s", other.SessionID, conflictPath(proposed, other))
	}

	draftData, err := guard.ReadFile(DraftPath(e.originalPath))
	if err != nil {
		return nil //nolint:nilerr // no draft file means no conflict to detect
	}
	_, draftMeta, parseErr := config.NewSetParser(e.schema).ParseWithMeta(string(draftData))
	if parseErr != nil {
		return nil //nolint:nilerr // unparseable draft means no conflict to detect
	}
	for _, sid := range draftMeta.AllSessions() {
		if sid == e.session.ID {
			continue
		}
		for _, entry := range draftMeta.SessionEntries(sid) {
			other := config.PendingChangeFromSessionEntry(entry)
			if seen[pendingChangeKey(other)] {
				continue
			}
			if !pendingChangesConflict(proposed, other) {
				continue
			}
			return fmt.Errorf("pending change conflict with %s at %s", sid, conflictPath(proposed, other))
		}
	}
	return nil
}

func conflictPath(a, b config.PendingChange) string {
	for _, aPath := range a.ConflictPaths() {
		for _, bPath := range b.ConflictPaths() {
			if pathOverlaps(aPath, bPath) {
				if len(aPath) >= len(bPath) {
					return aPath
				}
				return bPath
			}
		}
	}
	if a.Path != "" {
		return a.Path
	}
	return a.NewPath
}

func pendingConflictValue(change config.PendingChange) string {
	if change.Kind == config.PendingChangeRename {
		return change.Summary()
	}
	return change.Value
}

func pathOverlaps(a, b string) bool {
	return pathHasPrefix(a, b) || pathHasPrefix(b, a)
}

func pathHasPrefix(path, prefix string) bool {
	if path == prefix {
		return true
	}
	if prefix == "" || path == "" {
		return false
	}
	return strings.HasPrefix(path, prefix+" ")
}

// readDraftOrConfig reads and parses the draft file, falling back to config.conf.
// Returns the parsed tree and metadata. Uses guard for I/O (called within locked sections).
// If the draft exists but cannot be parsed (corrupt, outdated schema), falls back to
// the committed config so that save is never blocked by a bad draft.
func (e *Editor) readDraftOrConfig(guard storage.WriteGuard, draftPath string) (*config.Tree, *config.MetaTree) {
	parser := config.NewSetParser(e.schema)

	data, err := guard.ReadFile(draftPath)
	if err == nil {
		tree, meta, parseErr := parser.ParseWithMeta(string(data))
		if parseErr == nil {
			return tree, meta
		}
		// Draft exists but cannot be parsed (corrupt or schema mismatch).
		// Fall through to committed config so save is not blocked.
	}

	// No draft or unparseable draft: clone the in-memory tree and start with empty metadata.
	return e.tree.Clone(), config.NewMetaTree()
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
	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps := e.readChangeFile(guard, changePath)
	oldOps := filterStructuralOps(changeOps, oldSessionID)
	if len(oldEntries) == 0 && len(oldOps) == 0 {
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

	rewroteOps := false
	for i := range changeOps {
		if changeOps[i].SessionKey() != oldSessionID {
			continue
		}
		changeOps[i].User = e.session.User
		changeOps[i].Source = e.session.Origin
		changeOps[i].Time = e.session.StartTime
		rewroteOps = true
	}
	changeOps = config.CoalesceRenameOps(changeOps)

	// Serialize and write updated draft.
	output := config.SerializeSetWithMeta(tree, meta, e.schema)
	if err := guard.WriteFile(draftPath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write draft: %w", err)
	}
	changeOutput := config.SerializeChangeFile(changeTree, changeMeta, changeOps, e.schema)
	if rewroteOps || strings.TrimSpace(changeOutput) != "" {
		if err := guard.WriteFile(changePath, []byte(changeOutput), 0o600); err != nil {
			return fmt.Errorf("write change file: %w", err)
		}
	} else {
		_ = guard.Remove(changePath)
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
