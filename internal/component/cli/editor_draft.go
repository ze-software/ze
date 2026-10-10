// Design: docs/architecture/config/yang-config-design.md — write-through draft protocol
// Overview: editor.go — config editor (calls write-through from SetValue/DeleteValue)
// Detail: editor_walk.go — schema-aware tree/meta walking
// Detail: editor_commit.go — commit/discard/disconnect protocol
// Related: editor_session.go — session identity for concurrent editing

package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/cli/contract"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errNewKeyMustDifferFromCurrent = errors.New("new key must differ from current key")

var draftLogger = slogutil.Logger("cli.editor.draft")

// ConflictType is a type alias of contract.ConflictType.
type ConflictType = contract.ConflictType

// Conflict kinds use the authoritative contract constants.
const (
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
	guard, err := e.draftLock()
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	// Validate the path against the schema before mutating anything.
	// Use a temporary tree to check walkOrCreateIn succeeds.
	if _, walkErr := e.walkOrCreateIn(e.probeTree(), path); walkErr != nil {
		return fmt.Errorf("write-through set path: %w", walkErr)
	}

	// Read change file (sparse tree of this user's changes).
	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps, err := e.openChangeFile(guard, changePath)
	if err != nil {
		return err
	}

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
	if err := e.writeChangeFile(guard, changePath, changeTree, changeMeta, changeOps); err != nil {
		return err
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
	guard, err := e.draftLock()
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	// Validate the path against the schema.
	if _, walkErr := e.walkOrCreateIn(e.probeTree(), path); walkErr != nil {
		return fmt.Errorf("write-through create path: %w", walkErr)
	}

	// Read change file.
	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps, err := e.openChangeFile(guard, changePath)
	if err != nil {
		return err
	}

	// Create the path in the change tree (no leaf set).
	if _, walkErr := e.walkOrCreateIn(changeTree, path); walkErr != nil {
		return fmt.Errorf("write-through create change path: %w", walkErr)
	}

	// Serialize and write change file.
	if err := e.writeChangeFile(guard, changePath, changeTree, changeMeta, changeOps); err != nil {
		return err
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
	guard, err := e.draftLock()
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	// Verify path exists in in-memory tree before mutating.
	target := walkPath(e.tree, e.schema, path)
	if target == nil {
		return errPathNotFound
	}

	// Read change file.
	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps, err := e.openChangeFile(guard, changePath)
	if err != nil {
		return err
	}

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
	if err := e.writeChangeFile(guard, changePath, changeTree, changeMeta, changeOps); err != nil {
		return err
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
	guard, err := e.draftLock()
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	if oldKey == newKey {
		return errNewKeyMustDifferFromCurrent
	}

	working := e.tree.Clone()
	var validateTarget *config.Tree
	if len(parentPath) == 0 {
		validateTarget = working
	} else {
		validateTarget = walkPath(working, e.schema, parentPath)
	}
	if validateTarget == nil {
		return errPathNotFound
	}
	if err := validateTarget.RenameListEntry(listName, oldKey, newKey); err != nil {
		return err
	}

	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps, err := e.openChangeFile(guard, changePath)
	if err != nil {
		return err
	}
	proposedOp := config.StructuralOp{
		Type:       config.StructuralOpRename,
		User:       e.session.User,
		Source:     e.session.Origin,
		Time:       e.session.StartTime,
		ParentPath: textbuf.Join(parentPath, " "),
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

	if err := e.writeChangeFile(guard, changePath, changeTree, changeMeta, changeOps); err != nil {
		return err
	}

	var target *config.Tree
	if len(parentPath) == 0 {
		target = e.tree
	} else {
		target = walkPath(e.tree, e.schema, parentPath)
	}
	if target == nil {
		return errPathNotFound
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

// writeThroughCopy records a list-entry copy as one copy-entry structural op
// in the per-user change file and copies the entry in the in-memory tree. The
// copy is proved on a clone first, so a missing source or an existing
// destination refuses before anything is written.
//
// The commit applies the copy-entry op to the committed source before this
// session's leaf edits, so the copy would miss any pending edit of the source.
// The change file therefore carries the source's pending edits under the
// destination too, as writeThroughRename rebases them onto the new key.
func (e *Editor) writeThroughCopy(parentPath []string, listName, sourceKey, targetKey string) error {
	probe := walkPath(e.tree.Clone(), e.schema, parentPath)
	if probe == nil {
		return errPathNotFound
	}
	if err := probe.CopyListEntry(listName, sourceKey, targetKey); err != nil {
		return err
	}

	guard, err := e.draftLock()
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	op := config.StructuralOp{
		Type:       config.StructuralOpCopyEntry,
		User:       e.session.User,
		Source:     e.session.Origin,
		Time:       e.session.StartTime,
		ParentPath: textbuf.Join(parentPath, " "),
		ListName:   listName,
		OldKey:     sourceKey,
		NewKey:     targetKey,
	}
	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps, err := e.openChangeFile(guard, changePath)
	if err != nil {
		return err
	}
	if err := copyPendingListEntry(changeTree, changeMeta, e.schema, parentPath, listName, sourceKey, targetKey); err != nil {
		return err
	}
	changeOps = append(changeOps, op)
	if err := e.writeChangeFile(guard, changePath, changeTree, changeMeta, changeOps); err != nil {
		return err
	}

	if err := applyStructuralOps(e.tree, e.schema, []config.StructuralOp{op}, false); err != nil {
		return fmt.Errorf("write-through apply: %w", err)
	}
	if metaParent := walkMetaReadOnly(e.meta, e.schema, parentPath); metaParent != nil {
		if err := metaParent.CopyListEntry(listName, sourceKey, targetKey); err != nil {
			return err
		}
	}
	e.dirty.Store(true)
	e.draftSaved = false
	return nil
}

// copyPendingListEntry copies the pending edits a change file holds for a
// list entry, its sparse subtree and its metadata, to the copy's key. A source
// with no pending edit has nothing to copy.
func copyPendingListEntry(tree *config.Tree, meta *config.MetaTree, schema *config.Schema, parentPath []string, listName, sourceKey, targetKey string) error {
	parent := tree
	if len(parentPath) > 0 {
		parent = walkPath(tree, schema, parentPath)
	}
	if parent != nil && parent.GetList(listName)[sourceKey] != nil {
		if err := parent.CopyListEntry(listName, sourceKey, targetKey); err != nil {
			return err
		}
	}
	metaParent := walkMetaReadOnly(meta, schema, parentPath)
	if metaParent == nil {
		return nil
	}
	return metaParent.CopyListEntry(listName, sourceKey, targetKey)
}

// writeThroughToggle records a leaf or path deactivate/activate as one
// structural op. The caller MUST have checked the current state (path found,
// not already in the asked state), so the sentinel errors stay the caller's.
func (e *Editor) writeThroughToggle(opType config.StructuralOpType, parentPath []string, name string) error {
	return e.writeThroughStructuralOp(config.StructuralOp{
		Type:       opType,
		ParentPath: textbuf.Join(parentPath, " "),
		ListName:   name,
	})
}

// writeThroughPathToggle records a container or list-entry deactivate or
// activate. The root is not a node an operator can toggle, so an empty path
// is refused rather than indexed.
func (e *Editor) writeThroughPathToggle(opType config.StructuralOpType, path []string) error {
	if len(path) == 0 {
		return fmt.Errorf("%w: the root cannot be deactivated or activated", ErrPathNotFound)
	}
	return e.writeThroughToggle(opType, path[:len(path)-1], path[len(path)-1])
}

// writeThroughStructuralOp stamps op with the session identity, appends it to
// the per-user change file under the store lock, and then applies it to the
// in-memory tree through the same applyStructuralOps that SaveDraft and the
// commit use, so what the operator sees is what the commit will apply.
func (e *Editor) writeThroughStructuralOp(op config.StructuralOp) error {
	guard, err := e.draftLock()
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	op.User = e.session.User
	op.Source = e.session.Origin
	op.Time = e.session.StartTime

	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps, err := e.openChangeFile(guard, changePath)
	if err != nil {
		return err
	}
	changeOps = append(changeOps, op)
	if err := e.writeChangeFile(guard, changePath, changeTree, changeMeta, changeOps); err != nil {
		return err
	}

	if err := applyStructuralOps(e.tree, e.schema, []config.StructuralOp{op}, false); err != nil {
		return fmt.Errorf("write-through apply: %w", err)
	}
	e.dirty.Store(true)
	e.draftSaved = false
	return nil
}

// openChangeFile answers the session's change file, parsed. During a load it
// answers the load's staged copy, read and parsed once; the step edits it in
// place and records it with writeChangeFile.
func (e *Editor) openChangeFile(guard storage.WriteGuard, changePath string) (*config.Tree, *config.MetaTree, []config.StructuralOp, error) {
	if e.loadStage != nil {
		return e.loadStage.changeFile(e, changePath)
	}
	return e.readChangeFile(guard, changePath)
}

// writeChangeFile serializes and writes the change file a write-through step
// edited. During a load it records the edit in the stage instead, and the load
// writes the file once (loadStage.flush).
func (e *Editor) writeChangeFile(guard storage.WriteGuard, changePath string, tree *config.Tree, meta *config.MetaTree, ops []config.StructuralOp) error {
	if e.loadStage != nil {
		return e.loadStage.record(changePath, tree, meta, ops)
	}
	output := config.SerializeChangeFile(tree, meta, ops, e.schema)
	if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
	}
	return nil
}

// readChangeFile reads and parses a per-user change file.
// A change file that does not exist holds no pending change, so it answers
// empty collections. Any other read failure, and a file that does not parse,
// is an error naming the file: the file holds the user's pending changes, so
// reading it as empty would let a commit answer success with nothing applied
// and let the next edit overwrite it. The file is left in place.
func (e *Editor) readChangeFile(guard storage.WriteGuard, changePath string) (*config.Tree, *config.MetaTree, []config.StructuralOp, error) {
	data, err := guard.ReadFile(changePath)
	if errors.Is(err, fs.ErrNotExist) {
		return config.NewTree(), config.NewMetaTree(), nil, nil
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read change file %s: %w", changePath, err)
	}
	parser := config.NewSetParser(e.schema)
	tree, meta, ops, err := config.ParseChangeFile(string(data), parser)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("change file %s does not parse, pending changes kept in it: %w", changePath, err)
	}
	return tree, meta, ops, nil
}

// SaveDraft applies changes from the per-user change file to config.conf.draft.
// Creates a new draft (base + own changes), then deletes the change file.
func (e *Editor) SaveDraft() error {
	if e.session == nil {
		return errNoSessionSet
	}

	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("save lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock

	// Read the change file.
	changePath := ChangePath(e.originalPath, e.session.User)
	_, changeMeta, changeOps, err := e.readChangeFile(guard, changePath)
	if err != nil {
		return err
	}

	myEntries := changeMeta.SessionEntries(e.session.ID)
	myOps := filterStructuralOps(changeOps, e.session.ID)
	if len(myEntries) == 0 && len(myOps) == 0 {
		return nil // Nothing to save.
	}

	// Read base (draft if exists, else committed).
	draftPath := DraftPath(e.originalPath)
	baseTree, baseMeta, err := e.readDraftOrConfig(guard, draftPath)
	if err != nil {
		return fmt.Errorf("save: %w", err)
	}

	if err := applyStructuralOps(baseTree, e.schema, myOps, true); err != nil {
		return fmt.Errorf("save apply structural ops: %w", err)
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

		if err := e.applySessionEntryToTree(baseTree, parentPath, leafName, se.Entry); err != nil {
			return fmt.Errorf("save apply %s: %w", se.Path, err)
		}

		// Record metadata in draft.
		metaTarget := walkOrCreateMeta(baseMeta, e.schema, parentPath)
		metaTarget.SetEntry(leafName, se.Entry)
	}

	if e.preCommitValidate != nil {
		candidate := config.Serialize(baseTree, e.schema)
		if err := e.preCommitValidate(candidate); err != nil {
			return fmt.Errorf("validation: %w", err)
		}
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
		parentPath := strings.Fields(ops[i].ParentPath)
		switch ops[i].Type {
		case config.StructuralOpRename:
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
		case config.StructuralOpDeleteEntry:
			target := walkPath(tree, schema, parentPath)
			if target == nil {
				if allowAlreadyApplied {
					continue
				}
				return fmt.Errorf("path not found: %s", ops[i].ParentPath)
			}
			target.RemoveListEntry(ops[i].ListName, ops[i].OldKey)
		case config.StructuralOpDeleteContainer, config.StructuralOpDeleteList:
			target := walkPath(tree, schema, parentPath)
			if target == nil {
				if allowAlreadyApplied {
					continue
				}
				return fmt.Errorf("path not found: %s", ops[i].ParentPath)
			}
			if ops[i].Type == config.StructuralOpDeleteList {
				target.DeleteList(ops[i].ListName)
			} else {
				target.DeleteContainer(ops[i].ListName)
			}
		case config.StructuralOpInsertMember, config.StructuralOpDeactivateMember, config.StructuralOpActivateMember:
			if err := applyMemberOp(tree, schema, ops[i], allowAlreadyApplied); err != nil {
				return err
			}
		case config.StructuralOpCopyEntry:
			target := walkPath(tree, schema, parentPath)
			if target == nil {
				return fmt.Errorf("path not found: %s", ops[i].ParentPath)
			}
			if err := target.CopyListEntry(ops[i].ListName, ops[i].OldKey, ops[i].NewKey); err != nil {
				if allowAlreadyApplied && copyAlreadyApplied(target, ops[i].ListName, ops[i].OldKey, ops[i].NewKey) {
					continue
				}
				return err
			}
		case config.StructuralOpDeactivateLeaf, config.StructuralOpActivateLeaf,
			config.StructuralOpDeactivatePath, config.StructuralOpActivatePath:
			if err := applyToggleOp(tree, schema, ops[i], allowAlreadyApplied); err != nil {
				return err
			}
		case "":
			return fmt.Errorf("unsupported structural op %q", ops[i].Type)
		default:
			panic("BUG: invalid structural operation")
		}
	}
	return nil
}

// copyAlreadyApplied reports whether a replayed copy finds both entries in
// place, which is what a draft replay over its own earlier save looks like.
func copyAlreadyApplied(target *config.Tree, listName, sourceKey, targetKey string) bool {
	entries := target.GetList(listName)
	if entries == nil {
		return false
	}
	if entries[sourceKey] == nil {
		return false
	}
	return entries[targetKey] != nil
}

// applyToggleOp applies one leaf or path deactivate/activate op. The desired
// state already reached is success, as for the member toggles: a draft replay
// or a second session asking for the same state changes nothing. A path that
// no longer resolves is an error at commit and skipped on replay.
func applyToggleOp(tree *config.Tree, schema *config.Schema, op config.StructuralOp, allowAlreadyApplied bool) error {
	parentPath := strings.Fields(op.ParentPath)
	if op.Type == config.StructuralOpDeactivateLeaf || op.Type == config.StructuralOpActivateLeaf {
		target := walkPath(tree, schema, parentPath)
		if target == nil {
			if allowAlreadyApplied {
				return nil
			}
			return fmt.Errorf("%w: %s", ErrPathNotFound, op.ParentPath)
		}
		if op.Type == config.StructuralOpDeactivateLeaf {
			target.SetLeafInactive(op.ListName, true)
		} else {
			target.ClearLeafInactive(op.ListName)
		}
		return nil
	}
	target := walkPath(tree, schema, append(parentPath, op.ListName))
	if target == nil {
		if allowAlreadyApplied {
			return nil
		}
		return fmt.Errorf("%w: %s", ErrPathNotFound, op.SourcePath())
	}
	target.SetInactive(op.Type == config.StructuralOpDeactivatePath)
	return nil
}

// applyMemberOp applies one leaf-list member structural op. Desired state
// already reached (member present for insert, already inactive for
// deactivate, already active for activate) is treated as success so draft
// replays and concurrent idempotent sessions do not fail the apply.
func applyMemberOp(tree *config.Tree, schema *config.Schema, op config.StructuralOp, allowAlreadyApplied bool) error {
	parentPath := strings.Fields(op.ParentPath)
	target := walkPath(tree, schema, parentPath)
	if target == nil {
		if allowAlreadyApplied {
			return nil
		}
		return fmt.Errorf("path not found: %s", op.ParentPath)
	}

	present, inactive := target.MultiValueMemberState(op.ListName, op.NewKey)
	switch op.Type {
	case config.StructuralOpInsertMember:
		if present {
			return nil
		}
		if allowAlreadyApplied &&
			(op.Position == config.InsertBefore || op.Position == config.InsertAfter) {
			// Replay onto a base whose reference member was removed (or
			// deactivated) concurrently: keep the member by appending so
			// the draft never loses data. The commit pre-scan
			// (insertRefStaleConflict) surfaces the lost position to the
			// user as a stale conflict.
			if refPresent, refInactive := target.MultiValueMemberState(op.ListName, op.OldKey); !refPresent || refInactive {
				return target.InsertMultiValue(op.ListName, op.NewKey, config.InsertLast, "")
			}
		}
		return target.InsertMultiValue(op.ListName, op.NewKey, op.Position, op.OldKey)
	case config.StructuralOpDeactivateMember:
		if present && inactive {
			return nil
		}
		if !present {
			if allowAlreadyApplied {
				return nil
			}
			return fmt.Errorf("%q not found in %s", op.NewKey, op.ListName)
		}
		return target.DeactivateMultiValue(op.ListName, op.NewKey)
	case config.StructuralOpActivateMember:
		if present && !inactive {
			return nil
		}
		if !present {
			if allowAlreadyApplied {
				return nil
			}
			return fmt.Errorf("%q not found in %s", op.NewKey, op.ListName)
		}
		return target.ActivateMultiValue(op.ListName, op.NewKey)
	case "", config.StructuralOpRename, config.StructuralOpDeleteEntry, config.StructuralOpDeleteContainer, config.StructuralOpDeleteList,
		config.StructuralOpCopyEntry, config.StructuralOpDeactivateLeaf, config.StructuralOpActivateLeaf,
		config.StructuralOpDeactivatePath, config.StructuralOpActivatePath:
		return nil
	default:
		panic("BUG: invalid structural member operation")
	}
}

func applyStructuralOpsToMeta(meta *config.MetaTree, schema *config.Schema, ops []config.StructuralOp, allowAlreadyApplied bool) error {
	if meta == nil {
		return nil
	}
	for i := range ops {
		parentPath := strings.Fields(ops[i].ParentPath)
		switch ops[i].Type {
		case config.StructuralOpRename:
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
		case config.StructuralOpDeleteEntry:
			target := walkMetaReadOnly(meta, schema, parentPath)
			if target == nil {
				continue
			}
			target.DeleteMetaListEntry(ops[i].ListName, ops[i].OldKey)
		case config.StructuralOpDeleteContainer, config.StructuralOpDeleteList:
			target := walkMetaReadOnly(meta, schema, parentPath)
			if target == nil {
				continue
			}
			target.DeleteMetaContainer(ops[i].ListName)
		case config.StructuralOpInsertMember, config.StructuralOpDeactivateMember, config.StructuralOpActivateMember,
			config.StructuralOpDeactivateLeaf, config.StructuralOpActivateLeaf,
			config.StructuralOpDeactivatePath, config.StructuralOpActivatePath:
			// Member ops reorder or toggle values inside one leaf, and the
			// leaf and path toggles set a marker; the metadata tree
			// structure is unaffected.
			continue
		case config.StructuralOpCopyEntry:
			// The copied entry carries no session metadata of its own: the
			// copy op is the one attributed change.
			continue
		case "":
			return fmt.Errorf("unsupported structural op %q", ops[i].Type)
		default:
			panic("BUG: invalid structural metadata operation")
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
		tree, meta, err = parseConfigLenient(string(data), e.schema)
		if err != nil {
			return false
		}
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

// detectConflicts scans pending changes from other sessions and reports live overlaps.
func (e *Editor) detectConflicts() []Conflict {
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
					MyValue:    pendingConflictValue(e.schema, mine),
					OtherValue: pendingConflictValue(e.schema, other),
					OtherUser:  otherUser,
				})
			}
		}
	}

	return conflicts
}

func pendingChangesConflict(a, b config.PendingChange) bool {
	if !isEntryMove(a.Kind) && !isEntryMove(b.Kind) {
		if a.Path != b.Path {
			return false
		}
		if a.Member != "" || b.Member != "" {
			if a.Member != "" && b.Member != "" {
				// Leaf-list members are independent changes: two sessions
				// touching different members never conflict; the same member
				// conflicts only when one session sets it and the other
				// deletes it (Value empty = delete intent).
				return a.Member == b.Member && a.Value != b.Value
			}
			// A member operation against a whole-leaf operation (delete of
			// the entire leaf-list) on the same path always conflicts.
			return true
		}
		// A different kind on one path conflicts even with equal values:
		// deactivate and activate of one leaf both carry no value.
		return a.Kind != b.Kind || a.Value != b.Value
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
	for _, other := range e.pendingChanges(guard, "") {
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

// pendingConflictValue answers the text a conflict report may publish for one
// side of the overlap. Both sides read it, so the value another user typed
// reaches this operator's terminal through OtherValue, and the schema is what
// keeps a credential out of both.
func pendingConflictValue(schema *config.Schema, change config.PendingChange) string {
	if isEntryMove(change.Kind) {
		return change.Summary(schema)
	}
	return config.DisplayValueAtPath(schema, strings.Fields(change.Path), change.Value)
}

// isEntryMove reports whether a pending change takes a list entry from one
// path to another (rename, copy), so it conflicts by path overlap on both.
func isEntryMove(kind config.PendingChangeKind) bool {
	return kind == config.PendingChangeRename || kind == config.PendingChangeCopy
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
func (e *Editor) readDraftOrConfig(guard storage.WriteGuard, draftPath string) (*config.Tree, *config.MetaTree, error) {
	data, err := guard.ReadFile(draftPath)
	if errors.Is(err, fs.ErrNotExist) {
		// No draft: clone the in-memory tree and start with empty metadata.
		return e.tree.Clone(), config.NewMetaTree(), nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read draft %s: %w", draftPath, err)
	}
	// A draft that does not parse holds other sessions' saved changes, so
	// replacing it with this session's tree would drop them.
	tree, meta, err := config.NewSetParser(e.schema).ParseWithMeta(string(data))
	if err != nil {
		return nil, nil, fmt.Errorf("draft %s does not parse: %w", draftPath, err)
	}
	return tree, meta, nil
}

// readCommittedTree reads and parses config.conf under lock.
// Re-reads each time to capture external commits between write-through calls.
// A load holds the lock for all its steps, so no commit lands between them,
// and it reads the file once (loadStage.committedTree).
// Returns nil if the file cannot be read or parsed.
func (e *Editor) readCommittedTree(guard storage.WriteGuard) *config.Tree {
	if e.loadStage != nil {
		return e.loadStage.committedTree(e)
	}
	return e.readCommittedTreeFrom(guard)
}

// readCommittedTreeFrom reads and parses the committed config through guard,
// answering nil when it does not read or parse.
func (e *Editor) readCommittedTreeFrom(guard storage.WriteGuard) *config.Tree {
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
		return errNoSessionSet
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
	changeTree, changeMeta, changeOps, err := e.readChangeFile(guard, changePath)
	if err != nil {
		return err
	}
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
			Member:   se.Entry.Member,
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

// checkDraftChanged checks if the draft file has been modified by another session.
// Uses Storage.Stat for both filesystem (OS mtime) and blob (tracked mtime).
// Returns true if the draft mtime is newer than the last known mtime.
// Also re-reads and re-parses the draft on change to update in-memory state.
func (e *Editor) checkDraftChanged() (changed bool, notification string) {
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

	var tb textbuf.Buffer
	tb.Str("Draft updated by another session")
	if meta.ModifiedBy != "" {
		tb.Str(" (").Str(meta.ModifiedBy).Byte(')')
	}
	return true, tb.String()
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
	default:
		panic("BUG: invalid detected config format")
	}
}

// parseConfigLenient retries parsing with unknown fields skipped.
// Used when strict parsing fails so the editor can still show a tree view.
func parseConfigLenient(content string, schema *config.Schema) (*config.Tree, *config.MetaTree, error) {
	format := config.DetectFormat(content)

	switch format {
	case config.FormatSetMeta:
		sp := config.NewSetParser(schema)
		sp.SetPreMigration(true)
		return sp.ParseWithMeta(content)
	case config.FormatSet:
		sp := config.NewSetParser(schema)
		sp.SetPreMigration(true)
		tree, err := sp.Parse(content)
		return tree, config.NewMetaTree(), err
	case config.FormatHierarchical:
		tree, err := config.NewParser(schema).Parse(content)
		return tree, config.NewMetaTree(), err
	default:
		panic("BUG: invalid detected config format")
	}
}
