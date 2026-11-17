// Design: docs/architecture/web-interface.md -- read-only discard snapshots.
// Related: editor_resume.go -- active recovery changes custody; this read does not.

package cli

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
)

// PendingDiscardChanges reads this account's durable pending intent without
// assigning a session, rebinding custody or changing the editor. The editor
// remains private: callers MUST NOT publish it as an active candidate without
// SetSession and ResumeSession. The one storage guard supplies a coherent read;
// missing intent is empty, while a read, parse or release failure is an error.
func (e *Editor) PendingDiscardChanges(username string) ([]contract.PendingChange, error) {
	if err := ValidateUser(username); err != nil {
		return nil, err
	}
	if e.store == nil {
		return nil, pendingDiscardFailure(storage.ErrNoStore)
	}
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return nil, pendingDiscardFailure(err)
	}
	changes, readErr := e.readPendingDiscardChanges(guard, username)
	releaseErr := guard.Release()
	if err := errors.Join(readErr, releaseErr); err != nil {
		return nil, pendingDiscardFailure(err)
	}
	answer := make([]contract.PendingChange, len(changes))
	for i, change := range changes {
		answer[i] = contract.PendingChange{
			User: change.User, Kind: contract.PendingChangeKind(change.Kind),
			Path: change.Path, Previous: change.Previous, Value: change.Value,
			OldPath: change.OldPath, NewPath: change.NewPath, Member: change.Member,
		}
	}
	return answer, nil
}

func pendingDiscardFailure(err error) error {
	return opererr.Wrap(err,
		"Your pending configuration changes could not be read.",
		"The changes remain in storage; repair the named file or storage error before discarding them",
		"Reload /show/")
}

// readPendingDiscardChanges holds guard across every source of durable intent.
// The shared materialized-draft planner rebases saved metadata through recorded
// structural operations without publishing an editing session. Parsed entries
// and operations bound the loops; pending-change identity deduplicates overlap.
func (e *Editor) readPendingDiscardChanges(guard storage.WriteGuard, username string) ([]config.PendingChange, error) {
	committed, err := guard.ReadFile(e.originalPath)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", e.originalPath, err)
	}
	committedTree, _, err := parseEditingConfig(string(committed), e.schema)
	if err != nil {
		return nil, fmt.Errorf("parse config %s: %w", e.originalPath, err)
	}
	draftTree, draftMeta, err := e.readResumeDraft(guard)
	if err != nil {
		return nil, err
	}
	_, changeMeta, ops, err := e.readChangeFile(guard, ChangePath(e.originalPath, username))
	if err != nil {
		return nil, err
	}
	if !guard.Has(DraftPath(e.originalPath)) {
		draftTree = committedTree
	}
	if _, err := e.prepareDraftReplay(guard, draftTree, draftMeta, ops); err != nil {
		return nil, fmt.Errorf("prepare pending snapshot: %w", err)
	}
	// Unsaved entries replace the same session/member's saved assignment,
	// including set-to-delete transitions. SetEntry preserves other sessions.
	for _, session := range changeMeta.AllSessions() {
		for _, entry := range changeMeta.SessionEntries(e.schema, session) {
			parent := entry.Path[:len(entry.Path)-1]
			leaf := entry.Path[len(entry.Path)-1]
			walkOrCreateMeta(draftMeta, e.schema, parent).SetEntry(leaf, entry.Entry)
		}
	}
	seen := make(map[string]bool)
	var changes []config.PendingChange
	add := func(change config.PendingChange) {
		if change.User != username {
			return
		}
		key := pendingChangeKey(change)
		if seen[key] {
			return
		}
		seen[key] = true
		changes = append(changes, change)
	}
	for _, session := range draftMeta.AllSessions() {
		for _, entry := range draftMeta.SessionEntries(e.schema, session) {
			add(config.PendingChangeFromSessionEntry(entry))
		}
	}
	for i := range ops {
		add(ops[i].PendingChange(e.schema))
	}
	config.SortPendingChanges(changes)
	return changes, nil
}
