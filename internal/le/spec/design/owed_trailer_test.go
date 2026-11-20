// VALIDATES: the Design-approved: line a commit owes after an away admission
// landed: Thomas's later approval of the same text is still owed, once, and an
// away line already carried owes nothing more
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-30, AC-31).
// PREVENTS: his approval staying on one machine because an away line of the
// same digest was reachable, so every other checkout lists the spec as
// waiting for him forever.
package specdesign_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	specdesign "github.com/ze-software/ze/internal/le/spec/design"
	"github.com/ze-software/ze/internal/le/spec/design/designtest"
)

// TestOwedApprovalTrailerAfterAwayAdmission is AC-31 over an away line.
// Method: the clean fix spec is committed with its away trailer and Thomas is
// back. With no local approval the commit owes nothing; once his approval of
// the same text is recorded it owes his approval trailer; once a commit
// carries that line it owes nothing again, and with the local record gone
// the lookup answers his approval from the trailer, not the away admission.
func TestOwedApprovalTrailerAfterAwayAdmission(t *testing.T) {
	root := t.TempDir()
	if err := designtest.InitRepository(root); err != nil {
		t.Fatal(err)
	}
	spec, text := designtest.CleanSpec, designtest.CleanSpecText
	if err := designtest.WriteCleanTree(root, spec, text); err != nil {
		t.Fatal(err)
	}
	if err := designtest.CommitAwayAdmission(root, spec, text, "2026-11-19", "2026-11-25"); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

	owed, err := specdesign.OwedApprovalTrailer(root, spec, text, now)
	if err != nil {
		t.Fatal(err)
	}
	if owed != "" {
		t.Fatalf("an away line already carried, with no approval since, owes %q", owed)
	}

	if err := designtest.SeedApproval(root, spec, text, "2026-11-26", "approved on return"); err != nil {
		t.Fatal(err)
	}
	owed, err = specdesign.OwedApprovalTrailer(root, spec, text, now)
	if err != nil {
		t.Fatal(err)
	}
	want := specdesign.FormatApprovalTrailer(specdesign.Approval{Stem: "spec-widget",
		Digest: specdesign.DesignDigest(text), Date: "2026-11-26", Words: "approved on return"})
	if owed != want {
		t.Fatalf("Thomas's approval over an away line owes %q, want %q", owed, want)
	}

	// The next commit carrying the spec, its Updated cell moved (outside the
	// digest), lands the owed line, as ./le commit create writes it.
	updated := strings.Replace(text, "| Updated | 2026-11-19 |", "| Updated | 2026-11-26 |", 1)
	writeTree(t, root, map[string]string{spec: updated})
	for _, args := range [][]string{{"add", "--", spec}, {"commit", "-q", "-m", "the approval\n\n" + owed}} {
		if err := designtest.Git(root, args...); err != nil {
			t.Fatal(err)
		}
	}
	owed, err = specdesign.OwedApprovalTrailer(root, spec, updated, now)
	if err != nil {
		t.Fatal(err)
	}
	if owed != "" {
		t.Fatalf("a second approval line is owed after one landed: %q", owed)
	}

	// Another checkout holds the history but not the local record: the
	// lookup answers his approval from the trailer, never the away line.
	if err := os.Remove(filepath.Join(root, "tmp", "spec-design", "approved", "spec-widget.md")); err != nil {
		t.Fatal(err)
	}
	approval, found, err := specdesign.Approved(root, "spec-widget", updated)
	if err != nil || !found {
		t.Fatalf("the approval trailer was not found: %v", err)
	}
	if approval.Away || approval.Words != "approved on return" || approval.Commit == "" {
		t.Fatalf("the lookup answers %#v, want Thomas's approval from the trailer", approval)
	}
}
