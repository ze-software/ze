// Design: docs/architecture/web-interface.md -- Schema and config tree walking
// Overview: handler_config.go -- Config tree view handlers
// Related: handler_config_leaf.go -- Leaf input type and template helpers

package web

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// schemaGetter is the interface shared by Schema, ContainerNode, ListNode,
// FlexNode, and InlineListNode for walking the schema tree.
type schemaGetter interface {
	Get(name string) config.Node
}

// childLister extends schemaGetter with ordered child enumeration.
type childLister interface {
	schemaGetter
	Children() []string
}

// walkConfigPath walks both schema and config tree in parallel to the given
// path. Returns the schema node at the end of the path, the config subtree
// at that position (nil if not configured), and any error from schema lookup.
func walkConfigPath(schema *config.Schema, tree *config.Tree, path []string) (config.Node, *config.Tree, error) {
	schemaNode, err := walkSchema(schema, path)
	if err != nil {
		return nil, nil, err
	}

	subtree := walkTree(tree, schema, path)

	return schemaNode, subtree, nil
}

// errEmptyPath is returned when walkSchema is called with an empty path.
var errEmptyPath = fmt.Errorf("empty schema path")

// walkSchema walks the schema tree following path segments and returns the
// node at the path end. List keys consume 2 path segments (name + key value),
// except when the next segment is a valid child of the list (anonymous entry).
// Returns errEmptyPath when path has no segments.
// isListEntryPath returns true if the path ends at a list entry (key consumed),
// false if it ends at the list itself (no key). Only meaningful when walkSchema returns a ListNode.
func isListEntryPath(schema *config.Schema, path []string) bool {
	var current schemaGetter = schema
	for i := 0; i < len(path); {
		node := current.Get(path[i])
		if node == nil {
			return false
		}
		if listNode, ok := node.(*config.ListNode); ok {
			if i+1 >= len(path) {
				return false // Path ends at list name, no entry key.
			}
			if listNode.Get(path[i+1]) != nil {
				i++ // Anonymous access.
			} else {
				if i+2 >= len(path) {
					return true // Path ends at the entry key.
				}
				i += 2
			}
			current = listNode
			continue
		}
		if sg, ok := node.(schemaGetter); ok {
			current = sg
		}
		i++
	}
	return false
}

func walkSchema(schema *config.Schema, path []string) (config.Node, error) {
	if len(path) == 0 {
		return nil, errEmptyPath
	}

	var current schemaGetter = schema

	i := 0
	var lastNode config.Node

	for i < len(path) {
		name := path[i]
		node := current.Get(name)
		if node == nil {
			return nil, fmt.Errorf("unknown schema path element: %q", name)
		}

		switch n := node.(type) {
		case *config.ContainerNode:
			current = n
			lastNode = n
			i++

		case *config.ListNode:
			// Anonymous vs keyed: anonymous if no next element or next
			// element is a valid child of the list.
			if i+1 >= len(path) || n.Get(path[i+1]) != nil {
				i++
			} else {
				i += 2
			}
			current = n
			lastNode = n

		case *config.FlexNode:
			current = n
			lastNode = n
			i++

		case *config.InlineListNode:
			if i+1 >= len(path) || n.Get(path[i+1]) != nil {
				i++
			} else {
				i += 2
			}
			current = n
			lastNode = n

		case *config.FreeformNode:
			lastNode = n
			i++

			if i < len(path) {
				return nil, fmt.Errorf("path continues past terminal node %q", name)
			}

		case *config.LeafNode:
			lastNode = n
			i++

			if i < len(path) {
				return nil, fmt.Errorf("path continues past terminal node %q", name)
			}

		case *config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
			// Terminal leaf-like nodes that cannot be walked into further.
			lastNode = n
			i++

			if i < len(path) {
				return nil, fmt.Errorf("path continues past terminal node %q", name)
			}

		default: //nolint:gocritic // returns error for unhandled schema node types
			return nil, fmt.Errorf("unhandled schema node type at %q", name)
		}
	}

	return lastNode, nil
}

// walkTree walks the config tree following the same pattern as walkSchema,
// returning the subtree at the given path or nil if the path does not exist
// in the configured data.
func walkTree(tree *config.Tree, schema *config.Schema, path []string) *config.Tree {
	if tree == nil || len(path) == 0 {
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
			var key string
			var step int
			if i+1 >= len(path) || n.Get(path[i+1]) != nil {
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
			currentSchema = n
			i += step

		case *config.FlexNode:
			child := current.GetContainer(name)
			if child == nil {
				return nil
			}
			current = child
			currentSchema = n
			i++

		case *config.InlineListNode:
			var key string
			var step int
			if i+1 >= len(path) || n.Get(path[i+1]) != nil {
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
			currentSchema = n
			i += step

		case *config.LeafNode, *config.FreeformNode,
			*config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
			// Terminal nodes: tree walk cannot descend further.
			return current

		default: //nolint:gocritic // returns nil for unhandled schema node types
			return nil
		}
	}

	return current
}

// buildConfigViewData walks the schema and tree to the given path, determines
// the node kind, and assembles template data for the config view.
// For containers and list entries, leaves are placed in LeafFields and
// non-leaf children are placed in Children.
func buildConfigViewData(schema *config.Schema, tree *config.Tree, path []string) (*ConfigViewData, error) {
	data := &ConfigViewData{
		Path:        path,
		CurrentPath: strings.Join(path, "/"),
		Breadcrumbs: buildBreadcrumbs(path),
	}

	// Root view: list top-level schema children as containers.
	if len(path) == 0 {
		data.NodeKind = config.NodeContainer
		for _, name := range schema.Children() {
			node := schema.Get(name)
			data.Children = append(data.Children, ChildEntry{
				Name: name,
				Kind: nodeKindString(node.Kind()),
				URL:  "/show/" + name + "/",
			})
		}
		return data, nil
	}

	schemaNode, err := walkSchema(schema, path)
	if err != nil {
		return nil, err
	}
	if schemaNode == nil {
		return nil, fmt.Errorf("schema node not found at path: %s", strings.Join(path, "/"))
	}

	data.NodeKind = schemaNode.Kind()
	prefix := "/show/" + strings.Join(path, "/")

	switch n := schemaNode.(type) {
	case *config.ContainerNode:
		subtree := walkTree(tree, schema, path)
		populateContainerView(data, n, subtree, prefix)

	case *config.ListNode:
		// When the path ends at the list itself (no key selected),
		// show the list of keys with the base path for navigation.
		data.Keys = collectListKeys(tree, schema, path)
		data.BasePath = "/show/" + strings.Join(path, "/") + "/"

		// When we have a list node and the path included a key (walkTree
		// descended into an entry), also populate leaf fields.
		subtree := walkTree(tree, schema, path)
		if subtree != nil {
			data.DetailPath = strings.Join(path, "/")
			populateContainerView(data, n, subtree, prefix)
		}

	case *config.FlexNode:
		subtree := walkTree(tree, schema, path)
		populateContainerView(data, n, subtree, prefix)

	case *config.InlineListNode:
		data.Keys = collectListKeys(tree, schema, path)

	case *config.LeafNode:
		subtree := walkTree(tree, schema, path[:len(path)-1])
		leafName := path[len(path)-1]
		var value string
		var configured bool
		if subtree != nil {
			value, configured = subtree.Get(leafName)
		}
		data.LeafFields = []LeafField{buildLeafField(leafName, n, value, configured)}

	case *config.FreeformNode:
		subtree := walkTree(tree, schema, path)
		if subtree != nil {
			data.Entries = subtree.Values()
		}

	case *config.MultiLeafNode, *config.BracketLeafListNode, *config.ValueOrArrayNode:
		// Leaf-like terminal nodes: no further navigation.

		// No default: all config.Node implementations are handled above.
	}

	return data, nil
}

// populateContainerView fills LeafFields and Children from a container-like
// schema node. Leaves go into LeafFields (with values from the config tree),
// non-leaves go into Children as navigation links.
func populateContainerView(data *ConfigViewData, provider childLister, subtree *config.Tree, prefix string) {
	for _, name := range provider.Children() {
		child := provider.Get(name)

		if leaf, ok := child.(*config.LeafNode); ok {
			var value string
			var configured bool
			if subtree != nil {
				value, configured = subtree.Get(name)
			}
			data.LeafFields = append(data.LeafFields, buildLeafField(name, leaf, value, configured))
			continue
		}

		data.Children = append(data.Children, ChildEntry{
			Name: name,
			Kind: nodeKindString(child.Kind()),
			URL:  prefix + "/" + name + "/",
		})
	}
}

// collectListKeys reads the configured list keys from the tree for the list
// at the end of path. Returns key strings in insertion order.
// validateUniqueOnSet checks if setting a leaf value would violate a unique constraint.
// path is the set target (e.g., ["bgp", "peer", "thomas", "remote"]), leaf is "ip".
// It walks up the path to find the enclosing list, reconstructs the unique field path,
// and checks against other entries. Returns an error message or empty string.
func validateUniqueOnSet(tree *config.Tree, schema *config.Schema, path []string, leaf, value string) string {
	if tree == nil || value == "" {
		return ""
	}
	// Walk the schema to find the enclosing list node and where the entry key is.
	var current schemaGetter = schema
	for i := 0; i < len(path); i++ {
		node := current.Get(path[i])
		if node == nil {
			return ""
		}
		if listNode, ok := node.(*config.ListNode); ok {
			uniqueFields := collectUniqueFields(listNode)
			if len(uniqueFields) == 0 {
				current = listNode
				if i+1 < len(path) && listNode.Get(path[i+1]) == nil {
					i++ // skip entry key
				}
				continue
			}
			// Found a list with unique constraints. Entry key is path[i+1].
			if i+1 >= len(path) {
				return ""
			}
			entryKey := path[i+1]
			listPath := path[:i+1]
			// Reconstruct the unique field path from the remaining path segments + leaf.
			// e.g., path=["bgp","peer","thomas","remote"], leaf="ip" → fieldPath="remote/ip"
			suffix := path[i+2:]
			fieldPath := strings.Join(append(suffix, leaf), "/")
			// Check if this field is actually a unique field.
			for _, uf := range uniqueFields {
				if uf == fieldPath {
					if conflict := checkUniqueConstraint(tree, schema, listPath, entryKey, fieldPath, value); conflict != "" {
						return fmt.Sprintf("duplicate %s %q (already used by %s)", fieldPath, value, conflict)
					}
					return ""
				}
			}
			return ""
		}
		if sg, ok := node.(schemaGetter); ok {
			current = sg
		} else {
			return ""
		}
	}
	return ""
}

// checkUniqueConstraint checks if setting a value would violate a YANG unique constraint.
// listPath is the path to the list (e.g., ["bgp", "peer"]).
// entryKey is the key of the entry being modified.
// fieldPath is the unique field being set (e.g., "remote/ip").
// newValue is the value being set.
// Returns the conflicting entry key if a duplicate is found, empty string otherwise.
func checkUniqueConstraint(tree *config.Tree, schema *config.Schema, listPath []string, entryKey, fieldPath, newValue string) string {
	if newValue == "" {
		return ""
	}
	keys := collectListKeys(tree, schema, listPath)
	for _, k := range keys {
		if k == entryKey {
			continue // Skip the entry being modified.
		}
		entryPath := append(append([]string{}, listPath...), k)
		entryTree := walkTree(tree, schema, entryPath)
		if entryTree == nil {
			continue
		}
		existing := resolveNestedValue(entryTree, fieldPath)
		if existing == newValue {
			return k
		}
	}
	return ""
}

// findListNode returns the ListNode at path if it resolves to one.
func findListNode(schema *config.Schema, path []string) (*config.ListNode, bool) {
	node, err := walkSchema(schema, path)
	if err != nil {
		return nil, false
	}
	ln, ok := node.(*config.ListNode)
	return ln, ok
}

func collectListKeys(tree *config.Tree, schema *config.Schema, path []string) []string {
	var parentTree *config.Tree
	listName := path[len(path)-1]

	if len(path) == 1 {
		parentTree = tree
	} else {
		parentTree = walkTree(tree, schema, path[:len(path)-1])
	}

	if parentTree == nil {
		return nil
	}

	ordered := parentTree.GetListOrdered(listName)
	keys := make([]string, 0, len(ordered))
	for _, kv := range ordered {
		keys = append(keys, kv.Key)
	}

	return keys
}
