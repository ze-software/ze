// Design: docs/architecture/config/syntax.md — list and multi-leaf parsing
// Overview: parser.go — config parser core
// Related: parser_freeform.go — freeform, flex, and inline parsing

package config

import (
	"slices"
	"strings"
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
			tree.AddListEntry(name, KeyDefault, NewTree())
			return nil
		}

		// If the first word is a known child field → anonymous entry
		if (inner.Type == TokenWord || inner.Type == TokenString) && node.Get(inner.Value) != nil {
			return p.parseListFieldBlock(tree, name, node, KeyDefault)
		}

		// Otherwise → block of inline entries (each line = key [positional values...] ;)
		return p.parseListInlineBlock(tree, name, node)
	}

	// Word — this is the key
	if tok.Type == TokenWord || tok.Type == TokenString {
		key := tok.Value
		p.tok.Next()

		if err := ValidateValue(node.KeyType, key); err != nil {
			return p.errorf(tok, "invalid key for %s: %v", name, err)
		}

		tok = p.tok.Peek()

		// key { ... } — named block entry with child fields
		if tok.Type == TokenLBrace {
			p.tok.Next() // consume {
			return p.parseListFieldBlock(tree, name, node, key)
		}

		// key [values...] ; — single inline entry with positional values
		return p.parseListInlineEntry(tree, name, node, key)
	}

	return p.errorf(tok, "expected key or '{' for %s, got %s", name, tok.Type)
}

// parseListFieldBlock parses the inside of { ... } as named fields for a single list entry.
// The opening { has already been consumed.
func (p *Parser) parseListFieldBlock(tree *Tree, name string, node *ListNode, key string) error {
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

		if err := ValidateValue(node.KeyType, key); err != nil {
			return p.errorf(tok, "invalid key for %s: %v", name, err)
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
			part := "[ " + strings.Join(arrayVals, " ") + " ]"
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
		entry.Set(children[lastIdx], strings.Join(lastParts, " "))
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

	var value strings.Builder
	for i, w := range words {
		if i > 0 {
			value.WriteString(" ")
		}
		value.WriteString(w)
	}

	tree.Set(name, value.String())
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
	var value strings.Builder
	for i, item := range items {
		if i > 0 {
			value.WriteString(" ")
		}
		value.WriteString(item)
	}

	tree.Set(name, value.String())
	return nil
}

// parseValueOrArray parses either "value;" or "[ item item ... ];".
// Stores result as a slice via SetSlice for GetSlice() access.
// Also stores as space-separated string via Set for Get() access.
func (p *Parser) parseValueOrArray(tree *Tree, name string, node *ValueOrArrayNode) error {
	tok := p.tok.Peek()

	var items []string

	// Check if it's an array (starts with [)
	if tok.Type == TokenLBracket {
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
				return p.errorf(tok, "expected item or ']' in %s, got %s", name, tok.Type)
			}
		}

		// Expect semicolon
		tok = p.tok.Peek()
		if tok.Type != TokenSemicolon {
			return p.errorf(tok, "expected ';' after %s, got %s", name, tok.Type)
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
				return p.errorf(tok, "expected value or ';' in %s, got %s", name, tok.Type)
			}
		}
	}

	// Validate enum values if the schema defines valid values
	if node.ValidValues != nil {
		for _, item := range items {
			if !containsString(node.ValidValues, item) {
				return p.errorf(tok, "invalid value for %s: %q (valid: %s)", name, item, strings.Join(node.ValidValues, ", "))
			}
		}
	}

	// Store as slice for GetSlice() and as string for Get()
	tree.SetSlice(name, items)
	var value strings.Builder
	for i, item := range items {
		if i > 0 {
			value.WriteString(" ")
		}
		value.WriteString(item)
	}
	tree.Set(name, value.String())
	return nil
}

// containsString checks if a string slice contains a value.
func containsString(slice []string, val string) bool {
	return slices.Contains(slice, val)
}
