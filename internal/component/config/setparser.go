// Design: docs/architecture/config/syntax.md — config parsing and loading
// Related: parser.go — config parser core
// Related: serialize_set.go — set-format serialization (inverse of this file)
// Related: meta.go — MetaTree for metadata-aware parsing
// Detail: setparser_inline.go — inline arg parsing (ParseInlineArgs)
// Related: setparser_meta.go — metadata-annotated parsing (ParseWithMeta)

package config

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	cmdSet      = "set"
	cmdNop      = "nop"
	cmdDelete   = "delete"
	cmdInactive = "inactive"
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
//	set neighbor 192.0.2.1 family ipv4/unicast true
//	delete neighbor 192.0.2.1 peer-as
type SetParser struct {
	schema       *Schema
	preMigration bool     // collect unknown fields as warnings for migration (not final parse)
	warnings     []string // unknown field warnings from pre-migration parse
}

// NewSetParser creates a new set-style parser with the given schema.
func NewSetParser(schema *Schema) *SetParser {
	return &SetParser{schema: schema}
}

// SetPreMigration enables pre-migration mode: unknown fields are collected
// as warnings instead of causing parse errors. This allows parsing configs
// with stale fields so that tree-level migrations can remove them.
func (p *SetParser) SetPreMigration(v bool) {
	p.preMigration = v
}

// Warnings returns warnings collected during pre-migration parsing.
func (p *SetParser) Warnings() []string {
	return p.warnings
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
	case cmdSet:
		return p.parseSet(tree, tokens, lineNum)
	case cmdNop:
		return p.parseNop(tree, tokens, lineNum)
	case cmdDelete:
		return p.parseDelete(tree, tokens, lineNum)
	case cmdInactive:
		return p.parseInactive(tree, tokens, lineNum)
	default:
		return fmt.Errorf("line %d: unknown command: %s (expected set/nop/delete/inactive)", lineNum, cmd)
	}
}

// tokenizeLine splits a line into tokens, respecting quotes.
func (p *SetParser) tokenizeLine(line string) []string {
	var tokens []string
	var current textbuf.Buffer
	inQuote := false
	quoteChar := byte(0)

	for i := range len(line) {
		ch := line[i]

		if inQuote {
			if ch == quoteChar {
				inQuote = false
				tokens = append(tokens, current.String())
				current.Reset()
			} else {
				current.Byte(ch)
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

		current.Byte(ch)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// parseSet handles: set <path...> <value>.
// Also tolerates structural-only commands (e.g., "set bgp peer foo") that create
// containers or list entries without setting a leaf value. This allows incomplete
// configs (work-in-progress editing) to load without failing.
func (p *SetParser) parseSet(tree *Tree, tokens []string, lineNum int) error {
	if len(tokens) < 1 {
		return fmt.Errorf("line %d: set requires at least a path", lineNum)
	}

	// Walk the schema to find where to set the value
	return p.walkAndSet(tree, p.schema.root, tokens, lineNum)
}

// walkAndSet walks the path and sets the value at the leaf.
// Returns nil when tokens are exhausted at a non-leaf node, which means
// the caller already created the container or list entry at this level.
func (p *SetParser) walkAndSet(tree *Tree, parent Node, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		return nil // structural-only: container/entry exists, no leaf to set
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
		if p.preMigration {
			var bw textbuf.Buffer
			p.warnings = append(p.warnings, bw.Reset().Str("line ").Int(int64(lineNum)).Str(": unknown field: ").Str(name).Str(" (needs migration)").String())
			return nil
		}
		return fmt.Errorf("line %d: unknown field: %s%s", lineNum, name, RetiredKeywordHint(name))
	}

	//nolint:gocritic // if-else chain preferred over type switch for exhaustive node handling
	if leaf, ok := node.(*LeafNode); ok {
		if leaf.Type == TypeEmpty {
			// "type empty" leaves are presence flags: "set no-default-route".
			if len(tokens) != 0 {
				return fmt.Errorf("line %d: leaf %s is a flag and takes no value", lineNum, name)
			}
			tree.Set(name, configTrue)
			return nil
		}
		if len(tokens) != 1 {
			return fmt.Errorf("line %d: leaf %s expects exactly one value", lineNum, name)
		}
		value := tokens[0]
		if err := ValidateLeafValue(leaf, value); err != nil {
			return fmt.Errorf("line %d: invalid value for %s: %w", lineNum, name, err)
		}
		tree.Set(name, normalizeSetValue(leaf.Type, value))
		return nil
	}

	if multi, ok := node.(*MultiLeafNode); ok {
		if len(tokens) < 1 {
			return fmt.Errorf("line %d: multi-leaf %s expects at least one value", lineNum, name)
		}
		value := textbuf.Join(tokens, " ")
		if err := validateValuePatterns(multi.Type, multi.Patterns, value); err != nil {
			return fmt.Errorf("line %d: invalid value for %s: %w", lineNum, name, err)
		}
		tree.Set(name, value)
		return nil
	}

	if bracket, ok := node.(*BracketLeafListNode); ok {
		if len(tokens) == 0 {
			return nil // structural-only: no value to set
		}
		for _, item := range bracketItems(tokens) {
			if err := validateValuePatterns(bracket.Type, bracket.Patterns, item); err != nil {
				return fmt.Errorf("line %d: invalid value for %s: %w", lineNum, name, err)
			}
		}
		tree.Set(name, parseBracketValue(tokens))
		return nil
	}

	if valueOrArray, ok := node.(*ValueOrArrayNode); ok {
		if len(tokens) == 0 {
			return nil // structural-only: no value to set
		}
		// Normalize an inline "inactive:" member prefix into the out-of-band
		// marker (same accepted input form as the hierarchical parser), so the
		// stored member value stays clean and is validated clean.
		clean, deactivated := stripInactiveMemberPrefix(bracketItems(tokens))
		if err := validateValueOrArrayItems(valueOrArray, name, clean, lineNum); err != nil {
			return err
		}
		// Add-member merge: each line appends missing members (JunOS set
		// semantics); members land in the multi-value store the serializers
		// read, with the scalar map kept in sync for Get() callers.
		for _, item := range clean {
			tree.AddMultiValueMember(name, item)
		}
		for _, member := range deactivated {
			_ = tree.DeactivateMultiValue(name, member)
		}
		return nil
	}

	if container, ok := node.(*ContainerNode); ok {
		child := tree.GetContainer(name)
		if child == nil {
			child = NewTree()
			tree.SetContainer(name, child)
		}
		return p.walkAndSet(child, container, tokens, lineNum)
	}

	if list, ok := node.(*ListNode); ok {
		if len(tokens) == 0 {
			return nil // structural-only: list node declared, no entries
		}
		if len(tokens) == 1 {
			// Key-only: create empty list entry (incomplete config tolerance).
			key := tokens[0]
			if err := ValidateListKey(list, key); err != nil {
				return fmt.Errorf("line %d: invalid key for %s: %w", lineNum, name, err)
			}
			entries := tree.GetList(name)
			if entries == nil || entries[key] == nil {
				tree.AddListEntry(name, key, NewTree())
			}
			return nil
		}
		return p.walkAndSetListEntry(tree, list, name, tokens, lineNum)
	}

	if _, ok := node.(*FreeformNode); ok {
		if len(tokens) == 0 {
			return nil // structural-only: no key to set
		}
		return setFreeformValue(tree, name, tokens, lineNum)
	}

	if flex, ok := node.(*FlexNode); ok {
		return p.setFlexValue(tree, flex, name, tokens, lineNum)
	}

	if il, ok := node.(*InlineListNode); ok {
		if len(tokens) == 0 {
			return nil // structural-only: inline-list declared, no entries
		}
		if len(tokens) == 1 {
			// Key-only: create empty inline-list entry (incomplete config tolerance).
			key := tokens[0]
			entries := tree.GetList(name)
			if entries == nil || entries[key] == nil {
				tree.AddListEntry(name, key, NewTree())
			}
			return nil
		}
		return p.walkAndSetInlineListEntry(tree, il, name, tokens, lineNum)
	}

	return fmt.Errorf("line %d: unknown node type %T for %s", lineNum, node, name)
}

// parseNop handles: nop <path...> [<value>].
// Sets the value (like parseSet) and marks the resolved node inactive in one step.
func (p *SetParser) parseNop(tree *Tree, tokens []string, lineNum int) error {
	if len(tokens) < 1 {
		return fmt.Errorf("line %d: nop requires at least a path", lineNum)
	}
	if err := p.walkAndSet(tree, p.schema.root, tokens, lineNum); err != nil {
		return err
	}
	p.markNopInactive(tree, p.schema.root, tokens)
	return nil
}

// markNopInactive walks the schema path and marks the terminal node
// inactive. Unlike walkAndMarkInactive, it understands that leaf value tokens
// are part of the token stream (consumed by the prior walkAndSet) and stops
// at the leaf name rather than trying to descend into the value.
//
//nolint:cyclop // exhaustive node-type handling mirrors walkAndMarkInactive
func (p *SetParser) markNopInactive(tree *Tree, parent Node, tokens []string) {
	if len(tokens) == 0 {
		return
	}

	name := tokens[0]
	tokens = tokens[1:]

	node := resolveSchemaNode(p.schema, parent, name)
	if node == nil {
		return
	}

	if _, ok := node.(*LeafNode); ok {
		tree.SetLeafInactive(name, true)
		return
	}
	if _, ok := node.(*MultiLeafNode); ok {
		tree.SetLeafInactive(name, true)
		return
	}
	if _, ok := node.(*BracketLeafListNode); ok {
		tree.SetLeafInactive(name, true)
		return
	}

	if _, ok := node.(*ValueOrArrayNode); ok {
		for _, item := range bracketItems(tokens) {
			_ = tree.DeactivateMultiValue(name, item)
		}
		return
	}

	if container, ok := node.(*ContainerNode); ok {
		child := tree.GetContainer(name)
		if child == nil {
			return
		}
		if len(tokens) == 0 {
			child.SetInactive(true)
			return
		}
		p.markNopInactive(child, container, tokens)
		return
	}

	if list, ok := node.(*ListNode); ok {
		if len(tokens) == 0 {
			return
		}
		key := tokens[0]
		entries := tree.GetList(name)
		entry := entries[key]
		if entry == nil {
			return
		}
		if len(tokens) == 1 {
			entry.SetInactive(true)
			return
		}
		p.markNopInactive(entry, list, tokens[1:])
		return
	}

	if flex, ok := node.(*FlexNode); ok {
		child := tree.GetContainer(name)
		if child == nil {
			child = NewTree()
			tree.SetContainer(name, child)
		}
		if len(tokens) == 0 {
			child.SetInactive(true)
			return
		}
		p.markNopInactive(child, flex, tokens)
	}
}

// parseInactive handles: inactive <path...>.
//
// Single-keyword design: the presence of `inactive <path>` declares the
// node at <path> inactive. There is no symmetric "active" or
// "deactivate"/"activate" verb pair -- absence of the line means the
// node is active. Removing the inactive declaration (manually, or by
// the editor's CLI/TUI activate path which re-serializes without it)
// re-activates the node.
//
// The path resolves to a leaf, container, list entry, or single
// leaf-list value; dispatch mirrors the block-format `inactive:`
// prefix and the one-shot CLI verbs.
func (p *SetParser) parseInactive(tree *Tree, tokens []string, lineNum int) error {
	if len(tokens) < 1 {
		return fmt.Errorf("line %d: inactive requires a path", lineNum)
	}
	return p.walkAndMarkInactive(tree, p.schema.root, tokens, lineNum)
}

// walkAndMarkInactive walks the path through schema and tree and sets
// the inactive marker at the resolved node.
//
//nolint:cyclop // exhaustive node-type handling, mirrors walkAndDelete
func (p *SetParser) walkAndMarkInactive(tree *Tree, parent Node, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		return fmt.Errorf("line %d: incomplete inactive path", lineNum)
	}

	name := tokens[0]
	tokens = tokens[1:]

	node := resolveSchemaNode(p.schema, parent, name)
	if node == nil {
		// In pre-migration mode unknown fields are warnings rather than
		// errors so that a renamed YANG path can still parse and the
		// migration step can rewrite it. Mirror the set / delete paths.
		if p.preMigration {
			var bw textbuf.Buffer
			p.warnings = append(p.warnings, bw.Reset().Str("line ").Int(int64(lineNum)).Str(": unknown field: ").Str(name).Str(" (needs migration)").String())
			return nil
		}
		return fmt.Errorf("line %d: unknown field: %s%s", lineNum, name, RetiredKeywordHint(name))
	}

	if _, ok := node.(*LeafNode); ok {
		if len(tokens) != 0 {
			return fmt.Errorf("line %d: unexpected tokens after leaf %s", lineNum, name)
		}
		tree.SetLeafInactive(name, true)
		return nil
	}
	if _, ok := node.(*MultiLeafNode); ok {
		tree.SetLeafInactive(name, true)
		return nil
	}
	if _, ok := node.(*BracketLeafListNode); ok {
		tree.SetLeafInactive(name, true)
		return nil
	}

	// ValueOrArray: a final token denotes a single leaf-list value
	// (deactivate one element); no token denotes the whole leaf-list.
	if _, ok := node.(*ValueOrArrayNode); ok {
		if len(tokens) == 1 {
			return tree.DeactivateMultiValue(name, tokens[0])
		}
		if len(tokens) == 0 {
			tree.SetLeafInactive(name, true)
			return nil
		}
		return fmt.Errorf("line %d: inactive on leaf-list expects either no value or one value, got %d", lineNum, len(tokens))
	}

	// Container: mark via the Tree-level inactive bool.
	if container, ok := node.(*ContainerNode); ok {
		child := tree.GetContainer(name)
		if child == nil {
			child = NewTree()
			tree.SetContainer(name, child)
		}
		if len(tokens) == 0 {
			child.SetInactive(true)
			return nil
		}
		return p.walkAndMarkInactive(child, container, tokens, lineNum)
	}

	// List: a key token selects an entry; further tokens descend
	// into the entry's fields.
	if list, ok := node.(*ListNode); ok {
		if len(tokens) == 0 {
			return fmt.Errorf("line %d: inactive on list %s requires a key", lineNum, name)
		}
		key := tokens[0]
		entries := tree.GetList(name)
		entry := entries[key]
		if entry == nil {
			entry = NewTree()
			tree.AddListEntry(name, key, entry)
		}
		if len(tokens) == 1 {
			entry.SetInactive(true)
			return nil
		}
		return p.walkAndMarkInactive(entry, list, tokens[1:], lineNum)
	}

	if flex, ok := node.(*FlexNode); ok {
		child := tree.GetContainer(name)
		if child == nil {
			child = NewTree()
			tree.SetContainer(name, child)
		}
		if len(tokens) == 0 {
			child.SetInactive(true)
			return nil
		}
		return p.walkAndMarkInactive(child, flex, tokens, lineNum)
	}

	return fmt.Errorf("line %d: inactive not supported on %T (%s)", lineNum, node, name)
}

// parseDelete handles: delete <path...>.
func (p *SetParser) parseDelete(tree *Tree, tokens []string, lineNum int) error {
	if len(tokens) < 1 {
		return fmt.Errorf("line %d: delete requires path", lineNum)
	}

	return p.walkAndDelete(tree, p.schema.root, tokens, lineNum)
}

// walkAndDelete walks the path and deletes the target.
//
//nolint:cyclop // exhaustive node type handling
func (p *SetParser) walkAndDelete(tree *Tree, parent Node, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		return fmt.Errorf("line %d: incomplete delete path", lineNum)
	}

	name := tokens[0]
	tokens = tokens[1:]

	node := resolveSchemaNode(p.schema, parent, name)
	if node == nil {
		if p.preMigration {
			var bw textbuf.Buffer
			p.warnings = append(p.warnings, bw.Reset().Str("line ").Int(int64(lineNum)).Str(": unknown field: ").Str(name).Str(" (needs migration)").String())
			return nil
		}
		return fmt.Errorf("line %d: unknown field: %s%s", lineNum, name, RetiredKeywordHint(name))
	}

	// Leaf-list member delete: `delete <path> <member>` removes one member.
	if _, ok := node.(*ValueOrArrayNode); ok && len(tokens) == 1 {
		tree.RemoveMultiValueMember(name, tokens[0])
		return nil
	}

	// Leaf-like types: delete the value directly.
	if isLeafLike(node) {
		if len(tokens) != 0 {
			return fmt.Errorf("line %d: unexpected tokens after leaf %s", lineNum, name)
		}
		tree.Delete(name)
		return nil
	}

	if container, ok := node.(*ContainerNode); ok {
		if len(tokens) == 0 {
			tree.DeleteContainer(name)
			return nil
		}
		child := tree.GetContainer(name)
		if child == nil {
			return nil // Already doesn't exist
		}
		return p.walkAndDelete(child, container, tokens, lineNum)
	}

	if list, ok := node.(*ListNode); ok {
		return p.deleteFromList(tree, list, name, tokens, lineNum)
	}

	if _, ok := node.(*FreeformNode); ok {
		return deleteFreeformEntry(tree, name, tokens, lineNum)
	}

	if flex, ok := node.(*FlexNode); ok {
		if len(tokens) == 0 {
			// Delete the flex value/container.
			tree.Delete(name)
			tree.DeleteContainer(name)
			return nil
		}
		child := tree.GetContainer(name)
		if child == nil {
			return nil
		}
		return p.walkAndDelete(child, flex, tokens, lineNum)
	}

	if il, ok := node.(*InlineListNode); ok {
		return p.deleteFromInlineList(tree, il, name, tokens, lineNum)
	}

	return fmt.Errorf("line %d: unknown node type %T for %s", lineNum, node, name)
}

// isLeafLike returns true for terminal node types that store a single value.
func isLeafLike(node Node) bool {
	switch node.(type) {
	case *LeafNode, *MultiLeafNode, *BracketLeafListNode, *ValueOrArrayNode:
		return true
	}
	return false
}

// validateValueOrArrayItems checks every leaf-list item against the node's
// enum values and type patterns. Shared by the plain and metadata-aware
// set parsers.
func validateValueOrArrayItems(node *ValueOrArrayNode, name string, items []string, lineNum int) error {
	for _, item := range items {
		if node.ValidValues != nil && !containsString(node.ValidValues, item) {
			return fmt.Errorf("line %d: invalid value for %s: %q (valid: %s)", lineNum, name, item, textbuf.Join(node.ValidValues, ", "))
		}
		if err := validateValuePatterns(node.Type, node.Patterns, item); err != nil {
			return fmt.Errorf("line %d: invalid value for %s: %w", lineNum, name, err)
		}
	}
	return nil
}

// deleteFromList handles delete for ListNode (entire list, entry, or field within entry).
func (p *SetParser) deleteFromList(tree *Tree, list *ListNode, name string, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		tree.DeleteList(name)
		return nil
	}
	key := tokens[0]
	tokens = tokens[1:]
	entries := tree.GetList(name)
	if entries == nil {
		return nil
	}
	if len(tokens) == 0 {
		delete(entries, key)
		return nil
	}
	entry := entries[key]
	if entry == nil {
		return nil
	}
	return p.walkAndDelete(entry, list, tokens, lineNum)
}

// deleteFreeformEntry handles delete for FreeformNode entries.
func deleteFreeformEntry(tree *Tree, name string, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		tree.DeleteContainer(name)
		return nil
	}
	if len(tokens) != 1 {
		return fmt.Errorf("line %d: freeform delete expects 0 or 1 key after %s", lineNum, name)
	}
	child := tree.GetContainer(name)
	if child == nil {
		return nil
	}
	child.Delete(tokens[0])
	return nil
}

// deleteFromInlineList handles delete for InlineListNode entries.
func (p *SetParser) deleteFromInlineList(tree *Tree, il *InlineListNode, name string, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		tree.DeleteList(name)
		return nil
	}
	key := tokens[0]
	tokens = tokens[1:]
	entries := tree.GetList(name)
	if entries == nil {
		return nil
	}
	if len(tokens) == 0 {
		delete(entries, key)
		return nil
	}
	entry := entries[key]
	if entry == nil {
		return nil
	}
	return p.walkAndDelete(entry, il, tokens, lineNum)
}

// Delete removes a leaf value, its multi-value members, and its
// insertion-order entry. Clearing multiValues matters for leaf-lists: the
// serializers read the multi-value store, so leaving it populated would
// resurrect a deleted leaf-list on the next serialize.
// No-op if the key does not exist.
func (t *Tree) Delete(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.multiValues, name)
	if _, exists := t.values[name]; !exists {
		return
	}
	delete(t.values, name)

	// Remove from valuesOrder
	for i, k := range t.valuesOrder {
		if k == name {
			t.valuesOrder = append(t.valuesOrder[:i], t.valuesOrder[i+1:]...)
			break
		}
	}
}

// DeleteContainer removes a container from the tree.
func (t *Tree) DeleteContainer(name string) {
	delete(t.containers, name)
}

// DeleteList removes an entire list from the tree.
func (t *Tree) DeleteList(name string) {
	delete(t.lists, name)
}

// resolveSchemaNode looks up a child node from the parent, handling the root case.
// Supports all parent types that have children: Container, List, Flex, InlineList.
func resolveSchemaNode(schema *Schema, parent Node, name string) Node {
	if parent == nil {
		return schema.Get(name)
	}
	if c, ok := parent.(*ContainerNode); ok {
		return c.Get(name)
	}
	if l, ok := parent.(*ListNode); ok {
		return l.Get(name)
	}
	if f, ok := parent.(*FlexNode); ok {
		return f.Get(name)
	}
	if il, ok := parent.(*InlineListNode); ok {
		return il.Get(name)
	}
	return nil
}

// parseBracketValue joins tokens, stripping optional surrounding [ ] brackets.
// Handles: "value", "[ a b c ]", "a b c".
func parseBracketValue(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	// Strip surrounding brackets if present.
	if tokens[0] == "[" && tokens[len(tokens)-1] == "]" {
		tokens = tokens[1 : len(tokens)-1]
	}
	return textbuf.Join(tokens, " ")
}

func normalizeSetValue(typ ValueType, value string) string {
	if typ == TypeBool {
		return NormalizeBool(value)
	}
	return value
}

func bracketItems(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}
	if tokens[0] == "[" && tokens[len(tokens)-1] == "]" {
		return tokens[1 : len(tokens)-1]
	}
	return tokens
}

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

// setFlexValue handles FlexNode's multiple forms: flag, value, or container with children.
// If remaining tokens match a known child name, recurse. Otherwise treat as a leaf value.
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

	// Otherwise treat as a simple value.
	tree.Set(name, textbuf.Join(tokens, " "))
	return nil
}

// walkAndSetListEntry creates or finds a list entry and recurses.
func (p *SetParser) walkAndSetListEntry(tree *Tree, list *ListNode, name string, tokens []string, lineNum int) error {
	key := tokens[0]
	tokens = tokens[1:]
	if err := ValidateListKey(list, key); err != nil {
		return fmt.Errorf("line %d: invalid key for %s: %w", lineNum, name, err)
	}
	entries := tree.GetList(name)
	if entries == nil {
		entries = make(map[string]*Tree)
	}
	entry := entries[key]
	if entry == nil {
		entry = NewTree()
		tree.AddListEntry(name, key, entry)
	}
	return p.walkAndSet(entry, list, tokens, lineNum)
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

// Inline arg parsing moved to setparser_inline.go
