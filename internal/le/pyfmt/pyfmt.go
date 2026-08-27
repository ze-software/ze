// Design: docs/architecture/core-design.md -- Python's own renderings, for the ports
//
// Package pyfmt renders a value the way Python's repr and str render it.
//
// Several ported gates insert Python values directly into messages.
// Examples include `expected one of 'framework'` and `file not found under ['internal/component/bgp']`.
// In these messages, brackets and quotes are data rather than presentation.
// This package preserves exact output while the Python and Go implementations both exist.
//
// It is stated once because three tool packages need it and three hand-written
// copies is where they begin to disagree about a quote.
package pyfmt

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Repr renders one string like Python repr.
// It uses single quotes unless the value contains only a single quote.
// It escapes backslashes and the selected quote.
//
// Python also escapes nonprintable and non-ASCII characters.
// This renderer only receives paths, category names, and build tags.
// Escapes reports whether a value needs unsupported escaping, so this package omits that table.
func Repr(value string) string {
	quote := byte('\'')
	if strings.Contains(value, "'") && !strings.Contains(value, `"`) {
		quote = '"'
	}

	var tb textbuf.Buffer
	tb.Byte(quote)
	for i := range len(value) {
		c := value[i]
		if c == '\\' || c == quote {
			tb.Byte('\\')
		}
		tb.Byte(c)
	}
	return tb.Byte(quote).String()
}

// Escapes reports whether Python's repr would escape a character of the value,
// which is the case Repr does not reproduce.
func Escapes(value string) bool {
	for i := range len(value) {
		if value[i] < 0x20 || value[i] > 0x7e {
			return true
		}
	}
	return false
}

// List renders a string slice like a Python list.
// It wraps Repr values in brackets and separates them with a comma and space.
func List(items []string) string {
	var tb textbuf.Buffer
	tb.Byte('[')
	for i, item := range items {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Str(Repr(item))
	}
	return tb.Byte(']').String()
}
