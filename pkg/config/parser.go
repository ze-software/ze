package config

import (
	"fmt"
)

// Tree represents parsed configuration data.
type Tree struct {
	values     map[string]string
	containers map[string]*Tree
	lists      map[string]map[string]*Tree
}

// NewTree creates an empty config tree.
func NewTree() *Tree {
	return &Tree{
		values:     make(map[string]string),
		containers: make(map[string]*Tree),
		lists:      make(map[string]map[string]*Tree),
	}
}

// Get returns a leaf value.
func (t *Tree) Get(name string) (string, bool) {
	v, ok := t.values[name]
	return v, ok
}

// Set sets a leaf value.
func (t *Tree) Set(name, value string) {
	t.values[name] = value
}

// GetContainer returns a nested container.
func (t *Tree) GetContainer(name string) *Tree {
	return t.containers[name]
}

// SetContainer sets a nested container.
func (t *Tree) SetContainer(name string, child *Tree) {
	t.containers[name] = child
}

// GetList returns a list (keyed map of trees).
func (t *Tree) GetList(name string) map[string]*Tree {
	return t.lists[name]
}

// AddListEntry adds an entry to a list.
func (t *Tree) AddListEntry(name, key string, entry *Tree) {
	if t.lists[name] == nil {
		t.lists[name] = make(map[string]*Tree)
	}
	t.lists[name][key] = entry
}

// Parser parses ExaBGP-style configuration.
type Parser struct {
	schema *Schema
	tok    *Tokenizer
}

// NewParser creates a new parser with the given schema.
func NewParser(schema *Schema) *Parser {
	return &Parser{schema: schema}
}

// Parse parses the input string into a config tree.
func (p *Parser) Parse(input string) (*Tree, error) {
	p.tok = NewTokenizer(input)
	return p.parseRoot()
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
	case *ArrayLeafNode:
		return p.parseArrayLeaf(tree, name, n)
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
	default:
		return fmt.Errorf("unknown node type for %s", name)
	}
}

// parseLeaf parses a leaf value: `name value;`
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

// parseContainer parses a container block: `name { ... }`
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

	tree.SetContainer(name, child)
	return nil
}

// parseList parses a list entry: `name key { ... }`
func (p *Parser) parseList(tree *Tree, name string, node *ListNode) error {
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

	// Expect opening brace
	tok = p.tok.Peek()
	if tok.Type != TokenLBrace {
		return p.errorf(tok, "expected '{' after %s key, got %s", name, tok.Type)
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

// parseMultiLeaf parses multiple words until semicolon: `name word word;`
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

// parseArrayLeaf parses an array: `name [ item item ... ];`
func (p *Parser) parseArrayLeaf(tree *Tree, name string, _ *ArrayLeafNode) error {
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
		for {
			tok = p.tok.Peek()
			if tok.Type == TokenSemicolon {
				p.tok.Next()
				break
			}
			if tok.Type == TokenLBrace {
				// Skip nested block
				if err := p.skipBlock(); err != nil {
					return err
				}
				break
			}
			if tok.Type == TokenLBracket {
				// Capture array [ ... ] values
				arrayVals, err := p.collectArray()
				if err != nil {
					return err
				}
				words = append(words, arrayVals...)
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
				// No array: "ipv4 unicast;" -> key="ipv4 unicast", value="true"
				key := ""
				for i, w := range words {
					if i > 0 {
						key += " "
					}
					key += w
				}
				child.Set(key, "true")
			}
		}
	}

	tree.SetContainer(name, child)
	return nil
}

// parseFlex parses a flex node: flag (;), value (word;), or block ({}).
func (p *Parser) parseFlex(tree *Tree, name string, node *FlexNode) error {
	tok := p.tok.Peek()

	switch tok.Type {
	case TokenSemicolon:
		// Flag mode: just the name with semicolon = true
		p.tok.Next()
		tree.Set(name, "true")
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

		tree.SetContainer(name, child)
		return nil

	case TokenWord, TokenString:
		// Value mode: parse value and semicolon
		value := tok.Value
		p.tok.Next()

		tok = p.tok.Peek()
		if tok.Type != TokenSemicolon {
			return p.errorf(tok, "expected ';' after %s value, got %s", name, tok.Type)
		}
		p.tok.Next()

		tree.Set(name, value)
		return nil

	default:
		return p.errorf(tok, "expected ';', value, or '{' for %s, got %s", name, tok.Type)
	}
}

// parseInlineList parses a list with inline or block syntax.
// Inline: "route 10.0.0.0/8 next-hop 1.1.1.1;"
// Block: "route 10.0.0.0/8 { next-hop 1.1.1.1; }"
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

			// Get value
			tok = p.tok.Peek()
			if tok.Type != TokenWord && tok.Type != TokenString {
				return p.errorf(tok, "expected value for %s.%s, got %s", name, attrName, tok.Type)
			}
			attrValue := tok.Value
			p.tok.Next()

			// Validate if we know this attribute
			if fieldNode := node.Get(attrName); fieldNode != nil {
				if leaf, ok := fieldNode.(*LeafNode); ok {
					if err := ValidateValue(leaf.Type, attrValue); err != nil {
						return p.errorf(tok, "invalid value for %s.%s: %v", name, attrName, err)
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
		switch tok.Type {
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

// skipArray skips an array [ ... ], including nested arrays/blocks.
func (p *Parser) skipArray() error {
	tok := p.tok.Peek()
	if tok.Type != TokenLBracket {
		return p.errorf(tok, "expected '[', got %s", tok.Type)
	}
	p.tok.Next()

	depth := 1
	for depth > 0 {
		tok = p.tok.Next()
		switch tok.Type {
		case TokenLBracket:
			depth++
		case TokenRBracket:
			depth--
		case TokenEOF:
			return p.errorf(tok, "unexpected EOF in array")
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
		switch tok.Type {
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

// errorf creates a formatted error with line info.
func (p *Parser) errorf(tok Token, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("line %d: %s", tok.Line, msg)
}
