package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realRFCText reads rfc/full/<stem>.txt from this checkout, so a case quotes
// a sentence an RFC really carries rather than one written for the test.
func realRFCText(t *testing.T, stem string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "rfc", "full", stem+".txt"))
	if err != nil {
		t.Fatalf("the real text of %s is unreadable: %v", stem, err)
	}
	return string(body)
}

// The sentences of rfc/full texts the cases below quote.
const (
	rfc9915RelayReply = "Clients and servers MUST discard any received Relay-reply messages."
	rfc2784Compliant  = "Note that a compliant implementation MUST accept and process this field."
)

// VALIDATES: R1 -- a {not-applicable} quote is one of two shapes. A quote
// holding a BCP 14 keyword must be the row's own obligation sentence (the row
// text holds it, or it holds the row text), and a quote with no keyword must
// carry a scoping marker as a whole word; any other quote is refused, naming
// both shapes. A scoping sentence's bound role must be in it as whole words.
// METHOD: rows of the fixture RFC through the parser and
// (*evidenceReader).gap, the row text varied per case.
func TestNotApplicableQuoteIsOneOfTwoShapes(t *testing.T) {
	tree := t.TempDir()
	writeFixtureFiles(t, tree, map[string]string{"rfc/full/rfc9999.txt": quoteFixtureSource})
	own := evidenceRow(t, "RFC9999-2-1", "2", `not-applicable: "`+fixtureReflectorSentence+`"; binds a route reflector; it reflects no widget`)
	cases := []struct{ name, text, says string }{
		{"the row's own sentence", fixtureReflectorSentence, ""},
		{"the row text without its full stop", strings.TrimSuffix(fixtureReflectorSentence, "."), ""},
		{"another obligation of the section", "A speaker MUST send the widget before it sends any frame that depends on it.",
			"is not the row's own obligation sentence"},
		{"a row with no text", "", "is not the row's own obligation sentence"},
	}
	for _, one := range cases {
		req := own
		req.Text = one.text
		missing, lacks := newEvidenceReader(tree).gap(&req)
		if one.says == "" {
			if lacks {
				t.Errorf("%s: refused: %s", one.name, missing)
			}
			continue
		}
		if !lacks || !strings.Contains(missing, one.says) || !strings.Contains(missing, "one of two shapes") {
			t.Errorf("%s: answered %q, want a refusal saying %q and naming both shapes", one.name, missing, one.says)
		}
	}
	scoping := evidenceRow(t, "RFC9999-3-1", "3", `not-applicable: "`+fixtureScopingSentence+`"; binds relay agents; it relays nothing`)
	scoping.Text = "A receiver MUST NOT reorder the frames of one widget."
	if missing, lacks := newEvidenceReader(tree).gap(&scoping); lacks {
		t.Errorf("a scoping sentence of the cited section was refused: %s", missing)
	}
	parses := []struct{ payload, says string }{
		{`"This document describes widgets and the frames that carry them."; binds widgets; it carries nothing`,
			"holds no BCP 14 keyword and scopes nothing to a role"},
		{`"` + fixtureScopingSentence + `"; binds relay ag; it relays nothing`, "is not in the quoted sentence"},
		{`"This section is restricted to relay agents of the widget."; binds relay agents; it relays nothing`, ""},
		{`"This section is limited to relay agents of the widget."; binds relay agents; it relays nothing`, ""},
		{`"This section is limited relay agents of the widget."; binds relay agents; it relays nothing`, "scopes nothing"},
	}
	for _, one := range parses {
		_, err := parseAnnotation("not-applicable: "+one.payload, "fixture")
		if one.says == "" {
			if err != nil {
				t.Errorf("%s: refused: %v", one.payload, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), one.says) {
			t.Errorf("%s: answered %v, want a refusal saying %q", one.payload, err, one.says)
		}
	}
	_, err := parseAnnotation(`not-applicable: "This document describes widgets and the frames that carry them."; binds widgets; why`, "fixture")
	if err == nil || !strings.Contains(err.Error(), "one of two shapes") || !strings.Contains(err.Error(), "{gap: ...}") {
		t.Errorf("a quote of neither shape answered %v, want both shapes and the {gap} outlet named", err)
	}
}

// VALIDATES: R2 -- a {not-applicable} quote is a whole sentence of its cited
// section: one that starts mid-sentence or stops short of the terminator is
// refused, and one that crosses a page break or an RFC line wrap is whole.
// METHOD: wholeSentence over the fixture RFC and two real texts, and gap over
// a fragment row of the fixture.
func TestNotApplicableQuoteIsAWholeSentence(t *testing.T) {
	fixture := newQuoteSource(quoteFixtureSource)
	rfc2784 := newQuoteSource(realRFCText(t, "rfc2784"))
	cases := []struct {
		source  *quoteSource
		section string
		quote   string
		whole   bool
	}{
		{fixture, "2", fixtureReflectorSentence, true},
		{fixture, "2", "A speaker MUST send the widget before it sends any frame that depends on it.", true},
		{fixture, "2", fixtureRelaySentence, true},
		{fixture, "2", "route reflector MUST NOT reflect the widget to its own clients.", false},
		{fixture, "2", "A route reflector MUST NOT reflect the widget", false},
		{fixture, "2", "Widgets A speaker MUST send the widget before it sends any frame that depends on it.", false},
		{fixture, "3", fixtureScopingSentence, true},
		{newQuoteSource(realRFCText(t, "rfc9915")), "16.14", rfc9915RelayReply, true},
		{rfc2784, "2.2", rfc2784Compliant, true},
		{rfc2784, "2.2", "a compliant implementation MUST accept and process this field.", false},
	}
	for _, one := range cases {
		if got := one.source.wholeSentence(one.section, one.quote); got != one.whole {
			t.Errorf("wholeSentence(%s, %q) = %v, want %v", one.section, one.quote, got, one.whole)
		}
	}
	tree := t.TempDir()
	writeFixtureFiles(t, tree, map[string]string{"rfc/full/rfc9999.txt": quoteFixtureSource})
	fragment := evidenceRow(t, "RFC9999-2-1", "2",
		`not-applicable: "route reflector MUST NOT reflect the widget to its own clients."; binds a route reflector; it reflects no widget`)
	fragment.Text = fixtureReflectorSentence
	if missing, lacks := newEvidenceReader(tree).gap(&fragment); !lacks || !strings.Contains(missing, "is not a whole sentence of section 2") {
		t.Errorf("a fragment of a sentence answered %q, want the whole-sentence refusal", missing)
	}
}

// VALIDATES: R3 -- the subject of a quoted obligation is the bound role alone,
// with only determiners or quantifiers before it or between it and the
// keyword. A compound subject (and, or, nor, a comma), a clause (that, which,
// who), a qualifying preposition (of, from, to, by, with) or "not" is refused
// with the two honest outlets; a subject with other words before the role is
// refused as not the subject.
// METHOD: parseAnnotation over a real rfc9915 sentence and fixture sentences.
func TestNotApplicableSubjectNamesOneRole(t *testing.T) {
	if !strings.Contains(squashWhitespace(realRFCText(t, "rfc9915")), rfc9915RelayReply) {
		t.Fatalf("rfc/full/rfc9915.txt no longer carries %q", rfc9915RelayReply)
	}
	cases := []struct{ quote, bound, says string }{
		{rfc9915RelayReply, "servers", "holds 'and' before MUST"},
		{"Relay agents or proxies MUST forward the widget to every client.", "relay agents", "holds 'or'"},
		{"Neither relay agents nor proxies MAY forward the widget to a client.", "relay agents", "holds 'nor'"},
		{"Relay agents, proxies and caches MUST forward the widget to every client.", "relay agents", "holds ','"},
		{"A relay agent that receives the widget MUST drop it at once.", "a relay agent", "holds 'that'"},
		{"A relay agent which receives the widget MUST drop it at once.", "a relay agent", "holds 'which'"},
		{"A route reflector of a confederation MUST reflect the widget.", "a route reflector", "holds 'of'"},
		{"A route reflector with clients MUST reflect the widget to them.", "a route reflector", "holds 'with'"},
		{"A relay agent not in the path MUST NOT forward the widget.", "a relay agent", "holds 'not'"},
		{"Every remote route reflector MUST reflect the widget to every client.", "reflector", "is not the subject of the quoted obligation"},
		{"Route reflectors each MUST reflect the widget to every client.", "a route reflector", ""},
		{"All the route reflectors MUST reflect the widget to every client.", "route reflectors", ""},
	}
	for _, one := range cases {
		_, err := parseAnnotation(`not-applicable: "`+one.quote+`"; binds `+one.bound+`; it relays nothing`, "fixture")
		if one.says == "" {
			if err != nil {
				t.Errorf("%q binds %q: refused: %v", one.quote, one.bound, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), one.says) {
			t.Errorf("%q binds %q: answered %v, want a refusal saying %q", one.quote, one.bound, err, one.says)
			continue
		}
		if strings.Contains(one.says, "holds") && !strings.Contains(err.Error(), "{gap: ...}") {
			t.Errorf("%q binds %q: the refusal %v names no {gap} outlet", one.quote, one.bound, err)
		}
	}
}

// VALIDATES: R4 -- an adjective that says how a party behaves, never which
// role it fills (nonRoleAdjectives), leaves a generic role generic: "a
// compliant implementation" and "receiving routers" are refused as words for
// any party, while "a compliant route reflector" names a role.
// METHOD: genericRole directly, and the real rfc2784 sentence through the
// parser.
func TestNonRoleAdjectivesLeaveARoleGeneric(t *testing.T) {
	cases := []struct {
		role    string
		generic bool
	}{
		{"a compliant implementation", true},
		{"receiving routers", true},
		{"new conformant speakers", true},
		{"a compliant", true},
		{"a compliant route reflector", false},
		{"a route reflector", false},
	}
	for _, one := range cases {
		if got := genericRole(one.role); got != one.generic {
			t.Errorf("genericRole(%q) = %v, want %v", one.role, got, one.generic)
		}
	}
	if !strings.Contains(squashWhitespace(realRFCText(t, "rfc2784")), rfc2784Compliant) {
		t.Fatalf("rfc/full/rfc2784.txt no longer carries %q", rfc2784Compliant)
	}
	_, err := parseAnnotation(`not-applicable: "`+rfc2784Compliant+`"; binds a compliant implementation; ze is not one`, "fixture")
	if err == nil || !strings.Contains(err.Error(), "a word for any party to a protocol") {
		t.Errorf("binds a compliant implementation answered %v, want the generic refusal", err)
	}
}

// VALIDATES: roleKey and a subject's words treat punctuation the same way, so
// a parenthesised abbreviation and a trailing comma do not split one role into
// two spellings.
// METHOD: roleKey over spellings of one role, and the parser over a subject
// carrying the abbreviation.
func TestRoleKeyCutsPunctuationAsASubjectDoes(t *testing.T) {
	for _, spelling := range []string{"a route reflector (RR)", "Route Reflectors (RR)", "the route reflector (RR),"} {
		if got := roleKey(spelling); got != "route reflector rr" {
			t.Errorf("roleKey(%q) = %q, want %q", spelling, got, "route reflector rr")
		}
	}
	_, err := parseAnnotation(`not-applicable: "A route reflector (RR) MUST reflect the widget to every client."; binds a route reflector (RR); it reflects nothing`, "fixture")
	if err != nil {
		t.Errorf("binds a route reflector (RR) against its own subject was refused: %v", err)
	}
}

// VALIDATES: B1 -- a parsed row is judged by its quote, the text before its
// trailing (§N): a row stating the quoted obligation passes whether or not it
// keeps the sentence's full stop, and a row stating another obligation is
// refused naming the row's quote, never its (§N).
// METHOD: real summary lines through ParseSummaryText, then
// (*evidenceReader).gap on the parsed row.
func TestNotApplicableParsedRowIsJudgedByItsQuote(t *testing.T) {
	tree := t.TempDir()
	writeFixtureFiles(t, tree, map[string]string{"rfc/full/rfc9999.txt": quoteFixtureSource})
	annotation := ` (§2) {not-applicable: "` + fixtureReflectorSentence + `"; binds a route reflector; ze reflects nothing}`
	other := "A speaker MUST NOT send the widget before it sends any frame that depends on it"
	cases := []struct{ name, text, says string }{
		{"with its full stop", fixtureReflectorSentence, ""},
		{"without its full stop", strings.TrimSuffix(fixtureReflectorSentence, "."), ""},
		{"another obligation", other, "'" + other + "'"},
	}
	for _, one := range cases {
		row := "- [ ] [RFC9999-2-9] [MUST NOT] " + one.text + annotation
		rows, err := ParseSummaryText(fixtureWithZeRoles(t, "speaker", row), selftestStem, selftestSummaryRel)
		if err != nil {
			t.Fatalf("%s: the summary did not parse: %v", one.name, err)
		}
		found := false
		for i := range rows {
			if rows[i].RID != "RFC9999-2-9" {
				continue
			}
			found = true
			missing, lacks := newEvidenceReader(tree).gap(&rows[i])
			if one.says == "" {
				if lacks {
					t.Errorf("%s: a parsed row stating the quoted obligation was refused: %s", one.name, missing)
				}
				continue
			}
			if !lacks || !strings.Contains(missing, one.says) || strings.Contains(missing, "(§2)") {
				t.Errorf("%s: answered %q, want a refusal naming the row's quote %s without its (§2)", one.name, missing, one.says)
			}
		}
		if !found {
			t.Fatalf("%s: the parsed summary holds no RFC9999-2-9", one.name)
		}
	}
}

// compoundFixtureSource is an RFC whose section 2 holds one sentence of two
// clauses with different subjects, a negated scoping sentence, a bulleted
// obligation, an obligation opening in lower case after a full stop, and a
// sentence holding an abbreviation before a lower-case word.
const compoundFixtureSource = "Test RFC 9999\n\n1.  Introduction\n\n   This document describes widgets.\n\n" +
	"2.  Widgets\n\n" +
	"   " + compoundSentence + "\n\n" +
	"   " + compoundOneLevelSentence + "\n\n" +
	"   This section does not apply to route reflectors.\n\n" +
	"   o  A speaker MUST NOT drop the widget it received.\n\n" +
	"   The widget MUST be sent. " + lowerCaseSentence + "\n\n" +
	"   " + abbreviationSentence + "\n"

const (
	compoundSentence = "A route reflector MAY keep the widget, and a speaker MUST send the widget to every peer."
	// compoundOneLevelSentence has one keyword level in both clauses, so only
	// the span check tells its second clause from its first.
	compoundOneLevelSentence = "A route reflector MUST keep the widget, and a speaker MUST send the widget to every peer."
	lowerCaseSentence        = "iBGP speakers MUST forward the widget unchanged."
	// abbreviationSentence holds "etc." before a lower-case word, which
	// quoteAbbreviations alone keeps from ending the sentence.
	abbreviationSentence = "A speaker MAY drop widgets, frames, etc. and keep the rest."
)

// VALIDATES: I1 -- a quoted obligation binds the row only through the row's
// own clause: the quote's first BCP 14 keyword must lie inside the span the
// row's quote occupies, and must be the row's level. A compound sentence
// whose first clause names a route reflector cannot exclude ze from its
// second clause's speaker obligation.
// METHOD: rows over compoundFixtureSource through (*evidenceReader).gap.
func TestNotApplicableKeywordIsTheRowsClause(t *testing.T) {
	tree := t.TempDir()
	writeFixtureFiles(t, tree, map[string]string{"rfc/full/rfc9999.txt": compoundFixtureSource})
	compound := evidenceRow(t, "RFC9999-2-1", "2", `not-applicable: "`+compoundSentence+`"; binds a route reflector; ze reflects nothing`)
	cases := []struct {
		name, text, level string
		says              []string
	}{
		{"the first clause, at its level", "A route reflector MAY keep the widget (§2)", levelMay, nil},
		{"the second clause", "a speaker MUST send the widget to every peer (§2)", levelMust,
			[]string{"first BCP 14 keyword MAY", "outside the row's clause 'a speaker MUST send the widget to every peer'"}},
		{"the first clause at another level", "A route reflector MAY keep the widget (§2)", levelMust,
			[]string{"first BCP 14 keyword MAY", "the row's level is MUST"}},
		{"the whole sentence at another level", compoundSentence + " (§2)", levelMust,
			[]string{"first BCP 14 keyword MAY", "the row's level is MUST"}},
	}
	for _, one := range cases {
		req := compound
		req.Text, req.Level = one.text, one.level
		missing, lacks := newEvidenceReader(tree).gap(&req)
		if one.says == nil {
			if lacks {
				t.Errorf("%s: refused: %s", one.name, missing)
			}
			continue
		}
		if !lacks {
			t.Errorf("%s: passed, want a refusal", one.name)
			continue
		}
		for _, says := range one.says {
			if !strings.Contains(missing, says) {
				t.Errorf("%s: answered %q, want it to say %q", one.name, missing, says)
			}
		}
	}
	oneLevel := evidenceRow(t, "RFC9999-2-2", "2", `not-applicable: "`+compoundOneLevelSentence+`"; binds a route reflector; ze reflects nothing`)
	oneLevel.Text = "a speaker MUST send the widget to every peer (§2)"
	missing, lacks := newEvidenceReader(tree).gap(&oneLevel)
	if !lacks || !strings.Contains(missing, "first BCP 14 keyword MUST lies outside the row's clause") {
		t.Errorf("the second clause of a one-level compound answered %q, want the span refusal", missing)
	}
}

// VALIDATES: I2 -- a scoping sentence that negates or excepts its scope
// cannot bind the role it names: "does not apply to route reflectors" scopes
// the section away from them. Every word of scopingNegations is refused as a
// whole word, and a word holding one ("another", "nothing") is not.
// METHOD: parseAnnotation over scoping quotes holding each negation.
func TestNotApplicableScopingSentenceIsNotNegated(t *testing.T) {
	cases := []struct{ quote, says string }{
		{"This section does not apply to route reflectors.", "'not'"},
		{"No part of this section applies to route reflectors.", "'no'"},
		{"This section applies to every party except route reflectors.", "'except'"},
		{"This section applies to route reflectors unless they are clients.", "'unless'"},
		{"This section applies to parties other than route reflectors.", "'other than'"},
		{"This section applies to neither clients nor route reflectors.", "'neither'"},
		{"This section applies only to route reflectors and another widget.", ""},
	}
	for _, one := range cases {
		_, err := parseAnnotation(`not-applicable: "`+one.quote+`"; binds route reflectors; ze reflects nothing`, "fixture")
		if one.says == "" {
			if err != nil {
				t.Errorf("%q: refused: %v", one.quote, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), one.says) || !strings.Contains(err.Error(), "{gap: ...}") {
			t.Errorf("%q: answered %v, want a refusal naming %s and the {gap} outlet", one.quote, err, one.says)
		}
	}
}

// VALIDATES: I3 -- a bulleted obligation and one opening in lower case after a
// full stop are whole sentences, while an abbreviation ("e.g.", "i.e.")
// followed by a lower-case word does not end one.
// METHOD: wholeSentence over compoundFixtureSource and real RFC 1812 text.
func TestNotApplicableQuoteSentenceSplitsListsAndLowerCase(t *testing.T) {
	fixture := newQuoteSource(compoundFixtureSource)
	rfc1812 := newQuoteSource(realRFCText(t, "rfc1812"))
	cases := []struct {
		source  *quoteSource
		section string
		quote   string
		whole   bool
	}{
		{fixture, "2", "A speaker MUST NOT drop the widget it received.", true},
		{fixture, "2", lowerCaseSentence, true},
		{fixture, "2", "The widget MUST be sent.", true},
		{fixture, "2", "The widget MUST be sent. " + lowerCaseSentence, false},
		{fixture, "2", abbreviationSentence, true},
		{rfc1812, "4.2.2.11", "A router MUST receive and process normally any packets with a broadcast destination address.", true},
		{rfc1812, "F.1.2", "An implementation MUST NOT send a network back to the autonomous system from which it is learned, i.e. it MUST do split-horizon on an autonomous system level.", true},
		{rfc1812, "F.1.2", "it MUST do split-horizon on an autonomous system level.", false},
		{rfc1812, "5.3.13.2", "It is commonly but not always the case that the two limits are the same (e.g. a single-level interface).", true},
	}
	for _, one := range cases {
		if got := one.source.wholeSentence(one.section, one.quote); got != one.whole {
			t.Errorf("wholeSentence(%s, %q) = %v, want %v", one.section, one.quote, got, one.whole)
		}
	}
}

// VALIDATES: N1, N2 -- a bound role made only of determiners is generic, and
// one made only of pronouns, scoping markers or negations names no role.
// METHOD: parseAnnotation over a scoping quote that holds each bound word.
func TestNotApplicableBoundRoleNamesAParty(t *testing.T) {
	quote := "This section applies only to these relay agents and not all of them."
	cases := []struct{ bound, says string }{
		{"all", "a word for any party"},
		{"these", "a word for any party"},
		{"this only", "names no party"},
		{"them", "names no party"},
		{"not", "names no party"},
		{"relay agents", "'not'"},
	}
	for _, one := range cases {
		_, err := parseAnnotation(`not-applicable: "`+quote+`"; binds `+one.bound+`; ze relays nothing`, "fixture")
		if err == nil || !strings.Contains(err.Error(), one.says) {
			t.Errorf("binds %s: answered %v, want a refusal saying %q", one.bound, err, one.says)
		}
	}
}
