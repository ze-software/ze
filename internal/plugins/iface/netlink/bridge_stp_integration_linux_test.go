//go:build integration && linux

// Design: docs/features/interfaces.md -- bridge STP over rtnetlink.
// Related: bridge_linux.go -- BridgeSetSTP, bridgeSTPRequest.

package ifacenetlink

import (
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// kernelBridgeSTPState asks the kernel for the link at index with RTM_GETLINK
// and returns the IFLA_BR_STP_STATE the bridge reports. It reads the state
// over rtnetlink because /sys/class/net shows the namespace that mounted /sys,
// not the test's private one.
func kernelBridgeSTPState(t *testing.T, index int) uint32 {
	t.Helper()
	req := nl.NewNetlinkRequest(unix.RTM_GETLINK, 0)
	msg := nl.NewIfInfomsg(unix.AF_UNSPEC)
	msg.Index = int32(index) //nolint:gosec // a kernel ifindex is a positive int32
	req.AddData(msg)
	answers, err := req.Execute(unix.NETLINK_ROUTE, unix.RTM_NEWLINK)
	if err != nil {
		t.Fatalf("RTM_GETLINK index %d: %v", index, err)
	}
	if len(answers) != 1 {
		t.Fatalf("RTM_GETLINK index %d answered %d messages, want 1", index, len(answers))
	}
	if len(answers[0]) < unix.SizeofIfInfomsg {
		t.Fatalf("RTM_GETLINK answer is %d bytes, shorter than struct ifinfomsg", len(answers[0]))
	}
	kind, data, err := bridgeLinkInfo(answers[0][unix.SizeofIfInfomsg:])
	if err != nil {
		t.Fatalf("decode RTM_GETLINK answer: %v", err)
	}
	if kind != linkTypeBridge {
		t.Fatalf("kernel reports IFLA_INFO_KIND %q, want %q", kind, linkTypeBridge)
	}
	state, err := stpStateFrom(data)
	if err != nil {
		t.Fatalf("kernel bridge data: %v", err)
	}
	return state
}

// TestBridgeSetSTPKernel turns STP on and off through the real backend in a
// private network namespace, then reads the bridge back from the kernel over
// rtnetlink.
// VALIDATES: BridgeSetSTP changes the kernel's IFLA_BR_STP_STATE in the
// caller's namespace, both ways, and refuses a link that is not a bridge or
// does not exist, naming the device.
// PREVENTS: STP written through /sys, which a container mounts read-only and
// which shows another namespace's devices (the home CPE interop-topology lab
// failed at startup on exactly that write).
func TestBridgeSetSTPKernel(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, err := newNetlinkBackend()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	const bridge = "ze-br-stp"
	if err := backend.CreateBridge(bridge); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.DeleteInterface(bridge); err != nil {
			t.Error(err)
		}
	})
	link, err := netlink.LinkByName(bridge)
	if err != nil {
		t.Fatal(err)
	}
	index := link.Attrs().Index
	if got := kernelBridgeSTPState(t, index); got != 0 {
		t.Fatalf("new bridge reports STP state %d, want 0 before any change", got)
	}
	if err := backend.BridgeSetSTP(bridge, true); err != nil {
		t.Fatalf("BridgeSetSTP(true): %v", err)
	}
	if got := kernelBridgeSTPState(t, index); got != 1 {
		t.Fatalf("after BridgeSetSTP(true) the kernel reports STP state %d, want 1", got)
	}
	if err := backend.BridgeSetSTP(bridge, false); err != nil {
		t.Fatalf("BridgeSetSTP(false): %v", err)
	}
	if got := kernelBridgeSTPState(t, index); got != 0 {
		t.Fatalf("after BridgeSetSTP(false) the kernel reports STP state %d, want 0", got)
	}

	const dummy = "ze-br-stp-dum"
	if err := backend.CreateDummy(dummy); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.DeleteInterface(dummy); err != nil {
			t.Error(err)
		}
	})
	err = backend.BridgeSetSTP(dummy, true)
	if err == nil {
		t.Fatal("BridgeSetSTP on a dummy link succeeded, want a refusal")
	}
	if !strings.Contains(err.Error(), dummy) {
		t.Errorf("refusal %q does not name the device %q", err, dummy)
	}
	err = backend.BridgeSetSTP("ze-br-absent", true)
	if err == nil {
		t.Fatal("BridgeSetSTP on an absent bridge succeeded, want a refusal")
	}
	if !strings.Contains(err.Error(), "ze-br-absent") {
		t.Errorf("refusal %q does not name the device", err)
	}
}
