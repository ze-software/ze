// Design: docs/architecture/third-party-credits.md -- how a licence, a holder line and a banner are recognized
// Overview: check.go -- the refusals these answer for

package credits

import (
	"errors"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// licenceSentences maps each SPDX identifier the check knows to the titles
// and defining sentences that state it in a licence text. A declared
// identifier with no row is refused, never passed: a table that does not know
// a licence cannot say the text states it. A match ignores case and the
// layout of white space (normalized).
//
// A BSD-3-Clause text also states the BSD-2-Clause sentence, so the table
// cannot tell those two apart; it catches an identifier the text does not
// state at all, which is the error a hand-filled declaration makes. The 0BSD
// grant ends at "granted.", where ISC's goes on to "granted, provided".
var licenceSentences = map[string][]string{
	"0BSD":         {"Zero-Clause BSD", "Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted."},
	"Apache-2.0":   {"Apache License Version 2.0, January 2004", "Licensed under the Apache License, Version 2.0", "Apache license 2.0"},
	"BSD-2-Clause": {"Redistributions in binary form must reproduce the above copyright notice"},
	"BSD-3-Clause": {"may be used to endorse or promote products derived from this software", "BSD-3-Clause"},
	"CC0-1.0":      {"CC0 1.0"},
	"ISC":          {"copyright notice and this permission notice appear in all copies"},
	"MIT":          {"Permission is hereby granted, free of charge, to any person obtaining a copy", "MIT License", "@license MIT"},
	"MPL-2.0":      {"Mozilla Public License Version 2.0", "Mozilla Public License, v. 2.0", "Mozilla Public License 2.0"},
	"Unlicense":    {"This is free and unencumbered software released into the public domain"},
}

// licenceTable is where a refusal sends a reader to add an identifier.
const licenceTable = "licenceSentences in internal/le/deps/credits/licences.go"

// The operators an expression may join its identifiers with. The text must
// state every identifier under both: an OR expression is the choice the
// work's licence offers, so the text states each alternative, and an AND
// expression is several licences that all apply.
const (
	operatorOr  = "OR"
	operatorAnd = "AND"
)

// licenceIdentifiers answers the identifiers of an SPDX expression the check
// can judge: one identifier, or identifiers joined by OR alone or by AND
// alone. Any other shape (parentheses, WITH, a trailing "+", OR beside AND)
// is refused rather than half read, because no declaration uses one.
func licenceIdentifiers(expression string) ([]string, error) {
	fields := strings.Fields(expression)
	if len(fields) == 0 {
		return nil, errors.New("the licence expression names no identifier")
	}
	if strings.ContainsAny(expression, "()+") {
		return nil, errors.New("the check reads an expression without parentheses or \"+\"")
	}
	var identifiers []string
	operator := ""
	for index, field := range fields {
		if index%2 == 0 {
			if isOperator(field) {
				return nil, errors.New("the licence expression has an operator where an identifier belongs")
			}
			identifiers = append(identifiers, field)
			continue
		}
		if !isOperator(field) {
			return nil, errors.New("the check reads identifiers joined by OR or by AND, and finds " + field)
		}
		if operator != "" && operator != field {
			return nil, errors.New("the check reads identifiers joined by OR alone or by AND alone, not both")
		}
		operator = field
	}
	if len(fields)%2 == 0 {
		return nil, errors.New("the licence expression ends with an operator")
	}
	return identifiers, nil
}

// isOperator answers whether field is an operator the check reads.
func isOperator(field string) bool {
	return field == operatorOr || field == operatorAnd
}

// normalized is text in the form a sentence is matched against: each line
// with its leading comment markers removed, the lines joined by one space,
// white space collapsed, lower case.
func normalized(text string) string {
	var joined strings.Builder
	for line := range strings.SplitSeq(text, "\n") {
		joined.WriteString(strings.TrimLeft(line, " \t\r*/#!"))
		joined.WriteByte(' ')
	}
	return strings.ToLower(strings.Join(strings.Fields(joined.String()), " "))
}

// statesIdentifier answers whether normalizedText states identifier by one
// of its table rows. The caller has checked the identifier has a row.
func statesIdentifier(identifier, normalizedText string) bool {
	for _, sentence := range licenceSentences[identifier] {
		if strings.Contains(normalizedText, normalized(sentence)) {
			return true
		}
	}
	return false
}

// copyrightSymbols are the marks that stand for the word copyright.
var copyrightSymbols = []string{"(c)", "(C)", "©"}

// proseAfterCopyright are words that follow "Copyright" in a licence's own
// prose ("Grant of Copyright License", "Copyright Notice") rather than in a
// holder line.
var proseAfterCopyright = []string{"License", "Licence", "Notice", "Owner", "Holder", "Holders", "Act", "Law"}

// holderLine answers the first line of text that states a copyright holder,
// and false when none does. A line states one where it carries a copyright
// notice's own marks: the word "Copyright" or "copyright" followed by a year
// ("Copyright 2019 The Thing Authors"), the word followed by (c), (C) or ©
// and then a year or a name that is not licence prose ("Copyright (c) Acme
// Widgets"), or (c), (C) or © followed by a year. The word followed by a
// capitalized word alone is prose, not a notice: "Copyright Rights in the
// Protocol Registries" titles a statement. A template's "Copyright [yyyy]",
// the phrase "copyright notice", a list item "(c) You must" and the
// all-capitals "COPYRIGHT HOLDERS" do not state a holder either.
func holderLine(text string) (string, bool) {
	for line := range strings.SplitSeq(text, "\n") {
		if statesHolder(strings.Fields(line)) {
			return strings.TrimSpace(line), true
		}
	}
	return "", false
}

// statesHolder answers whether one line's words state a holder.
func statesHolder(words []string) bool {
	for index, word := range words {
		next := words[index+1:]
		if slices.Contains(copyrightSymbols, word) {
			if len(next) > 0 && startsWithDigit(next[0]) {
				return true
			}
			continue
		}
		if strings.TrimRight(word, ":") != "Copyright" && strings.TrimRight(word, ":") != "copyright" {
			continue
		}
		if len(next) == 0 {
			continue
		}
		if startsWithDigit(next[0]) {
			return true
		}
		if !slices.Contains(copyrightSymbols, next[0]) {
			continue
		}
		if len(next) > 1 && startsHolder(next[1]) {
			return true
		}
	}
	return false
}

func startsWithDigit(word string) bool {
	first, _ := utf8.DecodeRuneInString(word)
	return unicode.IsDigit(first)
}

// startsHolder answers whether word can open a holder's name or a year.
func startsHolder(word string) bool {
	if startsWithDigit(word) {
		return true
	}
	first, _ := utf8.DecodeRuneInString(word)
	if !unicode.IsUpper(first) {
		return false
	}
	return !slices.Contains(proseAfterCopyright, strings.TrimRight(word, ".,;:"))
}

// holders answers the holder names of a declared copyright string: several
// are joined by ", ".
func holders(copyright string) []string {
	var names []string
	for name := range strings.SplitSeq(copyright, ", ") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}

// companionSuffix ends the name of a bundle's companion licence file: the
// name webpack gives the banners it extracts from a bundle.
const companionSuffix = ".js.LICENSE.txt"

// banner is one comment of a companion file: one bundled library's notice.
type banner struct {
	line int
	text string
}

// banners answers each /* ... */ comment of a companion file with the line it
// opens on. An unclosed comment runs to the end of the file.
func banners(text string) []banner {
	var found []banner
	offset := 0
	for {
		start := strings.Index(text[offset:], "/*")
		if start < 0 {
			return found
		}
		start += offset
		end := strings.Index(text[start+2:], "*/")
		stop := len(text)
		if end >= 0 {
			stop = start + 2 + end + 2
		}
		found = append(found, banner{line: strings.Count(text[:start], "\n") + 1, text: text[start:stop]})
		offset = stop
	}
}

// title is a banner's first line of text, with its comment markers and a
// leading "@license" removed: what a refusal quotes.
func (b banner) title() string {
	for line := range strings.SplitSeq(b.text, "\n") {
		trimmed := strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "/*! "))
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "@license"))
		if trimmed != "" {
			return trimmed
		}
	}
	return strings.TrimSpace(b.text)
}

// claims answers whether a claimant ties itself to item. The banners share
// no field naming their library, so no name is read out of them: a claimant
// claims a banner its name appears in as a whole word, ignoring case, or one
// that contains its declared banner phrase verbatim.
func claims(claimant *entry, item banner) bool {
	if containsWord(item.text, claimant.name) {
		return true
	}
	if claimant.banner == "" {
		return false
	}
	return strings.Contains(item.text, claimant.banner)
}

// unclaimedBanners answers the banners no claimant claims.
func unclaimedBanners(found []banner, claimants []entry) []banner {
	var left []banner
	for _, item := range found {
		if claimedByAny(item, claimants) {
			continue
		}
		left = append(left, item)
	}
	return left
}

// claimedByAny answers whether one of claimants claims item.
func claimedByAny(item banner, claimants []entry) bool {
	for index := range claimants {
		if claims(&claimants[index], item) {
			return true
		}
	}
	return false
}

// phraseFound answers whether claimant's declared banner phrase is in one of
// the banners, so a phrase no banner writes is refused rather than kept as a
// link to nothing.
func phraseFound(claimant *entry, found []banner) bool {
	for _, item := range found {
		if strings.Contains(item.text, claimant.banner) {
			return true
		}
	}
	return false
}

// containsWord answers whether word appears in text, ignoring case, with no
// letter, digit, hyphen or underscore on either side: "buffer" is not in
// "safe-buffer".
func containsWord(text, word string) bool {
	if word == "" {
		return false
	}
	lowerText := strings.ToLower(text)
	lowerWord := strings.ToLower(word)
	offset := 0
	for {
		found := strings.Index(lowerText[offset:], lowerWord)
		if found < 0 {
			return false
		}
		start := offset + found
		stop := start + len(lowerWord)
		before, _ := utf8.DecodeLastRuneInString(lowerText[:start])
		after, _ := utf8.DecodeRuneInString(lowerText[stop:])
		if !nameRune(before) && !nameRune(after) {
			return true
		}
		offset = start + 1
	}
}

// nameRune answers whether r can be part of a library name. RuneError, which
// the decoders answer at either end of the text, is not.
func nameRune(r rune) bool {
	if r == '-' || r == '_' {
		return true
	}
	if r == utf8.RuneError {
		return false
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
