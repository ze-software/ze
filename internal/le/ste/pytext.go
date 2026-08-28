// Design: docs/architecture/core-design.md -- Python text semantics, stated in Go
// Overview: ste.go -- the checker these helpers serve
//
// ste_check.py processes TEXT. A port must preserve the string-library
// differences that change findings:
//
//   - Python whitespace classes and split functions use Unicode. Go regexp `\s`
//     uses five ASCII bytes. strings.Fields uses a similar but different set.
//     Python also treats U+001C through U+001F as whitespace.
//
//   - Python `\w` and `\b` use Unicode. Go uses `[0-9A-Za-z_]`.
//
//   - Python `\d` includes all Unicode decimal digits. Go uses `[0-9]`.
//
//   - Python lengths and slices count CHARACTERS. Go counts BYTES. A byte slice
//     can split an em dash and create mojibake.
//
//   - Go RE2 lacks lookaround, which six patterns require.
//
// DEFAULT_GLOBS currently select 8120 files with no non-ASCII whitespace,
// carriage returns, or form feeds (measured 2026-08-26). But `le ste review`
// also reads stdin and named files. The tool must preserve these semantics
// instead of relying on the current tree.
package ste

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The character classes that replace a Python escape whose meaning Go spells
// differently. A pattern names them by placeholder and classes.Replace puts
// them in, so the mapping is stated once and every pattern reads it.
const (
	// classSpace is Python's `\s` for a str pattern: the White_Space property
	// plus the four information separators U+001C to U+001F.
	classSpace = `[\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`
	// classSpaceBody is the same set with no brackets, for a pattern that puts
	// whitespace INSIDE a wider class. A class cannot nest one.
	classSpaceBody = `\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`
	// classNonSpace is Python's `\S`, the same set negated.
	classNonSpace = `[^\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`
	// classDigit is Python's `\d`, every Unicode decimal digit.
	classDigit = `\p{Nd}`
	// classNonWord is Python `\W`: neither a letter, a number, nor underscore.
	// Python `\w` is Py_UNICODE_ISALNUM plus `_`, which includes letter
	// categories and Nd, Nl, and No.
	classNonWord = `[^\p{L}\p{Nd}\p{Nl}\p{No}_]`
)

// classes inserts the classes above into a pattern. Placeholders avoid string
// concatenation that the repository check rejects. A one-token class also makes
// the pattern shape visible.
//
// The longer placeholder comes FIRST. strings.Replacer takes the earliest
// matching pair. If "{SP}" came first, it would consume the prefix of "{SPB}"
// and leave "B}" in the pattern.
var classes = strings.NewReplacer(
	"{SPB}", classSpaceBody,
	"{SP}", classSpace,
	"{NS}", classNonSpace,
	"{D}", classDigit,
	"{NW}", classNonWord,
)

// mustPattern compiles one pattern after the class placeholders are filled in.
// It panics on a pattern this package spelled wrong, which is a Ze defect at
// init and reachable from nothing an operator types.
func mustPattern(pattern string) *regexp.Regexp {
	return regexp.MustCompile(classes.Replace(pattern))
}

// pySpace reports whether r is whitespace to Python.
//
// unicode.IsSpace answers the White_Space property, which is Python's set minus
// the four information separators. CPython's Py_UNICODE_ISSPACE counts those
// four, so they are added here rather than left as a silent difference.
func pySpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

// pyWord reports whether r is a word character to Python: what `\w` matches and
// what `\b` measures its transition against.
func pyWord(r rune) bool {
	if r == '_' {
		return true
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r) ||
		unicode.Is(unicode.Nl, r) || unicode.Is(unicode.No, r)
}

// pyFields splits on runs of whitespace and drops the empty pieces, which is
// what `str.split()` with no argument does.
func pyFields(s string) []string {
	return strings.FieldsFunc(s, pySpace)
}

// pyStrip removes leading and trailing whitespace, which is what `str.strip()`
// with no argument does.
func pyStrip(s string) string { return strings.TrimFunc(s, pySpace) }

// pyRStrip removes trailing whitespace, which is what `str.rstrip()` does.
func pyRStrip(s string) string { return strings.TrimRightFunc(s, pySpace) }

// pyJoinFields is `" ".join(s.split())`: one space between the words, and no
// leading, trailing or repeated whitespace left.
func pyJoinFields(s string) string { return strings.Join(pyFields(s), " ") }

// pySplitLines is `str.splitlines()`: the terminator is removed, and eight
// characters beyond `\n` end a line. A Go source comment holding a form feed
// and a Windows document both reach this checker, and Go's own line split sees
// neither.
func pySplitLines(s string) []string {
	if s == "" {
		return nil
	}

	// The bound is the input: each iteration consumes at least one byte.
	var lines []string
	start := 0
	for index := 0; index < len(s); {
		r, size := utf8.DecodeRuneInString(s[index:])
		width := lineBreakWidth(s, index, r, size)
		if width == 0 {
			index += size
			continue
		}
		lines = append(lines, s[start:index])
		index += width
		start = index
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// lineBreakWidth answers the byte width of the line terminator at index, or 0
// when the rune there does not end a line. A carriage return followed by a line
// feed is ONE terminator.
func lineBreakWidth(s string, index int, r rune, size int) int {
	switch r {
	case '\r':
		if strings.HasPrefix(s[index+size:], "\n") {
			return size + 1
		}
		return size
	case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return size
	default:
		return 0
	}
}

// runeHead answers the first n CHARACTERS of s, which is what Python's `s[:n]`
// answers. A byte slice cuts an em dash in half, and this corpus is full of
// them.
func runeHead(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for index := range s {
		if count == n {
			return s[:index]
		}
		count++
	}
	return s
}

// ─── Lookaround, hand-written ───────────────────────────────────────────────

// isWordAt reports whether the rune ENDING at byte offset end is a word
// character. An offset at or before the start of the text is not.
func isWordBefore(text string, offset int) bool {
	if offset <= 0 || offset > len(text) {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(text[:offset])
	return pyWord(r)
}

// isWordAfter reports whether the rune STARTING at byte offset start is a word
// character. An offset at or past the end of the text is not.
func isWordAfter(text string, offset int) bool {
	if offset < 0 || offset >= len(text) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(text[offset:])
	return pyWord(r)
}

// wordBoundary reports whether `\b` holds at byte offset: one side is a word
// character and the other is not.
func wordBoundary(text string, offset int) bool {
	return isWordBefore(text, offset) != isWordAfter(text, offset)
}

// notWordDashBefore reports whether `(?<![\w-])` holds at offset.
func notWordDashBefore(text string, offset int) bool {
	if offset <= 0 || offset > len(text) {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(text[:offset])
	return !pyWord(r) && r != '-'
}

// notWordDashAfter reports whether `(?![\w-])` holds at offset.
func notWordDashAfter(text string, offset int) bool {
	if offset < 0 || offset >= len(text) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(text[offset:])
	return !pyWord(r) && r != '-'
}

// edge says whether the END of a pattern carries the Python `\b` its author
// wrote there.
//
// Only the trailing end varies. Every pattern this package runs through
// findBounded opens with `\b`, because each begins with a letter, so the
// leading boundary is applied unconditionally. The word LISTS use a different
// lookaround, `(?<![\w-])`, and wordlist.go applies it directly.
type edge uint8

const (
	// edgeNone is an end the Python pattern left unguarded.
	edgeNone edge = iota
	// edgeWord is `\b`, which measures a TRANSITION rather than one side. The
	// difference is live: `[A-Za-z%]+\b` after a `%` needs the next character
	// to BE a word character, and "50 % of" is therefore not a measurement.
	edgeWord
)

// holdsAfter reports whether this edge is satisfied at the end of a match.
func (e edge) holdsAfter(text string, offset int) bool {
	if e == edgeWord {
		return wordBoundary(text, offset)
	}
	return true
}

// findBounded runs re over text like Python finditer with a leading `\b` and
// the trailing guard named by tail. It answers submatch index sets.
//
// It REJECTS a match with invalid edges and resumes one CHARACTER after the
// start, not after the end. Python fails at that position and advances one
// character. If resumption starts after the end, the scan skips a contained
// shorter match.
func findBounded(re *regexp.Regexp, text string, tail edge) [][]int {
	var out [][]int
	eachBounded(re, text, tail, func(loc []int) bool {
		out = append(out, loc)
		return true
	})
	return out
}

// hasBounded reports whether any match holds its edges. It stops at the first
// one, which is the `.search()` form.
func hasBounded(re *regexp.Regexp, text string, tail edge) bool {
	found := false
	eachBounded(re, text, tail, func([]int) bool {
		found = true
		return false
	})
	return found
}

// eachBounded is the scan both forms share. It calls visit with each accepted
// match and stops when visit answers false.
//
// The bound is the input: every iteration either consumes a match of at least
// one byte or advances the offset past one character.
func eachBounded(re *regexp.Regexp, text string, tail edge, visit func(loc []int) bool) {
	for offset := 0; offset <= len(text); {
		loc := re.FindStringSubmatchIndex(text[offset:])
		if loc == nil {
			return
		}
		start, end := offset+loc[0], offset+loc[1]

		if !wordBoundary(text, start) || !tail.holdsAfter(text, end) {
			_, size := utf8.DecodeRuneInString(text[start:])
			if size < 1 {
				size = 1
			}
			offset = start + size
			continue
		}

		shifted := make([]int, len(loc))
		for i, value := range loc {
			if value < 0 {
				shifted[i] = -1
				continue
			}
			shifted[i] = offset + value
		}
		if !visit(shifted) {
			return
		}

		if end == start {
			offset = start + 1
			continue
		}
		offset = end
	}
}
