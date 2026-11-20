//go:build linux

// Design: docs/research/vpp-deployment-reference.md -- Linux pidfs process identity
// Related: process_peer_linux.go -- the pidfd exit observation under test
package vpp

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// Goal: a signal interrupting the pidfd poll of a live peer is not a failed
// observation, while an interruption that never ends still is.
// Method: a pidfd for this test process, with the poll replaced so it returns
// EINTR before reaching the real poll, then returns EINTR every time.
// MUTATION: removing the EINTR retry fails the first case, the way a counter
// read of a live VPP session failed under race-detector load.
func TestVPPPeerExitObservationRetriesInterruptedPoll(t *testing.T) {
	fd, err := unix.PidfdOpen(os.Getpid(), 0)
	if err != nil {
		t.Fatalf("pidfd unavailable, and VPP peer identity requires it: %v", err)
	}
	descriptor := &linuxProcessDescriptor{fd: fd}
	t.Cleanup(func() {
		if err := descriptor.Close(); err != nil {
			t.Error(err)
		}
	})
	t.Cleanup(func() { pollPeer = unix.Poll })

	interrupts := 0
	pollPeer = func(descriptors []unix.PollFd, timeout int) (int, error) {
		if interrupts < peerPollAttempts-1 {
			interrupts++
			return 0, unix.EINTR
		}
		return unix.Poll(descriptors, timeout)
	}
	exited, err := descriptor.Exited()
	if err != nil {
		t.Fatalf("interrupted poll of a live peer failed the observation: %v", err)
	}
	if exited {
		t.Fatal("this live process was observed as exited")
	}

	pollPeer = func([]unix.PollFd, int) (int, error) { return 0, unix.EINTR }
	if _, err := descriptor.Exited(); !errors.Is(err, unix.EINTR) {
		t.Fatalf("a poll that never completes answered: %v", err)
	}
}
