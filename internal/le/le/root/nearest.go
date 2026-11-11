// Design: docs/architecture/core-design.md -- the answer to an unknown command
// Related: dispatch.go -- dispatchLine and helpAsked refuse through refuseUnknown
//
// An unknown command answers in two lines: what was not found, with the
// nearest registered names, and the command that lists every one. The
// candidates are read from the registry (Commands, Hidden), so no list of
// names is written here, and a command that registers is a candidate the
// moment it exists.

package leroot

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// nearestListedMax bounds how many near names the refusal lists. A fourth is
// counted, never listed, so a short typo cannot answer a list as long as the
// usage page this refusal replaced.
const nearestListedMax = 3

// nearestDistanceMax is the largest edit distance a spelling may be from a
// command's word and still be near it. The distance must also be under half
// the typed word's length, so a two-letter word is never near another
// two-letter word that shares only one letter.
const nearestDistanceMax = 2

// refuseUnknown writes the refusal of a line whose words name no registered
// command and no namespace, and answers the usage code: nothing ran. typed is
// the line as the reader typed it after the program (and after the help word
// for a help question), so a run and a help question answer the same lines.
// It MUST be called only with a line whose own words are not empty.
func refuseUnknown(program string, typed []string) int {
	own, _ := splitChain(typed)
	own = own[:min(len(own), CommandWordsMax)]

	var tb textbuf.Buffer
	tb.Str(unknownLine(program, own[0], nearestNames(own, nearestCandidates()))).Byte('\n').
		Str("List every command: ").Str(program).Str(" help").Byte('\n').StdErr() //nolint:errcheck // CLI output
	return 2
}

// unknownLine answers the first line of the refusal: the word le could not
// resolve, then the near names, each spelled after the program so the reader
// can copy it, at most nearestListedMax of them and a count of the rest.
// With no near name the line stops after the word.
func unknownLine(program, word string, near []string) string {
	var tb textbuf.Buffer
	tb.Str("error: unknown command: ").Str(word)
	if len(near) == 0 {
		return tb.String()
	}
	tb.Str(". Nearest: ")
	for index, name := range near[:min(len(near), nearestListedMax)] {
		if index > 0 {
			tb.Str(", ")
		}
		tb.Str(program).Byte(' ').Str(name)
	}
	if extra := len(near) - nearestListedMax; extra > 0 {
		tb.Str(" (and ").Int(int64(extra)).Str(" more)")
	}
	return tb.String()
}

// nearestCandidates answers, in name order, every command name the registry
// holds that is not hidden, and every namespace token those names open with:
// `setup docker kernel` adds `setup` and `setup docker`. A namespace counts
// only through a visible member, as members() and the namespace page count it.
func nearestCandidates() []string {
	return candidatesFrom(visibleCommandNames())
}

// visibleCommandNames answers the name of every registered command that is
// not hidden. It is the registry read nearestCandidates makes, and a package
// variable so a test can answer its own names for the length of one test
// (useNearestNamesForTest) rather than register probes into the process-wide
// registry, where every later test of the binary would meet them.
var visibleCommandNames = func() []string {
	names := make([]string, 0, 64)
	for _, registered := range Commands() {
		if Hidden(registered.Name) {
			continue
		}
		names = append(names, registered.Name)
	}
	return names
}

// candidatesFrom answers, in name order and once each, the command names and
// every namespace token they open with.
func candidatesFrom(commands []string) []string {
	seen := make(map[string]bool, 64)
	names := make([]string, 0, 64)
	add := func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	for _, command := range commands {
		add(command)
		words := strings.Fields(command)
		for depth := 1; depth < len(words); depth++ {
			add(strings.Join(words[:depth], " "))
		}
	}
	slices.Sort(names)
	return names
}

// nearestNames answers the candidates near the typed words, in the order the
// candidates come in. Three ranks are tried in turn, and the first that finds
// a candidate decides:
//
//  1. the candidate's words equal the typed leading words once order, spaces
//     and hyphens are ignored (`unit test` and `test-unit` find `test unit`);
//  2. the candidate's last words equal the typed leading words (`lint` finds
//     `go lint`);
//  3. the typed first word is within nearestDistanceMax of one of the
//     candidate's words, and under half its own length (`comit` finds the
//     commit commands).
//
// Ranks 1 and 2 try the longest run of leading words first, and rank 3 keeps
// only the candidates at the smallest distance found, so the most specific
// answer wins within a rank. typed MUST NOT be empty.
func nearestNames(typed, candidates []string) []string {
	if found := sameWordsNames(typed, candidates); len(found) != 0 {
		return found
	}
	if found := endingNames(typed, candidates); len(found) != 0 {
		return found
	}
	return spelledNames(typed[0], candidates)
}

// sameWordsNames is rank 1 of nearestNames.
func sameWordsNames(typed, candidates []string) []string {
	for count := len(typed); count > 0; count-- {
		key := wordsKey(typed[:count])
		var found []string
		for _, name := range candidates {
			if wordsKey(strings.Fields(name)) == key {
				found = append(found, name)
			}
		}
		if len(found) != 0 {
			return found
		}
	}
	return nil
}

// wordsKey answers the words split at their hyphens, sorted and joined by one
// space, so two spellings with the same words in any order and any joining
// answer one key.
func wordsKey(words []string) string {
	pieces := make([]string, 0, len(words)+2)
	for _, word := range words {
		for piece := range strings.SplitSeq(word, "-") {
			if piece != "" {
				pieces = append(pieces, piece)
			}
		}
	}
	slices.Sort(pieces)
	return strings.Join(pieces, " ")
}

// endingNames is rank 2 of nearestNames. A candidate of no more words than the
// typed run is not a trailing match: equal words would have resolved.
func endingNames(typed, candidates []string) []string {
	for count := len(typed); count > 0; count-- {
		leading := typed[:count]
		var found []string
		for _, name := range candidates {
			words := strings.Fields(name)
			if len(words) > count && slices.Equal(words[len(words)-count:], leading) {
				found = append(found, name)
			}
		}
		if len(found) != 0 {
			return found
		}
	}
	return nil
}

// spelledNames is rank 3 of nearestNames: the candidates holding a word at the
// smallest accepted edit distance from typed. A candidate's words are its
// space-separated words and the pieces between their hyphens, so `worktre`
// is near a hyphenated word and near each of its parts.
func spelledNames(typed string, candidates []string) []string {
	const tooFar = nearestDistanceMax + 1
	word := []rune(typed)
	best := tooFar
	var found []string
	for _, name := range candidates {
		distance := tooFar
		for _, piece := range namePieces(name) {
			distance = min(distance, boundedDistance(word, []rune(piece), tooFar))
		}
		if distance == tooFar {
			continue
		}
		if 2*distance >= len(word) {
			continue
		}
		if distance < best {
			best = distance
			found = found[:0]
		}
		if distance == best {
			found = append(found, name)
		}
	}
	return found
}

// namePieces answers a name's space-separated words, and for a hyphenated
// word each piece between its hyphens as well.
func namePieces(name string) []string {
	words := strings.Fields(name)
	pieces := make([]string, 0, len(words)+2)
	for _, word := range words {
		pieces = append(pieces, word)
		if !strings.Contains(word, "-") {
			continue
		}
		for piece := range strings.SplitSeq(word, "-") {
			if piece != "" {
				pieces = append(pieces, piece)
			}
		}
	}
	return pieces
}

// boundedDistance answers the edit distance between a and b, counting an
// insertion, a deletion, a substitution and a swap of two neighbors as one
// each (optimal string alignment), or limit when the distance is limit or
// more. Two words whose lengths differ by limit or more are answered limit
// at once, so a long argv word costs no table: the work is bounded by the
// candidate word's length times a word at most limit longer.
func boundedDistance(a, b []rune, limit int) int {
	if abs(len(a)-len(b)) >= limit {
		return limit
	}
	// Three rows of the table: before the previous, previous and current.
	before := make([]int, len(b)+1)
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for column := range previous {
		previous[column] = column
	}
	for row := 1; row <= len(a); row++ {
		current[0] = row
		for column := 1; column <= len(b); column++ {
			cost := 1
			if a[row-1] == b[column-1] {
				cost = 0
			}
			current[column] = min(previous[column]+1, current[column-1]+1, previous[column-1]+cost)
			if row > 1 && column > 1 && a[row-1] == b[column-2] && a[row-2] == b[column-1] {
				current[column] = min(current[column], before[column-2]+1)
			}
		}
		before, previous, current = previous, current, before
	}
	return min(previous[len(b)], limit)
}

// abs answers the magnitude of n.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
