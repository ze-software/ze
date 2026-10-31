//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- network phase: IPCP on the PPP unit descriptor.
// Related: unit_reservation_integration_linux_test.go -- the wire peer this test drives.

package pppoeclient

import (
	"context"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestClientAnswersIPCPOnTheUnitDescriptor completes a real Dial against a wire
// peer, so the client's channel is connected to its PPP unit, then has the peer
// send an IPCP packet with the unused Code 12 and an IPCP Terminate-Request
// over the live PPPoE session. Linux queues both on the unit, not the channel
// (ppp_input and ppp_receive_nonmp_frame, drivers/net/ppp/ppp_generic.c), so
// the client answers them only if it reads the unit. The test requires an IPCP
// Code-Reject of the Code 12 packet, then an IPCP Terminate-Ack carrying the
// request's Identifier, then Done closed.
//
// It runs in its own user, network and mount namespace (userns.Enter), as
// TestPADTRetainsUnitUntilOwnerCleanup does, and needs /dev/ppp read-write.
func TestClientAnswersIPCPOnTheUnitDescriptor(t *testing.T) {
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
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, int(unitPeerNetworkOrder(unix.ETH_P_ALL)))
	if err != nil {
		t.Fatalf("open the wire peer's AF_PACKET socket: %v", err)
	}
	t.Cleanup(func() { _ = unix.Close(fd) })
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Ifindex: peerLink.Attrs().Index, Protocol: unitPeerNetworkOrder(unix.ETH_P_ALL)}); err != nil {
		t.Fatal(err)
	}
	if err := pppoe.SetRecvTimeout(fd, 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	// The client's own IPCP Configure-Ack during Dial lands here too.
	observed := make(chan ppp.LCPPacket, 16)
	peer := unitReservationPeer{fd: fd, ifindex: peerLink.Attrs().Index, sid: 43, observed: observed}
	copy(peer.mac[:], peerLink.Attrs().HardwareAddr)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	peerDone := make(chan error, 1)
	go func() {
		peerDone <- peer.serve(ctx)
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
	client := clientLink.Attrs().HardwareAddr

	// await returns the next IPCP packet with code the client sent, skipping
	// the others, or fails the test at the deadline.
	await := func(code uint8, what string) ppp.LCPPacket {
		t.Helper()
		for {
			select {
			case pkt := <-observed:
				if pkt.Code == code {
					return pkt
				}
			case <-ctx.Done():
				t.Fatalf("the client sent no %s: it does not read the IPCP the kernel queues on its PPP unit", what)
			}
		}
	}

	// RFC 1332 Section 2: "Other Codes should be treated as unrecognized and
	// should result in Code-Rejects."
	if err := peer.sendPPP(client, ppp.ProtoIPCP, 12, 0x42, []byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	reject := await(ppp.LCPCodeReject, "IPCP Code-Reject")
	if len(reject.Data) < 2 || reject.Data[0] != 12 || reject.Data[1] != 0x42 {
		t.Fatalf("IPCP Code-Reject rejects % x, not the Code 12 packet", reject.Data)
	}

	// RFC 1661 Section 5.5: "Upon reception of a Terminate-Request, a
	// Terminate-Ack MUST be transmitted."
	if err := peer.sendPPP(client, ppp.ProtoIPCP, ppp.LCPTerminateRequest, 0x5a, nil); err != nil {
		t.Fatal(err)
	}
	ack := await(ppp.LCPTerminateAck, "IPCP Terminate-Ack")
	if ack.Identifier != 0x5a {
		t.Fatalf("IPCP Terminate-Ack Identifier 0x%02x, want the request's 0x5a", ack.Identifier)
	}
	select {
	case <-sess.Done:
	case <-ctx.Done():
		t.Fatal("the session outlived an IPCP Terminate-Request")
	}
}
