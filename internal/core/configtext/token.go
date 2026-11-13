// Design: docs/architecture/config/syntax.md -- shared word boundaries and quoting.

package configtext

import "github.com/ze-software/ze/internal/core/textbuf"

// Pipe is the bare command pipe operator; a quoted pipe is a literal value.
const Pipe = "|"

// WordEnd returns the first bare word's end offset. A bracket after a colon
// may hold an address, as in redirect:[::1]:100.
func WordEnd(text string) int {
	i := 0
	for i < len(text) {
		c := text[i]
		if c == '[' && i > 0 && text[i-1] == ':' {
			if end, closed := addressEnd(text, i); closed {
				i = end
				continue
			}
		}
		if endsWord(c) {
			break
		}
		i++
	}
	return i
}

func addressEnd(text string, start int) (int, bool) {
	for i := start + 1; i < len(text); i++ {
		c := text[i]
		if c == ']' {
			return i + 1, i > start+1
		}
		if endsWord(c) {
			return 0, false
		}
	}
	return 0, false
}

func endsWord(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
		c == '{' || c == '}' || c == '[' || c == ']' || c == ';' ||
		c == '"' || c == '\'' || c == '#'
}

// bareWord reports whether text is one bare token. A leading parenthesis is
// structural; parentheses inside a word remain data.
func bareWord(text string) bool {
	return len(text) > 0 && text[0] != '(' && text[0] != ')' && WordEnd(text) == len(text)
}

// QuoteToken preserves every byte, including invalid UTF-8, with config quoting.
func QuoteToken(text string) string {
	if bareWord(text) {
		return text
	}
	var b textbuf.Buffer
	b.Byte('"')
	for i := range len(text) {
		c := text[i]
		switch c {
		case '"':
			b.Str(`\"`)
		case '\\':
			b.Str(`\\`)
		case '\n':
			b.Str(`\n`)
		case '\r':
			b.Str(`\r`)
		case '\t':
			b.Str(`\t`)
		default:
			b.Byte(c)
		}
	}
	b.Byte('"')
	return b.String()
}

// QuoteCommandWord additionally protects a literal pipe from command grammar.
func QuoteCommandWord(text string) string {
	if text == Pipe {
		return `"|"`
	}
	return QuoteToken(text)
}
