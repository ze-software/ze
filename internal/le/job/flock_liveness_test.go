package job

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/procscan"
)

// VALIDATES: a claimed job entry holds FLOCK=1 backed by an exclusive kernel
// flock; across PID namespaces (or with an unreachable PID) it still occupies
// its slot while locked, and once the lock drops (as on SIGKILL or sandbox
// teardown) scanAndClaim, Overview, Prune, and parentRunning immediately see
// the slot as free even when its recorded PID collides with a live process.
// PREVENTS: stale sandbox-PID entries (e.g. PID 35/38/39 colliding with host
// kernel threads) permanently consuming shared machine slots.
func TestFlockBackedEntryLivenessSurvivesPIDMismatchAndDropsImmediately(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	adm := &Admission{Root: root, Slots: 1, Poll: time.Millisecond, Banner: time.Hour, Stall: StallDefault, ResultWait: ResultWaitDefault, Unshared: true}

	ticket, err := adm.Admit("holder", []string{"le", "test"})
	if err != nil {
		t.Fatalf("Admit holder: %v", err)
	}
	entryPath := filepath.Join(root, filepath.FromSlash(ticket.Entry))
	body, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatalf("read claimed entry: %v", err)
	}
	if field(body, flockField) != flockHeld {
		t.Fatalf("claimed entry missing %s=%s:\n%s", flockField, flockHeld, body)
	}

	// While locked, even a second scan that sees no matching PID in its
	// process table (simulating another PID namespace) reports the slot occupied.
	emptyTable := processTable{running: map[int]struct{}{}}
	if _, occupies := adm.holderRow(entryPath, emptyTable, time.Now()); !occupies {
		t.Fatal("locked FLOCK=1 entry did not occupy slot in Overview despite empty process table")
	}
	if !parentRunning(entryPath, body) {
		t.Fatal("locked FLOCK=1 entry reported parent not running")
	}

	// Simulate holder dying via SIGKILL / sandbox teardown without removing its
	// .job file: close the flock fd while leaving the entry (with our live PID!)
	// on disk.
	if err := ticket.lockFile.Close(); err != nil {
		t.Fatalf("close lockFile: %v", err)
	}
	ticket.lockFile = nil

	// Even though the entry's PID is os.Getpid() (a live process!), every
	// liveness reader must now see the entry as dead because its flock dropped.
	liveTable := processTable{running: map[int]struct{}{os.Getpid(): {}}}
	if _, occupies := adm.holderRow(entryPath, liveTable, time.Now()); occupies {
		t.Fatal("unlocked FLOCK=1 entry still occupied slot in Overview")
	}
	if parentRunning(entryPath, body) {
		t.Fatal("unlocked FLOCK=1 entry still reported parent running")
	}

	// A new job must immediately claim the single slot rather than blocking.
	next, err := adm.Admit("next", []string{"le", "lint"})
	if err != nil {
		t.Fatalf("Admit next after flock drop: %v", err)
	}
	next.Release(0)

	// Prune must remove the dead flocked entry once it passes the age bound,
	// even when the process table still lists its recorded PID.
	old := time.Now().Add(-2 * PruneAgeMin)
	if err := os.Chtimes(entryPath, old, old); err != nil {
		t.Fatalf("Chtimes dead entry: %v", err)
	}
	report, err := Prune(t.Context(), root, PrunePolicy{
		Now: time.Now(),
		Age: PruneAgeMin,
		Scan: func() ([]procscan.Process, error) {
			return []procscan.Process{{PID: os.Getpid()}}, nil
		},
	})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(report.Removed) != 1 || report.Removed[0].Path != entryPath || report.Removed[0].Reason != PruneRemovedDeadEntry {
		t.Fatalf("Prune removed = %+v; kept = %+v; want dead entry removed", report.Removed, report.Kept)
	}
}

// VALIDATES: a flocked entry held by an external lock (simulating another
// process in another PID namespace whose PID does not exist locally) still
// blocks a single-slot admission until the flock is released.
func TestFlockHeldEntryBlocksAdmissionAcrossPIDNamespaces(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	jobsDir := filepath.Join(root, filepath.FromSlash(JobsDir))
	if err := os.MkdirAll(jobsDir, 0o755); err != nil {
		t.Fatalf("mkdir jobs dir: %v", err)
	}
	entryPath := filepath.Join(jobsDir, "remote.999999.claim"+entrySuffix)
	f, err := os.OpenFile(entryPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("flock entry: %v", err)
	}
	content := strings.Join([]string{
		"LABEL=remote",
		"PID=999999",
		"PGID=999999",
		"TREE=unknown",
		"KEY=unknown",
		"STARTED=" + strconv.FormatInt(time.Now().Unix(), 10),
		"LOG=",
		"STATE=running",
		"CMD=le lint",
		"FLOCK=1",
		"",
	}, "\n")
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write entry: %v", err)
	}

	adm := &Admission{Root: root, Slots: 1, Unshared: true}
	outcome := adm.claim(t.Context(), &pending{label: "asker", argv: []string{"le", "test"}, tree: Unknown, key: Unknown, unshared: true})
	if outcome.state != stateBusy {
		t.Fatalf("claim with locked remote entry = %v; want stateBusy", outcome.state)
	}
}

// VALIDATES: parseAliveOutput treats Linux kernel threads (ppid == 2) and
// zombies as gone while keeping init (ppid == 0) and ordinary user processes
// alive, and keeps non-Linux ppid == 2 processes alive.
func TestParseAliveOutputRejectsLinuxKernelThreads(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		reported string
		goos     string
		want     bool
		wantErr  bool
	}{
		{name: "linux kernel thread", reported: "S 2", goos: "linux", want: false},
		{name: "linux idle kernel thread", reported: "I 2", goos: "linux", want: false},
		{name: "linux init", reported: "S 0", goos: "linux", want: true},
		{name: "linux user process", reported: "R 1234", goos: "linux", want: true},
		{name: "linux zombie", reported: "Z 1234", goos: "linux", want: false},
		{name: "darwin ppid 2 user process", reported: "S 2", goos: "darwin", want: true},
		{name: "empty output fails closed", reported: "", goos: "linux", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseAliveOutput(tc.reported, tc.goos)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseAliveOutput(%q, %q) err = %v; wantErr %v", tc.reported, tc.goos, err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("parseAliveOutput(%q, %q) = %v; want %v", tc.reported, tc.goos, got, tc.want)
			}
		})
	}
}
