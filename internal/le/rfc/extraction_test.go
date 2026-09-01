// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7 and AC-8 -- the derivation and
// the sign-off reader are called as functions, the envelope is structured data
// with kebab-case keys, and each failure answers its own exit code.
// PREVENTS: a derivation that drops a sentence. Every check in this gate judges
// the requirements a summary LISTS, so a site the walk never produced is an
// obligation no ratchet, no annotation and no test can ever see.

package rfc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/leroot"
)

// fixtureTree writes a checkout the derivation can read and answers its root.
func fixtureTree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}
	return root
}

func TestAPageBreakDoesNotTruncateTheObligationItCrosses(t *testing.T) {
	// The blank lines bracketing a page break read as a paragraph boundary, so
	// removing only the three furniture lines would cut the quote at "the
	// first" and lose the rest of the sentence.
	text := "    A speaker MUST do the first\n\nAuthor            Standards Track           [Page 4]\n\f\n" +
		"RFC 9999            Widgets            October 2026\n\n    and the second thing.\n"
	got := stripPageFurniture(text)
	if strings.Contains(got, "[Page 4]") || strings.Contains(got, "Standards Track") {
		t.Fatalf("the furniture survived: %q", got)
	}
	sentence := sentences(got)
	if len(sentence) != 1 {
		t.Fatalf("the page break split the sentence: %q", sentence)
	}
	if sentence[0] != "A speaker MUST do the first and the second thing." {
		t.Errorf("the rejoined sentence is %q", sentence[0])
	}
}

func TestAHeadingsOwnTitleStaysInItsBody(t *testing.T) {
	// The heading pattern OVER-MATCHES: RFCs put column-0 attribute tables and
	// packet diagrams in the same text stream. Dropping a matched line drops
	// whatever it said, and for a false match that is a live obligation deleted
	// without a word.
	bodies := sectionBodies("1     Exactly one instance of this attribute MUST be present.\n")
	if len(bodies) != 2 {
		t.Fatalf("sections: %+v", bodies)
	}
	if !strings.Contains(bodies[1].body, "MUST be present") {
		t.Errorf("a false heading erased its own sentence: %q", bodies[1].body)
	}
}

func TestARepeatedSectionIdExtendsTheSectionItAlreadyOpened(t *testing.T) {
	// Two entries sharing an id would emit a duplicate row the artifact parser
	// refuses, and each body would restart the per-section site counter at 1 --
	// so both would produce a site "7:1" and one would disappear.
	bodies := sectionBodies("7.  First\n\n    A MUST here.\n\n7.  Again\n\n    B MUST here.\n")
	seen := map[string]int{}
	for _, one := range bodies {
		seen[one.id]++
	}
	if seen["7"] != 1 {
		t.Fatalf("section 7 appears %d times: %+v", seen["7"], bodies)
	}
	sites := sitesFor("7.  First\n\n    A MUST here.\n\n7.  Again\n\n    B MUST here.\n", siteKeywordRE)
	if len(sites) != 2 || sites[0].ID != "7:1" || sites[1].ID != "7:2" {
		t.Errorf("the locators restarted: %+v", sites)
	}
}

func TestTheKeyWordsParagraphIsNotASite(t *testing.T) {
	body := "1.  Terms\n\n    The key words MUST, MUST NOT are to be interpreted as described in RFC 2119.\n"
	if sites := sitesFor(body, siteKeywordRE); len(sites) != 0 {
		t.Errorf("the boilerplate was counted as an obligation: %+v", sites)
	}
}

func TestAnObligationFusedOntoTheKeyWordsParagraphIsCutFree(t *testing.T) {
	// The splitter cannot cut before a digit, which leaves "... RFC 2119. 6PE
	// routers MUST support X." as ONE sentence. Excluding the paragraph would
	// then take the obligation with it, and the gate would read an RFC that
	// asks for nothing.
	fused := "The key words are to be interpreted as described in RFC 2119. " +
		"6PE routers MUST support the widget."
	got := splitOffBoilerplate(fused)
	if len(got) != 2 {
		t.Fatalf("the obligation was not cut free: %q", got)
	}
	if !strings.HasPrefix(got[1], "6PE routers MUST") {
		t.Errorf("the tail is %q", got[1])
	}
}

func TestAFusedObligationStillBecomesASiteInTheInventory(t *testing.T) {
	// The cut has to happen where the SITES are produced, not only where a
	// direct caller asks for it. Without it the whole fused sentence matches
	// the key-words pattern and is dropped, so the gate reads an RFC that asks
	// for nothing -- and a summary that captured nothing then passes every
	// check built on the sites this walk did not produce.
	body := "1.  Terms\n\n    The key words are to be interpreted as described in RFC 2119. " +
		"6PE routers MUST support the widget.\n"
	sites := sitesFor(body, siteKeywordRE)
	if len(sites) != 1 {
		t.Fatalf("the fused obligation produced %d site(s): %+v", len(sites), sites)
	}
	if !strings.HasPrefix(sites[0].Quote, "6PE routers MUST") {
		t.Errorf("the site quotes %q", sites[0].Quote)
	}
}

func TestASentenceOpeningOnALowercaseWordStaysFused(t *testing.T) {
	// Demanding the follower rules out "e.g. the" and "Fig. 3" without an
	// abbreviation list.
	got := sentences("See Fig. 3 for the layout. A speaker MUST send it.")
	if len(got) != 2 {
		t.Fatalf("sentences: %q", got)
	}
	if got[0] != "See Fig. 3 for the layout." {
		t.Errorf("an abbreviation was split: %q", got[0])
	}
}

func TestTheRegisterIsDerivedFromTheSourceAndNeverClaimed(t *testing.T) {
	cases := []struct {
		name                  string
		keyword, prose, gated int
		want                  string
	}{
		{"capitalised keywords covering the checklist", 5, 5, 3, registerRFC2119},
		{"capitalised keywords covering it exactly", 3, 3, 3, registerRFC2119},
		{"fewer keyword sites than declared rows", 2, 9, 3, registerProse},
		{"no capitalised keyword at all", 0, 9, 3, registerProse},
		{"nothing to read either way", 0, 0, 3, registerManualWalk},
		{"no keyword and no declared row", 0, 0, 0, registerManualWalk},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			if got := DeriveRegister(one.keyword, one.prose, one.gated); got != one.want {
				t.Errorf("DeriveRegister(%d, %d, %d) = %q, want %q",
					one.keyword, one.prose, one.gated, got, one.want)
			}
		})
	}
}

func TestNoSourceTextIsNotAnEmptyInventory(t *testing.T) {
	// An empty inventory says "the source states no obligations"; nil says "I
	// could not look", and the two must never render alike.
	inv, err := NewDeriver(t.TempDir()).Inventory("rfc9999", 0)
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if inv != nil {
		t.Errorf("a stem with no source text derived an inventory: %+v", inv)
	}
}

// derivedFixture is the source and summary the inventory cases read.
const derivedFixture = `Test RFC 9999

1.  Introduction

    This document describes widgets.

2.  Widgets

    A speaker MUST send the widget. A speaker SHOULD log it.
`

func TestTheInventoryLocatesEverySiteInItsSection(t *testing.T) {
	tree := fixtureTree(t, map[string]string{"rfc/full/rfc9999.txt": derivedFixture})

	inv, err := NewDeriver(tree).Inventory("rfc9999", 1)
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if inv == nil {
		t.Fatal("the source is present and the inventory is nil")
	}
	if inv.Register != registerRFC2119 {
		t.Errorf("register = %q", inv.Register)
	}
	if len(inv.Sites) != 1 || inv.Sites[0].ID != "2:1" {
		t.Fatalf("sites: %+v", inv.Sites)
	}
	if inv.Sites[0].Quote != "A speaker MUST send the widget." {
		t.Errorf("quote = %q", inv.Sites[0].Quote)
	}
	if len(inv.Sections) != 3 || inv.Sections[0].ID != frontSection {
		t.Fatalf("sections: %+v", inv.Sections)
	}
	if inv.Sections[2].Sites != 1 {
		t.Errorf("section 2 holds %d site(s), want 1", inv.Sections[2].Sites)
	}
	if inv.SourcePath != "rfc/full/rfc9999.txt" {
		t.Errorf("source path = %q", inv.SourcePath)
	}
}

func TestADraftsSourceIsFoundWhenNoFullTextIs(t *testing.T) {
	tree := fixtureTree(t, map[string]string{"rfc/drafts/draft-x.txt": derivedFixture})

	rel, held := SourcePath(tree, "draft-x")
	if !held || rel != "rfc/drafts/draft-x.txt" {
		t.Errorf("SourcePath = %q, %v", rel, held)
	}
}

func TestTheFingerprintSurvivesAReflowAndNotAnEdit(t *testing.T) {
	// A sign-off must not be invalidated by rewrapping the source, and must be
	// invalidated by changing what it says.
	base := RequirementSHA("A speaker MUST send it.\n\nIt is required.\n")
	reflowed := RequirementSHA("  A speaker MUST send it.\n\n\n  It is required.\n")
	edited := RequirementSHA("A speaker MAY send it.\n\nIt is required.\n")
	if base != reflowed {
		t.Errorf("a reflow changed the fingerprint: %s against %s", base, reflowed)
	}
	if base == edited {
		t.Error("an edit left the fingerprint unchanged")
	}
	if len(base) != shaHexLen {
		t.Errorf("the fingerprint is %d characters, want %d", len(base), shaHexLen)
	}
}

func TestAnUnsignedSkeletonParsesAndStillFailsTheCheck(t *testing.T) {
	// An unsigned skeleton is a legal intermediate state. Having the writer
	// invent a date and a reviewer so its own output parses would fabricate a
	// sign-off record for a walk nobody performed.
	tree := fixtureTree(t, map[string]string{
		"rfc/full/rfc9999.txt": derivedFixture,
		"rfc/extraction/rfc9999.json": `{"schema-version": 1, "stem": "rfc9999",
 "register": "rfc2119", "source-path": "rfc/full/rfc9999.txt",
 "source-sha": "` + RequirementSHA(derivedFixture) + `",
 "signed-off": "", "reviewer": "",
 "sections": [{"id": "front", "sites": 0, "disposition": null},
              {"id": "1", "sites": 0, "disposition": null},
              {"id": "2", "sites": 1, "disposition": null}],
 "sites": [{"id": "2:1", "quote": "A speaker MUST send the widget.", "disposition": null}]}`,
	})

	art, err := ParseExtractionArtifact(tree, filepath.Join(tree, "rfc", "extraction", "rfc9999.json"))
	if err != nil {
		t.Fatalf("a skeleton must PARSE: %v", err)
	}
	if art.SignedOff != "" || art.Reviewer != "" {
		t.Errorf("the skeleton claims a sign-off: %+v", art)
	}

	signed, errs, err := evaluateExtractions(NewDeriver(tree), nil)
	if err != nil {
		t.Fatalf("EvaluateExtractions: %v", err)
	}
	if len(signed) != 0 {
		t.Errorf("a skeleton earned credit: %v", signed)
	}
	joined := strings.Join(errs, "\n")
	for _, want := range []string{"'signed-off' is empty", "'reviewer' is empty", "is UNCLASSIFIED"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the check does not say %q:\n%s", want, joined)
		}
	}
}

// VALIDATES: the exclusion vocabulary is a CLOSED set that now holds
// feature-out-of-scope. A site carrying it parses, and a kind nobody declared
// is still refused with the legal set printed in the message.
// PREVENTS: two failures at once. Without the kind, an obligation conditional
// on an optional feature Ze does not offer is written as binds-another-role,
// which asserts Ze plays no such role while Ze is exactly the role; or as a
// {gap}, which puts work Ze never owed on the public ledger. Without the
// refusal arm the set stops being closed, and every unmapped site becomes
// excludable by inventing a word for it.
func TestTheExclusionVocabularyTakesFeatureOutOfScopeAndRefusesAnInventedKind(t *testing.T) {
	for _, testcase := range []struct {
		name string
		kind string
		want string
	}{
		{"a feature Ze decided not to offer", "feature-out-of-scope", ""},
		{"a kind the closed set does not hold", "out-of-scope", "'excluded-kind' from"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			tree := fixtureTree(t, map[string]string{
				"rfc/full/rfc9999.txt": derivedFixture,
				"rfc/extraction/rfc9999.json": `{"schema-version": 1, "stem": "rfc9999",
 "register": "rfc2119", "source-path": "rfc/full/rfc9999.txt",
 "source-sha": "` + RequirementSHA(derivedFixture) + `",
 "signed-off": "2026-08-31", "reviewer": "the vocabulary test",
 "sections": [{"id": "front", "sites": 0, "disposition": "skipped",
               "skip-kind": "front-matter", "reason": "the title block"},
              {"id": "1", "sites": 0, "disposition": "walked"},
              {"id": "2", "sites": 1, "disposition": "walked"}],
 "sites": [{"id": "2:1", "quote": "A speaker MUST send the widget.",
            "disposition": "excluded", "excluded-kind": "` + testcase.kind + `",
            "reason": "the widget is optional and Ze does not offer it"}]}`,
			})

			art, err := ParseExtractionArtifact(tree, filepath.Join(tree, "rfc", "extraction", "rfc9999.json"))
			if testcase.want != "" {
				if err == nil {
					t.Fatalf("%q parsed, so the set is not closed: %+v", testcase.kind, art)
				}
				if !strings.Contains(err.Error(), testcase.want) {
					t.Errorf("the refusal does not say %q: %v", testcase.want, err)
				}
				for _, kind := range ExclusionKinds() {
					if !strings.Contains(err.Error(), kind) {
						t.Errorf("the refusal does not name the legal kind %q: %v", kind, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("%q must parse: %v", testcase.kind, err)
			}
			if len(art.Sites) != 1 {
				t.Fatalf("the artifact carries %d site(s), want 1", len(art.Sites))
			}
			if art.Sites[0].ExcludedKind != testcase.kind {
				t.Errorf("the site kind is %q, want %q", art.Sites[0].ExcludedKind, testcase.kind)
			}
		})
	}
}

func TestCreditIsScopedToTheEnrolledSet(t *testing.T) {
	// The floor once compared every valid artifact against a backlog of
	// enrolled ones, so a sign-off for a stem nobody enrolled raised the credit
	// without lowering the backlog.
	valid := map[string]Extraction{
		"rfc1": {Stem: "rfc1", Register: registerProse},
		"rfc2": {Stem: "rfc2", Register: registerRFC2119},
	}
	signed := credited(valid, map[string]bool{"rfc1": true})
	if len(signed) != 1 {
		t.Fatalf("credited: %v", signed)
	}
	counts := registerCounts(signed)
	if counts[registerProse] != 1 || counts[registerRFC2119] != 0 {
		t.Errorf("the split is %v", counts)
	}
	for _, name := range Registers() {
		if _, held := counts[name]; !held {
			t.Errorf("register %q is missing from the split rather than zero", name)
		}
	}
}

func TestTheEnvelopeIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	status := Status{
		SchemaVersion: 1, Enrolled: 3, Signed: 1,
		SignedByRegister: map[string]int{registerRFC2119: 1, registerProse: 0, registerManualWalk: 0},
		Relocated:        2, Backlog: 2, Unsigned: []string{"rfc1", "rfc2"},
	}

	raw, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("the envelope does not encode: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("the envelope is not an object: %v", err)
	}
	for _, key := range []string{"schema-version", "enrolled", "signed", "signed-by-register",
		"relocated", "backlog", "unsigned"} {
		if _, held := decoded[key]; !held {
			t.Errorf("the envelope has no %q key: %v", key, decoded)
		}
	}
	if len(decoded) != 7 {
		t.Errorf("the envelope holds %d keys, want 7: %v", len(decoded), decoded)
	}
}

func TestTheDefaultRenderingIsTheShapeTheConsumerReads(t *testing.T) {
	// The envelope's only consumer reads two-space indented, sorted-key JSON.
	// A command that answered Go's compact encoding by default would have moved
	// the contract while passing every verdict comparison.
	status := Status{
		SchemaVersion: 1, Enrolled: 1, Signed: 0,
		SignedByRegister: map[string]int{registerRFC2119: 0, registerProse: 0, registerManualWalk: 0},
		Backlog:          1, Unsigned: []string{"rfc9999"},
	}
	want := `{
  "backlog": 1,
  "enrolled": 1,
  "relocated": 0,
  "schema-version": 1,
  "signed": 0,
  "signed-by-register": {
    "manual-walk": 0,
    "prose": 0,
    "rfc2119": 0
  },
  "unsigned": [
    "rfc9999"
  ]
}
`
	if got := status.Text(); got != want {
		t.Errorf("the default rendering is\n%s\nwant\n%s", got, want)
	}
	if _, isProse := any(status).(leroot.Prose); !isProse {
		t.Error("the envelope does not carry its own default rendering")
	}
}

func TestTheTwoRenderingsOfTheEnvelopeCarryTheSameKeys(t *testing.T) {
	// The page spells the keys a second time, so this is what stops a tag
	// renamed on one side leaving `| json` and the default page disagreeing
	// about what the answer holds.
	status := Status{SignedByRegister: map[string]int{}, Unsigned: []string{}}

	raw, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("the envelope does not encode: %v", err)
	}
	var encoded map[string]any
	if err := json.Unmarshal(raw, &encoded); err != nil {
		t.Fatalf("the envelope is not an object: %v", err)
	}
	page := status.document()
	if len(page) != len(encoded) {
		t.Fatalf("the page holds %d keys and the encoder holds %d", len(page), len(encoded))
	}
	for key := range encoded {
		if _, held := page[key]; !held {
			t.Errorf("the page has no %q key, and `| json` does", key)
		}
	}
}

func TestAnEmptyBacklogRendersAsAnEmptyList(t *testing.T) {
	// A missing list and an empty one are different facts, and the consumer
	// reads the key either way.
	status := Status{
		SchemaVersion: 1, SignedByRegister: map[string]int{}, Unsigned: []string{},
	}
	if !strings.Contains(status.Text(), `"unsigned": []`) {
		t.Errorf("an empty backlog renders as %s", status.Text())
	}
}

func TestEachFailureAnswersItsOwnExitCode(t *testing.T) {
	// A tree with no rfc/ at all is a tree the gate could not judge, which is
	// the script's exit 2 rather than a clean envelope.
	tree := fixtureTree(t, map[string]string{"go.mod": "module x\n", "feature-gates.txt": ""})
	t.Setenv("ZE_REPO_ROOT", tree)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	if _, code := Answer([]string{"extraction-status"}); code != 2 {
		t.Errorf("a tree with no workflows answered %d, want 2", code)
	}
	if _, code := Answer([]string{"no-such-action"}); code != 2 {
		t.Errorf("an action this area does not hold answered %d, want 2", code)
	}
	if _, code := Answer([]string{"extraction-status", "rfc7606"}); code != 2 {
		t.Errorf("a value after an action that takes none answered %d, want 2", code)
	}
	listing, code := Answer(nil)
	if code != 0 {
		t.Errorf("the bare command answered %d, want 0", code)
	}
	if _, err := json.Marshal(listing); err != nil {
		t.Errorf("the listing is not structured data: %v", err)
	}
}

func TestEveryActionOfTheAreaCarriesItsGateAndItsReason(t *testing.T) {
	list := Actions()
	if list.Area != area {
		t.Errorf("the listing names %q", list.Area)
	}
	if len(list.Actions) == 0 {
		t.Fatal("the area holds no action")
	}
	writes := map[string]bool{}
	for _, row := range list.Actions {
		if row.Why == "" {
			t.Errorf("action %q carries no reason: %+v", row.Verb, row)
		}
		if row.Writes {
			writes[row.Verb] = true
		}
	}
	// Exactly four actions change the tree, and each one owns its output:
	// extraction-create owns one rfc/extraction artifact, discriminate-record
	// owns one rfc/discrimination artifact, re-seal owns rfc/audit/, and the
	// generator owns ai/RFC-REQUIREMENTS.md plus rfc/requirements/. Read-only is
	// the default and the listing prints the exception, so a reader never has to
	// look it up.
	if len(writes) != 4 || !writes["extraction-create"] || !writes["discriminate-record"] ||
		!writes["reseal"] || !writes["index-update"] {
		t.Errorf("the actions that write are %v, want exactly "+
			"[discriminate-record extraction-create index-update reseal]", sortedKeys(writes))
	}
	if Subs() == "" {
		t.Error("help renders no hint under the command")
	}
}

func TestAFusedObligationWithNoTerminatorIsStillCutFree(t *testing.T) {
	// The sibling above has a terminator after "RFC 2119", so boilerplateEnd
	// finds one and the cut is taken. This one does not: the paragraph runs
	// straight into the obligation with no end punctuation anywhere after the
	// match. The splitter used to give up there and hand the chunk back WHOLE,
	// and sitesFor drops a boilerplate-matching chunk entire, so the MUST inside
	// it never became a site.
	//
	// That is the one direction nothing downstream can see. An over-count is
	// visible to a reviewer, who deletes the row. An under-count is silent: the
	// gate cannot ask for evidence of an obligation it never knew was owed.
	fused := "The key words are to be interpreted as described in RFC 2119 " +
		"and 6PE routers MUST support the widget"

	got := splitOffBoilerplate(fused)

	if len(got) != 2 {
		t.Fatalf("the obligation was swallowed with the boilerplate: %q", got)
	}
	if !strings.Contains(got[1], "MUST support the widget") {
		t.Errorf("the tail is %q, and must still state the obligation", got[1])
	}
}

func TestATerminatorLessChunkWithNoObligationIsStillDroppedWhole(t *testing.T) {
	// The discrimination case. The cut is conditioned on the TAIL carrying a
	// MUST-level keyword, so a chunk that is only boilerplate must not be split:
	// splitting it would promote the paragraph's own keyword listing to an
	// obligation, which is the over-count the condition exists to avoid.
	onlyBoilerplate := "The key words are to be interpreted as described in RFC 2119 " +
		"and in RFC 8174 when they appear in all capitals"

	got := splitOffBoilerplate(onlyBoilerplate)

	if len(got) != 1 {
		t.Fatalf("a chunk stating no obligation was split: %q", got)
	}
}

func TestATerminatorLessFusedObligationBecomesASite(t *testing.T) {
	// The same case as the first test above, driven through the walk that
	// actually feeds the gate rather than through the splitter alone.
	body := "1.  Terms\n\n    The key words are to be interpreted as described in RFC 2119 " +
		"and 6PE routers MUST support the widget\n"

	sites := sitesFor(body, siteKeywordRE)

	if len(sites) != 1 {
		t.Fatalf("the obligation never became a site: %+v", sites)
	}
	if !strings.Contains(sites[0].Quote, "MUST support the widget") {
		t.Errorf("the site quote is %q", sites[0].Quote)
	}
}

// calendarDay parses a YYYY-MM-DD test date, so a case below reads as the calendar day it
// means rather than as a time.Date call the reader has to decode.
func calendarDay(t *testing.T, day string) time.Time {
	t.Helper()

	parsed, err := time.Parse("2006-01-02", day)
	if err != nil {
		t.Fatalf("test date %q: %v", day, err)
	}
	return parsed
}

// drainBudgetTree writes a checkout carrying nothing but the drain policy, which is the
// only file parseDrainBudget and checkDrainFloor read.
func drainBudgetTree(t *testing.T, body string) string {
	t.Helper()

	return fixtureTree(t, map[string]string{"rfc/drain-budget.txt": body})
}

// floorCase is one calendar day and the number of WHOLE months that have passed since
// start on it. Every months field is counted by hand from the calendar and none is taken
// from what requiredFloor answers: a case copied from the producer asserts only that the
// code does what it does.
type floorCase struct {
	start  string
	today  string
	months int
	why    string
}

// VALIDATES: AC-2 -- requiredFloor counts a calendar month only once it is COMPLETE, so the
// day before an anniversary owes what the month before it owed, and a today that precedes
// start owes nothing rather than a negative number.
// PREVENTS: an off-by-one that bills the tree a month early. The floor is cumulative, so
// one month too many is one sign-off too many on every later day, and the first evidence
// would arrive as a red gate on a session doing unrelated work.
//
// By hand from the calendar, with start on 2026-01-15: the anniversaries are 2026-02-15,
// 2026-03-15 and 2026-04-15. The rate is 1 entry per month and the enrolled cap is 100, so
// the floor IS the month count and each want below reads as the calendar.
func TestRequiredFloorExcludesTheIncompleteMonth(t *testing.T) {
	cases := []floorCase{
		{"2026-01-15", "2026-01-14", 0, "the day before start: no month has begun, and the count clamps at zero rather than going negative"},
		{"2026-01-15", "2026-01-15", 0, "the start day itself: the first month has begun and is not whole"},
		{"2026-01-15", "2026-02-14", 0, "one day before the first anniversary"},
		{"2026-01-15", "2026-02-15", 1, "the first anniversary: 15 January to 15 February is one whole month"},
		{"2026-01-15", "2026-04-14", 2, "one day before the third anniversary: February and March are whole, April is not"},
		{"2026-01-15", "2026-04-15", 3, "the third anniversary"},
	}
	for _, c := range cases {
		floor := requiredFloor(calendarDay(t, c.start), 1, 100, calendarDay(t, c.today))
		if floor != c.months {
			t.Errorf("start %s, today %s: the floor is %d, want %d (%s)",
				c.start, c.today, floor, c.months, c.why)
		}
	}
}

// VALIDATES: AC-3 -- a start on a day the target month does not have clamps to that month's
// last day, and the month counts as whole on it.
// PREVENTS: a schedule that skips a month whenever the anniversary falls off the end of the
// calendar. A start on the 31st has no anniversary in February, April, June, September or
// November, so without the clamp seven months of a year would each count one day late and
// the drain clock would drift behind the wall clock for as long as the schedule stands.
//
// By hand, from a start on 31 January 2026: February 2026 has 28 days and ends the first
// whole month on the 28th, March ends the second on the 31st, and April has 30 days and
// ends the third on the 30th. The two later starts test the same clamp where the last day
// of the month is derived rather than fixed: February 2024 has 29 days, and a December
// today is the case where the derivation asks for month 13 of the year.
func TestRequiredFloorClampsTheAnniversaryToTheShortMonth(t *testing.T) {
	cases := []floorCase{
		{"2026-01-31", "2026-02-27", 0, "one day before February ends, so no whole month has passed"},
		{"2026-01-31", "2026-02-28", 1, "February is whole on its last day, which is as close to the 31st as February goes"},
		{"2026-01-31", "2026-03-30", 1, "March has a 31st, so the second month is not whole until it arrives"},
		{"2026-01-31", "2026-03-31", 2, "the second anniversary, unclamped"},
		{"2026-01-31", "2026-04-29", 2, "one day before April ends"},
		{"2026-01-31", "2026-04-30", 3, "April has 30 days, so the third month is whole on the 30th"},
		{"2026-01-31", "2026-05-31", 4, "the fourth anniversary, unclamped"},
		{"2024-01-31", "2024-02-28", 0, "2024 is a leap year, so February runs one day longer and the month is not whole yet"},
		{"2024-01-31", "2024-02-29", 1, "the leap day is February's last, so the first month is whole on it"},
		{"2026-11-30", "2026-12-29", 0, "one day before the anniversary, in the month whose last day is read across the year boundary"},
		{"2026-11-30", "2026-12-30", 1, "December has a 30th, so no clamp applies and the month is whole on it"},
	}
	for _, c := range cases {
		floor := requiredFloor(calendarDay(t, c.start), 1, 100, calendarDay(t, c.today))
		if floor != c.months {
			t.Errorf("start %s, today %s: the floor is %d, want %d (%s)",
				c.start, c.today, floor, c.months, c.why)
		}
	}
}

// VALIDATES: AC-4 -- the floor caps at the WHOLE enrolled set, and checkDrainFloor is the
// caller that hands it that number rather than the remaining backlog.
// PREVENTS: a remainder cap, which counts every sign-off twice. It raises the cumulative
// total AND lowers the bar that total is measured against, collapsing the comparison to
// signed >= enrolled / 2 whatever the rate says.
func TestRequiredFloorCapsAtTheEnrolledSet(t *testing.T) {
	// By hand: 1 January 2026 to 1 January 2027 is twelve whole calendar months, the
	// anniversary falling on the first of each month and the twelfth landing on today. At
	// rate 3 the schedule demands ceil(3 x 12) = 36 sign-offs and 10 RFCs are enrolled, so
	// the floor is 10.
	const enrolled = 10
	today := calendarDay(t, "2027-01-01")
	if floor := requiredFloor(calendarDay(t, "2026-01-01"), 3, enrolled, today); floor != enrolled {
		t.Errorf("twelve months at rate 3 over %d enrolled RFC(s) answered a floor of %d, want %d",
			enrolled, floor, enrolled)
	}

	// Four of the ten are signed, so the remaining backlog is 6. A backlog cap would demand
	// 6 and report itself satisfied two walks sooner, so the property is asserted at the
	// call site that decides which number the cap is.
	enrolledSet := map[string]bool{}
	for _, stem := range []string{"rfc1", "rfc2", "rfc3", "rfc4", "rfc5",
		"rfc6", "rfc7", "rfc8", "rfc9", "rfc10"} {
		enrolledSet[stem] = true
	}
	signed := map[string]Extraction{}
	for _, stem := range []string{"rfc1", "rfc2", "rfc3", "rfc4"} {
		signed[stem] = Extraction{Stem: stem, Register: registerRFC2119}
	}

	errs := checkDrainFloor(drainBudgetTree(t, "start 2026-01-01\nrate 3\n"), enrolledSet, signed, today)
	if len(errs) != 1 {
		t.Fatalf("an under-quota corpus answered %d violation(s), want 1: %v", len(errs), errs)
	}
	for _, want := range []string{
		"requires 10 extraction sign-off(s) by now",
		"capped at the 10 enrolled RFC(s)",
		"and there are 4 (rfc2119 4, prose 0, manual-walk 0",
		"leaving 6 unsigned",
	} {
		if !strings.Contains(errs[0], want) {
			t.Errorf("the drain violation does not name %q:\n%s", want, errs[0])
		}
	}
}

// VALIDATES: an absent drain policy is a REFUSAL, and checkDrainFloor turns that refusal
// into a violation rather than a silent pass.
// PREVENTS: the zero value read as an answer (ai/rules/principles.md). A drainBudget{}
// carries rate 0 and a zero start date, which computes a floor of 0 and passes any backlog,
// so a deleted policy file would read as "nothing owed" and the gate would go green on the
// day the schedule disappeared.
func TestParseDrainBudgetRefusesAnAbsentFile(t *testing.T) {
	root := fixtureTree(t, nil)

	budget, err := parseDrainBudget(root)
	if err == nil {
		t.Fatalf("an absent policy parsed into %+v", budget)
	}
	for _, want := range []string{
		"rfc/drain-budget.txt",
		"cannot read the drain policy",
		"does NOT mean 'nothing owed'",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}

	// The refusal has to reach the gate, so the caller is driven too: one enrolled RFC, no
	// sign-off, and a corpus that a floor of zero would have passed without a word.
	errs := checkDrainFloor(root, map[string]bool{"rfc1": true}, nil, calendarDay(t, "2026-08-31"))
	if len(errs) != 1 {
		t.Fatalf("an absent policy answered %d violation(s), want 1: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "cannot read the drain policy") {
		t.Errorf("the gate does not report the unreadable policy:\n%s", errs[0])
	}
}

// VALIDATES: the file carries POLICY ONLY, so a row naming an RFC, a count, a stem list or
// a register is refused at the line that carries it.
// PREVENTS: the hand-kept registry the 2026-07-29 resolution rejected. Who has been signed
// off is derived from rfc/extraction/*.json, and a per-stem row here would be a second
// declaration of that fact with nothing to arbitrate the two.
func TestParseDrainBudgetRefusesAStemRow(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"an RFC with a count", "start 2026-07-29\nrate 1\nrfc9999 1\n"},
		{"a derived total", "start 2026-07-29\nrate 1\nsigned 6\n"},
		{"a stem list", "start 2026-07-29\nrate 1\nstems rfc1,rfc2\n"},
		{"a register column on the rate", "start 2026-07-29\nrate 1 rfc2119\n"},
	}
	for _, c := range cases {
		budget, err := parseDrainBudget(drainBudgetTree(t, c.body))
		if err == nil {
			t.Errorf("%s parsed into %+v", c.name, budget)
			continue
		}
		for _, want := range []string{
			"POLICY ONLY",
			"may never name an RFC, hold a count, or list stems",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: the refusal does not say %q: %v", c.name, want, err)
			}
		}
	}
}

// VALIDATES: the rate boundary below the schedule -- -0.001 is refused as negative, and 0
// stays accepted.
// PREVENTS: a backlog that un-drains. A negative rate lowers the floor as the months pass,
// so a tree that ever met the schedule could never fail it again.
func TestParseDrainBudgetRefusesANegativeRate(t *testing.T) {
	budget, err := parseDrainBudget(drainBudgetTree(t, "start 2026-07-29\nrate -0.001\n"))
	if err == nil {
		t.Fatalf("a negative rate parsed into %+v", budget)
	}
	for _, want := range []string{"rate -0.001", "is negative; a backlog cannot un-drain"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}

	// Zero is the last valid value below it, and it has to stay valid: the file ships at
	// rate 0 (owner decision D5) and every other check test reads that budget.
	if _, err := parseDrainBudget(drainBudgetTree(t, "start 2026-07-29\nrate 0\n")); err != nil {
		t.Errorf("the shipped inert budget was refused: %v", err)
	}
}

// VALIDATES: the rate boundary above the enrolled count -- a rate the whole corpus cannot
// supply is refused outright, and a rate equal to the enrolled count is not.
// PREVENTS: a schedule nobody can meet, which reds the gate on every commit for as long as
// it stands and ends in the rule being deleted rather than obeyed.
func TestTheDrainFloorRefusesARateTheEnrolledSetCannotMeet(t *testing.T) {
	enrolled := map[string]bool{"rfc1": true, "rfc2": true}
	today := calendarDay(t, "2026-08-31")

	errs := checkDrainFloor(drainBudgetTree(t, "start 2026-07-29\nrate 2.001\n"), enrolled, nil, today)
	if len(errs) != 1 {
		t.Fatalf("an unmeetable rate answered %d violation(s), want 1: %v", len(errs), errs)
	}
	for _, want := range []string{"rate 2.001 exceeds the whole enrolled set (2)", "no schedule can be met"} {
		if !strings.Contains(errs[0], want) {
			t.Errorf("the refusal does not say %q:\n%s", want, errs[0])
		}
	}

	// A rate equal to the enrolled count is the last valid one. It demands the whole corpus
	// after one whole month, which is punishing and arithmetically reachable, so the
	// diagnostic is the ordinary shortfall rather than the refusal above.
	errs = checkDrainFloor(drainBudgetTree(t, "start 2026-07-29\nrate 2\n"), enrolled, nil, today)
	if len(errs) != 1 {
		t.Fatalf("a rate equal to the enrolled count answered %d violation(s), want 1: %v", len(errs), errs)
	}
	if strings.Contains(errs[0], "no schedule can be met") {
		t.Errorf("a rate equal to the enrolled count was refused as unmeetable:\n%s", errs[0])
	}
	if !strings.Contains(errs[0], "requires 2 extraction sign-off(s) by now") {
		t.Errorf("the shortfall does not name the floor of 2:\n%s", errs[0])
	}
}

// VALIDATES: a section's own name is the opening sentence of its reason, and
// prose about the walk is not mistaken for one.
//
// A caller that prints "Section 3" owes the reader "Constructing the Next Hop
// field" beside it. The convention every record in rfc/extraction/ follows is
// that the reason opens with the section's title; where a reviewer opened with
// an account of the walk instead, there is no title to print.
func TestASectionTitleIsTheOpeningSentenceOfItsReason(t *testing.T) {
	for _, one := range []struct{ reason, want string }{
		{"Constructing the Next Hop field. The only section that binds a BGP speaker.",
			"Constructing the Next Hop field"},
		{"Introduction.", "Introduction"},
		{"", ""},
		{"Walked the whole section: state loss after a crash, INITIAL_CONTACT, liveness " +
			"detection, retransmission, and Child SA deletion.", ""},
	} {
		section := ExtractionSection{ID: "3", Reason: one.reason}
		if got := section.Title(); got != one.want {
			t.Errorf("the reason %q titles as %q, want %q", one.reason, got, one.want)
		}
	}
}

// VALIDATES: over the real corpus, a section title stays a title.
//
// The method is every record this checkout carries, so a reviewer who writes a
// reason in another shape is caught here rather than on a published page.
func TestEveryDerivedSectionTitleReadsAsATitle(t *testing.T) {
	extractions, err := LoadExtractions(checkoutRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	titled := 0
	for stem, extraction := range extractions {
		for _, section := range extraction.Sections {
			title := section.Title()
			if title == "" {
				continue
			}
			titled++
			if strings.HasSuffix(title, ".") {
				t.Errorf("%s section %s titles as %q, which keeps its full stop",
					stem, section.ID, title)
			}
			if len(title) > sectionTitleMax {
				t.Errorf("%s section %s titles as %q, %d characters",
					stem, section.ID, title, len(title))
			}
		}
	}
	if titled == 0 {
		t.Fatal("no section of this checkout derives a title, so this proves nothing")
	}
	t.Logf("%d section(s) carry a derived title", titled)
}
