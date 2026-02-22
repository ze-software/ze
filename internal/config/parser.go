// Design: docs/architecture/config/syntax.md — config parsing and loading

package config

import (
	"fmt"
)

// KeyDefault is the key used for anonymous list entries (e.g., "api { ... }").
const KeyDefault = "default"

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

// parseNode dispatches parsing based on schema node type.
// Every known node type is handled explicitly; unknown types return an error.
func (p *Parser) parseNode(tree *Tree, name string, node Node) error {
	if n, ok := node.(*LeafNode); ok {
		return p.parseLeaf(tree, name, n)
	}
	if n, ok := node.(*MultiLeafNode); ok {
		return p.parseMultiLeaf(tree, name, n)
	}
	if n, ok := node.(*BracketLeafListNode); ok {
		return p.parseBracketLeafList(tree, name, n)
	}
	if n, ok := node.(*ValueOrArrayNode); ok {
		return p.parseValueOrArray(tree, name, n)
	}
	if n, ok := node.(*ContainerNode); ok {
		return p.parseContainer(tree, name, n)
	}
	if n, ok := node.(*ListNode); ok {
		return p.parseList(tree, name, n)
	}
	if _, ok := node.(*FreeformNode); ok {
		return p.parseFreeform(tree, name)
	}
	if n, ok := node.(*FlexNode); ok {
		return p.parseFlex(tree, name, n)
	}
	if n, ok := node.(*InlineListNode); ok {
		return p.parseInlineList(tree, name, n)
	}
	return fmt.Errorf("unknown node type for %s", name)
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
			// Check if container allows unknown fields (ze:allow-unknown-fields)
			if node.AllowUnknown {
				// Parse unknown field as string leaf: "key value;"
				if err := p.parseUnknownField(child, fieldName); err != nil {
					return err
				}
				continue
			}
			return p.errorf(tok, "unknown field in %s: %s (line %d)", name, fieldName, tok.Line)
		}

		if err := p.parseNode(child, fieldName, fieldNode); err != nil {
			return err
		}
	}

	tree.MergeContainer(name, child)
	return nil
}

// parseUnknownField parses an unknown field as a string value.
// Used for containers with ze:allow-unknown-fields extension.
// Syntax: "key value;" where value is the next word/string token.
func (p *Parser) parseUnknownField(tree *Tree, name string) error {
	tok := p.tok.Peek()

	// Expect a value (word or string)
	if tok.Type != TokenWord && tok.Type != TokenString {
		return p.errorf(tok, "expected value for %s, got %s", name, tok.Type)
	}
	value := tok.Value
	p.tok.Next()

	// Expect semicolon
	tok = p.tok.Peek()
	if tok.Type != TokenSemicolon {
		return p.errorf(tok, "expected ';' after %s value, got %s", name, tok.Type)
	}
	p.tok.Next()

	tree.Set(name, value)
	return nil
}

// errorf creates a formatted error with line info.
func (p *Parser) errorf(tok Token, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("line %d: %s", tok.Line, msg)
}
