// Design: docs/guide/config-editor.md -- `commit now force` over another user's conflicting change
// Overview: editor_commit.go -- per-session commit protocol
// Related: editor_draft.go -- liveOverlaps, checkDraftChanged

package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// CommitSessionForce commits like CommitSession, but a LIVE or STALE conflict
// applies instead of refusing (AC-32). Every other user's pending change that
// conflicts LIVE is removed from that user's change file, no other change of
// theirs is touched, and that user's editor is told who discarded what. A STALE
// conflict involves no other user's pending change, so it discards nothing.
func (e *Editor) CommitSessionForce() (*CommitResult, error) {
	return e.commitSession(true)
}

// CommitSessionCandidateForce is CommitSessionCandidate with force. The
// overridden changes are discarded when MarkCommittedContent confirms the
// daemon took the candidate.
func (e *Editor) CommitSessionCandidateForce(stamp time.Time) (*CommitResult, string, error) {
	return e.commitSessionCandidate(stamp, true)
}

// DiscardNoticePath returns the file that tells user what a forced commit by
// another user discarded. It does not share the change-file prefix, so no
// pending-change scan reads it.
func DiscardNoticePath(configPath, user string) string {
	var tb textbuf.Buffer
	return tb.Str(configPath).Str(".discarded.").Str(sanitizeUser(user)).String()
}

// discardOverridden removes each overridden change from its owner's change
// file and leaves that owner a notice. The caller MUST hold guard.
func (e *Editor) discardOverridden(guard storage.WriteGuard, overlaps []liveOverlap) error {
	if len(overlaps) == 0 {
		return nil
	}
	byUser := make(map[string][]liveOverlap)
	for i := range overlaps {
		user, _, _ := strings.Cut(overlaps[i].other.SessionID, "@")
		byUser[user] = append(byUser[user], overlaps[i])
	}
	for user, owned := range byUser {
		if err := e.discardUserChanges(guard, user, owned); err != nil {
			return err
		}
	}
	return nil
}

// discardUserChanges rewrites one user's change file without the overridden
// changes, then appends one notice line per change: the conflict path, who
// forced, and the discarded change's session, path and member, so the owner's
// editor can drop it from its in-memory metadata too.
func (e *Editor) discardUserChanges(guard storage.WriteGuard, user string, owned []liveOverlap) error {
	changePath := ChangePath(e.originalPath, user)
	tree, meta, ops, err := e.readChangeFile(guard, changePath)
	if err != nil {
		return err
	}
	var notice textbuf.Buffer
	for i := range owned {
		ops = e.removePendingChange(tree, meta, ops, owned[i].other)
		other := owned[i].other
		notice.Str(owned[i].conflict.Path).Byte('\t').Str(e.session.User).Byte('\t').
			Str(other.SessionID).Byte('\t').Str(other.Path).Byte('\t').Str(other.Member).Byte('\n')
	}
	if len(meta.AllSessions()) == 0 && len(ops) == 0 {
		if err := guard.Remove(changePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("forced commit: remove %s: %w", changePath, err)
		}
	} else {
		output := config.SerializeChangeFile(tree, meta, ops, e.schema)
		if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
			return fmt.Errorf("forced commit: rewrite %s: %w", changePath, err)
		}
	}

	noticePath := DiscardNoticePath(e.originalPath, user)
	previous, err := guard.ReadFile(noticePath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("forced commit: read %s: %w", noticePath, err)
	}
	previous = append(previous, notice.String()...)
	if err := guard.WriteFile(noticePath, previous, 0o600); err != nil {
		return fmt.Errorf("forced commit: write %s: %w", noticePath, err)
	}
	return nil
}

// removePendingChange drops one pending change from a parsed change file and
// returns the structural ops that remain. A leaf entry leaves the meta and the
// tree; a leaf-list member entry takes only its own member, so the owner's
// other members survive; a structural op is matched by its pending-change key.
func (e *Editor) removePendingChange(tree *config.Tree, meta *config.MetaTree, ops []config.StructuralOp, change config.PendingChange) []config.StructuralOp {
	key := pendingChangeKey(change)
	kept := ops[:0]
	for i := range ops {
		if ops[i].SessionKey() == change.SessionID && pendingChangeKey(ops[i].PendingChange()) == key {
			continue
		}
		kept = append(kept, ops[i])
	}

	parts := strings.Fields(change.Path)
	if len(parts) == 0 {
		return kept
	}
	leaf := parts[len(parts)-1]
	parent := parts[:len(parts)-1]
	if metaTarget := walkMetaReadOnly(meta, e.schema, parent); metaTarget != nil {
		var others []config.MetaEntry
		for _, entry := range metaTarget.GetAllEntries(leaf) {
			if entry.SessionKey() == change.SessionID && entry.Member != change.Member {
				others = append(others, entry)
			}
		}
		metaTarget.RemoveSessionEntry(leaf, change.SessionID)
		for _, entry := range others {
			metaTarget.SetEntry(leaf, entry)
		}
	}
	if treeTarget := walkPath(tree, e.schema, parent); treeTarget != nil {
		if change.Member == "" {
			treeTarget.Delete(leaf)
		} else {
			treeTarget.RemoveMultiValueMember(leaf, change.Member)
		}
	}
	return kept
}

// takeDiscardNotice returns, once, what forced commits by other users
// discarded from this user's changes, and removes the notice. An empty answer
// means there is no notice; a notice that cannot be read is said so, because
// dropping it would hide a discarded change from its owner.
func (e *Editor) takeDiscardNotice() string {
	noticePath := DiscardNoticePath(e.originalPath, e.session.User)
	if !e.store.Exists(noticePath) {
		return ""
	}
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return "a forced commit discarded some of your changes, and the notice could not be read: " + err.Error()
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock

	data, err := guard.ReadFile(noticePath)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err != nil {
		return "a forced commit discarded some of your changes, and the notice could not be read: " + err.Error()
	}
	if err := guard.Remove(noticePath); err != nil {
		draftLogger.Warn("discard notice shown but not removed", "path", noticePath, "error", err)
	}

	var tb textbuf.Buffer
	first := true
	for line := range strings.Lines(string(data)) {
		fields := strings.Split(strings.TrimRight(line, "\n"), "\t")
		if len(fields) != 5 {
			continue
		}
		path, user := fields[0], fields[1]
		// The owner's editor also holds the discarded entry in memory, and
		// show | changes reads it from there as well as from the change file.
		if e.meta != nil {
			e.removePendingChange(config.NewTree(), e.meta, nil, config.PendingChange{SessionID: fields[2], Path: fields[3], Member: fields[4]})
		}
		if !first {
			tb.Str("; ")
		}
		first = false
		tb.Str("Your change at ").Str(path).Str(" was discarded by ").Str(user).Str("'s forced commit")
	}
	return tb.String()
}
