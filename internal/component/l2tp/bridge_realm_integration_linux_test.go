//go:build integration && linux

// Design: docs/architecture/l2tp/followup-l2tp-call.md -- realm relay LAC bridge
// Related: bridge_integration_linux_test.go -- TestLACChannelBridge, the ioctl wrappers alone
// Related: export_realm_bridge_linux_test.go -- the kernel L2TP session these tests open
//
// These tests run unprivileged inside userns.Enter's namespace, like
// TestLACChannelBridge, and need the same /dev/ppp grant from
// `./le setup install` (step ppp-access). The kernel needs pppoe, l2tp_core,
// l2tp_netlink and l2tp_ppp, which Ze's kernel carries. They live in package
// l2tp_test because the PPPoE package they drive imports package l2tp.

package l2tp_test

import (
	"bytes"
	"errors"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/internal/test/userns"
)

// realmBridgeRig is the kernel path a realm-relayed subscriber crosses, built
// the way the LAC builds it: the PPPoE server's session socket and /dev/ppp
// channel (pppoe.PPPoECreate, ppp.DevPPPSetup, as handlePADR does), and the
// kernel L2TP tunnel, session and pppol2tp channel
// (l2tp.OpenKernelL2TPSession, ppp.DevPPPSetup, as setupSession and
// pppSetupReal do). The far ends are a
// raw subscriber on the veth pair and a plain UDP socket standing for the
// LNS, so a test sees exactly which frames the kernel delivers where.
type realmBridgeRig struct {
	subscriber    *pppoeac.Peer
	acMAC         [pppoe.EthALen]byte
	pppoeFD       int // the subscriber's PPPoE session socket
	driverChanFD  int // the /dev/ppp channel the PPP driver reads
	l2tpChanFD    int // the /dev/ppp channel of the pppol2tp socket
	lnsFD         int // the LNS's end of the tunnel
	driverChanSet bool
}

const (
	realmRigSessionID = 0x2e
	realmRigLocalTID  = 101
	realmRigLocalSID  = 102
	realmRigRemoteTID = 201
	realmRigRemoteSID = 202
)

// newRealmBridgeRig builds the rig inside the test's own namespace. Every
// descriptor and kernel object it makes is released by t.Cleanup.
func newRealmBridgeRig(t *testing.T) *realmBridgeRig {
	t.Helper()
	rig := &realmBridgeRig{}
	rig.subscriber = pppoeac.OpenVeth(t, "zrl0", "zrs0", realmRigSessionID)
	acLink, err := netlink.LinkByName("zrl0")
	if err != nil {
		t.Fatal(err)
	}
	copy(rig.acMAC[:], acLink.Attrs().HardwareAddr)
	subscriberLink, err := netlink.LinkByName("zrs0")
	if err != nil {
		t.Fatal(err)
	}
	var subscriberMAC [pppoe.EthALen]byte
	copy(subscriberMAC[:], subscriberLink.Attrs().HardwareAddr)

	// The PPPoE side, as handlePADR builds it for a session that runs PPP
	// locally first.
	rig.pppoeFD, err = pppoe.PPPoECreate(pppoe.InterfaceBinding{Device: "zrl0"}, realmRigSessionID, subscriberMAC)
	if err != nil {
		t.Fatalf("PPPoE session socket: %v; the kernel needs pppoe (CONFIG_PPPOE)", err)
	}
	t.Cleanup(func() { pppoe.ClosePPPoxFD(rig.pppoeFD) })
	driverChanFD, driverUnitFD, _, err := ppp.DevPPPSetup(rig.pppoeFD, "")
	if err != nil {
		t.Fatalf("PPPoE /dev/ppp channel: %v", err)
	}
	rig.driverChanFD = driverChanFD
	rig.driverChanSet = true
	t.Cleanup(func() {
		rig.closeDriverChannel(t)
		closeFD(t, driverUnitFD)
	})

	// The L2TP side: the LAC's listener, the LNS's socket, and the kernel
	// tunnel and session between them.
	lacFD, _ := realmRigUDP(t, true)
	lnsFD, lnsAddr := realmRigUDP(t, false)
	rig.lnsFD = lnsFD
	if err := unix.SetsockoptTimeval(lnsFD, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 5}); err != nil {
		t.Fatal(err)
	}
	l2tpFD := l2tp.OpenKernelL2TPSession(t, lacFD, lnsAddr,
		realmRigLocalTID, realmRigLocalSID, realmRigRemoteTID, realmRigRemoteSID)
	l2tpChanFD, l2tpUnitFD, _, err := ppp.DevPPPSetup(l2tpFD, "")
	if err != nil {
		t.Fatalf("pppol2tp /dev/ppp channel: %v", err)
	}
	rig.l2tpChanFD = l2tpChanFD
	t.Cleanup(func() {
		closeFD(t, l2tpUnitFD)
		closeFD(t, l2tpChanFD)
	})
	return rig
}

// realmRigUDP opens a UDP socket on 127.0.0.1 with a kernel-chosen port.
// The LAC's listener carries SO_REUSEPORT, as Ze's does, because
// tunnelCreate binds the tunnel's connected socket to the same port.
func realmRigUDP(t *testing.T, reusePort bool) (int, netip.AddrPort) {
	t.Helper()
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeFD(t, fd) })
	if reusePort {
		if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := unix.Bind(fd, &unix.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}); err != nil {
		t.Fatal(err)
	}
	sa, err := unix.Getsockname(fd)
	if err != nil {
		t.Fatal(err)
	}
	inet, ok := sa.(*unix.SockaddrInet4)
	if !ok {
		t.Fatalf("UDP socket name is %T, want IPv4", sa)
	}
	return fd, netip.AddrPortFrom(netip.AddrFrom4(inet.Addr), uint16(inet.Port)) //nolint:gosec // a port fits 16 bits
}

func closeFD(t *testing.T, fd int) {
	t.Helper()
	if err := unix.Close(fd); err != nil {
		t.Errorf("close fd %d: %v", fd, err)
	}
}

// closeDriverChannel closes the driver's channel descriptor the way
// ReleaseSession does after a successful bridge. It is idempotent.
func (rig *realmBridgeRig) closeDriverChannel(t *testing.T) {
	t.Helper()
	if !rig.driverChanSet {
		return
	}
	rig.driverChanSet = false
	closeFD(t, rig.driverChanFD)
}

// sendMarked sends one LCP Configure-Request from the subscriber whose data
// carries marker, so the far end can tell this frame from any other.
func (rig *realmBridgeRig) sendMarked(t *testing.T, id uint8, marker []byte) {
	t.Helper()
	if err := rig.subscriber.SendControl(rig.acMAC[:], ppp.ProtoLCP, ppp.LCPConfigureRequest, id, marker); err != nil {
		t.Fatalf("subscriber send: %v", err)
	}
}

// waitDriverReadable waits until the driver's channel holds a frame, without
// reading it: the state a quiesced driver leaves a frame in.
func (rig *realmBridgeRig) waitDriverReadable(t *testing.T) {
	t.Helper()
	fds := []unix.PollFd{{Fd: int32(rig.driverChanFD), Events: unix.POLLIN}} //nolint:gosec // descriptors fit int32
	n, err := unix.Poll(fds, 5000)
	if err != nil {
		t.Fatalf("poll the driver channel: %v", err)
	}
	if n != 1 || fds[0].Revents&unix.POLLIN == 0 {
		t.Fatal("the subscriber's frame never reached the driver's channel")
	}
}

// driverPending reads the driver's channel without blocking. It returns the
// frame queued there, or nil when the channel is empty.
func (rig *realmBridgeRig) driverPending(t *testing.T) []byte {
	t.Helper()
	buf := make([]byte, 2048)
	n, err := unix.Read(rig.driverChanFD, buf)
	if errors.Is(err, unix.EAGAIN) {
		return nil
	}
	if err != nil {
		t.Fatalf("read the driver channel: %v", err)
	}
	return buf[:n]
}

// lnsReceive returns the next L2TP data message the LNS's socket receives,
// failing the test after the socket's 5 s receive timeout.
func (rig *realmBridgeRig) lnsReceive(t *testing.T) []byte {
	t.Helper()
	buf := make([]byte, 2048)
	n, _, err := unix.Recvfrom(rig.lnsFD, buf, 0)
	if err != nil {
		t.Fatalf("the LNS received nothing: %v", err)
	}
	// RFC 2661 Section 3.1: "The Type (T) bit indicates the type of message.
	// It is set to 0 for a data message and 1 for a control message."
	if n < 6 || buf[0]&0x80 != 0 {
		t.Fatalf("the LNS received %x, want an L2TP data message", buf[:n])
	}
	return buf[:n]
}

// lnsEmpty reports whether the LNS's socket holds no further message.
func (rig *realmBridgeRig) lnsEmpty(t *testing.T) bool {
	t.Helper()
	buf := make([]byte, 2048)
	_, _, err := unix.Recvfrom(rig.lnsFD, buf, unix.MSG_DONTWAIT)
	if errors.Is(err, unix.EAGAIN) {
		return true
	}
	if err != nil {
		t.Fatalf("read the LNS socket: %v", err)
	}
	return false
}

// TestLACRealmBridgeAfterLocalLCP checks assumption A-1 of spec-home-cpe-14:
// once PPPIOCBRIDGECHAN joins the subscriber's PPPoE channel to the pppol2tp
// channel, the subscriber's frames go to the LNS and no longer reach the
// /dev/ppp channel descriptor the LAC's PPP driver read during local LCP.
// Method: a frame before the bridge MUST arrive on the driver's channel (the
// local LCP stage); after bridgeChannels, a second frame MUST arrive at the
// LNS as an L2TP data message, and the driver's channel MUST then be empty.
// The emptiness check runs after the LNS saw the frame, so no timing decides
// it: the one frame cannot be in both places.
func TestLACRealmBridgeAfterLocalLCP(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	rig := newRealmBridgeRig(t)
	if err := unix.SetNonblock(rig.driverChanFD, true); err != nil {
		t.Fatal(err)
	}

	before := []byte{0xfe, 0x06, 'L', 'C', 'P', '1'}
	rig.sendMarked(t, 1, before)
	rig.waitDriverReadable(t)
	if got := rig.driverPending(t); !bytes.Contains(got, before) {
		t.Fatalf("before the bridge the driver read %x, want the subscriber's frame carrying %x", got, before)
	}

	if err := l2tp.BridgeChannels(rig.l2tpChanFD, rig.pppoeFD); err != nil {
		t.Fatalf("bridge: %v", err)
	}

	after := []byte{0xfe, 0x06, 'B', 'R', 'G', '1'}
	rig.sendMarked(t, 2, after)
	if got := rig.lnsReceive(t); !bytes.Contains(got, after) {
		t.Fatalf("after the bridge the LNS received %x, want the subscriber's frame carrying %x", got, after)
	}
	if got := rig.driverPending(t); got != nil {
		t.Fatalf("after the bridge the driver's channel still received %x (A-1 broken)", got)
	}
}

// TestLACRealmHandoffRetransmission checks AC-14 of spec-home-cpe-14 on the
// real kernel path: a frame the subscriber sends while the driver is
// quiesced (it no longer reads its channel, and the bridge is not yet set)
// is not delivered to the LNS; the subscriber's retransmission after the
// bridge is, and the bridge keeps carrying frames once the driver released
// the session and closed its channel.
// Method: send the "lost" frame and wait until the kernel queued it on the
// driver's channel without reading it; bridge; send the retransmission and
// require it to be the first message the LNS receives, with nothing else
// behind it; close the driver's channel as ReleaseSession does, and require
// one more subscriber frame to reach the LNS.
func TestLACRealmHandoffRetransmission(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	rig := newRealmBridgeRig(t)

	lost := []byte{0xfe, 0x06, 'Q', 'S', 'C', '1'}
	rig.sendMarked(t, 7, lost)
	rig.waitDriverReadable(t)

	if err := l2tp.BridgeChannels(rig.l2tpChanFD, rig.pppoeFD); err != nil {
		t.Fatalf("bridge: %v", err)
	}

	retransmit := []byte{0xfe, 0x06, 'R', 'T', 'X', '1'}
	rig.sendMarked(t, 7, retransmit)
	got := rig.lnsReceive(t)
	if bytes.Contains(got, lost) {
		t.Fatalf("the frame sent while quiesced reached the LNS: %x", got)
	}
	if !bytes.Contains(got, retransmit) {
		t.Fatalf("the LNS received %x, want the retransmission carrying %x", got, retransmit)
	}
	if !rig.lnsEmpty(t) {
		t.Fatal("the LNS received a second message: the frame sent while quiesced was delivered late")
	}

	// ReleaseSession closes the driver's channel; the queued frame goes with
	// it, and the bridge, which the pppox sockets hold, stays.
	rig.closeDriverChannel(t)
	upstream := []byte{0xfe, 0x06, 'U', 'P', 'S', '1'}
	rig.sendMarked(t, 8, upstream)
	if got := rig.lnsReceive(t); !bytes.Contains(got, upstream) {
		t.Fatalf("after the driver released, the LNS received %x, want %x", got, upstream)
	}
}
