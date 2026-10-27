// Design: docs/architecture/config/change-file-structural-ops.md -- entry identity.
// Related: list_key.go -- canonical addresses and quote-aware token codec.

package config

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// EditTokens preserves key boundaries inside the editor. Text exists only at
// persistence and presentation boundaries, where String applies the key codec.
type EditTokens []string

// String renders a token path with configuration-key quoting.
func (path EditTokens) String() string {
	return FormatEditPath(path)
}

// RebaseEditTokens moves a token path between complete schema-resolved entry
// addresses. The returned tokens are owned by the caller.
func RebaseEditTokens(schema *Schema, path, source, destination []string) (EditTokens, error) {
	resolved, err := resolveEditPath(schema, path, nil, true)
	if err != nil {
		return nil, err
	}
	from, err := resolveEditPath(schema, source, nil, true)
	if err != nil {
		return nil, err
	}
	target, err := resolveEditPath(schema, destination, nil, true)
	if err != nil {
		return nil, err
	}
	if !resolved.HasPrefix(from) {
		return resolved.tokens, nil
	}
	return append(target.tokens, resolved.tokens[len(from.tokens):]...), nil
}

// FormatEditPath writes path tokens with the same quoting as configuration keys.
func FormatEditPath(tokens []string) string {
	var b textbuf.Buffer
	writeListAddress(&b, tokens)
	return b.String()
}

// ParseEditPath reads the token boundaries FormatEditPath preserves.
func ParseEditPath(path string) ([]string, error) {
	return splitQuotedTokens(path)
}

// CanonicalEditPath normalizes every list address, not just the final entry.
// Explicit defaults and differently ordered key pairs name the same path.
func CanonicalEditPath(schema *Schema, tokens []string) ([]string, error) {
	resolved, err := resolveEditPath(schema, tokens, make([]string, 0, len(tokens)), false)
	return resolved.tokens, err
}

// editEntrySpan marks a multi-token entry address in canonical tokens. A prefix
// cannot end inside it: a shorter map key names a sibling, not an ancestor.
type editEntrySpan struct {
	start int
	end   int
}

// EditPath is a validated canonical address. Its comparisons cannot fail:
// callers resolve each ownership address before making any overlap decision.
type EditPath struct {
	tokens  []string
	entries []editEntrySpan
}

func (path EditPath) hasPrefix(prefix []string) bool {
	if len(path.tokens) < len(prefix) || !slices.Equal(path.tokens[:len(prefix)], prefix) {
		return false
	}
	for _, entry := range path.entries {
		if len(prefix) > entry.start && len(prefix) < entry.end {
			return false
		}
	}
	return true
}

// EditPathHasPrefix compares schema boundaries. Failure to parse or resolve
// either address is an error, never evidence of non-overlap or absent ownership.
func EditPathHasPrefix(schema *Schema, path, prefix string) (bool, error) {
	resolved, err := readEditPath(schema, path, true)
	if err != nil {
		return false, err
	}
	ancestor, err := readEditPath(schema, prefix, false)
	if err != nil {
		return false, err
	}
	return resolved.hasPrefix(ancestor.tokens), nil
}

// RebaseEditPath validates each operand once, replaces an actual ancestor and
// writes the unchanged suffix directly, without allocating a combined token list.
func RebaseEditPath(schema *Schema, path, source, destination string) (string, error) {
	resolved, err := readEditPath(schema, path, true)
	if err != nil {
		return "", err
	}
	ancestor, err := readEditPath(schema, source, false)
	if err != nil {
		return "", err
	}
	target, err := readEditPath(schema, destination, false)
	if err != nil {
		return "", err
	}
	if !resolved.hasPrefix(ancestor.tokens) {
		return path, nil
	}
	var b textbuf.Buffer
	writeListAddress(&b, target.tokens)
	suffix := resolved.tokens[len(ancestor.tokens):]
	if len(suffix) != 0 {
		if len(target.tokens) != 0 {
			b.Byte(' ')
		}
		writeListAddress(&b, suffix)
	}
	return b.String(), nil
}

func readEditPath(schema *Schema, path string, entryBounds bool) (EditPath, error) {
	tokens, err := ParseEditPath(path)
	if err != nil {
		return EditPath{}, fmt.Errorf("parse edit path %q: %w", path, err)
	}
	resolved, err := resolveEditPath(schema, tokens, tokens[:0], entryBounds)
	if err != nil {
		return EditPath{}, fmt.Errorf("resolve edit path %q: %w", path, err)
	}
	return resolved, nil
}

// resolveEditPath writes canonical tokens into output. A parsed path owns its
// tokens and may reuse them: canonical key ordering/default omission never
// expands an address, so writes cannot overtake the unread input. Borrowed
// token callers provide a separate output. Entry boundaries are optional.
func resolveEditPath(schema *Schema, tokens, output []string, entryBounds bool) (EditPath, error) {
	if schema == nil {
		return EditPath{}, errors.New("edit path schema unavailable")
	}
	var current ChildProvider = schema
	result := EditPath{tokens: output}
	for i := 0; i < len(tokens); {
		name := tokens[i]
		node := current.Get(name)
		if node == nil {
			return EditPath{}, fmt.Errorf("unknown path element: %s", name)
		}
		result.tokens = append(result.tokens, name)
		i++
		switch n := node.(type) {
		case *ContainerNode:
			current = n
		case *FlexNode:
			current = n
		case *ListNode:
			addr, err := ListPathStep(name, n, tokens[i:])
			if err != nil {
				return EditPath{}, err
			}
			start := len(result.tokens)
			if addr.Tokens != 0 {
				if n.HasMultiLeafKey() {
					result.tokens = append(result.tokens, ListKeyTokens(n, addr.Key)...)
				} else {
					result.tokens = append(result.tokens, addr.Key)
				}
			}
			if entryBounds && len(result.tokens)-start > 1 {
				result.entries = append(result.entries, editEntrySpan{start: start, end: len(result.tokens)})
			}
			i += addr.Tokens
			current = n
		case *InlineListNode:
			if i < len(tokens) && n.Get(tokens[i]) == nil {
				result.tokens = append(result.tokens, tokens[i])
				i++
			}
			current = n
		case *LeafNode:
			if i != len(tokens) {
				return EditPath{}, fmt.Errorf("path continues beyond scalar leaf: %s", name)
			}
			return result, nil
		case *FreeformNode, *MultiLeafNode, *BracketLeafListNode, *ValueOrArrayNode:
			// A member's value is the final component of its pending path.
			result.tokens = append(result.tokens, tokens[i:]...)
			return result, nil
		}
	}
	return result, nil
}

// ResolveEditPath validates one ownership address for repeated comparisons.
func ResolveEditPath(schema *Schema, path string) (EditPath, error) {
	return readEditPath(schema, path, true)
}

// HasPrefix compares complete canonical entry identities.
func (path EditPath) HasPrefix(prefix EditPath) bool {
	return path.hasPrefix(prefix.tokens)
}

// Equal reports whether two resolved addresses name the same node.
func (path EditPath) Equal(other EditPath) bool {
	return slices.Equal(path.tokens, other.tokens)
}
