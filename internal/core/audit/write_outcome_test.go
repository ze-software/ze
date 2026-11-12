package audit

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLogKeepsLastWriteFailure proves the audit log remembers why its last
// write failed, for every surface at once, and forgets it once a write lands.
// Method: a log whose directory does not exist fails two records (one CLI,
// one web), and its outcome names the error, the time, the count and the
// latest surface; once the directory exists the next record persists every
// held entry and the outcome reads clean.
//
// VALIDATES: AC-33 (spec-doctor-coverage-12-plugin-infra).
// PREVENTS: a failed audit write that only the writer's own log line records.
func TestLogKeepsLastWriteFailure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	log, err := Open(filepath.Join(dir, "ze.audit.jsonl"), MinMaxEntries)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if failure, failed := log.WriteFailure(); failed {
		t.Fatalf("a fresh log reports a write failure: %+v", failure)
	}

	before := time.Now().Add(-time.Second)
	if err := log.Record(Entry{Surface: CLI, Action: ActionDaemonReload, Outcome: OutcomeSuccess}); err == nil {
		t.Fatal("Record into a missing directory succeeded")
	}
	if err := log.Record(Entry{Surface: Web, Action: ActionConfigCommit, Outcome: OutcomeSuccess}); err == nil {
		t.Fatal("second Record into a missing directory succeeded")
	}
	failure, failed := log.WriteFailure()
	if !failed {
		t.Fatal("the log kept no write failure")
	}
	if failure.Err == "" || failure.Failures != 2 || failure.Surface != Web || failure.Action != ActionConfigCommit {
		t.Fatalf("failure = %+v; want the error, 2 failures, surface web, action config-commit", failure)
	}
	if failure.Time.Before(before) || failure.Path != filepath.Join(dir, "ze.audit.jsonl") {
		t.Fatalf("failure = %+v; want a time after %v and the log path", failure, before)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := log.Record(Entry{Surface: SSH, Action: ActionAuthFail, Outcome: OutcomeDenied}); err != nil {
		t.Fatalf("Record once the directory exists: %v", err)
	}
	if failure, failed := log.WriteFailure(); failed {
		t.Fatalf("a landed write left the failure in place: %+v", failure)
	}
	reopened, err := Open(filepath.Join(dir, "ze.audit.jsonl"), MinMaxEntries)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := len(reopened.entries); got != 3 {
		t.Fatalf("the landed write persisted %d entries; want the 3 held in memory", got)
	}
}

// TestPublishedWriteOutcome proves the daemon's log is readable by a doctor
// check, and that a process with no published log says so rather than
// answering a clean log. Method: read before and after Publish.
//
// VALIDATES: AC-33, AC-5 shape (not published is answered).
func TestPublishedWriteOutcome(t *testing.T) {
	t.Cleanup(func() { published.Store(nil) })
	published.Store(nil)
	if _, _, err := PublishedWriteFailure(); !errors.Is(err, ErrNotPublished) {
		t.Fatalf("PublishedWriteFailure with no log = %v; want ErrNotPublished", err)
	}
	log, err := NewMemory(MinMaxEntries)
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	log.Publish()
	if _, failed, err := PublishedWriteFailure(); err != nil || failed {
		t.Fatalf("PublishedWriteFailure on a clean log = failed %v, err %v; want clean", failed, err)
	}
}
