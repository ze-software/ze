// Design: docs/architecture/config/syntax.md — freeform, family, flex, and inline parsing
// Overview: parser.go — config parser core
// Related: parser_list.go — list and multi-leaf parsing

package config

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// parseFreeform parses a freeform block: `name { word word; word word; }`
// Also handles: `name subname { ... }` (skips subname)
// Stores each "word word" line as key -> "true".
func (p *Parser) parseFreeform(tree *Tree, name string) error {
	tok := p.tok.peek()

	// Skip optional words before the block (e.g., "api services { }")
	for tok.kind == tokenWord || tok.kind == tokenString {
		p.tok.next()
		tok = p.tok.peek()
	}

	if tok.kind != tokenLBrace {
		return p.errorf(tok, "expected '{' after %s, got %s", name, tok.kind)
	}
	p.tok.next()

	child := NewTree()

	for {
		tok = p.tok.peek()
		if tok.kind == tokenRBrace {
			p.tok.next()
			break
		}
		if tok.kind == tokenEOF {
			return p.errorf(tok, "unexpected EOF in %s block", name)
		}

		// Collect words until semicolon or nested block
		var words []string
		hadArray := false
		startLine := p.tok.peek().line
		for {
			tok = p.tok.peek()
			if tok.kind == tokenSemicolon {
				p.tok.next()
				break
			}
			if tok.kind == tokenLBrace {
				// Warn about nested block being skipped
				p.warn(startLine, "freeform '%s' contains nested block '%s' - data may be lost", name, textbuf.Join(words, " "))
				// Skip nested block
				if err := p.skipBlock(); err != nil {
					return err
				}
				break
			}
			if tok.kind == tokenLBracket {
				// Capture array [ ... ] values, preserving brackets
				arrayVals, err := p.collectArray()
				if err != nil {
					return err
				}
				// Preserve bracket syntax for freeform: "[ val1 val2 ]"
				var tb textbuf.Buffer
				bracketedVal := tb.Str("[ ").Join(arrayVals, " ").Str(" ]").String()
				words = append(words, bracketedVal)
				hadArray = true
				continue
			}
			if tok.kind == tokenRBrace || tok.kind == tokenEOF {
				break
			}
			if tok.kind == tokenWord || tok.kind == tokenString {
				words = append(words, tok.value)
				p.tok.next()
			} else {
				return p.errorf(tok, "unexpected token in %s block: %s", name, tok.kind)
			}
		}

		if len(words) > 0 {
			if hadArray && len(words) > 1 {
				// Array present: "processes [ watcher ];" -> key="processes", value="watcher"
				child.Set(words[0], textbuf.Join(words[1:], " "))
			} else {
				// No array: "ipv4/unicast;" -> key="ipv4/unicast", value="true"
				child.Set(textbuf.Join(words, " "), configTrue)
			}
		}
	}

	tree.MergeContainer(name, child)
	return nil
}

// parseFlex parses a flex node: flag (;), value (word;), or block ({}).
func (p *Parser) parseFlex(tree *Tree, name string, node *FlexNode) error {
	tok := p.tok.peek()

	switch tok.kind { //nolint:exhaustive // Only specific tokens valid here, others handled in final return
	case tokenSemicolon:
		// Flag mode: just the name with semicolon = true
		p.tok.next()
		tree.Set(name, configTrue)
		return nil

	case tokenLBrace:
		// Block mode: parse as container
		p.tok.next()
		child := NewTree()

		for {
			tok = p.tok.peek()
			if tok.kind == tokenRBrace {
				p.tok.next()
				break
			}
			if tok.kind == tokenEOF {
				return p.errorf(tok, "unexpected EOF in %s block", name)
			}
			if tok.kind != tokenWord {
				return p.errorf(tok, "expected keyword in %s block, got %s", name, tok.kind)
			}

			fieldName := tok.value
			p.tok.next()

			fieldNode := node.Get(fieldName)
			if fieldNode == nil {
				return p.errorf(tok, "unknown field in %s: %s (line %d)", name, fieldName, tok.line)
			}

			if err := p.parseNode(child, fieldName, fieldNode); err != nil {
				return err
			}
		}

		tree.MergeContainer(name, child)
		return nil

	case tokenLParen:
		// Parenthesized mode: parse ( ... ) and optional semicolon
		parenVals, err := p.collectParenthesized()
		if err != nil {
			return err
		}
		// Optional semicolon after parenthesized content
		tok = p.tok.peek()
		if tok.kind == tokenSemicolon {
			p.tok.next()
		}

		tree.Set(name, textbuf.Join(parenVals, " "))
		return nil

	case tokenLBracket:
		// Array mode: parse [ ... ] directly (e.g., "attribute [ 0x20 0xc0 ... ];")
		arrayVals, err := p.collectArray()
		if err != nil {
			return err
		}
		var tb textbuf.Buffer
		value := tb.Byte('[').Join(arrayVals, " ").Byte(']').String()

		// Expect semicolon
		tok = p.tok.peek()
		if tok.kind != tokenSemicolon {
			return p.errorf(tok, "expected ';' after %s array, got %s", name, tok.kind)
		}
		p.tok.next()

		tree.Set(name, value)
		return nil

	case tokenWord, tokenString:
		return p.parseFlexValue(tree, name, node, tok)
	}

	return p.errorf(tok, "expected ';', value, or '{' for %s, got %s", name, tok.kind)
}

// parseFlexValue handles the word/string case for parseFlex.
func (p *Parser) parseFlexValue(tree *Tree, name string, node *FlexNode, tok token) error {
	// Value mode: parse multiple words until semicolon or block delimiter
	var values []string
	for tok.kind == tokenWord || tok.kind == tokenString || tok.kind == tokenLBracket || tok.kind == tokenLParen {
		switch tok.kind { //nolint:exhaustive // Only handling specific types in loop condition
		case tokenLBracket:
			// Array: collect [ ... ]
			arrayVals, err := p.collectArray()
			if err != nil {
				return err
			}
			var tb textbuf.Buffer
			values = append(values, tb.Byte('[').Join(arrayVals, " ").Byte(']').String())
		case tokenLParen:
			// Parenthesized: collect ( ... )
			parenVals, err := p.collectParenthesized()
			if err != nil {
				return err
			}
			var tb2 textbuf.Buffer
			values = append(values, tb2.Byte('(').Join(parenVals, " ").Byte(')').String())
		default:
			values = append(values, tok.value)
			p.tok.next()
		}
		tok = p.tok.peek()
	}

	// Check if this is a named block (e.g., "vpls site5 { ... }")
	if tok.kind == tokenLBrace && len(values) == 1 {
		// Named block: the first value is the key
		key := values[0]
		p.tok.next() // consume {

		child := NewTree()
		for {
			tok = p.tok.peek()
			if tok.kind == tokenRBrace {
				p.tok.next()
				break
			}
			if tok.kind == tokenEOF {
				return p.errorf(tok, "unexpected EOF in %s block", name)
			}
			if tok.kind != tokenWord {
				return p.errorf(tok, "expected keyword in %s block, got %s", name, tok.kind)
			}

			fieldName := tok.value
			p.tok.next()

			fieldNode := node.Get(fieldName)
			if fieldNode == nil {
				// Unknown field - store as value
				p.warnings = append(p.warnings, "unknown field in "+AppendPath(name, key)+": "+fieldName)
				// Consume until semicolon
				for p.tok.peek().kind != tokenSemicolon && p.tok.peek().kind != tokenEOF {
					p.tok.next()
				}
				if p.tok.peek().kind == tokenSemicolon {
					p.tok.next()
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

	if tok.kind != tokenSemicolon {
		return p.errorf(tok, "expected ';' after %s value, got %s", name, tok.kind)
	}
	p.tok.next()

	// Use AppendValue to support multiple inline entries (e.g., multiple mup routes)
	tree.AppendValue(name, textbuf.Join(values, " "))
	return nil
}

// parseInlineList parses a list with inline or block syntax.
// Inline: "route 10.0.0.0/8 next-hop 1.1.1.1;"
// Block: "route 10.0.0.0/8 { next-hop 1.1.1.1; }".
func (p *Parser) parseInlineList(tree *Tree, name string, node *InlineListNode) error {
	// Get key
	tok := p.tok.peek()
	var key string
	keyLine := tok.line
	if tok.kind == tokenWord || tok.kind == tokenString {
		key = tok.value
		p.tok.next()
	} else {
		return p.errorf(tok, "expected key for %s, got %s", name, tok.kind)
	}

	// Validate key type
	if err := ValidateValue(node.KeyType, key); err != nil {
		return p.errorf(tok, "invalid key for %s: %v", name, err)
	}

	entry := NewTree()

	// Check for block or inline
	tok = p.tok.peek()
	if tok.kind == tokenLBrace {
		// Block mode
		p.tok.next()

		for {
			tok = p.tok.peek()
			if tok.kind == tokenRBrace {
				p.tok.next()
				break
			}
			if tok.kind == tokenEOF {
				return p.errorf(tok, "unexpected EOF in %s block", name)
			}
			if tok.kind != tokenWord {
				return p.errorf(tok, "expected keyword in %s block, got %s", name, tok.kind)
			}

			fieldName := tok.value
			p.tok.next()

			fieldNode := node.Get(fieldName)
			if fieldNode == nil {
				return p.errorf(tok, "unknown field in %s: %s (line %d)", name, fieldName, tok.line)
			}

			if err := p.parseNode(entry, fieldName, fieldNode); err != nil {
				return err
			}
		}
	} else {
		// Inline mode: parse "attr value attr value ... ;"
		for {
			tok = p.tok.peek()
			if tok.kind == tokenSemicolon {
				p.tok.next()
				break
			}
			if tok.kind == tokenEOF || tok.kind == tokenRBrace {
				return p.errorf(tok, "expected ';' in inline %s", name)
			}
			if tok.kind != tokenWord {
				return p.errorf(tok, "expected attribute name in inline %s, got %s", name, tok.kind)
			}

			attrName := tok.value
			p.tok.next()

			// Get value - can be word, string, array [ ... ], parenthesized ( ... ), or flag
			tok = p.tok.peek()
			var attrValue string
			switch tok.kind { //nolint:exhaustive // Other types handled in final error return
			case tokenLBracket:
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
			case tokenLParen:
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
			case tokenWord, tokenString:
				// Check if this word is a known attribute name - if so, current attr is a flag
				if node.Get(tok.value) != nil {
					attrValue = configTrue
					// Don't consume - it's the next attribute name
				} else {
					attrValue = tok.value
					p.tok.next()
				}
			case tokenSemicolon:
				// Flag without value - the attribute itself is the value (like "withdraw;")
				attrValue = configTrue
			default:
				return p.errorf(tok, "expected value for %s.%s, got %s", name, attrName, tok.kind)
			}

			// Validate if we know this attribute (skip for arrays since values are joined)
			if fieldNode := node.Get(attrName); fieldNode != nil {
				if leaf, ok := fieldNode.(*LeafNode); ok {
					// Only validate non-array simple values
					if tok.kind != tokenLBracket {
						if err := ValidateLeafValue(leaf, attrValue); err != nil {
							return p.errorf(tok, "invalid value for %s.%s: %v", name, attrName, err)
						}
					}
				}
			}

			entry.Set(attrName, attrValue)
		}
	}

	return p.addParsedInlineListEntry(tree, name, node, key, entry, keyLine)
}

func (p *Parser) addParsedInlineListEntry(
	tree *Tree,
	name string,
	node *InlineListNode,
	key string,
	entry *Tree,
	line int,
) error {
	if entries := tree.GetList(name); entries != nil {
		incomingInactive := entry.IsInactive()
		for existingKey, existing := range entries {
			if StripListKeySuffix(existingKey) != key {
				continue
			}
			if incomingInactive || existing.IsInactive() || allowsDuplicateInlineListEntry(node, existing, entry) {
				continue
			}
			return p.errorf(token{line: line}, "duplicate list key for %s: %s", name, key)
		}
	}
	tree.AddListEntry(name, key, entry)
	return nil
}

func allowsDuplicateInlineListEntry(node *InlineListNode, existing, incoming *Tree) bool {
	if node == nil || node.Get("path-information") == nil {
		return false
	}
	incomingPathInfo, incomingOK := incoming.Get("path-information")
	existingPathInfo, existingOK := existing.Get("path-information")
	return incomingOK && existingOK && incomingPathInfo != existingPathInfo
}

// skipBlock skips a nested block { ... }, including nested blocks.
func (p *Parser) skipBlock() error {
	tok := p.tok.peek()
	if tok.kind != tokenLBrace {
		return p.errorf(tok, "expected '{', got %s", tok.kind)
	}
	p.tok.next()

	depth := 1
	for depth > 0 {
		tok = p.tok.next()
		switch tok.kind { //nolint:exhaustive // Only tracking braces and EOF
		case tokenLBrace:
			depth++
		case tokenRBrace:
			depth--
		case tokenEOF:
			return p.errorf(tok, "unexpected EOF in nested block")
		}
	}
	return nil
}

// collectArray collects array values [ item item ... ] and returns them.
// Handles nested brackets by including them as literal text.
func (p *Parser) collectArray() ([]string, error) {
	tok := p.tok.peek()
	if tok.kind != tokenLBracket {
		return nil, p.errorf(tok, "expected '[', got %s", tok.kind)
	}
	p.tok.next() // consume [

	var items []string
	depth := 1
	var nested string

	for depth > 0 {
		tok = p.tok.peek()
		switch tok.kind { //nolint:exhaustive // Only specific tokens handled, others pass through
		case tokenRBracket:
			depth--
			if depth > 0 {
				nested += "]"
			}
			p.tok.next()
		case tokenLBracket:
			depth++
			nested += "["
			p.tok.next()
		case tokenWord, tokenString:
			if depth > 1 {
				if nested != "" && nested[len(nested)-1] != '[' {
					nested += " "
				}
				nested += tok.value
			} else {
				if nested != "" {
					items = append(items, nested)
					nested = ""
				}
				items = append(items, tok.value)
			}
			p.tok.next()
		case tokenEOF:
			return nil, p.errorf(tok, "unexpected EOF in array")
		default:
			// Include other tokens (parens, commas) in nested content
			if depth > 1 {
				nested += tok.value
			}
			p.tok.next()
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
	tok := p.tok.peek()
	if tok.kind != tokenLParen {
		return nil, p.errorf(tok, "expected '(', got %s", tok.kind)
	}
	p.tok.next() // consume (

	var items []string
	depth := 1
	var current string

	for depth > 0 {
		tok = p.tok.peek()
		switch tok.kind { //nolint:exhaustive // Only specific tokens handled
		case tokenRParen:
			depth--
			if depth > 0 {
				current += ")"
			}
			p.tok.next()
		case tokenLParen:
			depth++
			current += "("
			p.tok.next()
		case tokenLBracket:
			current += "["
			p.tok.next()
		case tokenRBracket:
			current += "]"
			p.tok.next()
		case tokenWord, tokenString:
			if current != "" && current[len(current)-1] != '(' && current[len(current)-1] != '[' {
				current += " "
			}
			current += tok.value
			p.tok.next()
		case tokenEOF:
			return nil, p.errorf(tok, "unexpected EOF in parenthesized expression")
		default:
			current += tok.value
			p.tok.next()
		}
	}

	if current != "" {
		items = append(items, current)
	}

	return items, nil
}
