// Design: docs/architecture/config/syntax.md — config parsing and loading
// Detail: parser_list.go — list and multi-leaf parsing
// Detail: parser_freeform.go — freeform, flex, and inline parsing
// Detail: tokenizer.go — lexical tokenizer for config input
// Related: tree.go — Tree data structure
// Related: setparser.go — set-style config parsing
// Related: reader.go — config file loading and handler routing

package config

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config/secret"
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

		// Handle "inactive: <name> ..." sugar at the top level. Mirrors
		// the same sugar inside container blocks (parseContainer) and list
		// entries (parseListFieldBlock); without it, top-level leaves and
		// containers cannot be deactivated through the file syntax.
		markInactive := false
		if name == InactiveLeafName+":" {
			markInactive = true
			tok = p.tok.Peek()
			if tok.Type != TokenWord {
				return nil, p.errorf(tok, "expected name after inactive:, got %s", tok.Type)
			}
			name = tok.Value
			p.tok.Next()
		}

		node := p.schema.Get(name)
		if node == nil {
			return nil, p.errorf(tok, "unknown top-level keyword: %s", name)
		}

		if err := p.parseNode(tree, name, node); err != nil {
			return nil, err
		}

		if markInactive {
			applyInactive(tree, name, node, p, tok.Line)
		}
	}

	return tree, nil
}

// applyInactive records the inactive flag on the just-parsed node. Shared
// by parseRoot, parseContainer, and parseListFieldBlock so that container,
// list-entry, and leaf cases follow one rule. FlexNode and InlineListNode
// are dual-natured; the helper checks where the parser actually deposited
// the data (values, multiValues, containers, lists) and marks accordingly.
func applyInactive(tree *Tree, name string, node Node, p *Parser, line int) {
	switch node.(type) {
	case *LeafNode, *MultiLeafNode, *BracketLeafListNode, *ValueOrArrayNode:
		tree.SetLeafInactive(name, true)
		return
	case *FlexNode:
		// Flex landed as scalar (values), multi (multiValues), or as a
		// container -- mark whichever the parser produced.
		if _, ok := tree.Get(name); ok {
			tree.SetLeafInactive(name, true)
			return
		}
		if mv := tree.GetMultiValues(name); len(mv) > 0 {
			tree.SetLeafInactive(name, true)
			return
		}
		if sub := tree.GetContainer(name); sub != nil {
			sub.Set(InactiveLeafName, configTrue)
			return
		}
	}
	if sub := tree.GetContainer(name); sub != nil {
		sub.Set(InactiveLeafName, configTrue)
		return
	}
	if entries := tree.GetList(name); entries != nil {
		order := tree.listOrder[name]
		if len(order) > 0 {
			lastKey := order[len(order)-1]
			if entry, ok := entries[lastKey]; ok {
				entry.Set(InactiveLeafName, configTrue)
				return
			}
		}
	}
	p.warn(line, "inactive: prefix ignored on %s (no node materialized)", name)
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
// Sensitive leaves accept $9$-encoded values and decode them to plaintext.
func (p *Parser) parseLeaf(tree *Tree, name string, node *LeafNode) error {
	tok := p.tok.Peek()

	var value string
	if tok.Type == TokenWord || tok.Type == TokenString {
		value = tok.Value
		p.tok.Next()
	} else {
		return p.errorf(tok, "expected value for %s, got %s", name, tok.Type)
	}

	// Decode $9$-encoded values on sensitive leaves.
	// Skipped for ze:bcrypt leaves: bcrypt is one-way, cannot share the
	// $9$ reversible path. A $9$-prefixed string on a bcrypt leaf is
	// preserved verbatim (and will fail bcrypt format validation later).
	if node.Sensitive && !node.Bcrypt && secret.IsEncoded(value) {
		decoded, err := secret.Decode(value)
		if err != nil {
			return p.errorf(tok, "invalid $9$ encoding for %s: %v", name, err)
		}
		value = decoded
	}

	// Validate value type and YANG restrictions (works on decoded plaintext too).
	if err := ValidateLeafValue(node, value); err != nil {
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
// YANG presence containers also accept: flag `name;`, value `name word;`,
// and parenthesized `name ( ... );` forms.
func (p *Parser) parseContainer(tree *Tree, name string, node *ContainerNode) error {
	tok := p.tok.Peek()

	// YANG presence containers accept flag (;), value (word;), paren (...), and block ({})
	if node.Presence && tok.Type == TokenSemicolon {
		// Flag mode: "route-refresh;" → true
		p.tok.Next()
		tree.Set(name, configTrue)
		return nil
	}
	if node.Presence && (tok.Type == TokenWord || tok.Type == TokenString) {
		// Value mode: "route-refresh true;" → store value
		value := tok.Value
		p.tok.Next()
		tok = p.tok.Peek()
		if tok.Type != TokenSemicolon {
			return p.errorf(tok, "expected ';' after %s value, got %s", name, tok.Type)
		}
		p.tok.Next()
		tree.Set(name, value)
		return nil
	}
	if node.Presence && tok.Type == TokenLParen {
		// Parenthesized mode: "bgp-prefix-sid-srv6 ( l3-service 2001:1:: );"
		return p.parsePresenceParenthesized(tree, name)
	}

	if tok.Type != TokenLBrace {
		if node.Presence {
			return p.errorf(tok, "expected ';', value, or '{' after %s, got %s", name, tok.Type)
		}
		// Automatic brace insertion (ABI): if the next token is a known child name,
		// parse it inline without braces -- same principle as the tokenizer's ASI.
		if tok.Type == TokenWord {
			if childNode := node.Get(tok.Value); childNode != nil {
				childName := tok.Value
				p.tok.Next() // consume child name
				child := NewTree()
				if err := p.parseNode(child, childName, childNode); err != nil {
					return err
				}
				tree.MergeContainer(name, child)
				return nil
			}
		}
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

		// Handle "inactive: <field> { ... }" sugar.
		// The tokenizer reads "inactive:" as a single word.
		markInactive := false
		if fieldName == InactiveLeafName+":" {
			markInactive = true
			// The real field name is the next token.
			tok = p.tok.Peek()
			if tok.Type != TokenWord {
				return p.errorf(tok, "expected field name after inactive:, got %s", tok.Type)
			}
			fieldName = tok.Value
			p.tok.Next()
		}

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

		if markInactive {
			applyInactive(child, fieldName, fieldNode, p, tok.Line)
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

// parsePresenceParenthesized handles parenthesized content for presence containers.
// Syntax: "name ( content ... );" — collects content and stores as value string.
func (p *Parser) parsePresenceParenthesized(tree *Tree, name string) error {
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
	tok := p.tok.Peek()
	if tok.Type == TokenSemicolon {
		p.tok.Next()
	}

	tree.Set(name, value.String())
	return nil
}

// errorf creates a formatted error with line info.
func (p *Parser) errorf(tok Token, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("line %d: %s", tok.Line, msg)
}
