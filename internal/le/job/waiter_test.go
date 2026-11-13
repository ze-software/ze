// Related: waiter.go -- the record these tests read
// Related: job.go -- the queue that writes and withdraws it
//
// VALIDATES: spec-main-thread-schedules-heavy-work AC-14. A job that asks for
// a slot when all are taken writes a waiter record, and removes it when it is
// admitted or canceled. The record is outside the slot count.
// PREVENTS: a queue nobody can see, a record that outlives a wait that ended,
// and a record that takes the slot it is waiting for.
//
// No case waits for the clock. Each synchronizes on registry state: the
// holder's entry, the waiter's record, a release file the holder polls for.

package job

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// queuedPair is one helper holding the only slot and a second waiting for it.
type queuedPair struct {
	holder *exec.Cmd
	waiter *exec.Cmd
	// ran is the marker the waiter's command writes once it is admitted.
	ran string
	// release lets the holder's command end.
	release func()
}

// The identity the waiting helper is started under.
const (
	queuedSession = "queued-session-1"
	queuedAgent   = "queued-agent-1"
)

// holdAndQueue starts a job that holds the only slot until released, then a
// second job that has to wait, and returns once the second has written its
// waiter record.
func holdAndQueue(t *testing.T, root string) queuedPair {
	t.Helper()
	gate := filepath.Join(root, "tmp", "release-holder")
	ran := filepath.Join(root, "tmp", "queued-ran")

	var tb textbuf.Buffer
	hold := tb.Str("until [ -e ").Str(gate).Str(" ]; do sleep 0.05; done").String()
	holder := startHelper(t, root, "holder", "sh", "-c", hold)
	waitForEntry(t, root, "holder")

	waiter := helperCommand(t, context.Background(), root, "queued",
		[]string{"CLAUDE_CODE_SESSION_ID=" + queuedSession, "ZE_AGENT_ID=" + queuedAgent},
		[]string{"sh", "-c", record(ran, "ran")})
	if err := waiter.Start(); err != nil {
		t.Fatalf("start the waiting helper: %v", err)
	}
	t.Cleanup(func() {
		if waiter.ProcessState == nil && waiter.Process != nil {
			_ = waiter.Process.Kill()
			_ = waiter.Wait()
		}
	})
	waitForWaiter(t, root, "queued")

	return queuedPair{
		holder: holder, waiter: waiter, ran: ran,
		release: func() {
			t.Helper()
			if err := os.WriteFile(gate, nil, 0o600); err != nil {
				t.Fatalf("release the holder: %v", err)
			}
		},
	}
}

// waiterRecords answers the waiter records of one label.
func waiterRecords(t *testing.T, root, label string) []string {
	t.Helper()
	var tb textbuf.Buffer
	names, err := filepath.Glob(filepath.Join(root, JobsDir, tb.Str(label).Str(".*").Str(waiterSuffix).String()))
	if err != nil {
		t.Fatalf("list the waiter records: %v", err)
	}
	return names
}

// waitForWaiter blocks until a job of this label has a readable waiter record
// and answers it. A record is created and then written, so one that does not
// parse yet is polled again.
func waitForWaiter(t *testing.T, root, label string) waiter {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, path := range waiterRecords(t, root, label) {
			if waiting, ok := readWaiter(path); ok {
				return waiting
			}
		}
		time.Sleep(helperPoll)
	}
	t.Fatalf("no %s waiter record appeared: the waiting job left no trace", label)
	return waiter{}
}

// TestWaitingJobWritesAndRemovesItsRecord drives two real processes against
// one registry with one slot. The second waits: its record carries its label,
// process, start, session, agent and command. The holder is released: the
// second is admitted, runs, and its record is gone.
func TestWaitingJobWritesAndRemovesItsRecord(t *testing.T) {
	root := fixtureRepo(t)
	began := time.Now()
	pair := holdAndQueue(t, root)

	waiting := waitForWaiter(t, root, "queued")
	if waiting.label != "queued" {
		t.Errorf("the record names label %q, want queued", waiting.label)
	}
	if waiting.pid != pair.waiter.Process.Pid {
		t.Errorf("the record names pid %d, want the waiting process %d", waiting.pid, pair.waiter.Process.Pid)
	}
	if waiting.who.session != queuedSession || waiting.who.agent != queuedAgent {
		t.Errorf("the record names owner %+v, want %s and %s", waiting.who, queuedSession, queuedAgent)
	}
	if want := "sh -c " + record(pair.ran, "ran"); waiting.command != want {
		t.Errorf("the record names command %q, want %q", waiting.command, want)
	}
	// The start is written in whole seconds, so the bound allows one.
	if waiting.since.Before(began.Add(-time.Second)) || waiting.since.After(time.Now().Add(time.Second)) {
		t.Errorf("the record says the wait started at %v, outside this test's own run", waiting.since)
	}
	if read(t, pair.ran) != "" {
		t.Fatal("the waiting job ran its command while the slot was held")
	}

	pair.release()
	if err := pair.holder.Wait(); err != nil {
		t.Fatalf("the holder: %v", err)
	}
	if err := pair.waiter.Wait(); err != nil {
		t.Fatalf("the waiting job was not admitted and run: %v", err)
	}
	if got := read(t, pair.ran); got != "ran\n" {
		t.Errorf("the admitted job's marker is %q, want its command to have run once", got)
	}
	if left := waiterRecords(t, root, "queued"); len(left) != 0 {
		t.Errorf("an admitted job left its waiter record behind: %v", left)
	}
}

// TestCanceledWaitRemovesItsRecord: the caller's cancellation is the other
// way out of a wait, and it withdraws the record too.
//
// The holder is a fabricated entry for a live process, so the slot never
// frees and the only exit is the cancel. The wait runs in-process, under a
// context this test cancels once the record is on disk.
func TestCanceledWaitRemovesItsRecord(t *testing.T) {
	detach(t)
	root := fixtureRepo(t)
	stand := sleeper(t)
	heldEntry(t, root, "busy", stand.Process.Pid)
	adm := admission(t, root)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	answered := make(chan error, 1)
	go func() {
		ticket, err := adm.AdmitContext(ctx, "stopped", []string{"true"})
		if err == nil {
			ticket.Release(0)
		}
		answered <- err
	}()

	waitForWaiter(t, root, "stopped")
	cancel()

	select {
	case err := <-answered:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("the canceled wait answered %v, want the context's cancellation", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the wait did not end when its context was canceled")
	}
	if left := waiterRecords(t, root, "stopped"); len(left) != 0 {
		t.Errorf("a canceled wait left its waiter record behind: %v", left)
	}
}

// TestWaiterCommandTextCannotForgeTheOwner writes a waiter record for a job
// whose argv holds line breaks followed by owner fields, and reads it back
// through readWaiter. The record puts CMD before SESSION and AGENT, as an
// entry does, so the same line break would name the same false owner.
//
// VALIDATES: the writer's invariant on the waiter record: the owner is the
// one the queue resolved, and the command is one line.
// PREVENTS: a waiting job that the report attributes to an argv word.
func TestWaiterCommandTextCannotForgeTheOwner(t *testing.T) {
	root := fixtureRepo(t)
	adm := admission(t, root)
	if err := os.MkdirAll(adm.jobsDir(), 0o755); err != nil { //nolint:gosec // the fixture's own registry
		t.Fatal(err)
	}

	job := &pending{
		label: "forged",
		argv:  []string{"true", "\nSESSION=evil\nAGENT=evil", "a\r\nPID=1"},
		who:   owner{session: queuedSession, agent: queuedAgent},
	}
	path, err := adm.writeWaiter(job, time.Now())
	if err != nil {
		t.Fatalf("write a waiter record: %v", err)
	}

	waiting, ok := readWaiter(path)
	if !ok {
		t.Fatal("the waiter record cannot be read")
	}
	if waiting.who.session != queuedSession || waiting.who.agent != queuedAgent {
		t.Errorf("the record names owner %+v, want %s and %s", waiting.who, queuedSession, queuedAgent)
	}
	if waiting.pid != os.Getpid() {
		t.Errorf("the record names pid %d, want this process %d", waiting.pid, os.Getpid())
	}
	if want := "true  SESSION=evil AGENT=evil a  PID=1"; waiting.command != want {
		t.Errorf("the record names command %q, want the whole text on one line: %q", waiting.command, want)
	}

	body := read(t, path)
	if strings.Contains(body, "\r") {
		t.Errorf("the record holds a carriage return an argv word supplied:\n%q", body)
	}
	for _, name := range []string{"SESSION=", "AGENT=", "PID=", "CMD="} {
		if lines := linesOpening(body, name); lines != 1 {
			t.Errorf("the record holds %d lines opening with %s, want 1:\n%s", lines, name, body)
		}
	}
}

// TestWaiterRecordNeverOccupiesASlot: a registry holding only a live waiter
// record has every slot free. A new job is admitted at once, Holders lists no
// waiter, and admission leaves the foreign record where it was.
//
// VALIDATES: AC-14's survivor, and the constraint that waiters never delete
// another process's files.
func TestWaiterRecordNeverOccupiesASlot(t *testing.T) {
	detach(t)
	root := fixtureRepo(t)
	adm := admission(t, root)
	if err := os.MkdirAll(adm.jobsDir(), 0o755); err != nil { //nolint:gosec // the fixture's own registry
		t.Fatal(err)
	}

	// The record names this test process, which is as live as a process gets.
	foreign, err := adm.writeWaiter(&pending{label: "queued", argv: []string{"sleep", "60"}}, time.Now())
	if err != nil {
		t.Fatalf("write a waiter record: %v", err)
	}

	holders, err := Holders(root)
	if err != nil {
		t.Fatalf("Holders: %v", err)
	}
	if len(holders) != 0 {
		t.Fatalf("Holders lists %+v for a registry that holds only a waiter record", holders)
	}

	// With one slot, a waiter record counted as an entry would queue this job
	// for ever; the bound turns that into a failure.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ticket, err := adm.AdmitContext(ctx, "fresh", []string{"true"})
	if err != nil {
		t.Fatalf("a job was not admitted beside a waiter record: %v", err)
	}
	defer ticket.Release(0)
	if ticket.Kind != KindClaimed {
		t.Fatalf("the job was admitted as %v, want a claimed slot", ticket.Kind)
	}

	holders, err = Holders(root)
	if err != nil {
		t.Fatalf("Holders: %v", err)
	}
	if len(holders) != 1 || holders[0].Label != "fresh" {
		t.Errorf("Holders lists %+v, want the admitted job alone", holders)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Errorf("admission touched a waiter record it did not write: %v", err)
	}
}
