// Related: prune.go -- the removal these tests judge
// Related: overview_test.go -- tableOf, the process scan a case injects
//
// VALIDATES: plan/journal/artifact-written-with-no-remover.md, 2026-11-13.
// Prune removes the registry files that are dead by the registry's own
// formats and older than the age bound, and nothing else.
// PREVENTS: a registry that keeps every receipt for ever, and the opposite
// failure: a cleaner that removes a receipt a follower may still read, the
// entry of a running job, or a file whose process nobody could judge.
//
// No case waits for the clock: each file's age is set with os.Chtimes and
// the instant ages are measured from is injected, as is the process table.

package job

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/procscan"
)

// pruneAge is the bound every case prunes with, the one scratch clean uses.
const pruneAge = 3 * time.Hour

// The ages a case gives a file: clearly past the bound, and clearly inside it.
const (
	pruneOld   = pruneAge + time.Hour
	pruneYoung = time.Minute
)

// pruneFixture is one registry under a temp directory, and the instant its
// files' ages are measured from.
type pruneFixture struct {
	t    *testing.T
	root string
	dir  string
	now  time.Time
}

// newPruneFixture makes an empty registry.
func newPruneFixture(t *testing.T) *pruneFixture {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, JobsDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("make the registry: %v", err)
	}
	return &pruneFixture{t: t, root: root, dir: dir, now: time.Now()}
}

// file writes one registry file last written age ago, and answers its path.
func (f *pruneFixture) file(name, body string, age time.Duration) string {
	f.t.Helper()
	path := filepath.Join(f.dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil { //nolint:gosec // a registry file this test fabricates
		f.t.Fatalf("write %s: %v", name, err)
	}
	written := f.now.Add(-age)
	if err := os.Chtimes(path, written, written); err != nil {
		f.t.Fatalf("age %s: %v", name, err)
	}
	return path
}

// pruneStem spells <label>.<pid>.<claim>, the name every registry file opens
// with.
func pruneStem(label string, pid int) string {
	return entryName(label, pid, ".7001")
}

// entry writes an entry as take spells it and the log beside it, both last
// written age ago, and answers both paths.
func (f *pruneFixture) entry(label string, pid int, age time.Duration) (string, string) {
	f.t.Helper()
	stem := pruneStem(label, pid)
	var tb textbuf.Buffer
	tb.Str("LABEL=").Str(label).Byte('\n')
	tb.Str("PID=").Int(int64(pid)).Byte('\n')
	tb.Str("PGID=0\nTREE=fabricated\nKEY=fabricated\n")
	tb.Str("STARTED=").Int(f.now.Add(-age).Unix()).Byte('\n')
	tb.Str("LOG=").Str(filepath.Join(JobsDir, stem+logSuffix)).Byte('\n')
	tb.Str("STATE=running\nCMD=sleep\n")
	return f.file(stem+entrySuffix, tb.String(), age), f.file(stem+logSuffix, "a stage\n", age)
}

// receipt writes a result receipt last written age ago.
func (f *pruneFixture) receipt(label string, pid int, age time.Duration) string {
	f.t.Helper()
	return f.file(pruneStem(label, pid)+resultSuffix, "0\n", age)
}

// waiter writes a waiter record as writeWaiter spells it, last written age
// ago.
func (f *pruneFixture) waiter(label string, pid int, age time.Duration) string {
	f.t.Helper()
	var tb textbuf.Buffer
	tb.Str("LABEL=").Str(label).Byte('\n')
	tb.Str("PID=").Int(int64(pid)).Byte('\n')
	tb.Str("WAITING=").Int(f.now.Add(-age).Unix()).Byte('\n')
	tb.Str("CMD=go test ./x/\n")
	return f.file(pruneStem(label, pid)+waiterSuffix, tb.String(), age)
}

// prune runs Prune over the fixture with the given process scan.
func (f *pruneFixture) prune(scan func() ([]procscan.Process, error)) PruneReport {
	f.t.Helper()
	report, err := Prune(f.t.Context(), f.root, PrunePolicy{Now: f.now, Age: pruneAge, Scan: scan})
	if err != nil {
		f.t.Fatalf("Prune: %v", err)
	}
	return report
}

// assertGone fails unless the file was removed and the report says why.
func (f *pruneFixture) assertGone(report PruneReport, path string, want PruneReason) {
	f.t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		f.t.Errorf("%s is still there (stat error %v), want it removed", filepath.Base(path), err)
	}
	for _, row := range report.Removed {
		if row.Path == path {
			if row.Reason != want {
				f.t.Errorf("%s removed for reason %d, want %d", filepath.Base(path), row.Reason, want)
			}
			return
		}
	}
	f.t.Errorf("%s is not in the removed rows: %+v", filepath.Base(path), report.Removed)
}

// assertKept fails unless the file is still there and the report says why.
func (f *pruneFixture) assertKept(report PruneReport, path string, want PruneReason) {
	f.t.Helper()
	if _, err := os.Lstat(path); err != nil {
		f.t.Errorf("%s was removed or cannot be read (%v), want it kept", filepath.Base(path), err)
	}
	for _, row := range report.Kept {
		if row.Path == path {
			if row.Reason != want {
				f.t.Errorf("%s kept for reason %d, want %d", filepath.Base(path), row.Reason, want)
			}
			return
		}
	}
	f.t.Errorf("%s is not in the kept rows: %+v", filepath.Base(path), report.Kept)
}

// TestPruneRemovesOnlyOldReceiptsWhoseEntryIsGone: a receipt is what every
// release leaves. An old one with no entry goes; a young one stays, because a
// follower may still read it; and one whose entry is still there stays with
// that entry while its process runs.
func TestPruneRemovesOnlyOldReceiptsWhoseEntryIsGone(t *testing.T) {
	fixture := newPruneFixture(t)
	old := fixture.receipt("finished", 4001, pruneOld)
	young := fixture.receipt("recent", 4002, pruneYoung)
	entry, log := fixture.entry("running", 4003, pruneOld)
	beside := fixture.receipt("running", 4003, pruneOld)

	report := fixture.prune(tableOf(4003))

	fixture.assertGone(report, old, PruneRemovedReceipt)
	fixture.assertKept(report, young, PruneKeptYoung)
	fixture.assertKept(report, beside, PruneKeptEntryPresent)
	fixture.assertKept(report, entry, PruneKeptRunning)
	fixture.assertKept(report, log, PruneKeptEntryPresent)
	if len(report.Removed) != 1 {
		t.Fatalf("removed %d files, want the one old receipt: %+v", len(report.Removed), report.Removed)
	}
	if report.Removed[0].Bytes != 2 {
		t.Errorf("the removed receipt is reported as %d bytes, want its 2", report.Removed[0].Bytes)
	}
}

// TestPruneRemovesAnOldDeadEntryWithItsLog: a killed job leaves its entry and
// its log. Both go once the process is gone and neither was written inside
// the bound, and the receipt beside them goes in the same run. The entry of
// a running process stays whatever its age.
func TestPruneRemovesAnOldDeadEntryWithItsLog(t *testing.T) {
	fixture := newPruneFixture(t)
	deadEntry, deadLog := fixture.entry("killed", 4101, pruneOld)
	deadReceipt := fixture.receipt("killed", 4101, pruneOld)
	liveEntry, liveLog := fixture.entry("long", 4102, pruneOld)
	youngEntry, youngLog := fixture.entry("fresh", 4103, pruneYoung)

	report := fixture.prune(tableOf(4102))

	fixture.assertGone(report, deadEntry, PruneRemovedDeadEntry)
	fixture.assertGone(report, deadLog, PruneRemovedDeadEntryLog)
	fixture.assertGone(report, deadReceipt, PruneRemovedReceipt)
	fixture.assertKept(report, liveEntry, PruneKeptRunning)
	fixture.assertKept(report, liveLog, PruneKeptEntryPresent)
	fixture.assertKept(report, youngEntry, PruneKeptYoung)
	fixture.assertKept(report, youngLog, PruneKeptEntryPresent)
}

// TestPruneKeepsADeadEntryWhoseLogStillGrows: an entry's age is that of the
// newer of the entry and its log. A process the scan does not list, whose log
// was written a minute ago, is not old: the scan may be the side that is
// wrong (a process in another pid namespace).
func TestPruneKeepsADeadEntryWhoseLogStillGrows(t *testing.T) {
	fixture := newPruneFixture(t)
	entry, log := fixture.entry("unseen", 4201, pruneOld)
	written := fixture.now.Add(-pruneYoung)
	if err := os.Chtimes(log, written, written); err != nil {
		t.Fatalf("touch the log: %v", err)
	}

	report := fixture.prune(tableOf())

	fixture.assertKept(report, entry, PruneKeptYoung)
	fixture.assertKept(report, log, PruneKeptEntryPresent)
	if len(report.Removed) != 0 {
		t.Errorf("removed %+v, want nothing", report.Removed)
	}
}

// TestPruneKeepsWhatItCannotJudge: with no process table, no file that names
// a process is removed, however old; an entry whose fields cannot be read
// stays too. A receipt whose entry is gone needs no process table, and goes.
func TestPruneKeepsWhatItCannotJudge(t *testing.T) {
	fixture := newPruneFixture(t)
	entry, log := fixture.entry("unjudged", 4301, pruneOld)
	waiter := fixture.waiter("queued", 4302, pruneOld)
	broken := fixture.file(pruneStem("broken", 4303)+entrySuffix, "not a registry entry\n", pruneOld)
	receipt := fixture.receipt("finished", 4304, pruneOld)
	failing := func() ([]procscan.Process, error) { return nil, errors.New("ps did not start") }

	report := fixture.prune(failing)

	fixture.assertKept(report, entry, PruneKeptLivenessUnknown)
	fixture.assertKept(report, log, PruneKeptEntryPresent)
	fixture.assertKept(report, waiter, PruneKeptLivenessUnknown)
	fixture.assertKept(report, broken, PruneKeptUnreadable)
	fixture.assertGone(report, receipt, PruneRemovedReceipt)

	// The unreadable entry stays with a readable process table as well.
	report = fixture.prune(tableOf())
	fixture.assertKept(report, broken, PruneKeptUnreadable)
}

// TestPruneRemovesAnOldDeadWaiterRecord: a killed waiter leaves its record.
// It goes once its process is gone and it is old; a live waiter's stays, and
// so does a young one.
func TestPruneRemovesAnOldDeadWaiterRecord(t *testing.T) {
	fixture := newPruneFixture(t)
	dead := fixture.waiter("gone", 4401, pruneOld)
	live := fixture.waiter("queued", 4402, pruneOld)
	young := fixture.waiter("recent", 4403, pruneYoung)

	report := fixture.prune(tableOf(4402))

	fixture.assertGone(report, dead, PruneRemovedDeadWaiter)
	fixture.assertKept(report, live, PruneKeptRunning)
	fixture.assertKept(report, young, PruneKeptYoung)
}

// TestPruneRemovesAnOldLogWhoseEntryIsGone: a holder that died between
// removing its entry and removing its log left the log with no owner.
func TestPruneRemovesAnOldLogWhoseEntryIsGone(t *testing.T) {
	fixture := newPruneFixture(t)
	old := fixture.file(pruneStem("orphan", 4501)+logSuffix, "a stage\n", pruneOld)
	young := fixture.file(pruneStem("orphan", 4502)+logSuffix, "a stage\n", pruneYoung)

	report := fixture.prune(tableOf())

	fixture.assertGone(report, old, PruneRemovedOrphanLog)
	fixture.assertKept(report, young, PruneKeptYoung)
}

// TestPruneLeavesForeignNamesAndTheLock: only a name the registry writes,
// <label>.<pid>.<claim> and one of its four suffixes, is ever judged. Every
// other file in the directory stays, however old, and so do the lock and a
// link that carries a registry name.
func TestPruneLeavesForeignNamesAndTheLock(t *testing.T) {
	fixture := newPruneFixture(t)
	lock := fixture.file(lockName, "", pruneOld)
	foreign := []string{
		fixture.file("notes.txt", "kept by hand\n", pruneOld),
		fixture.file("lint.4601.rc", "0\n", pruneOld),
		fixture.file("lint.notapid.7001.rc", "0\n", pruneOld),
		fixture.file("lint.4601.7001.rc.bak", "0\n", pruneOld),
		fixture.file("a.b.4601.7001.rc", "0\n", pruneOld),
	}
	outside := filepath.Join(fixture.root, "outside.txt")
	if err := os.WriteFile(outside, []byte("not the registry's\n"), 0o600); err != nil {
		t.Fatalf("write the link target: %v", err)
	}
	link := filepath.Join(fixture.dir, pruneStem("linked", 4602)+resultSuffix)
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("make the link: %v", err)
	}

	report := fixture.prune(tableOf())

	for _, path := range foreign {
		fixture.assertKept(report, path, PruneKeptForeignName)
	}
	fixture.assertKept(report, link, PruneKeptForeignName)
	if _, err := os.Lstat(outside); err != nil {
		t.Errorf("the link's target is gone: %v", err)
	}
	if _, err := os.Lstat(lock); err != nil {
		t.Errorf("the registry lock is gone: %v", err)
	}
	for _, row := range slices.Concat(report.Removed, report.Kept) {
		if row.Path == lock {
			t.Errorf("the lock is reported as a registry file: %+v", row)
		}
	}
	if len(report.Removed) != 0 {
		t.Errorf("removed %+v, want nothing", report.Removed)
	}
}

// TestPruneRefusesAnAgeBelowAFollowersRead: a bound smaller than the time a
// follower may still read a receipt is refused, and nothing is removed.
func TestPruneRefusesAnAgeBelowAFollowersRead(t *testing.T) {
	fixture := newPruneFixture(t)
	receipt := fixture.receipt("finished", 4701, pruneOld)

	_, err := Prune(t.Context(), fixture.root, PrunePolicy{Now: fixture.now, Age: PruneAgeMin - time.Second, Scan: tableOf()})

	if err == nil {
		t.Fatal("Prune accepted an age bound below PruneAgeMin")
	}
	if _, statErr := os.Lstat(receipt); statErr != nil {
		t.Errorf("the receipt is gone after a refused prune: %v", statErr)
	}
}

// TestPruneOfAnAbsentRegistrySaysSo: a checkout that never ran a job has no
// registry. The report says that, and no directory or lock is created.
func TestPruneOfAnAbsentRegistrySaysSo(t *testing.T) {
	root := t.TempDir()

	report, err := Prune(t.Context(), root, PrunePolicy{Now: time.Now(), Age: pruneAge, Scan: tableOf()})

	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if !report.Absent {
		t.Errorf("report = %+v, want it to say the registry is absent", report)
	}
	if _, statErr := os.Lstat(filepath.Join(root, JobsDir)); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("the prune created the registry (stat error %v)", statErr)
	}
}

// TestAdmissionStillCountsRightAfterAPrune: the scan that decides admission
// reads the registry a prune just changed. The dead entry is gone, the live
// one still holds its slot, the lock is free again, and a new claim is
// written beside what the prune kept.
//
// Method: the live holder is this test process, which the real process query
// of scanAndClaim finds; the dead one is a child that has exited and been
// reaped.
func TestAdmissionStillCountsRightAfterAPrune(t *testing.T) {
	exited := exec.CommandContext(t.Context(), "true")
	if err := exited.Run(); err != nil {
		t.Fatalf("run the stand-in for a killed job: %v", err)
	}
	fixture := newPruneFixture(t)
	deadEntry, _ := fixture.entry("killed", exited.Process.Pid, pruneOld)
	liveEntry, _ := fixture.entry("holder", os.Getpid(), pruneOld)

	report := fixture.prune(tableOf(os.Getpid()))
	fixture.assertGone(report, deadEntry, PruneRemovedDeadEntry)
	fixture.assertKept(report, liveEntry, PruneKeptRunning)

	adm := &Admission{Root: fixture.root, Slots: 1, Out: os.Stdout, Err: os.Stderr}
	busy := adm.claim(t.Context(), &pending{label: "probe", key: "k", argv: []string{"true"}})
	if busy.state != stateBusy {
		t.Fatalf("one slot, held by a live job the prune kept: state = %d, want busy: %+v", busy.state, busy)
	}
	if busy.holder != "holder" {
		t.Errorf("the scan names %q as the holder, want the live entry the prune kept", busy.holder)
	}

	adm.Slots = 2
	claimed := adm.claim(t.Context(), &pending{label: "probe", key: "k", argv: []string{"true"}})
	if claimed.state != stateClaimed {
		t.Fatalf("two slots, one held: state = %d, want a claim: %+v", claimed.state, claimed)
	}
	if _, err := os.Lstat(adm.abs(claimed.entry)); err != nil {
		t.Errorf("the new claim's entry was not written: %v", err)
	}
}
