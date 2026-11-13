//go:build linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- inbound SA scope and fragment classification.
// Related: fragment_boundary_integration_linux_test.go -- real Child installation and ordinary UDP senders.
// Related: rfc4301_fragment_linux_test.go -- ordinary IPv4 fragment construction.

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
	"github.com/ze-software/ze/internal/core/slogutil"
)

// fragmentContextPacket uses the existing normal UDP builder, with a two-octet
// body identifier so a late datagram cannot count as a later case's control.
func fragmentContextPacket(port, identifier uint16) []byte {
	packet := sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(port))
	binary.BigEndian.PutUint16(packet[len(packet)-2:], identifier)
	fragmentPacketID(packet, identifier)
	return packet
}

// fragmentContextDelivered observes one ordinary UDP datagram and checks its
// exact body. Its deadline is the same bounded socket observation sadSelDelivered
// uses; an authenticated whole-packet control follows every fragment case.
func fragmentContextDelivered(t *testing.T, conn *net.UDPConn, expected []byte) bool {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	var body [256]byte
	n, _, err := conn.ReadFromUDP(body[:])
	if err != nil {
		var timeout net.Error
		if errors.As(err, &timeout) && timeout.Timeout() {
			return false
		}
		t.Fatalf("read private fragment-context UDP receiver: %v", err)
	}
	if !bytes.Equal(body[:n], expected) {
		t.Fatalf("received body %x, want current case %x; a late datagram invalidates the observation", body[:n], expected)
	}
	return true
}

// TestRFC4301FragmentReassemblyKeepsProtectionContextsSeparate checks actual local
// reassembly using normal UDP fragments in private namespaces. Both-clear BYPASS
// and same-SA protected datagrams must arrive. A PROTECT datagram assembled from
// protected and clear fragments, or fragments from SAs with different selectors,
// must not arrive, whichever fragment arrives first.
//
// RFC 4301 Appendix D.4: "The concern is that BYPASS of a cleartext, non-initial
// fragment arriving at an IPsec implementation could undermine the security
// afforded IPsec-protected traffic directed to the same destination."
// Section 5.2 requires checking the selectors "for the SA via which it was received."
// This probes context separation; it does not claim to establish the distinct
// non-initial-fragment negotiation requirement or gateway forwarding behavior.
//
//ze:guest-root
func TestRFC4301FragmentReassemblyKeepsProtectionContextsSeparate(t *testing.T) {
	lab := newEspPolLab(t)
	espPolSPD(t)
	children := fragmentChildren(t,
		fragmentPorts{local: ipsec.PortSelector{Form: ipsec.PortSingle, Port: sadSelPort}, remote: ipsec.AnyPort()},
		fragmentPorts{local: ipsec.PortSelector{Form: ipsec.PortSingle, Port: fragmentDiscardPort}, remote: ipsec.AnyPort()},
	)
	first := newEspPolSender(t, lab, children[0])
	sibling := fragmentSiblingSender(t, first, children[1])
	plain := fragmentPlainSender(t, lab)
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)
	bypassed := sadSelListen(t, sadSelInnerLocal, fragmentBypassPort)
	other := sadSelListen(t, sadSelInnerLocal, fragmentDiscardPort)
	entries := map[string]ipsec.SPDPolicy{
		"context-bypass": {
			Name: "context-bypass", Action: dataplane.SPActionBypass, Order: 1000,
			Direction: ipsec.SPDDirIn, Protocol: sadSelProtoUDP,
			LocalPrefix:  mustCIDRNet(t, "10.2.0.0/16"),
			LocalPort:    ipsec.PortSelector{Form: ipsec.PortSingle, Port: fragmentBypassPort},
			RemotePrefix: mustCIDRNet(t, "10.1.0.0/16"), RemotePort: ipsec.AnyPort(),
		},
	}
	installSPDPolicies(dataplane.Get(), nil, entries, slogutil.DiscardLogger())
	t.Cleanup(func() { removeSPDPolicies(dataplane.Get(), entries, slogutil.DiscardLogger()) })
	fragmentWantOperatorPolicies(t, entries)
	clear := func(packet []byte) { fragmentSend(t, plain, packet) }
	protected := func(packet []byte) { first.sendEncap(t, packet) }
	second := func(packet []byte) { sibling.sendEncap(t, packet) }
	// Keep namespace-sensitive sends on the lab's locked thread, rather than t.Run.
	for index, tc := range []struct {
		name                string
		port                uint16
		listener            *net.UDPConn
		initial, nonInitial func([]byte)
		want                bool
	}{
		{"both clear BYPASS", fragmentBypassPort, bypassed, clear, clear, true},
		{"both protected first SA", sadSelPort, inside, protected, protected, true},
		{"both protected sibling SA", fragmentDiscardPort, other, second, second, true},
		{"both clear PROTECT", sadSelPort, inside, clear, clear, false},
		{"protected initial and clear tail", sadSelPort, inside, protected, clear, false},
		{"clear initial and protected tail", sadSelPort, inside, clear, protected, false},
		{"first SA initial and sibling tail", sadSelPort, inside, protected, second, false},
		{"sibling initial and first SA tail", fragmentDiscardPort, other, second, protected, false},
	} {
		for _, nonInitialFirst := range []bool{false, true} {
			identifier := uint16(0x6100 + index*4)
			if nonInitialFirst {
				identifier += 2
			}
			packet := fragmentContextPacket(tc.port, identifier)
			initial, tail := fragSplit(packet)
			fragmentPacketID(initial, identifier)
			fragmentPacketID(tail, identifier)
			if nonInitialFirst {
				tc.nonInitial(tail)
				tc.initial(initial)
			} else {
				tc.initial(initial)
				tc.nonInitial(tail)
			}
			delivered := fragmentContextDelivered(t, tc.listener, packet[28:])
			t.Logf("%s nonInitialFirst=%v delivered=%v, want=%v", tc.name, nonInitialFirst, delivered, tc.want)
			if delivered != tc.want {
				t.Errorf("%s nonInitialFirst=%v delivered=%v, want=%v", tc.name, nonInitialFirst, delivered, tc.want)
			}
			control := fragmentContextPacket(sadSelPort, identifier+1)
			protected(control)
			if !fragmentContextDelivered(t, inside, control[28:]) {
				t.Fatalf("%s nonInitialFirst=%v lost the following authenticated whole-packet control", tc.name, nonInitialFirst)
			}
		}
	}
}
