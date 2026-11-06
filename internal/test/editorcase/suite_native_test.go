// Design: docs/architecture/testing/runner-architecture.md -- native cancellation reporting.

package editorcase

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestEditorSuiteNativeBlockedRead reaches the registered child through the
// actual suite. A FIFO writer rendezvous proves the authored load has opened
// the FIFO before SIGTERM; keeping that writer open blocks the child's read.
// Diagnostics are reopened by the path the suite reports, after reap/cleanup.
func TestEditorSuiteNativeBlockedRead(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native signal and FIFO lifecycle proof runs on Linux")
	}
	executable := *nativeExecutable
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	if executable == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		build := exec.CommandContext(ctx, filepath.Join(root, "le"), "--name", "editor-suite-native-proof", "help")
		build.Dir = root
		build.Env = append(os.Environ(), "CGO_ENABLED=0")
		plugin.KillGroupOnCancel(build)
		if err := plugin.StartChild(build); err != nil {
			t.Fatal(err)
		}
		if err := build.Wait(); err != nil {
			t.Fatal(err)
		}
		executable = filepath.Join(root, "bin", "le-editor-suite-native-proof", "le")
	}
	for _, mode := range []string{"sigterm", "deadline"} {
		t.Run(mode, func(t *testing.T) { nativeBlockedSuite(t, executable, root, mode) })
	}
}

func nativeBlockedSuite(t *testing.T, executable, root, mode string) {
	t.Helper()
	directory := t.TempDir()
	fifo := filepath.Join(directory, "blocked.conf")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	suite := filepath.Join(directory, "suite")
	if err := os.Mkdir(suite, 0o700); err != nil {
		t.Fatal(err)
	}
	budget := "30s"
	if mode == "deadline" {
		budget = "2s"
	}
	content := "tmpfs=test.conf:terminator=END\nbgp { router-id 1.2.3.4; }\nEND\noption=file:path=test.conf\noption=timeout:value=" + budget + "\ninput=type:text=load file absolute replace " + fifo + "\ninput=enter\n"
	if err := os.WriteFile(filepath.Join(suite, "blocked.et"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	log, err := os.Create(filepath.Join(directory, "suite.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close() //nolint:errcheck // Readback is checked below.
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "test", "editor", "--dir", suite, "--all")
	cmd.Dir = root
	cmd.Stdout, cmd.Stderr = log, log
	plugin.KillGroupOnCancel(cmd)
	if err := plugin.StartChild(cmd); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	joined := false
	defer func() {
		if joined {
			return
		}
		// The suite owns its child, including failure-path cancellation.
		cmd.Process.Signal(syscall.SIGTERM) //nolint:errcheck // Already-exited is harmless during failure cleanup.
		cancel()
		<-done
	}()
	// A nonblocking writer open succeeds only when the authored reader exists.
	// Poll the kernel rendezvous, never assume startup finished after a delay.
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	var writer int
	for {
		writer, err = syscall.Open(fifo, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.ENXIO) {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			joined = true
			body, readErr := os.ReadFile(log.Name())
			t.Fatalf("suite exited before FIFO rendezvous: %v; read=%v\n%s", err, readErr, body)
		case <-ctx.Done():
			t.Fatal("suite never entered authored FIFO load")
		case <-ticker.C:
		}
	}
	defer syscall.Close(writer) //nolint:errcheck // Keep writer alive until child cleanup has been checked.
	if mode == "sigterm" {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
	}
	waitErr := <-done
	joined = true
	if ctx.Err() != nil {
		t.Fatal("suite needed the outer watchdog")
	}
	if waitErr == nil {
		t.Fatal("blocked suite reported success")
	}
	var exit *exec.ExitError
	if !errors.As(waitErr, &exit) {
		t.Fatalf("suite did not report an exit status: %v", waitErr)
	}
	if exit.ExitCode() != 1 {
		t.Fatalf("suite bypassed its failure reporter: %v", waitErr)
	}
	body, err := os.ReadFile(log.Name())
	if err != nil {
		t.Fatal(err)
	}
	report := string(body)
	cause := "context canceled"
	if mode == "deadline" {
		cause = "timed out after"
	}
	if !strings.Contains(report, cause) {
		t.Fatalf("suite lost %s:\n%s", cause, report)
	}
	_, rest, found := strings.Cut(report, "goroutine stacks: ")
	if !found {
		t.Fatalf("suite omitted diagnostic path:\n%s", report)
	}
	stacks, _, found := strings.Cut(rest, "; artifacts: ")
	if !found {
		t.Fatalf("incomplete diagnostic report: %s", rest)
	}
	_, identity, found := strings.Cut(report, "editor child pid ")
	if !found {
		t.Fatalf("suite omitted owned child identity:\n%s", report)
	}
	pidText, _, found := strings.Cut(identity, ":")
	if !found {
		t.Fatal("incomplete child identity")
	}
	childPID, err := strconv.Atoi(pidText)
	if err != nil {
		t.Fatal(err)
	}
	if childPID <= 0 {
		t.Fatal("invalid child identity")
	}
	if err := syscall.Kill(childPID, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("child survived suite exit: %v", err)
	}
	body, err = os.ReadFile(filepath.Join(filepath.Dir(stacks), "request.json"))
	if err != nil {
		t.Fatal(err)
	}
	var req request
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	if req.Content == nil {
		t.Fatal("retained request lost authored content")
	}
	if *req.Content != content {
		t.Fatal("report identifies a different authored case")
	}
	if _, err := os.Stat(req.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("work survived suite exit: %v", err)
	}
	data, err := os.ReadFile(stacks)
	if err != nil {
		t.Fatalf("reported stacks do not survive cleanup: %v", err)
	}
	if !strings.Contains(string(data), "cli.readFile") {
		t.Fatalf("diagnostic omitted blocked execution:\n%s", data)
	}
	if !strings.Contains(string(data), "testing.runTestCaseInContext") {
		t.Fatalf("diagnostic omitted editor execution:\n%s", data)
	}
	if filepath.Dir(stacks) == req.Directory {
		t.Fatal("diagnostic lies inside removed work")
	}
	t.Logf("%s: native suite failed; child %d reaped; work removed; reported diagnostic reopened: %s", mode, childPID, stacks)
	// Only artifacts belonging to the exact request above are ours to remove.
	if filepath.Dir(stacks) != filepath.Dir(req.Directory) {
		t.Fatal("reported diagnostic belongs to another case")
	}
	if err := os.RemoveAll(filepath.Dir(stacks)); err != nil {
		t.Fatal(err)
	}
}
