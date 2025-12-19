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
	case *ContainerNode:
		return p.parseContainer(tree, name, n)
	case *ListNode:
		return p.parseList(tree, name, n)
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

// errorf creates a formatted error with line info.
func (p *Parser) errorf(tok Token, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("line %d: %s", tok.Line, msg)
}
