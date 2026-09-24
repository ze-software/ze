//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- unit ownership through Cleanup.

package pppoeclient

import (
	"bytes"
	"context"
	"encoding/binary"
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
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
)

// TestPADTRetainsUnitUntilOwnerCleanup completes real Dial against a wire peer,
// then receives PADT before the owner configures the returned pppN. A competing
// subscriber cannot acquire that unit until Cleanup; late MTU setup therefore
// cannot modify another subscriber's interface.
func TestPADTRetainsUnitUntilOwnerCleanup(t *testing.T) {
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
		t.Skipf("network namespace requires CAP_SYS_ADMIN: %v", err)
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
		t.Skipf("requires /dev/ppp: %v", err)
	}
	_ = unix.Close(probe)
	if err := netlink.LinkAdd(&netlink.Veth{Name: "zec0", PeerName: "zea0"}); err != nil {
		if isPermErr(err) {
			t.Skipf("veth requires CAP_NET_ADMIN: %v", err)
		}
		t.Fatal(err)
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
		if isPermErr(err) {
			t.Skipf("wire peer requires CAP_NET_RAW: %v", err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Close(fd) })
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Ifindex: peerLink.Attrs().Index, Protocol: unitPeerNetworkOrder(unix.ETH_P_ALL)}); err != nil {
		t.Fatal(err)
	}
	if err := pppoe.SetRecvTimeout(fd, 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	peer := unitReservationPeer{fd: fd, ifindex: peerLink.Attrs().Index, sid: 42}
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
	var clientMAC [pppoe.EthALen]byte
	copy(clientMAC[:], clientLink.Attrs().HardwareAddr)
	var buf [pppoe.EthMaxLen]byte
	padt := pppoe.NewBuilder(buf[:], peer.mac, clientMAC, pppoe.CodePADT, peer.sid)
	if err := peer.send(padt.Finish()); err != nil {
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

// unitReservationPeer is a wire peer, not a substitute for the client's kernel
// setup. It negotiates no-auth IPv4 and leaves PADT timing to the consumer test.
type unitReservationPeer struct {
	fd, ifindex int
	mac         [pppoe.EthALen]byte
	sid         uint16
}

func unitPeerNetworkOrder(v uint16) uint16 { return v<<8 | v>>8 }

func (p *unitReservationPeer) send(frame []byte) error {
	var destination [8]byte
	copy(destination[:], frame[:pppoe.EthALen])
	return unix.Sendto(p.fd, frame, 0, &unix.SockaddrLinklayer{
		Ifindex: p.ifindex, Halen: pppoe.EthALen, Addr: destination,
		Protocol: unitPeerNetworkOrder(binary.BigEndian.Uint16(frame[12:14])),
	})
}

func (p *unitReservationPeer) sendPPP(remote []byte, proto uint16, code, id uint8, data []byte) error {
	var buf [pppoe.EthMaxLen]byte
	copy(buf[:6], remote)
	copy(buf[6:12], p.mac[:])
	binary.BigEndian.PutUint16(buf[12:14], unix.ETH_P_PPP_SES)
	buf[14] = 0x11
	binary.BigEndian.PutUint16(buf[16:18], p.sid)
	off := 20 + ppp.WriteFrame(buf[:], 20, proto, nil)
	off += ppp.WriteLCPPacket(buf[:], off, code, id, data)
	binary.BigEndian.PutUint16(buf[18:20], uint16(off-20))
	return p.send(buf[:off])
}

func (p *unitReservationPeer) serve(ctx context.Context) error {
	var incoming, outgoing [pppoe.EthMaxLen]byte
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		n, from, err := unix.Recvfrom(p.fd, incoming[:], 0)
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		link, ok := from.(*unix.SockaddrLinklayer)
		if !ok || link.Ifindex != p.ifindex || link.Pkttype == unix.PACKET_OUTGOING || n < 20 {
			continue
		}
		switch binary.BigEndian.Uint16(incoming[12:14]) {
		case unix.ETH_P_PPP_DISC:
			pkt, err := pppoe.ParseDiscovery(incoming[:n])
			if err != nil {
				return err
			}
			var reply []byte
			switch pkt.Code {
			case pppoe.CodePADI:
				reply = pppoe.BuildPADO(outgoing[:], p.mac, &pkt, "unit-owner-peer", nil, nil)
			case pppoe.CodePADR:
				reply = pppoe.BuildPADS(outgoing[:], p.mac, &pkt, "unit-owner-peer", p.sid)
			default:
				continue
			}
			if err := p.send(reply); err != nil {
				return err
			}
		case unix.ETH_P_PPP_SES:
			if !bytes.Equal(incoming[:6], p.mac[:]) || binary.BigEndian.Uint16(incoming[16:18]) != p.sid {
				continue
			}
			end := 20 + int(binary.BigEndian.Uint16(incoming[18:20]))
			if end > n {
				return errors.New("truncated PPPoE session frame")
			}
			proto, payload, _, err := ppp.ParseFrame(incoming[20:end])
			if err != nil {
				return err
			}
			pkt, err := ppp.ParseLCPPacket(payload)
			if err != nil {
				return err
			}
			if pkt.Code != ppp.LCPConfigureRequest {
				continue
			}
			switch proto {
			case ppp.ProtoLCP:
				if err := p.sendPPP(incoming[6:12], proto, ppp.LCPConfigureAck, pkt.Identifier, pkt.Data); err != nil {
					return err
				}
				if err := p.sendPPP(incoming[6:12], proto, ppp.LCPConfigureRequest, 77, nil); err != nil {
					return err
				}
			case ppp.ProtoIPCP:
				address := []byte{3, 6, 192, 0, 2, 2}
				if !bytes.Equal(pkt.Data, address) {
					if err := p.sendPPP(incoming[6:12], proto, ppp.LCPConfigureNak, pkt.Identifier, address); err != nil {
						return err
					}
					continue
				}
				if err := p.sendPPP(incoming[6:12], proto, ppp.LCPConfigureAck, pkt.Identifier, pkt.Data); err != nil {
					return err
				}
				if err := p.sendPPP(incoming[6:12], proto, ppp.LCPConfigureRequest, 78, []byte{3, 6, 192, 0, 2, 1}); err != nil {
					return err
				}
			}
		}
	}
}
