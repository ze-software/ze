package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zoneRefreshSentence is RFC 1035 Section 4.2's obligation, stated in the
// lowercase of a document published before RFC 2119, whose subject is an
// activity rather than a role.
const zoneRefreshSentence = "Zone refresh activities must use virtual circuits because of the need for reliable transfer."

// secondaryServerSentence is the RFC 1035 Section 2.2 sentence that names the
// role zone refresh belongs to.
const secondaryServerSentence = "Designated secondary servers can acquire zones and check for updates from the primary server using the zone transfer protocol of the DNS."

// TestRegisterByNumberReadsTheRFCNumber checks the one fact the register is
// derived from. Goal: an RFC numbered below 2119 reads lowercase must, and RFC
// 2119 itself, every later RFC and every draft read upper case alone. Method:
// registerByNumber over each kind of stem, the boundary on both sides of it
// included, and stems that carry no RFC number.
func TestRegisterByNumberReadsTheRFCNumber(t *testing.T) {
	cases := []struct {
		stem string
		want KeywordRegister
	}{
		{"rfc792", registerLowercase},
		{"rfc1035", registerLowercase},
		{"rfc2118", registerLowercase},
		{"rfc2119", registerUpperCase},
		{"rfc4035", registerUpperCase},
		{"draft-ietf-idr-widget-03", registerUpperCase},
		{"rfc", registerUpperCase},
		{"rfc12a", registerUpperCase},
		{"rfc99999999999999999999999", registerUpperCase},
	}
	for _, one := range cases {
		if got := registerByNumber(one.stem); got != one.want {
			t.Errorf("registerByNumber(%q) = %d, want %d", one.stem, got, one.want)
		}
	}
}

// TestPre2119ReadsLowercaseMustAndNothingElse checks the owner rule at the
// scanner every keyword reader shares. Goal: in a pre-RFC 2119 document
// lowercase "must" is MUST and "must not" is MUST NOT, a later document still
// reads no lowercase keyword, and lowercase "should" and "may" are never
// keywords. Method: firstKeyword, firstMandatoryKeyword and levelsIn over the
// same sentences in both registers.
func TestPre2119ReadsLowercaseMustAndNothingElse(t *testing.T) {
	cases := []struct {
		quote string
		pre   string
		upper string
	}{
		{zoneRefreshSentence, levelMust, ""},
		{"A resolver must not cache the answer.", levelMustNot, ""},
		{"A resolver MUST NOT cache the answer.", levelMustNot, levelMustNot},
		{"A resolver should cache the answer.", "", ""},
		{"A resolver may cache the answer.", "", ""},
		{"A resolver mustn't cache the answer.", "", ""},
	}
	for _, one := range cases {
		for _, read := range []struct {
			register KeywordRegister
			want     string
		}{{registerLowercase, one.pre}, {registerUpperCase, one.upper}} {
			at, keyword := read.register.firstKeyword(one.quote)
			if keyword != read.want || (at < 0) != (read.want == "") {
				t.Errorf("register %d firstKeyword(%q) = %d %q, want %q", read.register, one.quote, at, keyword, read.want)
			}
			mandatory := read.register.firstMandatoryKeyword(one.quote)
			if (mandatory < 0) != (read.want == "") {
				t.Errorf("register %d firstMandatoryKeyword(%q) = %d, want a keyword: %v", read.register, one.quote, mandatory, read.want != "")
			}
			if advisory, _ := read.register.levelsIn(one.quote); len(advisory) != 0 {
				t.Errorf("register %d levelsIn(%q) read the advisory levels %v in lowercase prose", read.register, one.quote, advisory)
			}
		}
	}
}

// TestPre2119NotApplicableShapeA checks the shape (a) a pre-RFC 2119 row takes.
// Goal: a row quoting its own lowercase obligation, whose subject is the bound
// role, parses and is the row's own obligation in a pre-RFC 2119 RFC; the same
// sentence in a later RFC is refused at the parser and at the evidence reader;
// and a lowercase "should" is no keyword in either. Method: parseAnnotation and
// ownObligationRefusal in both registers.
func TestPre2119NotApplicableShapeA(t *testing.T) {
	const must = "A secondary server must refresh the zone over a virtual circuit."
	const should = "A secondary server should refresh the zone over a virtual circuit."
	payload := func(quote string) string {
		return `not-applicable: "` + quote + `"; binds a secondary server; ze serves no zone`
	}
	parsed, err := parseAnnotation(payload(must), "fixture", registerLowercase)
	if err != nil {
		t.Fatalf("a pre-RFC 2119 shape (a) row was refused: %v", err)
	}
	if parsed.BoundRole != "a secondary server" || parsed.Quote != must {
		t.Errorf("parsed %+v, want the quote and the bound role", parsed)
	}
	row := strings.TrimSuffix(must, ".")
	if refusal, refused := ownObligationRefusal(row, levelMust, must, registerLowercase); refused {
		t.Errorf("a pre-RFC 2119 row's own lowercase obligation was refused: %s", refusal)
	}
	if _, err := parseAnnotation(payload(must), "fixture", registerUpperCase); err == nil ||
		!strings.Contains(err.Error(), "holds no BCP 14 keyword") {
		t.Errorf("a post-RFC 2119 row quoting lowercase must was not refused as keyword-less: %v", err)
	}
	if _, refused := ownObligationRefusal(row, levelMust, must, registerUpperCase); !refused {
		t.Error("a post-RFC 2119 row's lowercase must was read as its MUST")
	}
	for _, register := range []KeywordRegister{registerLowercase, registerUpperCase} {
		if _, err := parseAnnotation(payload(should), "fixture", register); err == nil ||
			!strings.Contains(err.Error(), "holds no BCP 14 keyword") {
			t.Errorf("register %d read lowercase should as a keyword: %v", register, err)
		}
	}
}

// TestPre2119RoleSentence checks the role sentence a pre-RFC 2119 row names
// its bound role from. Goal: RFC 1035 Section 4.2's obligation, whose subject
// is an activity, binds a secondary server through the Section 2.2 sentence
// that names the role, and the form is refused in a later RFC, with a role the
// sentence does not name, with a negated sentence, beside a subject that is
// already the role, beside a keyword-less quote, and with a sentence from
// another section or a fragment. Method: parseAnnotation in both registers,
// then ownObligationRefusal and roleSentenceGap over RFC 1035's own text.
func TestPre2119RoleSentence(t *testing.T) {
	payload := func(quote, bound, section, sentence string) string {
		return `not-applicable: "` + quote + `"; binds ` + bound + `; §` + section + ` "` + sentence + `"; ze serves no zone`
	}
	good := payload(zoneRefreshSentence, "a secondary server", "2.2", secondaryServerSentence)
	parsed, err := parseAnnotation(good, "fixture", registerLowercase)
	if err != nil {
		t.Fatalf("the RFC 1035 Section 4.2 annotation was refused: %v", err)
	}
	if parsed.RoleSection != "2.2" || parsed.RoleSentence != secondaryServerSentence {
		t.Errorf("parsed role sentence %q from §%q, want %q from §2.2", parsed.RoleSentence, parsed.RoleSection, secondaryServerSentence)
	}
	refusals := []struct {
		name     string
		payload  string
		register KeywordRegister
		want     string
	}{
		{"an upper-case-register RFC", payload("Zone refresh activities MUST use virtual circuits.", "a secondary server", "2.2", secondaryServerSentence),
			registerUpperCase, "only a document written in lowercase obligation language"},
		{"a role the sentence does not name", payload(zoneRefreshSentence, "a zone master", "2.2", secondaryServerSentence),
			registerLowercase, "which is not in its section 2.2 sentence"},
		{"a negated sentence", payload(zoneRefreshSentence, "a secondary server", "2.2", "Secondary servers do not acquire zones from the primary server."),
			registerLowercase, "carve the role out"},
		{"a subject that is the role", payload("A secondary server must use virtual circuits for zone refresh.", "a secondary server", "2.2", secondaryServerSentence),
			registerLowercase, "redundant"},
		{"a keyword-less quote", payload("This section applies only to secondary servers.", "a secondary server", "2.2", secondaryServerSentence),
			registerLowercase, "scoping sentence names its own role"},
		{"a section of no heading shape", payload(zoneRefreshSentence, "a secondary server", "2.x", secondaryServerSentence),
			registerLowercase, "no section id"},
	}
	for _, one := range refusals {
		if _, err := parseAnnotation(one.payload, "fixture", one.register); err == nil || !strings.Contains(err.Error(), one.want) {
			t.Errorf("%s: parseAnnotation answered %v, want a refusal holding %q", one.name, err, one.want)
		}
	}

	source := newQuoteSource(realRFCText(t, "rfc1035"))
	if !source.wholeSentence("4.2", zoneRefreshSentence) {
		t.Fatal("RFC 1035 Section 4.2 does not hold the zone refresh sentence whole")
	}
	row := strings.TrimSuffix(zoneRefreshSentence, ".")
	if refusal, refused := ownObligationRefusal(row, levelMust, zoneRefreshSentence, registerLowercase); refused {
		t.Errorf("RFC1035-4.2-1's own lowercase obligation was refused: %s", refusal)
	}
	req := &Requirement{RFC: "rfc1035", RID: "RFC1035-4.2-1", Level: levelMust, Section: "4.2", Annotation: parsed}
	if refusal, refused := roleSentenceGap(req, source); refused {
		t.Errorf("the Section 2.2 role sentence was refused: %s", refusal)
	}
	gaps := []struct {
		name     string
		section  string
		sentence string
		want     string
	}{
		{"another section", "4.2", secondaryServerSentence, "it is in section 2.2"},
		{"no heading", "2.9", secondaryServerSentence, "names no heading"},
		{"a fragment", "2.2", "Designated secondary servers can acquire zones", "not a whole sentence"},
	}
	for _, one := range gaps {
		moved := *parsed
		moved.RoleSection, moved.RoleSentence = one.section, one.sentence
		req.Annotation = &moved
		if refusal, refused := roleSentenceGap(req, source); !refused || !strings.Contains(refusal, one.want) {
			t.Errorf("%s: roleSentenceGap answered %q %v, want a refusal holding %q", one.name, refusal, refused, one.want)
		}
	}
}

// rfc2181TTLSentence is RFC 2181 Section 5.5's obligation, stated in the
// lowercase of a document numbered after RFC 2119 that never adopted its
// keywords (RFC 2181 Section 3).
const rfc2181TTLSentence = "Where duplicates are required this way, the TTL transmitted in each case must be the same."

// writeRegisterFixture writes the extraction sign-off and the RFC text of
// stem under tree, leaving out each one given as "".
func writeRegisterFixture(t *testing.T, tree, stem, extraction, text string) {
	t.Helper()
	for _, file := range []struct{ dir, suffix, body string }{
		{extractionRel, ".json", extraction},
		{fullRel, ".txt", text},
	} {
		if file.body == "" {
			continue
		}
		dir := filepath.Join(tree, filepath.FromSlash(file.dir))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, stem+file.suffix), []byte(file.body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// TestKeywordRegisterOfReadsTheExtraction checks the owner rule's objective:
// a document written in lowercase obligation language has its "must" read as
// MUST, whatever its number. Goal: an RFC from 2119 on whose extraction signs
// register prose and whose text never adopts BCP 14 reads lowercase must; one
// that adopts BCP 14 (in either spacing, across a line break) or signs
// rfc2119, or has no extraction, still reads upper case alone; an RFC below
// 2119 reads lowercase whatever its extraction says; a draft is unchanged
// even when it signs prose and cites nothing; an extraction that cannot be
// read, and a prose one with no RFC text, are errors. Method: KeywordRegisterOf
// over a fixture checkout holding each case, then the register's keyword
// scan over RFC 2181 Section 5.5's lowercase sentence.
func TestKeywordRegisterOfReadsTheExtraction(t *testing.T) {
	tree := t.TempDir()
	prose := `{"register": "prose"}`
	lowercase := "3. Terminology\n\n   This memo does not use the oft used expressions MUST, SHOULD, MAY.\n"
	adopts := "1. Conventions\n\n   The key words \"MUST\" in this document are to be interpreted as\n   described in RFC 2119.\n"
	adoptsBroken := "1. Conventions\n\n   The key words are to be interpreted as described in BCP\n   14 when they appear in all capitals.\n"
	writeRegisterFixture(t, tree, "rfc5999", prose, lowercase)
	writeRegisterFixture(t, tree, "rfc5998", prose, adopts)
	writeRegisterFixture(t, tree, "rfc5997", prose, adoptsBroken)
	writeRegisterFixture(t, tree, "rfc5996", `{"register": "rfc2119"}`, lowercase)
	writeRegisterFixture(t, tree, "rfc5995", "", lowercase)
	writeRegisterFixture(t, tree, "rfc1034", `{"register": "rfc2119"}`, adopts)
	writeRegisterFixture(t, tree, "draft-ietf-idr-widget-03", prose, lowercase)
	cases := []struct {
		stem string
		want KeywordRegister
	}{
		{"rfc5999", registerLowercase},
		{"rfc5998", registerUpperCase},
		{"rfc5997", registerUpperCase},
		{"rfc5996", registerUpperCase},
		{"rfc5995", registerUpperCase},
		{"rfc1034", registerLowercase},
		{"rfc1035", registerLowercase},
		{"draft-ietf-idr-widget-03", registerUpperCase},
	}
	for _, one := range cases {
		got, err := KeywordRegisterOf(tree, one.stem)
		if err != nil {
			t.Errorf("KeywordRegisterOf(%q) refused: %v", one.stem, err)
			continue
		}
		if got != one.want {
			t.Errorf("KeywordRegisterOf(%q) = %d, want %d", one.stem, got, one.want)
		}
	}

	writeRegisterFixture(t, tree, "rfc5994", prose, "")
	writeRegisterFixture(t, tree, "rfc5993", `{"register":`, lowercase)
	writeRegisterFixture(t, tree, "rfc5992", `{"register": "loud"}`, lowercase)
	refusals := []struct{ stem, want string }{
		{"rfc5994", "whether the document adopts BCP 14 cannot be read"},
		{"rfc5993", "is not JSON"},
		{"rfc5992", "is missing or unknown"},
	}
	for _, one := range refusals {
		got, err := KeywordRegisterOf(tree, one.stem)
		if err == nil || !strings.Contains(err.Error(), one.want) {
			t.Errorf("KeywordRegisterOf(%q) = %d, %v, want a refusal holding %q", one.stem, got, err, one.want)
		}
	}

	if _, keyword := registerLowercase.firstKeyword(rfc2181TTLSentence); keyword != levelMust {
		t.Errorf("the lowercase register reads %q in RFC 2181 Section 5.5, want %q", keyword, levelMust)
	}
	if at, _ := registerUpperCase.firstKeyword(rfc2181TTLSentence); at >= 0 {
		t.Errorf("the upper-case register reads a keyword at %d in a lowercase sentence", at)
	}
}

// TestKeywordRegisterOfOverTheCorpus checks the derivation against the
// documents that motivated it. Goal: RFC 2181, numbered after RFC 2119 with
// an extraction that signs prose and a Section 3 that declines the
// upper-case keywords, reads lowercase must; RFC 4035, whose extraction also
// signs prose but whose text adopts RFC 2119, stays upper case; RFC 1035
// reads lowercase by its number. Method: KeywordRegisterOf over this
// checkout.
func TestKeywordRegisterOfOverTheCorpus(t *testing.T) {
	root := checkoutRoot(t)
	cases := []struct {
		stem string
		want KeywordRegister
	}{
		{"rfc2181", registerLowercase},
		{"rfc4035", registerUpperCase},
		{"rfc1035", registerLowercase},
	}
	for _, one := range cases {
		got, err := KeywordRegisterOf(root, one.stem)
		if err != nil {
			t.Errorf("KeywordRegisterOf(%q) refused: %v", one.stem, err)
			continue
		}
		if got != one.want {
			t.Errorf("KeywordRegisterOf(%q) = %d, want %d", one.stem, got, one.want)
		}
	}
	if !strings.Contains(realRFCText(t, "rfc2181"), "the TTL transmitted in each case must be the same.") {
		t.Error("RFC 2181 no longer holds the Section 5.5 sentence this test reads")
	}
}

// mustKeywordRegister answers KeywordRegisterOf(tree, stem), failing the test
// when the register cannot be read.
func mustKeywordRegister(t *testing.T, tree, stem string) KeywordRegister {
	t.Helper()
	register, err := KeywordRegisterOf(tree, stem)
	if err != nil {
		t.Fatalf("the keyword register of %s cannot be read: %v", stem, err)
	}
	return register
}
