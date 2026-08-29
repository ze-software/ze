// Design: docs/architecture/core-design.md -- stable quoted diagnostic values.
//
// Package textrepr renders quoted strings and lists for diagnostics.
package textrepr

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Quote renders one printable ASCII string with a stable quote choice.
func Quote(value string) string {
	quote := byte('\'')
	if strings.Contains(value, "'") && !strings.Contains(value, `"`) {
		quote = '"'
	}
	var text textbuf.Buffer
	text.Byte(quote)
	for index := range len(value) {
		current := value[index]
		if current == '\\' || current == quote {
			text.Byte('\\')
		}
		text.Byte(current)
	}
	return text.Byte(quote).String()
}

// List renders a string slice as a bracketed, comma-separated quoted list.
func List(items []string) string {
	var text textbuf.Buffer
	text.Byte('[')
	for index, item := range items {
		if index > 0 {
			text.Str(", ")
		}
		text.Str(Quote(item))
	}
	return text.Byte(']').String()
}
