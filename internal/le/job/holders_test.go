package job

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestHoldersReadsAnEntryTheWayAdmissionWroteIt: Holders is the reader
// scratch clean's full empty uses to see a le job or verify run, so it must read
// what the admission writer actually writes. The entry is written by take, the
// method `le job run` claims its slot with, and the test reads back the pid and
// the label it was admitted under.
func TestHoldersReadsAnEntryTheWayAdmissionWroteIt(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, JobsDir), 0o750); err != nil {
		t.Fatal(err)
	}
	registry := &Admission{Root: root, Out: os.Stdout, Err: os.Stderr}
	claimed := registry.take(&pending{label: "unit-pkg", key: "k", argv: []string{"go", "test", "./x/"}}, time.Now())
	if claimed.state != stateClaimed {
		t.Fatalf("take did not claim a slot: %+v", claimed)
	}

	holders, err := Holders(root)
	if err != nil {
		t.Fatalf("Holders: %v", err)
	}
	if len(holders) != 1 {
		t.Fatalf("Holders answered %d entries, want the one take wrote: %+v", len(holders), holders)
	}
	if holders[0].PID != os.Getpid() {
		t.Errorf("Holders answered pid %d, want %d", holders[0].PID, os.Getpid())
	}
	if holders[0].Label != "unit-pkg" {
		t.Errorf("Holders answered label %q, want %q", holders[0].Label, "unit-pkg")
	}
}

// TestHoldersAnswersNoneForAnAbsentRegistry: a checkout where no job ever ran
// has no registry directory, and that is the one state that holds no entry.
func TestHoldersAnswersNoneForAnAbsentRegistry(t *testing.T) {
	holders, err := Holders(t.TempDir())
	if err != nil {
		t.Fatalf("Holders over an absent registry: %v", err)
	}
	if len(holders) != 0 {
		t.Fatalf("Holders over an absent registry answered %+v", holders)
	}
}

// TestHoldersRefusesAnUnreadableRegistry: a registry directory that exists and
// cannot be listed has no answer to "is a job running". Holders must say so,
// because the full cache empty refuses on that error; reading it as "no
// holder" would let the empty run under a live verify. The directory is made
// unreadable with mode 000, which root ignores, so the case skips as root.
func TestHoldersRefusesAnUnreadableRegistry(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode 000 directory, so the unreadable state cannot be built")
	}
	root := t.TempDir()
	jobs := filepath.Join(root, JobsDir)
	if err := os.MkdirAll(jobs, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobs, "unit-pkg.1.job"), []byte("PID=1\nLABEL=unit-pkg\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(jobs, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(jobs, 0o750); err != nil { //nolint:gosec // restores the test's own directory so t.TempDir can remove it
			t.Error(err)
		}
	})

	holders, err := Holders(root)
	if err == nil {
		t.Fatalf("Holders over an unreadable registry answered %+v and no error", holders)
	}
}
