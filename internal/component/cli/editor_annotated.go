// Design: docs/architecture/config/yang-config-design.md — annotated config display
// Overview: editor.go — config editor core
// Related: editor_session.go — session identity for concurrent editing

package cli

import (
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// AnnotatedView returns config content annotated with the enabled metadata columns.
// When setFormat is true, produces flat set commands; otherwise hierarchical tree.
// At a sub-path, the metadata tree is walked to match the context path.
func (e *Editor) AnnotatedView(path []string, columns config.ShowColumns, setFormat bool) string {
	tree := e.tree
	meta := e.meta
	schema := e.schema
	if meta == nil {
		meta = config.NewMetaTree()
	}

	// Walk to sub-path if needed
	if len(path) > 0 && e.treeValid {
		subtree, schemaNode := e.walkPathWithSchema(path)
		if subtree != nil && schemaNode != nil {
			tree = subtree
			// Walk meta tree in parallel
			meta = e.walkMetaPath(meta, path)
			// Use SerializeAnnotatedTree/Set with a schema wrapper
			return e.annotatedAtNode(tree, meta, schemaNode, columns, setFormat)
		}
	}

	if setFormat {
		return config.SerializeAnnotatedSet(tree, meta, schema, columns)
	}
	return config.SerializeAnnotatedTree(tree, meta, schema, columns)
}

// annotatedAtNode renders annotated output at a specific schema node (for sub-paths).
func (e *Editor) annotatedAtNode(tree *config.Tree, meta *config.MetaTree, node config.Node, columns config.ShowColumns, setFormat bool) string {
	cp, ok := node.(interface {
		Children() []string
		Get(string) config.Node
	})
	if !ok {
		return ""
	}

	if setFormat {
		return config.SerializeAnnotatedSubtreeSet(tree, meta, cp, columns)
	}
	return config.SerializeAnnotatedSubtree(tree, meta, cp, columns)
}

// walkMetaPath walks the metadata tree along the same path as the config tree.
// Tracks the config tree in parallel so #N positional indexes resolve at the correct depth.
// Returns the sub-MetaTree at the end, or an empty MetaTree if the path doesn't resolve.
func (e *Editor) walkMetaPath(meta *config.MetaTree, path []string) *config.MetaTree {
	if meta == nil || e.schema == nil || e.tree == nil || len(path) == 0 {
		return config.NewMetaTree()
	}

	current := meta
	currentTree := e.tree
	var currentSchema schemaGetter = e.schema

	i := 0
	for i < len(path) {
		name := path[i]
		schemaNode := currentSchema.Get(name)
		if schemaNode == nil {
			return config.NewMetaTree()
		}

		switch n := schemaNode.(type) {
		case *config.ContainerNode:
			child := current.GetContainer(name)
			if child == nil {
				return config.NewMetaTree()
			}
			current = child
			currentTree = currentTree.GetContainer(name)
			if currentTree == nil {
				return config.NewMetaTree()
			}
			currentSchema = n
			i++
		case *config.ListNode:
			// Lists: container(name) -> list(key)
			listMeta := current.GetContainer(name)
			if listMeta == nil {
				return config.NewMetaTree()
			}
			if i+1 < len(path) && n.Get(path[i+1]) == nil {
				// Keyed list entry -- resolve #N positional indexes using current tree depth
				key := path[i+1]
				entry := listMeta.GetListEntry(key)
				if entry == nil {
					if resolved := resolveMetaListKey(currentTree, name, key, listMeta); resolved != nil {
						entry = resolved
					}
				}
				if entry == nil {
					return config.NewMetaTree()
				}
				current = entry
				// Advance config tree into the list entry
				if entries := currentTree.GetList(name); entries != nil {
					if ct := entries[key]; ct != nil {
						currentTree = ct
					} else if resolved := resolveListKey(currentTree, name, key); resolved != nil {
						currentTree = resolved
					}
				}
				currentSchema = n
				i += 2
			} else {
				// Anonymous list entry
				entry := listMeta.GetListEntry(config.KeyDefault)
				if entry == nil {
					return config.NewMetaTree()
				}
				current = entry
				if entries := currentTree.GetList(name); entries != nil {
					if ct := entries[config.KeyDefault]; ct != nil {
						currentTree = ct
					}
				}
				currentSchema = n
				i++
			}
		case *config.FlexNode:
			child := current.GetContainer(name)
			if child == nil {
				return config.NewMetaTree()
			}
			current = child
			currentTree = currentTree.GetContainer(name)
			if currentTree == nil {
				return config.NewMetaTree()
			}
			currentSchema = n
			i++
		case *config.LeafNode, *config.FreeformNode,
			*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode,
			*config.InlineListNode:
			return config.NewMetaTree()
		}
	}

	return current
}

// resolveMetaListKey resolves a #N positional index to the actual list entry
// in the MetaTree. Uses the config tree to map positional indexes to actual keys,
// then looks up that key in the MetaTree.
func resolveMetaListKey(tree *config.Tree, listName, key string, listMeta *config.MetaTree) *config.MetaTree {
	if tree == nil || !strings.HasPrefix(key, "#") {
		return nil
	}
	idx, err := strconv.Atoi(key[1:])
	if err != nil || idx < 1 {
		return nil
	}
	ordered := tree.GetListOrdered(listName)
	if idx > len(ordered) {
		return nil
	}
	actualKey := ordered[idx-1].Key
	return listMeta.GetListEntry(actualKey)
}

// newShowColumnDefaults returns a fresh map of show column defaults (all disabled).
func newShowColumnDefaults() map[string]bool {
	return map[string]bool{
		"author":  false,
		"date":    false,
		"source":  false,
		"changes": false,
	}
}

// ShowColumnEnabled returns whether a show column is enabled.
func (e *Editor) ShowColumnEnabled(column string) bool {
	if e.showColumns == nil {
		return false
	}
	return e.showColumns[column]
}

// SetShowColumn enables or disables a show column preference.
// Only valid column names (author, date, source, changes) are accepted.
func (e *Editor) SetShowColumn(column string, enabled bool) {
	if e.showColumns == nil {
		e.showColumns = newShowColumnDefaults()
	}
	if _, ok := e.showColumns[column]; !ok {
		return // reject unknown column names
	}
	e.showColumns[column] = enabled
}

// DiffGutterEnabled returns whether the diff gutter (+/-) markers are shown.
func (e *Editor) DiffGutterEnabled() bool {
	return e.diffGutter
}

// SetDiffGutter enables or disables the diff gutter (+/-) markers.
func (e *Editor) SetDiffGutter(enabled bool) {
	e.diffGutter = enabled
}

// SavedDraftContent returns the content of the saved draft file on disk.
// Returns empty string if no draft exists.
func (e *Editor) SavedDraftContent() string {
	draftPath := e.originalPath + ".draft"
	data, err := e.store.ReadFile(draftPath)
	if err != nil {
		return ""
	}
	return string(data)
}
