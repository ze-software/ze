// Design: docs/architecture/config/syntax.md — config parsing and loading
// Related: parser.go — config parser core
// Related: serialize_set.go — set-format serialization (inverse of this file)
// Related: meta.go — MetaTree for metadata-aware parsing

package config

import (
	"bufio"
	"fmt"
	"strings"
	"time"
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

	for i := range len(line) {
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

// parseSet handles: set <path...> <value>.
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

	//nolint:gocritic // if-else chain preferred over type switch for exhaustive node handling
	if leaf, ok := node.(*LeafNode); ok {
		_ = leaf
		if len(tokens) != 1 {
			return fmt.Errorf("line %d: leaf %s expects exactly one value", lineNum, name)
		}
		tree.Set(name, tokens[0])
		return nil
	}

	if _, ok := node.(*MultiLeafNode); ok {
		if len(tokens) < 1 {
			return fmt.Errorf("line %d: multi-leaf %s expects at least one value", lineNum, name)
		}
		tree.Set(name, strings.Join(tokens, " "))
		return nil
	}

	if _, ok := node.(*BracketLeafListNode); ok {
		tree.Set(name, parseBracketValue(tokens))
		return nil
	}

	if _, ok := node.(*ValueOrArrayNode); ok {
		tree.Set(name, parseBracketValue(tokens))
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
		if len(tokens) < 2 {
			return fmt.Errorf("line %d: list %s requires key and field", lineNum, name)
		}
		return p.walkAndSetListEntry(tree, list, name, tokens, lineNum)
	}

	if _, ok := node.(*FreeformNode); ok {
		return setFreeformValue(tree, name, tokens, lineNum)
	}

	if flex, ok := node.(*FlexNode); ok {
		return p.setFlexValue(tree, flex, name, tokens, lineNum)
	}

	if il, ok := node.(*InlineListNode); ok {
		if len(tokens) < 2 {
			return fmt.Errorf("line %d: inline-list %s requires key and field", lineNum, name)
		}
		return p.walkAndSetInlineListEntry(tree, il, name, tokens, lineNum)
	}

	return fmt.Errorf("line %d: unknown node type %T for %s", lineNum, node, name)
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
		return fmt.Errorf("line %d: unknown field: %s", lineNum, name)
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

// Delete removes a leaf value and its insertion-order entry.
// No-op if the key does not exist.
func (t *Tree) Delete(name string) {
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

// ParseWithMeta parses set-format input with optional metadata prefixes.
// Returns both the config Tree and a MetaTree with authorship information.
//
// Metadata tokens are consumed before the set/delete command:
//   - #user -> MetaEntry.User
//   - @source -> MetaEntry.Source (connection origin)
//   - %ISO8601 -> MetaEntry.Time (session start time)
//   - "# text" (hash + space) -> comment, line skipped
func (p *SetParser) ParseWithMeta(input string) (*Tree, *MetaTree, error) {
	tree := NewTree()
	meta := NewMetaTree()

	scanner := bufio.NewScanner(strings.NewReader(input))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		// "# text" (hash + space) is a comment
		if strings.HasPrefix(line, "# ") {
			continue
		}

		// Extract metadata tokens and the remaining command
		entry, cmdLine := extractMeta(line)

		// After stripping metadata, we may have an empty line
		if cmdLine == "" {
			continue
		}

		if err := p.parseLineWithMeta(tree, meta, cmdLine, entry, lineNum); err != nil {
			return nil, nil, err
		}
	}

	return tree, meta, scanner.Err()
}

// extractMeta consumes metadata tokens from the beginning of a line.
// Returns the MetaEntry and the remaining command string.
func extractMeta(line string) (MetaEntry, string) {
	var entry MetaEntry
	remaining := line

	for remaining != "" {
		if remaining[0] == '#' && len(remaining) > 1 && remaining[1] != ' ' {
			// User metadata: #user
			end := strings.IndexByte(remaining, ' ')
			if end == -1 {
				entry.User = remaining[1:]
				remaining = ""
			} else {
				entry.User = remaining[1:end]
				remaining = strings.TrimSpace(remaining[end+1:])
			}
			continue
		}

		if remaining[0] == '@' {
			// Source metadata: @origin (e.g., "local", "192.168.1.5")
			end := strings.IndexByte(remaining, ' ')
			if end == -1 {
				entry.Source = remaining[1:]
				remaining = ""
			} else {
				entry.Source = remaining[1:end]
				remaining = strings.TrimSpace(remaining[end+1:])
			}
			continue
		}

		if remaining[0] == '%' {
			// Time metadata: %ISO8601 (session start time)
			end := strings.IndexByte(remaining, ' ')
			var timeStr string
			if end == -1 {
				timeStr = remaining[1:]
				remaining = ""
			} else {
				timeStr = remaining[1:end]
				remaining = strings.TrimSpace(remaining[end+1:])
			}
			if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
				entry.Time = t
			} else if t, err := time.Parse("2006-01-02T15:04:05Z", timeStr); err == nil {
				entry.Time = t
			} else if t, err := time.Parse("2006-01-02T15:04:05", timeStr); err == nil {
				entry.Time = t
			}
			continue
		}

		if remaining[0] == '^' {
			// Previous value metadata: ^value or ^"multi word value".
			// Quoted form supports backslash escapes: \" for quote, \\ for backslash.
			if len(remaining) > 2 && remaining[1] == '"' {
				// Quoted: find unescaped closing quote.
				var prev strings.Builder
				i := 2
				for i < len(remaining) {
					if remaining[i] == '\\' && i+1 < len(remaining) {
						next := remaining[i+1]
						if next == '"' || next == '\\' {
							prev.WriteByte(next)
							i += 2
							continue
						}
					}
					if remaining[i] == '"' {
						break
					}
					prev.WriteByte(remaining[i])
					i++
				}
				entry.Previous = prev.String()
				if i < len(remaining) {
					remaining = strings.TrimSpace(remaining[i+1:])
				} else {
					remaining = ""
				}
			} else {
				// Unquoted: ^value
				end := strings.IndexByte(remaining, ' ')
				if end == -1 {
					entry.Previous = remaining[1:]
					remaining = ""
				} else {
					entry.Previous = remaining[1:end]
					remaining = strings.TrimSpace(remaining[end+1:])
				}
			}
			continue
		}

		// Not a metadata token, stop consuming
		break
	}

	return entry, remaining
}

// parseLineWithMeta parses a set/delete command and records metadata.
func (p *SetParser) parseLineWithMeta(tree *Tree, meta *MetaTree, line string, entry MetaEntry, lineNum int) error {
	tokens := p.tokenizeLine(line)
	if len(tokens) == 0 {
		return nil
	}

	cmd := tokens[0]
	tokens = tokens[1:]

	switch cmd {
	case "set":
		return p.parseSetWithMeta(tree, meta, entry, tokens, lineNum)
	case "delete":
		if err := p.parseDelete(tree, tokens, lineNum); err != nil {
			return err
		}
		p.recordDeleteMeta(meta, entry, tokens)
		return nil
	}

	return fmt.Errorf("line %d: unknown command: %s (expected set/delete)", lineNum, cmd)
}

// parseSetWithMeta handles: set <path...> <value> and records metadata along the path.
func (p *SetParser) parseSetWithMeta(tree *Tree, meta *MetaTree, entry MetaEntry, tokens []string, lineNum int) error {
	if len(tokens) < 2 {
		return fmt.Errorf("line %d: set requires path and value", lineNum)
	}

	return p.walkAndSetWithMeta(tree, meta, p.schema.root, entry, tokens, lineNum)
}

// walkAndSetWithMeta walks the path, sets the value, and records metadata at the leaf.
//
//nolint:cyclop // schema node dispatch mirrors walkAndSet
func (p *SetParser) walkAndSetWithMeta(tree *Tree, meta *MetaTree, parent Node, entry MetaEntry, tokens []string, lineNum int) error {
	if len(tokens) == 0 {
		return fmt.Errorf("line %d: incomplete path", lineNum)
	}

	name := tokens[0]
	tokens = tokens[1:]

	node := resolveSchemaNode(p.schema, parent, name)
	if node == nil {
		return fmt.Errorf("line %d: unknown field: %s", lineNum, name)
	}

	hasMetadata := entry.User != "" || !entry.Time.IsZero() || entry.Source != ""

	// setLeafMeta records metadata and sets the value for a leaf-like node.
	setLeafMeta := func(value string) {
		tree.Set(name, value)
		if hasMetadata {
			entry.Value = value
			meta.SetEntry(name, entry)
		}
	}

	if _, ok := node.(*LeafNode); ok {
		if len(tokens) != 1 {
			return fmt.Errorf("line %d: leaf %s expects exactly one value", lineNum, name)
		}
		setLeafMeta(tokens[0])
		return nil
	}

	if _, ok := node.(*MultiLeafNode); ok {
		if len(tokens) < 1 {
			return fmt.Errorf("line %d: multi-leaf %s expects at least one value", lineNum, name)
		}
		setLeafMeta(strings.Join(tokens, " "))
		return nil
	}

	if _, ok := node.(*BracketLeafListNode); ok {
		setLeafMeta(parseBracketValue(tokens))
		return nil
	}

	if _, ok := node.(*ValueOrArrayNode); ok {
		setLeafMeta(parseBracketValue(tokens))
		return nil
	}

	if container, ok := node.(*ContainerNode); ok {
		child := tree.GetContainer(name)
		if child == nil {
			child = NewTree()
			tree.SetContainer(name, child)
		}
		childMeta := meta.GetOrCreateContainer(name)
		return p.walkAndSetWithMeta(child, childMeta, container, entry, tokens, lineNum)
	}

	if list, ok := node.(*ListNode); ok {
		if len(tokens) < 2 {
			return fmt.Errorf("line %d: list %s requires key and field", lineNum, name)
		}
		key := tokens[0]
		tokens = tokens[1:]
		entries := tree.GetList(name)
		if entries == nil {
			entries = make(map[string]*Tree)
		}
		treeEntry := entries[key]
		if treeEntry == nil {
			treeEntry = NewTree()
			tree.AddListEntry(name, key, treeEntry)
		}
		listMeta := meta.GetOrCreateContainer(name)
		entryMeta := listMeta.GetOrCreateListEntry(key)
		return p.walkAndSetWithMeta(treeEntry, entryMeta, list, entry, tokens, lineNum)
	}

	if _, ok := node.(*FreeformNode); ok {
		if err := setFreeformValue(tree, name, tokens, lineNum); err != nil {
			return err
		}
		if hasMetadata && len(tokens) > 0 {
			childMeta := meta.GetOrCreateContainer(name)
			entry.Value = tokens[len(tokens)-1]
			childMeta.SetEntry(tokens[0], entry)
		}
		return nil
	}

	if flex, ok := node.(*FlexNode); ok {
		// If first token matches a child, recurse into container form.
		if len(tokens) > 0 && flex.Get(tokens[0]) != nil {
			child := tree.GetOrCreateContainer(name)
			childMeta := meta.GetOrCreateContainer(name)
			return p.walkAndSetWithMeta(child, childMeta, flex, entry, tokens, lineNum)
		}
		// Otherwise treat as value/flag leaf.
		value := configTrue
		if len(tokens) > 0 {
			value = strings.Join(tokens, " ")
		}
		setLeafMeta(value)
		return nil
	}

	if il, ok := node.(*InlineListNode); ok {
		if len(tokens) < 2 {
			return fmt.Errorf("line %d: inline-list %s requires key and field", lineNum, name)
		}
		key := tokens[0]
		tokens = tokens[1:]
		entries := tree.GetList(name)
		if entries == nil {
			entries = make(map[string]*Tree)
		}
		treeEntry := entries[key]
		if treeEntry == nil {
			treeEntry = NewTree()
			tree.AddListEntry(name, key, treeEntry)
		}
		listMeta := meta.GetOrCreateContainer(name)
		entryMeta := listMeta.GetOrCreateListEntry(key)
		return p.walkAndSetWithMeta(treeEntry, entryMeta, il, entry, tokens, lineNum)
	}

	return fmt.Errorf("line %d: unknown node type %T for %s", lineNum, node, name)
}

// recordDeleteMeta walks the meta tree along the same path as a delete command
// and records metadata at the leaf. Called after parseDelete has already applied
// the deletion to the tree.
func (p *SetParser) recordDeleteMeta(meta *MetaTree, entry MetaEntry, tokens []string) {
	hasMetadata := entry.User != "" || !entry.Time.IsZero() || entry.Source != ""
	if !hasMetadata || len(tokens) == 0 {
		return
	}
	p.walkAndRecordDeleteMeta(meta, p.schema.root, entry, tokens)
}

// walkAndRecordDeleteMeta navigates the meta tree in parallel with the schema
// to find the correct position for recording delete metadata.
//
//nolint:cyclop // exhaustive node type dispatch mirrors walkAndSetWithMeta
func (p *SetParser) walkAndRecordDeleteMeta(meta *MetaTree, parent Node, entry MetaEntry, tokens []string) {
	if len(tokens) == 0 {
		return
	}

	name := tokens[0]
	tokens = tokens[1:]

	node := resolveSchemaNode(p.schema, parent, name)
	if node == nil {
		return
	}

	// Leaf-like types: record metadata at this position.
	if isLeafLike(node) {
		meta.SetEntry(name, entry)
		return
	}

	// Container: navigate into child.
	if container, ok := node.(*ContainerNode); ok {
		if len(tokens) == 0 {
			return // Container-level delete, no leaf metadata to record.
		}
		childMeta := meta.GetOrCreateContainer(name)
		p.walkAndRecordDeleteMeta(childMeta, container, entry, tokens)
		return
	}

	// List: navigate to specific entry.
	if list, ok := node.(*ListNode); ok {
		if len(tokens) < 2 {
			return // List or entry-level delete, no leaf metadata.
		}
		key := tokens[0]
		tokens = tokens[1:]
		listMeta := meta.GetOrCreateContainer(name)
		entryMeta := listMeta.GetOrCreateListEntry(key)
		p.walkAndRecordDeleteMeta(entryMeta, list, entry, tokens)
		return
	}

	// Flex: navigate into child.
	if flex, ok := node.(*FlexNode); ok {
		if len(tokens) == 0 {
			return
		}
		childMeta := meta.GetOrCreateContainer(name)
		p.walkAndRecordDeleteMeta(childMeta, flex, entry, tokens)
		return
	}

	// InlineList: navigate to specific entry.
	if il, ok := node.(*InlineListNode); ok {
		if len(tokens) < 2 {
			return
		}
		key := tokens[0]
		tokens = tokens[1:]
		listMeta := meta.GetOrCreateContainer(name)
		entryMeta := listMeta.GetOrCreateListEntry(key)
		p.walkAndRecordDeleteMeta(entryMeta, il, entry, tokens)
		return
	}
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
	return strings.Join(tokens, " ")
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
		value = strings.Join(tokens[1:], " ")
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
	tree.Set(name, strings.Join(tokens, " "))
	return nil
}

// walkAndSetListEntry creates or finds a list entry and recurses.
func (p *SetParser) walkAndSetListEntry(tree *Tree, list *ListNode, name string, tokens []string, lineNum int) error {
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
