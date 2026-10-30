// Design: docs/contributing/rfc-conformance-gates.md -- the exclusion evidence ratchet
// Overview: inventory.go -- quoteSource and sentences, the inventory splitter these refine
// Related: check_evidence.go -- roleQuoteGap, which holds a role-condition quote to a whole sentence
package rfc

import (
	"slices"
	"strings"
)

// quoteAbbreviations are the words, lower case and with their full stop, that
// end in a full stop without ending a sentence, where a lower-case word
// follows them: "e.g. a single-level interface". splitLowerCaseSentence does
// not cut after one. A word with a full stop inside it ("i.e.", "a.k.a."), a
// single letter (an author's initial, a list label) and a number are not cut
// after either, so the list holds only the abbreviations of whole words.
const quoteAbbreviations = "e.g. i.e. etc. sec. cf. vs. ed. al. no. fig. viz."

// quoteListMarkers are the list markers an RFC opens an item with, each
// followed by a blank: stripListMarker cuts one from the front of a sentence,
// so the bullet "o  A router MUST ..." is the sentence "A router MUST ...".
// A lettered or numbered item marker, "(a)", "a)" or "1)", is matched by
// shape in listItemMarker.
const quoteListMarkers = "o - * +"

// quoteSentences answers the sentences of one section body as a
// role-condition quote is held to them (wholeSentence): the inventory's own
// sentences(), each further cut where a full stop and a blank are followed by
// a lower-case word (splitLowerCaseSentence), and each with a leading list
// marker cut (stripListMarker). The inventory's site numbering reads
// sentences() itself, so this refinement changes no site id.
func quoteSentences(body string) []string {
	var out []string
	for _, chunk := range sentences(body) {
		for _, piece := range splitLowerCaseSentence(chunk) {
			if piece = stripListMarker(piece); piece != "" {
				out = append(out, piece)
			}
		}
	}
	return out
}

// splitLowerCaseSentence cuts a whitespace-collapsed sentence at every ". "
// followed by a lower-case letter, unless the word that ends in that full stop
// does not end a sentence (endsASentence). splitSentences demands an upper-case
// follower, so "The widget MUST be sent. iBGP speakers MUST ..." reaches here
// as one chunk.
func splitLowerCaseSentence(chunk string) []string {
	var out []string
	start := 0
	for i := 1; i+1 < len(chunk); i++ {
		if chunk[i] != ' ' || chunk[i-1] != '.' {
			continue
		}
		if next := chunk[i+1]; next < 'a' || next > 'z' {
			continue
		}
		word := chunk[strings.LastIndexByte(chunk[:i], ' ')+1 : i]
		if !endsASentence(word) {
			continue
		}
		out = append(out, chunk[start:i])
		start = i + 1
	}
	return append(out, chunk[start:])
}

// endsASentence answers whether word, which ends in a full stop, ends a
// sentence: not when it is one of quoteAbbreviations, holds another full stop
// ("i.e.", "..."), is a single letter or a number, once a leading "(" is cut.
func endsASentence(word string) bool {
	word = strings.ToLower(strings.TrimLeft(word, "("))
	stem := strings.TrimSuffix(word, ".")
	if stem == "" || strings.Contains(stem, ".") {
		return false
	}
	if slices.Contains(strings.Fields(quoteAbbreviations), word) {
		return false
	}
	if len(stem) == 1 {
		return false
	}
	return strings.Trim(stem, "0123456789") != ""
}

// stripListMarker cuts one leading list marker and its blank from a
// whitespace-collapsed sentence: one of quoteListMarkers, or a lettered or
// numbered item marker (listItemMarker).
func stripListMarker(sentence string) string {
	marker, rest, found := strings.Cut(sentence, " ")
	if !found {
		return sentence
	}
	if slices.Contains(strings.Fields(quoteListMarkers), marker) {
		return rest
	}
	if listItemMarker(marker) {
		return rest
	}
	return sentence
}

// listItemMarker answers whether marker is "(a)", "a)" or "1)": one letter or
// a run of digits, closed by ")" and opened by "(" or by nothing.
func listItemMarker(marker string) bool {
	label, closed := strings.CutSuffix(marker, ")")
	if !closed {
		return false
	}
	label = strings.TrimPrefix(label, "(")
	if label == "" {
		return false
	}
	if len(label) == 1 && label[0] >= 'a' && label[0] <= 'z' {
		return true
	}
	return strings.Trim(label, "0123456789") == ""
}
