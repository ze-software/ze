//go:build linux

// Design: docs/architecture/testing/runner-architecture.md -- an owned identity is not proof of a live parent.
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestServerStopRejectsPreexistingExitWithHeldPipe fences the existing exit
// observer after the kernel has reported an unreaped exit 1. A real descendant
// still holds stdout and its observer socket; the owner has not retired. Stop
// must not reinterpret that preexisting exit as its expected deliberate exit 1.
func TestServerStopRejectsPreexistingExitWithHeldPipe(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, self, "-test.run=^TestServerHeldPipeProcess$")
	command.Env = append(os.Environ(), "SERVER_HELD_PIPE_ROLE=parent", "SERVER_HELD_PIPE_ADDRESS="+listener.Addr().String())
	command.Stdout, command.Stderr = io.Discard, io.Discard
	process := boundProcessDeadline(command)
	observed := make(chan struct{})
	release := make(chan struct{})
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
		_ = server.Close() // The same cleanup failure is asserted below on ordinary paths.
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
		t.Fatal("the actual kernel exit was not observed")
	}
	process.signalMu.Lock()
	retired := process.retired
	process.signalMu.Unlock()
	if retired {
		t.Fatal("the fence did not retain the exited parent identity")
	}
	const grace = 3 * time.Second
	done := make(chan error, 1)
	go func() {
		switch stop := any(server.Stop).(type) {
		case func(time.Duration) error:
			done <- stop(grace)
		case func(syscall.Signal, time.Duration, int) error:
			done <- stop(syscall.SIGTERM, grace, 1)
		default:
			done <- errors.New("unexpected shared stop contract")
		}
	}()
	// Release only after an actual stop attempt; a correct already-exited path
	// may instead need the observer released to publish its original exit.
	select {
	case <-attempted:
	case <-time.After(grace + time.Second):
	case <-ctx.Done():
		t.Fatal("stop did not reach its bounded cleanup")
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case stopErr := <-done:
		var exited *exec.ExitError
		if !errors.As(stopErr, &exited) || exited.ExitCode() != 1 {
			t.Errorf("Stop concealed a preexisting exit behind its expected exit 1: %v", stopErr)
		}
		closeErr := server.Close()
		if stopErr == nil || closeErr == nil {
			t.Errorf("Stop/Close lost the preexisting failure: %v / %v", stopErr, closeErr)
		} else if stopErr.Error() != closeErr.Error() {
			t.Errorf("Stop/Close did not retain one cached result: %v / %v", stopErr, closeErr)
		}
	case <-ctx.Done():
		t.Fatal("Stop did not join its sole reaper after fence release")
	}
	if err := connection.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := connection.Read(one[:]); err == nil {
		t.Error("held-pipe descendant wrote after Stop")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Error("held-pipe descendant survived Stop")
	}
}

func TestServerHeldPipeProcess(t *testing.T) {
	switch os.Getenv("SERVER_HELD_PIPE_ROLE") {
	case "parent":
		child := exec.Command(os.Args[0], "-test.run=^TestServerHeldPipeProcess$")
		child.Env = append(os.Environ(), "SERVER_HELD_PIPE_ROLE=descendant")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		os.Exit(1)
	case "descendant":
		signal.Ignore(syscall.SIGTERM)
		connection, err := net.DialTimeout("tcp", os.Getenv("SERVER_HELD_PIPE_ADDRESS"), 5*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer connection.Close()
		if err := json.NewEncoder(connection).Encode(os.Getpid()); err != nil {
			t.Fatal(err)
		}
		var control [1]byte
		if _, err := connection.Read(control[:]); err == nil {
			t.Fatal("unexpected held-pipe control byte")
		}
		os.Exit(0)
	}
}
