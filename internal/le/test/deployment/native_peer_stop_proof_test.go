//go:build linux || darwin

// Design: docs/functional-tests.md -- native peer-first teardown retains the peer's exit contract.
package testdeployment

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

// TestNativePeerStopDistinguishesDeliberateExit drives the actual watched native
// process and stopNative operation. xl2tpd v1.3.18 death_handler sends StopCCN,
// cleans its files and exits 1 after SIGTERM; that deliberate status must not
// fail teardown, while the same status before Stop must remain an error.
// Source: https://github.com/xelerance/xl2tpd/blob/v1.3.18/xl2tpd.c#L308-L350.
func TestNativePeerStopDistinguishesDeliberateExit(t *testing.T) {
	for _, role := range []string{"term", "preexit"} {
		t.Run(role, func(t *testing.T) {
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
			command := exec.CommandContext(ctx, self, "-test.run=^TestNativePeerExitProcess$")
			command.Env = append(os.Environ(), "NATIVE_PEER_EXIT_ROLE="+role,
				"NATIVE_PEER_EXIT_ADDRESS="+listener.Addr().String())
			seen := newCollector()
			process, err := startWatched(command, "peer> ", seen, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer seen.wait()
			defer process.closeNative() //nolint:errcheck // Both ordinary cleanup outcomes are asserted below.
			connection := acceptScaleLifetime(t, listener)
			defer connection.Close()
			decoder := json.NewDecoder(connection)
			var pid int
			if err := decoder.Decode(&pid); err != nil {
				t.Fatal(err)
			}
			if role == "preexit" {
				if _, err := connection.Write([]byte{1}); err != nil {
					t.Fatal(err)
				}
				select {
				case <-process.done:
				case <-ctx.Done():
					t.Fatal("unexpected-exit control never reached the actual sole reaper")
				}
			}
			stopErr := process.stopNative()
			closeErr := process.closeNative()
			if role == "term" {
				var received int
				if err := decoder.Decode(&received); err != nil {
					t.Fatalf("the peer did not receive its deliberate stop: %v", err)
				}
				if received != int(syscall.SIGTERM) {
					t.Errorf("peer received signal %d, want TERM", received)
				}
				if stopErr != nil || closeErr != nil {
					t.Errorf("documented deliberate exit 1 failed teardown: Stop=%v Close=%v", stopErr, closeErr)
				}
				return
			}
			for _, result := range []error{stopErr, closeErr} {
				var exited *exec.ExitError
				if !errors.As(result, &exited) || exited.ExitCode() != 1 {
					t.Errorf("unexpected pre-stop exit 1 was concealed: %v", result)
				}
			}
			if stopErr == nil || closeErr == nil {
				return
			}
			if stopErr.Error() != closeErr.Error() {
				t.Errorf("Stop and Close did not retain one cached verdict: %v / %v", stopErr, closeErr)
			}
		})
	}
}

// TestNativePeerExitProcess is the real executable peer boundary. Signal
// readiness is installed before publishing the socket handshake; exit 1 is
// reached only by a received TERM or the independently controlled early exit.
func TestNativePeerExitProcess(t *testing.T) {
	role := os.Getenv("NATIVE_PEER_EXIT_ROLE")
	if role == "" {
		return
	}
	received := make(chan os.Signal, 1)
	signal.Notify(received, syscall.SIGTERM)
	defer signal.Stop(received)
	connection, err := net.DialTimeout("tcp", os.Getenv("NATIVE_PEER_EXIT_ADDRESS"), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := json.NewEncoder(connection).Encode(os.Getpid()); err != nil {
		t.Fatal(err)
	}
	if role == "term" {
		select {
		case delivered := <-received:
			if err := json.NewEncoder(connection).Encode(int(delivered.(syscall.Signal))); err != nil {
				t.Fatal(err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("native stop never delivered a signal")
		}
	} else {
		if err := connection.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatal(err)
		}
		var release [1]byte
		if _, err := io.ReadFull(connection, release[:]); err != nil {
			t.Fatal(err)
		}
	}
	os.Exit(1)
}
