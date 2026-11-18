//go:build integration && linux

// Design: docs/architecture/l2tp/followup-l2tp-call.md -- realm relay LAC bridge
// Related: bridge_integration_linux_test.go -- TestLACChannelBridge, original native consumer
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
	"os"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/fixture"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/internal/test/userns"
)

// realmBridgeRig is the kernel path a realm-relayed subscriber crosses, built
// the way the LAC builds it: the PPPoE server's session socket and /dev/ppp
// channel (pppoe.PPPoECreate, pppkernel.DevPPPSetup), and the kernel L2TP
// tunnel, session and pppol2tp channel (l2tp.OpenKernelL2TPSession,
// pppkernel.DevPPPSetup, as setupSession and pppSetupReal do). Both local
// channel/unit pairs transfer to pppkernel.NewSession; before bridging both
// handles close and fresh channel owners are captured from the SAME retained
// PPPoX files. The exact bridge then owns both new channels. The far ends are a
// raw subscriber on the veth pair and a plain UDP socket standing for the
// LNS, so a test sees exactly which frames the kernel delivers where.
type realmBridgeRig struct {
	subscriber    *pppoeac.Peer
	acMAC         [pppoe.EthALen]byte
	pppoeFile     *os.File                         // the original subscriber PPPoE session socket
	driverChannel *pppkernel.CreatedChannel        // original channel for short probes
	driverOwner   ppp.SessionHandle                // owns the driver's channel and unit
	l2tpFile      *os.File                         // SAME original native transport, retained through bridge cleanup
	l2tpOwner     ppp.SessionHandle                // original local owner, closed before fresh capture
	bridgeOwner   *pppkernel.OriginalChannelBridge // consumes fresh peers, never replayed session claims
	lnsFD         int                              // the LNS's end of the tunnel
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

	backend, err := iface.OpenBackend("netlink")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Errorf("close the kernel interface backend: %v", err)
		}
	})

	creator, err := pppkernel.NewUnitCreator(backend)
	if err != nil {
		t.Fatalf("capture original PPP unit creator: %v", err)
	}

	// The PPPoE side, as the kernel session factory builds it for a session
	// that runs PPP locally first.
	rig.pppoeFile, err = pppoe.PPPoECreate(pppoe.InterfaceBinding{Device: "zrl0"}, realmRigSessionID, subscriberMAC)
	if err != nil {
		t.Fatalf("PPPoE session socket: %v; the kernel needs pppoe (CONFIG_PPPOE)", err)
	}
	t.Cleanup(func() {
		if err := rig.pppoeFile.Close(); err != nil {
			t.Error(err)
		}
	})
	rig.driverChannel, rig.driverOwner = realmRigPPPSession(t, rig.pppoeFile, creator)

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
	rig.l2tpFile = l2tpFD
	_, rig.l2tpOwner = realmRigPPPSession(t, l2tpFD, creator)
	t.Cleanup(func() {
		if rig.bridgeOwner != nil {
			if err := rig.bridgeOwner.Close(); err != nil {
				t.Errorf("close exact realm bridge: %v", err)
			}
		}
	})
	return rig
}

// realmRigPPPSession transfers the genuine pair to the public kernel consumer.
// The returned channel grants only short native probes. NewSession owns Close;
// its t.Cleanup runs before the independently captured backend is closed.
func realmRigPPPSession(t *testing.T, original *os.File, creator *pppkernel.UnitCreator) (*pppkernel.CreatedChannel, ppp.SessionHandle) {
	t.Helper()
	channel, unit, err := pppkernel.DevPPPSetup(creator, original, "")
	if err != nil {
		// Every nonnil partial owner remains accountable even after setup's
		// rollback. The typed Close methods retain their original results.
		var cleanup error
		if unit != nil {
			cleanup = unit.Close()
		}
		if channel != nil {
			cleanup = errors.Join(cleanup, channel.Close())
		}
		t.Fatalf("kernel PPP channel/unit setup: %v", errors.Join(err, cleanup))
	}
	cfg, err := unit.SessionConfig(channel)
	if err != nil {
		t.Fatalf("kernel PPP original configuration: %v", errors.Join(err, unit.Close(), channel.Close()))
	}
	_, owner, err := pppkernel.NewSession(cfg)
	// The constructor consumes both owners on every outcome. It alone owns
	// rollback; no raw descriptor or second wrapper is created here.
	if err != nil {
		t.Fatalf("kernel PPP session owner: %v", err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Errorf("close the kernel PPP session: %v", err)
		}
	})
	return channel, owner
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

// closeDriverChannel releases the driver's channel and unit through their
// owner, as ReleaseSession does after a successful bridge. It is idempotent.
func (rig *realmBridgeRig) closeDriverChannel(t *testing.T) {
	t.Helper()
	if err := rig.driverOwner.Close(); err != nil {
		t.Errorf("release the driver's kernel PPP session: %v", err)
	}
}

// sendMarked sends one LCP Configure-Request from the subscriber whose data
// carries marker, so the far end can tell this frame from any other.
func (rig *realmBridgeRig) sendMarked(t *testing.T, id uint8, marker []byte) {
	t.Helper()
	if err := rig.subscriber.SendControl(rig.acMAC[:], ppp.ProtoLCP, ppp.LCPConfigureRequest, id, marker); err != nil {
		t.Fatalf("subscriber send: %v", err)
	}
}

// waitDriverReadable waits for the original channel to hold a frame without
// consuming it. Each descriptor borrow is a nonblocking native probe. The
// bounded fixture poll waits outside the borrow and never retains a raw label.
func (rig *realmBridgeRig) waitDriverReadable(t *testing.T) {
	t.Helper()
	var probeErr error
	ready := fixture.Poll(t.Context(), 501, 10*time.Millisecond, func() bool {
		readable := false
		probeErr = rig.driverChannel.WithDescriptor(func(fd int) error {
			fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}} //nolint:gosec // descriptors fit int32
			n, err := unix.Poll(fds, 0)
			if err != nil {
				return err
			}
			if n == 1 {
				readable = fds[0].Revents&unix.POLLIN != 0
			}
			return nil
		})
		if probeErr != nil {
			return true
		}
		return readable
	})
	if probeErr != nil {
		t.Fatalf("poll the driver channel: %v", probeErr)
	}
	if !ready {
		t.Fatal("the subscriber's frame never reached the driver's channel")
	}
}

// driverPending reads only the original channel's currently queued frame.
// The borrow covers one nonblocking read and never exports a descriptor.
func (rig *realmBridgeRig) driverPending(t *testing.T) []byte {
	t.Helper()
	buf := make([]byte, 2048)
	n := 0
	err := rig.driverChannel.WithDescriptor(func(fd int) error {
		var err error
		n, err = unix.Read(fd, buf)
		return err
	})
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
// original channel the LAC's PPP driver read during local LCP. That original
// is closed before fresh bridge attachment and cannot receive another frame.
// Method: a frame before the bridge MUST arrive on the driver's channel (the
// local LCP stage); after exact original bridge attachment, a second frame
// MUST arrive at the LNS, and the old driver's channel MUST be closed.
// The emptiness check runs after the LNS saw the frame, so no timing decides
// it: the one frame cannot be in both places.
func TestLACRealmBridgeAfterLocalLCP(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	rig := newRealmBridgeRig(t)
	if err := rig.driverChannel.WithDescriptor(func(fd int) error { return unix.SetNonblock(fd, true) }); err != nil {
		t.Fatal(err)
	}

	before := []byte{0xfe, 0x06, 'L', 'C', 'P', '1'}
	rig.sendMarked(t, 1, before)
	rig.waitDriverReadable(t)
	if got := rig.driverPending(t); !bytes.Contains(got, before) {
		t.Fatalf("before the bridge the driver read %x, want the subscriber's frame carrying %x", got, before)
	}

	if err := rig.bridgeOriginalPeers(); err != nil {
		t.Fatalf("bridge: %v", err)
	}

	after := []byte{0xfe, 0x06, 'B', 'R', 'G', '1'}
	rig.sendMarked(t, 2, after)
	if got := rig.lnsReceive(t); !bytes.Contains(got, after) {
		t.Fatalf("after the bridge the LNS received %x, want the subscriber's frame carrying %x", got, after)
	}
	if err := rig.driverChannel.WithDescriptor(func(int) error { return nil }); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("after local release the original driver channel must be closed, got %v", err)
	}
}

// TestLACRealmHandoffRetransmission checks AC-14 of spec-home-cpe-14 on the
// real kernel path: a frame the subscriber sends while the driver is
// quiesced (it no longer reads its channel, and the bridge is not yet set)
// is not delivered to the LNS; the subscriber's retransmission after the
// bridge is, and the bridge keeps carrying frames after repeated closure of
// the released driver's original owner.
// Method: send the "lost" frame and wait until the kernel queued it on the
// driver's channel without reading it; bridge; send the retransmission and
// require it to be the first message the LNS receives, with nothing else
// behind it; close the same old driver owner again, and require
// one more subscriber frame to reach the LNS.
func TestLACRealmHandoffRetransmission(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	rig := newRealmBridgeRig(t)

	lost := []byte{0xfe, 0x06, 'Q', 'S', 'C', '1'}
	rig.sendMarked(t, 7, lost)
	rig.waitDriverReadable(t)

	if err := rig.bridgeOriginalPeers(); err != nil {
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

	// Preparation already released the old local owner and its queued frame.
	// A repeated release cannot disturb the fresh bridge's independent claims.
	rig.closeDriverChannel(t)
	upstream := []byte{0xfe, 0x06, 'U', 'P', 'S', '1'}
	rig.sendMarked(t, 8, upstream)
	if got := rig.lnsReceive(t); !bytes.Contains(got, upstream) {
		t.Fatalf("after the driver released, the LNS received %x, want %x", got, upstream)
	}
}
