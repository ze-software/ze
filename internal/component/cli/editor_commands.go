// Design: docs/architecture/config/yang-config-design.md — config editor write operations
// Overview: editor.go — editor state and lifecycle
package cli

import (
	"errors"
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// SaveEditState saves the current working content to the .edit file.
func (e *Editor) SaveEditState() error {
	if !e.dirty.Load() {
		return nil // Nothing to save
	}

	editPath := e.originalPath + ".edit"
	if err := e.store.WriteFile(editPath, []byte(e.workingContent), 0o600); err != nil {
		return fmt.Errorf("failed to write edit file: %w", err)
	}
	return nil
}

// deleteEditFile removes the .edit file if it exists.
func (e *Editor) deleteEditFile() {
	editPath := e.originalPath + ".edit"
	_ = e.store.Remove(editPath) // Ignore error if doesn't exist
}

// SetWorkingContent sets the working content and parses it into the tree.
// If parsing fails, falls back to raw text mode (treeValid = false).
func (e *Editor) SetWorkingContent(content string) {
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
		return nil, fmt.Errorf("tree or schema not available")
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
func (e *Editor) SetValue(path []string, key, value string) error {
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

// DeleteValue removes a leaf value at the given path in the tree.
func (e *Editor) DeleteValue(path []string, key string) error {
	if e.session != nil {
		return e.writeThroughDelete(path, key)
	}
	target := e.WalkPath(path)
	if target == nil {
		return fmt.Errorf("path not found")
	}
	target.Delete(key)
	e.dirty.Store(true)
	return nil
}

// DeleteContainer removes a container at the given path in the tree.
func (e *Editor) DeleteContainer(path []string, name string) error {
	var target *config.Tree
	if len(path) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(path)
	}
	if target == nil {
		return fmt.Errorf("path not found")
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
		return fmt.Errorf("empty path")
	}
	if e.schema == nil {
		return fmt.Errorf("schema not available")
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

	// Not a list entry: try leaf delete, then container delete
	target := fullPath[len(fullPath)-1]
	parentPath := fullPath[:len(fullPath)-1]

	if err := e.DeleteValue(parentPath, target); err != nil {
		if errC := e.DeleteContainer(parentPath, target); errC != nil {
			return fmt.Errorf("not found: %s", strings.Join(fullPath, " "))
		}
	}
	return nil
}

// walkSchema walks the schema tree along the given path, returning the schema node
// at the end of the path (or nil if any element is not found or not navigable).
func (e *Editor) walkSchema(path []string) schemaGetter {
	var current schemaGetter = e.schema
	for _, name := range path {
		node := current.Get(name)
		if node == nil {
			return nil
		}
		switch n := node.(type) {
		case *config.ContainerNode:
			current = n
		case *config.ListNode:
			current = n
		case *config.FlexNode:
			current = n
		case *config.LeafNode, *config.FreeformNode,
			*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode,
			*config.InlineListNode:
			return nil // Can't navigate into leaf nodes
		}
	}
	return current
}

// DeleteListEntry removes a list entry at the given path in the tree.
func (e *Editor) DeleteListEntry(path []string, listName, key string) error {
	var target *config.Tree
	if len(path) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(path)
	}
	if target == nil {
		return fmt.Errorf("path not found")
	}
	target.RemoveListEntry(listName, key)
	e.dirty.Store(true)
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
		return fmt.Errorf("path not found")
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
		return fmt.Errorf("copy not supported in session mode")
	}
	var target *config.Tree
	if len(parentPath) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(parentPath)
	}
	if target == nil {
		return fmt.Errorf("path not found")
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
		return fmt.Errorf("insert not supported in session mode")
	}
	var target *config.Tree
	if len(path) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(path)
	}
	if target == nil {
		return fmt.Errorf("path not found")
	}
	if err := target.InsertMultiValue(leafListName, value, position, ref); err != nil {
		return err
	}
	e.dirty.Store(true)
	return nil
}

// DeactivateLeafListValue adds "inactive:" prefix to a value in a leaf-list.
func (e *Editor) DeactivateLeafListValue(path []string, leafListName, value string) error {
	if e.session != nil {
		return fmt.Errorf("deactivate not supported in session mode")
	}
	var target *config.Tree
	if len(path) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(path)
	}
	if target == nil {
		return fmt.Errorf("path not found")
	}
	if err := target.DeactivateMultiValue(leafListName, value); err != nil {
		return err
	}
	e.dirty.Store(true)
	return nil
}

// ActivateLeafListValue removes "inactive:" prefix from a value in a leaf-list.
func (e *Editor) ActivateLeafListValue(path []string, leafListName, value string) error {
	if e.session != nil {
		return fmt.Errorf("activate not supported in session mode")
	}
	var target *config.Tree
	if len(path) == 0 {
		target = e.tree
	} else {
		target = e.WalkPath(path)
	}
	if target == nil {
		return fmt.Errorf("path not found")
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
		return fmt.Errorf("deactivate not supported in session mode")
	}
	target := e.tree
	if len(parentPath) > 0 {
		target = e.WalkPath(parentPath)
	}
	if target == nil {
		return fmt.Errorf("%w: %s", ErrPathNotFound, strings.Join(parentPath, " "))
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
		return fmt.Errorf("activate not supported in session mode")
	}
	target := e.tree
	if len(parentPath) > 0 {
		target = e.WalkPath(parentPath)
	}
	if target == nil {
		return fmt.Errorf("%w: %s", ErrPathNotFound, strings.Join(parentPath, " "))
	}
	if !target.IsLeafInactive(leafName) {
		return fmt.Errorf("%w: %q", ErrLeafNotInactive, leafName)
	}
	target.ClearLeafInactive(leafName)
	e.dirty.Store(true)
	return nil
}

// DeactivatePath sets the schema-injected `inactive` leaf to true on
// the container or list entry at path. Strict on path resolution: it
// rejects non-existent paths rather than silently materializing them
// (which is what plain SetValue + walkOrCreate would do).
//
// Returns ErrPathNotFound when the path does not resolve in the tree,
// and ErrPathAlreadyInactive (wrapped) when the inactive flag is
// already set, so callers can use errors.Is for idempotent flows.
func (e *Editor) DeactivatePath(path []string) error {
	if e.session != nil {
		return fmt.Errorf("deactivate not supported in session mode")
	}
	target := e.WalkPath(path)
	if target == nil {
		return fmt.Errorf("%w: %s", ErrPathNotFound, strings.Join(path, " "))
	}
	if v, ok := target.Get(config.InactiveLeafName); ok && v == boolTrue {
		return fmt.Errorf("%w: %s", ErrPathAlreadyInactive, strings.Join(path, " "))
	}
	target.Set(config.InactiveLeafName, boolTrue)
	e.dirty.Store(true)
	return nil
}

// ActivatePath clears the schema-injected `inactive` leaf on the
// container or list entry at path. Strict on path resolution.
//
// Returns ErrPathNotFound or ErrPathNotInactive (wrapped) for the
// idempotent / mistyped-path cases.
func (e *Editor) ActivatePath(path []string) error {
	if e.session != nil {
		return fmt.Errorf("activate not supported in session mode")
	}
	target := e.WalkPath(path)
	if target == nil {
		return fmt.Errorf("%w: %s", ErrPathNotFound, strings.Join(path, " "))
	}
	if v, ok := target.Get(config.InactiveLeafName); !ok || v != boolTrue {
		return fmt.Errorf("%w: %s", ErrPathNotInactive, strings.Join(path, " "))
	}
	target.Delete(config.InactiveLeafName)
	e.dirty.Store(true)
	return nil
}

// resolveListTarget walks the schema-aware path and identifies the terminal
// list entry. Returns the tree-level parent path (for WalkPath), the list name,
// and the entry key. Returns an error if the path does not end at a list entry.
func (e *Editor) resolveListTarget(fullPath []string) (parentPath []string, listName, key string, err error) {
	if e.schema == nil {
		return nil, "", "", fmt.Errorf("schema not available")
	}
	if len(fullPath) < 2 {
		return nil, "", "", fmt.Errorf("path too short for list entry")
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
				return nil, "", "", fmt.Errorf("cannot rename anonymous list entry")
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
		return nil, "", "", fmt.Errorf("path does not end at a list entry")
	}

	// The last list entry must be at the end of the path
	if lastListIdx+2 != len(fullPath) {
		return nil, "", "", fmt.Errorf("rename target must be the last element in the path")
	}

	return fullPath[:lastListIdx], lastListName, lastKey, nil
}

// Save commits changes: creates backup of original, writes serialized tree.
// Returns an error when a session is active -- use CommitSession() instead.
func (e *Editor) Save() error {
	if e.session != nil {
		return fmt.Errorf("Save() not allowed with active session; use CommitSession()")
	}
	if !e.dirty.Load() {
		return nil
	}

	// Hash any plaintext-password siblings of ze:bcrypt leaves before
	// serialization. Mirrors the commit-time hashing done in CommitSession.
	if e.treeValid && e.tree != nil && e.schema != nil {
		if err := config.ApplyPasswordHashing(e.tree, e.schema); err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
	}

	// Create backup of original
	if err := e.createBackup(e.originalContent, nil); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Write serialized tree (or raw text fallback) to original path
	content := e.WorkingContent()
	if err := e.store.WriteFile(e.originalPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Update original to match saved
	e.originalContent = content
	e.dirty.Store(false)

	// Delete edit file on successful commit
	e.deleteEditFile()

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
