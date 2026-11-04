//go:build linux || darwin

package testdeployment

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

// TestScaleSimulatorCancellationJoinsActualDescendants cancels the actual
// simulator command producer after its grandchild holds an observer socket.
// The command must preserve cancellation and join the group before returning.
func TestScaleSimulatorCancellationJoinsActualDescendants(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := (realL2TPScaleSystem{}).Run(ctx, l2tpScaleCommand{
			argv:    []string{self, "-test.run=^TestScaleLifetimeProcess$"},
			environ: append(os.Environ(), "SCALE_LIFETIME_ROLE=parent", "SCALE_LIFETIME_ADDRESS="+listener.Addr().String()),
			timeout: 30 * time.Second,
		})
		done <- err
	}()
	connection := acceptScaleLifetime(t, listener)
	defer connection.Close()
	var pid int
	if err := json.NewDecoder(connection).Decode(&pid); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := connection.Close(); err != nil {
			t.Error(err)
		}
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("actual simulator command could not be joined even after fixture cleanup")
		}
	}()
	cancel()
	select {
	case err := <-done:
		done <- err
		if !errors.Is(err, context.Canceled) {
			t.Errorf("simulator lost originating cancellation: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled simulator waited on a surviving descendant")
	}
	if err := connection.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := connection.Read(one[:]); err == nil {
		t.Error("simulator descendant wrote after command returned")
	} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Error("simulator descendant survived the canceled command")
	}
}

// TestScaleActualProcessNormalCompletion retains the noncanceled controls for
// both real producers using the ordinary test binary's successful empty test
// selection, rather than an echo script standing in for command execution.
func TestScaleActualProcessNormalCompletion(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	argv := []string{self, "-test.run=^NoScaleLifetimeFixtureSelected$"}
	process, err := (realL2TPScaleSystem{}).Start(t.Context(), l2tpScaleStart{argv: argv, environ: os.Environ()})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := process.Stop(); err != nil {
			t.Error(err)
		}
	}()
	until := time.NewTimer(5 * time.Second)
	defer until.Stop()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		exited, code, stderr, err := process.Exited()
		if err != nil {
			t.Fatal(err)
		}
		if exited {
			if code != 0 {
				t.Fatalf("ordinary daemon command failed: %d, %s", code, stderr)
			}
			break
		}
		select {
		case <-until.C:
			t.Fatal("ordinary daemon command did not finish")
		case <-poll.C:
		}
	}
	result, err := (realL2TPScaleSystem{}).Run(t.Context(), l2tpScaleCommand{argv: argv, environ: os.Environ(), timeout: 5 * time.Second})
	if err != nil || result.code != 0 {
		t.Fatalf("ordinary simulator command failed: %+v, %v", result, err)
	}
}
