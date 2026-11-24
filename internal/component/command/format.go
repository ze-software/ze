// Design: docs/architecture/api/commands.md — output formatting
// Related: pipe.go — YAML pipe operator uses these

package command

import (
	"encoding/json"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// formatNumber displays integers without decimal points.
// Integer JSON tokens stay exact; fractional and exponent forms retain the
// established human-readable float formatting.
func formatNumber(v any) any {
	if number, ok := v.(json.Number); ok {
		if !strings.ContainsAny(number.String(), ".eE") {
			return number
		}
		parsed, err := number.Float64()
		if err != nil {
			return number
		}
		v = parsed
	}
	if n, ok := v.(float64); ok {
		if n == float64(int64(n)) {
			return int64(n)
		}
	}
	return v
}

// RenderYAML formats a parsed JSON value as valid YAML.
func RenderYAML(data any) string {
	var b textbuf.Buffer
	writeValue(&b, data, "")
	return b.String()
}

// writeValue recursively writes a JSON value as valid YAML with indentation.
func writeValue(b *textbuf.Buffer, v any, indent string) {
	switch val := v.(type) {
	case map[string]any:
		if len(val) == 0 {
			b.Str(indent).Str("{}\n")
			return
		}
		writeMap(b, val, indent)
	case []any:
		if len(val) == 0 {
			b.Str(indent).Str("[]\n")
			return
		}
		var tb textbuf.Buffer
		deeper := tb.Str(indent).Str("  ").String()
		for _, item := range val {
			switch elem := item.(type) {
			case map[string]any:
				if len(elem) == 0 {
					b.Str(indent).Str("- {}\n")
					continue
				}
				writeMapItem(b, elem, indent)
			case []any:
				if len(elem) == 0 {
					b.Str(indent).Str("- []\n")
					continue
				}
				b.Str(indent).Str("-\n")
				writeValue(b, elem, deeper)
			default:
				b.Str(indent).Str("- ")
				writeScalar(b, item)
				b.Byte('\n')
			}
		}
	case nil:
		b.Str(indent).Str("null\n")
	case bool:
		b.Str(indent).Bool(val).Byte('\n')
	case string:
		b.Str(indent)
		writeYAMLString(b, val)
		b.Byte('\n')
	case float64, json.Number, int, int64, uint64:
		b.Str(indent)
		writeScalar(b, val)
		b.Byte('\n')
	}
}

func writeScalar(b *textbuf.Buffer, v any) {
	v = formatNumber(v)
	switch s := v.(type) {
	case string:
		writeYAMLString(b, s)
	case int:
		b.Int(int64(s))
	case int64:
		b.Int(s)
	case uint64:
		b.Uint(s)
	case json.Number:
		b.Str(s.String())
	case float64:
		b.Float(s, -1)
	case bool:
		b.Bool(s)
	default:
		b.Str("null")
	}
}

// writeMap writes a map with sorted keys at the given indentation.
func writeMap(b *textbuf.Buffer, m map[string]any, indent string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, key := range keys {
		writeKeyValue(b, key, m[key], indent, indent)
	}
}

// writeKeyValue writes a single key-value pair with proper YAML formatting.
//
// prefix opens the key's OWN line and indent opens every line under it. The two
// differ for the first key of a sequence item, whose prefix carries the `- `
// that starts the item. They MUST NOT be merged: a `- ` inherited by a child
// line starts a new sequence entry there, so a row whose first key holds a list
// or a map renders as several rows and no YAML reader accepts it.
func writeKeyValue(b *textbuf.Buffer, key string, value any, prefix, indent string) {
	var tb textbuf.Buffer
	deeper := tb.Str(indent).Str("  ").String()
	b.Str(prefix)
	writeYAMLString(b, key)
	switch child := value.(type) {
	case map[string]any:
		if len(child) == 0 {
			b.Str(": {}\n")
			return
		}
		b.Str(":\n")
		writeMap(b, child, deeper)
	case []any:
		if len(child) == 0 {
			b.Str(": []\n")
			return
		}
		b.Str(":\n")
		writeValue(b, child, deeper)
	case nil:
		b.Str(": null\n")
	case bool:
		b.Str(": ").Bool(child).Byte('\n')
	case string:
		b.Str(": ")
		writeYAMLString(b, child)
		b.Byte('\n')
	case float64, json.Number, int, int64, uint64:
		b.Str(": ")
		writeScalar(b, child)
		b.Byte('\n')
	default:
		b.Str(": ")
		writeScalar(b, child)
		b.Byte('\n')
	}
}

// writeMapItem writes a map as a YAML sequence item (first key on the "- " line).
func writeMapItem(b *textbuf.Buffer, m map[string]any, indent string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	var tb textbuf.Buffer
	dashIndent := tb.Str(indent).Str("- ").String()
	contIndent := tb.Reset().Str(indent).Str("  ").String()
	for i, key := range keys {
		if i == 0 {
			writeKeyValue(b, key, m[key], dashIndent, contIndent)
			continue
		}
		writeKeyValue(b, key, m[key], contIndent, contIndent)
	}
}

const yamlHexDigits = "0123456789abcdef"

// writeYAMLString writes s as a plain YAML scalar when unambiguous under YAML
// 1.2 (and YAML 1.1 parsers), or as a double-quoted escaped string otherwise.
func writeYAMLString(b *textbuf.Buffer, s string) {
	if !yamlNeedsQuoting(s) {
		b.Str(s)
		return
	}
	writeYAMLQuoted(b, s)
}

// yamlNeedsQuoting reports whether s cannot be emitted as a plain YAML scalar
// without risking a type change (bool, null, number, timestamp) or syntax
// ambiguity under YAML 1.2 or YAML 1.1 parsers.
func yamlNeedsQuoting(s string) bool {
	if s == "" {
		return true
	}
	first, _ := utf8.DecodeRuneInString(s)
	if unicode.IsSpace(first) {
		return true
	}
	last, _ := utf8.DecodeLastRuneInString(s)
	if unicode.IsSpace(last) {
		return true
	}
	switch s[0] {
	case '-', '+', '.', '~', '=':
		return true
	}
	if isReservedYAMLLiteral(s) {
		return true
	}
	if hasYAMLUnsafeChar(s) {
		return true
	}
	return looksLikeYAMLNumberOrDate(s)
}

// isReservedYAMLLiteral reports whether s matches a YAML 1.2 or YAML 1.1
// non-string keyword (booleans, null, or the merge key).
func isReservedYAMLLiteral(s string) bool {
	if s == "<<" {
		return true
	}
	if len(s) > 5 {
		return false
	}
	switch {
	case strings.EqualFold(s, "true"):
		return true
	case strings.EqualFold(s, "false"):
		return true
	case strings.EqualFold(s, "null"):
		return true
	case strings.EqualFold(s, "yes"):
		return true
	case strings.EqualFold(s, "no"):
		return true
	case strings.EqualFold(s, "on"):
		return true
	case strings.EqualFold(s, "off"):
		return true
	case strings.EqualFold(s, "y"):
		return true
	case strings.EqualFold(s, "n"):
		return true
	}
	return false
}

// hasYAMLUnsafeChar reports whether s contains a YAML syntax indicator,
// backslash, control character, or non-printable rune.
func hasYAMLUnsafeChar(s string) bool {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if r < 0x20 {
			return true
		}
		if r == 0x7f {
			return true
		}
		if r >= 0x80 && r <= 0x9f {
			return true
		}
		if !unicode.IsPrint(r) {
			return true
		}
		switch r {
		case ':', '#', '{', '}', '[', ']', ',', '&', '*', '!', '|', '>', '\'', '"', '%', '@', '`', '?', '\\':
			return true
		}
	}
	return false
}

// looksLikeYAMLNumberOrDate reports whether a digit-leading string would be
// resolved as an integer, float, or ISO-8601 timestamp by a YAML parser.
// Multi-dot forms such as IPv4 addresses (192.0.2.1) and semver (1.2.3) return
// false so they remain unquoted plain scalars.
func looksLikeYAMLNumberOrDate(s string) bool {
	if !isASCIIDigit(s[0]) {
		return false
	}
	if looksLikeYAMLDatePrefix(s) {
		return true
	}
	if looksLikePrefixedBaseInt(s) {
		return true
	}
	return looksLikeDecimalNumber(s)
}

func looksLikeYAMLDatePrefix(s string) bool {
	if len(s) < 5 {
		return false
	}
	if s[4] != '-' {
		return false
	}
	for i := range 4 {
		if !isASCIIDigit(s[i]) {
			return false
		}
	}
	return true
}

func looksLikePrefixedBaseInt(s string) bool {
	if len(s) < 3 {
		return false
	}
	if s[0] != '0' {
		return false
	}
	switch s[1] {
	case 'x', 'X':
		return allDigitsOrUnderscore(s[2:], isHexDigit)
	case 'o', 'O':
		return allDigitsOrUnderscore(s[2:], isOctalDigit)
	case 'b', 'B':
		return allDigitsOrUnderscore(s[2:], isBinaryDigit)
	}
	return false
}

func allDigitsOrUnderscore(s string, match func(byte) bool) bool {
	hasDigit := false
	for i := range len(s) {
		c := s[i]
		if c == '_' {
			continue
		}
		if !match(c) {
			return false
		}
		hasDigit = true
	}
	return hasDigit
}

func skipDecimalDigits(s string, i int) int {
	for i < len(s) && (isASCIIDigit(s[i]) || s[i] == '_') {
		i++
	}
	return i
}

func looksLikeDecimalNumber(s string) bool {
	i := skipDecimalDigits(s, 0)
	if i == len(s) {
		return true
	}
	if s[i] == '.' {
		i = skipDecimalDigits(s, i+1)
		if i == len(s) {
			return true
		}
	}
	if s[i] != 'e' && s[i] != 'E' {
		return false
	}
	i++
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	return allDigitsOrUnderscore(s[i:], isASCIIDigit)
}

func isASCIIDigit(c byte) bool  { return c >= '0' && c <= '9' }
func isOctalDigit(c byte) bool  { return c >= '0' && c <= '7' }
func isBinaryDigit(c byte) bool { return c == '0' || c == '1' }
func isHexDigit(c byte) bool {
	if isASCIIDigit(c) {
		return true
	}
	if c >= 'a' && c <= 'f' {
		return true
	}
	return c >= 'A' && c <= 'F'
}

// writeYAMLQuoted writes s as a YAML 1.2 double-quoted string with escapes.
func writeYAMLQuoted(b *textbuf.Buffer, s string) {
	b.Byte('"')
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		raw := s[i : i+size]
		i += size
		switch r {
		case '\\':
			b.Str(`\\`)
		case '"':
			b.Str(`\"`)
		case '\x00':
			b.Str(`\0`)
		case '\a':
			b.Str(`\a`)
		case '\b':
			b.Str(`\b`)
		case '\t':
			b.Str(`\t`)
		case '\n':
			b.Str(`\n`)
		case '\v':
			b.Str(`\v`)
		case '\f':
			b.Str(`\f`)
		case '\r':
			b.Str(`\r`)
		case '\x1b':
			b.Str(`\e`)
		case '\u0085':
			b.Str(`\N`)
		case '\u00a0':
			b.Str(`\_`)
		case '\u2028':
			b.Str(`\L`)
		case '\u2029':
			b.Str(`\P`)
		default:
			writeYAMLQuotedRune(b, r, raw)
		}
	}
	b.Byte('"')
}

func isYAMLControlByte(r rune) bool {
	if r < 0x20 {
		return true
	}
	if r == 0x7f {
		return true
	}
	return r >= 0x80 && r <= 0x9f
}

func writeYAMLQuotedRune(b *textbuf.Buffer, r rune, raw string) {
	if isYAMLControlByte(r) {
		b.Str(`\x`)
		b.Byte(yamlHexDigits[(r>>4)&0xf])
		b.Byte(yamlHexDigits[r&0xf])
		return
	}
	if !unicode.IsPrint(r) {
		if r <= 0xffff {
			b.Str(`\u`)
			b.Byte(yamlHexDigits[(r>>12)&0xf])
			b.Byte(yamlHexDigits[(r>>8)&0xf])
			b.Byte(yamlHexDigits[(r>>4)&0xf])
			b.Byte(yamlHexDigits[r&0xf])
			return
		}
		b.Str(`\U`)
		for shift := 28; shift >= 0; shift -= 4 {
			b.Byte(yamlHexDigits[(r>>shift)&0xf])
		}
		return
	}
	b.Str(raw)
}
