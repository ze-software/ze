// Design: docs/architecture/config/syntax.md — schema-driven corrective syntax.
// Related: parser.go — enum and container refusals collect their original path.

package config

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// syntaxRefusal carries a corrective parse error. Its path is built only while
// unwinding a failed parse, so successful parsing allocates no diagnostic state.
type syntaxRefusal struct {
	path  string
	cause error
}

func (e *syntaxRefusal) Error() string {
	var b textbuf.Buffer
	return b.Str(e.path).Str(": ").Err(e.cause).String()
}

func (e *syntaxRefusal) Unwrap() error { return e.cause }

// prependSyntaxPath preserves other parser errors verbatim. Each caller adds
// exactly the node name or list key it owns as the refusal returns to the root.
func prependSyntaxPath(err error, name string) error {
	var refusal *syntaxRefusal
	if !errors.As(err, &refusal) {
		return err
	}
	if refusal.path == "" {
		refusal.path = name
		return err
	}
	refusal.path = name + "/" + refusal.path
	return err
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
			path:  textbuf.Join(tokens[:end], "/"),
			cause: fmt.Errorf("%w\n  write: set %s <%s>", err, textbuf.Join(tokens[:end], " "), textbuf.Join(field.Enums, "|")),
		}
	case *ContainerNode:
		if !field.Presence {
			return err
		}
		return &syntaxRefusal{
			path:  textbuf.Join(tokens[:end], "/"),
			cause: fmt.Errorf("%w\n  write: %s", err, containerValueFix(field, tokens[end-1])),
		}
	default: // Other schema node kinds retain their existing diagnostics.
		return err
	}
}
