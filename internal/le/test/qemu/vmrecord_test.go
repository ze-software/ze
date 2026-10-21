// Overview: vmrecord.go -- the record that lets the next run stop a VM whose le died
package testqemu

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// reapFixture is a Run whose process table is a map the test writes: a pid
// absent from starts is gone.
type reapFixture struct {
	run    *Run
	starts map[int]int64
	killed []int
	log    strings.Builder
}

func newReapFixture(t *testing.T) *reapFixture {
	t.Helper()
	f := &reapFixture{starts: map[int]int64{}}
	f.run = NewRun(t.TempDir(), RunOptions{})
	f.run.ops.StartTime = func(pid int) (int64, error) {
		start, alive := f.starts[pid]
		if !alive {
			return 0, errProcessGone
		}
		return start, nil
	}
	f.run.ops.Kill = func(pid int) error {
		f.killed = append(f.killed, pid)
		return nil
	}
	return f
}

// write stores record as the run would have, and answers its path.
func (f *reapFixture) write(t *testing.T, record vmRecord) string {
	t.Helper()
	dir := f.run.vmRecordDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, vmRecordName(record.QEMUPid))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func recordExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// VALIDATES: the reap at the start of a run stops a QEMU only when the le that
// recorded it is gone (its pid dead, or reused by a process that started at
// another time) AND the QEMU pid still names the process that was recorded;
// every other record is left alone or dropped, and never killed (review B1).
// METHOD: one record per case over a fake process table.
// PREVENTS: a VM orphaned on the Mac, where no Pdeathsig exists, holding its
// port and cores forever; and a reap that kills another session's live VM, or
// an unrelated process that reused a dead QEMU's pid.
func TestReapStopsOnlyAQEMUWhoseLeIsGone(t *testing.T) {
	for _, tc := range []struct {
		name   string
		starts map[int]int64
		kill   bool
		kept   bool
		says   string
	}{
		{"le gone, QEMU still the recorded one", map[int]int64{10: 100}, true, false, "stopped QEMU pid 10"},
		{"le pid reused by a later process", map[int]int64{10: 100, 20: 999}, true, false, "stopped QEMU pid 10"},
		{"le alive: another session's live VM", map[int]int64{10: 100, 20: 200}, false, true, ""},
		{"le gone, QEMU pid reused", map[int]int64{10: 555}, false, false, ""},
		{"le gone, QEMU gone", map[int]int64{}, false, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newReapFixture(t)
			f.starts = tc.starts
			path := f.write(t, vmRecord{QEMUPid: 10, QEMUStart: 100, LePid: 20, LeStart: 200})
			f.run.reapOrphanVMs(&f.log)
			if killed := len(f.killed) == 1 && f.killed[0] == 10; killed != tc.kill {
				t.Errorf("killed %v, want a kill of QEMU pid 10: %v", f.killed, tc.kill)
			}
			if recordExists(path) != tc.kept {
				t.Errorf("record kept = %v, want %v", recordExists(path), tc.kept)
			}
			if tc.says != "" && !strings.Contains(f.log.String(), tc.says) {
				t.Errorf("the reap said %q, want it to name %q", f.log.String(), tc.says)
			}
		})
	}
}

// VALIDATES: a record whose le cannot be judged (the start-time read fails for
// a reason other than a missing process) is kept and named, never acted on.
// PREVENTS: an unreadable process table read as "le gone", which kills a VM
// another session still drives.
func TestReapKeepsARecordItCannotJudge(t *testing.T) {
	f := newReapFixture(t)
	f.run.ops.StartTime = func(pid int) (int64, error) {
		if pid == 20 {
			return 0, errors.New("sysctl refused")
		}
		return 100, nil
	}
	path := f.write(t, vmRecord{QEMUPid: 10, QEMUStart: 100, LePid: 20, LeStart: 200})
	f.run.reapOrphanVMs(&f.log)
	if len(f.killed) != 0 {
		t.Errorf("killed %v on a record nobody could judge", f.killed)
	}
	if !recordExists(path) {
		t.Error("the record was dropped although nobody could judge it")
	}
	if !strings.Contains(f.log.String(), "sysctl refused") {
		t.Errorf("the reap did not name why it left the record: %q", f.log.String())
	}
}

// VALIDATES: an unreadable record younger than vmRecordUnreadableAge is kept
// and named, and only an older one is dropped (review round 2, ISSUE 1).
// PREVENTS: a reap deleting a record a concurrent run is still writing, which
// leaves that VM unrecorded and unreapable.
func TestReapKeepsAYoungUnreadableRecord(t *testing.T) {
	f := newReapFixture(t)
	dir := f.run.vmRecordDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	young := filepath.Join(dir, vmRecordName(10))
	old := filepath.Join(dir, vmRecordName(11))
	for _, path := range []string{young, old} {
		if err := os.WriteFile(path, []byte(`{"qemu-pid":`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stale := time.Now().Add(-2 * vmRecordUnreadableAge)
	if err := os.Chtimes(old, stale, stale); err != nil {
		t.Fatal(err)
	}
	f.run.reapOrphanVMs(&f.log)
	if !recordExists(young) {
		t.Error("a young unreadable record was dropped")
	}
	if recordExists(old) {
		t.Error("an old unreadable record was kept")
	}
	if !strings.Contains(f.log.String(), "kept the unreadable VM record "+young) {
		t.Errorf("the reap did not name the record it kept: %q", f.log.String())
	}
	if len(f.killed) != 0 {
		t.Errorf("killed %v on records that name no process", f.killed)
	}
}

// VALIDATES: a staged record older than vmRecordUnreadableAge, left by a le
// killed between staging and rename, is removed by the reap; a young one,
// which a live run may be about to rename, is kept (round 3, NOTE 2).
func TestReapRemovesAnOldStagedRecord(t *testing.T) {
	f := newReapFixture(t)
	dir := f.run.vmRecordDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	young := filepath.Join(dir, vmRecordStagedPrefix+"young")
	old := filepath.Join(dir, vmRecordStagedPrefix+"old")
	for _, path := range []string{young, old} {
		if err := os.WriteFile(path, []byte(`{"qemu-pid":`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stale := time.Now().Add(-2 * vmRecordUnreadableAge)
	if err := os.Chtimes(old, stale, stale); err != nil {
		t.Fatal(err)
	}
	f.run.reapOrphanVMs(&f.log)
	if !recordExists(young) {
		t.Error("a young staged record was removed")
	}
	if recordExists(old) {
		t.Error("an old staged record was kept")
	}
}

// VALIDATES: recordVM publishes the record whole, by a rename, and leaves no
// temporary file behind.
func TestRecordVMLeavesOnlyTheRecord(t *testing.T) {
	f := newReapFixture(t)
	f.starts = map[int]int64{10: 100, os.Getpid(): 200}
	remove, err := f.run.recordVM(10)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(f.run.vmRecordDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != vmRecordName(10) {
		t.Errorf("record directory holds %v, want only %s", entries, vmRecordName(10))
	}
	remove()
	if recordExists(filepath.Join(f.run.vmRecordDir(), vmRecordName(10))) {
		t.Error("remove left the record")
	}
}

// VALIDATES: executePlan records its QEMU with le's own pid and both start
// times while the VM runs, and removes the record once the VM is stopped.
// METHOD: the fake QEMU console; the fake guest ssh reads the record directory
// while it runs.
// PREVENTS: a run that leaves no record for the reap, or a stale record that
// a later reap reads after a clean stop.
func TestExecutePlanRecordsTheVMWhileItRuns(t *testing.T) {
	f := newGuestStopFixture(t)
	var seen []vmRecord
	f.answer(t, func(context.Context) (commandResult, error) {
		entries, err := os.ReadDir(f.run.vmRecordDir())
		if err != nil {
			return commandResult{}, err
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), vmRecordPrefix) {
				continue
			}
			data, err := os.ReadFile(filepath.Join(f.run.vmRecordDir(), entry.Name()))
			if err != nil {
				return commandResult{}, err
			}
			var record vmRecord
			if err := json.Unmarshal(data, &record); err != nil {
				return commandResult{}, err
			}
			seen = append(seen, record)
		}
		return commandResult{}, nil
	}, func() (commandResult, error) { return commandResult{}, nil })

	if _, err := f.run.executePlan(t.Context(), &f.report); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 {
		t.Fatalf("records while the VM ran = %v, want one", seen)
	}
	record := seen[0]
	if record.LePid != os.Getpid() || record.QEMUPid == 0 || record.QEMUStart == 0 || record.LeStart == 0 {
		t.Errorf("record = %+v, want le pid %d and both start times", record, os.Getpid())
	}
	entries, err := os.ReadDir(f.run.vmRecordDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), vmRecordPrefix) {
			t.Errorf("record left after the VM stopped: %s", entry.Name())
		}
	}
}
