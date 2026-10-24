// Design: docs/contributing/rfc-conformance-gates.md -- a record whose red needed paired breaks
package rfc

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// pairedProducerPath holds the second check of a pair: SendWidget's own
	// return and this one guard the same property, so breaking either alone
	// leaves the unit green and only both together redden it.
	pairedProducerPath   = "internal/sample/gate.go"
	pairedProducerUnit   = pairedProducerPath + "::GateWidget"
	pairedProducerSource = "package sample\n\n" +
		"// GateWidget refuses a widget nobody asked for.\n" +
		"func GateWidget(count int) bool {\n\treturn count > 0\n}\n"
	pairedBreak = "count > 0 -> true"
)

// pairedFixture is the selftest tree plus the second producer of a pair.
func pairedFixture() map[string]string {
	files := selftestDiscriminationSources()
	files[pairedProducerPath] = pairedProducerSource
	return files
}

// VALIDATES: a record that applied two breaks together names both, fingerprints
// both, survives the artifact round trip, and goes stale when EITHER producer
// changes, exactly as the single-break record does for its one producer.
// METHOD: seal a two-break record with the production minter, render it into the
// tree, load it back through the gate's loader, then verify it over the tree it
// was sealed against and over a tree where only the second producer changed.
// PREVENTS: the RFC4271-5.1.2-3 negative, whose red needs the AS_PATH dispatch in
// ASPathEdit.Record and planAttr's advertise gate broken together, being either
// unrecordable or recorded with the second break unchecked by `./le rfc check`.
func TestDiscriminationPairedBreakIsFingerprintedAndReverified(t *testing.T) {
	files := pairedFixture()
	record := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: selftestDiscriminationUnit, Route: RouteMutant,
		Producer: selftestProducerUnit, Break: selftestBreak,
		Paired: []PairedBreak{{Producer: pairedProducerUnit, Break: pairedBreak}},
		Solo:   pairedSoloRuns(),
	})
	if len(record.Paired) != 1 || !shaRE.MatchString(record.Paired[0].ProducerSHA) {
		t.Fatalf("the paired break was not fingerprinted: %+v", record.Paired)
	}

	tree := map[string]string{}
	maps.Copy(tree, files)
	putDiscrimination(t, tree, record)
	loaded, err := loadDiscrimination(discriminationTree(t, tree))
	if err != nil {
		t.Fatalf("a two-break record must load: %v", err)
	}
	if len(loaded) != 1 || len(loaded[0].Solo) != 2 || loaded[0].Solo[1] != record.Solo[1] {
		t.Fatalf("the solo runs did not survive the artifact: %+v", loaded)
	}
	if len(loaded[0].Paired) != 1 ||
		loaded[0].Paired[0] != record.Paired[0] {
		t.Fatalf("the paired break did not survive the artifact: %+v", loaded)
	}

	if verdicts := verifyFixture(t, files, nil, record); !verdicts[0].Verified() {
		t.Fatalf("the record went %s over the tree it was sealed against: %s",
			verdicts[0].State, verdicts[0].Detail)
	}
	changed := map[string]string{
		pairedProducerPath: strings.Replace(pairedProducerSource, "count > 0", "count > 1", 1),
	}
	verdicts := verifyFixture(t, files, changed, record)
	if verdicts[0].State != ProofProducerChanged {
		t.Fatalf("the second producer changed and the record went %s; want %s",
			verdicts[0].State, ProofProducerChanged)
	}
	if !strings.Contains(verdicts[0].Detail, pairedProducerUnit) {
		t.Fatalf("the stale verdict does not name the paired producer: %s", verdicts[0].Detail)
	}
}

// VALIDATES: the record's shape is refused when a paired break cannot mean what
// it says: one with no break text, one naming a whole file, one with no
// fingerprint, and one on the escape route, which states that no break exists.
// PREVENTS: a hand-written pair carrying an unchecked second producer.
func TestDiscriminationPairedBreakShapeIsRefused(t *testing.T) {
	files := pairedFixture()
	record := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: selftestDiscriminationUnit, Route: RouteMutant,
		Producer: selftestProducerUnit, Break: selftestBreak,
		Paired: []PairedBreak{{Producer: pairedProducerUnit, Break: pairedBreak}},
		Solo:   pairedSoloRuns(),
	})
	if err := validateDiscrimination(record, "fixture", 1); err != nil {
		t.Fatalf("the sealed pair must validate: %v", err)
	}
	cases := map[string]func(*DiscriminationRecord){
		"no break text":  func(r *DiscriminationRecord) { r.Paired[0].Break = " " },
		"a whole file":   func(r *DiscriminationRecord) { r.Paired[0].Producer = pairedProducerPath },
		"no fingerprint": func(r *DiscriminationRecord) { r.Paired[0].ProducerSHA = "" },
		"the escape route": func(r *DiscriminationRecord) {
			r.Route, r.Producer, r.ProducerSHA, r.Break = RouteNoBreak, selftestTablePath, "", ""
			r.Reason = escapeDeclaration
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			broken := record
			broken.Paired = append([]PairedBreak(nil), record.Paired...)
			mutate(&broken)
			if err := validateDiscrimination(broken, "fixture", 1); err == nil {
				t.Fatalf("a pair with %s validated", name)
			}
		})
	}
}

// VALIDATES: the mutant route applies every named mutant to one broken run, two
// in different files as two overlay files and two in one file as one file
// carrying both, records the first as the producer and the rest as paired, and
// refuses a selector named twice.
// PREVENTS: a set of breaks where only the first reached the compiler, which
// would observe a single break and record it as the pair.
func TestMutantRouteAppliesEveryNamedMutant(t *testing.T) {
	files := pairedFixture()
	files[selftestProducerPath] = selftestProducerSource +
		"\n// CountWidget answers how many were sent.\n" +
		"func CountWidget(count int) int {\n\treturn count + 0\n}\n"
	root := discriminationTree(t, files)
	report := filepath.Join(root, "report.json")
	mutants := []map[string]any{
		{"filePath": selftestProducerPath, "line": 5, "column": 2, "original": "return count", "mutated": "return 0"},
		{"filePath": pairedProducerPath, "line": 5, "column": 2, "original": "count > 0", "mutated": "true"},
		{"filePath": selftestProducerPath, "line": 10, "column": 2, "original": "count + 0", "mutated": "count + 1"},
	}
	var results []map[string]any
	for _, mutant := range mutants {
		results = append(results, map[string]any{"status": "KILLED", "mutant": mutant})
	}
	raw, err := json.Marshal(map[string]any{"results": results})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	first := selftestProducerPath + ":5:2#1"
	second := pairedProducerPath + ":5:2#1"
	third := selftestProducerPath + ":10:2#1"

	record := DiscriminationRecord{Route: RouteMutant}
	broken, solos, err := mutantBreak(root, newSourceReader(root), newScopeIndex(), &record,
		recordRequest{report: report, mutants: []string{first, second, third}})
	if err != nil {
		t.Fatalf("three named mutants must apply together: %v", err)
	}
	if record.Producer != selftestProducerUnit || record.Break != `line 5: "return count" -> "return 0"` {
		t.Fatalf("the first mutant is not the record's producer: %s, %q", record.Producer, record.Break)
	}
	want := []PairedBreak{
		{Producer: pairedProducerUnit, Break: `line 5: "count > 0" -> "true"`},
		{Producer: selftestProducerPath + "::CountWidget", Break: `line 10: "count + 0" -> "count + 1"`},
	}
	if len(record.Paired) != len(want) {
		t.Fatalf("paired = %+v, want %+v", record.Paired, want)
	}
	for index := range want {
		if record.Paired[index] != want[index] {
			t.Fatalf("paired[%d] = %+v, want %+v", index, record.Paired[index], want[index])
		}
	}
	contents := map[string]string{}
	for _, file := range broken {
		contents[file.rel] = file.content
	}
	if len(contents) != 2 || len(broken) != 2 {
		t.Fatalf("want one overlay file per touched file, got %d: %v", len(broken), contents)
	}
	producer := contents[selftestProducerPath]
	if !strings.Contains(producer, "return 0\n") || !strings.Contains(producer, "count + 1") {
		t.Fatalf("both mutants of one file must reach its one replacement:\n%s", producer)
	}
	if !strings.Contains(contents[pairedProducerPath], "return true") {
		t.Fatalf("the paired mutant did not reach its file:\n%s", contents[pairedProducerPath])
	}

	// Each mutant is also answered alone, over the tree's own copy of its file,
	// so the solo runs apply exactly one break each.
	if len(solos) != 3 {
		t.Fatalf("want one solo break set per mutant, got %d", len(solos))
	}
	for position, wantRel := range []string{selftestProducerPath, pairedProducerPath, selftestProducerPath} {
		if len(solos[position]) != 1 || solos[position][0].rel != wantRel {
			t.Fatalf("solo %d = %+v, want only %s", position, solos[position], wantRel)
		}
	}
	if alone := solos[0][0].content; !strings.Contains(alone, "return 0\n") || strings.Contains(alone, "count + 1") {
		t.Fatalf("the first solo must carry its own mutant and no other:\n%s", alone)
	}
	if alone := solos[2][0].content; strings.Contains(alone, "return 0\n") || !strings.Contains(alone, "count + 1") {
		t.Fatalf("the third solo must carry its own mutant and no other:\n%s", alone)
	}

	_, _, err = mutantBreak(root, newSourceReader(root), newScopeIndex(), &DiscriminationRecord{Route: RouteMutant},
		recordRequest{report: report, mutants: []string{first, first}})
	if err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("a selector named twice must be refused, got %v", err)
	}
}

// VALIDATES: one overlay replaces every file of a paired break, each with its
// own content, so the broken run compiles all of them.
func TestWriteOverlayReplacesEveryFileOfAPair(t *testing.T) {
	tree := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "overlay-pair-sid")
	overlay, err := writeOverlay(tree,
		overlayFile{rel: "internal/a/edit.go", content: "package a\n"},
		overlayFile{rel: "internal/b/edit.go", content: "package b\n"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(overlay)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct{ Replace map[string]string }
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]string{"internal/a/edit.go": "package a\n", "internal/b/edit.go": "package b\n"} {
		replacement, held := parsed.Replace[treePath(tree, rel)]
		if !held {
			t.Fatalf("the overlay does not replace %s: %v", rel, parsed.Replace)
		}
		got, err := os.ReadFile(replacement)
		if err != nil || string(got) != want {
			t.Fatalf("%s is replaced by %q, %v; want %q", rel, got, err, want)
		}
	}
}

// pairedSoloRuns answers the solo evidence a well-formed two-break record
// carries: one green run per break, the producer's first.
func pairedSoloRuns() []SoloGreen {
	return []SoloGreen{
		{Producer: selftestProducerUnit, Break: selftestBreak, Seconds: 1,
			Green: "interop: [1/1] relay-shape: pass (1s)"},
		{Producer: pairedProducerUnit, Break: pairedBreak, Seconds: 1,
			Green: "interop: [1/1] relay-shape: pass (1s)"},
	}
}

// VALIDATES: R2-2 -- a record with paired breaks carries one solo green run per
// break, in the order the breaks were applied, each naming the break it ran,
// and `./le rfc check` refuses the record without them. A single-break record
// carries none.
// METHOD: validateDiscrimination over a sealed pair with its solo runs, then
// over each way the solo evidence can be short, misattributed or empty, and over
// a single-break record that carries solo runs it has no use for. A pair with no
// solo runs at all still loads, so the derived files every hook builds stay
// readable, and the verifier refuses it as ProofSoloOwed.
// PREVENTS: a paired record whose second break was never shown to be needed,
// which is one break wearing the name of two.
func TestDiscriminationPairedBreakCarriesEverySoloGreen(t *testing.T) {
	files := pairedFixture()
	record := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: selftestDiscriminationUnit, Route: RouteMutant,
		Producer: selftestProducerUnit, Break: selftestBreak,
		Paired: []PairedBreak{{Producer: pairedProducerUnit, Break: pairedBreak}},
		Solo:   pairedSoloRuns(),
	})
	if err := validateDiscrimination(record, "fixture", 1); err != nil {
		t.Fatalf("a pair carrying both solo greens must validate: %v", err)
	}
	cases := map[string]func(*DiscriminationRecord){
		"one solo run short":   func(r *DiscriminationRecord) { r.Solo = r.Solo[:1] },
		"the breaks swapped":   func(r *DiscriminationRecord) { r.Solo[0], r.Solo[1] = r.Solo[1], r.Solo[0] },
		"another break's text": func(r *DiscriminationRecord) { r.Solo[1].Break = selftestBreak },
		"an empty green":       func(r *DiscriminationRecord) { r.Solo[1].Green = " " },
		"a negative duration":  func(r *DiscriminationRecord) { r.Solo[0].Seconds = -1 },
		"solo without a pair":  func(r *DiscriminationRecord) { r.Paired = nil },
		"solo on the escape": func(r *DiscriminationRecord) {
			r.Route, r.Producer, r.ProducerSHA, r.Break, r.Paired = RouteNoBreak, selftestTablePath, "", "", nil
			r.Reason = escapeDeclaration
		},
	}
	owing := record
	owing.Solo = nil
	if err := validateDiscrimination(owing, "fixture", 1); err != nil {
		t.Fatalf("a pair with no solo runs must still load: %v", err)
	}
	if verdicts := verifyFixture(t, files, nil, owing); verdicts[0].State != ProofSoloOwed {
		t.Fatalf("a pair with no solo runs went %s; want %s", verdicts[0].State, ProofSoloOwed)
	}
	if verdicts := verifyFixture(t, files, nil, record); !verdicts[0].Verified() {
		t.Fatalf("a pair with its solo runs went %s: %s", verdicts[0].State, verdicts[0].Detail)
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			broken := record
			broken.Paired = append([]PairedBreak(nil), record.Paired...)
			broken.Solo = append([]SoloGreen(nil), record.Solo...)
			mutate(&broken)
			if err := validateDiscrimination(broken, "fixture", 1); err == nil {
				t.Fatalf("a record with %s validated", name)
			}
		})
	}
}

// VALIDATES: R2-2 -- a unit carrier's clean run builds the coverage profile
// whenever the record carries paired breaks, on the mutant route too, so every
// paired producer is held to reachability; a single-break mutant record keeps
// its red as the reachability evidence and builds none.
// PREVENTS: the reachability check that was claimed for paired producers and
// never ran, because the profile was built for the revert route only.
func TestPairedUnitObservationBuildsTheCoverageProfile(t *testing.T) {
	cases := []struct {
		name   string
		kind   string
		route  string
		paired bool
		want   bool
	}{
		{"unit revert", kindUnit, RouteRevert, false, true},
		{"unit mutant, one break", kindUnit, RouteMutant, false, false},
		{"unit mutant, paired", kindUnit, RouteMutant, true, true},
		{"interop mutant, paired", kindInterop, RouteMutant, true, false},
	}
	for _, one := range cases {
		runner := observationRunner{carrier: Carrier{Kind: one.kind},
			record: DiscriminationRecord{Route: one.route}}
		if one.paired {
			runner.record.Paired = []PairedBreak{{Producer: pairedProducerUnit, Break: pairedBreak}}
		}
		if got := runner.wantsProfile(); got != one.want {
			t.Errorf("%s: wantsProfile = %v, want %v", one.name, got, one.want)
		}
	}
}

// VALIDATES: R2-2 -- a paired observation runs each break ALONE and records it
// only when every solo run stays green and the breaks together go red; a pair
// whose one break already reddens the unit by itself, which is also the shape a
// second break the unit never executes takes, is refused and nothing is
// recorded.
// METHOD: the interop carrier re-executes this test binary as its native
// action. The child reads which files the overlay replaces and fails the
// scenario at assertion 1 by the rule the parent names: "first" fails whenever
// the first break is applied, "both" only when both are.
// PREVENTS: the RFC4271-5.1.2-3 negative shape recorded with a paired break
// that contributes nothing to the red.
func TestPairedObservationRunsEachBreakAlone(t *testing.T) {
	const (
		rule     = "RFC_PAIRED_SOLO_RULE"
		selector = "RFC_PAIRED_SOLO_SCENARIO"
		scenario = "paired-solo"
	)
	if mode := os.Getenv(rule); mode != "" {
		pairedSoloChild(t, mode, scenario)
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "paired-solo-sid")
	t.Setenv("GOFLAGS", "")
	first := []overlayFile{{rel: "first.go", content: "package p\n"}}
	second := []overlayFile{{rel: "second.go", content: "package p\n"}}
	both := append(append([]overlayFile(nil), first...), second...)
	runner := func(t *testing.T, mode string) *observationRunner {
		t.Setenv(rule, mode)
		return &observationRunner{
			tree: t.TempDir(), carrier: Carrier{Kind: kindInterop}, scenario: scenario, self: binary,
			record: DiscriminationRecord{Unit: "internal/le/interoplab/bgp/check_rfc.go::checkPair",
				Route: RouteMutant, Citation: "1",
				Producer: selftestProducerUnit, Break: selftestBreak,
				Paired: []PairedBreak{{Producer: pairedProducerUnit, Break: pairedBreak}}},
			suite: interoplab.Catalog{SelectorVariable: selector,
				Action: "-test.run=^TestPairedObservationRunsEachBreakAlone$/-test.v"},
		}
	}

	t.Run("a break that reddens alone", func(t *testing.T) {
		_, solo, err := runner(t, "first").observeBreaks(both, [][]overlayFile{first, second})
		if err == nil {
			t.Fatalf("a pair whose first break reddens alone was recorded with solo %+v", solo)
		}
		if !strings.Contains(err.Error(), selftestProducerUnit) || !strings.Contains(err.Error(), "ALONE") {
			t.Errorf("the refusal does not name the break that reddens alone: %v", err)
		}
	})

	t.Run("a pair both breaks are needed for", func(t *testing.T) {
		red, solo, err := runner(t, "both").observeBreaks(both, [][]overlayFile{first, second})
		if err != nil {
			t.Fatalf("a pair whose breaks are both needed was refused: %v", err)
		}
		if !strings.Contains(red.Red, "assertion 1") {
			t.Errorf("the recorded red is not the scenario's: %s", red.Red)
		}
		if len(solo) != 2 || solo[0].Producer != selftestProducerUnit || solo[1].Producer != pairedProducerUnit ||
			solo[0].Break != selftestBreak || solo[1].Break != pairedBreak {
			t.Fatalf("solo runs = %+v, want one per break in order", solo)
		}
		for _, one := range solo {
			if !strings.Contains(one.Green, scenario+": pass (") {
				t.Errorf("solo run of %s does not carry the scenario's pass line: %q", one.Producer, one.Green)
			}
		}
	})
}

// pairedSoloChild is the native action TestPairedObservationRunsEachBreakAlone
// runs: it passes or fails its one scenario by which files the overlay replaces.
func pairedSoloChild(t *testing.T, mode, scenario string) {
	replaced := map[string]bool{}
	if overlay, held := strings.CutPrefix(os.Getenv("GOFLAGS"), "-overlay="); held && overlay != "" {
		raw, err := os.ReadFile(overlay)
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct{ Replace map[string]string }
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatal(err)
		}
		for source := range parsed.Replace {
			replaced[filepath.Base(source)] = true
		}
	}
	red := replaced["first.go"]
	if mode == "both" {
		red = replaced["first.go"] && replaced["second.go"]
	}
	if red {
		fmt.Fprintln(os.Stderr, "interop: [1/1] "+scenario+": FAIL (0s): scenario "+scenario+
			" assertion 1: the property broke")
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "interop: [1/1] "+scenario+": pass (0s)")
	os.Exit(0)
}

// VALIDATES: a re-take replaces a record the gate refuses. A paired record
// written before the solo runs existed verifies as ProofSoloOwed, and the
// recorder's merge drops it before it judges the file, so the refusal's own
// repair (`./le rfc discriminate-record`) lands.
// PREVENTS: the RFC4271-5.1.2-3 negative being unrepairable once its record
// lacked the solo runs.
func TestRetakeReplacesARecordTheGateRefuses(t *testing.T) {
	files := pairedFixture()
	fresh := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: selftestDiscriminationUnit, Route: RouteMutant,
		Producer: selftestProducerUnit, Break: selftestBreak,
		Paired: []PairedBreak{{Producer: pairedProducerUnit, Break: pairedBreak}},
		Solo:   pairedSoloRuns(),
	})
	stale := fresh
	stale.Solo = nil
	if verdicts := verifyFixture(t, files, nil, stale); verdicts[0].State != ProofSoloOwed {
		t.Fatalf("the stale pair must be refused by the gate, went %s", verdicts[0].State)
	}
	tree := map[string]string{}
	maps.Copy(tree, files)
	putDiscrimination(t, tree, stale)
	root := discriminationTree(t, tree)
	if err := mergeDiscrimination(root, stemOfFixture(t, fresh), fresh); err != nil {
		t.Fatalf("the re-take could not replace the refused record: %v", err)
	}
	loaded, err := loadDiscrimination(root)
	if err != nil {
		t.Fatalf("the file the re-take wrote does not load: %v", err)
	}
	if len(loaded) != 1 || len(loaded[0].Solo) != 2 {
		t.Fatalf("want the one re-taken record with its solo runs, got %+v", loaded)
	}
}

// stemOfFixture answers the stem a fixture record's file lives under.
func stemOfFixture(t *testing.T, record DiscriminationRecord) string {
	t.Helper()
	prefix, _, found := strings.Cut(record.RID, "-")
	if !found {
		t.Fatalf("the fixture record's id %q names no stem", record.RID)
	}
	return strings.ToLower(prefix)
}

// VALIDATES: R2-2 at the gate -- a COMMITTED paired record with no solo runs is
// a `./le rfc check` violation, and the violation names the defect and the one
// command that repairs it, rather than the fingerprint-drift text a stale
// record gets: nothing moved, the proof was simply never taken.
// METHOD: Check over a committed fixture holding a functional carrier, both
// producers and a sealed pair whose solo runs were removed.
// PREVENTS: a pair whose second break was never shown to be needed reading as
// a fingerprint drift, whose stated repair (`./le rfc discriminate`) records
// nothing.
func TestCheckRefusesAPairWithoutSoloRuns(t *testing.T) {
	committed := fixtureCorpus()
	committed[fixtureGadgetCIPath] = fixtureGadgetCI
	committed[pairedProducerPath] = pairedProducerSource
	record := sealFixture(t, committed, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: fixtureGadgetCIPath, Route: RouteRevert, Citation: selftestCIDirective,
		Producer: selftestProducerUnit, Break: selftestBreak,
		Paired: []PairedBreak{{Producer: pairedProducerUnit, Break: pairedBreak}},
	})
	putDiscrimination(t, committed, record)

	report, code := Check(commitFixtureTree(t, committed, nil), nil)
	if code != 2 || len(report.Violations) != 1 {
		t.Fatalf("a committed pair with no solo runs answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
	violation := report.Violations[0]
	for _, want := range []string{ProofSoloOwed, "solo", "`./le rfc discriminate-record`",
		selftestRIDSend} {
		if !strings.Contains(violation, want) {
			t.Errorf("the violation omits %q: %s", want, violation)
		}
	}
	for _, wrong := range []string{"moved fingerprint", "`./le rfc discriminate`,"} {
		if strings.Contains(violation, wrong) {
			t.Errorf("the violation reads as a drift (%q), but nothing moved: %s", wrong, violation)
		}
	}
}
