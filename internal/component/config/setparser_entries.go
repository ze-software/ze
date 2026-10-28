// Design: docs/architecture/config/syntax.md — set-format entry writers
// Overview: setparser.go — config parser core
// Related: setparser_meta.go — metadata-annotated parsing (ParseWithMeta)

package config

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// setFreeformValue stores a freeform key-value pair.
// Format: set <path> <freeform-name> <key> [value]
// Stored as container[name] -> Tree with key=value (or key="true" for flags).
func setFreeformValue(tree *Tree, name string, tokens []string, lineNum int) error {
	if len(tokens) < 1 {
		return fmt.Errorf("line %d: freeform %s requires at least a key", lineNum, name)
	}
	child := tree.GetOrCreateContainer(name)
	key := tokens[0]
	value := configTrue
	if len(tokens) > 1 {
		value = textbuf.Join(tokens[1:], " ")
	}
	child.Set(key, value)
	return nil
}

// setFlexValue handles FlexNode's forms: flag, container with children, named
// block entry, or value.
func (p *SetParser) setFlexValue(tree *Tree, flex *FlexNode, name string, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		// Flag form: just the name with no value.
		tree.Set(name, configTrue)
		return nil
	}

	// If the first token matches a child in the flex schema, recurse into container form.
	if flex.Get(tokens[0]) != nil {
		child := tree.GetOrCreateContainer(name)
		return p.walkAndSet(child, flex, tokens, lineNum)
	}

	if isFlexListEntry(flex, tokens) {
		key := tokens[0]
		return p.walkAndSet(ensureFlexListEntry(tree, name, key), flex, tokens[1:], lineNum)
	}

	// File replay keeps each value line, unlike the editor's scalar Set.
	appendSetFlexValue(tree, name, textbuf.Join(tokens, " "))
	return nil
}

// appendSetFlexValue preserves repeated Flex lines in file order, duplicates
// included. A lone value stays scalar so the editor can replace it through Set;
// the second line promotes both values to the store the Flex printers enumerate.
func appendSetFlexValue(tree *Tree, name, value string) {
	tree.mu.Lock()
	defer tree.mu.Unlock()
	if previous, ok := tree.values[name]; ok {
		delete(tree.values, name)
		for i, key := range tree.valuesOrder {
			if key == name {
				tree.valuesOrder = append(tree.valuesOrder[:i], tree.valuesOrder[i+1:]...)
				break
			}
		}
		tree.multiValues[name] = append(tree.multiValues[name], previous, value)
		return
	}
	if len(tree.multiValues[name]) > 0 {
		tree.multiValues[name] = append(tree.multiValues[name], value)
		return
	}
	tree.storeValueLocked(name, value)
}

// isFlexListEntry reports whether the tokens after a Flex node's name address
// a named block entry, `<key> <field> ...`: the first token is no child of the
// Flex schema and the second one is. That is the line serializeSetFlex and
// writeFlexLines write for each list entry, which the hierarchical parser read
// from `<name> <key> { <field> ...; }` (parseFlexValue). Any other token run is
// the Flex value.
func isFlexListEntry(flex *FlexNode, tokens []string) bool {
	if len(tokens) < 2 {
		return false
	}
	if flex.Get(tokens[0]) != nil {
		return false
	}
	return flex.Get(tokens[1]) != nil
}

// ensureFlexListEntry returns the entry key of the Flex list name, recording a
// new one after the entries already read, so the entries keep their written
// order.
func ensureFlexListEntry(tree *Tree, name, key string) *Tree {
	if entry := tree.GetList(name)[key]; entry != nil {
		return entry
	}
	entry := NewTree()
	tree.AddListEntry(name, key, entry)
	return entry
}

// walkAndSetListEntry creates or finds a list entry and recurses.
func (p *SetParser) walkAndSetListEntry(tree *Tree, list *ListNode, name string, tokens []string, lineNum int) error {
	if err := ValidateListKey(list, tokens[0]); err != nil {
		return fmt.Errorf("line %d: invalid key for %s: %w", lineNum, name, err)
	}
	addr, err := ParseListAddress(name, list, tokens)
	if err != nil {
		return fmt.Errorf("line %d: %w", lineNum, err)
	}
	entry := EnsureListEntry(tree, name, addr)
	return p.walkAndSet(entry, list, tokens[addr.Tokens:], lineNum)
}

// walkAndSetInlineListEntry creates or finds an inline list entry and recurses.
func (p *SetParser) walkAndSetInlineListEntry(tree *Tree, il *InlineListNode, name string, tokens []string, lineNum int) error {
	key := tokens[0]
	tokens = tokens[1:]
	entries := tree.GetList(name)
	if entries == nil {
		entries = make(map[string]*Tree)
	}
	entry := entries[key]
	if entry == nil {
		entry = NewTree()
		tree.AddListEntry(name, key, entry)
	}
	return p.walkAndSet(entry, il, tokens, lineNum)
}
