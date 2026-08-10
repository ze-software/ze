// Design: docs/architecture/config/yang-config-design.md — config editor write operations
// Overview: editor.go — editor state and lifecycle
package cli

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errTreeOrSchemaNotAvailable            = errors.New("tree or schema not available")
	errPathNotFound                        = errors.New("path not found")
	errEmptyPath                           = errors.New("empty path")
	errSchemaNotAvailable                  = errors.New("schema not available")
	errCopyNotSupportedInSessionMode       = errors.New("copy not supported in session mode")
	errDeactivateNotSupportedInSessionMode = errors.New("deactivate not supported in session mode")
	errActivateNotSupportedInSessionMode   = errors.New("activate not supported in session mode")
	errPathTooShortForListEntry            = errors.New("path too short for list entry")
	errCannotRenameAnonymousListEntry      = errors.New("cannot rename anonymous list entry")
	errPathDoesNotEndAtA                   = errors.New("path does not end at a list entry")
	errRenameTargetMustBeTheLast           = errors.New("rename target must be the last element in the path")
	errSaveNotAllowedWithActiveSession     = errors.New("Save() not allowed with active session; use CommitSession()")
	errNoSessionSet                        = errors.New("no session set")
	errLoadNotSupportedInSessionMode       = errors.New("load not supported in session mode")
)

// saveEditState saves the current working content to the .edit file.
func (e *Editor) saveEditState() error {
	if !e.dirty.Load() {
		return nil // Nothing to save
	}

	var tb textbuf.Buffer
	editPath := tb.Str(e.originalPath).Str(".edit").String()
	if err := e.store.WriteFile(editPath, []byte(e.workingContent), 0o600); err != nil {
		return fmt.Errorf("failed to write edit file: %w", err)
	}
	return nil
}

// deleteEditFile removes the .edit file if it exists.
// MUST NOT be called while a WriteGuard is held: e.store.Remove re-acquires the
// store mutex and self-deadlocks. Use deleteEditFileGuard inside a guarded section.
func (e *Editor) deleteEditFile() {
	var tb textbuf.Buffer
	editPath := tb.Str(e.originalPath).Str(".edit").String()
	_ = e.store.Remove(editPath) // Ignore error if doesn't exist
}

// deleteEditFileGuard removes the .edit file through an already-held write guard.
// CommitSession calls this while holding the store lock; routing the removal
// through the guard avoids the self-deadlock that e.store.Remove would cause.
func (e *Editor) deleteEditFileGuard(guard storage.WriteGuard) {
	var tb textbuf.Buffer
	editPath := tb.Str(e.originalPath).Str(".edit").String()
	guard.Remove(editPath) //nolint:errcheck // Best effort; ignore error if it doesn't exist
}

// setWorkingContent sets the working content and parses it into the tree.
// If parsing fails, falls back to raw text mode (treeValid = false).
func (e *Editor) setWorkingContent(content string) {
	e.workingContent = content
	if e.schema != nil {
		parser := config.NewParser(e.schema)
		tree, err := parser.Parse(content)
		if err == nil {
			e.tree = tree
			e.treeValid = true
		} else {
			e.treeValid = false
		}
	}
}

// walkOrCreate navigates the tree, creating containers along the way.
// Supports anonymous list entries (KeyDefault) for interactive editing.
// See walkOrCreateIn in editor_draft.go for the write-through variant
// that requires explicit list keys (used with full set-command paths).
func (e *Editor) walkOrCreate(path []string) (*config.Tree, error) {
	if e.tree == nil || e.schema == nil {
		return nil, errTreeOrSchemaNotAvailable
	}
	if len(path) == 0 {
		return e.tree, nil
	}

	currentTree := e.tree
	var currentSchema schemaGetter = e.schema

	i := 0
	for i < len(path) {
		name := path[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			return nil, fmt.Errorf("unknown path element: %s", name)
		}

		switch n := schemaNode.(type) {
		case *config.ContainerNode:
			currentTree = currentTree.GetOrCreateContainer(name)
			currentSchema = n
			i++
		case *config.ListNode:
			// Determine anonymous vs keyed: anonymous if no next element
			// or next element is a schema child of the list.
			var key string
			var step int
			if i+1 >= len(path) || n.Get(path[i+1]) != nil {
				key = config.KeyDefault
				step = 1
			} else {
				key = path[i+1]
				step = 2
			}
			entries := currentTree.GetList(name)
			if entries == nil || entries[key] == nil {
				entry := config.NewTree()
				currentTree.AddListEntry(name, key, entry)
				currentTree = entry
			} else {
				currentTree = entries[key]
			}
			currentSchema = n
			i += step
		case *config.FlexNode:
			currentTree = currentTree.GetOrCreateContainer(name)
			currentSchema = n
			i++
		case *config.InlineListNode:
			// Inline lists use the same key navigation as regular lists.
			var key string
			var step int
			if i+1 >= len(path) || n.Get(path[i+1]) != nil {
				key = config.KeyDefault
				step = 1
			} else {
				key = path[i+1]
				step = 2
			}
			entries := currentTree.GetList(name)
			if entries == nil || entries[key] == nil {
				entry := config.NewTree()
				currentTree.AddListEntry(name, key, entry)
				currentTree = entry
			} else {
				currentTree = entries[key]
			}
			currentSchema = n
			i += step
		case *config.LeafNode, *config.FreeformNode,
			*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
			return nil, fmt.Errorf("cannot navigate into %s (leaf node)", name)
		}
	}

	return currentTree, nil
}

// CreateEntry creates an empty list entry at the given path.
// The path must end at a list entry (e.g., ["bgp", "peer", "london"]).
// If the entry already exists, this is a no-op.
func (e *Editor) CreateEntry(path []string) error {
	if e.session != nil {
		return e.writeThroughCreate(path)
	}
	_, err := e.walkOrCreate(path)
	if err != nil {
		return err
	}
	e.dirty.Store(true)
	return nil
}

// SetValue sets a leaf value at the given path in the tree.
// Plain leaf-lists (ValueOrArrayNode) get JunOS add-member semantics: each
// set appends one member (idempotently); it never replaces the list. The
// member lands in the multi-value store — the store every serializer reads —
// not the scalar map, where it would be silently dropped.
func (e *Editor) SetValue(path []string, key, value string) error {
	if e.isValueOrArrayLeaf(path, key) {
		return e.setLeafListMember(path, key, value)
	}
	if e.session != nil {
		return e.writeThroughSet(path, key, value)
	}
	target, err := e.walkOrCreate(path)
	if err != nil {
		return err
	}
	target.Set(key, value)
	e.dirty.Store(true)
	return nil
}

// isValueOrArrayLeaf reports whether key under path is a plain leaf-list
// (ValueOrArrayNode). Bracket and multi-word leaf kinds keep scalar
// semantics.
func (e *Editor) isValueOrArrayLeaf(path []string, key string) bool {
	if e.schema == nil {
		return false
	}
	parentSchema := e.walkSchema(path)
	if parentSchema == nil {
		return false
	}
	_, ok := parentSchema.Get(key).(*config.ValueOrArrayNode)
	return ok
}

// setLeafListMember adds one member to a leaf-list (add-member semantics).
func (e *Editor) setLeafListMember(path []string, key, member string) error {
	if e.session != nil {
		return e.writeThroughSetMember(path, key, member)
	}
	target, err := e.walkOrCreate(path)
	if err != nil {
		return err
	}
	target.AddMultiValueMember(key, member)
	e.dirty.Store(true)
	return nil
}

// deleteLeafListValue removes one member from a leaf-list (the inverse of
// add-member set). Returns an error if the member is not present.
func (e *Editor) deleteLeafListValue(path []string, leafListName, value string) error {
	if e.session != nil {
		return e.writeThroughDeleteMember(path, leafListName, value)
	}
	var target *config.Tree
	if len(path) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(path)
	}
	if target == nil {
		return errPathNotFound
	}
	if !target.RemoveMultiValueMember(leafListName, value) {
		return fmt.Errorf("%q not found in %s", value, leafListName)
	}
	e.dirty.Store(true)
	return nil
}

// DeleteValue removes a leaf value at the given path in the tree.
func (e *Editor) DeleteValue(path []string, key string) error {
	if e.session != nil {
		return e.writeThroughDelete(path, key)
	}
	target := e.WalkPath(path)
	if target == nil {
		return errPathNotFound
	}
	target.Delete(key)
	e.dirty.Store(true)
	return nil
}

// DeleteContainer removes a container at the given path in the tree.
func (e *Editor) DeleteContainer(path []string, name string) error {
	if e.session != nil {
		return e.writeThroughDeleteContainer(path, name)
	}
	var target *config.Tree
	if len(path) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(path)
	}
	if target == nil {
		return errPathNotFound
	}
	target.DeleteContainer(name)
	e.dirty.Store(true)
	return nil
}

// DeleteByPath deletes the element at the given absolute path using schema awareness.
// It determines whether the target is a leaf, container, or list entry and calls
// the appropriate delete method.
func (e *Editor) DeleteByPath(fullPath []string) error {
	if len(fullPath) == 0 {
		return errEmptyPath
	}
	if e.schema == nil {
		return errSchemaNotAvailable
	}

	// Walk the schema to find what the second-to-last element is.
	// If it's a ListNode, the last element is a key → DeleteListEntry.
	// Otherwise, last element is a leaf or container name.
	if len(fullPath) >= 2 {
		possibleListName := fullPath[len(fullPath)-2]
		possibleKey := fullPath[len(fullPath)-1]
		parentPath := fullPath[:len(fullPath)-2]

		// Walk schema to the parent of possibleListName
		parentSchema := e.walkSchema(parentPath)
		if parentSchema != nil {
			schemaNode := parentSchema.Get(possibleListName)
			if _, isList := schemaNode.(*config.ListNode); isList {
				return e.DeleteListEntry(parentPath, possibleListName, possibleKey)
			}
		}
	}

	// Leaf-list member: path ends `<leaf-list> <member>` — delete one member.
	if len(fullPath) >= 2 {
		memberParent := fullPath[:len(fullPath)-2]
		leafName := fullPath[len(fullPath)-2]
		if e.isValueOrArrayLeaf(memberParent, leafName) {
			return e.deleteLeafListValue(memberParent, leafName, fullPath[len(fullPath)-1])
		}
	}

	// Not a list entry: resolve the target node from schema so session mode
	// preserves container deletes as structural ops instead of misrecording them
	// as leaf deletes.
	target := fullPath[len(fullPath)-1]
	parentPath := fullPath[:len(fullPath)-1]
	parentSchema := e.walkSchema(parentPath)
	if parentSchema != nil {
		switch parentSchema.Get(target).(type) {
		case *config.LeafNode, *config.FreeformNode,
			*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
			return e.DeleteValue(parentPath, target)
		case *config.ContainerNode, *config.FlexNode:
			return e.DeleteContainer(parentPath, target)
		case *config.ListNode, *config.InlineListNode:
			return e.DeleteList(parentPath, target)
		}
	}

	if err := e.DeleteValue(parentPath, target); err != nil {
		if errC := e.DeleteContainer(parentPath, target); errC != nil {
			return fmt.Errorf("not found: %s", textbuf.Join(fullPath, " "))
		}
	}
	return nil
}

// walkSchema walks the schema tree along the given path, returning the schema node
// at the end of the path (or nil if any element is not found or not navigable).
func (e *Editor) walkSchema(path []string) schemaGetter {
	var current schemaGetter = e.schema
	for i := 0; i < len(path); i++ {
		name := path[i]
		node := current.Get(name)
		if node == nil {
			return nil
		}
		switch n := node.(type) {
		case *config.ContainerNode:
			current = n
		case *config.ListNode:
			current = n
			if i+1 < len(path) && n.Get(path[i+1]) == nil {
				i++
			}
		case *config.FlexNode:
			current = n
		case *config.InlineListNode:
			current = n
			if i+1 < len(path) && n.Get(path[i+1]) == nil {
				i++
			}
		case *config.LeafNode, *config.FreeformNode,
			*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
			return nil // Can't navigate into leaf nodes
		}
	}
	return current
}

// isListKeyLeafPath checks if the last two path elements are a list name
// and its key leaf keyword. Uses the config schema which flattens choice/case.
func (e *Editor) isListKeyLeafPath(path []string) bool {
	if e.schema == nil || len(path) < 2 {
		return false
	}
	keyLeafName := path[len(path)-1]
	listName := path[len(path)-2]

	// Walk the schema to the parent of the list, skipping list keys.
	var current schemaGetter = e.schema
	for i := 0; i < len(path)-2; i++ {
		name := path[i]
		node := current.Get(name)
		if node == nil {
			return false
		}
		switch n := node.(type) {
		case *config.ContainerNode:
			current = n
		case *config.ListNode:
			current = n
			if i+1 < len(path)-2 && n.Get(path[i+1]) == nil {
				i++
			}
		case *config.FlexNode:
			current = n
		default:
			return false
		}
	}

	node := current.Get(listName)
	listNode, ok := node.(*config.ListNode)
	if !ok {
		return false
	}
	return listNode.KeyName == keyLeafName
}

// ensureListEntry creates a list entry if it does not already exist.
// Validates the key value against the list's YANG key type.
func (e *Editor) ensureListEntry(parentPath []string, listName, key string) error {
	if e.schema != nil {
		listNode := e.resolveListNode(parentPath, listName)
		if listNode != nil {
			if err := config.ValidateListKey(listNode, key); err != nil {
				return fmt.Errorf("invalid %s key %q: %w", listName, key, err)
			}
		}
	}
	fullPath := make([]string, 0, len(parentPath)+2)
	fullPath = append(fullPath, parentPath...)
	fullPath = append(fullPath, listName, key)
	if e.session != nil {
		return e.writeThroughCreate(fullPath)
	}
	target, err := e.walkOrCreate(parentPath)
	if err != nil {
		return err
	}
	entries := target.GetList(listName)
	if entries != nil && entries[key] != nil {
		return nil
	}
	target.AddListEntry(listName, key, config.NewTree())
	e.dirty.Store(true)
	return nil
}

// resolveListNode walks the config schema to find the ListNode for the given
// parent path and list name, skipping list keys along the way.
func (e *Editor) resolveListNode(parentPath []string, listName string) *config.ListNode {
	var current schemaGetter = e.schema
	for i := 0; i < len(parentPath); i++ {
		node := current.Get(parentPath[i])
		if node == nil {
			return nil
		}
		switch n := node.(type) {
		case *config.ContainerNode:
			current = n
		case *config.ListNode:
			current = n
			if i+1 < len(parentPath) && n.Get(parentPath[i+1]) == nil {
				i++
			}
		case *config.FlexNode:
			current = n
		default:
			return nil
		}
	}
	node := current.Get(listName)
	listNode, ok := node.(*config.ListNode)
	if !ok {
		return nil
	}
	return listNode
}

// DeleteListEntry removes a list entry at the given path in the tree.
func (e *Editor) DeleteListEntry(path []string, listName, key string) error {
	if e.session != nil {
		return e.writeThroughDeleteListEntry(path, listName, key)
	}
	var target *config.Tree
	if len(path) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(path)
	}
	if target == nil {
		return errPathNotFound
	}
	target.RemoveListEntry(listName, key)
	e.dirty.Store(true)
	return nil
}

// writeThroughDeleteListEntry records a list-entry deletion as a structural op
// in the per-user change file and removes the entry from the in-memory tree.
func (e *Editor) writeThroughDeleteListEntry(parentPath []string, listName, key string) error {
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps := e.readChangeFile(guard, changePath)

	op := config.StructuralOp{
		Type:       config.StructuralOpDeleteEntry,
		User:       e.session.User,
		Source:     e.session.Origin,
		Time:       e.session.StartTime,
		ParentPath: textbuf.Join(parentPath, " "),
		ListName:   listName,
		OldKey:     key,
	}
	changeOps = append(changeOps, op)

	var changeTarget *config.Tree
	if len(parentPath) == 0 {
		changeTarget = changeTree
	} else {
		changeTarget = walkPath(changeTree, e.schema, parentPath)
	}
	if changeTarget != nil {
		changeTarget.RemoveListEntry(listName, key)
	}

	output := config.SerializeChangeFile(changeTree, changeMeta, changeOps, e.schema)
	if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
	}

	var target *config.Tree
	if len(parentPath) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(parentPath)
	}
	if target == nil {
		return errPathNotFound
	}
	target.RemoveListEntry(listName, key)

	e.dirty.Store(true)
	e.draftSaved = false
	return nil
}

// writeThroughDeleteContainer records a container deletion as a structural op
// in the per-user change file and removes the container from the in-memory tree.
func (e *Editor) writeThroughDeleteContainer(parentPath []string, containerName string) error {
	return e.writeThroughDeleteNamed(parentPath, containerName, config.StructuralOpDeleteContainer, (*config.Tree).DeleteContainer)
}

// DeleteList removes an entire list (all entries) at the given path.
func (e *Editor) DeleteList(path []string, name string) error {
	if e.session != nil {
		return e.writeThroughDeleteNamed(path, name, config.StructuralOpDeleteList, (*config.Tree).DeleteList)
	}
	var target *config.Tree
	if len(path) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(path)
	}
	if target == nil {
		return errPathNotFound
	}
	target.DeleteList(name)
	e.dirty.Store(true)
	return nil
}

// writeThroughDeleteNamed records a named-child deletion (container or list)
// as a structural op in the per-user change file and removes the child from
// the in-memory tree.
func (e *Editor) writeThroughDeleteNamed(parentPath []string, name string, opType config.StructuralOpType, remove func(*config.Tree, string)) error {
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	changePath := ChangePath(e.originalPath, e.session.User)
	changeTree, changeMeta, changeOps := e.readChangeFile(guard, changePath)

	op := config.StructuralOp{
		Type:       opType,
		User:       e.session.User,
		Source:     e.session.Origin,
		Time:       e.session.StartTime,
		ParentPath: textbuf.Join(parentPath, " "),
		ListName:   name,
	}
	changeOps = append(changeOps, op)

	var changeTarget *config.Tree
	if len(parentPath) == 0 {
		changeTarget = changeTree
	} else {
		changeTarget = walkPath(changeTree, e.schema, parentPath)
	}
	if changeTarget != nil {
		remove(changeTarget, name)
	}

	output := config.SerializeChangeFile(changeTree, changeMeta, changeOps, e.schema)
	if err := guard.WriteFile(changePath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
	}

	var target *config.Tree
	if len(parentPath) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(parentPath)
	}
	if target == nil {
		return errPathNotFound
	}
	remove(target, name)

	e.dirty.Store(true)
	e.draftSaved = false
	return nil
}

// RenameListEntry renames a list entry key at the given path.
// The parentPath navigates to the tree containing the list.
// In session mode, the rename is recorded as a structural op in the per-user
// change file and the in-memory tree/meta are updated immediately.
func (e *Editor) RenameListEntry(parentPath []string, listName, oldKey, newKey string) error {
	if e.session != nil {
		return e.writeThroughRename(parentPath, listName, oldKey, newKey)
	}
	var target *config.Tree
	if len(parentPath) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(parentPath)
	}
	if target == nil {
		return errPathNotFound
	}
	if err := target.RenameListEntry(listName, oldKey, newKey); err != nil {
		return err
	}
	e.dirty.Store(true)
	return nil
}

// CopyListEntry clones a list entry under a new key at the given path.
// The parentPath navigates to the tree containing the list.
// MetaTree is not updated because copy is blocked in session mode (meta is session-only).
func (e *Editor) CopyListEntry(parentPath []string, listName, srcKey, dstKey string) error {
	if e.session != nil {
		return errCopyNotSupportedInSessionMode
	}
	var target *config.Tree
	if len(parentPath) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(parentPath)
	}
	if target == nil {
		return errPathNotFound
	}
	if err := target.CopyListEntry(listName, srcKey, dstKey); err != nil {
		return err
	}
	e.dirty.Store(true)
	return nil
}

// InsertLeafListValue inserts a value into a leaf-list at the specified position.
// path navigates to the container holding the leaf-list. leafListName is the
// leaf-list field name. position is first/last/before/after, ref is the
// reference value for before/after.
func (e *Editor) InsertLeafListValue(path []string, leafListName, value, position, ref string) error {
	if e.session != nil {
		return e.writeThroughMemberOp(path, config.StructuralOpInsertMember, leafListName, value, position, ref)
	}
	var target *config.Tree
	if len(path) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(path)
	}
	if target == nil {
		return errPathNotFound
	}
	if err := target.InsertMultiValue(leafListName, value, position, ref); err != nil {
		return err
	}
	e.dirty.Store(true)
	return nil
}

// DeactivateLeafListValue marks a leaf-list member deactivated (out-of-band).
func (e *Editor) DeactivateLeafListValue(path []string, leafListName, value string) error {
	if e.session != nil {
		return e.writeThroughMemberOp(path, config.StructuralOpDeactivateMember, leafListName, value, "", "")
	}
	var target *config.Tree
	if len(path) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(path)
	}
	if target == nil {
		return errPathNotFound
	}
	if err := target.DeactivateMultiValue(leafListName, value); err != nil {
		return err
	}
	e.dirty.Store(true)
	return nil
}

// ActivateLeafListValue clears a leaf-list member's deactivation marker.
func (e *Editor) ActivateLeafListValue(path []string, leafListName, value string) error {
	if e.session != nil {
		return e.writeThroughMemberOp(path, config.StructuralOpActivateMember, leafListName, value, "", "")
	}
	var target *config.Tree
	if len(path) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(path)
	}
	if target == nil {
		return errPathNotFound
	}
	if err := target.ActivateMultiValue(leafListName, value); err != nil {
		return err
	}
	e.dirty.Store(true)
	return nil
}

// Sentinel errors returned by the leaf and path deactivation helpers.
// Callers (the CLI verb, the TUI command) use errors.Is to distinguish
// "no change" from real failures so they can decide whether to surface
// or swallow the result.
var (
	ErrLeafAlreadyInactive = errors.New("leaf already inactive")
	ErrLeafNotInactive     = errors.New("leaf is not inactive")
	ErrPathAlreadyInactive = errors.New("path already inactive")
	ErrPathNotInactive     = errors.New("path is not inactive")
	ErrPathNotFound        = errors.New("path not found")
)

// DeactivateLeaf marks a leaf inactive on the tree at parentPath. The
// leaf value (if any) is preserved verbatim; PruneInactive removes the
// entry at apply time so consumers see it as absent. Permissive on
// absent leaves -- pre-marking before set is allowed, matching the
// Tree.SetLeafInactive contract; this is what lets a leaf with a YANG
// default be deactivated without a prior explicit set.
//
// Returns ErrLeafAlreadyInactive (wrapped) when the leaf is already
// marked, so callers can use errors.Is for idempotent flows.
func (e *Editor) DeactivateLeaf(parentPath []string, leafName string) error {
	if e.session != nil {
		return errDeactivateNotSupportedInSessionMode
	}
	target := e.tree
	if len(parentPath) > 0 {
		target = e.WalkPath(parentPath)
	}
	if target == nil {
		return fmt.Errorf("%w: %s", ErrPathNotFound, textbuf.Join(parentPath, " "))
	}
	if target.IsLeafInactive(leafName) {
		return fmt.Errorf("%w: %q", ErrLeafAlreadyInactive, leafName)
	}
	target.SetLeafInactive(leafName, true)
	e.dirty.Store(true)
	return nil
}

// ActivateLeaf clears the inactive marker on a leaf at parentPath.
// Returns ErrLeafNotInactive (wrapped) when the leaf is already active.
func (e *Editor) ActivateLeaf(parentPath []string, leafName string) error {
	if e.session != nil {
		return errActivateNotSupportedInSessionMode
	}
	target := e.tree
	if len(parentPath) > 0 {
		target = e.WalkPath(parentPath)
	}
	if target == nil {
		return fmt.Errorf("%w: %s", ErrPathNotFound, textbuf.Join(parentPath, " "))
	}
	if !target.IsLeafInactive(leafName) {
		return fmt.Errorf("%w: %q", ErrLeafNotInactive, leafName)
	}
	target.ClearLeafInactive(leafName)
	e.dirty.Store(true)
	return nil
}

// DeactivatePath sets the inactive flag on
// the container or list entry at path. Strict on path resolution: it
// rejects non-existent paths rather than silently materializing them
// (which is what plain SetValue + walkOrCreate would do).
//
// Returns ErrPathNotFound when the path does not resolve in the tree,
// and ErrPathAlreadyInactive (wrapped) when the inactive flag is
// already set, so callers can use errors.Is for idempotent flows.
func (e *Editor) DeactivatePath(path []string) error {
	if e.session != nil {
		return errDeactivateNotSupportedInSessionMode
	}
	target := e.WalkPath(path)
	if target == nil {
		return fmt.Errorf("%w: %s", ErrPathNotFound, textbuf.Join(path, " "))
	}
	if target.IsInactive() {
		return fmt.Errorf("%w: %s", ErrPathAlreadyInactive, textbuf.Join(path, " "))
	}
	target.SetInactive(true)
	e.dirty.Store(true)
	return nil
}

// ActivatePath clears the inactive flag on the container or list entry
// at path. Strict on path resolution.
//
// Returns ErrPathNotFound or ErrPathNotInactive (wrapped) for the
// idempotent / mistyped-path cases.
func (e *Editor) ActivatePath(path []string) error {
	if e.session != nil {
		return errActivateNotSupportedInSessionMode
	}
	target := e.WalkPath(path)
	if target == nil {
		return fmt.Errorf("%w: %s", ErrPathNotFound, textbuf.Join(path, " "))
	}
	if !target.IsInactive() {
		return fmt.Errorf("%w: %s", ErrPathNotInactive, textbuf.Join(path, " "))
	}
	target.SetInactive(false)
	e.dirty.Store(true)
	return nil
}

// resolveListTarget walks the schema-aware path and identifies the terminal
// list entry. Returns the tree-level parent path (for WalkPath), the list name,
// and the entry key. Returns an error if the path does not end at a list entry.
func (e *Editor) resolveListTarget(fullPath []string) (parentPath []string, listName, key string, err error) {
	if e.schema == nil {
		return nil, "", "", errSchemaNotAvailable
	}
	if len(fullPath) < 2 {
		return nil, "", "", errPathTooShortForListEntry
	}

	var currentSchema schemaGetter = e.schema
	lastListIdx := -1
	var lastListName, lastKey string

	i := 0
	for i < len(fullPath) {
		name := fullPath[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			return nil, "", "", fmt.Errorf("unknown path element: %s", name)
		}

		switch n := schemaNode.(type) {
		case *config.ContainerNode:
			currentSchema = n
			i++
		case *config.ListNode:
			if i+1 >= len(fullPath) {
				return nil, "", "", fmt.Errorf("list %s requires a key", name)
			}
			// Check if next element is a child (anonymous) or a key
			if n.Get(fullPath[i+1]) != nil {
				return nil, "", "", errCannotRenameAnonymousListEntry
			}
			lastListIdx = i
			lastListName = name
			lastKey = fullPath[i+1]
			currentSchema = n
			i += 2
		case *config.FlexNode:
			currentSchema = n
			i++
		default:
			return nil, "", "", fmt.Errorf("cannot navigate into %s", name)
		}
	}

	if lastListIdx == -1 {
		return nil, "", "", errPathDoesNotEndAtA
	}

	// The last list entry must be at the end of the path
	if lastListIdx+2 != len(fullPath) {
		return nil, "", "", errRenameTargetMustBeTheLast
	}

	return fullPath[:lastListIdx], lastListName, lastKey, nil
}

// Save commits changes: creates backup of original, writes serialized tree.
// Returns an error when a session is active -- use CommitSession() instead.
func (e *Editor) Save() error {
	if e.session != nil {
		return errSaveNotAllowedWithActiveSession
	}

	// stdin-sourced editor ("-"): emit the working config to the stdout sink
	// instead of writing a file. Emit even when unchanged so the command stays a
	// well-formed pipeline stage; no backup, no store write, no reload.
	if e.stdoutSink != nil {
		content, err := e.commitContent()
		if err != nil {
			return err
		}
		if _, err := io.WriteString(e.stdoutSink, content); err != nil {
			return fmt.Errorf("failed to write config to stdout: %w", err)
		}
		return nil
	}

	if !e.dirty.Load() {
		return nil
	}

	content, err := e.commitContent()
	if err != nil {
		return err
	}

	// Create backup of original
	if err := e.createBackup(e.originalContent, nil); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Write serialized tree (or raw text fallback) to original path
	if err := e.store.WriteFile(e.originalPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	e.MarkCommittedContent(content)

	return nil
}

// StageCandidate writes the current working config as a timestamped candidate.
// It does not update the editor's committed state; callers do that only after
// the daemon promotes the candidate.
func (e *Editor) StageCandidate(stamp time.Time) (string, string, error) {
	if e.session != nil {
		return "", "", errSaveNotAllowedWithActiveSession
	}
	content, err := e.commitContent()
	if err != nil {
		return "", "", err
	}
	stampStr, err := storage.WriteCandidateVersion(e.store, e.originalPath, []byte(content), stamp)
	if err != nil {
		return "", "", fmt.Errorf("failed to write candidate: %w", err)
	}
	return content, stampStr, nil
}

// MarkCommittedContent updates editor state after the daemon has promoted a candidate.
func (e *Editor) MarkCommittedContent(content string) {
	e.cleanupCommittedSession()
	e.originalContent = content
	e.workingContent = content
	if e.schema != nil {
		if tree, meta, err := parseConfigWithFormat(content, e.schema); err == nil {
			e.tree = tree
			e.meta = meta
			e.treeValid = true
		} else {
			e.treeValid = false
		}
	}
	e.dirty.Store(false)
	e.deleteEditFile()
}

func (e *Editor) commitContent() (string, error) {
	// Hash any plaintext-password siblings of ze:bcrypt leaves before
	// serialization. Mirrors the commit-time hashing done in CommitSession.
	if e.treeValid && e.tree != nil && e.schema != nil {
		if err := config.RejectMaskedBcryptLeaves(e.tree, e.schema); err != nil {
			return "", err
		}
		if err := config.ApplyPasswordHashing(e.tree, e.schema); err != nil {
			return "", fmt.Errorf("hash password: %w", err)
		}
	}
	return config.FormatSchemaStamp() + e.WorkingContent(), nil
}

// RestoreOriginalContent writes the previous committed content back to disk
// after a failed runtime reload while keeping the current candidate in memory.
func (e *Editor) RestoreOriginalContent(content string) error {
	if err := e.store.WriteFile(e.originalPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to restore config: %w", err)
	}
	e.originalContent = content
	e.dirty.Store(true)
	return nil
}

// Discard reverts working content and tree to original state.
func (e *Editor) Discard() error {
	e.workingContent = e.originalContent
	e.dirty.Store(false)

	// Re-parse original content into tree
	if e.schema != nil {
		parser := config.NewParser(e.schema)
		tree, err := parser.Parse(e.originalContent)
		if err == nil {
			e.tree = tree
			e.treeValid = true
		}
	}

	// Delete edit file on discard
	e.deleteEditFile()

	return nil
}
