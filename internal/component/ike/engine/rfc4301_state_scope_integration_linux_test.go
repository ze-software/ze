//go:build integration && linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- negotiated per-SA scope.
// Related: fragment_boundary_integration_linux_test.go -- the production Child creator and peer sender.
// RFC naming: untagged -- kernel boundary probes awaiting observed discrimination records.

package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// rfcScopePairs describes two independently permitted services in one negotiated SA.
func rfcScopePairs(t *testing.T, first, second uint16) []tsPair {
	t.Helper()
	return []tsPair{
		{
			I: tsSelector{Net: mustCIDRNet(t, "10.2.0.0/16"), Proto: sadSelProtoUDP,
				Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: first}},
			R: tsSelector{Net: mustCIDRNet(t, "10.1.0.0/16"), Proto: sadSelProtoUDP, Port: ipsec.AnyPort()},
		},
		{
			I: tsSelector{Net: mustCIDRNet(t, "10.2.0.0/16"), Proto: sadSelProtoUDP,
				Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: second}},
			R: tsSelector{Net: mustCIDRNet(t, "10.1.0.0/16"), Proto: sadSelProtoUDP, Port: ipsec.AnyPort()},
		},
	}
}

// rfcScopeChildren MUST run in the Ze namespace, as fragmentChildren does. It
// returns the first IKE SA as well so a replacement can obtain the current NAT-T
// ports through takeEncapPorts, just like the production rekey callers.
func rfcScopeChildren(t *testing.T, sets ...[]tsPair) ([]*ChildSA, *SA) {
	t.Helper()
	natt := mbTransport(t, net.JoinHostPort(espPolZeOuter, "4500"), true)
	if err := transport.EnableESPInUDP(natt.Conn()); err != nil {
		t.Fatalf("enable the private receiver's ESP-in-UDP: %v", err)
	}
	group := ipsec.ESPGroup{
		Name: "scope-gcm", Lifetime: 3600, PFS: ipsec.PFSDisable,
		Proposals: []ipsec.ESPProposal{{Number: 1, Encryption: ipsec.EncryptionAES256GCM}},
	}
	children := make([]*ChildSA, 0, len(sets))
	var first *SA
	for index, pairs := range sets {
		sa := testSA()
		sa.IsInitiator = true
		sa.ChildInboundSPI = 0x6f510001 + uint32(index)*2
		sa.ChildOutboundSPI = sa.ChildInboundSPI + 1
		sa.bindSockets(nil, natt)
		sa.floatToNATTPort()
		sa.NATDetected = true
		sa.peerEndpoint = &net.UDPAddr{IP: net.ParseIP(espPolPeerOuter), Port: transport.NATTPort}
		sa.NegotiatedPairs = pairs
		sa.NegotiatedTSi, sa.NegotiatedTSr = pairs[0].I.Net, pairs[0].R.Net
		child, err := createFirstChildSA(sa, group, espPolZeOuter, espPolPeerOuter,
			0, dataplane.Get(), slogutil.DiscardLogger())
		if err != nil {
			t.Fatalf("create negotiated multi-pair Child %d: %v", index, err)
		}
		t.Cleanup(child.Clear)
		if !child.ESPInstalled {
			t.Fatalf("multi-pair Child %d did not reach the real kernel", index)
		}
		if index == 0 {
			first = sa
		}
		children = append(children, child)
	}
	return children, first
}

// rfcScopeDelivery sends a unique UDP payload on the selected actual SA. A
// negative verdict requires that packet's state-selector rejection receipt;
// silence is never a passing rejection. Every positive pins source and payload.
func rfcScopeDelivery(t *testing.T, sender *espPolSender, listener *net.UDPConn,
	source string, port uint16, token []byte, want bool,
) {
	t.Helper()
	segment := make([]byte, 8, 8+len(token))
	binary.BigEndian.PutUint16(segment[0:2], sadSelSourcePort)
	binary.BigEndian.PutUint16(segment[2:4], port)
	binary.BigEndian.PutUint16(segment[4:6], uint16(8+len(token)))
	segment = append(segment, token...)
	before := espPolStat(t, sadSelStatStateMismatch)
	sender.sendEncap(t, sadSelIPv4(source, sadSelProtoUDP, segment))
	deadline := time.Now().Add(3 * time.Second)
	var buf [256]byte
	for range 64 {
		if !time.Now().Before(deadline) {
			break
		}
		if err := listener.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		octets, address, err := listener.ReadFromUDP(buf[:])
		if err == nil {
			if !bytes.Equal(buf[:octets], token) {
				t.Fatalf("SPI %#x port %d received another packet: %x, want %x", sender.spi, port, buf[:octets], token)
			}
			if !address.IP.Equal(net.ParseIP(source)) {
				t.Fatalf("SPI %#x received source %s, want %s", sender.spi, address.IP, source)
			}
			if address.Port != sadSelSourcePort {
				t.Fatalf("SPI %#x received source port %d, want %d", sender.spi, address.Port, sadSelSourcePort)
			}
			if got := espPolStat(t, sadSelStatStateMismatch); got != before {
				t.Fatalf("SPI %#x delivery also counted a state-selector rejection: %d -> %d", sender.spi, before, got)
			}
			if !want {
				t.Errorf("SPI %#x delivered UDP port %d outside its own negotiated scope; token %x", sender.spi, port, token)
			}
			return
		}
		var timeout net.Error
		if !errors.As(err, &timeout) {
			t.Fatalf("read the actual protected UDP receiver: %v", err)
		}
		if !timeout.Timeout() {
			t.Fatalf("read the actual protected UDP receiver: %v", err)
		}
		if got := espPolStat(t, sadSelStatStateMismatch); got != before {
			if want {
				t.Fatalf("SPI %#x rejected its permitted UDP port %d: state mismatch %d -> %d", sender.spi, port, before, got)
			}
			if got != before+1 {
				t.Fatalf("SPI %#x sent one packet but state mismatch moved %d -> %d", sender.spi, before, got)
			}
			t.Logf("SPI %#x rejected UDP port %d with its own state-selector receipt; token %x", sender.spi, port, token)
			return
		}
	}
	t.Fatalf("SPI %#x UDP port %d produced neither exact delivery nor a state-selector rejection receipt; token %x", sender.spi, port, token)
}

// RFC 4301 Section 5.2: "If an IPsec system receives an inbound packet on an SA and the
// packet's header fields are not consistent with the selectors for the SA, it MUST
// discard the packet." The sibling's valid SPD entry cannot enlarge this SA.
//
//ze:guest-root
func TestRFC4301MultipairSAsRejectSiblingScope(t *testing.T) {
	lab := newEspPolLab(t)
	espPolSPD(t)
	children, _ := rfcScopeChildren(t, rfcScopePairs(t, 5000, 5002), rfcScopePairs(t, 5001, 5003))
	if children[0].ReqID != children[1].ReqID {
		t.Fatal("the sibling fixture does not exercise the shared request-id boundary")
	}
	first := newEspPolSender(t, lab, children[0])
	senders := []*espPolSender{first, fragmentSiblingSender(t, first, children[1])}
	ports := []uint16{5000, 5002, 5001, 5003}
	listeners := make([]*net.UDPConn, len(ports))
	for index, port := range ports {
		listeners[index] = sadSelListen(t, sadSelInnerLocal, int(port))
	}
	for index, port := range ports {
		rfcScopeDelivery(t, senders[index/2], listeners[index], sadSelInnerRemote,
			port, []byte{0x51, 0, byte(index)}, true)
	}
	for index, port := range ports {
		rfcScopeDelivery(t, senders[1-index/2], listeners[index], sadSelInnerRemote,
			port, []byte{0x51, 1, byte(index)}, false)
		// A successful same-SA delivery after every denial rules out a poisoned fixture.
		rfcScopeDelivery(t, senders[index/2], listeners[index], sadSelInnerRemote,
			port, []byte{0x51, 2, byte(index)}, true)
	}
}

// Equal selector scopes may share their policy binding while retaining distinct
// SPIs. ANY selectors remain usable; they are not a reason to reject every SA.
//
//ze:guest-root
func TestRFC4301ParallelMultipairSAsKeepPermittedDelivery(t *testing.T) {
	for _, anyScope := range []bool{false, true} {
		name := "exact-services"
		if anyScope {
			name = "any-protocol-and-ports"
		}
		t.Run(name, func(t *testing.T) {
			lab := newEspPolLab(t)
			espPolSPD(t)
			pairs := rfcScopePairs(t, 5000, 5002)
			sources := []string{sadSelInnerRemote, sadSelInnerRemote}
			if anyScope {
				for index := range pairs {
					pairs[index].I.Proto, pairs[index].R.Proto = 0, 0
					pairs[index].I.Port, pairs[index].R.Port = ipsec.AnyPort(), ipsec.AnyPort()
				}
				pairs[1].R.Net = mustCIDRNet(t, "10.3.0.0/16")
				sources[1] = "10.3.0.7"
			}
			children, _ := rfcScopeChildren(t, pairs, pairs)
			if children[0].InboundSPI == children[1].InboundSPI {
				t.Fatal("parallel multi-pair controls have the same SPI")
			}
			if children[0].ReqID != children[1].ReqID {
				t.Fatal("parallel multi-pair controls no longer share their policy binding")
			}
			first := newEspPolSender(t, lab, children[0])
			senders := []*espPolSender{first, fragmentSiblingSender(t, first, children[1])}
			listeners := []*net.UDPConn{
				sadSelListen(t, sadSelInnerLocal, 5000), sadSelListen(t, sadSelInnerLocal, 5002),
			}
			for index, sender := range senders {
				for pairIndex, port := range []uint16{5000, 5002} {
					rfcScopeDelivery(t, sender, listeners[pairIndex], sources[pairIndex], port,
						[]byte{0x52, byte(index), byte(pairIndex)}, true)
				}
			}
		})
	}
}

// A legitimate widened rekey must work beside its predecessor. The predecessor
// keeps its old scope even while the successor's wider policy is installed.
//
//ze:guest-root
func TestRFC4301WidenedRekeyKeepsPredecessorScope(t *testing.T) {
	lab := newEspPolLab(t)
	espPolSPD(t)
	pairs := rfcScopePairs(t, 5000, 5002)
	children, sa := rfcScopeChildren(t, pairs)
	old := children[0]
	first := newEspPolSender(t, lab, old)
	firstPort := sadSelListen(t, sadSelInnerLocal, 5000)
	secondPort := sadSelListen(t, sadSelInnerLocal, 5002)
	extraPort := sadSelListen(t, sadSelInnerLocal, 5004)
	rfcScopeDelivery(t, first, firstPort, sadSelInnerRemote, 5000, []byte{0x53, 0}, true)
	rfcScopeDelivery(t, first, secondPort, sadSelInnerRemote, 5002, []byte{0x53, 1}, true)
	wider := rfcScopePairs(t, 5000, 5002)
	wider[0].I.Port = ipsec.AnyPort()
	next := newRekeyedChild(old, 0x6f520001, 0x6f520002, old.Keys, true, wider, modeTunnel)
	t.Cleanup(next.Clear)
	if err := next.takeEncapPorts(sa); err != nil {
		t.Fatal(err)
	}
	if err := installChildSA(next, old.ESPGroup.Proposals[0], dataplane.Get(), slogutil.DiscardLogger()); err != nil {
		t.Fatalf("install a legitimate widened rekey beside its predecessor: %v", err)
	}
	if next.ReqID != old.ReqID {
		t.Fatal("the rekey fixture lost its shared request-id overlap")
	}
	second := mhBoundarySender(t, next, first.encap)
	rfcScopeDelivery(t, second, extraPort, sadSelInnerRemote, 5004, []byte{0x53, 2}, true)
	rfcScopeDelivery(t, second, firstPort, sadSelInnerRemote, 5000, []byte{0x53, 3}, true)
	rfcScopeDelivery(t, second, secondPort, sadSelInnerRemote, 5002, []byte{0x53, 4}, true)
	rfcScopeDelivery(t, first, extraPort, sadSelInnerRemote, 5004, []byte{0x53, 5}, false)
	rfcScopeDelivery(t, first, firstPort, sadSelInnerRemote, 5000, []byte{0x53, 6}, true)
	rfcScopeDelivery(t, first, secondPort, sadSelInnerRemote, 5002, []byte{0x53, 7}, true)
}
