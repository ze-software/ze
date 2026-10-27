package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newSummaryKeyWordsSentence is the RFC 8174 form of the key-words sentence:
// four MUST-level keywords that tell a reader how to read the other sentences
// and oblige nobody.
const newSummaryKeyWordsSentence = "   The key words \"MUST\", \"MUST NOT\", \"REQUIRED\", \"SHALL\", \"SHALL\n" +
	"   NOT\", \"SHOULD\", \"SHOULD NOT\", \"RECOMMENDED\", \"NOT RECOMMENDED\",\n" +
	"   \"MAY\", and \"OPTIONAL\" in this document are to be interpreted as\n" +
	"   described in BCP 14 [RFC2119] [RFC8174] when, and only when, they\n" +
	"   appear in all capitals, as shown here.\n\n"

// newSummaryBoilerplateOnlySource is the RFC 7454 shape: MUST-level keywords
// only inside the key-words sentence, and SHOULD-level advice elsewhere.
const newSummaryBoilerplateOnlySource = "Test RFC 9999\n\n1.  Introduction\n\n" +
	newSummaryKeyWordsSentence +
	"2.  Operations\n\n" +
	"   Operators SHOULD filter the prefixes a customer announces.\n"

// newSummaryGenuineMustSource adds one sentence that obliges a speaker.
const newSummaryGenuineMustSource = newSummaryBoilerplateOnlySource +
	"\n3.  Speakers\n\n" +
	"   A speaker MUST discard a route whose prefix is too long.\n"

// writeNewSummarySource writes source as rfc/full/<stem>.txt under a fresh tree
// and answers the tree.
func writeNewSummarySource(t *testing.T, stem, source string) string {
	t.Helper()
	tree := t.TempDir()
	full := filepath.Join(tree, "rfc", "full")
	if err := os.MkdirAll(full, 0o750); err != nil {
		t.Fatalf("make rfc/full: %v", err)
	}
	if err := os.WriteFile(filepath.Join(full, stem+".txt"), []byte(source), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	return tree
}

// VALIDATES: AC-16 of spec-bcp194-1-communities, at the gate that judges a new
// summary. A new, un-enrolled summary declaring zero gated requirements passes
// when the source's only MUST-level keywords sit inside the RFC 2119 / RFC 8174
// key-words sentence, and still fails when the source also states one genuine
// MUST.
// PREVENTS: two opposite defects. Counting the key-words sentence refuses every
// zero-MUST BCP (RFC 7454 hits four keywords, all in that sentence), and an
// exclusion wider than the sentence lets a summary that captured nothing hide
// a real obligation. The method runs checkNewSummaries over a fixture tree for
// each polarity, and first proves the boilerplate fixture does carry
// capitalised keywords, so the passing case is not vacuous. It also reads
// sourceKeywordCount, the ledger's uppercase measurement, which must agree with
// the gate: zero for the boilerplate, one for the genuine MUST.
func TestNewSummaryBoilerplateOnlyPasses(t *testing.T) {
	const stem = "rfc9999"
	judge := func(tree string) []string {
		return checkNewSummaries(
			NewDeriver(tree), map[string]bool{stem: true}, map[string]bool{"rfc9998": true},
			map[string]bool{}, nil, map[string]string{}, true, map[string]Meta{},
		)
	}

	t.Run("boilerplate only passes", func(t *testing.T) {
		if raw := len(sourceKeywordRE.FindAllString(newSummaryBoilerplateOnlySource, -1)); raw == 0 {
			t.Fatal("the fixture carries no capitalised MUST-level keyword, so a pass proves nothing")
		}
		tree := writeNewSummarySource(t, stem, newSummaryBoilerplateOnlySource)
		if errs := judge(tree); len(errs) != 0 {
			t.Fatalf("the key-words sentence was read as an obligation: %v", errs)
		}
		upper, held := sourceKeywordCount(tree, stem)
		if !held {
			t.Fatal("the fixture source is not where the measurement reads it")
		}
		if upper != 0 {
			t.Errorf("the ledger's uppercase count read the key-words sentence as %d obligation keyword(s)", upper)
		}
	})

	t.Run("genuine MUST still fails", func(t *testing.T) {
		tree := writeNewSummarySource(t, stem, newSummaryGenuineMustSource)
		errs := judge(tree)
		if len(errs) != 1 {
			t.Fatalf("want one refusal for the genuine MUST, got %d: %v", len(errs), errs)
		}
		if !strings.Contains(errs[0], "declares NO MUST-level requirement, but the source text has 1 MUST-level keyword(s)") {
			t.Errorf("the refusal does not count exactly the one genuine MUST: %s", errs[0])
		}
		if upper, _ := sourceKeywordCount(tree, stem); upper != 1 {
			t.Errorf("the ledger's uppercase count is %d, want the one genuine MUST", upper)
		}
	})
}
