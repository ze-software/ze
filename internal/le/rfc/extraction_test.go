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
	// Exactly three ported actions change the tree, and each one owns its
	// output: extraction-create owns one rfc/extraction artifact, re-seal owns
	// rfc/audit/, and the generator owns ai/RFC-REQUIREMENTS.md plus
	// rfc/requirements/. Read-only is the default and the listing prints the
	// exception, so a reader never has to look it up.
	if len(writes) != 3 || !writes["extraction-create"] || !writes["reseal"] ||
		!writes["index-update"] {
		t.Errorf("the actions that write are %v, want exactly [extraction-create index-update reseal]",
			sortedKeys(writes))
	}
	if Subs() == "" {
		t.Error("help renders no hint under the command")
	}
}
