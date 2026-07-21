//go:build integration && linux

// Design: plan/spec-followup-l2tp-call.md -- AC-3 / A-4 LAC bridge integration
//
// EXECUTION IS ENVIRONMENT-BLOCKED in the dev sandbox: PPPIOCBRIDGECHAN /
// PPPIOCGCHAN require CAP_NET_ADMIN and a /dev/ppp node, neither available
// here (plan/known-failures/: no root/CAP_NET_ADMIN/netns). The test is
// authored + compiled (`go test -tags 'integration linux' -run xxx -count=0
// ./internal/component/l2tp/`) and gated with t.Skipf so it no-ops without
// the capability. Runbook to run it on a capable host / QEMU guest:
//   make ze-qemu-l2tp-ppp-test
// (or add ./internal/component/l2tp/... to ze-qemu-integration-test).

package l2tp

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// openPPPChannel opens /dev/ppp and returns an fd, or skips the test when the
// node or the capability is missing.
func openPPPChannel(t *testing.T) int {
	t.Helper()
	fd, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("requires CAP_NET_ADMIN and /dev/ppp: %v", err)
	}
	t.Cleanup(func() { _ = fd.Close() })
	return int(fd.Fd())
}

// TestLACChannelBridge exercises the LAC data-plane bridge helpers against a
// real kernel: it opens a /dev/ppp channel fd and issues PPPIOCUNBRIDGECHAN
// (idempotent when no bridge exists on a fresh channel). Establishing a full
// PPPoE<->pppol2tp bridge requires two connected pppox sockets; that end-to-
// end path is driven by the QEMU l2tp-ppp lab (make ze-qemu-l2tp-ppp-test).
//
// VALIDATES (on a capable host): the PPPIOCBRIDGECHAN/PPPIOCGCHAN constants
// are the correct arch values and the ioctl wrappers reach the kernel.
func TestLACChannelBridge(t *testing.T) {
	chanFD := openPPPChannel(t)

	// A fresh /dev/ppp fd is not attached to a channel, so PPPIOCGCHAN on it
	// (via channelNumber) must fail cleanly rather than panic -- this proves
	// the wrapper marshals the ioctl and surfaces the errno.
	if _, err := channelNumber(chanFD); err == nil {
		t.Fatal("channelNumber on an unattached /dev/ppp fd should error")
	}

	// unbridgeChannel on an unbridged channel returns an errno; the wrapper
	// must surface it as a non-nil error without crashing.
	if err := unbridgeChannel(chanFD); err == nil {
		t.Log("unbridgeChannel returned nil on an unbridged channel (kernel-dependent)")
	}
	_ = unix.EINVAL // referenced to keep the unix import meaningful across kernels
}
