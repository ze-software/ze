// Design: docs/architecture/config/syntax.md — config parsing and loading
// Detail: parser_list.go — list and multi-leaf parsing
// Detail: parser_freeform.go — freeform, flex, and inline parsing
// Detail: tokenizer.go — lexical tokenizer for config input
// Related: tree.go — Tree data structure
// Related: setparser.go — set-style config parsing
// Related: reader.go — config file loading and handler routing

package config

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/config/secret"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// KeyDefault is the key used for anonymous list entries (e.g., "api { ... }").
const KeyDefault = "default"

// Parser parses ExaBGP-style configuration.
type Parser struct {
	schema   *Schema
	tok      *tokenizer
	warnings []string
}

// NewParser creates a new parser with the given schema.
func NewParser(schema *Schema) *Parser {
	return &Parser{schema: schema}
}

// Parse parses the input string into a config tree.
func (p *Parser) Parse(input string) (*Tree, error) {
	p.tok = newTokenizer(input)
	p.warnings = nil
	return p.parseRoot()
}

// Warnings returns any warnings generated during parsing.
func (p *Parser) Warnings() []string {
	return p.warnings
}

func (p *Parser) warn(line int, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	var b textbuf.Buffer
	p.warnings = append(p.warnings, b.Reset().Str("line ").Int(int64(line)).Str(": ").Str(msg).String())
}

// ParseAt parses input as the body of the node contextPath names, so a
// fragment loaded at an editor context ("load terminal relative" at bgp peer
// peer1) is checked against that node's children. The answer is rooted AT the
// context. An empty contextPath is Parse.
func (p *Parser) ParseAt(input string, contextPath []string) (*Tree, error) {
	if len(contextPath) == 0 {
		return p.Parse(input)
	}
	if p.schema.LookupTokenPath(contextPath) == nil {
		var b textbuf.Buffer
		return nil, errors.New(b.Str("unknown context path: ").Join(contextPath, " ").String())
	}
	p.tok = newTokenizer(input)
	p.warnings = nil
	context := slices.Clip(slices.Clone(contextPath))
	var known []string
	if parent, ok := p.schema.LookupTokenPath(context).(ChildProvider); ok {
		known = parent.Children()
	}
	return p.parseBody("unknown keyword", known, func(name string) Node {
		return p.schema.LookupTokenPath(append(context, name))
	})
}

// parseRoot parses the top level of the config.
func (p *Parser) parseRoot() (*Tree, error) {
	return p.parseBody("unknown top-level keyword", p.schema.Children(), p.schema.Get)
}

// parseBody parses statements until EOF, resolving each keyword through
// lookup: the schema root for a whole file, a context node for ParseAt. known
// lists the keywords valid there, for the closest-key hint of a refusal.
func (p *Parser) parseBody(unknown string, known []string, lookup func(name string) Node) (*Tree, error) {
	tree := NewTree()

	for {
		tok := p.tok.peek()
		if tok.kind == tokenEOF {
			break
		}

		if tok.kind != tokenWord {
			return nil, p.errorf(tok, "expected keyword, got %s", tok.kind)
		}

		name := tok.value
		p.tok.next() // consume name

		// Handle "inactive: <name> ..." sugar at the top level. Mirrors
		// the same sugar inside container blocks (parseContainer) and list
		// entries (parseListFieldBlock); without it, top-level leaves and
		// containers cannot be deactivated through the file syntax.
		markInactive := false
		if name == InactiveLeafName+":" {
			markInactive = true
			tok = p.tok.peek()
			if tok.kind != tokenWord {
				return nil, p.errorf(tok, "expected name after inactive:, got %s", tok.kind)
			}
			name = tok.value
			p.tok.next()
		}

		node := lookup(name)
		if node == nil {
			return nil, p.errorf(tok, "%s: %s%s", unknown, name, UnknownKeywordHint(name, known))
		}

		if markInactive {
			if err := p.parseNodeInactive(tree, name, node, tok.line); err != nil {
				return nil, err
			}
			continue
		}

		if err := p.parseNode(tree, name, node); err != nil {
			return nil, err
		}
	}

	return tree, nil
}

// parseNodeInactive parses one statement that carried the "inactive:"
// prefix. Leaf-lists support member-level deactivation --
// `inactive: <leaf-list> <member>` is the hierarchical spelling of the set
// format's `nop <leaf-list> <member>`: it deactivates <member>, adding it
// deactivated when the list does not hold it yet (the form the serializer
// writes for deactivated members); a bracket leaf-list takes this form
// whenever no '[' follows its name. Every other shape parses normally and
// the whole node is marked inactive.
// Shared by parseRoot, parseContainer, and parseListFieldBlock.
func (p *Parser) parseNodeInactive(tree *Tree, name string, node Node, line int) error {
	if vn, ok := node.(*ValueOrArrayNode); ok {
		return p.parseInactiveValueOrArray(tree, name, vn, line)
	}
	if bn, ok := node.(*BracketLeafListNode); ok {
		if tok := p.tok.peek(); tok.kind != tokenLBracket {
			return p.parseInactiveBracketMember(tree, name, bn)
		}
	}
	if err := p.parseNode(tree, name, node); err != nil {
		return err
	}
	applyInactive(tree, name, node, p, line)
	return nil
}

// parseInactiveValueOrArray handles `inactive: <leaf-list> ...` statements.
// A single bare value is one member, as `nop <leaf-list> <member>` is in set
// format (owner decision 2026-10-11): an active member is deactivated, a
// deactivated one stays so, and a value the list does not hold is added
// deactivated. The bracket form and multiple values keep the whole-leaf
// semantics: the items replace the leaf content and the leaf itself is
// marked inactive.
func (p *Parser) parseInactiveValueOrArray(tree *Tree, name string, node *ValueOrArrayNode, line int) error {
	items, bracket, tok, err := p.collectValueOrArrayItems(name)
	if err != nil {
		return err
	}
	if !bracket && len(items) == 1 {
		present, inactive := tree.MultiValueMemberState(name, items[0])
		if inactive {
			return nil
		}
		if !present {
			if err := p.storeValueOrArray(tree, name, node, items, tok); err != nil {
				return err
			}
		}
		if err := tree.DeactivateMultiValue(name, items[0]); err != nil {
			return p.errorf(tok, "deactivate member %s %s: %v", name, items[0], err)
		}
		return nil
	}
	if err := p.storeValueOrArray(tree, name, node, items, tok); err != nil {
		return err
	}
	applyInactive(tree, name, node, p, line)
	return nil
}

// parseInactiveBracketMember handles `inactive: <leaf-list> <member>;` for a
// bracket leaf-list, the statement the serializer writes after the list for
// each deactivated member. Like `nop <leaf-list> <member>` in set format, it
// adds a member the list does not hold, deactivated (owner decision
// 2026-10-11). `inactive: <leaf-list> [ ... ];` is the whole leaf and does not
// come here.
func (p *Parser) parseInactiveBracketMember(tree *Tree, name string, node *BracketLeafListNode) error {
	tok := p.tok.peek()
	if tok.kind != tokenWord && tok.kind != tokenString {
		return p.errorf(tok, "expected '[' or a member after inactive: %s, got %s", name, tok.kind)
	}
	member := tok.value
	p.tok.next()
	if end := p.tok.peek(); end.kind != tokenSemicolon {
		return p.errorf(end, "expected ';' after inactive: %s %s, got %s", name, member, end.kind)
	}
	p.tok.next()
	present, inactive := tree.MultiValueMemberState(name, member)
	if inactive {
		return nil
	}
	if !present {
		if err := validateValuePatterns(node.Type, node.Patterns, member); err != nil {
			return p.errorf(tok, "invalid value for %s: %v", name, err)
		}
		tree.AddMultiValueMember(name, member)
	}
	if err := tree.DeactivateMultiValue(name, member); err != nil {
		return p.errorf(tok, "deactivate member %s %s: %v", name, member, err)
	}
	return nil
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
			sub.SetInactive(true)
			return
		}
	}
	if sub := tree.GetContainer(name); sub != nil {
		sub.SetInactive(true)
		return
	}
	if entries := tree.GetList(name); entries != nil {
		order := tree.listOrder[name]
		if len(order) > 0 {
			lastKey := order[len(order)-1]
			if entry, ok := entries[lastKey]; ok {
				entry.SetInactive(true)
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
	// "type empty" leaves are presence flags. The canonical form is a bare
	// "no-default-route;"; "no-default-route true;" is also accepted so
	// serialized value-form output round-trips and the pre-TypeEmpty value
	// form stays valid. Presence always records true.
	//
	// RFC 7950 Section 9.11.2 gives an empty leaf no lexical value at all, so
	// any other word is refused rather than ignored: "disable false;" used to
	// store presence and disable the interface it meant to keep enabled.
	if node.Type == TypeEmpty {
		tok := p.tok.peek()
		if tok.kind == tokenWord || tok.kind == tokenString {
			if tok.value != configTrue {
				return p.errorf(tok, "%s is a flag and takes no value, got %q: the line would set the flag whatever the word says\n  write %q to set it, or delete the line to clear it",
					name, tok.value, name+";")
			}
			p.tok.next() // the round-trip presence value
			tok = p.tok.peek()
		}
		if tok.kind != tokenSemicolon {
			return p.errorf(tok, "expected ';' after %s, got %s", name, tok.kind)
		}
		p.tok.next()
		tree.Set(name, configTrue)
		return nil
	}

	tok := p.tok.peek()

	var value string
	if tok.kind != tokenWord && tok.kind != tokenString {
		if fix := leafValueFix(node, name, ""); fix != "" {
			return p.errorf(tok, "%s needs a value, one of %s, and is not a block or a bare word\n  write: %s",
				name, textbuf.Join(node.Enums, ", "), fix)
		}
		return p.errorf(tok, "expected value for %s, got %s", name, tok.kind)
	}

	value = tok.value
	p.tok.next()

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
		if fix := leafValueFix(node, name, value); fix != "" {
			return p.errorf(tok, "invalid value for %s: %v\n  write: %s", name, err, fix)
		}
		return p.errorf(tok, "invalid value for %s: %v", name, err)
	}

	// Rewrite the accepted value into the one spelling the tree stores
	// (enable to true, an asdot AS number to its decimal value).
	value = NormalizeLeafValue(node.Type, value)

	// Expect semicolon
	tok = p.tok.peek()
	if tok.kind != tokenSemicolon {
		return p.errorf(tok, "expected ';' after %s value, got %s", name, tok.kind)
	}
	p.tok.next()

	tree.Set(name, value)
	return nil
}

// parseContainer parses a container block: `name { ... }`.
// YANG presence containers also accept: flag `name;`, value `name word;`,
// and parenthesized `name ( ... );` forms.
func (p *Parser) parseContainer(tree *Tree, name string, node *ContainerNode) error {
	tok := p.tok.peek()

	// YANG presence containers accept flag (;), value (word;), paren (...), and block ({})
	if node.Presence && tok.kind == tokenSemicolon {
		// Flag mode: "route-refresh;" → true
		p.tok.next()
		tree.Set(name, configTrue)
		return nil
	}
	if node.Presence && (tok.kind == tokenWord || tok.kind == tokenString) {
		// Value mode: "route-refresh true;" → store value
		value := tok.value
		p.tok.next()
		tok = p.tok.peek()
		if tok.kind != tokenSemicolon {
			return p.errorf(tok, "expected ';' after %s value, got %s", name, tok.kind)
		}
		p.tok.next()
		tree.Set(name, value)
		return nil
	}
	if node.Presence && tok.kind == tokenLParen {
		// Parenthesized mode: "bgp-prefix-sid-srv6 ( l3-service 2001:1:: );"
		return p.parsePresenceParenthesized(tree, name)
	}

	if tok.kind != tokenLBrace {
		if node.Presence {
			return p.errorf(tok, "expected ';', value, or '{' after %s, got %s", name, tok.kind)
		}
		// Automatic brace insertion (ABI): if the next token is a known child name,
		// parse it inline without braces -- same principle as the tokenizer's ASI.
		if tok.kind == tokenWord {
			if childNode := node.Get(tok.value); childNode != nil {
				childName := tok.value
				p.tok.next() // consume child name
				child := NewTree()
				if err := p.parseNode(child, childName, childNode); err != nil {
					return err
				}
				if err := tree.MergeContainer(name, child, node); err != nil {
					return p.errorf(tok, "%v", err)
				}
				return nil
			}
		}
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
		if tok.kind != tokenWord {
			return p.errorf(tok, "expected keyword in %s block, got %s", name, tok.kind)
		}

		fieldName := tok.value
		p.tok.next()

		// Handle "inactive: <field> { ... }" sugar.
		// The tokenizer reads "inactive:" as a single word.
		markInactive := false
		if fieldName == InactiveLeafName+":" {
			markInactive = true
			// The real field name is the next token.
			tok = p.tok.peek()
			if tok.kind != tokenWord {
				return p.errorf(tok, "expected field name after inactive:, got %s", tok.kind)
			}
			fieldName = tok.value
			p.tok.next()
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
			return p.errorf(tok, "unknown field in %s: %s%s (line %d)", name, fieldName, UnknownKeywordHint(fieldName, node.Children()), tok.line)
		}

		if markInactive {
			if err := p.parseNodeInactive(child, fieldName, fieldNode, tok.line); err != nil {
				return err
			}
			continue
		}

		if err := p.parseNode(child, fieldName, fieldNode); err != nil {
			return err
		}
	}

	if err := tree.MergeContainer(name, child, node); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// leafValueFix answers the line that replaces an enumeration leaf written
// without a usable value, or "" when the leaf is not an enumeration. Three
// capabilities (route-refresh, extended-message, link-local-nexthop) were
// presence containers until 2026-10-11 and accepted true and false: their old
// bare, block and true spellings meant enable, and false meant disable, so the
// fix names that word when the enumeration offers it. Any other value gets the
// leaf with its words as a choice, because Ze cannot know which one was meant.
func leafValueFix(node *LeafNode, name, given string) string {
	if len(node.Enums) == 0 {
		return ""
	}
	var b textbuf.Buffer
	b.Str(name).Byte(' ')
	switch {
	case (given == "" || given == configTrue) && slices.Contains(node.Enums, "enable"):
		b.Str("enable")
	case given == configFalse && slices.Contains(node.Enums, "disable"):
		b.Str("disable")
	default:
		b.Byte('<').Str(textbuf.Join(node.Enums, "|")).Byte('>')
	}
	b.Byte(';')
	return b.String()
}

// parseUnknownField parses an unknown field as a string value.
// Used for containers with ze:allow-unknown-fields extension.
// Syntax: "key value;" where value is the next word/string token.
func (p *Parser) parseUnknownField(tree *Tree, name string) error {
	tok := p.tok.peek()

	// Expect a value (word or string)
	if tok.kind != tokenWord && tok.kind != tokenString {
		return p.errorf(tok, "expected value for %s, got %s", name, tok.kind)
	}
	value := tok.value
	p.tok.next()

	// Expect semicolon
	tok = p.tok.peek()
	if tok.kind != tokenSemicolon {
		return p.errorf(tok, "expected ';' after %s value, got %s", name, tok.kind)
	}
	p.tok.next()

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
	// Optional semicolon after parenthesized content
	tok := p.tok.peek()
	if tok.kind == tokenSemicolon {
		p.tok.next()
	}

	tree.Set(name, textbuf.Join(parenVals, " "))
	return nil
}

// errorf creates a formatted error with line info.
func (p *Parser) errorf(tok token, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("line %d: %s", tok.line, msg)
}
