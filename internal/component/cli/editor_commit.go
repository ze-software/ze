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
// Returns a CommitResult with conflicts (if any) or the number of applied changes.
//
//nolint:cyclop // commit protocol has inherently many steps
func (e *Editor) CommitSession() (*CommitResult, error) {
	if e.session == nil {
		return nil, fmt.Errorf("no session set")
	}

	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return nil, fmt.Errorf("commit lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths

	// Read and parse config.conf (committed), detecting format to handle
	// both hierarchical and set+meta formats (config.conf becomes set+meta
	// after the first session commit).
	committedData, err := guard.ReadFile(e.originalPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	committedContent := string(committedData)
	committedTree, existingMeta, err := parseConfigWithFormat(committedContent, e.schema)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Apply tree structure migrations if the committed config is hierarchical
	// (e.g., neighbor->peer renaming). This runs once on the first session commit
	// of a legacy config; subsequent commits read set+meta and skip this.
	var migrationWarning string
	if config.DetectFormat(committedContent) == config.FormatHierarchical {
		if migration.NeedsMigration(committedTree) {
			result, migrateErr := migration.Migrate(committedTree)
			if migrateErr == nil {
				committedTree = result.Tree
			} else {
				// Format conversion still happens (set+meta output),
				// but tree transforms were skipped. Surface the warning.
				migrationWarning = fmt.Sprintf("tree migration skipped: %v", migrateErr)
			}
		}
	}

	// Read and parse config.draft.
	draftPath := DraftPath(e.originalPath)
	draftData, err := guard.ReadFile(draftPath)
	if err != nil {
		return nil, fmt.Errorf("read draft: %w", err)
	}
	setParser := config.NewSetParser(e.schema)
	draftTree, draftMeta, err := setParser.ParseWithMeta(string(draftData))
	if err != nil {
		return nil, fmt.Errorf("parse draft: %w", err)
	}

	// Find my changes.
	myEntries := draftMeta.SessionEntries(e.session.ID)
	if len(myEntries) == 0 {
		return &CommitResult{Applied: 0}, nil
	}

	// Check for conflicts.
	var conflicts []Conflict
	for _, se := range myEntries {
		pathParts := strings.Fields(se.Path)

		// Use session metadata for this session's intent, not the draft tree.
		// The draft tree holds the merged state of all sessions (last writer wins),
		// which may not reflect this session's actual intent (e.g., if another
		// session set a value after this session deleted it).
		myValue := se.Entry.Value

		// Check stale Previous: compare config.conf current value with recorded Previous.
		// Three checks:
		// 1. Convergent agreement: if committed already matches my intent, no conflict
		//    (handles concurrent delete: both sessions deleted the same value).
		// 2. Previous != "" and committed changed: the value I edited was modified by another commit.
		// 3. Previous == "" and committed != "": I added a new value, but someone else also
		//    added and committed a value at the same path -- stale conflict.
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

		// Check live disagreement: another session has a pending change at same path.
		conflicts = append(conflicts, checkLiveConflicts(draftMeta, e.session.ID, se.Path, pathParts, myValue, e.schema)...)
	}

	if len(conflicts) > 0 {
		return &CommitResult{Conflicts: conflicts}, nil
	}

	// No conflicts: apply my changes to committed tree.
	// Use session metadata (Entry.Value) for each change, not the draft tree.
	// Empty Value with a session ID means delete.
	applied := 0
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

	// Create backup before overwriting config.conf. Use freshly-read data
	// (not cached originalContent) in case another session committed since
	// this editor was created.
	if err := e.createBackup(string(committedData), guard); err != nil {
		return nil, fmt.Errorf("backup: %w", err)
	}

	// Write committed tree to config.conf (with user/time metadata, no session).
	now := time.Now()
	commitMeta := buildCommitMeta(existingMeta, draftMeta, myEntries, e.session.User, now, e.schema)
	committedOutput := config.SerializeSetWithMeta(committedTree, commitMeta, e.schema)
	if err := guard.WriteFile(e.originalPath, []byte(committedOutput), 0o600); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	// Remove my session from draft.
	draftMeta.RemoveSession(e.session.ID)

	// Check if other sessions remain.
	remainingSessions := draftMeta.AllSessions()
	if len(remainingSessions) == 0 {
		// No more pending changes: delete draft.
		guard.Remove(draftPath) //nolint:errcheck // Best effort
	} else {
		// Regenerate draft without my entries.
		draftOutput := config.SerializeSetWithMeta(draftTree, draftMeta, e.schema)
		if err := guard.WriteFile(draftPath, []byte(draftOutput), 0o600); err != nil {
			return nil, fmt.Errorf("write draft: %w", err)
		}
	}

	// Update in-memory state: originalContent always tracks config.conf.
	e.originalContent = committedOutput
	if len(remainingSessions) == 0 {
		// No pending changes: show committed state.
		e.tree = committedTree
		e.meta = commitMeta
	} else {
		// Other sessions have pending changes: show draft state so
		// show/blame/changes commands are consistent (tree and meta
		// both reflect the draft, not a mix of committed + draft).
		e.tree = draftTree
		e.meta = draftMeta
	}
	e.dirty.Store(false)

	return &CommitResult{Applied: applied, MigrationWarning: migrationWarning}, nil
}

// DiscardSessionPath discards this session's changes at the given path.
// If path is nil, discards all changes for this session.
func (e *Editor) DiscardSessionPath(path []string) error {
	if e.session == nil {
		return fmt.Errorf("no session set")
	}

	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("discard lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths

	draftPath := DraftPath(e.originalPath)
	draftData, err := guard.ReadFile(draftPath)
	if err != nil {
		return fmt.Errorf("read draft: %w", err)
	}
	setParser := config.NewSetParser(e.schema)
	draftTree, draftMeta, err := setParser.ParseWithMeta(string(draftData))
	if err != nil {
		return fmt.Errorf("parse draft: %w", err)
	}

	// Read committed config under lock (not cached originalContent) to capture
	// external commits since the editor was created. Spec: "restore committed
	// value (from config.conf)".
	committedData, readErr := guard.ReadFile(e.originalPath)
	if readErr != nil {
		return fmt.Errorf("read config: %w", readErr)
	}
	committedTree, _, err := parseConfigWithFormat(string(committedData), e.schema)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// Find my entries to discard.
	myEntries := draftMeta.SessionEntries(e.session.ID)
	pathPrefix := strings.Join(path, " ")

	for _, se := range myEntries {
		// Filter by path prefix (empty prefix = discard all).
		// Use prefix + space boundary to avoid "bgp peer" matching "bgp peer-group".
		if pathPrefix != "" && se.Path != pathPrefix && !strings.HasPrefix(se.Path, pathPrefix+" ") {
			continue
		}

		pathParts := strings.Fields(se.Path)
		if len(pathParts) == 0 {
			continue
		}
		leafName := pathParts[len(pathParts)-1]
		parentPath := pathParts[:len(pathParts)-1]

		// Remove this session's metadata entry (preserves other sessions' entries).
		metaTarget := walkOrCreateMeta(draftMeta, e.schema, parentPath)
		metaTarget.RemoveSessionEntry(leafName, e.session.ID)

		// If another session still has a pending value at this leaf,
		// restore that session's value (not the committed value).
		target, walkErr := e.walkOrCreateIn(draftTree, parentPath)
		if walkErr != nil {
			continue
		}
		remaining := metaTarget.GetAllEntries(leafName)
		if len(remaining) > 0 && remaining[len(remaining)-1].Value != "" {
			target.Set(leafName, remaining[len(remaining)-1].Value)
		} else {
			// No other session has a pending value: restore committed value.
			committedValue := getValueAtPath(committedTree, e.schema, pathParts)
			if committedValue != "" {
				target.Set(leafName, committedValue)
			} else {
				target.Delete(leafName)
			}
		}
	}

	// Remove my session from draft metadata.
	if pathPrefix == "" {
		draftMeta.RemoveSession(e.session.ID)
	}

	// Check if other sessions remain.
	remainingSessions := draftMeta.AllSessions()
	if len(remainingSessions) == 0 {
		guard.Remove(draftPath) //nolint:errcheck // Best effort
	} else {
		draftOutput := config.SerializeSetWithMeta(draftTree, draftMeta, e.schema)
		if err := guard.WriteFile(draftPath, []byte(draftOutput), 0o600); err != nil {
			return fmt.Errorf("write draft: %w", err)
		}
	}

	// Update in-memory state. Refresh originalContent from disk to capture
	// external commits since this editor was created (prevents stale backup
	// content in subsequent operations).
	e.tree = draftTree
	e.meta = draftMeta
	e.originalContent = string(committedData)
	// Mark dirty only if this session still has pending entries (partial discard
	// keeps other entries alive; discard-all clears everything).
	e.dirty.Store(len(draftMeta.SessionEntries(e.session.ID)) > 0)
	return nil
}

// DisconnectSession removes all entries for another session from the draft.
// Restores committed values where the disconnected session was the only editor.
func (e *Editor) DisconnectSession(sessionID string) error {
	if e.session == nil {
		return fmt.Errorf("no session set")
	}

	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("disconnect lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths

	draftPath := DraftPath(e.originalPath)
	draftData, err := guard.ReadFile(draftPath)
	if err != nil {
		return fmt.Errorf("read draft: %w", err)
	}
	setParser := config.NewSetParser(e.schema)
	draftTree, draftMeta, err := setParser.ParseWithMeta(string(draftData))
	if err != nil {
		return fmt.Errorf("parse draft: %w", err)
	}

	// Parse committed config for restoring values.
	committedData, readErr := guard.ReadFile(e.originalPath)
	if readErr != nil {
		return fmt.Errorf("read config: %w", readErr)
	}
	committedTree, _, parseErr := parseConfigWithFormat(string(committedData), e.schema)
	if parseErr != nil {
		return fmt.Errorf("parse config: %w", parseErr)
	}

	// Restore values for the disconnected session's entries.
	// Remove metadata first, then check if another session has pending values.
	entries := draftMeta.SessionEntries(sessionID)
	for _, se := range entries {
		pathParts := strings.Fields(se.Path)
		if len(pathParts) == 0 {
			continue
		}
		leafName := pathParts[len(pathParts)-1]
		parentPath := pathParts[:len(pathParts)-1]

		// Remove this session's metadata entry first (preserves other sessions').
		metaTarget := walkOrCreateMeta(draftMeta, e.schema, parentPath)
		metaTarget.RemoveSessionEntry(leafName, sessionID)

		// If another session still has a pending value, use that instead.
		target, walkErr := e.walkOrCreateIn(draftTree, parentPath)
		if walkErr != nil {
			continue
		}
		remaining := metaTarget.GetAllEntries(leafName)
		if len(remaining) > 0 && remaining[len(remaining)-1].Value != "" {
			target.Set(leafName, remaining[len(remaining)-1].Value)
		} else {
			// No other session has a pending value: restore committed value.
			committedValue := getValueAtPath(committedTree, e.schema, pathParts)
			if committedValue != "" {
				target.Set(leafName, committedValue)
			} else {
				target.Delete(leafName)
			}
		}
	}

	draftMeta.RemoveSession(sessionID)

	// Check if any sessions remain.
	remainingSessions := draftMeta.AllSessions()
	if len(remainingSessions) == 0 {
		guard.Remove(draftPath) //nolint:errcheck // Best effort
	} else {
		draftOutput := config.SerializeSetWithMeta(draftTree, draftMeta, e.schema)
		if err := guard.WriteFile(draftPath, []byte(draftOutput), 0o600); err != nil {
			return fmt.Errorf("write draft: %w", err)
		}
	}

	// Update in-memory state. Refresh originalContent from disk to capture
	// external commits since this editor was created (matching DiscardSessionPath).
	e.tree = draftTree
	e.meta = draftMeta
	e.originalContent = string(committedData)
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

// buildCommitMeta creates metadata for the committed config.conf.
// Starts from existing committed metadata (preserving prior commit annotations),
// overlays with the committer's entries, and copies non-session metadata from draft.
func buildCommitMeta(existingMeta, draftMeta *config.MetaTree, myEntries []config.SessionEntry, user string, commitTime time.Time, schema *config.Schema) *config.MetaTree {
	// Start from existing committed metadata to preserve prior annotations.
	commitMeta := existingMeta
	if commitMeta == nil {
		commitMeta = config.NewMetaTree()
	}

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
