// Design: docs/architecture/web-interface.md -- reopening an isolated candidate.
// Related: editor_commit.go -- materialized replay and committed-session merge.
// Related: editor_draft.go -- durable pending intent and structural replay.

package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
)

// ResumeSession reopens pending intent of this account and connection origin.
// The caller MUST assign SetSession first and MUST keep the editor private
// until this returns successfully. It is not safe for concurrent use.
// Two guarded identity writes are not atomic: either may have landed when an
// error returns. Every record keeps the same eligible account and origin, so
// another attempt can resume both old IDs and its own ID without discarding it.
func (e *Editor) ResumeSession() error {
	if e.session == nil {
		return errNoSessionSet
	}
	if e.store == nil {
		return resumeSessionFailure(storage.ErrNoStore)
	}
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return resumeSessionFailure(err)
	}
	tree, pending, resumeErr := e.resumeSession(guard)
	releaseErr := guard.Release()
	if err := errors.Join(resumeErr, releaseErr); err != nil {
		return resumeSessionFailure(err)
	}
	e.tree = tree
	e.treeValid = true
	e.workingContent = config.Serialize(tree, e.schema)
	e.dirty.Store(pending > 0)
	e.draftSaved = false
	return nil
}

func resumeSessionFailure(err error) error {
	return opererr.Wrap(err,
		"Your preserved configuration changes could not be reopened.",
		"The pending changes remain in storage; repair the named file or storage error before reopening the candidate",
		"Reload /show/")
}

// resumeSession holds guard through identity publication and both existing
// replay steps. The constructor chooses the visible view: CLI keeps the shared
// materialization, while an isolated editor projects only the resumed session.
func (e *Editor) resumeSession(guard storage.WriteGuard) (*config.Tree, int, error) {
	committedData, err := storage.ReadActiveConfigWithGuard(e.store, guard, e.originalPath)
	if err != nil {
		return nil, 0, fmt.Errorf("read config %s: %w", e.originalPath, err)
	}
	if _, _, err := parseEditingConfig(string(committedData), e.schema); err != nil {
		return nil, 0, fmt.Errorf("parse config %s: %w", e.originalPath, err)
	}
	draftTree, draftMeta, err := e.readResumeDraft(guard)
	if err != nil {
		return nil, 0, err
	}
	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps, err := e.readChangeFile(guard, changePath)
	if err != nil {
		return nil, 0, err
	}
	sessions := append(draftMeta.AllSessions(), changeMeta.AllSessions()...)
	for i := range changeOps {
		sessions = append(sessions, changeOps[i].SessionKey())
	}
	sessions = e.session.OrphanedSessions(sessions)
	// UTC RFC3339 identity has second resolution. A failed prior open may
	// already have rebound one file to exactly this attempt's identity.
	sessions = append(sessions, e.session.ID)
	slices.Sort(sessions)
	sessions = slices.Compact(sessions)
	draftEntries := e.resumeSessionMetadata(draftMeta, draftTree, sessions)
	changeEntries := e.resumeSessionMetadata(changeMeta, changeTree, sessions)
	for i := range changeOps {
		if !slices.Contains(sessions, changeOps[i].SessionKey()) {
			continue
		}
		changeOps[i].User = e.session.User
		changeOps[i].Source = e.session.Origin
		changeOps[i].Time = e.session.StartTime
	}
	myOps := filterStructuralOps(changeOps, e.session.ID)
	guard.SetModifier(e.session.ID)
	if draftEntries > 0 {
		output := config.SerializeSetWithMeta(draftTree, draftMeta, e.schema)
		if err := guard.WriteFile(DraftPath(e.originalPath), []byte(output), 0o600); err != nil {
			return nil, 0, fmt.Errorf("resume write draft: %w", err)
		}
	}
	if changeEntries+len(myOps) > 0 {
		output := config.SerializeChangeFile(changeTree, changeMeta, changeOps, e.schema)
		if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
			return nil, 0, fmt.Errorf("resume write change file %s: %w", changePath, err)
		}
	}
	// Guarded ownership now agrees across the two files. Reuse the saved
	// materialization planner before selecting this session's final entries;
	// replaying saved metadata at historical addresses would lose later moves.
	if err := e.reloadSessionView(guard); err != nil {
		return nil, 0, err
	}
	entries := e.meta.SessionEntriesOrdered(e.schema, e.session.ID, e.tree)
	return e.tree, len(entries) + len(myOps), nil
}

func (e *Editor) readResumeDraft(guard storage.WriteGuard) (*config.Tree, *config.MetaTree, error) {
	path := DraftPath(e.originalPath)
	data, err := guard.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return config.NewTree(), config.NewMetaTree(), nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read draft %s: %w", path, err)
	}
	// A materialized draft uses ordinary set syntax, never change-file
	// structural directives. Its normal parser must retain that refusal.
	tree, meta, err := config.NewSetParser(e.schema).ParseWithMeta(string(data))
	if err != nil {
		return nil, nil, fmt.Errorf("parse draft %s: %w", path, err)
	}
	return tree, meta, nil
}

// resumeSessionMetadata changes custody without borrowing values from the
// shared tree. Sessions are sorted chronologically; a newer assignment to the
// same leaf/member replaces an older assignment of this account and origin.
// Foreign sessions and their metadata remain unchanged.
func (e *Editor) resumeSessionMetadata(meta *config.MetaTree, order *config.Tree, sessions []string) int {
	entries := make([]config.SessionEntry, 0, len(sessions))
	for _, session := range sessions {
		entries = append(entries, meta.SessionEntriesOrdered(e.schema, session, order)...)
		meta.RemoveSession(session)
	}
	for _, entry := range entries {
		parent := entry.Path[:len(entry.Path)-1]
		leaf := entry.Path[len(entry.Path)-1]
		entry.Entry.User = e.session.User
		entry.Entry.Source = e.session.Origin
		entry.Entry.Time = e.session.StartTime
		walkOrCreateMeta(meta, e.schema, parent).SetEntry(leaf, entry.Entry)
	}
	return len(entries)
}

// workingSessionMetadata excludes other pending owners from candidate text.
// The full metadata remains on the editor for conflict checks; the serializer
// otherwise emits every contested value, even with an isolated value tree.
// Callers MUST have an active session and nonnil metadata.
func (e *Editor) workingSessionMetadata() *config.MetaTree {
	meta := e.meta.Clone()
	for _, session := range meta.AllSessions() {
		if session != e.session.ID {
			meta.RemoveSession(session)
		}
	}
	return meta
}
