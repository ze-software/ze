// Design: docs/architecture/core-design.md -- the Python spellings a message carries
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// pyfmt.go holds the four Python behaviors this gate's OUTPUT depends on, kept
// rather than replaced: repr(), rune-counted slicing, sorted() over a set, and
// json.dumps(indent=2, sort_keys=True).
//
// They are here because a message is DATA to the parity proof. Go's %q escapes
// differently from repr(), Go's [:80] cuts an em dash in half where Python's
// cuts eighty CHARACTERS, and Go's json.Marshal writes one line where the
// envelope's only consumer reads two-space indentation. Substituting the Go
// spelling in any of the four turns a byte comparison into a verdict
// comparison.
//
// internal/le/rules has a repr for its own corpus, which is strings alone. This one
// takes a decoded JSON value, because the artifact parser reports whatever an
// author wrote in a field: null, a number, a list. They are not the same
// function and neither is a layer over the other.
package rfc

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// sortedKeys answers a set's members in sorted order, which is what Python's
// sorted() over a frozenset produces in every message that names one.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// firstRunes answers the first n CHARACTERS of s, which is what Python's [:n]
// takes. A byte slice cuts a multi-byte rune in half, and the RFC corpus is
// full of them: section marks, em dashes and smart quotes all sit inside the
// quotes this gate reports.
func firstRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}

// pyRepr renders one decoded JSON value the way Python's repr() does.
//
// The artifact parser interpolates `{value!r}` into eleven messages, and the
// value is whatever an author wrote: a string, a number, true, null, a list, an
// object. Each has its own Python spelling and none of them is Go's.
func pyRepr(value any) string {
	var tb textbuf.Buffer
	writeRepr(&tb, value)
	return tb.String()
}

func writeRepr(tb *textbuf.Buffer, value any) {
	switch typed := value.(type) {
	case nil:
		tb.Str("None")
	case bool:
		if typed {
			tb.Str("True")
			return
		}
		tb.Str("False")
	case string:
		writeStrRepr(tb, typed)
	case json.Number:
		// The literal the author wrote. Python reads `3` as an int and `3.0`
		// as a float, and reprs each as it was written; a Go float64 would
		// have lost the difference before this line.
		tb.Str(typed.String())
	case int:
		tb.Int(int64(typed))
	case []any:
		tb.Byte('[')
		for i, one := range typed {
			if i > 0 {
				tb.Str(", ")
			}
			writeRepr(tb, one)
		}
		tb.Byte(']')
	case []string:
		tb.Byte('[')
		for i, one := range typed {
			if i > 0 {
				tb.Str(", ")
			}
			writeStrRepr(tb, one)
		}
		tb.Byte(']')
	case map[string]any:
		// Sorted, where Python renders insertion order. The two agree for
		// every artifact this gate has ever reported, because a dict reaches a
		// message only as the whole document and no message names one today.
		// Sorting is the deterministic answer available to a Go map, and a
		// message that varies run to run is worse than one that orders
		// differently from the script.
		tb.Byte('{')
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for i, key := range keys {
			if i > 0 {
				tb.Str(", ")
			}
			writeStrRepr(tb, key)
			tb.Str(": ")
			writeRepr(tb, typed[key])
		}
		tb.Byte('}')
	default:
		tb.Str(pyTypeName(value))
	}
}

// writeStrRepr is CPython's str repr: single quotes unless the string holds one
// and no double quote, backslash escapes for the quote, the backslash and the
// three whitespace controls, and a hex escape for anything unprintable.
func writeStrRepr(tb *textbuf.Buffer, s string) {
	quote := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	tb.Byte(quote)
	for _, r := range s {
		switch {
		case r == rune(quote) || r == '\\':
			tb.Byte('\\').Byte(byte(r))
		case r == '\t':
			tb.Str(`\t`)
		case r == '\n':
			tb.Str(`\n`)
		case r == '\r':
			tb.Str(`\r`)
		case r < 0x20 || r == 0x7f:
			writeHexEscape(tb, r)
		case r < 0x80:
			tb.Byte(byte(r))
		case unicode.IsPrint(r):
			tb.WriteRune(r) //nolint:errcheck // a buffer write cannot fail
		default:
			writeHexEscape(tb, r)
		}
	}
	tb.Byte(quote)
}

const hexDigits = "0123456789abcdef"

// writeHexEscape writes the \xNN, \uNNNN or \UNNNNNNNN form Python picks by the
// rune's width.
func writeHexEscape(tb *textbuf.Buffer, r rune) {
	width := 2
	prefix := `\x`
	switch {
	case r > 0xffff:
		width, prefix = 8, `\U`
	case r > 0xff:
		width, prefix = 4, `\u`
	}
	tb.Str(prefix)
	for shift := (width - 1) * 4; shift >= 0; shift -= 4 {
		tb.Byte(hexDigits[(r>>shift)&0xf])
	}
}

// pyTypeName answers the Python type name of a decoded JSON value, which is
// what `type(x).__name__` renders in the two messages that name a type rather
// than a value.
func pyTypeName(value any) string {
	switch typed := value.(type) {
	case nil:
		return "NoneType"
	case bool:
		return "bool"
	case string:
		return "str"
	case []any:
		return "list"
	case map[string]any:
		return "dict"
	case json.Number:
		if _, err := typed.Int64(); err == nil {
			return "int"
		}
		return "float"
	}
	return "object"
}

// pyDump renders a value the way json.dumps(indent=2, sort_keys=True) does.
//
// The envelope's only consumer reads this shape, so it is the command's default
// rendering. `| json` still goes to the engine: this is a second reading of the
// same payload, never a substitute for it (leroot.Prose).
func pyDump(value any) string {
	var tb textbuf.Buffer
	writeDump(&tb, value, 0)
	return tb.String()
}

func writeDump(tb *textbuf.Buffer, value any, depth int) {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			tb.Str("{}")
			return
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		tb.Str("{\n")
		for i, key := range keys {
			if i > 0 {
				tb.Str(",\n")
			}
			writeIndent(tb, depth+1)
			writeJSONString(tb, key)
			tb.Str(": ")
			writeDump(tb, typed[key], depth+1)
		}
		tb.Byte('\n')
		writeIndent(tb, depth)
		tb.Byte('}')
	case map[string]int:
		converted := make(map[string]any, len(typed))
		for key, count := range typed {
			converted[key] = count
		}
		writeDump(tb, converted, depth)
	case []string:
		if len(typed) == 0 {
			tb.Str("[]")
			return
		}
		tb.Str("[\n")
		for i, one := range typed {
			if i > 0 {
				tb.Str(",\n")
			}
			writeIndent(tb, depth+1)
			writeJSONString(tb, one)
		}
		tb.Byte('\n')
		writeIndent(tb, depth)
		tb.Byte(']')
	case []any:
		// The shape a decoded array comes back as. The audit writer renders a
		// document it read rather than one it built, so every JSON value has
		// to survive the round trip.
		if len(typed) == 0 {
			tb.Str("[]")
			return
		}
		tb.Str("[\n")
		for i, one := range typed {
			if i > 0 {
				tb.Str(",\n")
			}
			writeIndent(tb, depth+1)
			writeDump(tb, one, depth+1)
		}
		tb.Byte('\n')
		writeIndent(tb, depth)
		tb.Byte(']')
	case json.Number:
		// The literal an author wrote. Python's json writes an int as an int
		// and a float as a float, and a decoded float64 has already lost which
		// one it was.
		tb.Str(typed.String())
	case string:
		writeJSONString(tb, typed)
	case int:
		tb.Int(int64(typed))
	case bool:
		tb.Bool(typed)
	case nil:
		tb.Str("null")
	default:
		tb.Str("null")
	}
}

func writeIndent(tb *textbuf.Buffer, depth int) { tb.Repeat("  ", depth) }

// writeJSONString escapes the way Python's json does with ensure_ascii on: the
// two mandatory escapes, the five short forms, and \uNNNN for everything else
// outside printable ASCII.
func writeJSONString(tb *textbuf.Buffer, s string) {
	tb.Byte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			tb.Byte('\\').Byte(byte(r))
		case r == '\n':
			tb.Str(`\n`)
		case r == '\r':
			tb.Str(`\r`)
		case r == '\t':
			tb.Str(`\t`)
		case r == '\b':
			tb.Str(`\b`)
		case r == '\f':
			tb.Str(`\f`)
		case r < 0x20:
			tb.Str(`\u00`)
			tb.Byte(hexDigits[(r>>4)&0xf]).Byte(hexDigits[r&0xf])
		case r < 0x7f:
			tb.Byte(byte(r))
		case r > 0xffff:
			// A surrogate pair, which is how Python's json writes an
			// astral-plane character under ensure_ascii.
			r -= 0x10000
			writeUnicodeEscape(tb, 0xd800+((r>>10)&0x3ff))
			writeUnicodeEscape(tb, 0xdc00+(r&0x3ff))
		default:
			writeUnicodeEscape(tb, r)
		}
	}
	tb.Byte('"')
}

func writeUnicodeEscape(tb *textbuf.Buffer, r rune) {
	tb.Str(`\u`)
	for shift := 12; shift >= 0; shift -= 4 {
		tb.Byte(hexDigits[(r>>shift)&0xf])
	}
}

// pyIsSpace matches Python's str.isspace, which strip() and lstrip() consume.
//
// It is unicode.IsSpace plus the four information separators U+001C to U+001F.
// Python counts those as whitespace and Go's White_Space property does not.
// pyLStrip walks back over a doc comment, so a Go answer that trimmed a
// different set would resolve a tag to a different unit from the module's.
func pyIsSpace(r rune) bool { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) }

// pyLStrip is Python's str.lstrip() with no argument.
func pyLStrip(s string) string { return strings.TrimLeftFunc(s, pyIsSpace) }
