// Design: docs/guide/config-editor.md -- `commit now force` over another user's conflicting change
// Overview: editor_commit.go -- per-session commit protocol
// Related: editor_draft.go -- liveOverlaps, checkDraftChanged

package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// CommitSessionForce commits like CommitSession, but a LIVE or STALE conflict
// applies instead of refusing (AC-32). Every other user's pending change that
// conflicts LIVE is removed from that user's change file, or from the shared
// draft when that user already saved it, no other change of
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

// overriddenNotDiscarded is what a forced commit that landed reports when its
// discard of the overridden changes failed: those changes stay pending for
// their owners, and the owner's next commit would apply them again.
func overriddenNotDiscarded(err error) error {
	return fmt.Errorf("commit applied, but the changes it overrode were not discarded and stay pending for their owners: %w", err)
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

// discardUserChanges removes the overridden changes from wherever the owner
// holds them, then appends one notice line per change it removed: the
// conflict path and who forced. A pending change lives in the owner's change
// file until the owner saves, and in the shared draft after (SaveDraft), so
// both are rewritten; a notice for a change found in neither would be false.
// The owner's editor rebuilds its working tree from the rewritten files when
// it reads the notice (TakeDiscardNotice).
func (e *Editor) discardUserChanges(guard storage.WriteGuard, user string, owned []liveOverlap) error {
	fromChange, err := e.discardFromChangeFile(guard, user, owned)
	if err != nil {
		return err
	}
	fromDraft, err := e.discardFromDraft(guard, owned)
	if err != nil {
		return err
	}

	noticePath := DiscardNoticePath(e.originalPath, user)
	previous, err := guard.ReadFile(noticePath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("forced commit: read %s: %w", noticePath, err)
	}
	lines := parseDiscardNotice(previous)

	// The stamp is later than every line already logged, so a session that
	// showed those never takes this commit's lines for seen ones.
	now := time.Now()
	stamp := now.UnixNano()
	for _, line := range lines {
		stamp = max(stamp, line.stamp+1)
	}
	added := 0
	for i := range owned {
		if !fromChange[i] && !fromDraft[i] {
			continue
		}
		lines = append(lines, discardNoticeLine{stamp: stamp, path: owned[i].conflict.Path, forcer: e.session.User})
		added++
	}
	if added == 0 {
		return nil
	}
	if err := writeDiscardNotice(guard, noticePath, lines, now); err != nil {
		return fmt.Errorf("forced commit: %w", err)
	}
	return nil
}

// discardFromChangeFile rewrites one user's change file without the
// overridden changes. It reports, per overlap, whether the change was there.
func (e *Editor) discardFromChangeFile(guard storage.WriteGuard, user string, owned []liveOverlap) ([]bool, error) {
	removed := make([]bool, len(owned))
	changePath := ChangePath(e.originalPath, user)
	tree, meta, ops, err := e.readChangeFile(guard, changePath)
	if err != nil {
		return nil, err
	}
	for i := range owned {
		ops, removed[i] = e.removePendingChange(tree, meta, ops, owned[i].other)
	}
	if !slices.Contains(removed, true) {
		return removed, nil
	}
	if len(meta.AllSessions()) == 0 && len(ops) == 0 {
		if err := guard.Remove(changePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("forced commit: remove %s: %w", changePath, err)
		}
		return removed, nil
	}
	output := config.SerializeChangeFile(tree, meta, ops, e.schema)
	if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
		return nil, fmt.Errorf("forced commit: rewrite %s: %w", changePath, err)
	}
	return removed, nil
}

// discardFromDraft takes the overridden changes the owner already saved out
// of the shared draft. The draft holds a whole tree, so a removed change puts
// back the committed value at its path (the forced one), where a change file
// would simply lose the leaf. It reports, per overlap, whether the change was
// there. The caller MUST have written the forced commit to the config file.
func (e *Editor) discardFromDraft(guard storage.WriteGuard, owned []liveOverlap) ([]bool, error) {
	removed := make([]bool, len(owned))
	draftPath := DraftPath(e.originalPath)
	data, err := guard.ReadFile(draftPath)
	if errors.Is(err, fs.ErrNotExist) {
		return removed, nil
	}
	if err != nil {
		return nil, fmt.Errorf("forced commit: read %s: %w", draftPath, err)
	}
	tree, meta, err := config.NewSetParser(e.schema).ParseWithMeta(string(data))
	if err != nil {
		return nil, fmt.Errorf("forced commit: draft %s does not parse: %w", draftPath, err)
	}
	for i := range owned {
		removed[i] = removeMetaChange(meta, e.schema, owned[i].other)
	}
	if !slices.Contains(removed, true) {
		return removed, nil
	}
	committed := e.readCommittedTreeFrom(guard)
	if committed == nil {
		return nil, fmt.Errorf("forced commit: read the committed config to restore %s", draftPath)
	}
	for i := range owned {
		if removed[i] {
			e.restoreCommittedValue(tree, committed, owned[i].other)
		}
	}
	if len(meta.AllSessions()) == 0 {
		if err := guard.Remove(draftPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("forced commit: remove %s: %w", draftPath, err)
		}
		return removed, nil
	}
	output := config.SerializeSetWithMeta(tree, meta, e.schema)
	if err := guard.WriteFile(draftPath, []byte(output), 0o600); err != nil {
		return nil, fmt.Errorf("forced commit: rewrite %s: %w", draftPath, err)
	}
	return removed, nil
}

// restoreCommittedValue sets the leaf or leaf-list member a discarded change
// named back to what the committed tree holds there, absent included.
func (e *Editor) restoreCommittedValue(tree, committed *config.Tree, change config.PendingChange) {
	parts := strings.Fields(change.Path)
	if len(parts) == 0 {
		return
	}
	leaf := parts[len(parts)-1]
	parent := parts[:len(parts)-1]
	target := walkPath(tree, e.schema, parent)
	if target == nil {
		return
	}
	source := walkPath(committed, e.schema, parent)
	if change.Member != "" {
		if source != nil && source.HasMultiValueMember(leaf, change.Member) {
			target.AddMultiValueMember(leaf, change.Member)
			return
		}
		target.RemoveMultiValueMember(leaf, change.Member)
		return
	}
	if source != nil {
		if value, ok := source.Get(leaf); ok {
			target.Set(leaf, value)
			return
		}
	}
	target.Delete(leaf)
}

// removePendingChange drops one pending change from a parsed change file and
// returns the structural ops that remain, and whether the change was there. A
// leaf entry leaves the meta and the tree; a leaf-list member entry takes only
// its own member, so the owner's other members survive; a structural op is
// matched by its pending-change key.
func (e *Editor) removePendingChange(tree *config.Tree, meta *config.MetaTree, ops []config.StructuralOp, change config.PendingChange) ([]config.StructuralOp, bool) {
	key := pendingChangeKey(change)
	removed := false
	kept := ops[:0]
	for i := range ops {
		if ops[i].SessionKey() == change.SessionID && pendingChangeKey(ops[i].PendingChange()) == key {
			removed = true
			continue
		}
		kept = append(kept, ops[i])
	}

	parts := strings.Fields(change.Path)
	if len(parts) == 0 {
		return kept, removed
	}
	if !removeMetaChange(meta, e.schema, change) {
		return kept, removed
	}
	leaf := parts[len(parts)-1]
	if treeTarget := walkPath(tree, e.schema, parts[:len(parts)-1]); treeTarget != nil {
		if change.Member == "" {
			treeTarget.Delete(leaf)
		} else {
			treeTarget.RemoveMultiValueMember(leaf, change.Member)
		}
	}
	return kept, true
}

// removeMetaChange removes one pending change's meta entry and reports whether
// it was there. A leaf-list member entry takes only its own member, so the
// owner's other members survive.
func removeMetaChange(meta *config.MetaTree, schema *config.Schema, change config.PendingChange) bool {
	parts := strings.Fields(change.Path)
	if len(parts) == 0 {
		return false
	}
	leaf := parts[len(parts)-1]
	metaTarget := walkMetaReadOnly(meta, schema, parts[:len(parts)-1])
	if metaTarget == nil {
		return false
	}
	found := false
	var others []config.MetaEntry
	for _, entry := range metaTarget.GetAllEntries(leaf) {
		if entry.SessionKey() != change.SessionID {
			continue
		}
		if entry.Member == change.Member {
			found = true
			continue
		}
		others = append(others, entry)
	}
	if !found {
		return false
	}
	metaTarget.RemoveSessionEntry(leaf, change.SessionID)
	for _, entry := range others {
		metaTarget.SetEntry(leaf, entry)
	}
	return true
}

// The discard notice is one file per user, a log every session of that user
// reads: each line is "<stamp>\t<shown>\t<path>\t<forcer>", stamp and
// shown in Unix nanoseconds, shown 0 until a session first showed the line.
// Each session shows every line stamped after the last one it showed, so all
// of a user's sessions are told, once each. A line shown at least
// discardNoticeLinger ago is pruned (the SSH and web polls look every one to
// two seconds), and the log keeps at most discardNoticeLinesMax lines.
const (
	discardNoticeLinger   = time.Minute
	discardNoticeLinesMax = 64
)

// discardNoticeLine is one line of the discard notice log.
type discardNoticeLine struct {
	stamp  int64
	shown  int64
	path   string
	forcer string
}

// parseDiscardNotice reads the notice log; a line it cannot read is skipped.
func parseDiscardNotice(data []byte) []discardNoticeLine {
	var lines []discardNoticeLine
	for raw := range strings.Lines(string(data)) {
		fields := strings.SplitN(strings.TrimRight(raw, "\n"), "\t", 4)
		if len(fields) != 4 {
			continue
		}
		stamp, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		shown, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		lines = append(lines, discardNoticeLine{stamp: stamp, shown: shown, path: fields[2], forcer: fields[3]})
	}
	return lines
}

// writeDiscardNotice prunes the lines shown long enough ago, keeps the
// newest discardNoticeLinesMax, and writes the log, or removes it when empty.
// The caller MUST hold guard.
func writeDiscardNotice(guard storage.WriteGuard, noticePath string, lines []discardNoticeLine, now time.Time) error {
	kept := lines[:0]
	for _, line := range lines {
		if line.shown != 0 && now.Sub(time.Unix(0, line.shown)) >= discardNoticeLinger {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) > discardNoticeLinesMax {
		kept = kept[len(kept)-discardNoticeLinesMax:]
	}
	if len(kept) == 0 {
		if err := guard.Remove(noticePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("discard notice: remove %s: %w", noticePath, err)
		}
		return nil
	}
	var tb textbuf.Buffer
	for _, line := range kept {
		tb.Int(line.stamp).Byte('\t').Int(line.shown).Byte('\t').Str(line.path).Byte('\t').Str(line.forcer).Byte('\n')
	}
	if err := guard.WriteFile(noticePath, []byte(tb.String()), 0o600); err != nil {
		return fmt.Errorf("discard notice: write %s: %w", noticePath, err)
	}
	return nil
}

// TakeDiscardNotice returns, once for this session, what forced commits by
// other users discarded from this user's changes, and rebuilds this editor's
// working tree without the discarded values: PendingDiscardNotice, then
// AckDiscardNotice. An empty answer means there is no notice.
func (e *Editor) TakeDiscardNotice() string {
	notice, through := e.PendingDiscardNotice()
	if through == 0 {
		return notice
	}
	if err := e.AckDiscardNotice(through); err != nil {
		var tb textbuf.Buffer
		return tb.Str(notice).Str("; your editor could not reload, so it may still show the discarded value until you reconnect: ").Err(err).String()
	}
	return notice
}

// PendingDiscardNotice returns what this session has not yet shown of the
// user's discard notice, and the stamp it runs through; the caller MUST call
// AckDiscardNotice with that stamp once the notice reached the user, and the
// notice is offered again until it does. A through of 0 with a notice means
// the log could not be read: said, because dropping it would hide a discarded
// change from its owner.
func (e *Editor) PendingDiscardNotice() (string, int64) {
	noticePath := DiscardNoticePath(e.originalPath, e.session.User)
	if !e.store.Exists(noticePath) {
		return "", 0
	}
	data, err := e.store.ReadFile(noticePath)
	if errors.Is(err, fs.ErrNotExist) {
		return "", 0
	}
	if err != nil {
		return "a forced commit discarded some of your changes, and the notice could not be read: " + err.Error(), 0
	}
	var tb textbuf.Buffer
	var through int64
	for _, line := range parseDiscardNotice(data) {
		if line.stamp <= e.discardNoticeSeen {
			continue
		}
		if through != 0 {
			tb.Str("; ")
		}
		through = max(through, line.stamp)
		tb.Str("Your change at ").Str(line.path).Str(" was discarded by ").Str(line.forcer).Str("'s forced commit")
	}
	return tb.String(), through
}

// AckDiscardNotice records that this session showed the notice through the
// stamp PendingDiscardNotice returned, marks those lines shown in the log,
// and rebuilds the view from disk: the editor still holds the discarded value
// in its working tree and meta, which show and show | changes read. It MUST be
// called after PendingDiscardNotice, once the notice reached the user.
func (e *Editor) AckDiscardNotice(through int64) error {
	if through <= e.discardNoticeSeen {
		return nil
	}
	e.discardNoticeSeen = through
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return err
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock

	noticePath := DiscardNoticePath(e.originalPath, e.session.User)
	data, err := guard.ReadFile(noticePath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("discard notice: read %s: %w", noticePath, err)
	}
	now := time.Now()
	lines := parseDiscardNotice(data)
	for i := range lines {
		if lines[i].stamp <= through && lines[i].shown == 0 {
			lines[i].shown = now.UnixNano()
		}
	}
	if err := writeDiscardNotice(guard, noticePath, lines, now); err != nil {
		draftLogger.Warn("discard notice shown but not marked", "path", noticePath, "error", err)
	}
	return e.reloadSessionView(guard)
}
