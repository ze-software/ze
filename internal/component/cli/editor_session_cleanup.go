// Design: docs/architecture/config/change-file-structural-ops.md -- session-owned consumption.
// Related: editor_draft.go -- saving moves only the current session's leaves.
// Related: editor_commit.go -- accepted publication consumes its structural ops too.

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

// sessionCleanupWarning reports a cleanup failure only after publication.
// The shared change file can still belong to another origin, so recovery
// starts with read-only inspection rather than another commit or a discard.
func sessionCleanupWarning(err error) *opererr.Error {
	return opererr.Wrap(err,
		"commit applied, but its session cleanup failed",
		"The configuration is already in use; do not repeat the commit. "+
			"Keep the session files and review the retained changes before repairing the reported failure",
		"show | changes all")
}

// consumeSessionChangeEntries removes the current session's leaves from a
// parsed sparse change file. Rebuild each affected leaf from its surviving
// metadata using the normal entry producer: another session can own another
// value or member at that same path. The caller MUST hold the config guard.
// Loops are bounded by the parsed change file's entries and members.
func (e *Editor) consumeSessionChangeEntries(tree *config.Tree, meta *config.MetaTree) error {
	entries := meta.SessionEntriesOrdered(e.schema, e.session.ID, tree)
	meta.RemoveSession(e.session.ID)
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		key := entry.Path.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		parts := entry.Path
		leaf := parts[len(parts)-1]
		parent := parts[:len(parts)-1]
		target, err := e.walkOrCreateIn(tree, parent)
		if err != nil {
			return err
		}
		stored, hasStored := target.StoredSecret(leaf)
		target.Delete(leaf)
		remaining := walkMetaReadOnly(meta, e.schema, parent)
		if remaining == nil {
			continue
		}
		for _, kept := range remaining.GetAllEntries(leaf) {
			if err := e.applySessionEntryToTree(tree, parent, leaf, kept); err != nil {
				return err
			}
		}
		if hasStored {
			target.SetStoredSecret(leaf, stored)
		}
	}
	return nil
}

// writeChangeFileRemainder publishes an already-parsed residual under the
// caller's config guard. Structural ops remain authoritative even with no
// leaf metadata; a file is absent only when neither kind of intent remains.
func (e *Editor) writeChangeFileRemainder(guard storage.WriteGuard, path string, tree *config.Tree, meta *config.MetaTree, ops []config.StructuralOp) error {
	if len(meta.AllSessions()) == 0 {
		if len(ops) == 0 {
			err := guard.Remove(path)
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
	}
	content := config.SerializeChangeFile(tree, meta, ops, e.schema)
	return guard.WriteFile(path, []byte(content), 0o600)
}

// cleanupSessionFiles consumes only the session whose commit applied. It MUST
// run under the existing commit guard, or cleanupCommittedSession's separately
// acquired guard. Its error is a warning on an applied commit, never a reason
// to roll publication back. Both files are attempted so one failure does not
// suppress cleanup of the other.
func (e *Editor) cleanupSessionFiles(guard storage.WriteGuard) error {
	return errors.Join(e.cleanupSessionDraft(guard), e.cleanupSessionChanges(guard))
}

func (e *Editor) cleanupSessionDraft(guard storage.WriteGuard) error {
	path := DraftPath(e.originalPath)
	data, err := guard.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read saved draft %s: %w", path, err)
	}
	tree, meta, err := config.NewSetParser(e.schema).ParseWithMeta(string(data))
	if err != nil {
		return fmt.Errorf("parse saved draft %s: %w", path, err)
	}
	owned := meta.HasSession(e.session.ID)
	meta.RemoveSession(e.session.ID)
	if len(meta.AllSessions()) == 0 {
		if err := guard.Remove(path); err != nil {
			return fmt.Errorf("remove saved draft %s: %w", path, err)
		}
		return nil
	}
	// A structural-only commit can leave a materialization with no leaf owner.
	// Remove that empty-ownership draft above; otherwise a confirmed revert
	// would reload the just-committed move. Other owners keep their draft.
	if !owned {
		return nil
	}
	content := config.SerializeSetWithMeta(tree, meta, e.schema)
	if err := guard.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write saved draft %s: %w", path, err)
	}
	return nil
}

func (e *Editor) cleanupSessionChanges(guard storage.WriteGuard) error {
	path := ChangePath(e.originalPath, e.session.User)
	tree, meta, ops, err := e.readChangeFile(guard, path)
	if err != nil {
		return err
	}
	kept := slices.DeleteFunc(slices.Clone(ops), func(op config.StructuralOp) bool {
		return op.SessionKey() == e.session.ID
	})
	if !meta.HasSession(e.session.ID) {
		if len(kept) == len(ops) {
			return nil
		}
	}
	if err := e.consumeSessionChangeEntries(tree, meta); err != nil {
		return err
	}
	if err := e.writeChangeFileRemainder(guard, path, tree, meta, kept); err != nil {
		return fmt.Errorf("write remaining changes %s: %w", path, err)
	}
	return nil
}
