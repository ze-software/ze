// Design: docs/architecture/core-design.md -- a word list, scanned rather than compiled
// Overview: ste.go -- the checker whose lists this scans for
//
// Four habits use LITERAL word lists. The script compiles each list into one
// alternation because separate expressions require approximately 100 searches
// for each prose unit. A repository review then takes approximately 100 seconds.
//
// The same implementation is slow in Go. Python `re` adds a first-character
// filter to its backtracking matcher. Go RE2 simulates an automaton over every
// byte. For one 90 KB document, the four lists took 3.9 seconds in Go. The full
// Python review took 0.13 seconds. These literals therefore use one direct scan.
//
// At each valid left boundary, candidates retain the alternation order.
// This order preserves parity. Longer entries come first. Equal-length entries
// retain dictionary declaration order through Python's stable sort.
package ste

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// literalList is one word list, ordered the way its alternation would be tried.
type literalList struct {
	entries []string
}

// newLiteralList orders a list longest-first, stably.
func newLiteralList(words []string) literalList {
	ordered := make([]string, len(words))
	copy(ordered, words)
	sort.SliceStable(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })
	return literalList{entries: ordered}
}

// match is one literal found in the text: where it starts, where it ends, and
// which entry it is.
type match struct {
	Start int
	End   int
	Entry string
}

// findAll answers nonoverlapping list-entry matches from left to right. Each
// match requires `(?<![\w-])` before it and `(?![\w-])` after it.
//
// If the right boundary fails, the scan advances one CHARACTER, as Python does.
// It cannot try a shorter entry at that position. Such an entry would prefix
// the longer entry, so its right boundary would also fall within the same word.
//
// Input length bounds the scan because each iteration advances at least one
// character.
func (l literalList) findAll(text string) []match {
	var out []match
	for offset := 0; offset < len(text); {
		_, size := utf8.DecodeRuneInString(text[offset:])
		if !notWordDashBefore(text, offset) {
			offset += size
			continue
		}

		found := l.at(text, offset)
		if found == "" {
			offset += size
			continue
		}
		end := offset + len(found)
		out = append(out, match{Start: offset, End: end, Entry: text[offset:end]})
		offset = end
	}
	return out
}

// has reports whether findAll would answer at least one match, and stops at the
// first one.
//
// The bound is the input, as in findAll.
func (l literalList) has(text string) bool {
	for offset := 0; offset < len(text); {
		_, size := utf8.DecodeRuneInString(text[offset:])
		if !notWordDashBefore(text, offset) {
			offset += size
			continue
		}
		if found := l.at(text, offset); found != "" {
			return true
		}
		offset += size
	}
	return false
}

// at answers the entry that matches at this offset with its right lookaround
// holding, or "" when none does.
func (l literalList) at(text string, offset int) string {
	for _, entry := range l.entries {
		end, ok := foldPrefix(text, offset, entry)
		if !ok {
			continue
		}
		if notWordDashAfter(text, end) {
			return text[offset:end]
		}
	}
	return ""
}

// foldPrefix reports whether want matches text at offset with Unicode simple
// case folding. It also answers the byte offset after the match.
//
// Both engines use simple folding. For example, `(?i)k` matches U+212A in Python
// and Go, although the spellings have different byte lengths.
func foldPrefix(text string, offset int, want string) (end int, ok bool) {
	index := offset
	for _, wanted := range want {
		if index >= len(text) {
			return 0, false
		}
		got, size := utf8.DecodeRuneInString(text[index:])
		if !foldEqual(got, wanted) {
			return 0, false
		}
		index += size
	}
	return index, true
}

// foldEqual reports whether two runes are equal under Unicode simple case
// folding. It is the comparison strings.EqualFold makes, one rune at a time.
//
// The bound is the fold orbit, which Unicode defines as a cycle: SimpleFold
// returns to its argument after at most a handful of steps.
func foldEqual(a, b rune) bool {
	if a == b {
		return true
	}
	if a > b {
		a, b = b, a
	}
	for r := unicode.SimpleFold(a); r != a; r = unicode.SimpleFold(r) {
		if r == b {
			return true
		}
		if r > b {
			return false
		}
	}
	return false
}

// lowerFields is `" ".join(text.lower().split())`, which is how a finding names
// the phrase it found: one space between the words, whatever the prose had.
func lowerFields(text string) string { return pyJoinFields(strings.ToLower(text)) }
