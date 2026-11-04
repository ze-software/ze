//go:build linux

// Design: docs/architecture/testing/runner-architecture.md -- cleanup intent cannot replace an earlier command outcome.
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestServerCloseAndStopPreservePreexistingSignalsWithHeldPipe fences actual
// kernel-reported signal death before retirement. A live descendant still holds
// stdout and a socket. Closing that owned group, or requesting the same signal
// after its parent's death, must not relabel the original death as intentional.
func TestServerCloseAndStopPreservePreexistingSignalsWithHeldPipe(t *testing.T) {
	for _, action := range []string{"close-kill", "stop-kill", "stop-term"} {
		t.Run(action, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			signalName, wantSignal := "KILL", syscall.SIGKILL
			if action == "stop-term" {
				signalName, wantSignal = "TERM", syscall.SIGTERM
			}
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, self, "-test.run=^TestServerPreexistingSignalParent$")
			command.Env = append(os.Environ(), "SERVER_PREEXISTING_SIGNAL="+signalName,
				"SERVER_HELD_PIPE_ADDRESS="+listener.Addr().String())
			command.Stdout, command.Stderr = io.Discard, io.Discard
			process := boundProcessDeadline(command)
			observed, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			process.observe = func(p *ownedProcess) error {
				var info unix.Siginfo
				if err := unix.Waitid(unix.P_PID, p.cmd.Process.Pid, &info, unix.WEXITED|unix.WNOWAIT, nil); err != nil {
					return err
				}
				close(observed)
				<-release
				return p.beforeReap()
			}
			attempted := make(chan struct{}, 1)
			signalGroup := process.groupSignal
			process.groupSignal = func(pid int, signal syscall.Signal) error {
				select {
				case attempted <- struct{}{}:
				default:
				}
				return signalGroup(pid, signal)
			}
			if err := plugin.StartChild(command); err != nil {
				t.Fatal(err)
			}
			server := &ServerProcess{process: process}
			go process.reap()
			t.Cleanup(func() {
				releaseOnce.Do(func() { close(release) })
				cancel()
				_ = server.Close() // The cached unexpected failure is asserted below.
			})
			if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			connection, err := listener.Accept()
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			var descendantPID int
			if err := json.NewDecoder(connection).Decode(&descendantPID); err != nil {
				t.Fatal(err)
			}
			select {
			case <-observed:
			case <-ctx.Done():
				t.Fatal("the actual kernel signal death was not observed")
			}
			process.signalMu.Lock()
			retired := process.retired
			process.signalMu.Unlock()
			if retired {
				t.Fatal("the fence did not retain the exited parent identity")
			}
			done := make(chan error, 1)
			go func() {
				if action == "close-kill" {
					done <- server.Close()
				} else {
					done <- server.Stop(wantSignal, 100*time.Millisecond, 0)
				}
			}()
			select {
			case <-attempted:
			case <-time.After(time.Second):
			case <-ctx.Done():
				t.Fatal("cleanup did not reach its bounded owned-group phase")
			}
			releaseOnce.Do(func() { close(release) })
			select {
			case result := <-done:
				var exited *exec.ExitError
				if !errors.As(result, &exited) {
					t.Errorf("%s concealed the preexisting signal death: %v", action, result)
				} else if status, ok := exited.Sys().(syscall.WaitStatus); !ok || !status.Signaled() || status.Signal() != wantSignal {
					t.Errorf("%s lost the actual preexisting signal %v: %v", action, wantSignal, result)
				}
				if repeated := server.Close(); result == nil || repeated != result {
					t.Errorf("%s did not retain its single cached unexpected failure: %v / %v", action, result, repeated)
				}
			case <-ctx.Done():
				t.Fatal("cleanup did not join its reaper after the observer fence opened")
			}
			if err := connection.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			var one [1]byte
			if _, err := connection.Read(one[:]); err == nil {
				t.Error("held-pipe descendant wrote after cleanup")
			} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				t.Error("held-pipe descendant survived cleanup")
			}
		})
	}
}

func TestServerPreexistingSignalParent(t *testing.T) {
	name := os.Getenv("SERVER_PREEXISTING_SIGNAL")
	if name == "" {
		return
	}
	child := exec.Command(os.Args[0], "-test.run=^TestServerHeldPipeProcess$")
	child.Env = append(os.Environ(), "SERVER_HELD_PIPE_ROLE=descendant")
	child.Stdout, child.Stderr = os.Stdout, os.Stderr
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	signal := syscall.SIGKILL
	if name == "TERM" {
		signal = syscall.SIGTERM
	}
	if err := syscall.Kill(os.Getpid(), signal); err != nil {
		t.Fatal(err)
	}
	select {}
}
