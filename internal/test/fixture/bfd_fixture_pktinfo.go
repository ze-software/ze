// Design: docs/architecture/bfd.md -- "One session per neighbor" and the first-packet index
// RFC: rfc/short/rfc5880.md
// Related: bfd_fixture.go -- the observer shape and the session-detail poll this reuses
//
// bfd_fixture_pktinfo.go drives the one BFD path a unit test cannot reach: the
// kernel's own IP_PKTINFO control message. The engine selects a session for a
// packet whose Your Discriminator is zero by (peer, local, interface, vrf,
// mode), and the socket binds the wildcard, so the local address and the
// ingress interface are knowable only from that control message. A synthesized
// cmsg buffer proves the parser matches this file's MODEL of the kernel; only a
// real packet through a real kernel proves the model.

package fixture

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The session test/bfd/bfd-first-packet-pktinfo.ci configures, and the
// discriminator the injected packet carries.
//
// Both addresses are on loopback, which every Linux kernel terminates, so the
// packet is a real one through the real receive path without needing an
// interface the test would have to create. The daemon's session is keyed on
// local 127.0.0.1; the packet is sent to that address from 127.0.0.2, so
// ipi_addr reports 127.0.0.1 and the session is selected. Before readLoop read
// the control message it stamped the wildcard 0.0.0.0 on every Inbound, and
// this session could never be found.
const (
	bfdPktinfoPeer       = addrLoopbackSecond
	bfdPktinfoLocal      = addrLoopback
	bfdPktinfoPeerDiscr  = 0x51501234
	bfdPktinfoSingleHop  = 3784
	bfdPktinfoSendWindow = 12 * time.Second

	// The IPv6 arm. IPV6_PKTINFO carries a 16-byte address and its ifindex
	// AFTER it, the opposite order to the IPv4 struct, and the parser reads
	// both from offsets this test is the only thing to have exercised on a
	// real kernel.
	//
	// Linux terminates exactly one address in ::/128, so unlike 127/8 there is
	// no second loopback address to send from. The .ci adds this pair to lo
	// before the daemon starts, which keeps the shape of the IPv4 arm: the
	// packet is sent FROM one address TO another, and the session is keyed on
	// the one it is sent to.
	bfdPktinfoPeerV6      = "fd00:5882::2"
	bfdPktinfoLocalV6     = "fd00:5882::1"
	bfdPktinfoPeerDiscrV6 = 0x6150abcd
)

// bfdFirstPacketPktinfoScenario sends BFD Control packets carrying Your
// Discriminator = 0 and waits for the daemon to report the peer's
// discriminator, which it can only learn from a packet it selected a session
// for.
//
// RFC 5880 Section 6.8.6: "If the Your Discriminator field is zero, the session
// MUST be selected based on some combination of other fields." The fields are
// this session's own local address and ingress interface, and the assertion is
// that a real kernel delivers both.
func bfdFirstPacketPktinfoScenario(ctx context.Context, plugin *sdk.Plugin) error {
	return bfdFirstPacketFor(ctx, plugin, "udp4", bfdPktinfoPeer, bfdPktinfoLocal, bfdPktinfoPeerDiscr)
}

// bfdFirstPacketPktinfoV6Scenario is the same proof over IPv6, where the
// control message is IPV6_PKTINFO and its layout is not the IPv4 one.
func bfdFirstPacketPktinfoV6Scenario(ctx context.Context, plugin *sdk.Plugin) error {
	return bfdFirstPacketFor(ctx, plugin, "udp6", bfdPktinfoPeerV6, bfdPktinfoLocalV6, bfdPktinfoPeerDiscrV6)
}

func bfdFirstPacketFor(ctx context.Context, plugin *sdk.Plugin, network, peer, local string, discr uint32) error {
	stop := make(chan struct{})
	defer close(stop)
	sendErr := make(chan error, 1)
	go bfdInjectFirstPackets(network, peer, local, discr, stop, sendErr) //nolint:goroutine-lifecycle // per-fixture lifecycle, stopped by the defer above

	deadline := time.Now().Add(bfdPktinfoSendWindow)
	for time.Now().Before(deadline) {
		select {
		case err := <-sendErr:
			return err
		default:
		}
		session, err := bfdSessionDetail(ctx, plugin, peer)
		if err != nil {
			return err
		}
		seen, ok := session["remote-discriminator"].(float64)
		if ok && uint32(seen) == discr {
			fmt.Fprintf(os.Stderr, "OK: the first packet was selected by its destination address %s, remote-discriminator %#x\n", local, uint32(seen))
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("no session was selected for a Control packet with Your Discriminator 0 sent to %s: remote-discriminator stayed zero, so the kernel's PKTINFO did not reach firstPacketKey", local)
}

// bfdInjectFirstPackets sends one Control packet every 200ms until stop closes.
// It repeats because the daemon's session may not exist yet when the first one
// lands, and a single packet would make the test a race.
func bfdInjectFirstPackets(network, peer, local string, discr uint32, stop <-chan struct{}, fail chan<- error) {
	// RFC 5881 Section 4: the source port is in the 49152-65535 range, which is
	// what an ephemeral bind gives. Binding the peer address rather than the
	// wildcard is what makes ipi_addr on the receive side 127.0.0.1.
	source, err := net.ResolveUDPAddr(network, net.JoinHostPort(peer, "0"))
	if err != nil {
		fail <- fmt.Errorf("resolve %s: %w", peer, err)
		return
	}
	conn, err := net.ListenUDP(network, source)
	if err != nil {
		fail <- fmt.Errorf("listen on %s: %w", peer, err)
		return
	}
	defer func() { _ = conn.Close() }()
	if err := bfdSetOutboundTTL255(conn, network); err != nil {
		fail <- err
		return
	}

	control := packet.Control{
		Version:               packet.Version,
		State:                 packet.StateDown,
		DetectMult:            3,
		Length:                packet.MandatoryLen,
		MyDiscriminator:       discr,
		YourDiscriminator:     0,
		DesiredMinTxInterval:  300_000,
		RequiredMinRxInterval: 300_000,
	}
	buf := make([]byte, packet.MandatoryLen)
	control.WriteTo(buf, 0)
	target := net.UDPAddrFromAddrPort(netip.AddrPortFrom(netip.MustParseAddr(local), bfdPktinfoSingleHop))
	for {
		select {
		case <-stop:
			return
		default:
		}
		if _, err := conn.WriteToUDP(buf, target); err != nil {
			fail <- fmt.Errorf("send to %s: %w", target, err)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// bfdSetOutboundTTL255 puts the RFC 5881 Section 5 TTL on the injected packets.
// Without it the engine's own GTSM gate drops them and the test would fail for
// a reason that has nothing to do with the session selection it is about.
func bfdSetOutboundTTL255(conn *net.UDPConn, network string) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("syscall conn: %w", err)
	}
	level, option := unix.IPPROTO_IP, unix.IP_TTL
	if network == "udp6" {
		level, option = unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS
	}
	var innerErr error
	if ctrlErr := raw.Control(func(fd uintptr) {
		innerErr = unix.SetsockoptInt(int(fd), level, option, 255)
	}); ctrlErr != nil {
		return fmt.Errorf("rawconn control: %w", ctrlErr)
	}
	if innerErr != nil {
		return fmt.Errorf("setsockopt hop limit 255 on %s: %w", network, innerErr)
	}
	return nil
}
