// Design: docs/architecture/config/yang-config-design.md -- Value refusals
// Related: parser_diagnostics.go -- typed refusals and their enclosing paths
// Related: cli/cmd_validate.go -- collects every safe parser diagnostic

package config

import "errors"

// ParseDiagnostics parses input with the normal grammar and collects value
// refusals whose scalar statements have ended. Structural errors stop the
// parse. Any error returns a nil tree, so a partial configuration cannot be
// used. Parse retains its first-error behavior. Parser is not concurrent-safe.
func (p *Parser) ParseDiagnostics(input string) (*Tree, []error) {
	p.collecting = true
	p.refusals = nil
	defer func() {
		p.collecting = false
		p.refusals = nil
	}()

	tree, err := p.Parse(input)
	refusals := p.refusals
	for i := range refusals {
		refusals[i] = namedRefusal(refusals[i])
	}
	if err != nil {
		refusals = append(refusals, err)
	}
	if len(refusals) > 0 {
		return nil, refusals
	}
	return tree, nil
}

// prefixRefusals adds an enclosing node or list key to the diagnostics
// collected inside it. Each enclosing parser owns its own path segment.
func (p *Parser) prefixRefusals(start int, name string) {
	for i := start; i < len(p.refusals); i++ {
		p.refusals[i] = prependSyntaxPath(p.refusals[i], name)
	}
}

// collectValueRefusal resumes only at a scalar statement's own terminator.
// A failed container or list has not consumed its closing delimiter, so it
// cannot be treated as a complete statement and is left to unwind normally.
func (p *Parser) collectValueRefusal(err error, node Node) error {
	if !p.collecting {
		return err
	}
	if _, ok := operatorRefusal(err); !ok {
		return err
	}
	switch node.(type) {
	case *LeafNode, *MultiLeafNode, *BracketLeafListNode, *ValueOrArrayNode:
		// These nodes consume one scalar statement, never a child block.
	default:
		return err
	}
	var refusal *syntaxRefusal
	if !errors.As(err, &refusal) {
		return err
	}
	if !refusal.statementEnded {
		if p.tok.peek().kind != tokenSemicolon {
			return err
		}
		p.tok.next()
	}
	p.refusals = append(p.refusals, err)
	return nil
}

// ParseSetDiagnostics preserves ParseSetFormat's normal verdict and collects
// additional typed value refusals at complete set or set-meta line boundaries.
// Structural errors stop collection. Every invalid input returns a nil tree.
// format MUST be FormatSet or FormatSetMeta, as returned by DetectFormat.
func ParseSetDiagnostics(schema *Schema, input string, format ConfigFormat) (*Tree, []error) {
	tree, err := ParseSetFormat(schema, input, format)
	if err == nil {
		return tree, nil
	}
	if _, ok := operatorRefusal(err); !ok {
		return nil, []error{err}
	}
	p := NewSetParser(schema)
	p.collecting = true
	switch format {
	case FormatSet:
		_, err = p.Parse(input)
	case FormatSetMeta:
		_, _, err = p.ParseWithMeta(input)
	case FormatHierarchical:
		panic("BUG: ParseSetDiagnostics called with hierarchical input")
	}
	if err != nil {
		p.refusals = append(p.refusals, err)
	}
	return nil, p.refusals
}

// collectLineRefusal records only typed value failures. The scanner has
// already consumed the whole line, so the next line starts independently.
func (p *SetParser) collectLineRefusal(err error) bool {
	if !p.collecting {
		return false
	}
	if _, ok := operatorRefusal(err); !ok {
		return false
	}
	p.refusals = append(p.refusals, err)
	return true
}
