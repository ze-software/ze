// Design: docs/architecture/config/syntax.md — list and multi-leaf parsing
// Related: parser.go — config parser core

package config

import (
	"strings"
)

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
