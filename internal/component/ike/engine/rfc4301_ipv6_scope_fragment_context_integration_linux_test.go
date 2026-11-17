//go:build integration && linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- IPv6 fragment queue SA identities.
// Related: rfc4301_any_scope_fragment_context_integration_linux_test.go -- IPv4 matrix and SA counters.
// Related: rfc4301_transport_fragment_boundary_integration_linux_test.go -- IPv6 packet builders.
// RFC naming: untagged -- defensive IPv6 identical-ANY-scope isolation probe awaiting kernel proof.

package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
)

// TestRFC4301IPv6AnyScopeFragmentsKeepSAIdentity sends ordinary IPv6 UDP
// fragments through two real IPv4-outer tunnel Children with identical ANY
// scopes. Same-SA pieces must deliver their exact body and source; mixed-SA
// pieces must not. Both arrival orders, with and without preceding Destination
// Options, retain exact receiving-SA counts and following whole controls on both
// SAs. Successful UDP6 delivery also requires the builder's nonzero checksum.
//
// RFC 4301 Section 7.1: "An SA defined in this fashion will carry all traffic
// including initial and non-initial fragments for the indicated Local/Remote
// addresses and specified Next Layer protocol(s)."
// ANY selectors permit these tails without Section 7.3 negotiation. Raw scope
// readback requires the production noninitial permission to remain false.
// This additional permitted case changes no existing constrained-fragment test.
//
// MUTATION: In a private K2 proof kernel, after the original validation in
// inet_frag_xfrm_context_init, zero only context->scopes and set context->len to 0.
// Preserve K3's context->raw authority, K1 scope matching and K3 admission.
// Mixed-SA fragments then assemble and this test's own non-delivery assertions
// must fail while same-SA and whole-packet controls survive. A run must identify the actual
// reassembly path; delivery alone does not prove both IPv6 queue constructors ran.
//
//ze:guest-root
func TestRFC4301IPv6AnyScopeFragmentsKeepSAIdentity(t *testing.T) {
	lab := newEspPolLab(t)
	for _, conf := range []string{"all", "default", "lo"} {
		path := "/proc/sys/net/ipv6/conf/" + conf + "/disable_ipv6"
		if err := os.WriteFile(path, []byte("0"), 0o600); err != nil {
			t.Fatalf("enable IPv6 in the private receiver: %v", err)
		}
	}
	mhBoundaryAddress(t, "lo", transportFragmentLocal+"/128")
	espPolSPD(t)
	pairs := []tsPair{{
		I: tsSelector{Net: mustCIDRNet(t, transportFragmentLocal+"/128"), Proto: 0, Port: ipsec.AnyPort()},
		R: tsSelector{Net: mustCIDRNet(t, transportFragmentRemote+"/128"), Proto: 0, Port: ipsec.AnyPort()},
	}}
	// RFC 4301 Sections 4.4.2 and 7.1: install naturally permitted IPv6 ANY scopes.
	children, _ := rfcScopeChildren(t, pairs, pairs)
	if children[0].InboundSPI == children[1].InboundSPI {
		t.Fatal("the IPv6 tunnel Children do not have independent inbound SPIs")
	}
	for _, child := range children {
		// RFC 4301 Section 4.4.2: verify both exact directional scopes and zero flags.
		rfcScopeWantState(t, child, pairs, true, false)
		rfcScopeWantState(t, child, pairs, false, false)
	}
	first := newEspPolSender(t, lab, children[0])
	second := fragmentSiblingSender(t, first, children[1])
	inside := transportFragmentListen(t, transportFragmentPort)
	anyScopeFragmentWantPackets(t, first)
	anyScopeFragmentWantPackets(t, second)
	// Keep all namespace-sensitive work on the fixture's locked thread.
	for optionsIndex, options := range []bool{false, true} {
		for caseIndex, tc := range []struct {
			name          string
			initial, tail *espPolSender
			want          bool
		}{
			{"A+A", first, first, true},
			{"B+B", second, second, true},
			{"A+B", first, second, false},
			{"B+A", second, first, false},
		} {
			for _, tailFirst := range []bool{false, true} {
				identifier := uint32(0x6a600000 + optionsIndex*0x100 + caseIndex*8)
				if tailFirst {
					identifier += 4
				}
				body := ipv6ScopeFragmentBody(identifier)
				udp := transportFragmentUDP(transportFragmentPort, body)
				initialNext, initial := transportFragmentOptions(
					transportFragmentHeader(udp[:16], 1, identifier), options)
				tailNext, tail := transportFragmentOptions(
					transportFragmentHeader(udp[16:], 16, identifier), options)
				initial = transportFragmentIPv6(initialNext, initial)
				tail = transportFragmentIPv6(tailNext, tail)
				if tailFirst {
					ipv6ScopeFragmentSend(t, tc.tail, tail)
					ipv6ScopeFragmentSend(t, tc.initial, initial)
				} else {
					ipv6ScopeFragmentSend(t, tc.initial, initial)
					ipv6ScopeFragmentSend(t, tc.tail, tail)
				}
				delivered := ipv6ScopeFragmentDelivered(t, inside, body)
				if delivered != tc.want {
					t.Errorf("%s options=%v tailFirst=%v id=%#x delivered=%v, want %v",
						tc.name, options, tailFirst, identifier, delivered, tc.want)
				}
				anyScopeFragmentWantPackets(t, first)
				anyScopeFragmentWantPackets(t, second)
				for controlIndex, sender := range []*espPolSender{first, second} {
					controlBody := ipv6ScopeFragmentBody(identifier + 1 + uint32(controlIndex))
					control := transportFragmentIPv6(unix.IPPROTO_UDP,
						transportFragmentUDP(transportFragmentPort, controlBody))
					ipv6ScopeFragmentSend(t, sender, control)
					if !ipv6ScopeFragmentDelivered(t, inside, controlBody) {
						t.Fatalf("%s options=%v tailFirst=%v lost whole control on SA %#x",
							tc.name, options, tailFirst, sender.spi)
					}
					anyScopeFragmentWantPackets(t, sender)
				}
				t.Logf("%s options=%v tailFirst=%v id=%#x delivered=%v",
					tc.name, options, tailFirst, identifier, delivered)
			}
		}
	}
}

// ipv6ScopeFragmentBody keeps a unique case token in a fragmentable UDP body.
func ipv6ScopeFragmentBody(identifier uint32) []byte {
	body := make([]byte, 24)
	copy(body, "ipv6-any-context-")
	binary.BigEndian.PutUint32(body[20:], identifier)
	return body
}

// TestRFC4301IPv6FragmentGatherKeepsFirstHopOptions checks the receive cursor
// after Hop-by-Hop processing and reassembly. Destination Options belong to
// the fragmentable data, before UDP. A completing tail can have different
// per-fragment headers, but the offset-zero packet supplies the retained headers.
// RFC 8200 Section 4.5: "The number and content of the headers preceding the
// Fragment header of different fragments of the same original packet may differ."
// "Only those headers in the Offset zero fragment packet are retained in the
// reassembled packet."
// The eleven Destination Options headers plus one processed Hop-by-Hop header
// reach Linux 7.3's twelve-header traversal limit (include/net/ipv6.h).
// Reprocessing Hop-by-Hop as Destination Options would exceed that limit.
// RFC 8200 Section 4.1: "IPv6 nodes must accept and attempt to process extension
// headers in any order and occurring any number of times in the same packet,
// except for the Hop-by-Hop Options header, which is restricted to
// appear immediately after an IPv6 header only."
// MUTATION: restore the completing fragment's receive cursor after gathering,
// or always restart at byte 40; this test's exact deliveries must then fail.
//
//ze:guest-root
func TestRFC4301IPv6FragmentGatherKeepsFirstHopOptions(t *testing.T) {
	lab := newEspPolLab(t)
	for _, conf := range []string{"all", "default", "lo"} {
		path := "/proc/sys/net/ipv6/conf/" + conf + "/disable_ipv6"
		if err := os.WriteFile(path, []byte("0"), 0o600); err != nil {
			t.Fatalf("enable IPv6 in the private Hop-by-Hop receiver: %v", err)
		}
	}
	mhBoundaryAddress(t, "lo", transportFragmentLocal+"/128")
	espPolSPD(t)
	pairs := []tsPair{{
		I: tsSelector{Net: mustCIDRNet(t, transportFragmentLocal+"/128"), Proto: 0, Port: ipsec.AnyPort()},
		R: tsSelector{Net: mustCIDRNet(t, transportFragmentRemote+"/128"), Proto: 0, Port: ipsec.AnyPort()},
	}}
	// RFC 4301 Sections 4.4.2 and 7.1: the real Child permits these IPv6 fragments.
	children, _ := rfcScopeChildren(t, pairs)
	child := children[0]
	rfcScopeWantState(t, child, pairs, true, false)
	rfcScopeWantState(t, child, pairs, false, false)
	sender := newEspPolSender(t, lab, child)
	inside := transportFragmentListen(t, transportFragmentPort)
	anyScopeFragmentWantPackets(t, sender)
	// A whole packet first proves that the header chain and UDP checksum are
	// accepted without reassembly. Each option header has six bytes of PadN.
	controlBody := ipv6ScopeFragmentBody(0x6a610000)
	controlPayload := ipv6ScopeDestinationChain(controlBody)
	ipv6ScopeFragmentSend(t, sender, ipv6ScopeHopPacket(unix.IPPROTO_DSTOPTS, controlPayload, true))
	if !ipv6ScopeFragmentDelivered(t, inside, controlBody) {
		t.Fatal("whole Hop-by-Hop and Destination Options control was not delivered")
	}
	anyScopeFragmentWantPackets(t, sender)
	// The fixture locks this thread; do not move namespace-sensitive work to t.Run.
	for caseIndex, tc := range []struct {
		name                string
		initialHop, tailHop bool
	}{
		{"both Hop-by-Hop", true, true},
		{"initial Hop-by-Hop only", true, false},
		{"tail Hop-by-Hop only", false, true},
	} {
		for _, tailFirst := range []bool{false, true} {
			identifier := uint32(0x6a611000 + caseIndex*8)
			if tailFirst {
				identifier += 4
			}
			body := ipv6ScopeFragmentBody(identifier)
			// RFC 8200 Sections 4.1 and 4.5: Destination Options follow Fragment.
			// The initial 104 bytes contain every option and the full UDP header,
			// plus eight body bytes. The tail's offset excludes per-fragment HBH.
			const initialOctets = ipv6ScopeDestinationHeaderCount*8 + 16
			fragmentable := ipv6ScopeDestinationChain(body)
			initial := transportFragmentHeader(fragmentable[:initialOctets], 1, identifier)
			tail := transportFragmentHeader(fragmentable[initialOctets:], initialOctets, identifier)
			initial[0], tail[0] = unix.IPPROTO_DSTOPTS, unix.IPPROTO_DSTOPTS
			initial = ipv6ScopeHopPacket(unix.IPPROTO_FRAGMENT, initial, tc.initialHop)
			tail = ipv6ScopeHopPacket(unix.IPPROTO_FRAGMENT, tail, tc.tailHop)
			if tailFirst {
				ipv6ScopeFragmentSend(t, sender, tail)
				ipv6ScopeFragmentSend(t, sender, initial)
			} else {
				ipv6ScopeFragmentSend(t, sender, initial)
				ipv6ScopeFragmentSend(t, sender, tail)
			}
			delivered := ipv6ScopeFragmentDelivered(t, inside, body)
			if !delivered {
				t.Errorf("%s tailFirst=%v id=%#x was not delivered", tc.name, tailFirst, identifier)
			}
			anyScopeFragmentWantPackets(t, sender)
			following := ipv6ScopeFragmentBody(identifier + 1)
			ipv6ScopeFragmentSend(t, sender, transportFragmentIPv6(unix.IPPROTO_UDP,
				transportFragmentUDP(transportFragmentPort, following)))
			if !ipv6ScopeFragmentDelivered(t, inside, following) {
				t.Fatalf("%s tailFirst=%v lost the following whole control", tc.name, tailFirst)
			}
			anyScopeFragmentWantPackets(t, sender)
			t.Logf("%s tailFirst=%v id=%#x delivered=%v", tc.name, tailFirst, identifier, delivered)
		}
	}
}

// Linux 7.3 includes the already processed HBH in IP6_MAX_EXT_HDRS_CNT (12).
// A chain at that bound exposes an extra parse without requiring a harmful option.
const ipv6ScopeDestinationHeaderCount = 11

func ipv6ScopeDestinationChain(body []byte) []byte {
	payload := transportFragmentUDP(transportFragmentPort, body)
	next := byte(unix.IPPROTO_UDP)
	for range ipv6ScopeDestinationHeaderCount {
		payload = append([]byte{next, 0, 1, 4, 0, 0, 0, 0}, payload...)
		next = unix.IPPROTO_DSTOPTS
	}
	return payload
}

// ipv6ScopeHopPacket places a valid eight-byte Hop-by-Hop header immediately
// after IPv6. RFC 8200 Section 4.2 defines PadN as type 1, length N-2 and zero data.
func ipv6ScopeHopPacket(next byte, payload []byte, hop bool) []byte {
	if hop {
		payload = append([]byte{next, 0, 1, 4, 0, 0, 0, 0}, payload...)
		next = unix.IPPROTO_HOPOPTS
	}
	return transportFragmentIPv6(next, payload)
}

// ipv6ScopeFragmentSend reuses the fixture's authenticated encoder and socket,
// changing only the ESP next-header to IPv6 for the actual inner packet family.
func ipv6ScopeFragmentSend(t *testing.T, sender *espPolSender, packet []byte) {
	t.Helper()
	target := &net.UDPAddr{IP: net.ParseIP(espPolZeOuter), Port: transport.NATTPort}
	// RFC 4303 Section 2.6: the ESP trailer identifies the encapsulated IPv6 packet.
	if _, err := sender.encap.WriteToUDP(sender.seal(unix.IPPROTO_IPV6, packet), target); err != nil {
		t.Fatalf("send private IPv6 inner ESP-in-UDP: %v", err)
	}
}

// ipv6ScopeFragmentDelivered bounds the observation and refuses a late case's
// body or a different source, so neither can masquerade as the current control.
func ipv6ScopeFragmentDelivered(t *testing.T, listener *net.UDPConn, expected []byte) bool {
	t.Helper()
	if err := listener.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	var body [256]byte
	n, source, err := listener.ReadFromUDP(body[:])
	if err != nil {
		var timeout net.Error
		if errors.As(err, &timeout) && timeout.Timeout() {
			return false
		}
		t.Fatalf("read private IPv6 fragment-context UDP: %v", err)
	}
	if !source.IP.Equal(net.ParseIP(transportFragmentRemote)) {
		t.Fatalf("IPv6 fragment-context source = %s, want %s", source, transportFragmentRemote)
	}
	if source.Port != sadSelSourcePort {
		t.Fatalf("IPv6 fragment-context source port = %d, want %d", source.Port, sadSelSourcePort)
	}
	if !bytes.Equal(body[:n], expected) {
		t.Fatalf("IPv6 fragment-context body = %x, want current case %x", body[:n], expected)
	}
	return true
}
