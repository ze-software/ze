// Design: docs/architecture/config/syntax.md — list and multi-leaf parsing
// Overview: parser.go — config parser core
// Related: parser_freeform.go — freeform, flex, and inline parsing

package config

import (
	"fmt"
	"slices"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// parseList parses a YANG list in three forms:
//   - Block of inline entries:  name { key1; key2 val; }
//   - Single inline entry:     name key val;
//   - Named block entry:       name key { field val; }
//   - Anonymous block entry:   name { field val; }  (key defaults to KeyDefault)
//
// Disambiguation when seeing `name {`: if the first word inside is a known
// child field → anonymous entry; otherwise → block of inline entries.
func (p *Parser) parseList(tree *Tree, name string, node *ListNode) error {
	tok := p.tok.Peek()

	// Direct `{` — either anonymous entry or block of inline entries
	if tok.Type == TokenLBrace {
		p.tok.Next() // consume {

		// Peek at first word to decide
		inner := p.tok.Peek()
		if inner.Type == TokenRBrace {
			// Empty block — anonymous entry
			p.tok.Next()
			return p.addParsedListEntry(tree, name, node, KeyDefault, NewTree(), tok.Line)
		}

		// If the first word is a known child field → anonymous entry
		if (inner.Type == TokenWord || inner.Type == TokenString) && node.Get(inner.Value) != nil {
			return p.parseListFieldBlock(tree, name, node, KeyDefault, inner.Line)
		}

		// Otherwise → block of inline entries (each line = key [positional values...] ;)
		return p.parseListInlineBlock(tree, name, node)
	}

	// Word — this is the key
	if tok.Type == TokenWord || tok.Type == TokenString {
		key := tok.Value
		p.tok.Next()

		if err := ValidateListKey(node, key); err != nil {
			return p.invalidListKeyError(tok, name, err)
		}

		tok = p.tok.Peek()

		// key { ... } — named block entry with child fields
		if tok.Type == TokenLBrace {
			p.tok.Next() // consume {
			return p.parseListFieldBlock(tree, name, node, key, tok.Line)
		}

		// key [values...] ; — single inline entry with positional values
		return p.parseListInlineEntry(tree, name, node, key)
	}

	return p.errorf(tok, "expected key or '{' for %s, got %s", name, tok.Type)
}

// parseListFieldBlock parses the inside of { ... } as named fields for a single list entry.
// The opening { has already been consumed.
func (p *Parser) parseListFieldBlock(tree *Tree, name string, node *ListNode, key string, keyLine int) error {
	entry := NewTree()

	for {
		tok := p.tok.Peek()
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

		// Handle "inactive: <field> { ... }" sugar (same as in parseContainer).
		markInactive := false
		if fieldName == InactiveLeafName+":" {
			markInactive = true
			tok = p.tok.Peek()
			if tok.Type != TokenWord {
				return p.errorf(tok, "expected field name after inactive:, got %s", tok.Type)
			}
			fieldName = tok.Value
			p.tok.Next()
		}

		fieldNode := node.Get(fieldName)
		if fieldNode == nil {
			return p.errorf(tok, "unknown field in %s: %s (line %d)", name, fieldName, tok.Line)
		}

		if markInactive {
			if err := p.parseNodeInactive(entry, fieldName, fieldNode, tok.Line); err != nil {
				return err
			}
			continue
		}

		if err := p.parseNode(entry, fieldName, fieldNode); err != nil {
			return err
		}
	}

	return p.addParsedListEntry(tree, name, node, key, entry, keyLine)
}

// parseListInlineBlock parses a { ... } block containing multiple inline entries.
// Each line: key [positional_values...] ;
// The opening { has already been consumed.
func (p *Parser) parseListInlineBlock(tree *Tree, name string, node *ListNode) error {
	for {
		tok := p.tok.Peek()
		if tok.Type == TokenRBrace {
			p.tok.Next()
			return nil
		}
		if tok.Type == TokenEOF {
			return p.errorf(tok, "unexpected EOF in %s block", name)
		}
		if tok.Type != TokenWord && tok.Type != TokenString {
			return p.errorf(tok, "expected entry key in %s, got %s", name, tok.Type)
		}

		key := tok.Value
		p.tok.Next()

		if err := ValidateListKey(node, key); err != nil {
			return p.invalidListKeyError(tok, name, err)
		}

		// Disambiguate: key { ... } = named block entry, key [values] ; = inline entry.
		if p.tok.Peek().Type == TokenLBrace {
			p.tok.Next() // consume {
			if err := p.parseListFieldBlock(tree, name, node, key, tok.Line); err != nil {
				return err
			}
			continue
		}

		if err := p.parseListInlineEntry(tree, name, node, key); err != nil {
			return err
		}
	}
}

// parseListInlineEntry parses a single inline list entry after the key is consumed.
// Assigns values positionally to children in YANG definition order until ;.
// The last child absorbs all remaining tokens (space-joined), supporting variable-length
// content like NLRI entries: "ipv4/unicast add 10.0.0.0/24;" → content="add 10.0.0.0/24".
// Bracket content ([ ... ]) is collected and included in the joined string.
func (p *Parser) parseListInlineEntry(tree *Tree, name string, node *ListNode, key string) error {
	entry := NewTree()
	children := node.Children()

	// Threshold: tokens beyond this index are collected into the last child.
	// For N children, first N-1 get one token each; last child gets all remaining.
	lastIdx := len(children) - 1
	childIdx := 0
	var lastParts []string

	for {
		tok := p.tok.Peek()
		if tok.Type == TokenSemicolon {
			p.tok.Next()
			break
		}

		// Handle bracket content: [ val1 val2 ... ]
		if tok.Type == TokenLBracket {
			arrayVals, err := p.collectArray()
			if err != nil {
				return err
			}
			var tb textbuf.Buffer
			part := tb.Str("[ ").Join(arrayVals, " ").Str(" ]").String()
			if childIdx <= lastIdx && lastIdx >= 0 {
				lastParts = append(lastParts, part)
				if childIdx < lastIdx {
					childIdx = lastIdx // Jump to last child collection
				}
			}
			continue
		}

		if tok.Type == TokenWord || tok.Type == TokenString {
			if childIdx < lastIdx {
				// Positional assignment for children before the last
				if err := validateInlineListChildValue(node, children[childIdx], tok.Value); err != nil {
					return p.errorf(tok, "invalid value for %s.%s: %v", name, children[childIdx], err)
				}
				entry.Set(children[childIdx], tok.Value)
				childIdx++
			} else if lastIdx >= 0 {
				// Collect into last child
				lastParts = append(lastParts, tok.Value)
			}
			p.tok.Next()
		} else {
			return p.errorf(tok, "expected value or ';' in %s entry, got %s", name, tok.Type)
		}
	}

	// Store collected values in the last child
	if lastIdx >= 0 && len(lastParts) > 0 {
		value := textbuf.Join(lastParts, " ")
		if err := validateInlineListChildValue(node, children[lastIdx], value); err != nil {
			return p.errorf(Token{Line: p.tok.line}, "invalid value for %s.%s: %v", name, children[lastIdx], err)
		}
		entry.Set(children[lastIdx], value)
	}

	return p.addParsedListEntry(tree, name, node, key, entry, p.tok.line)
}

func (p *Parser) addParsedListEntry(tree *Tree, name string, node *ListNode, key string, entry *Tree, line int) error {
	if entries := tree.GetList(name); entries != nil {
		incomingInactive := entry.IsInactive()
		allowDuplicate := allowsDuplicateParsedListEntries(node, key)
		for existingKey, existing := range entries {
			if StripListKeySuffix(existingKey) != key {
				continue
			}
			if allowDuplicate || incomingInactive || existing.IsInactive() {
				continue
			}
			return p.errorf(Token{Line: line}, "duplicate list key for %s: %s", name, key)
		}
	}
	tree.AddListEntry(name, key, entry)
	return nil
}

func allowsDuplicateParsedListEntries(node *ListNode, key string) bool {
	if node == nil {
		return false
	}
	if key == KeyDefault && node.KeyName == "" && node.DisplayKey != "" {
		return true
	}
	children := node.Children()
	if node.KeyName == "" || len(children) != 1 || children[0] != "content" {
		return false
	}
	_, ok := node.Get("content").(*LeafNode)
	return ok
}

func (p *Parser) invalidListKeyError(tok Token, name string, err error) error {
	if name == "peer" {
		return p.errorf(tok, "invalid peer name: %v", err)
	}
	return p.errorf(tok, "invalid key for %s: %v", name, err)
}

// parseMultiLeaf parses multiple words until semicolon: `name word word;`.
func (p *Parser) parseMultiLeaf(tree *Tree, name string, node *MultiLeafNode) error {
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

	joined := textbuf.Join(words, " ")
	if err := validateValuePatterns(node.Type, node.Patterns, joined); err != nil {
		return p.errorf(Token{Line: p.tok.line}, "invalid value for %s: %v", name, err)
	}
	tree.Set(name, joined)
	return nil
}

// parseBracketLeafList parses a bracketed leaf-list: `name [ item item ... ];`.
func (p *Parser) parseBracketLeafList(tree *Tree, name string, node *BracketLeafListNode) error {
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

	for _, item := range items {
		if err := validateValuePatterns(node.Type, node.Patterns, item); err != nil {
			return p.errorf(tok, "invalid value for %s: %v", name, err)
		}
	}

	// Store the members AND the joined scalar mirror, exactly like the sibling
	// leaf-list path (storeValueOrArray). AppendSlice (not SetSlice) so a leaf-list
	// spelled as repeated `name value;` statements accumulates per YANG (RFC 7950
	// sec 7.7) instead of the last statement silently winning; a single bracket
	// statement on an empty leaf-list is unchanged. AppendSlice re-syncs the scalar
	// mirror to the active members, so Get() keeps returning the joined form and
	// stays consistent with GetSlice; without a slice store, GetSlice returns nil
	// and ToMap emits the joined text as one string, so every consumer sees
	// `[ a b ]` as the single value "a b" -- which is why a unit could not carry
	// two addresses. An ordered SEQUENCE (ze:ordered) preserves duplicates.
	if node.Ordered {
		tree.AppendSequence(name, items)
	} else {
		tree.AppendSlice(name, items)
	}
	return nil
}

// parseValueOrArray parses either "value;" or "[ item item ... ];".
// Stores result as a slice via SetSlice for GetSlice() access.
// Also stores as space-separated string via Set for Get() access.
func (p *Parser) parseValueOrArray(tree *Tree, name string, node *ValueOrArrayNode) error {
	items, _, tok, err := p.collectValueOrArrayItems(name)
	if err != nil {
		return err
	}
	return p.storeValueOrArray(tree, name, node, items, tok)
}

// collectValueOrArrayItems reads the value part of a plain leaf-list
// statement: "value;", "value value ...;" or "[ item item ... ];".
// Returns the raw items, whether the bracket form was used, and the
// terminating token (for error positions).
func (p *Parser) collectValueOrArrayItems(name string) (items []string, bracket bool, tok Token, err error) {
	tok = p.tok.Peek()

	// Check if it's an array (starts with [)
	if tok.Type == TokenLBracket {
		bracket = true
		p.tok.Next() // consume [

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
				return nil, bracket, tok, p.errorf(tok, "expected item or ']' in %s, got %s", name, tok.Type)
			}
		}

		// Expect semicolon
		tok = p.tok.Peek()
		if tok.Type != TokenSemicolon {
			return nil, bracket, tok, p.errorf(tok, "expected ';' after %s, got %s", name, tok.Type)
		}
		p.tok.Next()
	} else {
		// Parse as single value or multiple space-separated values
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
				return nil, bracket, tok, p.errorf(tok, "expected value or ';' in %s, got %s", name, tok.Type)
			}
		}
	}
	return items, bracket, tok, nil
}

// storeValueOrArray validates collected leaf-list items against the schema
// and stores them (slice for GetSlice, joined string for Get). An inline
// "inactive:" member prefix (the compact deactivation form operators may write
// directly in a bracket list, e.g. `import [ inactive:no-self-as ... ]`) is
// normalized here into the out-of-band per-member marker: the stored member
// value is clean and validated clean, so no value ever carries the prefix.
func (p *Parser) storeValueOrArray(tree *Tree, name string, node *ValueOrArrayNode, items []string, tok Token) error {
	clean, deactivated := stripInactiveMemberPrefix(items)

	// Validate enum values if the schema defines valid values
	if node.ValidValues != nil {
		for _, item := range clean {
			if !containsString(node.ValidValues, item) {
				return p.errorf(tok, "invalid value for %s: %q (valid: %s)", name, item, textbuf.Join(node.ValidValues, ", "))
			}
		}
	}
	for _, item := range clean {
		if err := validateValuePatterns(node.Type, node.Patterns, item); err != nil {
			return p.errorf(tok, "invalid value for %s: %v", name, err)
		}
	}

	// Store clean members; AppendSlice (not SetSlice) so repeated `name value;`
	// statements accumulate per YANG (RFC 7950 sec 7.7) instead of the last one
	// silently winning; a single statement on an empty leaf-list is unchanged.
	// AppendSlice re-syncs the scalar mirror to the active members (excluding any
	// member deactivated by an earlier statement); the loop below then deactivates
	// this statement's inactive: members, each re-syncing again. An ordered
	// SEQUENCE (ze:ordered: AS_PATH, MPLS labels) preserves duplicates -- collapsing
	// `as-path [ 65001 65001 ]` to one AS drops a load-bearing prepend.
	if node.Ordered {
		tree.AppendSequence(name, clean)
	} else {
		tree.AppendSlice(name, clean)
	}
	for _, member := range deactivated {
		// An ordered sequence refuses deactivation of a REPEATED value (value-keyed
		// deactivation would blank every copy); surface that as a parse error rather
		// than silently drop the prepend. A set tolerates a repeated inactive: prefix
		// on the same value -- the marker is idempotent and the value is unique there.
		if err := tree.DeactivateMultiValue(name, member); err != nil && node.Ordered {
			return p.errorf(tok, "invalid value for %s: %v", name, err)
		}
	}
	return nil
}

// stripInactiveMemberPrefix splits leaf-list items into their clean values (any
// leading "inactive:" removed) and the subset that carried the prefix. It is the
// parse-boundary inverse of the serializer's inactive-member rendering: the
// prefix is an accepted input form but is never stored on a value.
func stripInactiveMemberPrefix(items []string) (clean, deactivated []string) {
	clean = make([]string, len(items))
	for i, item := range items {
		// Require a non-empty remainder: a bare "inactive:" token (e.g. a stray
		// space in `[ inactive: X ]`) is kept literal, not turned into an empty
		// member, so validation rejects it instead of a silent empty ref.
		if rest, ok := strings.CutPrefix(item, "inactive:"); ok && rest != "" {
			clean[i] = rest
			deactivated = append(deactivated, rest)
		} else {
			clean[i] = item
		}
	}
	return clean, deactivated
}

func validateInlineListChildValue(list *ListNode, childName, value string) error {
	child := list.Get(childName)
	switch n := child.(type) {
	case *LeafNode:
		return ValidateLeafValue(n, value)
	case *MultiLeafNode:
		return validateValuePatterns(n.Type, n.Patterns, value)
	case *BracketLeafListNode:
		for _, item := range inlineValueItems(value) {
			if err := validateValuePatterns(n.Type, n.Patterns, item); err != nil {
				return err
			}
		}
	case *ValueOrArrayNode:
		for _, item := range inlineValueItems(value) {
			if n.ValidValues != nil && !containsString(n.ValidValues, item) {
				return fmt.Errorf("invalid enum: %q (expected one of: %s)", item, textbuf.Join(n.ValidValues, ", "))
			}
			if err := validateValuePatterns(n.Type, n.Patterns, item); err != nil {
				return err
			}
		}
	}
	return nil
}

func inlineValueItems(value string) []string {
	items := strings.Fields(value)
	if len(items) >= 2 && items[0] == "[" && items[len(items)-1] == "]" {
		return items[1 : len(items)-1]
	}
	return items
}

// containsString checks if a string slice contains a value.
func containsString(slice []string, val string) bool {
	return slices.Contains(slice, val)
}
