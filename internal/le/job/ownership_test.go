// Design: docs/contributing/running-commands.md -- observational admission waiters.
// Related: contention_test.go -- real processes and isolated registry fixtures.

package job

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestHelperObservationalWaiter runs the real admission loop under a signal root.
// Its report lets the parent distinguish cancellation from an admitted child.
func TestHelperObservationalWaiter(t *testing.T) {
	args := helperArgs()
	if len(args) < 3 {
		return
	}
	root, label, argv := args[0], args[1], args[2:]
	adm, err := NewIn(root)
	if err != nil {
		t.Fatal(err)
	}
	adm.Poll = helperPoll
	adm.Banner = helperPoll
	adm.Color = false
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	report, code := adm.RunContext(ctx, label, argv, root, nil)
	stopSignals()
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		t.Fatal(err)
	}
	os.Exit(code)
}

// observeQueue waits for two real waiting banners, then interrupts only its own
// waiter. The marker command must never run, and cancellation must return a report.
func observeQueue(t *testing.T, root, diagnostic string, argv ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), runContextBound)
	defer cancel()
	cmd := helperCommand(t, ctx, root, "probe", nil, argv)
	cmd.Args[1] = "-test.run=^TestHelperObservationalWaiter$"
	var answer bytes.Buffer
	cmd.Stdout = &answer
	log, err := os.CreateTemp(filepath.Join(root, "tmp"), "waiter-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			t.Error(err)
		}
	}()
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cmd.ProcessState == nil {
			stop(t, cmd)
		}
	}()

	ticker := time.NewTicker(helperPoll)
	defer ticker.Stop()
	for strings.Count(read(t, log.Name()), diagnostic) < 2 {
		select {
		case <-ctx.Done():
			t.Fatalf("waiter never reported two observations of %q: %s", diagnostic, read(t, log.Name()))
		case <-ticker.C:
		}
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("interrupt the test's waiter: %v", err)
	}
	err = cmd.Wait()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("queued waiter returned %v, want cancellation exit 2", err)
	}
	if exit.ExitCode() != 2 {
		t.Fatalf("queued waiter exited %d, want cancellation exit 2: %s", exit.ExitCode(), read(t, log.Name()))
	}
	if ctx.Err() != nil {
		t.Fatal("queued waiter needed the test deadline to stop after SIGINT")
	}
	var report Report
	if err := json.Unmarshal(answer.Bytes(), &report); err != nil {
		t.Fatalf("decode the waiter's report: %v: %s", err, answer.String())
	}
	if report.Admission != KindUnspecified.String() || report.Code != 2 || report.Label != "probe" {
		t.Errorf("canceled queue reported an admission or wrong verdict: %+v", report)
	}
	got := read(t, log.Name())
	if !strings.Contains(got, "stopped by an interrupt or SIGTERM") {
		t.Errorf("waiter did not report the operator's stop: %s", got)
	}
	return got
}

// preserveRegistryFiles records content, inode and modification time before a
// waiter scans. The returned assertion detects removal, replacement and rewriting.
func preserveRegistryFiles(t *testing.T, paths ...string) func() {
	t.Helper()
	type evidence struct {
		path string
		body []byte
		info os.FileInfo
	}
	files := make([]evidence, 0, len(paths))
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, evidence{path: path, body: body, info: info})
	}
	return func() {
		t.Helper()
		for _, file := range files {
			body, err := os.ReadFile(file.path)
			if err != nil {
				t.Errorf("foreign file did not survive: %s: %v", file.path, err)
				continue
			}
			info, err := os.Stat(file.path)
			if err != nil {
				t.Error(err)
				continue
			}
			if !bytes.Equal(body, file.body) || !os.SameFile(info, file.info) || !info.ModTime().Equal(file.info.ModTime()) {
				t.Errorf("waiter changed foreign file %s", file.path)
			}
		}
	}
}

// TestAWaiterPreservesOldForeignReceipts exercises both a scan and an owned
// release with aged foreign evidence and the obsolete shared owner copy present.
func TestAWaiterPreservesOldForeignReceipts(t *testing.T) {
	root := fixtureRepo(t)
	receipt := filepath.Join(root, JobsDir, "foreign.1.rc")
	if err := os.MkdirAll(filepath.Dir(receipt), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receipt, []byte("7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ownerCopy := filepath.Join(root, "tmp", ".ze-verify.lock.owner")
	if err := os.WriteFile(ownerCopy, []byte("foreign owner evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	age(t, receipt)
	age(t, ownerCopy)
	unchanged := preserveRegistryFiles(t, receipt, ownerCopy)
	ran := filepath.Join(root, "tmp", "ran")
	if out, code := runHelper(t, root, "probe", nil, "sh", "-c", record(ran, "admitted")); code != 0 {
		t.Fatalf("free capacity was not admitted: %d: %s", code, out)
	}
	if got := read(t, ran); got != "admitted\n" {
		t.Errorf("admitted work = %q, want one run", got)
	}
	unchanged()
}

// TestClaimsPreservePriorPIDNames proves that PID reuse cannot overwrite old
// entries, logs or receipts, and repeated claims keep distinct owned results.
func TestClaimsPreservePriorPIDNames(t *testing.T) {
	detach(t)
	root := fixtureRepo(t)
	gone := sleeper(t)
	stop(t, gone)
	oldEntry, oldLog := heldEntry(t, root, "probe", os.Getpid())
	body := strings.Replace(read(t, oldEntry), "PID="+strconv.Itoa(os.Getpid())+"\n",
		"PID="+strconv.Itoa(gone.Process.Pid)+"\n", 1)
	if err := os.WriteFile(oldEntry, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	oldReceipt := swapSuffix(oldEntry, resultSuffix)
	if err := os.WriteFile(oldReceipt, []byte("7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unchanged := preserveRegistryFiles(t, oldEntry, oldLog, oldReceipt)
	adm := admission(t, root)
	first, err := adm.Admit("probe", []string{"true"})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release(1)
	if first.Kind != KindClaimed {
		t.Fatalf("first admission = %s, want a claimed slot", first.Kind)
	}
	if adm.abs(first.Entry) == oldEntry || adm.abs(first.Log) == oldLog {
		t.Fatal("claim reused foreign evidence paths")
	}
	first.Release(3)
	unchanged()
	for _, path := range []string{first.Entry, first.Log} {
		if _, err := os.Stat(adm.abs(path)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("owner release left %s, or its removal cannot be confirmed: %v", path, err)
		}
	}
	second, err := adm.Admit("probe", []string{"true"})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release(1)
	if second.Kind != KindClaimed || second.Entry == first.Entry {
		t.Fatalf("repeated admission did not create its own claim: %+v", second)
	}
	second.Release(5)
	if code, ok := adm.readResult(first.Entry); !ok || code != 3 {
		t.Errorf("first owned receipt = (%d, %v), want (3, true)", code, ok)
	}
	if code, ok := adm.readResult(second.Entry); !ok || code != 5 {
		t.Errorf("second owned receipt = (%d, %v), want (5, true)", code, ok)
	}
	unchanged()
}

// TestAQuietHolderOccupiesExactlyOneSlot admits into a second slot without
// dropping or stopping the quiet foreign holder in the first.
func TestAQuietHolderOccupiesExactlyOneSlot(t *testing.T) {
	if runtime.NumCPU() < 2 {
		t.Skip("the admission policy permits two slots only on a host with two CPUs")
	}
	root := fixtureRepo(t)
	holder := sleeper(t)
	entry, logPath := heldEntry(t, root, "quiet-holder", holder.Process.Pid)
	age(t, logPath)
	unchanged := preserveRegistryFiles(t, entry, logPath)
	ran := filepath.Join(root, "tmp", "ran")
	if out, code := runHelper(t, root, "probe", []string{"ZE_RUN_SLOTS=2"}, "sh", "-c", record(ran, "admitted")); code != 0 {
		t.Fatalf("unused second slot was not admitted: %d: %s", code, out)
	}
	if got := read(t, ran); got != "admitted\n" {
		t.Errorf("second-slot work = %q, want one run", got)
	}
	if running, err := alive(t.Context(), holder.Process.Pid); err != nil || !running {
		t.Errorf("quiet foreign holder did not survive: running %v, error %v", running, err)
	}
	unchanged()
}

// TestProcessQueryFailureKeepsCapacityOccupied removes ps from PATH, rather
// than substituting invented process output, and checks the real scan's refusal.
func TestProcessQueryFailureKeepsCapacityOccupied(t *testing.T) {
	root := fixtureRepo(t)
	holder := sleeper(t)
	entry, logPath := heldEntry(t, root, "holder", holder.Process.Pid)
	unchanged := preserveRegistryFiles(t, entry, logPath)
	adm := admission(t, root)
	t.Setenv("PATH", "")
	result := adm.claim(t.Context(), &pending{label: "probe"})
	if result.state != stateBusy || result.liveness == nil || result.pid != holder.Process.Pid {
		t.Fatalf("failed process query did not retain the occupied slot: %+v", result)
	}
	var notes bytes.Buffer
	adm.Err = &notes
	adm.reportBusy("probe", result)
	if !strings.Contains(notes.String(), "liveness unknown") {
		t.Errorf("waiter invented a known process state: %s", notes.String())
	}
	unchanged()
}

// TestCancellationInterruptsAContendedRegistryLock holds the real scan lock
// while a canceled caller requests it. Cancellation must not wait for lockWait.
func TestCancellationInterruptsAContendedRegistryLock(t *testing.T) {
	root := fixtureRepo(t)
	adm := admission(t, root)
	if err := os.MkdirAll(adm.jobsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	held, err := adm.lockRegistry(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer held.release(adm)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	answered := make(chan error, 1)
	go func() {
		lock, err := adm.lockRegistry(ctx)
		if err == nil {
			lock.release(adm)
		}
		answered <- err
	}()
	select {
	case err := <-answered:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("contended lock answered %v, want context cancellation", err)
		}
	case <-time.After(runContextBound):
		t.Fatal("canceled waiter stayed inside the registry lock wait")
	}
}

// TestResultCreationDoesNotOverwriteExistingEvidence puts another result at
// the destination and checks that exclusive creation preserves its bytes.
func TestResultCreationDoesNotOverwriteExistingEvidence(t *testing.T) {
	root := fixtureRepo(t)
	entry := writeEntry(t, root, "foreign", "PID=1\n")
	receipt := swapSuffix(entry, resultSuffix)
	if err := os.WriteFile(receipt, []byte("7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unchanged := preserveRegistryFiles(t, entry, receipt)
	adm := admission(t, root)
	var notes bytes.Buffer
	adm.Err = &notes
	adm.writeResult(adm.rel(entry), 0)
	if notes.Len() == 0 {
		t.Error("result collision was not reported")
	}
	unchanged()
}
