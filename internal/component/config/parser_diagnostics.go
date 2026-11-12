// Design: docs/architecture/config/syntax.md — schema-driven corrective syntax.
// Related: parser.go — enum and container refusals collect their original path.

package config

import (
	"errors"
	"fmt"
	"slices"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// syntaxRefusal carries a corrective parse error. Its path is built only while
// unwinding a failed parse, so successful parsing allocates no diagnostic state.
// path is the slash-joined form the refusal prints; segments holds the same
// names one per element, a list key holding a slash included, so a value
// refusal can be rebuilt naming them (namedRefusal).
type syntaxRefusal struct {
	path     string
	segments []string
	cause    error
}

func (e *syntaxRefusal) Error() string {
	var b textbuf.Buffer
	return b.Str(e.path).Str(": ").Err(e.cause).String()
}

func (e *syntaxRefusal) Unwrap() error { return e.cause }

// prependSyntaxPath preserves other parser errors verbatim. Each caller adds
// exactly the node names or list key it owns as the refusal returns to the
// root: one name, or the whole context of a ParseAt.
func prependSyntaxPath(err error, names ...string) error {
	var refusal *syntaxRefusal
	if !errors.As(err, &refusal) {
		return err
	}
	refusal.segments = append(slices.Clone(names), refusal.segments...)
	prefix := textbuf.Join(names, "/")
	if refusal.path == "" {
		refusal.path = prefix
		return err
	}
	refusal.path = prefix + "/" + refusal.path
	return err
}

// operatorRefusal answers the enum, union, range, length or pattern refusal
// err holds, the one kind that carries an operator error a frame can rebuild.
func operatorRefusal(err error) (*configyang.ValidationError, bool) {
	var refusal *configyang.ValidationError
	if !errors.As(err, &refusal) {
		return nil, false
	}
	holds := refusal.HoldsOperatorError()
	return refusal, holds
}

// valueRefusal answers err, the refusal of the value or key the parser read
// at tok. An operator refusal goes back inside a syntaxRefusal, so each caller
// names its node or key on the way to the root, where Parse writes the whole
// path into it once (namedRefusal). Its fix is the line the operator writes in
// the file, which fix builds from the refusal's correction (the close value,
// or the accepted values): fileFix, or leafFix for a leaf. segments are the
// names this arm owns below its caller's, nil when the caller names them all.
// Any other refusal keeps its text: the line error format and args describe.
func (p *Parser) valueRefusal(tok token, err error, fix func(correction string) string, segments []string, format string, args ...any) error {
	refusal, ok := operatorRefusal(err)
	if !ok {
		return p.errorf(tok, format, args...)
	}
	return &syntaxRefusal{
		path:     textbuf.Join(segments, "/"),
		segments: slices.Clone(segments),
		cause:    newLineError(tok.line, refusal.WithFix(fix(refusal.Correction()))),
	}
}

// namedRefusal writes the path the unwind collected into a value refusal, at
// the parse root: the refusal is rebuilt naming the path the operator types,
// words separated by spaces and keys included, so no slash path is printed
// before it. Every other refusal is answered as it came.
func namedRefusal(err error) error {
	var refusal *syntaxRefusal
	if !errors.As(err, &refusal) {
		return err
	}
	var line *LineError
	if !errors.As(refusal.cause, &line) {
		return err
	}
	value, ok := operatorRefusal(line.cause)
	if !ok {
		return err
	}
	return newLineError(line.line, value.WithPath(refusal.segments))
}

// containerValueFix renders the immediate schema children, not a feature-specific
// example. Nested containers are named without recursing into their schemas.
func containerValueFix(node *ContainerNode, name string) string {
	var b textbuf.Buffer
	b.Str(name).Str(" {")
	for _, child := range node.Children() {
		b.Byte(' ')
		switch field := node.Get(child).(type) {
		case *LeafNode:
			switch {
			case len(field.Enums) != 0:
				b.Str(leafValueFix(field, child, ""))
			case field.Type == TypeEmpty:
				b.Str(child).Byte(';')
			default: // A leaf with no enumeration takes any VALUE.
				b.Str(child).Str(" VALUE;")
			}
		case *MultiLeafNode, *BracketLeafListNode, *ValueOrArrayNode:
			b.Str(child).Str(" [ VALUE ];")
		default: // Other schema node kinds need their own nested fields.
			b.Str(child).Str(" { ... }")
		}
	}
	return b.Str(" }").String()
}

// containerGivenValue refuses a value written on a non-presence container and
// names the container's fields as the line to write instead.
func (p *Parser) containerGivenValue(tok token, name string, node *ContainerNode) error {
	return &syntaxRefusal{cause: p.errorf(tok, "expected '{' after %s, got %s\n  write: %s", name, tok.kind, containerValueFix(node, name))}
}

// setSyntaxRefusal adds the same schema corrections to set and set-meta errors.
// The walk is bounded by the refused line's tokens and runs only on failure.
func (p *SetParser) setSyntaxRefusal(tokens []string, err error) error {
	if err == nil {
		return nil
	}
	// A value refusal already names the path, the accepted values and the
	// set line that fixes it.
	if _, ok := operatorRefusal(err); ok {
		return err
	}
	var parent ChildProvider = p.schema
	var node Node
	end := 0
	for end < len(tokens) {
		child := parent.Get(tokens[end])
		if child == nil {
			break
		}
		node = child
		end++
		if list, ok := node.(*ListNode); ok {
			end += ListAddressLen(list, tokens[end:])
		}
		next, ok := node.(ChildProvider)
		if !ok {
			break
		}
		parent = next
	}
	if end == 0 {
		return err
	}
	switch field := node.(type) {
	case *LeafNode:
		if len(field.Enums) == 0 {
			return err
		}
		return &syntaxRefusal{
			path:     textbuf.Join(tokens[:end], "/"),
			segments: slices.Clone(tokens[:end]),
			cause:    fmt.Errorf("%w\n  write: set %s <%s>", err, textbuf.Join(tokens[:end], " "), textbuf.Join(field.Enums, "|")),
		}
	case *ContainerNode:
		if !field.Presence {
			return err
		}
		return &syntaxRefusal{
			path:     textbuf.Join(tokens[:end], "/"),
			segments: slices.Clone(tokens[:end]),
			cause:    fmt.Errorf("%w\n  write: %s", err, containerValueFix(field, tokens[end-1])),
		}
	default: // Other schema node kinds retain their existing diagnostics.
		return err
	}
}
