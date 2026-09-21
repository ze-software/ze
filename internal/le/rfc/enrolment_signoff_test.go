// VALIDATES: every enrolled RFC owes an extraction sign-off, whether or not
// this commit is the one that enrolled it.
// PREVENTS: the failure of 2026-09-21. The clause read `newly` and
// grandfathered everything enrolled before the gate existed, so 125 of 184
// documents were gated on a requirement list nobody had read against the RFC.
// The weekly update published that list's figures as a conformance measure,
// and the walks that followed found 882 MUST-level obligations the standards
// state and no summary carried.

package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// enrolmentFixture answers a tree carrying RFC source text for each stem, which
// is the other precondition checkEnrolment tests before it reaches the sign-off
// clause.
func enrolmentFixture(t *testing.T, stems ...string) string {
	t.Helper()
	tree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tree, fullRel), 0o750); err != nil {
		t.Fatalf("the fixture directory: %v", err)
	}
	for _, stem := range stems {
		path := filepath.Join(tree, fullRel, stem+".txt")
		if err := os.WriteFile(path, []byte("A speaker MUST answer.\n"), 0o600); err != nil {
			t.Fatalf("the fixture source text: %v", err)
		}
	}
	return tree
}

func TestEveryEnrolledRFCOwesASignOff(t *testing.T) {
	const stem = "rfc9999"
	set := map[string]bool{stem: true}

	// Enrolled in an earlier commit, so `newly` is empty and the grandfathered
	// clause had nothing to say about it.
	t.Run("long enrolled and unsigned", func(t *testing.T) {
		tree := enrolmentFixture(t, stem)
		errs := checkEnrolment(tree, set, set, set, map[string]bool{})
		if !containsText(errs, "is enrolled with no valid extraction sign-off") {
			t.Fatalf("an unsigned RFC enrolled long ago is accepted: %v", errs)
		}
	})

	t.Run("signed", func(t *testing.T) {
		tree := enrolmentFixture(t, stem)
		errs := checkEnrolment(tree, set, set, set, set)
		if containsText(errs, "extraction sign-off") {
			t.Fatalf("a signed RFC is refused: %v", errs)
		}
	})
}

// containsText answers whether any finding carries want.
func containsText(errs []string, want string) bool {
	for _, err := range errs {
		if strings.Contains(err, want) {
			return true
		}
	}
	return false
}
