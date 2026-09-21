// VALIDATES: the ledger's headline counts carry, in the same sentence, how many
// gated documents have been read against their own text.
// PREVENTS: the failure of 2026-09-21. The weekly update published "3,322
// checked, 228 owing" as a conformance measure while 125 of 184 documents had
// never been compared to their RFC. The walks that followed found 882
// MUST-level obligations the standards state and no summary carried, taking the
// owing figure to 1,139. A caveat existed three paragraphs below the number and
// did not stop it being quoted alone, so the bound now travels inside the
// sentence a reader copies.

package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// boundFixture answers a RenderInput over `enrolled` gated stems, of which
// `walked` carry a sign-off on disk.
func boundFixture(t *testing.T, enrolled, walked int) RenderInput {
	t.Helper()
	tree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tree, extractionRel), 0o750); err != nil {
		t.Fatalf("the fixture directory: %v", err)
	}
	metas := map[string]Meta{}
	for index := range enrolled {
		stem := "rfc" + string(rune('a'+index))
		metas[stem] = Meta{
			Enrolment: enrolmentEnrolled, EnrolmentReason: "gated",
			Implementation: implementationZe, ImplementationReason: "the fixture Go answers it",
		}
		if index < walked {
			path := filepath.Join(tree, extractionRel, stem+".json")
			if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
				t.Fatalf("the fixture sign-off: %v", err)
			}
		}
	}
	return RenderInput{Tree: tree, Metas: metas}
}

func TestTheHeadlineCountsCarryTheirOwnBound(t *testing.T) {
	t.Run("every document walked", func(t *testing.T) {
		got := extractionBoundSentence(boundFixture(t, 4, 4))
		if !strings.Contains(got, "Every one of the 4 gated documents") {
			t.Fatalf("a fully walked corpus says: %q", got)
		}
		if strings.Contains(got, "measure the list") {
			t.Errorf("a fully walked corpus warns anyway: %q", got)
		}
	})

	t.Run("some unwalked", func(t *testing.T) {
		got := extractionBoundSentence(boundFixture(t, 4, 1))
		for _, want := range []string{
			"READ THIS BEFORE QUOTING THE FIGURES ABOVE",
			"only 1 of the 4 gated documents",
			"measure the list rather than the software",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("the bound does not say %q:\n%s", want, got)
			}
		}
	})

	// A corpus with nothing gated has no figure to bound, so it says nothing
	// rather than warning about a population that does not exist.
	t.Run("nothing gated", func(t *testing.T) {
		if got := extractionBoundSentence(boundFixture(t, 0, 0)); got != "" {
			t.Fatalf("an empty corpus says %q", got)
		}
	})
}
