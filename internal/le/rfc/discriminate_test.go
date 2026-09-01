package rfc

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// sealFixture answers record with its fingerprints minted against files.
//
// It goes through sealDiscrimination, the production minter, so no test can
// spell a sha and no test can compute one a different way than the verifier
// reads it. The tree is thrown away; only the fingerprints are kept.
func sealFixture(t *testing.T, files map[string]string, record DiscriminationRecord) DiscriminationRecord {
	t.Helper()

	root := discriminationTree(t, files)
	covers, err := tagCoversIn(root)
	if err != nil {
		t.Fatalf("resolve the fixture's tagged units: %v", err)
	}
	sealed, err := sealDiscrimination(root, covers, record)
	if err != nil {
		t.Fatalf("seal the fixture record for %s: %v", record.RID, err)
	}
	return sealed
}

// discriminationArtifact renders records as one rfc/discrimination/<stem>.json.
//
// It marshals the schema type, so a fixture cannot drift from the schema the
// loader reads.
func discriminationArtifact(t *testing.T, records ...DiscriminationRecord) string {
	t.Helper()

	raw, err := json.Marshal(discriminationFile{RFC: selftestStem, Records: records})
	if err != nil {
		t.Fatalf("render the fixture artifact: %v", err)
	}
	return string(raw)
}

// discriminationFixture is the sources a proof record is verified against, plus
// that record.
//
// One builder, so every test that needs a verifiable proof gets the same one
// and a test cannot accidentally prove something the others do not.
func discriminationFixture(t *testing.T) (map[string]string, DiscriminationRecord) {
	t.Helper()

	files := selftestDiscriminationSources()
	return files, sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit:     selftestDiscriminationUnit,
		Route:    RouteMutant,
		Producer: selftestProducerUnit,
		Break:    selftestBreak,
	})
}

// discriminationTree writes a fixture tree and answers its root.
func discriminationTree(t *testing.T, files map[string]string) string {
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

// VALIDATES: R-6 and A-7 -- a record is keyed on the producer function NAME and the
// tagged unit's behavior hash, so it survives every mechanical edit that changes no
// behavior, and dies on every edit that changes one.
// METHOD: one record verified against five trees. The first is the tree it was recorded
// against. Three are mechanical: a file-top header that shifts every line, a comment
// inside both units, and a reflow. The last inverts the assertion.
// PREVENTS: the re-stamp burden rfc/audit/rfc7606.json records, where a 9-line inserted
// header shifted every key and cost two whole paragraphs of mechanical re-sealing.
func TestDiscriminationRecordKeyedOnUnitHashNotLine(t *testing.T) {
	files, record := discriminationFixture(t)

	mechanical := map[string]map[string]string{
		"a header inserted above every line": {
			selftestTestPath: "// Copyright.\n// Nine.\n// Lines.\n// Of.\n// Header.\n" +
				"// That.\n// Shifts.\n// Every.\n// Line.\n\n" + selftestTestSource,
			selftestProducerPath: "// Copyright.\n\n" + selftestProducerSource,
		},
		"a comment added inside both units": {
			selftestTestPath: strings.Replace(selftestTestSource,
				"func TestWidget() {\n", "func TestWidget() {\n\t// Check the count.\n", 1),
			selftestProducerPath: strings.Replace(selftestProducerSource,
				"func SendWidget(count int) int {\n", "func SendWidget(count int) int {\n\t// Answer it.\n", 1),
		},
		"the producer's doc comment rewritten": {
			selftestProducerPath: strings.Replace(selftestProducerSource,
				"// SendWidget answers the widget the speaker sends.",
				"// SendWidget answers the count.\n// A second line, for the reflow.", 1),
		},
		"blank lines added around the body": {
			selftestTestPath: strings.Replace(selftestTestSource,
				"func TestWidget() {\n", "func TestWidget() {\n\n\n", 1),
		},
	}
	for name, overlay := range mechanical {
		t.Run(name, func(t *testing.T) {
			verdicts := verifyFixture(t, files, overlay, record)
			if !verdicts[0].Verified() {
				t.Fatalf("the record went %s under %s; a mechanical edit must not void a proof",
					verdicts[0].State, name)
			}
		})
	}

	behavioral := map[string]map[string]string{
		"the tagged unit's assertion inverted": {
			selftestTestPath: strings.Replace(selftestTestSource, "!= 1", "== 1", 1),
		},
		"the producer rewritten": {
			selftestProducerPath: strings.Replace(selftestProducerSource, "return count", "return 0", 1),
		},
	}
	for name, overlay := range behavioral {
		t.Run(name, func(t *testing.T) {
			verdicts := verifyFixture(t, files, overlay, record)
			if verdicts[0].Verified() {
				t.Fatalf("the record still verified under %s; the red was never re-observed over that code", name)
			}
		})
	}
}

// VALIDATES: a rename of the tagged unit kills its record, so the record dies with the
// tag it proves rather than pointing at a function that no longer exists.
// PREVENTS: an orphaned record read as a proof of whatever now carries the tag.
func TestDiscriminationRecordDiesWithItsUnit(t *testing.T) {
	files, record := discriminationFixture(t)

	verdicts := verifyFixture(t, files, map[string]string{
		selftestTestPath: strings.Replace(selftestTestSource, "TestWidget", "TestGadget", 1),
	}, record)
	if verdicts[0].State != ProofUnitGone {
		t.Fatalf("a renamed tagged unit answered %q, want %q", verdicts[0].State, ProofUnitGone)
	}

	verdicts = verifyFixture(t, files, map[string]string{
		selftestProducerPath: strings.Replace(selftestProducerSource, "SendWidget", "EmitWidget", 2),
	}, record)
	if verdicts[0].State != ProofProducerGone {
		t.Fatalf("a renamed producer answered %q, want %q", verdicts[0].State, ProofProducerGone)
	}
}

// verifyFixture re-verifies records against the fixture sources with one overlay applied.
func verifyFixture(t *testing.T, files, overlay map[string]string,
	records ...DiscriminationRecord) []DiscriminationVerdict {
	t.Helper()

	tree := map[string]string{}
	maps.Copy(tree, files)
	maps.Copy(tree, overlay)
	root := discriminationTree(t, tree)
	covers, err := tagCoversIn(root)
	if err != nil {
		t.Fatalf("resolve the fixture's tagged units: %v", err)
	}
	verdicts, err := verifyDiscrimination(root, records, covers)
	if err != nil {
		t.Fatalf("re-verify the fixture: %v", err)
	}
	return verdicts
}

// gomuReport is the shape `gomu run --output json` writes, decoded here rather than in
// production code because this test's whole purpose is to MEASURE that shape.
type gomuReport struct {
	TotalFiles     int `json:"totalFiles"`
	ProcessedFiles int `json:"processedFiles"`
	Results        []struct {
		Mutant struct {
			FilePath string `json:"filePath"`
			Line     int    `json:"line"`
			Original string `json:"original"`
			Mutated  string `json:"mutated"`
			Function string `json:"function"`
		} `json:"mutant"`
		Status     string `json:"status"`
		Output     string `json:"output"`
		TestOutput []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"testOutput"`
	} `json:"results"`
}

// goTestFailRE matches the test name `go test` prints for each failing function.
var goTestFailRE = regexp.MustCompile(`--- FAIL: ([A-Za-z0-9_]+)`)

// VALIDATES: A-2 -- what a gomu report actually attributes a kill to.
// METHOD: a real report, trimmed to three results, taken from
// `gomu run --output json` over internal/component/bgp/plugins/role on 2026-08-31.
// MEASURED: `testOutput` is populated in NONE of the 1,042 results of that run, and
// neither is `mutant.function`. `Result.TestOutput` and `Mutant.Function` are declared in
// vendor/github.com/sivchari/gomu/internal/mutation/engine.go and assigned nowhere in the
// vendored tree. The killing test IS recoverable, from the raw `go test` text in `output`.
// PREVENTS: a claim-scoped proof route designed on a field gomu never fills, which would
// attribute every kill in a package to whichever test the report happened to name.
func TestGomuReportTestAttributionParses(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "gomu-report.json"))
	if err != nil {
		t.Fatalf("read the gomu report fixture: %v", err)
	}
	var report gomuReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("decode the gomu report fixture: %v", err)
	}
	if len(report.Results) == 0 {
		t.Fatal("the fixture carries no result; there is nothing to measure")
	}

	var killed, attributed, named int
	for _, result := range report.Results {
		if result.Mutant.FilePath == "" || result.Mutant.Line == 0 || result.Mutant.Mutated == "" {
			t.Errorf("a mutant carries no file, line or mutated text: %+v", result.Mutant)
		}
		if result.Mutant.Function != "" {
			named++
		}
		if result.Status != "KILLED" {
			continue
		}
		killed++
		if len(result.TestOutput) > 0 {
			attributed++
		}
		if len(goTestFailRE.FindAllStringSubmatch(result.Output, -1)) == 0 {
			t.Errorf("a killed mutant's output names no failing test:\n%s", result.Output)
		}
	}
	if killed == 0 {
		t.Fatal("the fixture carries no killed mutant, so it measures no attribution")
	}
	if attributed != 0 {
		t.Errorf("%d of %d killed mutants now carry testOutput. gomu attributes kills to named"+
			" tests, so `./le rfc discriminate` no longer has to parse `go test` text: revisit A-2",
			attributed, killed)
	}
	if named != 0 {
		t.Errorf("%d mutant(s) now carry mutant.function. gomu names the enclosing function, so"+
			" the producer key no longer has to be resolved from the file and line: revisit A-2", named)
	}
}

// proposeFixture is the mutants, the coverage and the claim one proposal reads.
//
// Two mutants in one producer and one in a function the tagged unit never
// reaches, so a proposer that ignored coverage and a proposer that returned
// nothing both fail the same test.
func proposeFixture() (Tag, []reportMutant, coverageSet, map[string]string) {
	tag := Tag{RID: selftestRIDSend, Polarity: PolarityPositive,
		File: selftestTestPath, Line: 3}
	mutants := []reportMutant{
		{FilePath: selftestProducerPath, Line: 5, Column: 2,
			Original: "return count", Mutated: "return 0"},
		{FilePath: selftestProducerPath, Line: 5, Column: 9,
			Original: "count", Mutated: "count + 1"},
		{FilePath: selftestProducerPath, Line: 40, Column: 2,
			Original: "return other", Mutated: "return 0"},
	}
	covered := coverageSet{selftestProducerPath: {{first: 4, last: 6}}}
	for index := range mutants {
		mutants[index].Ordinal = 1
		mutants[index].Killed = true
	}
	producers := map[string]string{
		mutants[0].at(): selftestProducerUnit,
		mutants[1].at(): selftestProducerUnit,
		mutants[2].at(): selftestProducerPath + "::OtherWidget",
	}
	return tag, mutants, covered, producers
}

// VALIDATES: AC-5 -- a proposal offers only mutants inside code the tagged unit
// EXECUTES, and it writes nothing.
// METHOD: three mutants against a coverage set that holds one block. Two fall inside
// it and one does not.
// PREVENTS: an author spending a `go test` run on a break the unit can never reach, and
// a proposer that answers "no candidate" because it filtered everything out.
func TestDiscriminateProposesCoveredMutantsOnly(t *testing.T) {
	tag, mutants, covered, producers := proposeFixture()

	got := candidatesFor(tag, selftestDiscriminationUnit, "the widget count is sent", mutants,
		covered, producers)
	if len(got) != 2 {
		t.Fatalf("the proposal offers %d candidate(s), want the 2 that lie inside the covered block: %+v", len(got), got)
	}
	for _, candidate := range got {
		if candidate.Producer != selftestProducerUnit {
			t.Errorf("candidate %+v names a producer the tagged unit never executes", candidate)
		}
		if candidate.RID != tag.RID || candidate.Polarity != tag.Polarity ||
			candidate.Unit != selftestDiscriminationUnit {
			t.Errorf("candidate %+v does not carry the tag it was proposed for", candidate)
		}
		if candidate.Break == "" || candidate.Mutant == "" {
			t.Errorf("candidate %+v names neither a break to apply nor the mutant to apply it from", candidate)
		}
	}

	// An empty coverage set is the unreached producer, and it must answer
	// nothing rather than everything.
	if none := candidatesFor(tag, selftestDiscriminationUnit, "", mutants, coverageSet{},
		producers); len(none) != 0 {
		t.Errorf("a unit that covers nothing was offered %d candidate(s)", len(none))
	}
}

// VALIDATES: AC-5's ranking -- a break whose text touches a symbol the tag's own prose
// names is offered before one that touches none.
// METHOD: two candidates in one covered producer. The claim names SendWidget, which one
// break's producer key carries and the other's text does not.
// PREVENTS: the lazy break R-7 describes: an author takes the first candidate offered, so
// the order decides which break gets recorded.
func TestDiscriminateRanksBySymbolInClaim(t *testing.T) {
	tag, mutants, covered, producers := proposeFixture()
	producers[mutants[1].at()] = selftestProducerPath + "::Unrelated"

	got := candidatesFor(tag, selftestDiscriminationUnit,
		"SendWidget answers what a speaker sends", mutants, covered, producers)
	if len(got) != 2 {
		t.Fatalf("the proposal offers %d candidate(s), want 2", len(got))
	}
	if len(got[0].Symbols) == 0 {
		t.Fatalf("the first candidate touches none of the claim's symbols: %+v", got)
	}
	if len(got[1].Symbols) != 0 {
		t.Errorf("the second candidate touches %v, so the ranking is not by claim symbol", got[1].Symbols)
	}
	if got[0].Producer != selftestProducerUnit {
		t.Errorf("the first candidate is %q, want the one the claim names", got[0].Producer)
	}

	// A claim naming no identifier costs a RANKING, never a candidate: the
	// gate never judges whether a break is a good one (R-7).
	if flat := candidatesFor(tag, selftestDiscriminationUnit, "", mutants, covered,
		producers); len(flat) != 2 {
		t.Errorf("a claim naming no symbol was offered %d candidate(s), want 2", len(flat))
	}
}

// judgeFixture is a runner over the fixture's tagged Go unit.
func judgeFixture(unit, names string) *observationRunner {
	return &observationRunner{
		carrier: Carrier{Kind: kindUnit},
		record: DiscriminationRecord{RID: selftestRIDSend, Polarity: PolarityPositive,
			Unit: selftestDiscriminationUnit, Route: RouteMutant,
			Producer: selftestProducerUnit, Break: selftestBreak},
		unitName: unit, names: names,
	}
}

// VALIDATES: AC-11 -- record mode refuses to write a proof it did not OBSERVE. A run that
// stayed green under the break, and a run that went red without naming the tagged unit,
// are each refused.
// METHOD: the judgement `requireRed` makes, driven over the three results a run can have.
// PREVENTS: the failure the whole artifact exists to prevent -- a stored record that
// publishes a red nobody saw. It also prevents the subtler one: a red that came from a
// build error, a sibling test or a flake, credited to this claim's test.
func TestDiscriminateRefusesUnobservedRed(t *testing.T) {
	runner := judgeFixture("TestWidget", "TestWidget")

	green := runner.judgeRed(true, "ok  \tgithub.com/ze-software/ze/internal/sample\t0.01s")
	if green == nil {
		t.Fatal("a run that stayed GREEN under the break was recorded as a proof")
	}
	if !strings.Contains(green.Error(), "GREEN") || !strings.Contains(green.Error(), selftestBreak) {
		t.Errorf("the refusal does not say what stayed green under which break: %v", green)
	}

	elsewhere := runner.judgeRed(false, "--- FAIL: TestGadget (0.00s)\n\tgadget_test.go:9: no")
	if elsewhere == nil {
		t.Fatal("a red another test produced was recorded as this unit's proof")
	}
	if !strings.Contains(elsewhere.Error(), "TestWidget") {
		t.Errorf("the refusal does not name the unit whose red was owed: %v", elsewhere)
	}

	if err := runner.judgeRed(false, "--- FAIL: TestWidget (0.00s)\n\twidget_test.go:5: 0 != 1"); err != nil {
		t.Errorf("a red naming the tagged unit was refused: %v", err)
	}

	// A file-scoped unit has no `--- FAIL:` line, so its attribution is the
	// carrier's own name. A red that names neither is still refused.
	file := judgeFixture("", "widget")
	if err := file.judgeRed(false, "FAIL widget (2 assertions)"); err != nil {
		t.Errorf("a red naming the file-scoped unit was refused: %v", err)
	}
	if file.judgeRed(false, "FAIL gadget") == nil {
		t.Error("a red naming another carrier was recorded as this one's proof")
	}
}

// VALIDATES: AC-6 and R-10 -- a revert record naming a producer the tagged unit's own
// coverage profile never executes is refused, and so is one that does not resolve.
// METHOD: the fixture's producer with a real coverage profile written twice, once with
// the producer's block executed and once with it compiled and never run.
// PREVENTS: the BMP over-claim shape -- a claim published as met while its producing
// function was unreachable on the rail the tag named.
func TestDiscriminationRevertRequiresReachableProducer(t *testing.T) {
	files := selftestDiscriminationSources()
	tree := discriminationTree(t, files)
	module := "example.test"
	runner := &observationRunner{
		tree: tree, carrier: Carrier{Kind: kindUnit},
		record: DiscriminationRecord{RID: selftestRIDSend, Polarity: PolarityPositive,
			Unit: selftestDiscriminationUnit, Route: RouteRevert, Producer: selftestProducerUnit},
	}
	// SendWidget spans lines 4 to 6 of the fixture producer: the doc comment,
	// the declaration and the body.
	profile := func(count string) string {
		return "mode: set\n" + module + "/" + selftestProducerPath + ":4.1,6.14 1 " + count + "\n"
	}
	written := filepath.Join(t.TempDir(), "cover.out")

	if err := os.WriteFile(written, []byte(profile("0")), 0o600); err != nil {
		t.Fatalf("write the unreached profile: %v", err)
	}
	err := runner.requireProducerReachedIn(module, written)
	if err == nil {
		t.Fatal("a producer the tagged unit never executes was accepted as a proof")
	}
	if !strings.Contains(err.Error(), selftestProducerUnit) {
		t.Errorf("the refusal does not name the unreached producer: %v", err)
	}

	if err := os.WriteFile(written, []byte(profile("3")), 0o600); err != nil {
		t.Fatalf("write the reached profile: %v", err)
	}
	if err := runner.requireProducerReachedIn(module, written); err != nil {
		t.Errorf("a producer the tagged unit does execute was refused: %v", err)
	}

	// A producer that does not resolve at all is the same defect, caught before
	// any coverage is read.
	runner.record.Producer = selftestProducerPath + "::NoSuchFunction"
	if runner.requireProducerReachedIn(module, written) == nil {
		t.Error("a producer this tree does not hold was accepted as a proof")
	}
}

// VALIDATES: AC-7 -- the escape is refused outright for a unit-tier tag whose producer
// resolves and lies in a file gomu can mutate, and the refusal is the GATE's, not the
// recorder's.
// METHOD: one escape against four trees: a mutatable producer, a producer .gomuignore
// excludes, a declaration-only producer, and a functional carrier. Driven through
// escapeCheck.verdict, which is what `./le rfc check` runs -- a guard that only ran where
// records are WRITTEN would be invisible to a record authored by hand.
// PREVENTS: R-9, the escape becoming the answer. Where a break can be generated for the
// asking, "no break exists" is false whatever reason is offered.
func TestDiscriminationEscapeRefusedForMutatableUnitTag(t *testing.T) {
	const gomuignore = "# patterns\nvendor/\n*_gen.go\ninternal/skipped/\n"
	files := selftestDiscriminationSources()
	files[".gomuignore"] = gomuignore
	files["internal/skipped/widget.go"] = selftestProducerSource
	reader := newSourceReader(discriminationTree(t, files))
	index := newScopeIndex()
	escape := DiscriminationRecord{RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: selftestDiscriminationUnit, Route: RouteNoBreak, Reason: escapeDeclaration}
	unitCheck := func(record DiscriminationRecord) DiscriminationVerdict {
		check := escapeCheck{reader: reader, index: index, carrier: Carrier{Kind: kindUnit},
			carried: true, record: record}
		return check.verdict()
	}
	carrierCheck := func(carrier Carrier, record DiscriminationRecord) DiscriminationVerdict {
		check := escapeCheck{reader: reader, index: index, carrier: carrier,
			carried: true, record: record}
		return check.verdict()
	}

	escape.Producer = selftestProducerUnit
	verdict := unitCheck(escape)
	if verdict.Verified() {
		t.Fatal("the escape was accepted for a unit tag whose producer gomu mutates")
	}
	for _, want := range []string{selftestProducerPath, RouteMutant, selftestRIDSend, "gomu"} {
		if !strings.Contains(verdict.Detail, want) {
			t.Errorf("the refusal omits %q: %s", want, verdict.Detail)
		}
	}

	// The three below may still be refused for their REASON, which is another
	// row's subject. What must not happen is the blanket refusal, so the tell is
	// the word it alone uses.
	for name, allowed := range map[string]DiscriminationVerdict{
		"a producer .gomuignore excludes":  unitCheck(withProducer(escape, "internal/skipped/widget.go::SendWidget")),
		"a producer with no function body": unitCheck(withProducer(escape, selftestTablePath)),
		// A functional carrier is outside gomu by construction: it runs unit
		// tests only (docs/contributing/testing.md).
		"a functional carrier": carrierCheck(Carrier{Kind: kindFunctional},
			withProducer(escape, selftestProducerUnit)),
	} {
		if strings.Contains(allowed.Detail, "gomu") {
			t.Errorf("the blanket refusal fired for %s, which gomu generates no break for: %s",
				name, allowed.Detail)
		}
	}
}

// withProducer answers the record with one producer swapped in.
func withProducer(record DiscriminationRecord, producer string) DiscriminationRecord {
	record.Producer = producer
	return record
}

// VALIDATES: AC-7 -- the escape takes a CLOSED vocabulary, each reason's precondition is
// checked against the tree, and each escape is tied to the CLAIM it discharges.
// METHOD: every reason supplied WITHOUT its precondition, without its claim tie, and with
// both. A reason outside the vocabulary, an absent reason, and a foreign escape citing
// nothing are refused at load time.
// PREVENTS: the blanket opt-out. A reason whose fact holds about SOME file discharges every
// tag equally: naming any doc.go would escape any tag on any tier, and carrier kind alone
// would escape all 37 interop tags in one command (R-9).
func TestDiscriminationEscapeVocabularyIsClosed(t *testing.T) {
	const generated = "// Code generated by ze. DO NOT EDIT.\n\npackage sample\n\nfunc Emit() int { return 1 }\n"
	const generatedPath = "internal/sample/emit_generated.go"
	const checker = "func CheckWidget() error {\n\tif a { return fail(7, err) }\n\treturn nil\n}\n"
	files := selftestDiscriminationSources()
	files[generatedPath] = generated
	files["internal/le/interoplab/bgp/check_widget.go"] = "package bgp\n\n" + checker
	reader := newSourceReader(discriminationTree(t, files))
	index := newScopeIndex()
	interop := Carrier{Kind: kindInterop}
	unit := Carrier{Kind: kindUnit}
	// The two producer-naming reasons are exercised on a FUNCTIONAL carrier, where
	// gomu never runs. On a unit carrier the blanket refusal answers first
	// whatever the reason is, and that refusal has its own test above: one row per
	// refusal is what says which one fired.
	functional := Carrier{Kind: kindFunctional}

	base := DiscriminationRecord{RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: selftestDiscriminationUnit, Route: RouteNoBreak,
		Source: selftestDiscriminationRel}
	// The claim each escape is tied to. It NAMES what its producer file
	// declares: Widgets in the table, Emit in the generated file.
	tableClaim := []Tag{{Claim: "the daemon drops a widget that is not in the Widgets table"}}
	generatedClaim := []Tag{{Claim: "Emit answers one widget for each request"}}
	strangerClaim := []Tag{{Claim: "the daemon drops a widget it was never asked for"}}

	// The vocabulary is closed at LOAD time, and so is the shape each reason
	// owes: a reason outside it, no reason at all, and a foreign escape citing
	// nothing never reach the tree.
	for name, refused := range map[string]DiscriminationRecord{
		"a reason outside the vocabulary": withReason(base, "there is nothing to break here", selftestTablePath),
		"no reason at all":                withReason(base, "", selftestTablePath),
		"a foreign escape citing nothing": withReason(base, escapeForeign, ""),
	} {
		if validateDiscrimination(refused, selftestDiscriminationRel, 1) == nil {
			t.Errorf("%s was accepted as an escape", name)
		}
	}

	// Each reason WITHOUT the fact it claims, and each reason WITHOUT the claim
	// that ties it to this tag.
	unfounded := map[string]escapeCheck{
		escapeForeign + " on a carrier this repository builds": {carrier: unit,
			record: withCitation(withReason(base, escapeForeign, ""), "7")},
		escapeForeign + " citing an assertion the checker does not hold": {carrier: interop,
			unit: checker, record: withCitation(withReason(base, escapeForeign, ""), "3")},
		escapeDeclaration + " on a file that declares functions": {carrier: functional,
			tagged: tableClaim, record: withReason(base, escapeDeclaration, selftestProducerPath)},
		escapeGenerated + " on a file carrying no generated marker": {carrier: functional,
			tagged: generatedClaim, record: withReason(base, escapeGenerated, selftestProducerPath)},
		escapeDeclaration + " whose claim names nothing that file declares": {carrier: functional,
			tagged: strangerClaim, record: withReason(base, escapeDeclaration, selftestTablePath)},
		escapeGenerated + " whose claim names nothing that file declares": {carrier: functional,
			tagged: strangerClaim, record: withReason(base, escapeGenerated, generatedPath)},
	}
	for name, one := range unfounded {
		one.reader, one.index, one.carried = reader, index, true
		verdict := one.verdict()
		if verdict.Verified() {
			t.Errorf("%s was accepted: the escape is not tied to the claim it discharges", name)
		}
		if verdict.State != ProofEscapeUnfounded {
			t.Errorf("%s answered %q, want %q", name, verdict.State, ProofEscapeUnfounded)
		}
	}

	// Each reason WITH its fact and its claim tie.
	founded := map[string]escapeCheck{
		escapeForeign: {carrier: interop, unit: checker,
			record: withCitation(withReason(base, escapeForeign, ""), "7")},
		escapeDeclaration: {carrier: functional, tagged: tableClaim,
			record: withReason(base, escapeDeclaration, selftestTablePath)},
		escapeGenerated: {carrier: functional, tagged: generatedClaim,
			record: withReason(base, escapeGenerated, generatedPath)},
	}
	for name, one := range founded {
		one.reader, one.index, one.carried = reader, index, true
		if verdict := one.verdict(); !verdict.Verified() {
			t.Errorf("%s was refused with its precondition and its claim tie holding: %s",
				name, verdict.Detail)
		}
	}
	if len(founded) != len(escapeReasons) {
		t.Errorf("this test exercises %d of the %d reasons in the closed vocabulary",
			len(founded), len(escapeReasons))
	}
}

// withReason answers the record with one reason and producer swapped in.
func withReason(record DiscriminationRecord, reason, producer string) DiscriminationRecord {
	record.Reason = reason
	record.Producer = producer
	return record
}

// withCitation answers the record with one citation swapped in.
func withCitation(record DiscriminationRecord, citation string) DiscriminationRecord {
	record.Citation = citation
	return record
}

// VALIDATES: an assertion whose number the checker COMPUTES cannot be cited, because a
// citation is checked against the numbers the checker writes out.
// METHOD: a checker whose only site is `fail(index+2, err)`, cited at 2.
// PREVENTS: bounding a citation by the COUNT of literal sites, which is what a first cut
// did: checkNoExportBoundary writes 1, 6, 7 and 7 and numbers the rest by expression, so a
// count would have bounded a 9-assertion checker at 4 and refused its own assertion 7.
func TestDiscriminationCitationRefusesAComputedAssertionNumber(t *testing.T) {
	const computed = "func CheckGadget() error {\n\tif a { return fail(index+2, err) }\n\treturn nil\n}\n"
	const written = "func CheckGadget() error {\n\tif a { return fail(2, err) }\n\treturn nil\n}\n"
	record := DiscriminationRecord{RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: "internal/le/interoplab/bgp/check_gadget.go::CheckGadget", Route: RouteRevert,
		Citation: "2"}
	interop := Carrier{Kind: kindInterop}

	if state, _ := citationState(interop, true, record, computed); state != ProofCitationGone {
		t.Error("an assertion number the checker computes rather than writes was accepted")
	}
	if state, detail := citationState(interop, true, record, written); state != ProofVerified {
		t.Errorf("an assertion number the checker writes out was refused: %s", detail)
	}
}

// VALIDATES: AC-8 -- an interop or functional record citing an assertion its carrier does
// not contain is refused, and so is a citation on a carrier that owes none.
// METHOD: an interop checker with three numbered `fail(N, ...)` sites, cited at every
// boundary of that range, and a `.ci` cited with a directive it holds and one it does not.
// PREVENTS: a citation that points nowhere. No generated break reaches either carrier, so
// the citation is the only thing tying the recorded red to one named assertion.
func TestDiscriminationCitationMustExistInCarrier(t *testing.T) {
	const checker = "func CheckWidget() error {\n" +
		"\tif a { return fail(1, err) }\n\tif b { return fail(2, err) }\n" +
		"\tif c { return fail( 3 , err) }\n\treturn nil\n}\n"
	const ci = "# RFC requirement: RFC9999-2-1 positive\nexpect=widget sent\nexpect=widget counted\n"
	record := DiscriminationRecord{RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: "internal/le/interoplab/bgp/check_widget.go::CheckWidget", Route: RouteRevert}
	interop := Carrier{Kind: kindInterop}

	// Boundaries of the numbered range: 0 below, 3 last valid, 4 above.
	for _, citation := range []string{"0", "4", "12", "one", ""} {
		record.Citation = citation
		state, detail := citationState(interop, true, record, checker)
		if state != ProofCitationGone {
			t.Errorf("assertion %q was accepted against a checker holding 3 numbered sites: %s",
				citation, detail)
		}
	}
	for _, citation := range []string{"1", "2", "3"} {
		record.Citation = citation
		if state, detail := citationState(interop, true, record, checker); state != ProofVerified {
			t.Errorf("assertion %q was refused against the checker that holds it: %s", citation, detail)
		}
	}

	functional := Carrier{Kind: kindFunctional}
	record.Unit = selftestCIPath
	record.Citation = "expect=widget counted"
	if state, detail := citationState(functional, true, record, ci); state != ProofVerified {
		t.Errorf("a directive the .ci holds was refused: %s", detail)
	}
	record.Citation = "expect=widget dropped"
	if state, _ := citationState(functional, true, record, ci); state != ProofCitationGone {
		t.Error("a directive the .ci does not hold was accepted")
	}
	record.Citation = ""
	if state, _ := citationState(functional, true, record, ci); state != ProofCitationGone {
		t.Error("a functional record citing nothing was accepted")
	}

	// A unit record's proof is the break, which the gate does check, so a
	// citation on one is an unchecked string beside a checked proof.
	record.Unit = selftestDiscriminationUnit
	record.Citation = "expect=widget counted"
	if state, _ := citationState(Carrier{Kind: kindUnit}, true, record, ci); state != ProofCitationGone {
		t.Error("a unit record carrying a citation was accepted")
	}
	record.Citation = ""
	if state, _ := citationState(Carrier{Kind: kindUnit}, true, record, ci); state != ProofVerified {
		t.Error("a unit record owing no citation was refused")
	}
}

// VALIDATES: R-6 over the REAL records -- a mechanical edit that shifts every line of the
// files a recorded proof names does not change that proof's verdict, so an unrelated
// rename cannot report mass staleness.
// METHOD: each record's unit file and producer file are copied out of this checkout
// TWICE, once unchanged and once with a nine-line header prepended, which shifts every
// line in it and changes what no unit DOES. The unheadered replay is the baseline, and
// only a record that verified there is required to verify with the header on. A record
// already stale in this checkout says a commit moved a producer it fingerprints, which is
// the ratchet reporting a real change; asserting over it would make this test fail for
// something it does not test. The test still fails if EVERY record is already stale,
// because then the header was never put to the question.
// PREVENTS: the re-stamp burden rfc/audit/rfc7606.json already pays. Two whole paragraphs
// of that artifact exist because a nine-line inserted header shifted every key it held,
// and this is the same edit applied to the same question.
func TestDiscriminationRealRecordsSurviveAMechanicalRename(t *testing.T) {
	root := checkoutRoot(t)
	records, err := loadDiscrimination(root)
	if err != nil {
		t.Fatalf("load the checkout's records: %v", err)
	}
	if len(records) == 0 {
		t.Skip("this checkout carries no discrimination record to replay")
	}

	// Nine lines, the height of the header that cost the audit artifact its
	// re-stamps, in each carrier's own comment syntax so the normalization the
	// record hashes with strips it.
	header := map[string]string{
		".go": "// Copyright.\n// Nine.\n// Lines.\n// Of.\n// Header.\n// That.\n// Shifts.\n// Every.\n// Line.\n\n",
		".ci": "# Copyright.\n# Nine.\n# Lines.\n# Of.\n# Header.\n# That.\n# Shifts.\n# Every.\n# Line.\n",
	}
	// The workflow directory comes across too. An interop carrier's TIER is
	// derived from the scheduled job that runs it, so a replay tree without it
	// would refuse the interop record's own unit as evidence nothing executes.
	// plain carries the same files WITHOUT the header. A record can be stale
	// before this test touches anything, because a commit by anyone can move a
	// producer this checkout's records fingerprint, and that staleness is the
	// ratchet working rather than a defect here. The question R-6 asks is
	// whether the HEADER changes a verdict, so the unheadered tree is the
	// baseline each verdict is compared against.
	files, plain := map[string]string{}, map[string]string{}
	workflows, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(workflowsRel)))
	if err != nil {
		t.Fatalf("read the checkout's workflows: %v", err)
	}
	for _, entry := range workflows {
		if entry.IsDir() || !hasWorkflowSuffix(entry.Name()) {
			continue
		}
		rel := workflowsRel + "/" + entry.Name()
		raw, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if readErr != nil {
			t.Fatalf("read %s out of the checkout: %v", rel, readErr)
		}
		files[rel] = string(raw)
		plain[rel] = string(raw)
	}

	for position := range records {
		for _, key := range []string{records[position].Unit, records[position].Producer} {
			if key == "" {
				continue
			}
			rel := keyFile(key)
			if _, held := files[rel]; held {
				continue
			}
			raw, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if readErr != nil {
				t.Fatalf("read %s out of the checkout: %v", rel, readErr)
			}
			files[rel] = header[filepath.Ext(rel)] + string(raw)
			plain[rel] = string(raw)
		}
	}

	tree := discriminationTree(t, files)
	covers, err := tagCoversIn(tree)
	if err != nil {
		t.Fatalf("resolve the replayed tree's tagged units: %v", err)
	}
	verdicts, err := verifyDiscrimination(tree, records, covers)
	if err != nil {
		t.Fatalf("replay the records: %v", err)
	}
	baseTree := discriminationTree(t, plain)
	baseCovers, err := tagCoversIn(baseTree)
	if err != nil {
		t.Fatalf("resolve the unheadered tree's tagged units: %v", err)
	}
	base, err := verifyDiscrimination(baseTree, records, baseCovers)
	if err != nil {
		t.Fatalf("replay the records without the header: %v", err)
	}
	if len(base) != len(verdicts) {
		t.Fatalf("the two replays answered %d and %d verdict(s), so they cannot be compared",
			len(base), len(verdicts))
	}

	shifted, carried := 0, 0
	for index := range verdicts {
		if !base[index].Verified() {
			// Stale before the header went on. Someone's commit moved a producer
			// this record fingerprints, which is the ratchet reporting a real
			// change rather than anything the header did.
			carried++
			continue
		}
		if verdicts[index].Verified() {
			continue
		}
		shifted++
		t.Errorf("%s %s at %s verified WITHOUT the header and went %s WITH it: %s",
			verdicts[index].Record.RID, verdicts[index].Record.Polarity,
			verdicts[index].Record.Unit, verdicts[index].State, verdicts[index].Detail)
	}
	if carried == len(verdicts) {
		t.Fatalf("all %d record(s) were already stale before the header, so this replay "+
			"proved nothing about the header", carried)
	}
	t.Logf("replayed %d record(s) across a nine-line header: %d shifted by it, "+
		"%d already stale before it", len(verdicts), shifted, carried)
}

// VALIDATES: the Security Review Checklist's input-validation row -- a gomu report's file
// path is AUTHORED input that reaches a tree write, so it passes the refusal a record's own
// keys pass.
// METHOD: three report paths, one per shape that leaves the checkout, beside the ordinary
// path the same loader accepts. The accepted case is what says the refusal is not "refuse
// everything".
// PREVENTS: `..` and an absolute path reaching the Go overlay's Replace key and, on the
// interop carrier, the os.WriteFile target requireRedInTree writes the broken source to.
func TestGomuReportRefusesAPathOutsideTheCheckout(t *testing.T) {
	tree := t.TempDir()
	report := func(file string) string {
		return `{"results":[{"status":"KILLED","mutant":{"filePath":` +
			strconv.Quote(file) + `,"line":4,"column":2,"original":"a","mutated":"b"}}]}`
	}
	write := func(body string) string {
		path := filepath.Join(tree, "report.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write the report: %v", err)
		}
		return path
	}

	for name, file := range map[string]string{
		"a parent-directory traversal": "internal/../../etc/passwd",
		"an absolute path":             "/etc/passwd",
		"a home-relative path":         "~/.ssh/authorized_keys",
	} {
		mutants, err := loadGomuReport(tree, write(report(file)))
		if err == nil {
			t.Errorf("%s was loaded as a mutant target: %+v", name, mutants)
			continue
		}
		if !strings.Contains(err.Error(), file) {
			t.Errorf("the refusal for %s does not name the path: %v", name, err)
		}
	}

	mutants, err := loadGomuReport(tree, write(report("internal/sample/widget.go")))
	if err != nil || len(mutants) != 1 || mutants[0].FilePath != "internal/sample/widget.go" {
		t.Errorf("an ordinary repo-relative path was refused: %v, %+v", err, mutants)
	}
}

// The escape recipe a second review found by measurement, spelled exactly as it
// stood: 605 of the 4,020 claims in the tree carry a whole word that some
// function-free file declares, and this file is the one a claim naming "path"
// reaches. It declares `const path` and no function, so gomu generates no break
// for it and the reason's own fact holds.
const (
	recipeProducerPath = "internal/le/yang/migration/testdata/path/move-success/internal/path.go"
	recipeProducer     = "package internal\n\nconst path = \"move-success\"\n"
	recipeClaim        = "the migration moves the leaf to the path it was given."
)

// VALIDATES: an escape is refused unless its producer is code the tagged unit REACHES:
// under testdata/ nothing is compiled, and a Go unit reaches its own package and the
// packages its file imports.
// METHOD: the measured escape recipe driven through escapeCheck.verdict on both carrier
// kinds, then a non-testdata file in a package the unit neither sits in nor imports, then
// the two honest shapes -- a producer beside the unit and a producer the unit imports.
// PREVENTS: the blanket opt-out the closed vocabulary was supposed to replace (R-9). The
// reason's fact and the claim tie both judge the producer FILE, so without this an author
// who cannot prove a claim goes and finds a file somewhere that fits the words.
func TestDiscriminationEscapeRefusedForUnreachableProducer(t *testing.T) {
	const strangerPath = "internal/stranger/table.go"
	const stranger = "package stranger\n\nvar Widgets = []int{1, 2, 3}\n"
	const importerPath = "internal/sample/importer_test.go"
	const importer = "package sample\n\nimport \"github.com/ze-software/ze/internal/stranger\"\n\n" +
		"func TestImports() {\n\t_ = stranger.Widgets\n}\n"
	files := selftestDiscriminationSources()
	files[recipeProducerPath] = recipeProducer
	files[strangerPath] = stranger
	files[importerPath] = importer
	reader := newSourceReader(discriminationTree(t, files))
	index := newScopeIndex()
	base := DiscriminationRecord{RID: selftestRIDSend, Polarity: PolarityPositive,
		Route: RouteNoBreak, Reason: escapeDeclaration, Source: selftestDiscriminationRel}
	pathClaim := []Tag{{Claim: recipeClaim}}
	widgetsClaim := []Tag{{Claim: "the daemon drops a widget that is not in the Widgets table"}}
	verdictOf := func(one escapeCheck) DiscriminationVerdict {
		one.reader, one.index, one.carried = reader, index, true
		return one.verdict()
	}

	recipe := base
	recipe.Unit, recipe.Producer = selftestCIPath, recipeProducerPath
	refused := verdictOf(escapeCheck{carrier: Carrier{Kind: kindFunctional},
		tagged: pathClaim, record: recipe})
	if refused.Verified() {
		t.Errorf("the measured escape recipe was accepted on the loosest carrier: %s", refused.Detail)
	}
	for _, want := range []string{recipeProducerPath, "testdata"} {
		if !strings.Contains(refused.Detail, want) {
			t.Errorf("the refusal omits %q: %s", want, refused.Detail)
		}
	}

	unreachable := map[string]escapeCheck{
		"the recipe on a unit carrier": {carrier: Carrier{Kind: kindUnit}, tagged: pathClaim,
			record: withProducer(withUnit(base, selftestDiscriminationUnit), recipeProducerPath)},
		"a compiled file the unit neither sits in nor imports": {carrier: Carrier{Kind: kindUnit},
			tagged: widgetsClaim,
			record: withProducer(withUnit(base, selftestDiscriminationUnit), strangerPath)},
		// A carrier that runs the daemon reaches every compiled file, so reach
		// says nothing there and used to exempt it. A predicate every file
		// satisfies ties the escape to no claim, which left the whole route
		// open for the .ci and interop tags after it was shut for unit tags.
		// These two reasons are now unreachable on that carrier: it names no
		// import and shares a directory with no producer.
		"a compiled file under a carrier that runs the daemon": {carrier: Carrier{Kind: kindFunctional},
			tagged: widgetsClaim, record: withProducer(withUnit(base, selftestCIPath), strangerPath)},
	}
	for name, one := range unreachable {
		verdict := verdictOf(one)
		if verdict.Verified() {
			t.Errorf("%s was accepted: the escape names code the tagged unit never runs", name)
		}
		if verdict.State != ProofEscapeUnfounded {
			t.Errorf("%s answered %q, want %q", name, verdict.State, ProofEscapeUnfounded)
		}
	}

	reachable := map[string]escapeCheck{
		"a producer in the unit's own package": {carrier: Carrier{Kind: kindUnit},
			tagged: widgetsClaim,
			record: withProducer(withUnit(base, selftestDiscriminationUnit), selftestTablePath)},
		"a producer in a package the unit's file imports": {carrier: Carrier{Kind: kindUnit},
			tagged: widgetsClaim,
			record: withProducer(withUnit(base, importerPath+"::TestImports"), strangerPath)},
	}
	for name, one := range reachable {
		if verdict := verdictOf(one); !verdict.Verified() {
			t.Errorf("%s was refused, and it is the honest case the escape exists for: %s",
				name, verdict.Detail)
		}
	}
}

// withUnit answers the record with one tagged unit swapped in.
func withUnit(record DiscriminationRecord, unit string) DiscriminationRecord {
	record.Unit = unit
	return record
}

// VALIDATES: an escape naming `<file>::<Func>` is refused when the file declares no such
// function, on both producer-naming reasons.
// METHOD: one declaration-only producer and one generated producer, each named with a
// function neither file holds, against the same records that verify without the symbol.
// PREVENTS: a half-read key. Each reason's fact is about the FILE, so a symbol nobody
// resolves would sit in a published record saying nothing (ai/rules/principles.md).
func TestDiscriminationEscapeRefusedForUnresolvedProducerSymbol(t *testing.T) {
	const generated = "// Code generated by ze. DO NOT EDIT.\n\npackage sample\n\nfunc Emit() int { return 1 }\n"
	const generatedPath = "internal/sample/emit_generated.go"
	files := selftestDiscriminationSources()
	files[generatedPath] = generated
	reader := newSourceReader(discriminationTree(t, files))
	index := newScopeIndex()
	// The unit sits beside both producers, so reach is satisfied and the symbol
	// half of the key is what this test is left measuring. A .ci under test/
	// would be refused for reach before the symbol is ever read.
	base := DiscriminationRecord{RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: "internal/sample/widget.ci", Route: RouteNoBreak, Source: selftestDiscriminationRel}
	tableClaim := []Tag{{Claim: "the daemon drops a widget that is not in the Widgets table"}}
	generatedClaim := []Tag{{Claim: "Emit answers one widget for each request"}}
	verdictOf := func(one escapeCheck) DiscriminationVerdict {
		one.reader, one.index, one.carried, one.carrier = reader, index, true,
			Carrier{Kind: kindFunctional}
		return one.verdict()
	}

	for name, one := range map[string]escapeCheck{
		escapeDeclaration + " naming a function the file does not declare": {tagged: tableClaim,
			record: withReason(base, escapeDeclaration, selftestTablePath+"::Widgets")},
		escapeGenerated + " naming a function the file does not declare": {tagged: generatedClaim,
			record: withReason(base, escapeGenerated, generatedPath+"::NoSuchFunc")},
	} {
		verdict := verdictOf(one)
		if verdict.Verified() {
			t.Errorf("%s was accepted: the symbol half of the producer key was read by nothing", name)
		}
		if !strings.Contains(verdict.Detail, "no such function") {
			t.Errorf("%s answered %q, which does not say the symbol is absent", name, verdict.Detail)
		}
	}

	// The pair: the same reasons verify when the key names the file alone, so a
	// rule that refused everything cannot pass this test.
	for name, one := range map[string]escapeCheck{
		escapeDeclaration: {tagged: tableClaim, record: withReason(base, escapeDeclaration, selftestTablePath)},
		escapeGenerated:   {tagged: generatedClaim, record: withReason(base, escapeGenerated, generatedPath)},
	} {
		if verdict := verdictOf(one); !verdict.Verified() {
			t.Errorf("%s was refused with its file-scoped producer resolving: %s", name, verdict.Detail)
		}
	}
}

// VALIDATES: a stored proof's verdict is readable from another package: the
// record it judges, the state the tree puts it in, and what moved.
//
// RenderInput.Discrimination was an exported field of an unexported element
// type until 2026-09-01, which is a name with no reachable value: no package
// outside this one could read a single field of it. The published per-RFC page
// states whether each tagged unit carries a proof, so the type it is stated
// through has to be reachable.
func TestTheDiscriminationVerdictIsReadableFromAnotherPackage(t *testing.T) {
	verdict := DiscriminationVerdict{
		Record: DiscriminationRecord{RID: "RFC9999-2-1", Polarity: PolarityPositive,
			Unit: "internal/a_test.go::TestWidget", Route: RouteNoBreak, Reason: escapeForeign},
		State:  ProofTagGone,
		Detail: "the unit is still in the tree and no longer carries this tag",
	}
	if verdict.Record.RID != "RFC9999-2-1" || verdict.Record.Route != RouteNoBreak {
		t.Errorf("the record reads %+v", verdict.Record)
	}
	if verdict.State != ProofTagGone || verdict.Detail == "" {
		t.Errorf("the state reads %q and the detail %q", verdict.State, verdict.Detail)
	}
	if verdict.Verified() {
		t.Error("a tag-gone verdict answers verified, so a stale record would publish as a proof")
	}
	if verdict.Record.Proves() {
		t.Error("a no-break record answers proves, so the escape would be counted as a proof")
	}
	proven := DiscriminationVerdict{Record: DiscriminationRecord{Route: RouteRevert},
		State: ProofVerified}
	if !proven.Verified() || !proven.Record.Proves() {
		t.Error("a verified revert record does not read as a proof")
	}
}

// VALIDATES: the cover key is readable and constructible from another package.
func TestTheCoverKeyIsReadableFromAnotherPackage(t *testing.T) {
	record := DiscriminationRecord{RID: "RFC9999-2-1", Polarity: PolarityNegative,
		Unit: "internal/a_test.go::TestWidget"}
	key := record.Cover()
	if key.RID != record.RID || key.Polarity != record.Polarity || key.Unit != record.Unit {
		t.Errorf("the cover reads %+v, want the record's own three fields", key)
	}
	if key != (Cover{RID: "RFC9999-2-1", Polarity: PolarityNegative,
		Unit: "internal/a_test.go::TestWidget"}) {
		t.Error("a cover built from its fields does not equal the record's own")
	}
}
