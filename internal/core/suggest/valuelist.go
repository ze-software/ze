// Design: docs/architecture/config/yang-config-design.md -- Value refusals
// Related: suggest.go -- the closest-match helper a refusal's hint uses

package suggest

import (
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// helpGap is the space between the longest value name and every help text.
const helpGap = 2

// Value is one accepted value a refusal lists: its name, and the one-line
// help the model declares for it ("" when it declares none).
type Value struct {
	Name string
	Help string
}

// ValueLines writes one line per value, in the order given: indent, the name,
// then, when the value has a help, enough spaces to start every help two
// columns after the longest name. A value with no help is its name alone,
// with no trailing space. Lines are joined by a newline, with none after the
// last. An empty list answers "".
//
// Both the config refusal (internal/component/config/yang) and the command
// argument refusal (internal/component/command) write their lists through
// this one function, so the two cannot disagree on the layout.
func ValueLines(values []Value, indent string) string {
	width := 0
	for _, value := range values {
		width = max(width, utf8.RuneCountInString(value.Name))
	}
	var b textbuf.Buffer
	for i, value := range values {
		if i > 0 {
			b.Byte('\n')
		}
		b.Str(indent)
		if value.Help == "" {
			b.Str(value.Name)
			continue
		}
		b.PadRight(value.Name, width+helpGap).Str(value.Help)
	}
	return b.String()
}

// Alternation writes the names as the placeholder an operator replaces:
// "<a|b|c>", in the order given. A list of one value answers "<a>".
func Alternation(values []Value) string {
	var b textbuf.Buffer
	b.Byte('<')
	for i, value := range values {
		if i > 0 {
			b.Byte('|')
		}
		b.Str(value.Name)
	}
	return b.Byte('>').String()
}

// Names answers the names of values, in the order given.
func Names(values []Value) []string {
	names := make([]string, 0, len(values))
	for _, value := range values {
		names = append(names, value.Name)
	}
	return names
}
