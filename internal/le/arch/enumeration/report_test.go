// Goal: prove the answer a reader sees names the registry each literal
// restates, and counts the rows by that registry.
//
// Method: render a hand-built row set, because the renderer is what the tally
// and the remedy are read from, and neither is reachable from a finding alone.

package archenumeration

import (
	"strings"
	"testing"
)

// TestTextNamesTheCorpusAndTheRemedy proves a person reading the answer learns
// which registry to derive from, and how to mark a policy list.
func TestTextNamesTheCorpusAndTheRemedy(t *testing.T) {
	found := Findings{
		{Kind: KindLiteral, File: "internal/x/y.go", Line: 12, Symbol: "seedNames",
			Corpus: "plugin names", Keys: []string{"as112", "bfd"}, Detail: "restates plugin names"},
		{Kind: KindMarker, File: "internal/x/z.go", Line: 4, Detail: "the marker suppresses nothing"},
	}
	text := found.Text()
	for _, want := range []string{"internal/x/y.go:12", "seedNames", "restates plugin names", "as112, bfd", "enumeration: exempt"} {
		if !strings.Contains(text, want) {
			t.Errorf("the answer does not say %q:\n%s", want, text)
		}
	}
}

// TestTallyCountsByCorpus proves the report's tally groups the copies by the
// registry they restate, the rows that restate none by their kind, and the
// gated rows of a corpus apart from its findings.
func TestTallyCountsByCorpus(t *testing.T) {
	found := Findings{
		{Kind: KindLiteral, Corpus: "plugin names"},
		{Kind: KindLiteral, Corpus: "plugin names"},
		{Kind: KindLiteral, Corpus: "CLI verbs"},
		{Kind: KindDoctorCheck},
		{Kind: KindLiteral, Corpus: "YANG enumerations"},
		{Kind: KindGated, Corpus: "YANG enumerations"},
		{Kind: KindGated, Corpus: "YANG enumerations"},
	}
	tally := found.Tally()
	for name, want := range map[string]Count{
		"plugin names": {Findings: 2}, "CLI verbs": {Findings: 1}, KindDoctorCheck: {Findings: 1},
		"YANG enumerations": {Findings: 1, Gated: 2},
	} {
		if tally[name] != want {
			t.Errorf("the tally counts %+v for %q, want %+v", tally[name], name, want)
		}
	}
}

// TestTextOnAnEmptyRunSaysOK proves a clean tree answers a verdict rather than
// an empty page.
func TestTextOnAnEmptyRunSaysOK(t *testing.T) {
	if text := (Findings{}).Text(); !strings.Contains(text, "OK") {
		t.Errorf("a clean run rendered %q", text)
	}
}

// TestTextListsGatedRowsAfterTheFindings proves a gated row stays visible: it
// is printed in its own section after the findings, in a form that names the
// test, and the tally counts it apart from the findings.
func TestTextListsGatedRowsAfterTheFindings(t *testing.T) {
	found := Findings{
		{Kind: KindGated, File: "internal/a/b.go", Line: 20, Symbol: "speeds", Corpus: "YANG enumerations",
			Detail: "holds every value of the enumeration at ze-x/y, gated by TestSpeedsMatchTheModel"},
		{Kind: KindLiteral, File: "internal/x/y.go", Line: 12, Symbol: "modes", Corpus: "YANG enumerations",
			Keys: []string{"a", "b", "c"}, Detail: "holds every value of the enumeration at ze-x/z, so the two must agree"},
	}
	text := found.Text()
	row := "gated: internal/a/b.go:20: speeds: holds every value of the enumeration at ze-x/y, gated by TestSpeedsMatchTheModel"
	finding := strings.Index(text, "internal/x/y.go:12")
	gated := strings.Index(text, row)
	if finding < 0 || gated < 0 {
		t.Fatalf("the answer lacks the finding or the gated row:\n%s", text)
	}
	if gated < finding {
		t.Errorf("the gated row is printed before the findings:\n%s", text)
	}
	if !strings.Contains(text, "enumeration: 1 finding(s)") {
		t.Errorf("the header counts the gated row as a finding:\n%s", text)
	}
	if !strings.Contains(text, "YANG enumerations: 1 findings, 1 gated") {
		t.Errorf("the tally does not count the gated row apart:\n%s", text)
	}
	if !strings.Contains(text, "gated by") {
		t.Errorf("the remedy does not say what a gated row is:\n%s", text)
	}

	onlyGated := found[:1].Text()
	if !strings.Contains(onlyGated, "enumeration: OK") || !strings.Contains(onlyGated, row) {
		t.Errorf("a run holding only gated rows should answer OK and still list them:\n%s", onlyGated)
	}
}

// TestTheRemedySaysWhatTheGateVerifiesOfAGatedRow proves a reader is told the
// limit of a gated row: the gate checks that the named test is declared in
// the package the marker resolves to, and never that the test reads the leaf,
// so a gated row is proved by its test and not by the gate. It also proves the
// remedy shows the spelling for a test in another package.
func TestTheRemedySaysWhatTheGateVerifiesOfAGatedRow(t *testing.T) {
	text := Findings{{Kind: KindGated, File: "internal/a/b.go", Line: 20, Symbol: "speeds",
		Corpus: "YANG enumerations", Detail: "holds every value of the enumeration at ze-x/y, gated by TestSpeedsMatchTheModel"}}.Text()
	for _, want := range []string{"only that", "declared", "not that it reads the leaf", "gated by <dir>:TestX"} {
		if !strings.Contains(text, want) {
			t.Errorf("the remedy does not say %q:\n%s", want, text)
		}
	}
}
