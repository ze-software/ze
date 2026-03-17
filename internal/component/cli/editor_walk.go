// Design: docs/architecture/config/yang-config-design.md — schema-aware tree/meta walking
// Overview: editor_draft.go — write-through draft protocol (uses walking for navigation)
// Related: editor_commit.go — commit/discard/disconnect (uses walking for value lookup)

package cli

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// walkOrCreateIn is like walkOrCreate but operates on an arbitrary tree (not e.tree).
func (e *Editor) walkOrCreateIn(tree *config.Tree, path []string) (*config.Tree, error) {
	if tree == nil || e.schema == nil {
		return nil, fmt.Errorf("tree or schema not available")
	}
	if len(path) == 0 {
		return tree, nil
	}

	currentTree := tree
	var currentSchema schemaGetter = e.schema

	i := 0
	for i < len(path) {
		name := path[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			return nil, fmt.Errorf("unknown path element: %s", name)
		}

		if container, ok := schemaNode.(*config.ContainerNode); ok {
			child := currentTree.GetOrCreateContainer(name)
			currentTree = child
			currentSchema = container
			i++
			continue
		}

		if listNode, ok := schemaNode.(*config.ListNode); ok {
			// Anonymous vs keyed: anonymous if no next element
			// or next element is a schema child of the list.
			var key string
			var step int
			if i+1 >= len(path) || listNode.Get(path[i+1]) != nil {
				key = config.KeyDefault
				step = 1
			} else {
				key = path[i+1]
				step = 2
			}
			entries := currentTree.GetList(name)
			if entries == nil {
				entries = make(map[string]*config.Tree)
			}
			entry := entries[key]
			if entry == nil {
				entry = config.NewTree()
				currentTree.AddListEntry(name, key, entry)
			}
			currentTree = entry
			currentSchema = listNode
			i += step
			continue
		}

		if flexNode, ok := schemaNode.(*config.FlexNode); ok {
			child := currentTree.GetOrCreateContainer(name)
			currentTree = child
			currentSchema = flexNode
			i++
			continue
		}

		if ilNode, ok := schemaNode.(*config.InlineListNode); ok {
			// Inline lists use the same key navigation as regular lists.
			var key string
			var step int
			if i+1 >= len(path) || ilNode.Get(path[i+1]) != nil {
				key = config.KeyDefault
				step = 1
			} else {
				key = path[i+1]
				step = 2
			}
			entries := currentTree.GetList(name)
			if entries == nil {
				entries = make(map[string]*config.Tree)
			}
			entry := entries[key]
			if entry == nil {
				entry = config.NewTree()
				currentTree.AddListEntry(name, key, entry)
			}
			currentTree = entry
			currentSchema = ilNode
			i += step
			continue
		}

		return nil, fmt.Errorf("unexpected schema node type at %s", name)
	}

	return currentTree, nil
}

// walkOrCreateMeta walks the MetaTree path using schema to distinguish containers
// from list entries. List names map to containers; list keys map to list entries.
// This matches the setparser pattern (GetOrCreateContainer for list name,
// GetOrCreateListEntry for key) and the serializer's metaListEntry reader.
func walkOrCreateMeta(meta *config.MetaTree, schema *config.Schema, path []string) *config.MetaTree {
	current := meta
	var currentSchema schemaGetter = schema

	i := 0
	for i < len(path) {
		name := path[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			// Unknown path element: treat as container (best-effort fallback).
			current = current.GetOrCreateContainer(name)
			i++
			continue
		}

		switch n := schemaNode.(type) {
		case *config.ContainerNode:
			current = current.GetOrCreateContainer(name)
			currentSchema = n
			i++
		case *config.ListNode:
			// List name -> container, then key -> list entry.
			// Anonymous list detection: if the next path element is a schema
			// child of the list, treat as anonymous (no key in path).
			// Anonymous lists use KeyDefault to match walkOrCreateIn.
			listContainer := current.GetOrCreateContainer(name)
			if i+1 < len(path) && n.Get(path[i+1]) == nil {
				current = listContainer.GetOrCreateListEntry(path[i+1])
				currentSchema = n
				i += 2
			} else {
				current = listContainer.GetOrCreateListEntry(config.KeyDefault)
				currentSchema = n
				i++
			}
		case *config.FlexNode:
			current = current.GetOrCreateContainer(name)
			currentSchema = n
			i++
		case *config.InlineListNode:
			// Anonymous inline lists use KeyDefault to match walkOrCreateIn.
			listContainer := current.GetOrCreateContainer(name)
			if i+1 < len(path) && n.Get(path[i+1]) == nil {
				current = listContainer.GetOrCreateListEntry(path[i+1])
				currentSchema = n
				i += 2
			} else {
				current = listContainer.GetOrCreateListEntry(config.KeyDefault)
				currentSchema = n
				i++
			}
		case *config.LeafNode, *config.FreeformNode,
			*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
			// Leaf-like nodes should not appear mid-path. Use container as
			// best-effort so we don't lose metadata.
			current = current.GetOrCreateContainer(name)
			i++
		}
	}
	return current
}

// walkMetaReadOnly navigates MetaTree using schema for read-only access.
// Returns nil if any segment is missing (does not create nodes).
func walkMetaReadOnly(meta *config.MetaTree, schema *config.Schema, path []string) *config.MetaTree {
	current := meta
	var currentSchema schemaGetter = schema

	i := 0
	for i < len(path) {
		name := path[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			return nil
		}

		switch n := schemaNode.(type) {
		case *config.ContainerNode:
			child := current.GetContainer(name)
			if child == nil {
				return nil
			}
			current = child
			currentSchema = n
			i++
		case *config.ListNode:
			listContainer := current.GetContainer(name)
			if listContainer == nil {
				return nil
			}
			if i+1 < len(path) && n.Get(path[i+1]) == nil {
				entry := listContainer.GetListEntry(path[i+1])
				if entry == nil {
					return nil
				}
				current = entry
				currentSchema = n
				i += 2
			} else {
				// Anonymous list: use KeyDefault to match walkOrCreateMeta.
				entry := listContainer.GetListEntry(config.KeyDefault)
				if entry == nil {
					return nil
				}
				current = entry
				currentSchema = n
				i++
			}
		case *config.FlexNode:
			child := current.GetContainer(name)
			if child == nil {
				return nil
			}
			current = child
			currentSchema = n
			i++
		case *config.InlineListNode:
			listContainer := current.GetContainer(name)
			if listContainer == nil {
				return nil
			}
			if i+1 < len(path) && n.Get(path[i+1]) == nil {
				entry := listContainer.GetListEntry(path[i+1])
				if entry == nil {
					return nil
				}
				current = entry
				currentSchema = n
				i += 2
			} else {
				// Anonymous inline list: use KeyDefault to match walkOrCreateMeta.
				entry := listContainer.GetListEntry(config.KeyDefault)
				if entry == nil {
					return nil
				}
				current = entry
				currentSchema = n
				i++
			}
		case *config.LeafNode, *config.FreeformNode,
			*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
			return nil // Leaf-like nodes cannot be navigated into.
		}
	}
	return current
}

// walkPath walks an arbitrary tree using schema to navigate lists.
func walkPath(tree *config.Tree, schema *config.Schema, path []string) *config.Tree {
	if len(path) == 0 {
		return tree
	}

	current := tree
	var currentSchema schemaGetter = schema

	i := 0
	for i < len(path) {
		name := path[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			return nil
		}

		if container, ok := schemaNode.(*config.ContainerNode); ok {
			child := current.GetContainer(name)
			if child == nil {
				return nil
			}
			current = child
			currentSchema = container
			i++
			continue
		}

		if listNode, ok := schemaNode.(*config.ListNode); ok {
			// Anonymous vs keyed: anonymous if no next element
			// or next element is a schema child of the list.
			var key string
			var step int
			if i+1 >= len(path) || listNode.Get(path[i+1]) != nil {
				key = config.KeyDefault
				step = 1
			} else {
				key = path[i+1]
				step = 2
			}
			entries := current.GetList(name)
			if entries == nil {
				return nil
			}
			entry := entries[key]
			if entry == nil {
				return nil
			}
			current = entry
			currentSchema = listNode
			i += step
			continue
		}

		if flexNode, ok := schemaNode.(*config.FlexNode); ok {
			child := current.GetContainer(name)
			if child == nil {
				return nil
			}
			current = child
			currentSchema = flexNode
			i++
			continue
		}

		if ilNode, ok := schemaNode.(*config.InlineListNode); ok {
			var key string
			var step int
			if i+1 >= len(path) || ilNode.Get(path[i+1]) != nil {
				key = config.KeyDefault
				step = 1
			} else {
				key = path[i+1]
				step = 2
			}
			entries := current.GetList(name)
			if entries == nil {
				return nil
			}
			entry := entries[key]
			if entry == nil {
				return nil
			}
			current = entry
			currentSchema = ilNode
			i += step
			continue
		}

		return nil
	}

	return current
}

// getValueAtPath retrieves a leaf value from a tree at the given YANG path.
func getValueAtPath(tree *config.Tree, schema *config.Schema, pathParts []string) string {
	if len(pathParts) == 0 {
		return ""
	}
	leafName := pathParts[len(pathParts)-1]
	parentPath := pathParts[:len(pathParts)-1]
	target := walkPath(tree, schema, parentPath)
	if target == nil {
		return ""
	}
	val, _ := target.Get(leafName)
	return val
}
