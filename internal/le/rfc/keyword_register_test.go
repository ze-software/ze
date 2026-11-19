package rfc

import (
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

// TestKeywordRegisterOfReadsTheRFCNumber checks the one fact the register is
// derived from. Goal: an RFC numbered below 2119 reads lowercase must, and RFC
// 2119 itself, every later RFC and every draft read upper case alone. Method:
// keywordRegisterOf over each kind of stem, the boundary on both sides of it
// included, and stems that carry no RFC number.
func TestKeywordRegisterOfReadsTheRFCNumber(t *testing.T) {
	cases := []struct {
		stem string
		want keywordRegister
	}{
		{"rfc792", registerPre2119},
		{"rfc1035", registerPre2119},
		{"rfc2118", registerPre2119},
		{"rfc2119", registerUpperCase},
		{"rfc4035", registerUpperCase},
		{"draft-ietf-idr-widget-03", registerUpperCase},
		{"rfc", registerUpperCase},
		{"rfc12a", registerUpperCase},
		{"rfc99999999999999999999999", registerUpperCase},
	}
	for _, one := range cases {
		if got := keywordRegisterOf(one.stem); got != one.want {
			t.Errorf("keywordRegisterOf(%q) = %d, want %d", one.stem, got, one.want)
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
			register keywordRegister
			want     string
		}{{registerPre2119, one.pre}, {registerUpperCase, one.upper}} {
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
	parsed, err := parseAnnotation(payload(must), "fixture", registerPre2119)
	if err != nil {
		t.Fatalf("a pre-RFC 2119 shape (a) row was refused: %v", err)
	}
	if parsed.BoundRole != "a secondary server" || parsed.Quote != must {
		t.Errorf("parsed %+v, want the quote and the bound role", parsed)
	}
	row := strings.TrimSuffix(must, ".")
	if refusal, refused := ownObligationRefusal(row, levelMust, must, registerPre2119); refused {
		t.Errorf("a pre-RFC 2119 row's own lowercase obligation was refused: %s", refusal)
	}
	if _, err := parseAnnotation(payload(must), "fixture", registerUpperCase); err == nil ||
		!strings.Contains(err.Error(), "holds no BCP 14 keyword") {
		t.Errorf("a post-RFC 2119 row quoting lowercase must was not refused as keyword-less: %v", err)
	}
	if _, refused := ownObligationRefusal(row, levelMust, must, registerUpperCase); !refused {
		t.Error("a post-RFC 2119 row's lowercase must was read as its MUST")
	}
	for _, register := range []keywordRegister{registerPre2119, registerUpperCase} {
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
	parsed, err := parseAnnotation(good, "fixture", registerPre2119)
	if err != nil {
		t.Fatalf("the RFC 1035 Section 4.2 annotation was refused: %v", err)
	}
	if parsed.RoleSection != "2.2" || parsed.RoleSentence != secondaryServerSentence {
		t.Errorf("parsed role sentence %q from §%q, want %q from §2.2", parsed.RoleSentence, parsed.RoleSection, secondaryServerSentence)
	}
	refusals := []struct {
		name     string
		payload  string
		register keywordRegister
		want     string
	}{
		{"a post-RFC 2119 RFC", payload("Zone refresh activities MUST use virtual circuits.", "a secondary server", "2.2", secondaryServerSentence),
			registerUpperCase, "only an RFC published before RFC 2119"},
		{"a role the sentence does not name", payload(zoneRefreshSentence, "a zone master", "2.2", secondaryServerSentence),
			registerPre2119, "which is not in its section 2.2 sentence"},
		{"a negated sentence", payload(zoneRefreshSentence, "a secondary server", "2.2", "Secondary servers do not acquire zones from the primary server."),
			registerPre2119, "carve the role out"},
		{"a subject that is the role", payload("A secondary server must use virtual circuits for zone refresh.", "a secondary server", "2.2", secondaryServerSentence),
			registerPre2119, "redundant"},
		{"a keyword-less quote", payload("This section applies only to secondary servers.", "a secondary server", "2.2", secondaryServerSentence),
			registerPre2119, "scoping sentence names its own role"},
		{"a section of no heading shape", payload(zoneRefreshSentence, "a secondary server", "2.x", secondaryServerSentence),
			registerPre2119, "no section id"},
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
	if refusal, refused := ownObligationRefusal(row, levelMust, zoneRefreshSentence, registerPre2119); refused {
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
