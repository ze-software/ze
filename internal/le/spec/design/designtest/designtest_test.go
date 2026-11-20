// VALIDATES: the records designtest seeds are the records the readers read:
// the seeded approval is the one lookup's answer, the seeded absence is the
// active one, and the clean fix spec with a seeded judge approval is admitted
// while Thomas is away.
// PREVENTS: a gate test passing against a record spelling the real readers
// would refuse, which would make every seeded approval prove nothing.
package designtest

import (
	"testing"
	"time"

	specdesign "github.com/ze-software/ze/internal/le/spec/design"
)

// TestSeededRecordsAreRead seeds each record into a clean tree and asks the
// reader that consumes it.
func TestSeededRecordsAreRead(t *testing.T) {
	root := t.TempDir()
	if err := WriteCleanTree(root, CleanSpec, CleanSpecText); err != nil {
		t.Fatal(err)
	}
	if err := SeedApproval(root, CleanSpec, CleanSpecText, "2026-11-20", "approved for the fixture"); err != nil {
		t.Fatal(err)
	}
	approval, found, err := specdesign.Approved(root, "spec-widget", CleanSpecText)
	if err != nil || !found {
		t.Fatalf("the seeded approval was not found: %v", err)
	}
	if approval.Date != "2026-11-20" || approval.Words != "approved for the fixture" {
		t.Fatalf("the seeded approval reads back as %#v", approval)
	}

	away := t.TempDir()
	if err := InitRepository(away); err != nil {
		t.Fatal(err)
	}
	if err := WriteCleanTree(away, CleanSpec, CleanSpecText); err != nil {
		t.Fatal(err)
	}
	if err := SeedAway(away, "2026-11-19", "2026-11-25"); err != nil {
		t.Fatal(err)
	}
	if err := SeedJudgment(away, CleanSpec, CleanSpecText, "approve"); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	if _, active, err := specdesign.ActiveAway(away, now); err != nil || !active {
		t.Fatalf("the seeded absence is not active: %v", err)
	}
	admission, err := specdesign.Admit(away, CleanSpec, CleanSpecText, now)
	if err != nil {
		t.Fatal(err)
	}
	if !admission.Admitted || !admission.Approval.Away {
		t.Fatalf("the judged clean fix spec was not admitted while away: %#v", admission)
	}

	if err := CommitAwayAdmission(away, CleanSpec, CleanSpecText, "2026-11-19", "2026-11-25"); err != nil {
		t.Fatal(err)
	}
	trailer, found, err := specdesign.Approved(away, "spec-widget", CleanSpecText)
	if err != nil || !found {
		t.Fatalf("the committed away admission was not found: %v", err)
	}
	if !trailer.Away || trailer.From != "2026-11-19" || trailer.Until != "2026-11-25" || trailer.Commit == "" {
		t.Fatalf("the committed away admission reads back as %#v", trailer)
	}
}
