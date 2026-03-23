// Design: docs/architecture/config/syntax.md — inline config arg parsing
// Overview: setparser.go — config parser core

package config

import (
	"fmt"
	"strings"
)

// ParseInlineArgs builds a Tree from a flat sequence of schema-driven key-value
// tokens. The schema determines how many tokens each field consumes:
// a leaf consumes name + value (2 tokens), a container consumes name + recurse.
//
// Example: ["remote", "as", "65001", "hold-time", "90"] with a peer-fields
// schema node produces a Tree with remote.as=65001 and hold-time=90.
//
// Unlike Parse which handles "set path value" lines, this handles inline
// args where multiple fields are concatenated without delimiters.
// parent MUST NOT be nil.
func ParseInlineArgs(parent Node, args []string) (*Tree, error) {
	if parent == nil {
		return nil, fmt.Errorf("ParseInlineArgs: parent schema node must not be nil")
	}
	tree := NewTree()
	remaining := args
	for len(remaining) > 0 {
		consumed, err := consumeOneField(tree, parent, remaining)
		if err != nil {
			return nil, err
		}
		remaining = remaining[consumed:]
	}
	return tree, nil
}

// consumeOneField uses the schema to greedily consume tokens for one field
// setting and stores the result in tree. Returns the number of tokens consumed.
//
// Supported schema node types:
//   - NodeLeaf: name + value (2 tokens)
//   - NodeContainer: name + recurse into children
//   - NodeList: name + key + recurse into list entry children
//   - NodeFlex: like container (recurse if next token is a known child), else name + value (2 tokens)
func consumeOneField(tree *Tree, parent Node, tokens []string) (int, error) {
	name := strings.ToLower(tokens[0])

	child := resolveSchemaNode(nil, parent, name)
	if child == nil {
		return 0, fmt.Errorf("unknown option: %s", tokens[0])
	}

	switch child.Kind() {
	case NodeLeaf:
		if len(tokens) < 2 {
			return 0, fmt.Errorf("missing value for %s", name)
		}
		if err := validateLeafValue(child, name, tokens[1]); err != nil {
			return 0, err
		}
		tree.Set(name, tokens[1])
		return 2, nil

	case NodeContainer:
		if len(tokens) < 2 {
			return 0, fmt.Errorf("%s requires a sub-key", name)
		}
		childTree := tree.GetOrCreateContainer(name)
		innerConsumed, err := consumeOneField(childTree, child, tokens[1:])
		if err != nil {
			return 0, err
		}
		return 1 + innerConsumed, nil

	case NodeList:
		// List: name + key + field path. E.g., "family ipv4/unicast mode enable"
		if len(tokens) < 3 {
			return 0, fmt.Errorf("%s requires key and field", name)
		}
		key := tokens[1]
		entries := tree.GetList(name)
		if entries == nil {
			entries = make(map[string]*Tree)
		}
		entry := entries[key]
		if entry == nil {
			entry = NewTree()
			tree.AddListEntry(name, key, entry)
		}
		innerConsumed, err := consumeOneField(entry, child, tokens[2:])
		if err != nil {
			return 0, err
		}
		return 2 + innerConsumed, nil

	case NodeFlex:
		// Flex: if next token is a known child, recurse. Otherwise treat as value.
		if len(tokens) < 2 {
			return 0, fmt.Errorf("missing value for %s", name)
		}
		nextChild := resolveSchemaNode(nil, child, strings.ToLower(tokens[1]))
		if nextChild != nil {
			childTree := tree.GetOrCreateContainer(name)
			innerConsumed, err := consumeOneField(childTree, child, tokens[1:])
			if err != nil {
				return 0, err
			}
			return 1 + innerConsumed, nil
		}
		// No matching child: treat as simple value
		tree.Set(name, tokens[1])
		return 2, nil

	case NodeFreeform, NodeInlineList:
		return 0, fmt.Errorf("unsupported schema type for inline arg: %s", name)
	}

	return 0, fmt.Errorf("unsupported schema type for inline arg: %s (%d)", name, child.Kind())
}

// validateLeafValue validates a value against any leaf-like schema node type.
// Returns nil for non-leaf types (validation not applicable).
func validateLeafValue(node Node, name, value string) error {
	var typ ValueType
	//nolint:gocritic // exhaustive leaf-type dispatch, not a config switch
	if leaf, ok := node.(*LeafNode); ok {
		typ = leaf.Type
	} else if ml, ok := node.(*MultiLeafNode); ok {
		typ = ml.Type
	} else if bl, ok := node.(*BracketLeafListNode); ok {
		typ = bl.Type
	} else if va, ok := node.(*ValueOrArrayNode); ok {
		typ = va.Type
	} else {
		return nil // enum nodes and non-leaf types: no type validation here
	}
	if err := ValidateValue(typ, value); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
