// Design: docs/architecture/config/yang-config-design.md — leaf-list member write-through
// Overview: editor_draft.go — write-through draft protocol (scalar leaves, structural op apply)
// Related: editor_commit.go — commit/discard protocol consuming member entries and ops
// Related: editor_walk.go — applySessionEntryToTree (member-aware apply helper)

package cli

import (
	"fmt"
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
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	// Validate the path against the schema before mutating anything.
	if _, walkErr := e.walkOrCreateIn(e.tree.Clone(), path); walkErr != nil {
		return fmt.Errorf("write-through set path: %w", walkErr)
	}

	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps := e.readChangeFile(guard, changePath)

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

	output := config.SerializeChangeFile(changeTree, changeMeta, changeOps, e.schema)
	if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
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
	guard, err := e.store.AcquireLock(e.originalPath)
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
	changeTree, changeMeta, changeOps := e.readChangeFile(guard, changePath)

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

	output := config.SerializeChangeFile(changeTree, changeMeta, changeOps, e.schema)
	if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
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
	guard, err := e.store.AcquireLock(e.originalPath)
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
	switch opType { //nolint:exhaustive // only member op types reach this method
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
	default:
		return fmt.Errorf("unsupported member op %q", opType)
	}

	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps := e.readChangeFile(guard, changePath)
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

	output := config.SerializeChangeFile(changeTree, changeMeta, changeOps, e.schema)
	if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
	}

	e.dirty.Store(true)
	e.draftSaved = false
	return nil
}
