// Design: docs/contributing/rfc-conformance-gates.md -- a record whose red needed paired breaks
package rfc

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if len(loaded) != 1 || len(loaded[0].Paired) != 1 ||
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
	broken, err := mutantBreak(root, newSourceReader(root), newScopeIndex(), &record,
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

	_, err = mutantBreak(root, newSourceReader(root), newScopeIndex(), &DiscriminationRecord{Route: RouteMutant},
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
