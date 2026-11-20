//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the IKE bypass on Ze's own IKE sockets
// Related: rfc4301_fragment_forward_integration_linux_test.go -- the protected host, its raw sender and the outside capture
// Related: rfc4301_fragment_boundary_integration_linux_test.go -- fragmentChildren, the production Child SA creator
// Related: ../transport/bypass_linux.go -- bypassIKESocket, the producer

package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"golang.org/x/net/ipv4"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// ikeTransitSourcePorts are the protected host's source ports, the two IKE ports and
// an ordinary one, written as IANA numbers so a mutated transport constant moves one
// side only. The ordinary port is the positive control.
var ikeTransitSourcePorts = [...]uint16{500, 4500, sadSelSourcePort}

// ikeTransitWire is what the outside peer's ingress saw of the test datagrams.
type ikeTransitWire struct {
	protected int            // ESP carrying the Child SA's outbound SPI
	clear     map[uint16]int // cleartext UDP from the protected host, by source port
}

// ikeTransitPacket is an ordinary transit datagram from the protected host to the
// outside peer's protected address, with its UDP source port set. The UDP checksum
// stays zero (none, for IPv4) and the IPv4 header checksum covers no port.
func ikeTransitPacket(t *testing.T, sourcePort, id uint16) []byte {
	t.Helper()
	packet := fragmentForwardPacket(t, sadSelPort, id)
	binary.BigEndian.PutUint16(packet[20:22], sourcePort)
	return packet
}

// ikeTransitObserve reads the outside peer's ingress until want ESP datagrams with
// spi arrived, or the capture budget ends. It never stops early on the cleartext
// side: a leaked datagram can arrive before or after the ESP ones, so the whole
// budget is read whenever the ESP count falls short. At most 64 frames are read.
func ikeTransitObserve(t *testing.T, fd int, spi uint32, want int) ikeTransitWire {
	t.Helper()
	seen := ikeTransitWire{clear: map[uint16]int{}}
	deadline := time.Now().Add(fragmentCaptureWait)
	buf := make([]byte, 65535)
	for range 64 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return seen
		}
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		ready, err := unix.Poll(poll, max(1, int(remaining/time.Millisecond)))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			t.Fatalf("wait on the outside peer's capture: %v", err)
		}
		if ready == 0 {
			return seen
		}
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			t.Fatalf("read the outside peer's ingress: %v", err)
		}
		h, err := ipv4.ParseHeader(buf[:n])
		if err != nil {
			t.Fatalf("parse an ingress IPv4 header: %v", err)
		}
		if h.Protocol == protoESP && n >= h.Len+4 && binary.BigEndian.Uint32(buf[h.Len:h.Len+4]) == spi {
			seen.protected++
		}
		if h.Protocol == sadSelProtoUDP && h.Src.Equal(net.ParseIP(fragmentHostAddress)) && n >= h.Len+2 {
			seen.clear[binary.BigEndian.Uint16(buf[h.Len:h.Len+2])]++
		}
		if seen.protected >= want {
			return seen
		}
	}
	return seen
}

// ikeOwnExchangeOctets is the size of each datagram of ikeOwnExchange: the 28-octet IKE
// header, the floor below which UDPTransport.Run drops a datagram before Recv sees it.
// The transport parses nothing else, so the content needs no IKE header form.
const ikeOwnExchangeOctets = 28

// ikeOwnExchangeDatagram is a datagram of ikeOwnExchangeOctets starting with label.
func ikeOwnExchangeDatagram(label string) []byte {
	datagram := make([]byte, ikeOwnExchangeOctets)
	copy(datagram, label)
	return datagram
}

// ikeOwnExchange sends one datagram each way between Ze's IKE socket on UDP 500 and
// the outside peer's, and fails unless both arrive. Ze's socket is opened through
// transport.NewUDPTransport, the constructor every production IKE socket comes from.
// The caller MUST be in Ze's namespace; it is left there.
func ikeOwnExchange(t *testing.T, lab *espPolLab, unmatched string) {
	t.Helper()
	ze := mbTransport(t, net.JoinHostPort(espPolZeOuter, strconv.Itoa(transport.IKEPort)), false)
	lab.enter(t, lab.peer)
	peer := sadSelListen(t, espPolPeerOuter, transport.IKEPort)
	lab.enter(t, lab.ze)

	peerIKE := &net.UDPAddr{IP: net.ParseIP(espPolPeerOuter), Port: transport.IKEPort}
	if err := ze.Send(ikeOwnExchangeDatagram("ze-ike-out"), peerIKE); err != nil {
		t.Fatalf("%s: Ze's own IKE send was refused (%v); the SPD captured it", unmatched, err)
	}
	if !sadSelDelivered(t, peer) {
		t.Fatalf("%s: Ze's own IKE datagram never reached the peer", unmatched)
	}
	zeIKE := &net.UDPAddr{IP: net.ParseIP(espPolZeOuter), Port: transport.IKEPort}
	reply := ikeOwnExchangeDatagram("ze-ike-in")
	if _, err := peer.WriteToUDP(reply, zeIKE); err != nil {
		t.Fatalf("%s: the peer's IKE reply could not be sent: %v", unmatched, err)
	}
	select {
	case got := <-ze.Recv():
		if !bytes.Equal(got.Data, reply) {
			t.Fatalf("%s: Ze's IKE socket read %q, want the peer's reply", unmatched, got.Data)
		}
	case <-time.After(fragmentCaptureWait):
		t.Fatalf("%s: the peer's %d-octet datagram to Ze's IKE socket on UDP %d was not read from the socket within %v",
			unmatched, len(reply), transport.IKEPort, fragmentCaptureWait)
	}
}

// TestRFC4301TransitFromIKEPortIsNotBypassed proves the IKE exemption is Ze's own and
// nobody else's (AC-17).
//
// A protected host behind Ze sends UDP from source port 500, 4500 and an ordinary port
// to an address the Child SA's selectors cover (10.2.0.0/16 to 10.1.0.0/16, any UDP
// port), under each `unmatched` value. Two observations at the outside link: the ESP
// datagrams carrying the Child SA's outbound SPI, the peer's inbound SA traffic, and
// any cleartext UDP from the host. All three must arrive as ESP, none in clear. The
// ordinary port is the positive control: if it leaves no ESP, the tunnel is not
// carrying transit at all and the IKE rows judge nothing. Then Ze's own IKE socket
// exchanges a datagram each way with the peer, which under `unmatched discard` only a
// bypass can carry.
//
// RFC 4301 Section 5.2: "(This implies that IKE traffic MUST have an explicit BYPASS
// entry in the SPD.)"
//
// RFC requirement: RFC4301-5.2-4 negative -- a forwarded datagram from UDP source port 500 or 4500 is not IKE traffic of this node and leaves inside the tunnel, never in clear, while this node's own IKE socket still exchanges in both directions.
//
//ze:guest-root
func TestRFC4301TransitFromIKEPortIsNotBypassed(t *testing.T) {
	for _, tc := range []struct {
		name      string
		unmatched dataplane.SPAction
	}{
		{"unmatched bypass", dataplane.SPActionBypass},
		{"unmatched discard", dataplane.SPActionDiscard},
	} {
		unmatched := tc.unmatched
		t.Run(tc.name, func(t *testing.T) {
			lab := newEspPolLab(t)
			sender, capture := fragmentForwardPeers(t, lab)
			espPolSPD(t)
			child := fragmentChildren(t, fragmentPorts{local: ipsec.AnyPort(), remote: ipsec.AnyPort()})[0]
			// RFC 4301 Sections 4.4.1 and 5.1: the configured final disposition.
			if err := installUnmatched(dataplane.Get(), unmatched, slogutil.DiscardLogger()); err != nil {
				t.Fatalf("select %s: %v", tc.name, err)
			}

			for index, port := range ikeTransitSourcePorts {
				fragmentSend(t, sender, ikeTransitPacket(t, port, uint16(0x7a00+index)))
			}
			seen := ikeTransitObserve(t, capture, child.OutboundSPI, len(ikeTransitSourcePorts))
			if seen.clear[sadSelSourcePort] != 0 || seen.protected == 0 {
				t.Fatalf("positive control: ordinary transit from port %d left %d in clear and %d ESP datagrams in all; "+
					"the tunnel does not carry transit from the protected host, so the IKE rows judge nothing",
					sadSelSourcePort, seen.clear[sadSelSourcePort], seen.protected)
			}
			for _, port := range ikeTransitSourcePorts[:2] {
				if n := seen.clear[port]; n != 0 {
					t.Errorf("transit UDP from source port %d left in clear %d time(s): the IKE bypass matched traffic that is not this node's IKE", port, n)
				}
			}
			if seen.protected != len(ikeTransitSourcePorts) {
				t.Errorf("%d ESP datagrams with SPI %#x reached the peer, want %d (one per source port)",
					seen.protected, child.OutboundSPI, len(ikeTransitSourcePorts))
			}

			ikeOwnExchange(t, lab, tc.name)
		})
	}
}

// TestRFC4301IKESocketsCarryTheirOwnBypass reads back, from the kernel, the bypass an
// IKE socket carries, with the application override guard attached under `deny`.
//
// The guard refuses the per-socket policy option to every process but Ze's own, so
// the socket policy appearing at all proves the guard let Ze's IKE socket through.
// The listing must name both directions ike-bypass, BYPASS, per-socket, and doctor's
// application override check must report nothing about them.
//
// RFC 4301 Section 5.2: "(This implies that IKE traffic MUST have an explicit BYPASS
// entry in the SPD.)"
//
// RFC requirement: RFC4301-5.2-4 positive -- every IKE socket Ze opens carries an inbound and an outbound BYPASS policy of its own, which the kernel holds and the application override guard admits as Ze's.
//
//ze:guest-root
func TestRFC4301IKESocketsCarryTheirOwnBypass(t *testing.T) {
	newEspPolLab(t)
	espPolSPD(t)
	dp := dataplane.Get()
	if err := applyApplicationOverride(dp, ipsec.ApplicationOverrideDeny); err != nil {
		t.Fatalf("attach the application override guard under deny: %v", err)
	}
	t.Cleanup(func() { releaseApplicationOverride(dp, slogutil.DiscardLogger()) })

	mbTransport(t, net.JoinHostPort(espPolZeOuter, strconv.Itoa(transport.IKEPort)), false)
	mbTransport(t, net.JoinHostPort(espPolZeOuter, strconv.Itoa(transport.NATTPort)), true)

	policies, err := dp.ListPolicies()
	if err != nil {
		t.Fatalf("list the kernel's policies: %v", err)
	}
	counts := map[dataplane.SADir]int{}
	for i := range policies {
		p := &policies[i]
		if !p.PerSocket || p.Role != dataplane.PolicyRoleIKEBypass {
			continue
		}
		if p.Action != dataplane.SPActionBypass {
			t.Errorf("an IKE socket policy (dir %v) has action %v, want bypass", p.Dir, p.Action)
		}
		counts[p.Dir]++
	}
	if counts[dataplane.SADirIn] != 2 || counts[dataplane.SADirOut] != 2 {
		t.Fatalf("IKE socket policies by direction = %v, want 2 in and 2 out (two IKE sockets) among %d policies", counts, len(policies))
	}
	if diags := checkIPsecApplicationOverride(registry.DoctorCheckContext{}); len(diags) != 0 {
		t.Errorf("doctor reports Ze's own IKE socket bypass: %v", diags)
	}
}
