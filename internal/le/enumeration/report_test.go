// Goal: prove the answer a reader sees names the registry each literal
// restates, and counts the rows by that registry.
//
// Method: render a hand-built row set, because the renderer is what the tally
// and the remedy are read from, and neither is reachable from a finding alone.

package enumeration

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
// registry they restate, and the rows that restate none by their kind.
func TestTallyCountsByCorpus(t *testing.T) {
	found := Findings{
		{Kind: KindLiteral, Corpus: "plugin names"},
		{Kind: KindLiteral, Corpus: "plugin names"},
		{Kind: KindLiteral, Corpus: "CLI verbs"},
		{Kind: KindDoctorCheck},
	}
	tally := found.Tally()
	for name, want := range map[string]int{"plugin names": 2, "CLI verbs": 1, KindDoctorCheck: 1} {
		if tally[name] != want {
			t.Errorf("the tally counts %d for %q, want %d", tally[name], name, want)
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
