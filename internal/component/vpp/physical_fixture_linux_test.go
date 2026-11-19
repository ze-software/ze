//go:build integration && linux

// Design: docs/research/vpp-deployment-reference.md -- genuine model peer custody
// Related: runtime_stats_test.go -- original counter payload and readiness controls
package vpp

import (
	"errors"
	"net"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// A socketpair's peer is the actual creating test process. Capture it through
// the same production SO_PEERPIDFD/pidfs path before closing either transport.
// The test owns one independent retained witness per original API/stats input;
// no fake counter payload, logical epoch or claimed PID grants this custody.
func newRuntimeStatsTestPeer(t *testing.T) *processWitness {
	t.Helper()
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	first := os.NewFile(uintptr(pair[0]), "physical-model-peer")
	second := os.NewFile(uintptr(pair[1]), "physical-model-other")
	if first == nil {
		if err := unix.Close(pair[0]); err != nil {
			t.Error(err)
		}
		if err := unix.Close(pair[1]); err != nil {
			t.Error(err)
		}
		t.Fatal("owned socketpair returned no first file")
	}
	if second == nil {
		if err := first.Close(); err != nil {
			t.Error(err)
		}
		if err := unix.Close(pair[1]); err != nil {
			t.Error(err)
		}
		t.Fatal("owned socketpair returned no second file")
	}
	connection, openErr := net.FileConn(first)
	cleanup := errors.Join(first.Close(), second.Close())
	if err := errors.Join(openErr, cleanup); err != nil {
		if connection != nil {
			if err := connection.Close(); err != nil {
				t.Error(err)
			}
		}
		t.Fatal(err)
	}
	peer, ok := connection.(*net.UnixConn)
	if !ok {
		if err := connection.Close(); err != nil {
			t.Error(err)
		}
		t.Fatal("owned Unix socketpair returned another transport")
	}
	witness, captureErr := captureProcessWitness(peer)
	if err := errors.Join(captureErr, peer.Close()); err != nil {
		if witness != nil {
			if err := witness.release(); err != nil {
				t.Error(err)
			}
		}
		t.Fatal("Ze kernel must provide genuine SO_PEERPIDFD/pidfs for these ownership questions", err)
	}
	t.Cleanup(func() {
		if err := witness.release(); err != nil {
			t.Error(err)
		}
	})
	return witness
}
