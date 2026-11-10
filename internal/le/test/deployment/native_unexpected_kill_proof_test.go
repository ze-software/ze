//go:build linux || darwin

// Design: docs/functional-tests.md -- native command failure is not successful cleanup.
package testdeployment

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	buildcompile "github.com/ze-software/ze/internal/le/build/compile"
)

// TestNativeCommandPreservesUnexpectedKill drives the production command owner
// with a live root and an executable which kills itself, not owner cancellation.
func TestNativeCommandPreservesUnexpectedKill(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	err := runNativeCommand(ctx, exec.CommandContext(ctx, "/bin/sh", "-c", "kill -KILL $$"))
	if ctx.Err() != nil {
		t.Fatalf("the fixture root ended independently: %v", ctx.Err())
	}
	var exited *exec.ExitError
	if !errors.As(err, &exited) {
		t.Fatalf("native command concealed an unexpected SIGKILL: %v", err)
	}
	status, ok := exited.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatalf("native command lost the actual unexpected SIGKILL: %v", err)
	}
}

// TestNativeCommandCancellationIsNotUnexpectedKill is the opposite control:
// the actual owner kills a socket-observed process group because its root ends.
func TestNativeCommandCancellationIsNotUnexpectedKill(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close() //nolint:errcheck // The test owns this listening socket.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	command := exec.CommandContext(ctx, self, "-test.run=^TestScaleLifetimeProcess$")
	command.Env = append(os.Environ(), "SCALE_LIFETIME_ROLE=parent", "SCALE_LIFETIME_ADDRESS="+listener.Addr().String())
	done := make(chan error, 1)
	go func() { done <- runNativeCommand(ctx, command) }()
	owned := acceptScaleLifetime(t, listener)
	defer owned.Close() //nolint:errcheck // The test owns this accepted socket.
	var descendantPID int
	if err := json.NewDecoder(owned).Decode(&descendantPID); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("native command lost its originating cancellation: %v", err)
		}
		if _, exited := errors.AsType[*exec.ExitError](err); exited {
			t.Errorf("owner-requested cancellation was reported as an unexpected command exit: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("native cancellation did not join the command owner")
	}
	if err := owned.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := owned.Read(one[:]); err == nil {
		t.Error("native descendant wrote after cancellation returned")
	} else if timeout, ok := errors.AsType[net.Error](err); ok && timeout.Timeout() {
		t.Error("native descendant survived cancellation")
	}
}

// TestNativeBuildHostRejectsKilledCompilerWithStaleOutput enters BuildHost, not
// a substituted build runner. The isolated compiler executable either produces
// a new artifact, kills itself with an active root, or waits for root cancellation.
// A stale artifact must never turn the latter two outcomes into build success.
func TestNativeBuildHostRejectsKilledCompilerWithStaleOutput(t *testing.T) {
	for _, mode := range []string{"fresh", "kill", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			tree, _ := stageRootedBuildTree(t)
			output := filepath.Join(tree, buildcompile.HostBinary)
			if err := os.WriteFile(output, []byte("stale-build"), 0o700); err != nil {
				t.Fatal(err)
			}
			bin := t.TempDir()
			reached := filepath.Join(bin, "compiler-reached")
			const compiler = `#!/bin/sh
if [ "$1" != "build" ]; then exit 0; fi
printf 'compiler reached\n' > "$NATIVE_KILL_REACHED"
case "$NATIVE_KILL_MODE" in
fresh) printf 'fresh-build' > "$NATIVE_KILL_OUTPUT" ;;
kill) kill -KILL $$ ;;
cancel) export SCALE_LIFETIME_ROLE=parent; exec "$NATIVE_KILL_SELF" -test.run='^TestScaleLifetimeProcess$' ;;
*) exit 93 ;;
esac
`
			if err := os.WriteFile(filepath.Join(bin, "go"), []byte(compiler), 0o700); err != nil {
				t.Fatal(err)
			}
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("NATIVE_KILL_MODE", mode)
			t.Setenv("NATIVE_KILL_OUTPUT", output)
			t.Setenv("NATIVE_KILL_REACHED", reached)
			t.Setenv("NATIVE_KILL_SELF", self)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			type result struct {
				report buildcompile.BuildReport
				code   int
			}
			done := make(chan result, 1)
			var owned net.Conn
			if mode == "cancel" {
				listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close() //nolint:errcheck // The test owns this listening socket.
				t.Setenv("SCALE_LIFETIME_ADDRESS", listener.Addr().String())
				go func() {
					report, code := buildcompile.BuildHost(ctx, tree)
					done <- result{report: report, code: code}
				}()
				owned = acceptScaleLifetime(t, listener)
				defer owned.Close() //nolint:errcheck // The test owns this accepted socket.
				var descendantPID int
				if err := json.NewDecoder(owned).Decode(&descendantPID); err != nil {
					t.Fatal(err)
				}
				cancel()
			} else {
				report, code := buildcompile.BuildHost(ctx, tree)
				done <- result{report: report, code: code}
			}
			var got result
			select {
			case got = <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("BuildHost did not join its compiler")
			}
			if _, err := os.ReadFile(reached); err != nil {
				t.Fatalf("the actual compiler phase was not reached: %v", err)
			}
			if got.report.Code != got.code || got.report.Output != output {
				t.Errorf("BuildHost report disagreed with actual build outcome: %+v, code %d", got.report, got.code)
			}
			body, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "fresh" {
				if got.code != 0 || string(body) != "fresh-build" {
					t.Errorf("completed compiler lost its fresh successful artifact: code %d, output %q", got.code, body)
				}
			} else if got.code == 0 || string(body) != "stale-build" {
				t.Errorf("failed compiler treated stale output as a successful new build: code %d, output %q", got.code, body)
			}
			if mode != "cancel" && ctx.Err() != nil {
				t.Errorf("the fixture root ended independently: %v", ctx.Err())
			}
			if owned != nil {
				if err := owned.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
				var one [1]byte
				if _, err := owned.Read(one[:]); err == nil {
					t.Error("compiler descendant wrote after canceled BuildHost returned")
				} else if timeout, ok := errors.AsType[net.Error](err); ok && timeout.Timeout() {
					t.Error("compiler descendant survived canceled BuildHost")
				}
			}
		})
	}
}
