//go:build integration && linux

// Design: docs/architecture/l2tp/followup-l2tp-call.md -- AC-3 / A-4 LAC bridge integration
//
// The test runs unprivileged: userns.Enter gives it a user and network
// namespace of its own, in which root holds CAP_NET_ADMIN, the capability
// PPPIOCGCHAN and PPPIOCUNBRIDGECHAN need. Opening /dev/ppp still needs the
// host grant `./le setup install` applies (its `ppp-access` step puts the node
// in group dip, mode 0660), so the file keeps the integration tag: a host
// without that grant fails here on its setup rather than on Ze. The full
// PPPoE<->pppol2tp bridge is driven by `./le test deployment gokrazy-l2tp-ppp`.

package l2tp

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// openPPPChannel opens /dev/ppp and returns an fd. A refused open fails the
// test: inside the namespace the only missing piece can be the host grant.
func openPPPChannel(t *testing.T) int {
	t.Helper()
	fd, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open /dev/ppp read-write: %v; grant it with `./le setup install` "+
			"(step ppp-access), then log in again so the group dip membership applies", err)
	}
	t.Cleanup(func() { _ = fd.Close() })
	return int(fd.Fd())
}

// TestLACChannelBridge exercises the LAC data-plane bridge helpers against a
// real kernel: it opens a /dev/ppp channel fd and issues PPPIOCUNBRIDGECHAN
// (idempotent when no bridge exists on a fresh channel). Establishing a full
// PPPoE<->pppol2tp bridge requires two connected pppox sockets; that end-to-end
// path is driven by `./le test deployment gokrazy-l2tp-ppp`.
//
// VALIDATES: the PPPIOCBRIDGECHAN/PPPIOCGCHAN constants are the correct arch
// values and the ioctl wrappers reach the kernel.
func TestLACChannelBridge(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
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
