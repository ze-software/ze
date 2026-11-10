//go:build linux || darwin

package testdeployment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/env"
)

// TestScaleStopJoinsActualDescendants observes a socket held by a TERM-ignoring
// grandchild of the actual scale process producer. Stop must close that socket
// before returning; a separately owned sentinel must remain connected.
func TestScaleStopJoinsActualDescendants(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close() //nolint:errcheck // The test owns this listening socket.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	environment := append(os.Environ(), "SCALE_LIFETIME_ADDRESS="+listener.Addr().String())
	foreignCtx, stopForeign := context.WithCancel(t.Context())
	defer stopForeign()
	foreign := exec.CommandContext(foreignCtx, self, "-test.run=^TestScaleLifetimeProcess$")
	foreign.Env = slices.Concat(environment, []string{"SCALE_LIFETIME_ROLE=descendant"})
	plugin.KillGroupOnCancel(foreign)
	if err := plugin.StartChild(foreign); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stopForeign()
		if err := foreign.Wait(); err != nil {
			if _, exited := errors.AsType[*exec.ExitError](err); !exited {
				t.Error(err)
			}
		}
	}()
	foreignConnection := acceptScaleLifetime(t, listener)
	defer foreignConnection.Close() //nolint:errcheck // The test owns this accepted socket.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	process, err := (realL2TPScaleSystem{}).Start(ctx, l2tpScaleStart{
		argv:    []string{self, "-test.run=^TestScaleLifetimeProcess$"},
		environ: append(environment, "SCALE_LIFETIME_ROLE=parent"),
	})
	if err != nil {
		t.Fatal(err)
	}
	owned := acceptScaleLifetime(t, listener)
	defer owned.Close() //nolint:errcheck // The test owns this accepted socket.
	var pid int
	if err := json.NewDecoder(owned).Decode(&pid); err != nil {
		t.Fatal(err)
	}
	if err := process.Stop(); err != nil {
		t.Errorf("scale owner did not join its process: %v", err)
	}
	if err := owned.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := owned.Read(one[:]); err == nil {
		t.Error("owned descendant wrote after Stop returned")
	} else if timeout, ok := errors.AsType[net.Error](err); ok && timeout.Timeout() {
		t.Error("owned descendant still holds its socket after Stop returned")
	}
	if err := foreignConnection.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	// Consume the foreign child's initial PID before observing its lifetime.
	var foreignPID int
	if err := json.NewDecoder(foreignConnection).Decode(&foreignPID); err != nil {
		t.Fatal(err)
	}
	if _, err := foreignConnection.Read(one[:]); err == nil {
		t.Error("foreign sentinel unexpectedly wrote")
	} else if timeout, ok := errors.AsType[net.Error](err); !ok || !timeout.Timeout() {
		t.Errorf("scale Stop touched the foreign sentinel: %v", err)
	}
}

// TestScaleEntryCancellationStopsBeforeSimulation sends TERM only after the
// registered deployment action has started its actual daemon command. The
// observer must see daemon death and an unsuccessful ordinary action exit,
// never the signal's default process exit or a later simulator invocation.
func TestScaleEntryCancellationStopsBeforeSimulation(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close() //nolint:errcheck // The test owns this listening socket.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\nexport SCALE_LIFETIME_ROLE=descendant\nexec \"$SCALE_LIFETIME_SELF\" -test.run='^TestScaleLifetimeProcess$'\n"
	if err := os.WriteFile(filepath.Join(root, "bin", "ze"), []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cmd := exec.CommandContext(ctx, self, "-test.run=^TestScaleLifetimeProcess$")
	cmd.Env = append(env.Without(os.Environ(), "ze.repo.root"),
		"ZE_REPO_ROOT="+root, "SCALE_LIFETIME_ROLE=entry", "SCALE_LIFETIME_SELF="+self,
		"SCALE_LIFETIME_ADDRESS="+listener.Addr().String())
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	plugin.KillGroupOnCancel(cmd)
	if err := plugin.StartChild(cmd); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("entry fixture did not reap")
		}
	}()
	connection := acceptScaleLifetime(t, listener)
	defer connection.Close() //nolint:errcheck // The test owns this accepted socket.
	var pid int
	if err := json.NewDecoder(connection).Decode(&pid); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		done <- err
		var exited *exec.ExitError
		if !errors.As(err, &exited) || exited.ExitCode() != 1 {
			t.Errorf("entry did not return its cancellation failure: %v\n%s", err, output.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("entry did not stop during its startup phase")
	}
	if err := connection.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := connection.Read(one[:]); err == nil {
		t.Error("daemon wrote after action verdict")
	} else if timeout, ok := errors.AsType[net.Error](err); ok && timeout.Timeout() {
		t.Error("daemon survived the canceled action verdict")
	}
}

func acceptScaleLifetime(t *testing.T, listener net.Listener) net.Conn {
	t.Helper()
	tcp, ok := listener.(*net.TCPListener)
	if !ok {
		t.Fatalf("listener %T is not a TCP listener", listener)
	}
	if err := tcp.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	connection, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	return connection
}

// TestScaleLifetimeProcess is the real executable boundary used above. Its
// parent deliberately does not give the descendant a parent-death signal:
// only the production scale owner can discharge that lifetime obligation.
func TestScaleLifetimeProcess(t *testing.T) {
	switch os.Getenv("SCALE_LIFETIME_ROLE") {
	case "entry":
		env.ResetCache()
		_, code := Answer([]string{L2TPScaleAction})
		os.Exit(code)
	case "parent":
		child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestScaleLifetimeProcess$")
		child.Env = append(os.Environ(), "SCALE_LIFETIME_ROLE=descendant")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		if err := child.Wait(); err != nil {
			t.Fatal(err)
		}
	case "descendant":
		signal.Ignore(syscall.SIGTERM)
		holdScaleLifetimeSocket(t)
		os.Exit(0)
	}
}

// holdScaleLifetimeSocket is the descendant of TestScaleLifetimeProcess: it
// publishes its PID and holds the socket until the test side closes it. Its
// deferred close runs before the caller exits the process.
func holdScaleLifetimeSocket(t *testing.T) {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	connection, err := dialer.DialContext(t.Context(), "tcp", os.Getenv("SCALE_LIFETIME_ADDRESS"))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close() //nolint:errcheck // The process exits next; the test side observes the stream.
	if err := json.NewEncoder(connection).Encode(os.Getpid()); err != nil {
		t.Fatal(err)
	}
	if err := connection.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var control [1]byte
	// A test closes its own socket on failure; no recycled numeric PID is signaled.
	if _, err := connection.Read(control[:]); err == nil {
		t.Fatal("unexpected fixture control byte")
	}
}
