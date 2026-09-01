package rfc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
)

// This source and the expected bytes below are the native writer's committed
// fixture. Keeping both exact makes later rendering changes explicit.
const extractionCreateSource = `Network Working Group                                          A. Tester
Request for Comments: 9999                                  October 2026


1.  Introduction

   The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT",
   "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this
   document are to be interpreted as described in RFC 2119.

2.  Requirements

   A speaker MUST do the first thing.  A speaker MUST NOT do the second
   thing.

3.  References

   Nothing normative lives here.
`

const extractionCreateSummary = `# RFC 9999

## Compliance Checklist

- [ ] [RFC9999-2-1] [MUST] A speaker MUST do the first thing (§2)
- [ ] [RFC9999-2-2] [MUST NOT] A speaker MUST NOT do the second thing (§2)
`

func extractionCreateTree(t *testing.T) string {
	t.Helper()
	return fixtureTree(t, map[string]string{
		"feature-gates.txt":    "",
		"go.mod":               "module fixture\n",
		"rfc/full/rfc9999.txt": extractionCreateSource,
		"rfc/short/rfc9999.md": extractionCreateSummary,
	})
}

func TestExtractionCreateWritesTheCommittedNativeSkeletonBytes(t *testing.T) {
	tree := extractionCreateTree(t)
	report, err := createExtraction(tree, "rfc9999")
	if err != nil {
		t.Fatalf("CreateExtraction: %v", err)
	}
	if report.Register != registerRFC2119 || report.Sites != 2 || report.Sections != 4 {
		t.Fatalf("the report is %+v", report)
	}
	if report.UnclassifiedSites != 2 || report.UnclassifiedSections != 4 {
		t.Errorf("the unclassified census is %+v", report)
	}
	if report.Placed || report.Destination != "rfc/extraction/rfc9999.json" {
		t.Fatalf("an unclassified skeleton claimed a place in the corpus: %+v", report)
	}
	wantStatus := "wrote " + report.Path + ": register rfc2119, 2 site(s) in 4 section(s).\n" +
		"2 site(s) and 4 section(s) are UNCLASSIFIED, so the skeleton was NOT written to " +
		"rfc/extraction/rfc9999.json.\n" +
		"Classify every one by hand in the file above, then move it in:\n" +
		"  mv " + report.Path + " rfc/extraction/rfc9999.json\n" +
		"Generation cannot produce a sign-off; only a walk can, and an unclassified artifact " +
		"under rfc/extraction/ fails `./le rfc check` for the whole corpus.\n"
	if report.Text() != wantStatus {
		t.Errorf("the writer status is %q, want %q", report.Text(), wantStatus)
	}

	body, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(report.Path)))
	if err != nil {
		t.Fatalf("read skeleton: %v", err)
	}
	want := fmt.Sprintf(`{
  "schema-version": 1,
  "stem": "rfc9999",
  "register": "rfc2119",
  "source-path": "rfc/full/rfc9999.txt",
  "source-sha": "%s",
  "signed-off": "",
  "reviewer": "",
  "sections": [
    {
      "id": "front",
      "sites": 0,
      "disposition": null
    },
    {
      "id": "1",
      "sites": 0,
      "disposition": null
    },
    {
      "id": "2",
      "sites": 2,
      "disposition": null
    },
    {
      "id": "3",
      "sites": 0,
      "disposition": null
    }
  ],
  "sites": [
    {
      "id": "2:1",
      "quote": "A speaker MUST do the first thing.",
      "disposition": null
    },
    {
      "id": "2:2",
      "quote": "A speaker MUST NOT do the second thing.",
      "disposition": null
    }
  ]
}
`, RequirementSHA(extractionCreateSource))
	if string(body) != want {
		t.Errorf("skeleton bytes differ from the committed native fixture:\nwant:\n%s\ngot:\n%s", want, body)
	}

	artifact, err := ParseExtractionArtifact(tree, filepath.Join(tree, filepath.FromSlash(report.Path)))
	if err != nil {
		t.Fatalf("the production parser refused the skeleton: %v", err)
	}
	if artifact.Mapped() != 0 || artifact.Excluded() != 0 {
		t.Errorf("generation invented a disposition: %+v", artifact.Sites)
	}
	inventory, err := NewDeriver(tree).Inventory("rfc9999", 2)
	if err != nil || inventory == nil {
		t.Fatalf("re-derive generated skeleton: %v, %+v", err, inventory)
	}
	if errs := evaluateExtraction(tree, artifact, inventory, nil); len(errs) == 0 {
		t.Error("a freshly generated skeleton passed the sign-off check")
	}
}

func TestExtractionCreatePreservesEveryAuthoredDispositionField(t *testing.T) {
	inventory := &Inventory{
		Stem: "rfc9999", Register: registerRFC2119,
		SourcePath: "rfc/full/rfc9999.txt", SourceSHA: "abc",
		Sections: []SectionEntry{{ID: "2", Sites: 1}},
		Sites:    []Site{{ID: "2:1", Quote: "A speaker MUST act.", Section: "2"}},
	}
	previous := &Extraction{
		Stem: "rfc9999", Register: registerProse,
		RegisterReason: "reviewed under the weaker register",
		ResignReason:   "source was refreshed",
		Sections: []ExtractionSection{{
			ID: "2", Sites: 1, Disposition: "skipped", SkipKind: "iana",
			Reason: "registry prose", UnsourcedIDs: []string{"RFC9999-2-2"},
		}},
		Sites: []ExtractionSite{{
			ID: "2:1", Quote: "A speaker MUST act.",
			Disposition: DispositionExcluded, ExcludedKind: exclusionDuplicate,
			Reason: "restates the mapped row", MappedTo: "RFC9999-2-2",
		}},
	}
	document := newExtractionDocument(inventory, previous)
	if document.Register != registerProse || document.RegisterReason != previous.RegisterReason ||
		document.ResignReason != previous.ResignReason {
		t.Errorf("the refresh lost artifact fields: %+v", document)
	}
	site := document.Sites[0]
	if site.Disposition == nil || *site.Disposition != DispositionExcluded ||
		site.ExcludedKind != exclusionDuplicate || site.Reason != previous.Sites[0].Reason ||
		site.MappedTo != "RFC9999-2-2" {
		t.Errorf("the duplicate disposition lost a required field: %+v", site)
	}
	section := document.Sections[0]
	if section.Disposition == nil || *section.Disposition != "skipped" ||
		section.SkipKind != "iana" || section.Reason != "registry prose" ||
		len(section.UnsourcedIDs) != 1 {
		t.Errorf("the skipped section lost a required field: %+v", section)
	}

	previous.Sites[0] = ExtractionSite{
		ID: "2:1", Quote: "A speaker MUST act.",
		Disposition: DispositionExcluded, ExcludedKind: relocatedToSpec,
		Reason:      "the implementation belongs to the destination spec",
		RelocatedTo: "plan/spec-widget.md", ReservedID: "RFC9999-2-3",
	}
	relocated := newExtractionDocument(inventory, previous).Sites[0]
	if relocated.RelocatedTo != "plan/spec-widget.md" || relocated.ReservedID != "RFC9999-2-3" {
		t.Errorf("the relocation lost its destination: %+v", relocated)
	}
}

func TestExtractionCreateRefreshPreservesOnlyDecisionsForTheSameSentence(t *testing.T) {
	tree := extractionCreateTree(t)
	inventory, err := NewDeriver(tree).Inventory("rfc9999", 2)
	if err != nil || inventory == nil {
		t.Fatalf("derive fixture inventory: %v, %+v", err, inventory)
	}
	document := newExtractionDocument(inventory, nil)
	mapped := DispositionMapped
	document.SignedOff = "2026-07-29"
	document.Reviewer = "tester"
	document.Sites[0].Disposition = &mapped
	document.Sites[0].MappedTo = "RFC9999-2-1"
	document.Sites[1].Disposition = &mapped
	document.Sites[1].MappedTo = "RFC9999-2-2"
	walked := "walked"
	for index := range document.Sections {
		document.Sections[index].Disposition = &walked
	}
	corpus := filepath.Join(tree, "rfc", "extraction")
	if err := writeExtractionDocument(tree, corpus, "rfc9999", document); err != nil {
		t.Fatalf("write classified fixture: %v", err)
	}

	moved := strings.Replace(extractionCreateSource,
		"A speaker MUST do the first thing.", "A speaker MUST do something else.", 1)
	if err := os.WriteFile(filepath.Join(tree, "rfc", "full", "rfc9999.txt"), []byte(moved), 0o600); err != nil {
		t.Fatalf("move fixture sentence: %v", err)
	}
	report, err := createExtraction(tree, "rfc9999")
	if err != nil {
		t.Fatalf("refresh skeleton: %v", err)
	}
	// One sentence moved, so one site is unclassified again and the refreshed
	// document is a skeleton. The valid sign-off it would replace stays in the
	// corpus, and the gate stays green while the walk is redone.
	if report.Placed || report.UnclassifiedSites != 1 {
		t.Fatalf("a refresh with an unclassified site landed in the corpus: %+v", report)
	}
	standing, err := ParseExtractionArtifact(tree, filepath.Join(tree, "rfc", "extraction", "rfc9999.json"))
	if err != nil {
		t.Fatalf("parse the standing artifact: %v", err)
	}
	if sites, sections := standing.Unclassified(); sites+sections != 0 {
		t.Errorf("the refresh left the standing sign-off unclassified: %d site(s), %d section(s)",
			sites, sections)
	}

	artifact, err := ParseExtractionArtifact(tree, filepath.Join(tree, filepath.FromSlash(report.Path)))
	if err != nil {
		t.Fatalf("parse refreshed skeleton: %v", err)
	}
	byID := map[string]ExtractionSite{}
	for _, site := range artifact.Sites {
		byID[site.ID] = site
	}
	if byID["2:1"].Disposition != "" {
		t.Errorf("the changed sentence retained its old decision: %+v", byID["2:1"])
	}
	if byID["2:2"].MappedTo != "RFC9999-2-2" {
		t.Errorf("the unchanged sentence lost its decision: %+v", byID["2:2"])
	}
	if artifact.SignedOff != "2026-07-29" || artifact.Reviewer != "tester" {
		t.Errorf("the refresh lost sign-off metadata: %+v", artifact)
	}
}

func TestExtractionCreateRefusalsLeaveAnExistingArtifactUntouched(t *testing.T) {
	tree := extractionCreateTree(t)
	path := filepath.Join(tree, "rfc", "extraction", "rfc9999.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("artifact directory: %v", err)
	}
	before := []byte("not valid JSON\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatalf("existing artifact: %v", err)
	}
	if _, err := createExtraction(tree, "rfc9999"); err == nil {
		t.Fatal("a malformed existing artifact was overwritten")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read refused artifact: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("the refusal changed the artifact: %q", after)
	}
}

func TestExtractionCreateRoundTripRefusalIsAtomic(t *testing.T) {
	tree := extractionCreateTree(t)
	path := filepath.Join(tree, "rfc", "extraction", "rfc9999.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("artifact directory: %v", err)
	}
	before := []byte("reviewer evidence survives\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatalf("existing artifact: %v", err)
	}

	inventory, err := NewDeriver(tree).Inventory("rfc9999", 2)
	if err != nil || inventory == nil {
		t.Fatalf("derive fixture inventory: %v, %+v", err, inventory)
	}
	document := newExtractionDocument(inventory, nil)
	document.Sections = append(document.Sections, document.Sections[1])
	corpus := filepath.Join(tree, "rfc", "extraction")
	if err := writeExtractionDocument(tree, corpus, "rfc9999", document); err == nil ||
		!strings.Contains(err.Error(), "duplicate section") {
		t.Fatalf("the staged parser did not refuse the broken document: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read existing artifact: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Errorf("the round-trip refusal changed the artifact: %q", after)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove previous artifact: %v", err)
	}
	if err := writeExtractionDocument(tree, corpus, "rfc9999", document); err == nil {
		t.Fatal("the staged parser accepted the broken document without a previous artifact")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a refused first skeleton landed on disk: %v", err)
	}
}

func TestExtractionCreateValidatesStemAndSourceBeforeWriting(t *testing.T) {
	for _, stem := range []string{"rfc4271", "sflow-v5", "draft-ietf-bess-mup-safi-00"} {
		if err := validateExtractionStem(stem); err != nil {
			t.Errorf("real stem %q was refused: %v", stem, err)
		}
	}

	for _, stem := range []string{"../full/rfc4271", "rfc/short/rfc4271", "..", "/etc/passwd", "RFC4271", "rfc4271\n"} {
		t.Run(strings.ReplaceAll(stem, "/", "_"), func(t *testing.T) {
			tree := extractionCreateTree(t)
			if _, err := createExtraction(tree, stem); err == nil ||
				!strings.Contains(err.Error(), "not an RFC or draft stem") {
				t.Fatalf("stem %q was not refused: %v", stem, err)
			}
			if _, err := os.Stat(filepath.Join(tree, "rfc", "extraction")); !os.IsNotExist(err) {
				t.Errorf("stem %q changed the extraction directory: %v", stem, err)
			}
		})
	}

	tree := t.TempDir()
	if _, err := createExtraction(tree, "rfc0000"); err == nil ||
		!strings.Contains(err.Error(), "no source text") {
		t.Fatalf("a missing source was not refused: %v", err)
	}
}

func TestExtractionCreateSweepsOnlyAbandonedStagingDirectories(t *testing.T) {
	tree := extractionCreateTree(t)
	// The sweep runs in the directory the write targets, and an unclassified
	// skeleton targets the session scratch.
	directory, err := extractionScratch(tree)
	if err != nil {
		t.Fatalf("resolve the skeleton scratch: %v", err)
	}
	stale := filepath.Join(directory, extractionStagingPrefix+"stale")
	fresh := filepath.Join(directory, extractionStagingPrefix+"fresh")
	for _, path := range []string{stale, fresh} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatalf("staging directory: %v", err)
		}
	}
	old := time.Now().Add(-10 * extractionStagingMaxAge)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatalf("age stale directory: %v", err)
	}
	if _, err := createExtraction(tree, "rfc9999"); err != nil {
		t.Fatalf("CreateExtraction: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("the abandoned staging directory remains: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("the concurrent staging directory was removed: %v", err)
	}
}

func TestExtractionCreateActionIsArgumentAwareAndClaimedByTheCensus(t *testing.T) {
	var found bool
	for _, action := range Actions().Actions {
		if action.Verb == "extraction-create" {
			found = action.Writes
		}
	}
	if !found {
		t.Fatal("the RFC action catalog does not publish extraction-create as a writer")
	}
	if answer, code := Answer([]string{"extraction-create"}); code != 2 || answer != nil {
		t.Errorf("a missing stem answered (%v, %d), want (nil, 2)", answer, code)
	}
	if _, code := Answer([]string{"extraction-create", "rfc9999"}); code != 2 {
		t.Errorf("a value before its keyword answered %d, want grammar refusal 2", code)
	}

	tree := extractionCreateTree(t)
	t.Setenv("ZE_REPO_ROOT", tree)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	answer, code := Answer([]string{"extraction-create", "stem", "rfc9999"})
	if code != 0 {
		t.Fatalf("the native action answered %d, want 0", code)
	}
	if _, ok := answer.(extractionCreateReport); !ok {
		t.Errorf("the native action answered %T, want ExtractionCreateReport", answer)
	}
}

func TestExtractionCreateReportUsesKebabCaseStructuredFields(t *testing.T) {
	body, err := json.Marshal(extractionCreateReport{})
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	for _, key := range []string{"unclassified-sites", "unclassified-sections"} {
		if !strings.Contains(string(body), `"`+key+`"`) {
			t.Errorf("report JSON has no %q field: %s", key, body)
		}
	}
}

// TestAnUnclassifiedSkeletonNeverReachesTheCorpus is the procedural guard the
// 2026-08-30 owner directive asked for: the generator's own output must not be
// able to red the gate. It drives the writer over a source with no previous
// artifact and asserts that rfc/extraction/ is untouched, that the skeleton is
// under this session's scratch, and that moving it in is what the writer says
// to do next.
func TestAnUnclassifiedSkeletonNeverReachesTheCorpus(t *testing.T) {
	tree := extractionCreateTree(t)

	report, err := createExtraction(tree, "rfc9999")
	if err != nil {
		t.Fatalf("createExtraction: %v", err)
	}
	if report.Placed {
		t.Errorf("the writer claims it placed an unclassified skeleton: %+v", report)
	}
	if _, err := os.Stat(filepath.Join(tree, "rfc", "extraction", "rfc9999.json")); !os.IsNotExist(err) {
		t.Fatalf("an unclassified skeleton landed in the corpus: %v", err)
	}
	scratch, err := extractionScratch(tree)
	if err != nil {
		t.Fatalf("resolve the skeleton scratch: %v", err)
	}
	if want := relTo(tree, filepath.Join(scratch, "rfc9999.json")); report.Path != want {
		t.Errorf("the skeleton is at %q, want %q", report.Path, want)
	}
	if _, err := os.Stat(filepath.Join(tree, filepath.FromSlash(report.Path))); err != nil {
		t.Errorf("the writer named a file it did not write: %v", err)
	}
	// A message that says only "something is wrong" teaches nothing. The move
	// that ends the walk must be in it, spelled out.
	move := "mv " + report.Path + " rfc/extraction/rfc9999.json"
	if !strings.Contains(report.Text(), move) {
		t.Errorf("the writer does not name the move %q:\n%s", move, report.Text())
	}
}

// TestAFullyClassifiedRefreshStillLandsInTheCorpus proves the guard cuts the
// skeleton and nothing else. A source change that leaves every locator and
// every sentence intact carries every decision forward, so the refreshed
// document IS a sign-off and belongs where sign-offs live. Deleting the feature
// was never an option: a walk needs the skeleton, and a re-derive needs the
// artifact replaced in place.
func TestAFullyClassifiedRefreshStillLandsInTheCorpus(t *testing.T) {
	tree := extractionCreateTree(t)
	inventory, err := NewDeriver(tree).Inventory("rfc9999", 2)
	if err != nil || inventory == nil {
		t.Fatalf("derive fixture inventory: %v, %+v", err, inventory)
	}
	document := newExtractionDocument(inventory, nil)
	mapped, walked := DispositionMapped, "walked"
	document.SignedOff = "2026-08-30"
	document.Reviewer = "tester"
	document.Sites[0].Disposition, document.Sites[0].MappedTo = &mapped, "RFC9999-2-1"
	document.Sites[1].Disposition, document.Sites[1].MappedTo = &mapped, "RFC9999-2-2"
	for index := range document.Sections {
		document.Sections[index].Disposition = &walked
	}
	corpus := filepath.Join(tree, "rfc", "extraction")
	if err := writeExtractionDocument(tree, corpus, "rfc9999", document); err != nil {
		t.Fatalf("write the classified fixture: %v", err)
	}

	// Whitespace only: every site id and every quote survives it, so every
	// decision carries forward and only the source fingerprint moves.
	source := strings.Replace(extractionCreateSource, "3.  References", "3.   References", 1)
	if err := os.WriteFile(filepath.Join(tree, "rfc", "full", "rfc9999.txt"), []byte(source), 0o600); err != nil {
		t.Fatalf("rewrite the fixture source: %v", err)
	}

	report, err := createExtraction(tree, "rfc9999")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if !report.Placed || report.Path != "rfc/extraction/rfc9999.json" {
		t.Fatalf("a fully classified refresh did not land in the corpus: %+v", report)
	}
	refreshed, err := ParseExtractionArtifact(tree, filepath.Join(corpus, "rfc9999.json"))
	if err != nil {
		t.Fatalf("parse the refreshed artifact: %v", err)
	}
	if refreshed.SourceSHA != RequirementSHA(source) || refreshed.Mapped() != 2 {
		t.Errorf("the refresh did not re-stamp a complete sign-off: %+v", refreshed)
	}
	if strings.Contains(report.Text(), "UNCLASSIFIED") {
		t.Errorf("a complete refresh reported unclassified work:\n%s", report.Text())
	}
}

// TestTheCheckLeadsWithAnActionableUnclassifiedCensus covers the other half of
// the guard: an artifact that arrives in the corpus by hand still has to be
// caught, and the author has to be told what to do about it. The per-site
// detail runs to one line per sentence, so the census must come FIRST.
func TestTheCheckLeadsWithAnActionableUnclassifiedCensus(t *testing.T) {
	tree := extractionCreateTree(t)
	inventory, err := NewDeriver(tree).Inventory("rfc9999", 2)
	if err != nil || inventory == nil {
		t.Fatalf("derive fixture inventory: %v, %+v", err, inventory)
	}
	document := newExtractionDocument(inventory, nil)
	scratch, err := extractionScratch(tree)
	if err != nil {
		t.Fatalf("resolve the skeleton scratch: %v", err)
	}
	if err := writeExtractionDocument(tree, scratch, "rfc9999", document); err != nil {
		t.Fatalf("stage the skeleton: %v", err)
	}
	corpus := filepath.Join(tree, "rfc", "extraction")
	if err := os.MkdirAll(corpus, 0o750); err != nil {
		t.Fatalf("corpus directory: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(scratch, "rfc9999.json"))
	if err != nil {
		t.Fatalf("read the staged skeleton: %v", err)
	}
	if err := os.WriteFile(filepath.Join(corpus, "rfc9999.json"), body, 0o600); err != nil {
		t.Fatalf("hand-place the skeleton: %v", err)
	}

	requirements, err := parseSummaryText(extractionCreateSummary, "rfc9999", "rfc/short/rfc9999.md")
	if err != nil {
		t.Fatalf("parse the fixture summary: %v", err)
	}
	signed, errs, err := evaluateExtractions(NewDeriver(tree), requirements)
	if err != nil {
		t.Fatalf("evaluateExtractions: %v", err)
	}
	if len(signed) != 0 {
		t.Fatalf("a hand-placed skeleton earned credit: %v", signed)
	}
	if len(errs) == 0 {
		t.Fatal("a hand-placed skeleton did not fail the check")
	}
	census := errs[0]
	for _, want := range []string{
		"rfc/extraction/rfc9999.json",
		"2 of 2 site(s) and 4 of 4 section(s) are UNCLASSIFIED",
		"Classify every one by hand",
		"./le rfc extraction-create stem rfc9999",
	} {
		if !strings.Contains(census, want) {
			t.Errorf("the leading violation does not say %q:\n%s", want, census)
		}
	}
}

// TestACompletedSignOffIsNeverSilentlyUncounted covers the reporting hole the
// owner found by counting files: seven artifacts, six credited. credited() is
// right to count only enrolled stems, so the fix is to name the remainder
// rather than to move the arithmetic.
func TestACompletedSignOffIsNeverSilentlyUncounted(t *testing.T) {
	valid := map[string]Extraction{
		"rfc1": {Stem: "rfc1", Register: registerProse},
		"rfc2": {Stem: "rfc2", Register: registerRFC2119},
		"rfc3": {Stem: "rfc3", Register: registerProse},
	}
	enrolled := map[string]bool{"rfc2": true}

	got := uncredited(valid, enrolled)
	if len(got) != 2 || got[0] != "rfc1" || got[1] != "rfc3" {
		t.Fatalf("the uncredited set is %v, want [rfc1 rfc3]", got)
	}
	if len(credited(valid, enrolled))+len(got) != len(valid) {
		t.Errorf("credited and uncredited do not partition the valid set")
	}

	report := CheckReport{
		Enrolled: 1, Unsigned: 0, SignedByRegister: map[string]int{"rfc2119": 1},
		SignedUnenrolled: got,
	}
	text := report.Text()
	for _, want := range []string{"valid but uncounted above", "rfc1, rfc3",
		"declares `| Enrolment | enrolled |`"} {
		if !strings.Contains(text, want) {
			t.Errorf("the check summary does not say %q:\n%s", want, text)
		}
	}
	// Text takes a pointer receiver: CheckReport grew past the linter's size
	// floor when the gate's findings joined it, so a composite literal is bound
	// to a name before the method is called.
	uncredited := CheckReport{Enrolled: 1, SignedByRegister: map[string]int{}}
	if strings.Contains(uncredited.Text(), "valid but uncounted above") {
		t.Error("the clause renders when no sign-off is uncredited")
	}
}
