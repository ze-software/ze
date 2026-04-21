// Design: docs/architecture/config/yang-config-design.md — per-session commit, discard, disconnect
// Overview: editor_draft.go — write-through draft protocol
// Related: editor_walk.go — schema-aware tree/meta walking used by commit/discard

package cli

import (
	"fmt"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/migration"
)

// CommitSession commits the current session's changes to config.conf.
// First saves (change file → draft), then applies draft to config.conf.
// Returns a CommitResult with conflicts (if any) or the number of applied changes.
//
//nolint:cyclop // commit protocol has inherently many steps
func (e *Editor) CommitSession() (*CommitResult, error) {
	if e.session == nil {
		return nil, fmt.Errorf("no session set")
	}

	// Check for live conflicts before saving (scanning change files).
	if liveConflicts := e.DetectConflicts(); len(liveConflicts) > 0 {
		return &CommitResult{Conflicts: liveConflicts}, nil
	}

	// Save: apply change file → draft.
	if err := e.SaveDraft(); err != nil {
		return nil, fmt.Errorf("commit save: %w", err)
	}

	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return nil, fmt.Errorf("commit lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths

	// Read and parse config.conf (committed).
	committedData, err := guard.ReadFile(e.originalPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	committedContent := string(committedData)
	committedTree, existingMeta, err := parseConfigWithFormat(committedContent, e.schema)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Apply tree structure migrations if the committed config is hierarchical.
	var migrationWarning string
	if config.DetectFormat(committedContent) == config.FormatHierarchical {
		if migration.NeedsMigration(committedTree) {
			result, migrateErr := migration.Migrate(committedTree)
			if migrateErr == nil {
				committedTree = result.Tree
			} else {
				migrationWarning = fmt.Sprintf("tree migration skipped: %v", migrateErr)
			}
		}
	}

	// Read and parse draft (created by SaveDraft above).
	draftPath := DraftPath(e.originalPath)
	draftData, err := guard.ReadFile(draftPath)
	if err != nil {
		// No draft means SaveDraft had nothing to save.
		return &CommitResult{Applied: 0}, nil
	}
	setParser := config.NewSetParser(e.schema)
	draftTree, draftMeta, err := setParser.ParseWithMeta(string(draftData))
	if err != nil {
		return nil, fmt.Errorf("parse draft: %w", err)
	}

	changePath := ChangePath(e.originalPath, e.session.User)
	_, _, changeOps := e.readChangeFile(guard, changePath)

	// Find my changes from the draft metadata and preserved structural ops.
	myEntries := draftMeta.SessionEntries(e.session.ID)
	myOps := filterStructuralOps(changeOps, e.session.ID)
	if len(myEntries) == 0 && len(myOps) == 0 {
		return &CommitResult{Applied: 0}, nil
	}

	// Check for stale conflicts (committed changed since editing).
	var conflicts []Conflict
	for i := range myOps {
		if conflict := renameStaleConflict(committedTree, e.schema, myOps[i]); conflict != nil {
			conflicts = append(conflicts, *conflict)
		}
	}
	for _, se := range myEntries {
		pathParts := strings.Fields(se.Path)
		myValue := se.Entry.Value

		committedValue := getValueAtPath(committedTree, e.schema, pathParts)
		isStale := myValue != committedValue &&
			((se.Entry.Previous != "" && committedValue != se.Entry.Previous) ||
				(se.Entry.Previous == "" && committedValue != ""))
		if isStale {
			conflicts = append(conflicts, Conflict{
				Path:          se.Path,
				Type:          ConflictStale,
				MyValue:       myValue,
				OtherValue:    committedValue,
				PreviousValue: se.Entry.Previous,
			})
		}
	}

	if len(conflicts) > 0 {
		return &CommitResult{Conflicts: conflicts}, nil
	}

	// No conflicts: apply my changes to committed tree.
	if err := applyStructuralOps(committedTree, e.schema, myOps, false); err != nil {
		return nil, fmt.Errorf("apply rename: %w", err)
	}
	applied := len(myOps)
	for _, se := range myEntries {
		pathParts := strings.Fields(se.Path)
		value := se.Entry.Value
		if len(pathParts) > 0 {
			leafName := pathParts[len(pathParts)-1]
			parentPath := pathParts[:len(pathParts)-1]
			target, walkErr := e.walkOrCreateIn(committedTree, parentPath)
			if walkErr != nil {
				continue
			}
			if value != "" {
				target.Set(leafName, value)
			} else {
				target.Delete(leafName)
			}
			applied++
		}
	}

	// Create backup before overwriting config.conf.
	if err := e.createBackup(string(committedData), guard); err != nil {
		return nil, fmt.Errorf("backup: %w", err)
	}

	// Hash any plaintext-password siblings of ze:bcrypt leaves into their
	// canonical form and remove the plaintext. Junos-style one-way commit
	// so the serialized config never carries plaintext. Drop the matching
	// SessionEntries so commit metadata does not record orphan annotations
	// for a leaf that no longer exists in the tree.
	if err := config.ApplyPasswordHashing(committedTree, e.schema); err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	myEntries = dropPlaintextPasswordEntries(myEntries)

	// Write committed tree to config.conf.
	now := time.Now()
	commitMeta := buildCommitMeta(existingMeta, draftMeta, myEntries, myOps, e.session.User, now, e.schema)
	committedOutput := config.SerializeSetWithMeta(committedTree, commitMeta, e.schema)
	if err := guard.WriteFile(e.originalPath, []byte(committedOutput), 0o600); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	// Remove my session from draft and clean up.
	draftMeta.RemoveSession(e.session.ID)
	remainingSessions := draftMeta.AllSessions()
	if len(remainingSessions) == 0 {
		guard.Remove(draftPath) //nolint:errcheck // Best effort
	} else {
		// Rewrite draft without my entries so other sessions' metadata stays current.
		draftOutput := config.SerializeSetWithMeta(draftTree, draftMeta, e.schema)
		if err := guard.WriteFile(draftPath, []byte(draftOutput), 0o600); err != nil {
			return nil, fmt.Errorf("write draft: %w", err)
		}
	}

	// Also clean up the per-user change file now that structural ops are committed.
	guard.Remove(changePath) //nolint:errcheck // Best effort

	// Update in-memory state.
	e.originalContent = committedOutput
	e.tree = committedTree
	e.meta = commitMeta
	e.dirty.Store(false)

	return &CommitResult{Applied: applied, MigrationWarning: migrationWarning}, nil
}

// DiscardSessionPath discards this session's changes at the given path.
// If path is nil, discards all changes (deletes change file, reloads from base).
func (e *Editor) DiscardSessionPath(path []string) error {
	if e.session == nil {
		return fmt.Errorf("no session set")
	}

	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("discard lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths

	changePath := ChangePath(e.originalPath, e.session.User)
	pathPrefix := strings.Join(path, " ")

	if pathPrefix == "" {
		// Discard all: delete change file entirely.
		guard.Remove(changePath) //nolint:errcheck // Best effort
	} else {
		// Partial discard: remove matching entries from change file.
		changeTree, changeMeta, changeOps := e.readChangeFile(guard, changePath)

		myEntries := changeMeta.SessionEntries(e.session.ID)
		for _, se := range myEntries {
			if se.Path != pathPrefix && !strings.HasPrefix(se.Path, pathPrefix+" ") {
				continue
			}
			pathParts := strings.Fields(se.Path)
			if len(pathParts) == 0 {
				continue
			}
			leafName := pathParts[len(pathParts)-1]
			parentPath := pathParts[:len(pathParts)-1]
			metaTarget := walkOrCreateMeta(changeMeta, e.schema, parentPath)
			metaTarget.RemoveSessionEntry(leafName, e.session.ID)
			// Also remove the value from the change tree so it is not serialized back.
			if treeTarget := walkPath(changeTree, e.schema, parentPath); treeTarget != nil {
				treeTarget.Delete(leafName)
			}
		}
		filteredOps := changeOps[:0]
		for i := range changeOps {
			if changeOps[i].SessionKey() != e.session.ID || !renameMatchesPath(changeOps[i], pathPrefix) {
				filteredOps = append(filteredOps, changeOps[i])
			}
		}
		changeOps = filteredOps

		// Check if any sessions have remaining entries (not just this session —
		// ChangePath is per-user, so multiple sessions from the same user share a file).
		if len(changeMeta.AllSessions()) == 0 && len(changeOps) == 0 {
			guard.Remove(changePath) //nolint:errcheck // Best effort
		} else {
			output := config.SerializeChangeFile(changeTree, changeMeta, changeOps, e.schema)
			if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
				return fmt.Errorf("discard write change: %w", err)
			}
		}
	}

	// Reload in-memory state from base (draft if exists, else committed config).
	// Cannot use readDraftOrConfig here because its fallback clones e.tree which
	// still has the user's changes. Read committed config directly instead.
	draftPath := DraftPath(e.originalPath)
	var baseTree *config.Tree
	var baseMeta *config.MetaTree
	if draftData, draftErr := guard.ReadFile(draftPath); draftErr == nil {
		baseTree, baseMeta, err = config.NewSetParser(e.schema).ParseWithMeta(string(draftData))
		if err != nil {
			return fmt.Errorf("discard parse draft: %w", err)
		}
	} else {
		committedData, readErr := guard.ReadFile(e.originalPath)
		if readErr != nil {
			return fmt.Errorf("discard read config: %w", readErr)
		}
		baseTree, baseMeta, err = parseConfigWithFormat(string(committedData), e.schema)
		if err != nil {
			return fmt.Errorf("discard parse config: %w", err)
		}
	}

	// Re-apply remaining changes from change file (if partial discard).
	if pathPrefix != "" {
		_, changeMeta, changeOps := e.readChangeFile(guard, changePath)
		if err := applyStructuralOps(baseTree, e.schema, changeOps, true); err != nil {
			return fmt.Errorf("discard apply rename: %w", err)
		}
		if err := applyStructuralOpsToMeta(baseMeta, e.schema, changeOps, true); err != nil {
			return fmt.Errorf("discard apply rename meta: %w", err)
		}
		for _, sid := range changeMeta.AllSessions() {
			for _, se := range changeMeta.SessionEntries(sid) {
				pathParts := strings.Fields(se.Path)
				if len(pathParts) == 0 {
					continue
				}
				leafName := pathParts[len(pathParts)-1]
				parentPath := pathParts[:len(pathParts)-1]
				if se.Entry.Value != "" {
					target, walkErr := e.walkOrCreateIn(baseTree, parentPath)
					if walkErr == nil {
						target.Set(leafName, se.Entry.Value)
					}
				} else {
					// Delete operation: remove leaf from base tree.
					if target := walkPath(baseTree, e.schema, parentPath); target != nil {
						target.Delete(leafName)
					}
				}
				metaTarget := walkOrCreateMeta(baseMeta, e.schema, parentPath)
				metaTarget.SetEntry(leafName, se.Entry)
			}
		}
	}

	// Refresh originalContent from disk to capture external commits.
	committedData, readErr := guard.ReadFile(e.originalPath)
	if readErr != nil {
		return fmt.Errorf("discard read config: %w", readErr)
	}
	e.originalContent = string(committedData)

	e.tree = baseTree
	e.meta = baseMeta
	// Dirty if change file still has entries (partial discard).
	e.dirty.Store(e.store.Exists(changePath))
	return nil
}

func renameStaleConflict(committedTree *config.Tree, schema *config.Schema, op config.StructuralOp) *Conflict {
	parentPath := strings.Fields(op.ParentPath)
	target := walkPath(committedTree, schema, parentPath)
	if target == nil {
		return &Conflict{
			Path:       op.SourcePath(),
			Type:       ConflictStale,
			MyValue:    op.PendingChange().Summary(),
			OtherValue: "rename source path missing",
		}
	}
	entries := target.GetList(op.ListName)
	if entries == nil || entries[op.OldKey] == nil {
		return &Conflict{
			Path:       op.SourcePath(),
			Type:       ConflictStale,
			MyValue:    op.PendingChange().Summary(),
			OtherValue: "rename source missing",
		}
	}
	if entries[op.NewKey] != nil {
		return &Conflict{
			Path:       op.DestinationPath(),
			Type:       ConflictStale,
			MyValue:    op.PendingChange().Summary(),
			OtherValue: "rename destination already exists",
		}
	}
	return nil
}

func renameMatchesPath(op config.StructuralOp, pathPrefix string) bool {
	return pathOverlaps(op.SourcePath(), pathPrefix) || pathOverlaps(op.DestinationPath(), pathPrefix)
}

// DisconnectSession removes another user's change file.
// In the per-user change file model, each user has their own file,
// so disconnect simply deletes the other user's change file.
func (e *Editor) DisconnectSession(sessionID string) error {
	if e.session == nil {
		return fmt.Errorf("no session set")
	}

	// Extract user from session ID (format: "user@origin%time").
	otherUser, _, _ := strings.Cut(sessionID, "@")

	changePath := ChangePath(e.originalPath, otherUser)
	_ = e.store.Remove(changePath) // Best effort — file may not exist.
	return nil
}

// checkLiveConflicts checks if other sessions have pending changes at the same YANG path.
// myValue is this session's Entry.Value (empty string = delete intent).
// Other sessions' values are also read from Entry.Value, not the draft tree.
func checkLiveConflicts(meta *config.MetaTree, mySessionID, yangPath string, pathParts []string, myValue string, schema *config.Schema) []Conflict {
	if len(pathParts) == 0 {
		return nil
	}
	leafName := pathParts[len(pathParts)-1]
	parentPath := pathParts[:len(pathParts)-1]

	// Walk MetaTree to the leaf's parent using schema-aware navigation
	// so list entries are found in .lists (not .containers).
	metaTarget := walkMetaReadOnly(meta, schema, parentPath)
	if metaTarget == nil {
		return nil
	}

	entries := metaTarget.GetAllEntries(leafName)
	if len(entries) == 0 {
		return nil
	}

	// Check all entries for other-session disagreements.
	// Both myValue and otherValue come from Entry.Value (session intent).
	// Empty Value with non-empty SessionKey = delete intent.
	var conflicts []Conflict
	for _, entry := range entries {
		if entry.SessionKey() != mySessionID && entry.SessionKey() != "" {
			otherValue := entry.Value
			if otherValue != myValue {
				conflicts = append(conflicts, Conflict{
					Path:       yangPath,
					Type:       ConflictLive,
					MyValue:    myValue,
					OtherValue: otherValue,
					OtherUser:  entry.User,
				})
			}
		}
	}

	return conflicts
}

// plaintextPasswordLeafPrefix is the leaf-name prefix used by the Junos-style
// auto-hash convention; entries targeting these leaves are dropped from the
// commit metadata after ApplyPasswordHashing removes the leaves from the tree.
// Mirror of password_hash.go's plaintextPrefix.
const plaintextPasswordLeafPrefix = "plaintext-"

// dropPlaintextPasswordEntries filters out SessionEntry records whose final
// path segment starts with "plaintext-". Called after ApplyPasswordHashing so
// commit metadata does not annotate a leaf that no longer exists in the tree.
//
// Returns a new slice; the input is not mutated. Callers should expect the
// length to drop by one for each plaintext-* entry; the relative order of
// the remaining entries is preserved.
//
// The "plaintext-" prefix matches the convention enforced by
// config.ApplyPasswordHashing -- a future YANG schema author who names a
// non-bcrypt-companion leaf "plaintext-foo" would have it dropped from
// commit metadata here. No such leaf exists today.
func dropPlaintextPasswordEntries(entries []config.SessionEntry) []config.SessionEntry {
	out := make([]config.SessionEntry, 0, len(entries))
	for _, se := range entries {
		parts := strings.Fields(se.Path)
		if len(parts) > 0 && strings.HasPrefix(parts[len(parts)-1], plaintextPasswordLeafPrefix) {
			continue
		}
		out = append(out, se)
	}
	return out
}

// buildCommitMeta creates metadata for the committed config.conf.
// Starts from existing committed metadata (preserving prior commit annotations),
// overlays with the committer's entries, and copies non-session metadata from draft.
func buildCommitMeta(existingMeta, draftMeta *config.MetaTree, myEntries []config.SessionEntry, myOps []config.StructuralOp, user string, commitTime time.Time, schema *config.Schema) *config.MetaTree {
	// Start from existing committed metadata to preserve prior annotations.
	commitMeta := config.NewMetaTree()
	if existingMeta != nil {
		commitMeta = existingMeta.Clone()
	}
	_ = applyStructuralOpsToMeta(commitMeta, schema, myOps, true)

	// For each committed entry, record user and time (no session, no previous).
	// This overwrites any prior metadata for leaves this session changed.
	// For deletes (Value=""), remove any existing metadata instead of creating
	// a tombstone -- deleted leaves don't need metadata in the committed config.
	for _, se := range myEntries {
		pathParts := strings.Fields(se.Path)
		if len(pathParts) == 0 {
			continue
		}
		leafName := pathParts[len(pathParts)-1]
		parentPath := pathParts[:len(pathParts)-1]
		target := walkOrCreateMeta(commitMeta, schema, parentPath)
		if se.Entry.Value == "" {
			// Delete: remove all metadata for this leaf (not a tombstone).
			target.RemoveEntry(leafName)
		} else {
			target.SetEntry(leafName, config.MetaEntry{
				User: user,
				Time: commitTime,
			})
		}
	}

	// Copy metadata from draft for non-session entries (hand-written lines).
	copyNonSessionMeta(commitMeta, draftMeta)

	return commitMeta
}

// copyNonSessionMeta copies entries without session IDs from src to dst.
// Uses AllEntries to iterate all entries per leaf (not just the last),
// preserving any hand-written metadata that might have multiple entries.
func copyNonSessionMeta(dst, src *config.MetaTree) {
	if src == nil {
		return
	}

	for name, entries := range src.AllEntries() {
		for _, entry := range entries {
			if entry.Source == "" {
				if _, exists := dst.GetEntry(name); !exists {
					dst.SetEntry(name, entry)
				}
			}
		}
	}

	for name, child := range src.Containers() {
		dstChild := dst.GetOrCreateContainer(name)
		copyNonSessionMeta(dstChild, child)
	}

	for key, child := range src.Lists() {
		dstChild := dst.GetOrCreateListEntry(key)
		copyNonSessionMeta(dstChild, child)
	}
}
