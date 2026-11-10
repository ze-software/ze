// Design: docs/architecture/testing/runner-architecture.md -- editor process ownership.

package editorcase

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	editortest "github.com/ze-software/ze/internal/component/cli/testing"
	"github.com/ze-software/ze/internal/component/config/storage"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

// TestMain gives the parent real faulting child processes without adding fault
// commands or mutable hooks to the shipped worker.
func TestMain(m *testing.M) {
	if len(os.Args) == 3 {
		if os.Args[1] == "mock" && os.Args[2] == "editor-case" {
			os.Exit(parentFixture())
		}
	}
	os.Exit(m.Run())
}

func parentFixture() int {
	descriptor, err := syscall.Dup(3)
	if err != nil {
		return 89
	}
	input := os.NewFile(uintptr(descriptor), "request-inspection")
	req, err := readRequest(input)
	if closeErr := input.Close(); closeErr != nil {
		return 88
	}
	if err != nil {
		return 90
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(req.Directory), "pid"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return 91
	}
	if !strings.HasPrefix(*req.Content, "# child-fixture:") {
		return Main(nil)
	}
	mode, _, _ := strings.Cut(strings.TrimPrefix(*req.Content, "# child-fixture:"), "\n")
	mode, witness, _ := strings.Cut(mode, ":")
	if mode == "diagnostic-error" {
		return diagnosticFixture(req)
	}
	if mode == "storage-lock" {
		return blockedStoreFixture(req)
	}
	if mode == "diagnostic-blocked" {
		return blockedDiagnosticFixture(req)
	}
	output := os.NewFile(4, "result")
	result := &editortest.TestResult{Passed: true, TempDir: req.Directory}
	switch mode {
	case "late-write":
		if err := json.NewEncoder(output).Encode(result); err != nil {
			return 98
		}
		if err := os.WriteFile(witness, []byte("written after result"), 0o600); err != nil {
			return 99
		}
		return 0
	case "cleanup-error":
		if err := os.Chmod(filepath.Dir(req.Directory), 0o500); err != nil {
			return 108
		}
		err = json.NewEncoder(output).Encode(result)
	case "duplicate":
		body, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return 109
		}
		_, err = output.Write(append([]byte(`{"passed":true,`), body[1:]...))
	case "null-required":
		body, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return 110
		}
		_, err = output.WriteString(strings.Replace(string(body), `"passed":true`, `"passed":null`, 1))
	case "case-key":
		body, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return 111
		}
		_, err = output.WriteString(strings.Replace(string(body), `"passed":true`, `"Passed":true`, 1))
	case "step-partial":
		body, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return 112
		}
		_, err = output.WriteString(strings.Replace(string(body), `"steps":null`, `"steps":[{"step":1,"kind":"expect","passed":true}]`, 1))
	case "crash":
		panic("intentional child crash")
	case "missing":
		return 0
	case "null":
		_, err = output.WriteString("null")
	case "partial":
		_, err = output.WriteString(`{"passed":true`)
	case "incomplete":
		_, err = output.WriteString(`{"passed":true}`)
	case "contradictory":
		result.Error = "failure hidden by pass"
		err = json.NewEncoder(output).Encode(result)
	case "trailing":
		err = json.NewEncoder(output).Encode(result)
		if err == nil {
			_, err = output.WriteString(" {}")
		}
	case "exit-failure":
		if err := json.NewEncoder(output).Encode(result); err != nil {
			return 92
		}
		return 7
	default:
		return 93
	}
	if err != nil {
		return 94
	}
	return 0
}

func diagnosticFixture(req *request) int {
	file, err := os.Open(filepath.Join(filepath.Dir(req.Directory), "request.json"))
	if err != nil {
		return 95
	}
	if err := syscall.Dup2(int(file.Fd()), 5); err != nil {
		return 96
	}
	if err := file.Close(); err != nil {
		return 97
	}
	// The real worker owns and closes all inherited descriptors. The fixture
	// changes only the descriptor's write permission, never its verdict path.
	code := Main(nil)
	if code != 0 {
		os.Stderr.WriteString("diagnostic error: worker refused the diagnostic descriptor\n") //nolint:errcheck // Isolated failing child diagnostic.
	}
	return code
}

// blockedStoreFixture holds a real storage writer guard and blocks on a second
// acquisition. Only the process owner's hard cancellation can end this child.
func blockedStoreFixture(req *request) int {
	if err := unix.SetNonblock(6, true); err != nil {
		return 103
	}
	control := os.NewFile(6, "control")
	diagnostic := os.NewFile(5, "diagnostic")
	if err := control.SetReadDeadline(req.Deadline); err != nil {
		return 104
	}
	ctx, cancel := context.WithDeadline(context.Background(), req.Deadline)
	defer cancel()
	go watchControl(control, diagnostic, cancel, req.Deadline, make(chan struct{}), make(chan error, 1))
	store := storage.CreateMemory()
	guard, err := store.AcquireLock("fixture")
	if err != nil {
		return 105
	}
	second, err := store.AcquireLock("fixture")
	if err != nil {
		return 106
	}
	if err := errors.Join(second.Release(), guard.Release(), store.Close(), ctx.Err()); err != nil {
		return 107
	}
	return 0
}

// TestEditorParentRejectsBadChild proves process exit cannot substitute for a
// complete consistent result, and checks cleanup and reap on every refusal.
func TestEditorParentRejectsBadChild(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"crash", "missing", "null", "partial", "incomplete", "contradictory", "trailing", "exit-failure", "duplicate", "null-required", "case-key", "step-partial"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			result := Run(context.Background(), self, "# child-fixture:"+mode+"\noption=mode:value=command\n", root, 1)
			if result.Passed || result.Error == "" {
				t.Fatalf("accepted %s: %+v", mode, result)
			}
			if result.TempDir != "" {
				t.Fatalf("work cleanup incomplete: %+v", result)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatalf("expected one diagnostic artifact directory, got %d", len(entries))
			}
			artifacts := filepath.Join(root, entries[0].Name())
			if _, err := os.Stat(filepath.Join(artifacts, "work")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("work survived: %v", err)
			}
			pidText, err := os.ReadFile(filepath.Join(artifacts, "pid"))
			if err != nil {
				t.Fatal(err)
			}
			pid, err := strconv.Atoi(string(pidText))
			if err != nil {
				t.Fatal(err)
			}
			if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
				t.Fatalf("child %d not reaped: %v", pid, err)
			}
		})
	}
}

// TestEditorParentCleanupFailure keeps a real OS removal failure visible after
// a successful child assertion; the caller must not mistake it for a pass.
func TestEditorParentCleanupFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission refusal requires an unprivileged process")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	result := Run(context.Background(), self, "# child-fixture:cleanup-error\noption=mode:value=command\n", root, 1)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("missing retained ownership: %v", entries)
	}
	artifact := filepath.Join(root, entries[0].Name())
	if err := os.Chmod(artifact, 0o700); err != nil {
		t.Fatal(err)
	}
	if result.Passed || !strings.Contains(result.Error, "cleanup-incomplete") {
		t.Fatalf("%+v", result)
	}
	if result.TempDir != filepath.Join(artifact, "work") {
		t.Fatalf("lost failed-cleanup path: %+v", result)
	}
	if _, err := os.Stat(result.TempDir); err != nil {
		t.Fatalf("work retention: %v", err)
	}
}

// TestEditorParentDescriptorCloseFailure proves a close error cannot leave an
// assertion-pass marked successful, even after the working directory is removed.
func TestEditorParentDescriptorCloseFailure(t *testing.T) {
	files, err := newCaseFiles(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := files.handles[1].Close(); err != nil {
		t.Fatal(err)
	}
	result := &editortest.TestResult{Passed: true}
	files.finish(result)
	if result.Passed || !strings.Contains(result.Error, "close editor descriptor") {
		t.Fatalf("%+v", result)
	}
	if result.TempDir != "" {
		t.Fatalf("work was not removed: %+v", result)
	}
}

// TestEditorParentRetainsUnreapedOwnership controls the Wait completion signal.
// It proves the kernel-stuck state transition without claiming to create a real
// uninterruptible kernel task: no return or cleanup is allowed before that signal.
func TestEditorParentRetainsUnreapedOwnership(t *testing.T) {
	files, err := newCaseFiles(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer files.finish(&editortest.TestResult{Error: "retained-wait fixture"})
	hard, force := context.WithCancel(context.Background())
	defer force()
	child := &runningChild{done: make(chan struct{}), force: force}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.SetReadDeadline(time.Now().Add(8 * time.Second)); err != nil {
		t.Fatal(err)
	}
	previous := os.Stderr
	os.Stderr = writer
	var release sync.Once
	joined := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = child.wait(ctx, files)
		close(joined)
	}()
	defer func() {
		release.Do(func() { close(child.done) })
		<-joined
		os.Stderr = previous
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
	}()
	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "cleanup-incomplete") {
		t.Fatalf("missing unreaped state: %s", line)
	}
	select {
	case <-hard.Done():
	default:
		t.Fatal("never forced cancellation")
	}
	select {
	case <-joined:
		t.Fatal("published without reaping")
	default:
	}
	if _, err := os.Stat(files.work); err != nil {
		t.Fatalf("removed unreaped resources: %v", err)
	}
	if _, err := files.handles[1].Stat(); err != nil {
		t.Fatalf("closed unreaped descriptor: %v", err)
	}
	release.Do(func() { close(child.done) })
	<-joined
	if waitErr == nil {
		t.Fatal("lost incomplete-cleanup failure")
	}
}

// TestEditorParentNormalCleanup proves a completed child is followed by parent
// cleanup before success is returned, not by asynchronous directory removal.
func TestEditorParentNormalCleanup(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	result := Run(context.Background(), self, "option=mode:value=command\n", root, 1)
	if !result.Passed {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			log, err := os.ReadFile(filepath.Join(root, entry.Name(), "child.log"))
			if err != nil {
				t.Fatal(err)
			}
			t.Log(string(log))
		}
		t.Fatalf("%+v", result)
	}
	if result.TempDir != "" {
		t.Fatalf("cleanup not published: %+v", result)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("success left %d artifact directories", len(entries))
	}
}

// TestEditorParentBlockedIO requires real blocked file/store stacks, then proves
// reaping and removal precede publication and diagnostics survive the removal.
// It also proves the case deadline stops the run: Run must return within the
// case budget plus every timer it arms after the deadline, and a watchdog
// kills a child Run no longer stops, so a hang fails here by name rather than
// at go test's timeout.
// MUTATION: in Run (parent.go) replace child.wait(caseCtx, files) with
// child.wait(context.WithoutCancel(caseCtx), files).
func TestEditorParentBlockedIO(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"file-read", "storage-lock"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			fifo := filepath.Join(t.TempDir(), "blocked.conf")
			if err := syscall.Mkfifo(fifo, 0o600); err != nil {
				t.Fatal(err)
			}
			content := "# child-fixture:storage-lock\noption=mode:value=command\noption=timeout:value=3s\n"
			frame := "storage.(*store).acquire"
			if mode == "file-read" {
				content = "tmpfs=test.conf:terminator=END\nbgp { router-id 1.2.3.4; }\nEND\noption=file:path=test.conf\noption=timeout:value=3s\ninput=type:text=load file absolute replace " + fifo + "\ninput=enter\n"
				frame = "cli.readFile"
			}
			budget, err := editortest.ContentBudget(content, 1)
			if err != nil {
				t.Fatal(err)
			}
			// After the deadline Run waits at most the cooperative grace, then
			// the exec WaitDelay and the reap notice window after the hard kill.
			bound := budget + cooperativeGrace + childPipeWait + reapNoticeAfter
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			started := time.Now()
			answer := make(chan *editortest.TestResult, 1)
			go func() { answer <- Run(ctx, self, content, root, 1) }()
			var result *editortest.TestResult
			select {
			case result = <-answer:
			case <-ctx.Done():
				killHungEditorChild(t, root)
				result = <-answer
			}
			if elapsed := time.Since(started); elapsed > bound {
				t.Fatalf("editor case outlived its %s deadline: Run returned after %s (bound %s)", budget, elapsed, bound)
			}
			if result.Passed || !strings.Contains(result.Error, "timed out") {
				t.Fatalf("%+v", result)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatalf("missing artifacts: %v", entries)
			}
			artifact := filepath.Join(root, entries[0].Name())
			stacks, err := os.ReadFile(filepath.Join(artifact, "stacks.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(stacks), frame) {
				t.Fatalf("child never reached %s:\n%s", frame, stacks)
			}
			if _, err := os.Stat(filepath.Join(artifact, "work")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("work survived: %v", err)
			}
			pidText, err := os.ReadFile(filepath.Join(artifact, "pid"))
			if err != nil {
				t.Fatal(err)
			}
			pid, err := strconv.Atoi(string(pidText))
			if err != nil {
				t.Fatal(err)
			}
			if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
				t.Fatalf("child survived result: %v", err)
			}
		})
	}
}

// killHungEditorChild kills the child of each case under root, read from the
// pid file the fixture writes, so a Run that ignores its deadline returns and
// the test can name the hang. A missing pid fails the test at once.
func killHungEditorChild(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("hung editor case: read artifacts: %v", err)
	}
	for _, entry := range entries {
		pidText, err := os.ReadFile(filepath.Join(root, entry.Name(), "pid"))
		if err != nil {
			t.Fatalf("hung editor case left no child pid to kill: %v", err)
		}
		pid, err := strconv.Atoi(string(pidText))
		if err != nil {
			t.Fatalf("hung editor case pid %q: %v", pidText, err)
		}
		if err := unix.Kill(pid, unix.SIGKILL); err != nil && !errors.Is(err, unix.ESRCH) {
			t.Fatalf("kill hung editor child %d: %v", pid, err)
		}
	}
}

// TestEditorParentWaitsPastResult proves a result is not a completion signal:
// a child's write after encoding must have happened before Run can return.
func TestEditorParentWaitsPastResult(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	witness := filepath.Join(t.TempDir(), "after-result")
	result := Run(context.Background(), self, "# child-fixture:late-write:"+witness+"\noption=mode:value=command\n", t.TempDir(), 1)
	if !result.Passed {
		t.Fatalf("%+v", result)
	}
	data, err := os.ReadFile(witness)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "written after result" {
		t.Fatalf("wrong witness: %q", data)
	}
}

// TestEditorParentCancellation checks inherited cancellation and a diagnostic
// write failure both fail rather than accepting a missing report as success.
func TestEditorParentCancellation(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Run("root-context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		result := Run(ctx, self, "option=mode:value=command\nwait=ms:60000\n", t.TempDir(), 1)
		if result.Passed || !strings.Contains(result.Error, "deadline exceeded") {
			t.Fatalf("%+v", result)
		}
	})
	t.Run("diagnostic-error", func(t *testing.T) {
		root := t.TempDir()
		result := Run(context.Background(), self, "# child-fixture:diagnostic-error\noption=timeout:value=100ms\noption=mode:value=command\nwait=ms:60000\n", root, 1)
		if result.Passed {
			t.Fatalf("accepted failed diagnostics: %+v", result)
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Fatal("missing diagnostics directory")
		}
		log, err := os.ReadFile(filepath.Join(root, entries[0].Name(), "child.log"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(log), "diagnostic error:") {
			t.Fatalf("wrong failure: %s", log)
		}
		if !strings.Contains(string(log), "bad file descriptor") {
			t.Fatalf("worker lost diagnostic write cause: %s", log)
		}
	})
}

// TestEditorParentSteps checks the complete step envelope survives the actual
// editor child and the strict parent decoder, including the final assertion.
func TestEditorParentSteps(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	content := "tmpfs=test.conf:terminator=END\nbgp { router-id 1.2.3.4; }\nEND\noption=file:path=test.conf\ninput=type:text=set bgp router-id 5.6.7.8\ninput=enter\nexpect=dirty:true\n"
	result := Run(context.Background(), self, content, t.TempDir(), 1)
	if !result.Passed {
		t.Fatalf("%+v", result)
	}
	if len(result.Steps) != 3 {
		t.Fatalf("lost steps: %+v", result)
	}
	last := result.Steps[2]
	if !last.Passed || last.Kind != "expect" || last.Assert != "dirty" {
		t.Fatalf("lost assertion: %+v", last)
	}
}

// TestEditorParentPreflight checks refusal before any child resources or effects.
func TestEditorParentPreflight(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"invalid-budget", "canceled", "headroom"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			content, headroom := "option=mode:value=command\n", 1
			switch mode {
			case "invalid-budget":
				content += "option=timeout:value=invalid\n"
			case "canceled":
				cancel()
			case "headroom":
				headroom = 0
			}
			root := t.TempDir()
			result := Run(ctx, self, content, root, headroom)
			if result.Passed || result.Error == "" {
				t.Fatalf("%+v", result)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("started effects before refusal: %v", entries)
			}
		})
	}
}
