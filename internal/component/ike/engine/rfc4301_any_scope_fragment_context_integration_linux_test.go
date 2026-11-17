//go:build integration && linux

// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- immutable SA identities in fragment queues.
// Related: rfc4301_state_scope_integration_linux_test.go -- production Child creation.
// Related: fragment_context_integration_linux_test.go -- exact fragment payload observation.
// RFC naming: untagged -- defensive identical-ANY-scope queue isolation probe awaiting kernel proof.

package engine

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// TestRFC4301AnyScopeFragmentsKeepSAIdentity checks that identical allowed
// selectors do not make independently authenticated SAs share a reassembly queue.
// Same-SA fragments must deliver their exact UDP body; mixed-SA fragments must
// not, in both arrival orders. Every case also delivers a whole-packet control
// on each SA and checks each receiving SA's exact authenticated packet count.
//
// RFC 4301 Section 7.1: "An SA defined in this fashion will carry all traffic
// including initial and non-initial fragments for the indicated Local/Remote
// addresses and specified Next Layer protocol(s)."
// Both SAs have protocol ANY and ports ANY. Their noninitial permission remains
// false: the Section 7.3 negotiation condition applies to non-trivial selectors.
// This defensive check does not replace the different-selector or clear/protected
// cases in TestRFC4301FragmentReassemblyKeepsProtectionContextsSeparate.
//
// MUTATION: In a private K2 proof kernel, after the original validation in
// inet_frag_xfrm_context_init, zero only context->scopes and set context->len to 0.
// Preserve K3's context->raw authority, K1 scope matching and K3 admission.
// Mixed-SA pieces remain individually allowed but assemble,
// so their exact-body non-delivery assertions must fail; same-SA controls survive.
//
//ze:guest-root
func TestRFC4301AnyScopeFragmentsKeepSAIdentity(t *testing.T) {
	lab := newEspPolLab(t)
	espPolSPD(t)
	pairs := []tsPair{{
		I: tsSelector{Net: mustCIDRNet(t, "10.2.0.0/16"), Proto: 0, Port: ipsec.AnyPort()},
		R: tsSelector{Net: mustCIDRNet(t, "10.1.0.0/16"), Proto: 0, Port: ipsec.AnyPort()},
	}}
	// RFC 4301 Sections 4.4.2 and 7.1: create real SAs with identical ANY scopes.
	children, _ := rfcScopeChildren(t, pairs, pairs)
	if children[0].InboundSPI == children[1].InboundSPI {
		t.Fatal("the two production Children did not receive independent inbound SPIs")
	}
	for _, child := range children {
		// RFC 4301 Section 4.4.2: read the installed scope, including zero permission flags.
		rfcScopeWantState(t, child, pairs, true, false)
		rfcScopeWantState(t, child, pairs, false, false)
	}
	first := newEspPolSender(t, lab, children[0])
	second := fragmentSiblingSender(t, first, children[1])
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)
	anyScopeFragmentWantPackets(t, first)
	anyScopeFragmentWantPackets(t, second)
	// Namespace-sensitive operations MUST stay on the lab's locked thread.
	for index, tc := range []struct {
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
			identifier := uint16(0x6a00 + index*8)
			if tailFirst {
				identifier += 4
			}
			packet := fragmentContextPacket(sadSelPort, identifier)
			initial, tail := fragSplit(packet)
			fragmentPacketID(initial, identifier)
			fragmentPacketID(tail, identifier)
			if tailFirst {
				tc.tail.sendEncap(t, tail)
				tc.initial.sendEncap(t, initial)
			} else {
				tc.initial.sendEncap(t, initial)
				tc.tail.sendEncap(t, tail)
			}
			delivered := fragmentContextDelivered(t, inside, packet[28:])
			if delivered != tc.want {
				t.Errorf("%s tailFirst=%v id=%#x delivered=%v, want %v",
					tc.name, tailFirst, identifier, delivered, tc.want)
			}
			// A receive counter proves each authenticated piece reached its actual SA.
			anyScopeFragmentWantPackets(t, first)
			anyScopeFragmentWantPackets(t, second)
			for controlIndex, sender := range []*espPolSender{first, second} {
				control := fragmentContextPacket(sadSelPort, identifier+1+uint16(controlIndex))
				sender.sendEncap(t, control)
				if !fragmentContextDelivered(t, inside, control[28:]) {
					t.Fatalf("%s tailFirst=%v lost whole control on SA %#x",
						tc.name, tailFirst, sender.spi)
				}
				anyScopeFragmentWantPackets(t, sender)
			}
			t.Logf("%s tailFirst=%v id=%#x delivered=%v", tc.name, tailFirst, identifier, delivered)
		}
	}
}

// anyScopeFragmentWantPackets compares the receiving SAD counter with the exact
// ESP sequence sent from the private peer. A missing fragment is never evidence
// of queue isolation, even when its matching whole-packet control arrives.
func anyScopeFragmentWantPackets(t *testing.T, sender *espPolSender) {
	t.Helper()
	state, err := netlink.XfrmStateGet(&netlink.XfrmState{
		Dst: net.ParseIP(espPolZeOuter), Spi: int(sender.spi), Proto: netlink.XFRM_PROTO_ESP,
	})
	if err != nil {
		t.Fatalf("read receiving SA %#x packet counter: %v", sender.spi, err)
	}
	if state.Statistics.Packets != uint64(sender.seq) {
		t.Fatalf("receiving SA %#x authenticated %d packets, want exactly %d sent pieces and controls",
			sender.spi, state.Statistics.Packets, sender.seq)
	}
}
