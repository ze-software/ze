// Design: docs/architecture/config/syntax.md — freeform, family, flex, and inline parsing
// Related: parser.go — config parser core

package config

import (
	"fmt"
	"strings"
)

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
				var key strings.Builder
				for i, w := range words {
					if i > 0 {
						key.WriteString(" ")
					}
					key.WriteString(w)
				}
				p.warn(startLine, "freeform '%s' contains nested block '%s' - data may be lost", name, key.String())
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
				var value strings.Builder
				for i, w := range words[1:] {
					if i > 0 {
						value.WriteString(" ")
					}
					value.WriteString(w)
				}
				child.Set(key, value.String())
			} else {
				// No array: "ipv4/unicast;" -> key="ipv4/unicast", value="true"
				var key strings.Builder
				for i, w := range words {
					if i > 0 {
						key.WriteString(" ")
					}
					key.WriteString(w)
				}
				child.Set(key.String(), configTrue)
			}
		}
	}

	tree.MergeContainer(name, child)
	return nil
}

// parseFlex parses a flex node: flag (;), value (word;), or block ({}).
func (p *Parser) parseFlex(tree *Tree, name string, node *FlexNode) error {
	tok := p.tok.Peek()

	switch tok.Type { //nolint:exhaustive // Only specific tokens valid here, others handled in final return
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
		var value strings.Builder
		for i, v := range parenVals {
			if i > 0 {
				value.WriteString(" ")
			}
			value.WriteString(v)
		}

		// Optional semicolon after parenthesized content
		tok = p.tok.Peek()
		if tok.Type == TokenSemicolon {
			p.tok.Next()
		}

		tree.Set(name, value.String())
		return nil

	case TokenLBracket:
		// Array mode: parse [ ... ] directly (e.g., "attribute [ 0x20 0xc0 ... ];")
		arrayVals, err := p.collectArray()
		if err != nil {
			return err
		}
		value := "[" + strings.Join(arrayVals, " ") + "]"

		// Expect semicolon
		tok = p.tok.Peek()
		if tok.Type != TokenSemicolon {
			return p.errorf(tok, "expected ';' after %s array, got %s", name, tok.Type)
		}
		p.tok.Next()

		tree.Set(name, value)
		return nil

	case TokenWord, TokenString:
		return p.parseFlexValue(tree, name, node, tok)
	}

	return p.errorf(tok, "expected ';', value, or '{' for %s, got %s", name, tok.Type)
}

// parseFlexValue handles the word/string case for parseFlex.
func (p *Parser) parseFlexValue(tree *Tree, name string, node *FlexNode, tok Token) error {
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
			values = append(values, "["+strings.Join(arrayVals, " ")+"]")
		case TokenLParen:
			// Parenthesized: collect ( ... )
			parenVals, err := p.collectParenthesized()
			if err != nil {
				return err
			}
			values = append(values, "("+strings.Join(parenVals, " ")+")")
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
	tree.AppendValue(name, strings.Join(values, " "))
	return nil
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
			switch tok.Type { //nolint:exhaustive // Other types handled in final error return
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
