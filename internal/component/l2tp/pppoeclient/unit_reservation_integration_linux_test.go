//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- unit ownership through Cleanup.

package pppoeclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestPADTRetainsUnitUntilOwnerCleanup completes real Dial against a wire peer,
// then receives PADT before the owner configures the returned pppN. A competing
// subscriber cannot acquire that unit until Cleanup; late MTU setup therefore
// cannot modify another subscriber's interface.
//
// The test runs unprivileged in a user, network and mount namespace of its own
// (userns.Enter), where root holds the capabilities the veth, the raw socket
// and PPPIOCNEWUNIT need, so a refusal of any of them fails the test. Opening
// /dev/ppp needs the host grant from `./le setup install` (step ppp-access).
func TestPADTRetainsUnitUntilOwnerCleanup(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	if iface.GetBackend() == nil {
		if err := iface.LoadBackend("netlink"); err != nil {
			t.Fatalf("load netlink backend: %v", err)
		}
		t.Cleanup(func() { _ = iface.CloseBackend() })
	}
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	original, err := netns.Get()
	if err != nil {
		t.Fatal(err)
	}
	ns, err := netns.New()
	if err != nil {
		_ = original.Close()
		t.Fatalf("create a network namespace inside the user namespace: %v", err)
	}
	t.Cleanup(func() {
		if err := netns.Set(original); err != nil {
			t.Errorf("restore namespace: %v", err)
		}
		_ = ns.Close()
		_ = original.Close()
	})
	// Check the prerequisite before starting a peer or allocating a session.
	probe, err := unix.Open("/dev/ppp", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("open /dev/ppp read-write: %v; grant it with `./le setup install` "+
			"(step ppp-access), then log in again so the group dip membership applies", err)
	}
	_ = unix.Close(probe)
	if err := netlink.LinkAdd(&netlink.Veth{Name: "zec0", PeerName: "zea0"}); err != nil {
		t.Fatalf("create veth zec0/zea0: %v", err)
	}
	for _, name := range []string{"zec0", "zea0"} {
		link, err := netlink.LinkByName(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := netlink.LinkSetUp(link); err != nil {
			t.Fatal(err)
		}
	}
	peerLink, err := netlink.LinkByName("zea0")
	if err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, int(pppoeac.NetworkOrder(unix.ETH_P_ALL)))
	if err != nil {
		t.Fatalf("open the wire peer's AF_PACKET socket: %v", err)
	}
	t.Cleanup(func() { _ = unix.Close(fd) })
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Ifindex: peerLink.Attrs().Index, Protocol: pppoeac.NetworkOrder(unix.ETH_P_ALL)}); err != nil {
		t.Fatal(err)
	}
	if err := pppoe.SetRecvTimeout(fd, 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	var peerMAC [pppoe.EthALen]byte
	copy(peerMAC[:], peerLink.Attrs().HardwareAddr)
	peer, err := pppoeac.New(fd, peerLink.Attrs().Index, peerMAC, 42)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	peerDone := make(chan error, 1)
	go func() {
		peerDone <- peer.ServeNoAuth(ctx, nil)
		cancel()
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-peerDone; err != nil {
			t.Errorf("wire peer: %v", err)
		}
	})
	sess, err := (&Dialer{}).Dial(iface.PPPoEClientConfig{SourceInterface: "zec0", MTU: 1492}, ctx.Done(), slog.Default())
	if err != nil {
		t.Fatalf("real Dial: %v", err)
	}
	t.Cleanup(sess.Cleanup)
	clientLink, err := netlink.LinkByName("zec0")
	if err != nil {
		t.Fatal(err)
	}
	var clientMAC [pppoe.EthALen]byte
	copy(clientMAC[:], clientLink.Attrs().HardwareAddr)
	var buf [pppoe.EthMaxLen]byte
	padt := pppoe.NewBuilder(buf[:], peerMAC, clientMAC, pppoe.CodePADT, 42)
	if err := peer.SendFrame(padt.Finish()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-sess.Done:
	case <-ctx.Done():
		t.Fatal("PADT did not stop the session")
	}
	contender, err := unix.Open("/dev/ppp", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Close(contender) })
	// PPPIOCNEWUNIT asks for the exact identity the consumer is still using.
	const newUnit = 0xc004743e
	if err := unix.IoctlSetPointerInt(contender, newUnit, sess.UnitNum); !errors.Is(err, unix.EEXIST) {
		t.Fatalf("subscriber reused ppp%d before owner Cleanup: allocation error %v, want EEXIST", sess.UnitNum, err)
	}
	name := fmt.Sprintf("ppp%d", sess.UnitNum)
	owned, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("owner lost %s before interface setup: %v", name, err)
	}
	if err := netlink.LinkSetMTU(owned, 1300); err != nil {
		t.Fatal(err)
	}
	configured, err := netlink.LinkByName(name)
	if err != nil || configured.Attrs().MTU != 1300 {
		t.Fatalf("owner MTU setup did not survive PADT: link=%v error=%v", configured, err)
	}
	sess.Cleanup()
	if err := unix.IoctlSetPointerInt(contender, newUnit, sess.UnitNum); err != nil {
		t.Fatalf("Cleanup did not release %s for reuse: %v", name, err)
	}
	reused, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetMTU(reused, 1400); err != nil {
		t.Fatal(err)
	}
	sess.Cleanup()
	reused, err = netlink.LinkByName(name)
	if err != nil || reused.Attrs().MTU != 1400 {
		t.Fatalf("repeated old Cleanup damaged replacement: link=%v error=%v", reused, err)
	}
}
