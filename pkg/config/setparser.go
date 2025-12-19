package config

import (
	"bufio"
	"fmt"
	"strings"
)

// SetParser parses set-style configuration.
//
// Format:
//
//	set <path> <value>
//	delete <path>
//
// Examples:
//
//	set router-id 1.2.3.4
//	set neighbor 192.0.2.1 local-as 65000
//	set neighbor 192.0.2.1 family ipv4 unicast true
//	delete neighbor 192.0.2.1 peer-as
type SetParser struct {
	schema *Schema
}

// NewSetParser creates a new set-style parser with the given schema.
func NewSetParser(schema *Schema) *SetParser {
	return &SetParser{schema: schema}
}

// Parse parses the input string into a config tree.
func (p *SetParser) Parse(input string) (*Tree, error) {
	tree := NewTree()

	scanner := bufio.NewScanner(strings.NewReader(input))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if err := p.parseLine(tree, line, lineNum); err != nil {
			return nil, err
		}
	}

	return tree, scanner.Err()
}

// parseLine parses a single set/delete line.
func (p *SetParser) parseLine(tree *Tree, line string, lineNum int) error {
	tokens := p.tokenizeLine(line)
	if len(tokens) == 0 {
		return nil
	}

	cmd := tokens[0]
	tokens = tokens[1:]

	switch cmd {
	case "set":
		return p.parseSet(tree, tokens, lineNum)
	case "delete":
		return p.parseDelete(tree, tokens, lineNum)
	default:
		return fmt.Errorf("line %d: unknown command: %s (expected set/delete)", lineNum, cmd)
	}
}

// tokenizeLine splits a line into tokens, respecting quotes.
func (p *SetParser) tokenizeLine(line string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(line); i++ {
		ch := line[i]

		if inQuote {
			if ch == quoteChar {
				inQuote = false
				tokens = append(tokens, current.String())
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
			continue
		}

		if ch == '"' || ch == '\'' {
			inQuote = true
			quoteChar = ch
			continue
		}

		if ch == ' ' || ch == '\t' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(ch)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// parseSet handles: set <path...> <value>
func (p *SetParser) parseSet(tree *Tree, tokens []string, lineNum int) error {
	if len(tokens) < 2 {
		return fmt.Errorf("line %d: set requires path and value", lineNum)
	}

	// Walk the schema to find where to set the value
	return p.walkAndSet(tree, p.schema.root, tokens, lineNum)
}

// walkAndSet walks the path and sets the value at the leaf.
func (p *SetParser) walkAndSet(tree *Tree, parent Node, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		return fmt.Errorf("line %d: incomplete path", lineNum)
	}

	name := tokens[0]
	tokens = tokens[1:]

	var node Node
	if parent == nil {
		// Start from schema root
		node = p.schema.Get(name)
	} else {
		switch n := parent.(type) {
		case *ContainerNode:
			node = n.Get(name)
		case *ListNode:
			node = n.Get(name)
		default:
			return fmt.Errorf("line %d: cannot traverse %T", lineNum, parent)
		}
	}

	if node == nil {
		return fmt.Errorf("line %d: unknown field: %s", lineNum, name)
	}

	switch n := node.(type) {
	case *LeafNode:
		// This is the target - remaining token is the value
		if len(tokens) != 1 {
			return fmt.Errorf("line %d: leaf %s expects exactly one value", lineNum, name)
		}
		value := tokens[0]

		if err := ValidateValue(n.Type, value); err != nil {
			return fmt.Errorf("line %d: invalid value for %s: %v", lineNum, name, err)
		}

		tree.Set(name, value)
		return nil

	case *ContainerNode:
		// Get or create the container in tree
		child := tree.GetContainer(name)
		if child == nil {
			child = NewTree()
			tree.SetContainer(name, child)
		}
		return p.walkAndSet(child, n, tokens, lineNum)

	case *ListNode:
		// Next token is the key
		if len(tokens) < 2 {
			return fmt.Errorf("line %d: list %s requires key and field", lineNum, name)
		}

		key := tokens[0]
		tokens = tokens[1:]

		if err := ValidateValue(n.KeyType, key); err != nil {
			return fmt.Errorf("line %d: invalid key for %s: %v", lineNum, name, err)
		}

		// Get or create the list entry
		entries := tree.GetList(name)
		if entries == nil {
			entries = make(map[string]*Tree)
		}
		entry := entries[key]
		if entry == nil {
			entry = NewTree()
			tree.AddListEntry(name, key, entry)
		}

		return p.walkAndSet(entry, n, tokens, lineNum)

	default:
		return fmt.Errorf("line %d: unknown node type for %s", lineNum, name)
	}
}

// parseDelete handles: delete <path...>
func (p *SetParser) parseDelete(tree *Tree, tokens []string, lineNum int) error {
	if len(tokens) < 1 {
		return fmt.Errorf("line %d: delete requires path", lineNum)
	}

	return p.walkAndDelete(tree, p.schema.root, tokens, lineNum)
}

// walkAndDelete walks the path and deletes the target.
func (p *SetParser) walkAndDelete(tree *Tree, parent Node, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		return fmt.Errorf("line %d: incomplete delete path", lineNum)
	}

	name := tokens[0]
	tokens = tokens[1:]

	var node Node
	switch n := parent.(type) {
	case *ContainerNode:
		node = n.Get(name)
	case *ListNode:
		node = n.Get(name)
	default:
		node = p.schema.Get(name)
	}

	if node == nil {
		return fmt.Errorf("line %d: unknown field: %s", lineNum, name)
	}

	switch n := node.(type) {
	case *LeafNode:
		// Delete the leaf
		if len(tokens) != 0 {
			return fmt.Errorf("line %d: unexpected tokens after leaf %s", lineNum, name)
		}
		tree.Delete(name)
		return nil

	case *ContainerNode:
		if len(tokens) == 0 {
			// Delete entire container
			tree.DeleteContainer(name)
			return nil
		}
		// Recurse into container
		child := tree.GetContainer(name)
		if child == nil {
			return nil // Already doesn't exist
		}
		return p.walkAndDelete(child, n, tokens, lineNum)

	case *ListNode:
		if len(tokens) == 0 {
			// Delete entire list
			tree.DeleteList(name)
			return nil
		}

		key := tokens[0]
		tokens = tokens[1:]

		entries := tree.GetList(name)
		if entries == nil {
			return nil // Already doesn't exist
		}

		if len(tokens) == 0 {
			// Delete entire list entry
			delete(entries, key)
			return nil
		}

		// Recurse into list entry
		entry := entries[key]
		if entry == nil {
			return nil // Already doesn't exist
		}

		return p.walkAndDelete(entry, n, tokens, lineNum)

	default:
		return fmt.Errorf("line %d: unknown node type for %s", lineNum, name)
	}
}

// Delete removes a leaf value from the tree.
func (t *Tree) Delete(name string) {
	delete(t.values, name)
}

// DeleteContainer removes a container from the tree.
func (t *Tree) DeleteContainer(name string) {
	delete(t.containers, name)
}

// DeleteList removes an entire list from the tree.
func (t *Tree) DeleteList(name string) {
	delete(t.lists, name)
}
