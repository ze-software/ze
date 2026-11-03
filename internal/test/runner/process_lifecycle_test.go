// Design: docs/architecture/testing/runner-architecture.md -- process ownership proofs.

package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestRunnerOwnedReturnPaths exercises actual command launches and independent
// return arms. Readiness follows trap installation and publication of the PID.
// wait4 must report ECHILD after return, rather than merely finding a dead PID.
func TestRunnerOwnedReturnPaths(t *testing.T) {
	zero := 0
	for _, tc := range []struct {
		name      string
		next      RunCommand
		timeout   string
		wantError string
		wantTerm  bool
	}{
		{"normal", RunCommand{Mode: modeForeground, Exec: "true"}, "5s", "", true},
		{"named-term", RunCommand{Mode: modeStop, Name: "owned", Signal: signalTerm}, "5s", "", true},
		{"named-kill", RunCommand{Mode: modeStop, Name: "owned", Signal: signalKill}, "5s", "", false},
		{"bad-argv", RunCommand{Mode: modeForeground, Exec: "sh '"}, "5s", "unclosed quote", false},
		{"missing-stdin", RunCommand{Mode: modeForeground, Exec: "cat", Stdin: "absent"}, "5s", "not found", false},
		{"start-failure", RunCommand{Mode: modeForeground, Exec: "/nonexistent/owned-runner-helper"}, "5s", "start", false},
		{"helper-failure", RunCommand{Mode: modeForeground, Exec: "sh -c 'exit 17'", ExitCode: &zero}, "5s", "expected exit code 0, got 17", false},
		{"foreground-timeout", RunCommand{Mode: modeForeground, Exec: "sleep 60"}, "200ms", "foreground helper did not exit within 200ms", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "child.pid")
			termFile := filepath.Join(t.TempDir(), "term")
			script := filepath.Join(t.TempDir(), "child.sh")
			if err := os.WriteFile(script, []byte(`trap 'echo TERM > "$OWNED_TERM_FILE"; exit 0' TERM
echo $$ > "$OWNED_PID_FILE"
echo owned-ready
while :; do sleep 0.05; done
`), 0o600); err != nil {
				t.Fatal(err)
			}
			r, rec := lastHelperRecord(t, tc.name, tc.timeout, &RunCommand{
				Mode: modeBackground, Seq: 1, Name: "owned", Ready: "owned-ready",
				Exec: `sh "` + script + `"`,
			})
			rec.EnvVars = append(rec.EnvVars, "OWNED_PID_FILE="+pidFile, "OWNED_TERM_FILE="+termFile)
			tc.next.Seq = 2
			rec.RunCommands = append(rec.RunCommands, tc.next)
			if tc.wantError == "" {
				rec.ExpectExitCode = &zero
			}
			passed := r.runTest(t.Context(), rec, &RunOptions{})
			if passed != (tc.wantError == "") {
				t.Fatalf("passed=%v, error=%v", passed, rec.Error)
			}
			if tc.wantError != "" {
				if rec.Error == nil || !strings.Contains(rec.Error.Error(), tc.wantError) {
					t.Errorf("error=%v, want %q", rec.Error, tc.wantError)
				}
			}
			assertOwnedPIDReaped(t, pidFile)
			term, err := os.ReadFile(termFile)
			if tc.wantTerm {
				if err != nil || string(term) != "TERM\n" {
					t.Errorf("SIGTERM handler=%q, %v", term, err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Errorf("forced teardown ran SIGTERM handler: %q, %v", term, err)
			}
			if _, err := os.Stat(rec.WorkDir); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("work directory remains: %v", err)
			}
			lease, err := LeaseTestPorts(rec.Port)
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Release()
			if lease.Start != rec.Port {
				t.Errorf("port lease %d was not released, got %d", rec.Port, lease.Start)
			}
		})
	}
}

func assertOwnedPIDReaped(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	var status syscall.WaitStatus
	reaped, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
	if !errors.Is(err, syscall.ECHILD) {
		t.Errorf("child %d not reaped: wait4=%d, %v", pid, reaped, err)
		if reaped == 0 {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			_, _ = syscall.Wait4(pid, &status, 0, nil)
		}
	}
}

// TestOwnedProcessCachesExit proves multiple concurrent consumers see the actual
// exit status, not the error produced by calling exec.Cmd.Wait a second time.
func TestOwnedProcessCachesExit(t *testing.T) {
	p := startOwnedTestProcess(t, exec.CommandContext(t.Context(), "sh", "-c", "exit 23"))
	results := make(chan error, 8)
	for range cap(results) {
		go func() { results <- p.Wait() }()
	}
	for range cap(results) {
		err := <-results
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 23 {
			t.Fatalf("cached exit=%v, want status 23", err)
		}
	}
	terminateGracefully(p)
	if err := p.Wait(); err != p.err {
		t.Fatalf("teardown replaced exit: %v", err)
	}
}

// TestOwnedProcessPanicCleanup exercises the same adjacent defer used by both
// runner start paths. The original panic survives and the child is reaped.
func TestOwnedProcessPanicCleanup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	func() {
		defer func() {
			if got := recover(); got != "owned-panic" {
				t.Errorf("panic=%v", got)
			}
		}()
		cmd := exec.CommandContext(t.Context(), "sh", "-c", `echo $$ > "$OWNED_PID_FILE"; echo ready; exec sleep 60`)
		cmd.Env = append(os.Environ(), "OWNED_PID_FILE="+pidFile)
		ready := newSyncWriterPattern("ready")
		cmd.Stdout = ready
		p := startOwnedTestProcess(t, cmd)
		defer p.cleanup()
		if !ready.waitFor(t.Context()) {
			t.Fatal("child did not publish readiness")
		}
		panic("owned-panic")
	}()
	assertOwnedPIDReaped(t, pidFile)
}

// TestPeerDrainExpiryJoinsOwner proves expiry is a stop-and-join, not permission
// to return with a waiter still running. A native SIGTERM trap supplies the exit.
func TestPeerDrainExpiryJoinsOwner(t *testing.T) {
	ready := newSyncWriterPattern("ready")
	cmd := exec.CommandContext(t.Context(), "sh", "-c", "trap 'exit 19' TERM; echo ready; while :; do sleep 0.05; done")
	cmd.Stdout = ready
	p := startOwnedTestProcess(t, cmd)
	if !ready.waitFor(t.Context()) {
		t.Fatal("child did not install signal handler")
	}
	peers := []peerOutput{{proc: p, checkMode: true}}
	drainPeers(peers, time.Nanosecond)
	select {
	case <-p.done:
	default:
		t.Fatal("drain returned before reaping")
	}
	var exit *exec.ExitError
	if !errors.As(p.Wait(), &exit) || exit.ExitCode() != 19 {
		t.Fatalf("drain did not preserve the native SIGTERM exit: %v", p.Wait())
	}
}

// gatedProcessOutput holds the exec copy worker at an explicit rendezvous.
// cleanup cannot finish while this writer owns an undelivered output byte.
type gatedProcessOutput struct {
	entered chan struct{}
	release chan struct{}
}

func (w *gatedProcessOutput) Write(data []byte) (int, error) {
	close(w.entered)
	<-w.release
	return len(data), nil
}

// TestOwnedCleanupPrecedesResourceRelease makes output copying block after a
// native child writes. The cleanup scope MUST retain its work and port lease
// until that copy is joined, rather than killing and releasing resources first.
func TestOwnedCleanupPrecedesResourceRelease(t *testing.T) {
	lease, err := LeaseTestPorts(0)
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(t.TempDir(), "work")
	if err := os.Mkdir(work, 0o700); err != nil {
		lease.Release()
		t.Fatal(err)
	}
	gate := &gatedProcessOutput{entered: make(chan struct{}), release: make(chan struct{})}
	cmd := exec.CommandContext(t.Context(), "sh", "-c", "printf x")
	cmd.Stdout = gate
	p := startOwnedTestProcess(t, cmd)
	<-gate.entered
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer lease.Release()
		defer os.RemoveAll(work)
		defer p.cleanup()
	}()
	select {
	case <-done:
		t.Error("cleanup returned while its output worker was still blocked")
	case <-time.After(50 * time.Millisecond):
		// Negative control: the blocked writer has no path to finish yet.
	}
	if _, err := os.Stat(work); err != nil {
		t.Errorf("work removed before output joined: %v", err)
	}
	probe, err := LeaseTestPorts(lease.Start)
	if err != nil {
		t.Error(err)
	} else {
		if probe.Start == lease.Start {
			t.Error("port lease released before output joined")
		}
		probe.Release()
	}
	close(gate.release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not join released output")
	}
	if _, err := os.Stat(work); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("work not removed after joined cleanup: %v", err)
	}
}

// TestOwnedStopEscalatesToGroupKill uses a ready barrier after installing an
// ignored SIGTERM. The native exit must be SIGKILL after the teardown grace.
func TestOwnedStopEscalatesToGroupKill(t *testing.T) {
	ready := newSyncWriterPattern("ready")
	cmd := exec.CommandContext(t.Context(), "sh", "-c", "trap '' TERM; echo ready; while :; do sleep 1; done")
	cmd.Stdout = ready
	p := startOwnedTestProcess(t, cmd)
	if !ready.waitFor(t.Context()) {
		t.Fatal("child did not install signal disposition")
	}
	err := p.stop()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("stop exit=%v, want SIGKILL", err)
	}
	status, ok := exit.Sys().(syscall.WaitStatus)
	if !ok || status.Signal() != syscall.SIGKILL {
		t.Fatalf("stop status=%v, want SIGKILL", exit.Sys())
	}
}

// TestLegacyOwnedReturnPaths reaches both successful legacy exchange teardown
// and the early client-start failure after the implicit peer became ready.
func TestLegacyOwnedReturnPaths(t *testing.T) {
	for _, failClient := range []bool{false, true} {
		t.Run(strconv.FormatBool(failClient), func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "peer.pid")
			r, rec := lastHelperRecord(t, "legacy-owner", "5s", &RunCommand{})
			rec.RunCommands = nil
			r.lePath = filepath.Join(t.TempDir(), "peer")
			body := "echo successful\n"
			if failClient {
				body = "exec sleep 60\n"
			}
			// The legacy peer receives childEnv, not rec.EnvVars.
			script := "#!/bin/sh\necho $$ > '" + pidFile + "'\necho 'listening on fixture'\n" + body
			if err := os.WriteFile(r.lePath, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			r.zePath = filepath.Join(t.TempDir(), "daemon")
			if !failClient {
				if err := os.WriteFile(r.zePath, []byte("#!/bin/sh\nexec sleep 60\n"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if passed := r.runTest(t.Context(), rec, &RunOptions{}); passed == failClient {
				t.Fatalf("passed=%v, error=%v", passed, rec.Error)
			}
			if failClient {
				if rec.Error == nil || !strings.Contains(rec.Error.Error(), "start client:") {
					t.Fatalf("error=%v, want client-start failure", rec.Error)
				}
			}
			assertOwnedPIDReaped(t, pidFile)
		})
	}
}

// TestOwnedCancellationJoinsLifetime cancels after native readiness, then checks
// the owned group exit and the lifetime worker join. No elapsed-time pass exists.
func TestOwnedCancellationJoinsLifetime(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", "echo ready; exec sleep 60")
	ready := newSyncWriterPattern("ready")
	cmd.Stdout = ready
	p, err := startWithETXTBSYRetry(ctx, cmd.Path, cmd.Args[1:], cmd, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer p.cleanup()
	startBackgroundLifetime(ctx, &RunCommand{Timeout: "1h"}, p)
	if !ready.waitFor(t.Context()) {
		t.Fatal("child did not announce readiness")
	}
	cancel()
	p.cleanup()
	select {
	case <-p.lifetimeDone:
	default:
		t.Error("cleanup did not join the lifetime worker")
	}
	var exit *exec.ExitError
	if !errors.As(p.Wait(), &exit) {
		t.Fatalf("canceled child exit=%v", p.Wait())
	}
}
