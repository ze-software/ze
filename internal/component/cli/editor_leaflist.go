// Design: docs/architecture/config/yang-config-design.md — leaf-list member write-through
// Overview: editor_draft.go — write-through draft protocol (scalar leaves, structural op apply)
// Related: editor_commit.go — commit/discard protocol consuming member entries and ops
// Related: editor_walk.go — applySessionEntryToTree (member-aware apply helper)

package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
)

// writeThroughSetMember implements the write-through protocol for adding one
// leaf-list member (JunOS add-member semantics). The member is stored in the
// multi-value map of both the change tree and the in-memory tree, with a
// per-member metadata entry (Member set) so concurrent sessions can add or
// remove independent members without contesting the whole leaf. No Previous
// is recorded: member operations are idempotent and skip stale-conflict
// detection.
func (e *Editor) writeThroughSetMember(path []string, key, member string) error {
	guard, err := e.draftLock()
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	// Validate the path against the schema before mutating anything.
	if _, walkErr := e.walkOrCreateIn(e.probeTree(), path); walkErr != nil {
		return fmt.Errorf("write-through set path: %w", walkErr)
	}

	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps, err := e.openChangeFile(guard, changePath)
	if err != nil {
		return err
	}

	changeTarget, err := e.walkOrCreateIn(changeTree, path)
	if err != nil {
		return fmt.Errorf("write-through set change path: %w", err)
	}
	changeTarget.AddMultiValueMember(key, member)

	entry := config.MetaEntry{
		User:   e.session.User,
		Source: e.session.Origin,
		Time:   e.session.StartTime,
		Value:  member,
		Member: member,
	}
	changeMetaTarget := walkOrCreateMeta(changeMeta, e.schema, path)
	changeMetaTarget.SetEntry(key, entry)

	if err := e.writeChangeFile(guard, changePath, changeTree, changeMeta, changeOps); err != nil {
		return err
	}

	// Update in-memory tree directly (base + own changes).
	target, _ := e.walkOrCreateIn(e.tree, path)
	target.AddMultiValueMember(key, member)
	metaTarget := walkOrCreateMeta(e.meta, e.schema, path)
	metaTarget.SetEntry(key, entry)

	e.dirty.Store(true)
	e.draftSaved = false
	return nil
}

// writeThroughDeleteMember implements the write-through protocol for removing
// one leaf-list member. Records a delete intent (Member set, Value empty) in
// the per-user change file and removes the member from both trees.
func (e *Editor) writeThroughDeleteMember(path []string, key, member string) error {
	guard, err := e.draftLock()
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	// Verify the member exists in the in-memory tree before mutating.
	target := walkPath(e.tree, e.schema, path)
	if target == nil {
		return errPathNotFound
	}
	if !target.HasMultiValueMember(key, member) {
		return fmt.Errorf("%q not found in %s", member, key)
	}

	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps, err := e.openChangeFile(guard, changePath)
	if err != nil {
		return err
	}

	// The member leaves the list, so this session's insert, deactivate and
	// activate ops naming it have nothing left to act on. Kept, a deactivate
	// held for a member the session added would run after the delete entry
	// and fail the commit with "not found". A repeated member of an ordered
	// list keeps its ops while another copy stays.
	if memberOccurrences(target, key, member) == 1 {
		changeOps = dropSessionMemberOps(changeOps, e.session.ID, path, key, func(op config.StructuralOp) bool {
			return op.NewKey == member
		})
	}

	// Drop the member from the change tree in case this session added it
	// earlier, and ensure the parent path exists so the serializer can
	// navigate to the delete-intent metadata.
	if _, walkErr := e.walkOrCreateIn(changeTree, path); walkErr != nil {
		return fmt.Errorf("write-through delete change path: %w", walkErr)
	}
	if changeTarget := walkPath(changeTree, e.schema, path); changeTarget != nil {
		changeTarget.RemoveMultiValueMember(key, member)
	}

	entry := config.MetaEntry{
		User:   e.session.User,
		Source: e.session.Origin,
		Time:   e.session.StartTime,
		Member: member,
	}
	changeMetaTarget := walkOrCreateMeta(changeMeta, e.schema, path)
	changeMetaTarget.SetEntry(key, entry)

	if err := e.writeChangeFile(guard, changePath, changeTree, changeMeta, changeOps); err != nil {
		return err
	}

	// Update in-memory tree.
	target.RemoveMultiValueMember(key, member)
	metaTarget := walkOrCreateMeta(e.meta, e.schema, path)
	metaTarget.SetEntry(key, entry)

	e.dirty.Store(true)
	e.draftSaved = false
	return nil
}

// writeThroughMemberOp records an ordered leaf-list member operation
// (insert at position, deactivate, activate) as a structural op in the
// per-user change file and applies it to the in-memory tree. Structural ops
// preserve the exact position/toggle through SaveDraft and both commit
// paths, where plain add-member metadata entries would degrade to append.
func (e *Editor) writeThroughMemberOp(path []string, opType config.StructuralOpType, leafListName, member, position, ref string) error {
	guard, err := e.draftLock()
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	// Apply to the in-memory tree first so user errors (missing member,
	// missing reference, already deactivated) surface before recording.
	target := walkPath(e.tree, e.schema, path)
	if target == nil {
		return errPathNotFound
	}
	switch opType {
	case config.StructuralOpInsertMember:
		if err := target.InsertMultiValue(leafListName, member, position, ref); err != nil {
			return err
		}
	case config.StructuralOpDeactivateMember:
		if err := target.DeactivateMultiValue(leafListName, member); err != nil {
			return err
		}
	case config.StructuralOpActivateMember:
		if err := target.ActivateMultiValue(leafListName, member); err != nil {
			return err
		}
	case "", config.StructuralOpRename, config.StructuralOpDeleteEntry, config.StructuralOpDeleteContainer, config.StructuralOpDeleteList,
		config.StructuralOpCopyEntry, config.StructuralOpDeactivateLeaf, config.StructuralOpActivateLeaf,
		config.StructuralOpDeactivatePath, config.StructuralOpActivatePath:
		return fmt.Errorf("unsupported member op %q", opType)
	default:
		panic("BUG: invalid structural member operation")
	}

	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps, err := e.openChangeFile(guard, changePath)
	if err != nil {
		return err
	}
	changeOps = append(changeOps, config.StructuralOp{
		Type:       opType,
		User:       e.session.User,
		Source:     e.session.Origin,
		Time:       e.session.StartTime,
		ParentPath: strings.Join(path, " "),
		ListName:   leafListName,
		NewKey:     member,
		OldKey:     ref,
		Position:   position,
	})

	if err := e.writeChangeFile(guard, changePath, changeTree, changeMeta, changeOps); err != nil {
		return err
	}

	e.dirty.Store(true)
	e.draftSaved = false
	return nil
}

// replaceSessionMembers prepares a whole-leaf set of the bracket leaf-list key
// under path. It answers the value to record, the ops to keep, and which of
// this session's member entries on the leaf the set replaces; it has already
// removed those from changeMeta, and the caller removes them from the
// in-memory metadata with the same answer.
//
// The list the set writes replaces what the session's member entries added,
// and a delete of a member the list names: replayed after the list line, that
// delete would drop the member again. A delete of a member the list does not
// name stays, as the committed tree may hold that member deactivated, which a
// whole-leaf set keeps. The session's member ops on the leaf go too, except a
// deactivate whose member the view still holds, and an activate whose member
// the view still holds or the committed tree holds: a positional insert means
// nothing once the whole list is written, and any other op would fail the
// commit or bring its member back.
//
// The view keeps a deactivated member the new list does not name
// (Tree.SetLeafListMembers), so the answered value names it as well: the
// commit applies the value to the committed tree, and a member the session
// added and deactivated must be there for its kept deactivate. A member this
// session deleted and now lists again, which the committed tree holds
// deactivated, shows active in the view, so it gets an activate op.
func (e *Editor) replaceSessionMembers(changeMeta *config.MetaTree, changeOps []config.StructuralOp, committedTree *config.Tree, path []string, key, value string) (string, []config.StructuralOp, func(config.MetaEntry) bool) {
	items := config.SplitLeafListMembers(value)
	listed := len(items)
	if view := walkPath(e.tree, e.schema, path); view != nil {
		for _, state := range view.GetMultiValuesState(key) {
			if state.Inactive && !slices.Contains(items, state.Value) {
				items = append(items, state.Value)
			}
		}
	}
	if len(items) != listed {
		value = config.JoinLeafListMembers(items)
	}

	var committed *config.Tree
	if committedTree != nil {
		committed = walkPath(committedTree, e.schema, path)
	}
	committedHolds := func(member string) bool {
		if committed == nil {
			return false
		}
		present, _ := committed.MultiValueMemberState(key, member)
		return present
	}
	changeOps = dropSessionMemberOps(changeOps, e.session.ID, path, key, func(op config.StructuralOp) bool {
		if op.Type == config.StructuralOpInsertMember {
			return true
		}
		if slices.Contains(items, op.NewKey) {
			return false
		}
		return op.Type != config.StructuralOpActivateMember || !committedHolds(op.NewKey)
	})

	dropEntry := func(entry config.MetaEntry) bool {
		return entry.Value != "" || slices.Contains(items, entry.Member)
	}
	removed := walkOrCreateMeta(changeMeta, e.schema, path).RemoveSessionMemberEntries(key, e.session.ID, dropEntry)
	for _, entry := range removed {
		if entry.Value != "" {
			continue // An added member: the list now says whether it stays.
		}
		if committed == nil {
			continue
		}
		if present, inactive := committed.MultiValueMemberState(key, entry.Member); present && inactive {
			changeOps = append(changeOps, config.StructuralOp{
				Type:       config.StructuralOpActivateMember,
				User:       e.session.User,
				Source:     e.session.Origin,
				Time:       e.session.StartTime,
				ParentPath: strings.Join(path, " "),
				ListName:   key,
				NewKey:     entry.Member,
			})
		}
	}
	return value, changeOps, dropEntry
}

// dropSessionMemberOps answers ops without the member ops (insert, deactivate,
// activate) of session sessionID on the leaf-list key under path that drop
// selects. Every other op stays, in order.
func dropSessionMemberOps(ops []config.StructuralOp, sessionID string, path []string, key string, drop func(config.StructuralOp) bool) []config.StructuralOp {
	parentPath := strings.Join(path, " ")
	kept := make([]config.StructuralOp, 0, len(ops))
	for i := range ops {
		if isSessionMemberOp(ops[i], sessionID, parentPath, key) && drop(ops[i]) {
			continue
		}
		kept = append(kept, ops[i])
	}
	return kept
}

// isSessionMemberOp reports whether op is a member op of session sessionID on
// the leaf-list key under parentPath.
func isSessionMemberOp(op config.StructuralOp, sessionID, parentPath, key string) bool {
	if !isMemberOp(op) {
		return false
	}
	if op.SessionKey() != sessionID {
		return false
	}
	if op.ParentPath != parentPath {
		return false
	}
	return op.ListName == key
}

// memberOccurrences answers how many times the leaf-list key of target holds
// member, deactivated copies included. Only an ordered list repeats one.
func memberOccurrences(target *config.Tree, key, member string) int {
	n := 0
	for _, state := range target.GetMultiValuesState(key) {
		if state.Value == member {
			n++
		}
	}
	return n
}
