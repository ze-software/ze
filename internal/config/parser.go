package config

import (
	"fmt"
	"strings"
)

// KeyDefault is the key used for anonymous list entries (e.g., "api { ... }").
const KeyDefault = "default"

// Tree represents parsed configuration data.
type Tree struct {
	values      map[string]string
	valuesOrder []string            // Preserves insertion order for value keys
	multiValues map[string][]string // For multiple inline values (e.g., multiple mup entries)
	containers  map[string]*Tree
	lists       map[string]map[string]*Tree
	listOrder   map[string][]string // Preserves insertion order for list keys
}

// NewTree creates an empty config tree.
func NewTree() *Tree {
	return &Tree{
		values:      make(map[string]string),
		multiValues: make(map[string][]string),
		containers:  make(map[string]*Tree),
		lists:       make(map[string]map[string]*Tree),
		listOrder:   make(map[string][]string),
	}
}

// Get returns a leaf value.
func (t *Tree) Get(name string) (string, bool) {
	v, ok := t.values[name]
	return v, ok
}

// Set sets a leaf value.
func (t *Tree) Set(name, value string) {
	// Track insertion order for new keys
	if _, exists := t.values[name]; !exists {
		t.valuesOrder = append(t.valuesOrder, name)
	}
	t.values[name] = value
}

// AppendValue appends a value to the multi-values list (for Flex nodes with multiple entries).
func (t *Tree) AppendValue(name, value string) {
	t.multiValues[name] = append(t.multiValues[name], value)
}

// GetMultiValues returns all values for a multi-value field.
func (t *Tree) GetMultiValues(name string) []string {
	return t.multiValues[name]
}

// Clone creates a deep copy of the Tree.
// Used by migrations to safely transform config without affecting original.
func (t *Tree) Clone() *Tree {
	if t == nil {
		return nil
	}

	clone := NewTree()

	// Clone values
	for k, v := range t.values {
		clone.values[k] = v
	}

	// Clone multiValues
	for k, v := range t.multiValues {
		copied := make([]string, len(v))
		copy(copied, v)
		clone.multiValues[k] = copied
	}

	// Clone containers (deep)
	for k, v := range t.containers {
		clone.containers[k] = v.Clone()
	}

	// Clone lists (deep)
	for listName, entries := range t.lists {
		clone.lists[listName] = make(map[string]*Tree)
		for entryKey, entryTree := range entries {
			clone.lists[listName][entryKey] = entryTree.Clone()
		}
	}

	// Clone listOrder
	for k, v := range t.listOrder {
		copied := make([]string, len(v))
		copy(copied, v)
		clone.listOrder[k] = copied
	}

	return clone
}

// GetFlex returns a value from either leaf values or the first multiValue.
// Used for Flex nodes that can be parsed as either Set() or AppendValue().
func (t *Tree) GetFlex(name string) (string, bool) {
	if v, ok := t.values[name]; ok {
		return v, true
	}
	if mv := t.multiValues[name]; len(mv) > 0 {
		return mv[0], true
	}
	return "", false
}

// GetContainer returns a nested container.
func (t *Tree) GetContainer(name string) *Tree {
	return t.containers[name]
}

// SetContainer sets a nested container.
func (t *Tree) SetContainer(name string, child *Tree) {
	t.containers[name] = child
}

// RemoveContainer removes a nested container and returns it.
// Returns nil if the container doesn't exist.
func (t *Tree) RemoveContainer(name string) *Tree {
	c := t.containers[name]
	delete(t.containers, name)
	return c
}

// MergeContainer merges a container into existing one (or creates if not exists).
// This handles the case of multiple same-named blocks in config (e.g., multiple announce blocks).
func (t *Tree) MergeContainer(name string, child *Tree) {
	existing := t.containers[name]
	if existing == nil {
		t.containers[name] = child
		return
	}
	// Merge values.
	for k, v := range child.values {
		existing.values[k] = v
	}
	// Merge multiValues (append).
	for k, v := range child.multiValues {
		existing.multiValues[k] = append(existing.multiValues[k], v...)
	}
	// Merge containers (recursively).
	for k, v := range child.containers {
		existing.MergeContainer(k, v)
	}
	// Merge lists (preserving order).
	for k, v := range child.lists {
		if existing.lists[k] == nil {
			existing.lists[k] = v
			existing.listOrder[k] = child.listOrder[k]
		} else {
			// Append new keys in child's order.
			for _, key := range child.listOrder[k] {
				if _, exists := existing.lists[k][key]; !exists {
					existing.listOrder[k] = append(existing.listOrder[k], key)
				}
				existing.lists[k][key] = v[key]
			}
		}
	}
}

// GetList returns a list (keyed map of trees).
func (t *Tree) GetList(name string) map[string]*Tree {
	return t.lists[name]
}

// AddListEntry adds an entry to a list.
// For duplicate keys, generates unique keys by appending #N suffix.
// This supports ADD-PATH routes with same prefix but different path-info.
func (t *Tree) AddListEntry(name, key string, entry *Tree) {
	if t.lists[name] == nil {
		t.lists[name] = make(map[string]*Tree)
	}

	// Generate unique key for duplicates
	uniqueKey := key
	if _, exists := t.lists[name][key]; exists {
		// Find next available suffix
		for i := 1; ; i++ {
			uniqueKey = fmt.Sprintf("%s#%d", key, i)
			if _, exists := t.lists[name][uniqueKey]; !exists {
				break
			}
		}
	}

	t.listOrder[name] = append(t.listOrder[name], uniqueKey)
	t.lists[name][uniqueKey] = entry
}

// GetListOrdered returns list entries in insertion order.
func (t *Tree) GetListOrdered(name string) []struct {
	Key   string
	Value *Tree
} {
	order := t.listOrder[name]
	list := t.lists[name]
	if list == nil {
		return nil
	}
	result := make([]struct {
		Key   string
		Value *Tree
	}, 0, len(order))
	for _, key := range order {
		if entry, ok := list[key]; ok {
			result = append(result, struct {
				Key   string
				Value *Tree
			}{key, entry})
		}
	}
	return result
}

// ListKeys returns the keys for a list (e.g., neighbor IPs).
func (t *Tree) ListKeys(name string) []string {
	list := t.lists[name]
	if list == nil {
		return nil
	}
	keys := make([]string, 0, len(list))
	for k := range list {
		keys = append(keys, k)
	}
	return keys
}

// Values returns all value keys in insertion order (for iterating Freeform entries).
func (t *Tree) Values() []string {
	// Return in insertion order if available, otherwise fallback to map order
	if len(t.valuesOrder) > 0 {
		return t.valuesOrder
	}
	keys := make([]string, 0, len(t.values))
	for k := range t.values {
		keys = append(keys, k)
	}
	return keys
}

// GetOrCreateContainer returns an existing container or creates a new one.
// Used by migrations to ensure a container exists before adding to it.
func (t *Tree) GetOrCreateContainer(name string) *Tree {
	if c := t.containers[name]; c != nil {
		return c
	}
	c := NewTree()
	t.containers[name] = c
	return c
}

// RemoveListEntry removes and returns a specific list entry.
// Returns nil if the entry doesn't exist.
func (t *Tree) RemoveListEntry(listName, key string) *Tree {
	list := t.lists[listName]
	if list == nil {
		return nil
	}
	entry, exists := list[key]
	if !exists {
		return nil
	}
	delete(list, key)

	// Remove from order
	newOrder := make([]string, 0, len(t.listOrder[listName]))
	for _, k := range t.listOrder[listName] {
		if k != key {
			newOrder = append(newOrder, k)
		}
	}
	t.listOrder[listName] = newOrder

	return entry
}

// ClearList removes all entries from a list.
// Reserved for future migrations that need bulk list replacement.
// Current migration uses RemoveListEntry for order preservation.
func (t *Tree) ClearList(name string) {
	delete(t.lists, name)
	delete(t.listOrder, name)
}

// Parser parses ExaBGP-style configuration.
type Parser struct {
	schema   *Schema
	tok      *Tokenizer
	warnings []string
}

// NewParser creates a new parser with the given schema.
func NewParser(schema *Schema) *Parser {
	return &Parser{schema: schema}
}

// Parse parses the input string into a config tree.
func (p *Parser) Parse(input string) (*Tree, error) {
	p.tok = NewTokenizer(input)
	p.warnings = nil
	return p.parseRoot()
}

// Warnings returns any warnings generated during parsing.
func (p *Parser) Warnings() []string {
	return p.warnings
}

func (p *Parser) warn(line int, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	p.warnings = append(p.warnings, fmt.Sprintf("line %d: %s", line, msg))
}

// parseRoot parses the top level of the config.
func (p *Parser) parseRoot() (*Tree, error) {
	tree := NewTree()

	for {
		tok := p.tok.Peek()
		if tok.Type == TokenEOF {
			break
		}

		if tok.Type != TokenWord {
			return nil, p.errorf(tok, "expected keyword, got %s", tok.Type)
		}

		name := tok.Value
		p.tok.Next() // consume name

		node := p.schema.Get(name)
		if node == nil {
			return nil, p.errorf(tok, "unknown top-level keyword: %s", name)
		}

		if err := p.parseNode(tree, name, node); err != nil {
			return nil, err
		}
	}

	return tree, nil
}

// parseNode parses a node based on its schema type.
func (p *Parser) parseNode(tree *Tree, name string, node Node) error {
	switch n := node.(type) {
	case *LeafNode:
		return p.parseLeaf(tree, name, n)
	case *MultiLeafNode:
		return p.parseMultiLeaf(tree, name, n)
	case *BracketLeafListNode:
		return p.parseBracketLeafList(tree, name, n)
	case *ValueOrArrayNode:
		return p.parseValueOrArray(tree, name, n)
	case *ContainerNode:
		return p.parseContainer(tree, name, n)
	case *ListNode:
		return p.parseList(tree, name, n)
	case *FreeformNode:
		return p.parseFreeform(tree, name)
	case *FlexNode:
		return p.parseFlex(tree, name, n)
	case *InlineListNode:
		return p.parseInlineList(tree, name, n)
	case *FamilyBlockNode:
		return p.parseFamilyBlock(tree, name)
	default:
		return fmt.Errorf("unknown node type for %s", name)
	}
}

// parseLeaf parses a leaf value: `name value;`.
func (p *Parser) parseLeaf(tree *Tree, name string, node *LeafNode) error {
	tok := p.tok.Peek()

	var value string
	if tok.Type == TokenWord || tok.Type == TokenString {
		value = tok.Value
		p.tok.Next()
	} else {
		return p.errorf(tok, "expected value for %s, got %s", name, tok.Type)
	}

	// Validate value type
	if err := ValidateValue(node.Type, value); err != nil {
		return p.errorf(tok, "invalid value for %s: %v", name, err)
	}

	// Normalize bool values (enable->true, disable->false)
	if node.Type == TypeBool {
		value = NormalizeBool(value)
	}

	// Expect semicolon
	tok = p.tok.Peek()
	if tok.Type != TokenSemicolon {
		return p.errorf(tok, "expected ';' after %s value, got %s", name, tok.Type)
	}
	p.tok.Next()

	tree.Set(name, value)
	return nil
}

// parseContainer parses a container block: `name { ... }`.
func (p *Parser) parseContainer(tree *Tree, name string, node *ContainerNode) error {
	tok := p.tok.Peek()
	if tok.Type != TokenLBrace {
		return p.errorf(tok, "expected '{' after %s, got %s", name, tok.Type)
	}
	p.tok.Next()

	child := NewTree()

	for {
		tok = p.tok.Peek()
		if tok.Type == TokenRBrace {
			p.tok.Next()
			break
		}
		if tok.Type == TokenEOF {
			return p.errorf(tok, "unexpected EOF in %s block", name)
		}
		if tok.Type != TokenWord {
			return p.errorf(tok, "expected keyword in %s block, got %s", name, tok.Type)
		}

		fieldName := tok.Value
		p.tok.Next()

		fieldNode := node.Get(fieldName)
		if fieldNode == nil {
			return p.errorf(tok, "unknown field in %s: %s (line %d)", name, fieldName, tok.Line)
		}

		if err := p.parseNode(child, fieldName, fieldNode); err != nil {
			return err
		}
	}

	tree.MergeContainer(name, child)
	return nil
}

// parseList parses a list entry: `name key { ... }` or `name { ... }` (anonymous).
// Anonymous entries use KeyDefault as the key.
func (p *Parser) parseList(tree *Tree, name string, node *ListNode) error {
	tok := p.tok.Peek()
	var key string

	// Check for anonymous block (no key, direct `{`) or named entry
	switch tok.Type { //nolint:exhaustive // default handles all other token types
	case TokenLBrace:
		// Anonymous entry - use default key
		key = KeyDefault
	case TokenWord, TokenString:
		// Named entry
		key = tok.Value
		p.tok.Next()

		// Validate key type
		if err := ValidateValue(node.KeyType, key); err != nil {
			return p.errorf(tok, "invalid key for %s: %v", name, err)
		}

		// Now check for opening brace
		tok = p.tok.Peek()
	default:
		return p.errorf(tok, "expected key or '{' for %s, got %s", name, tok.Type)
	}

	// Expect opening brace
	if tok.Type != TokenLBrace {
		return p.errorf(tok, "expected '{' after %s, got %s", name, tok.Type)
	}
	p.tok.Next()

	entry := NewTree()

	for {
		tok = p.tok.Peek()
		if tok.Type == TokenRBrace {
			p.tok.Next()
			break
		}
		if tok.Type == TokenEOF {
			return p.errorf(tok, "unexpected EOF in %s block", name)
		}
		if tok.Type != TokenWord {
			return p.errorf(tok, "expected keyword in %s block, got %s", name, tok.Type)
		}

		fieldName := tok.Value
		p.tok.Next()

		fieldNode := node.Get(fieldName)
		if fieldNode == nil {
			return p.errorf(tok, "unknown field in %s: %s (line %d)", name, fieldName, tok.Line)
		}

		if err := p.parseNode(entry, fieldName, fieldNode); err != nil {
			return err
		}
	}

	tree.AddListEntry(name, key, entry)
	return nil
}

// parseMultiLeaf parses multiple words until semicolon: `name word word;`.
func (p *Parser) parseMultiLeaf(tree *Tree, name string, _ *MultiLeafNode) error {
	var words []string

	for {
		tok := p.tok.Peek()
		if tok.Type == TokenSemicolon {
			p.tok.Next()
			break
		}
		if tok.Type == TokenWord || tok.Type == TokenString {
			words = append(words, tok.Value)
			p.tok.Next()
		} else {
			return p.errorf(tok, "expected value or ';' for %s, got %s", name, tok.Type)
		}
	}

	value := ""
	for i, w := range words {
		if i > 0 {
			value += " "
		}
		value += w
	}

	tree.Set(name, value)
	return nil
}

// parseBracketLeafList parses a bracketed leaf-list: `name [ item item ... ];`.
func (p *Parser) parseBracketLeafList(tree *Tree, name string, _ *BracketLeafListNode) error {
	tok := p.tok.Peek()
	if tok.Type != TokenLBracket {
		return p.errorf(tok, "expected '[' after %s, got %s", name, tok.Type)
	}
	p.tok.Next() // consume [

	var items []string

	for {
		tok = p.tok.Peek()
		if tok.Type == TokenRBracket {
			p.tok.Next() // consume ]
			break
		}
		if tok.Type == TokenWord || tok.Type == TokenString {
			items = append(items, tok.Value)
			p.tok.Next()
		} else {
			return p.errorf(tok, "expected item or ']' in array %s, got %s", name, tok.Type)
		}
	}

	// Expect semicolon
	tok = p.tok.Peek()
	if tok.Type != TokenSemicolon {
		return p.errorf(tok, "expected ';' after %s array, got %s", name, tok.Type)
	}
	p.tok.Next()

	// Store as space-separated string
	value := ""
	for i, item := range items {
		if i > 0 {
			value += " "
		}
		value += item
	}

	tree.Set(name, value)
	return nil
}

// parseValueOrArray parses either "value;" or "[ item item ... ];".
// Stores result as space-separated string in both cases.
func (p *Parser) parseValueOrArray(tree *Tree, name string, _ *ValueOrArrayNode) error {
	tok := p.tok.Peek()

	// Check if it's an array (starts with [)
	if tok.Type == TokenLBracket {
		p.tok.Next() // consume [

		var items []string
		for {
			tok = p.tok.Peek()
			if tok.Type == TokenRBracket {
				p.tok.Next() // consume ]
				break
			}
			if tok.Type == TokenWord || tok.Type == TokenString {
				items = append(items, tok.Value)
				p.tok.Next()
			} else {
				return p.errorf(tok, "expected item or ']' in %s, got %s", name, tok.Type)
			}
		}

		// Expect semicolon
		tok = p.tok.Peek()
		if tok.Type != TokenSemicolon {
			return p.errorf(tok, "expected ';' after %s, got %s", name, tok.Type)
		}
		p.tok.Next()

		// Store as space-separated string
		value := ""
		for i, item := range items {
			if i > 0 {
				value += " "
			}
			value += item
		}
		tree.Set(name, value)
		return nil
	}

	// Otherwise, parse as a single value (or multiple space-separated values)
	var items []string
	for {
		tok = p.tok.Peek()
		if tok.Type == TokenSemicolon {
			p.tok.Next() // consume ;
			break
		}
		if tok.Type == TokenWord || tok.Type == TokenString {
			items = append(items, tok.Value)
			p.tok.Next()
		} else {
			return p.errorf(tok, "expected value or ';' in %s, got %s", name, tok.Type)
		}
	}

	// Store as space-separated string
	value := ""
	for i, item := range items {
		if i > 0 {
			value += " "
		}
		value += item
	}
	tree.Set(name, value)
	return nil
}

// parseFreeform parses a freeform block: `name { word word; word word; }`
// Also handles: `name subname { ... }` (skips subname)
// Stores each "word word" line as key -> "true".
func (p *Parser) parseFreeform(tree *Tree, name string) error {
	tok := p.tok.Peek()

	// Skip optional words before the block (e.g., "api services { }")
	for tok.Type == TokenWord || tok.Type == TokenString {
		p.tok.Next()
		tok = p.tok.Peek()
	}

	if tok.Type != TokenLBrace {
		return p.errorf(tok, "expected '{' after %s, got %s", name, tok.Type)
	}
	p.tok.Next()

	child := NewTree()

	for {
		tok = p.tok.Peek()
		if tok.Type == TokenRBrace {
			p.tok.Next()
			break
		}
		if tok.Type == TokenEOF {
			return p.errorf(tok, "unexpected EOF in %s block", name)
		}

		// Collect words until semicolon or nested block
		var words []string
		hadArray := false
		startLine := p.tok.Peek().Line
		for {
			tok = p.tok.Peek()
			if tok.Type == TokenSemicolon {
				p.tok.Next()
				break
			}
			if tok.Type == TokenLBrace {
				// Warn about nested block being skipped
				key := ""
				for i, w := range words {
					if i > 0 {
						key += " "
					}
					key += w
				}
				p.warn(startLine, "freeform '%s' contains nested block '%s' - data may be lost", name, key)
				// Skip nested block
				if err := p.skipBlock(); err != nil {
					return err
				}
				break
			}
			if tok.Type == TokenLBracket {
				// Capture array [ ... ] values, preserving brackets
				arrayVals, err := p.collectArray()
				if err != nil {
					return err
				}
				// Preserve bracket syntax for freeform: "[ val1 val2 ]"
				bracketedVal := "[ " + strings.Join(arrayVals, " ") + " ]"
				words = append(words, bracketedVal)
				hadArray = true
				continue
			}
			if tok.Type == TokenRBrace || tok.Type == TokenEOF {
				break
			}
			if tok.Type == TokenWord || tok.Type == TokenString {
				words = append(words, tok.Value)
				p.tok.Next()
			} else {
				return p.errorf(tok, "unexpected token in %s block: %s", name, tok.Type)
			}
		}

		if len(words) > 0 {
			if hadArray && len(words) > 1 {
				// Array present: "processes [ watcher ];" -> key="processes", value="watcher"
				key := words[0]
				value := ""
				for i, w := range words[1:] {
					if i > 0 {
						value += " "
					}
					value += w
				}
				child.Set(key, value)
			} else {
				// No array: "ipv4/unicast;" -> key="ipv4/unicast", value="true"
				key := ""
				for i, w := range words {
					if i > 0 {
						key += " "
					}
					key += w
				}
				child.Set(key, configTrue)
			}
		}
	}

	tree.MergeContainer(name, child)
	return nil
}

// parseFamilyBlock parses a family block with inline and block syntax.
//
// Supports:
//   - Inline: "ipv4/unicast;" or "ipv4/unicast require;"
//   - Block: "ipv4 { unicast; multicast require; }"
//   - Mixed: both in same block
//
// Stores entries as "AFI SAFI" -> "MODE" where MODE is:
//   - "true" or "" for enable (default)
//   - "disable" for disable
//   - "require" for require
//
// Also handles "ignore-mismatch enable/disable" as special case.
func (p *Parser) parseFamilyBlock(tree *Tree, name string) error {
	tok := p.tok.Peek()

	if tok.Type != TokenLBrace {
		return p.errorf(tok, "expected '{' after %s, got %s", name, tok.Type)
	}
	p.tok.Next()

	child := NewTree()

	for {
		tok = p.tok.Peek()
		if tok.Type == TokenRBrace {
			p.tok.Next()
			break
		}
		if tok.Type == TokenEOF {
			return p.errorf(tok, "unexpected EOF in %s block", name)
		}
		if tok.Type != TokenWord {
			return p.errorf(tok, "expected AFI or keyword in %s block, got %s", name, tok.Type)
		}

		// First word is AFI or special keyword (ignore-mismatch)
		afi := tok.Value
		p.tok.Next()

		tok = p.tok.Peek()

		// Check for block syntax: "ipv4 { ... }"
		if tok.Type == TokenLBrace {
			p.tok.Next()
			// Parse SAFI entries inside block
			for {
				tok = p.tok.Peek()
				if tok.Type == TokenRBrace {
					p.tok.Next()
					break
				}
				if tok.Type == TokenEOF {
					return p.errorf(tok, "unexpected EOF in %s %s block", name, afi)
				}
				if tok.Type != TokenWord {
					return p.errorf(tok, "expected SAFI in %s %s block, got %s", name, afi, tok.Type)
				}

				safi := tok.Value
				p.tok.Next()

				// Check for mode or terminator
				mode := configTrue // default to enable
				tok = p.tok.Peek()
				if tok.Type == TokenWord {
					// Mode specified: require, disable, enable, true, false
					mode = tok.Value
					p.tok.Next()
					tok = p.tok.Peek()
				}

				// Expect semicolon or rbrace
				if tok.Type == TokenSemicolon {
					p.tok.Next()
				}
				// rbrace without semicolon is also valid (end of block)

				// Store as "AFI/SAFI" -> mode
				key := afi + "/" + safi
				child.Set(key, mode)
			}
		} else {
			// Inline syntax: "ipv4/unicast;" or "ipv4/unicast require;"
			// Requires "/" format (e.g., "ipv4/unicast")
			var safi string
			switch {
			case afi == "ignore-mismatch":
				// Special case: ignore-mismatch keyword
				mode := configTrue
				if tok.Type == TokenWord {
					mode = tok.Value
					p.tok.Next()
					tok = p.tok.Peek()
				}
				if tok.Type == TokenSemicolon {
					p.tok.Next()
				}
				child.Set("ignore-mismatch "+mode, configTrue)
				continue
			case strings.Contains(afi, "/"):
				// Format: "ipv4/unicast" as single token
				parts := strings.SplitN(afi, "/", 2)
				afi = parts[0]
				safi = parts[1]
			default:
				return p.errorf(tok, "expected afi/safi format (e.g., ipv4/unicast), got %q", afi)
			}

			// Check for mode or terminator
			mode := configTrue // default to enable
			if tok.Type == TokenWord {
				// Mode specified
				mode = tok.Value
				p.tok.Next()
				tok = p.tok.Peek()
			}

			// Expect semicolon or rbrace
			if tok.Type == TokenSemicolon {
				p.tok.Next()
			}

			// Store as "AFI/SAFI" -> mode
			key := afi + "/" + safi
			child.Set(key, mode)
		}
	}

	tree.MergeContainer(name, child)
	return nil
}

// parseFlex parses a flex node: flag (;), value (word;), or block ({}).
func (p *Parser) parseFlex(tree *Tree, name string, node *FlexNode) error {
	tok := p.tok.Peek()

	switch tok.Type { //nolint:exhaustive // Only specific tokens valid here, others handled in default
	case TokenSemicolon:
		// Flag mode: just the name with semicolon = true
		p.tok.Next()
		tree.Set(name, configTrue)
		return nil

	case TokenLBrace:
		// Block mode: parse as container
		p.tok.Next()
		child := NewTree()

		for {
			tok = p.tok.Peek()
			if tok.Type == TokenRBrace {
				p.tok.Next()
				break
			}
			if tok.Type == TokenEOF {
				return p.errorf(tok, "unexpected EOF in %s block", name)
			}
			if tok.Type != TokenWord {
				return p.errorf(tok, "expected keyword in %s block, got %s", name, tok.Type)
			}

			fieldName := tok.Value
			p.tok.Next()

			fieldNode := node.Get(fieldName)
			if fieldNode == nil {
				return p.errorf(tok, "unknown field in %s: %s (line %d)", name, fieldName, tok.Line)
			}

			if err := p.parseNode(child, fieldName, fieldNode); err != nil {
				return err
			}
		}

		tree.MergeContainer(name, child)
		return nil

	case TokenLParen:
		// Parenthesized mode: parse ( ... ) and optional semicolon
		parenVals, err := p.collectParenthesized()
		if err != nil {
			return err
		}
		value := ""
		for i, v := range parenVals {
			if i > 0 {
				value += " "
			}
			value += v
		}

		// Optional semicolon after parenthesized content
		tok = p.tok.Peek()
		if tok.Type == TokenSemicolon {
			p.tok.Next()
		}

		tree.Set(name, value)
		return nil

	case TokenWord, TokenString:
		// Value mode: parse multiple words until semicolon or block delimiter
		var values []string
		for tok.Type == TokenWord || tok.Type == TokenString || tok.Type == TokenLBracket || tok.Type == TokenLParen {
			switch tok.Type { //nolint:exhaustive // Only handling specific types in loop condition
			case TokenLBracket:
				// Array: collect [ ... ]
				arrayVals, err := p.collectArray()
				if err != nil {
					return err
				}
				values = append(values, "["+joinStrings(arrayVals, " ")+"]")
			case TokenLParen:
				// Parenthesized: collect ( ... )
				parenVals, err := p.collectParenthesized()
				if err != nil {
					return err
				}
				values = append(values, "("+joinStrings(parenVals, " ")+")")
			default:
				values = append(values, tok.Value)
				p.tok.Next()
			}
			tok = p.tok.Peek()
		}

		// Check if this is a named block (e.g., "vpls site5 { ... }")
		if tok.Type == TokenLBrace && len(values) == 1 {
			// Named block: the first value is the key
			key := values[0]
			p.tok.Next() // consume {

			child := NewTree()
			for {
				tok = p.tok.Peek()
				if tok.Type == TokenRBrace {
					p.tok.Next()
					break
				}
				if tok.Type == TokenEOF {
					return p.errorf(tok, "unexpected EOF in %s block", name)
				}
				if tok.Type != TokenWord {
					return p.errorf(tok, "expected keyword in %s block, got %s", name, tok.Type)
				}

				fieldName := tok.Value
				p.tok.Next()

				fieldNode := node.Get(fieldName)
				if fieldNode == nil {
					// Unknown field - store as value
					p.warnings = append(p.warnings, fmt.Sprintf("unknown field in %s.%s: %s", name, key, fieldName))
					// Consume until semicolon
					for p.tok.Peek().Type != TokenSemicolon && p.tok.Peek().Type != TokenEOF {
						p.tok.Next()
					}
					if p.tok.Peek().Type == TokenSemicolon {
						p.tok.Next()
					}
					continue
				}

				if err := p.parseNode(child, fieldName, fieldNode); err != nil {
					return err
				}
			}

			tree.AddListEntry(name, key, child)
			return nil
		}

		if tok.Type != TokenSemicolon {
			return p.errorf(tok, "expected ';' after %s value, got %s", name, tok.Type)
		}
		p.tok.Next()

		// Use AppendValue to support multiple inline entries (e.g., multiple mup routes)
		tree.AppendValue(name, joinStrings(values, " "))
		return nil

	default:
		return p.errorf(tok, "expected ';', value, or '{' for %s, got %s", name, tok.Type)
	}
}

// parseInlineList parses a list with inline or block syntax.
// Inline: "route 10.0.0.0/8 next-hop 1.1.1.1;"
// Block: "route 10.0.0.0/8 { next-hop 1.1.1.1; }".
func (p *Parser) parseInlineList(tree *Tree, name string, node *InlineListNode) error {
	// Get key
	tok := p.tok.Peek()
	var key string
	if tok.Type == TokenWord || tok.Type == TokenString {
		key = tok.Value
		p.tok.Next()
	} else {
		return p.errorf(tok, "expected key for %s, got %s", name, tok.Type)
	}

	// Validate key type
	if err := ValidateValue(node.KeyType, key); err != nil {
		return p.errorf(tok, "invalid key for %s: %v", name, err)
	}

	entry := NewTree()

	// Check for block or inline
	tok = p.tok.Peek()
	if tok.Type == TokenLBrace {
		// Block mode
		p.tok.Next()

		for {
			tok = p.tok.Peek()
			if tok.Type == TokenRBrace {
				p.tok.Next()
				break
			}
			if tok.Type == TokenEOF {
				return p.errorf(tok, "unexpected EOF in %s block", name)
			}
			if tok.Type != TokenWord {
				return p.errorf(tok, "expected keyword in %s block, got %s", name, tok.Type)
			}

			fieldName := tok.Value
			p.tok.Next()

			fieldNode := node.Get(fieldName)
			if fieldNode == nil {
				return p.errorf(tok, "unknown field in %s: %s (line %d)", name, fieldName, tok.Line)
			}

			if err := p.parseNode(entry, fieldName, fieldNode); err != nil {
				return err
			}
		}
	} else {
		// Inline mode: parse "attr value attr value ... ;"
		for {
			tok = p.tok.Peek()
			if tok.Type == TokenSemicolon {
				p.tok.Next()
				break
			}
			if tok.Type == TokenEOF || tok.Type == TokenRBrace {
				return p.errorf(tok, "expected ';' in inline %s", name)
			}
			if tok.Type != TokenWord {
				return p.errorf(tok, "expected attribute name in inline %s, got %s", name, tok.Type)
			}

			attrName := tok.Value
			p.tok.Next()

			// Get value - can be word, string, array [ ... ], parenthesized ( ... ), or flag
			tok = p.tok.Peek()
			var attrValue string
			switch tok.Type { //nolint:exhaustive // Other types handled in default
			case TokenLBracket:
				// Array value: [ item item ... ]
				arrayVals, err := p.collectArray()
				if err != nil {
					return err
				}
				// Join array items with space
				for i, v := range arrayVals {
					if i > 0 {
						attrValue += " "
					}
					attrValue += v
				}
			case TokenLParen:
				// Parenthesized value: ( item item ... )
				parenVals, err := p.collectParenthesized()
				if err != nil {
					return err
				}
				// Join items with space
				for i, v := range parenVals {
					if i > 0 {
						attrValue += " "
					}
					attrValue += v
				}
			case TokenWord, TokenString:
				// Check if this word is a known attribute name - if so, current attr is a flag
				if node.Get(tok.Value) != nil {
					attrValue = configTrue
					// Don't consume - it's the next attribute name
				} else {
					attrValue = tok.Value
					p.tok.Next()
				}
			case TokenSemicolon:
				// Flag without value - the attribute itself is the value (like "withdraw;")
				attrValue = configTrue
			default:
				return p.errorf(tok, "expected value for %s.%s, got %s", name, attrName, tok.Type)
			}

			// Validate if we know this attribute (skip for arrays since values are joined)
			if fieldNode := node.Get(attrName); fieldNode != nil {
				if leaf, ok := fieldNode.(*LeafNode); ok {
					// Only validate non-array simple values
					if tok.Type != TokenLBracket {
						if err := ValidateValue(leaf.Type, attrValue); err != nil {
							return p.errorf(tok, "invalid value for %s.%s: %v", name, attrName, err)
						}
					}
				}
			}

			entry.Set(attrName, attrValue)
		}
	}

	tree.AddListEntry(name, key, entry)
	return nil
}

// skipBlock skips a nested block { ... }, including nested blocks.
func (p *Parser) skipBlock() error {
	tok := p.tok.Peek()
	if tok.Type != TokenLBrace {
		return p.errorf(tok, "expected '{', got %s", tok.Type)
	}
	p.tok.Next()

	depth := 1
	for depth > 0 {
		tok = p.tok.Next()
		switch tok.Type { //nolint:exhaustive // Only tracking braces and EOF
		case TokenLBrace:
			depth++
		case TokenRBrace:
			depth--
		case TokenEOF:
			return p.errorf(tok, "unexpected EOF in nested block")
		}
	}
	return nil
}

// collectArray collects array values [ item item ... ] and returns them.
// Handles nested brackets by including them as literal text.
func (p *Parser) collectArray() ([]string, error) {
	tok := p.tok.Peek()
	if tok.Type != TokenLBracket {
		return nil, p.errorf(tok, "expected '[', got %s", tok.Type)
	}
	p.tok.Next() // consume [

	var items []string
	depth := 1
	var nested string

	for depth > 0 {
		tok = p.tok.Peek()
		switch tok.Type { //nolint:exhaustive // Only specific tokens handled, others pass through
		case TokenRBracket:
			depth--
			if depth > 0 {
				nested += "]"
			}
			p.tok.Next()
		case TokenLBracket:
			depth++
			nested += "["
			p.tok.Next()
		case TokenWord, TokenString:
			if depth > 1 {
				if nested != "" && nested[len(nested)-1] != '[' {
					nested += " "
				}
				nested += tok.Value
			} else {
				if nested != "" {
					items = append(items, nested)
					nested = ""
				}
				items = append(items, tok.Value)
			}
			p.tok.Next()
		case TokenEOF:
			return nil, p.errorf(tok, "unexpected EOF in array")
		default:
			// Include other tokens (parens, commas) in nested content
			if depth > 1 {
				nested += tok.Value
			}
			p.tok.Next()
		}
	}

	if nested != "" {
		items = append(items, nested)
	}

	return items, nil
}

// collectParenthesized collects parenthesized values ( item item ... ) and returns them.
// Handles nested content including brackets.
func (p *Parser) collectParenthesized() ([]string, error) {
	tok := p.tok.Peek()
	if tok.Type != TokenLParen {
		return nil, p.errorf(tok, "expected '(', got %s", tok.Type)
	}
	p.tok.Next() // consume (

	var items []string
	depth := 1
	var current string

	for depth > 0 {
		tok = p.tok.Peek()
		switch tok.Type { //nolint:exhaustive // Only specific tokens handled
		case TokenRParen:
			depth--
			if depth > 0 {
				current += ")"
			}
			p.tok.Next()
		case TokenLParen:
			depth++
			current += "("
			p.tok.Next()
		case TokenLBracket:
			current += "["
			p.tok.Next()
		case TokenRBracket:
			current += "]"
			p.tok.Next()
		case TokenWord, TokenString:
			if current != "" && current[len(current)-1] != '(' && current[len(current)-1] != '[' {
				current += " "
			}
			current += tok.Value
			p.tok.Next()
		case TokenEOF:
			return nil, p.errorf(tok, "unexpected EOF in parenthesized expression")
		default:
			current += tok.Value
			p.tok.Next()
		}
	}

	if current != "" {
		items = append(items, current)
	}

	return items, nil
}

// joinStrings joins strings with a separator.
func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

// errorf creates a formatted error with line info.
func (p *Parser) errorf(tok Token, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("line %d: %s", tok.Line, msg)
}
