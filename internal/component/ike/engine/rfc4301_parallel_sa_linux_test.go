// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA install into the dataplane
// Related: rfc4301_sad_selector_linux_test.go -- the namespace probe, sender and listeners reused here
//
// VALIDATES: against a real Linux XFRM stack, that two parallel Child SAs negotiated with
// the SAME selectors (RFC 4301 Section 4.1, the QoS case) each have their inbound packets
// decrypted and delivered, interleaved, and that both are held to the one shared policy.
// PREVENTS: a second Child SA on a selector the first already holds taking the inbound
// policy over, so the first SA's packets fail the template check and the peer's traffic
// on it goes unheard.
//
// No privilege is needed: sadSelOwnNamespace re-runs the unit in a user and network
// namespace of its own, as its sibling file explains.

//go:build linux

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// parallelChildPair installs two Child SAs through createFirstChildSA over the real XFRM
// backend, both negotiated for the SAME pair: UDP from 10.1.0.0/16 to local port 5000 of
// 10.2.0.0/16, tunnel mode, one IKE SA.
func parallelChildPair(t *testing.T) (first, second *ChildSA) {
	t.Helper()
	if err := dataplane.Load("xfrm"); err != nil {
		t.Fatalf("load xfrm backend: %v", err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Errorf("close xfrm backend: %v", err)
		}
	})

	sa := testSA()
	sa.IsInitiator = true
	tsi := mustCIDRNet(t, "10.2.0.0/16")
	tsr := mustCIDRNet(t, "10.1.0.0/16")
	sa.NegotiatedTSi, sa.NegotiatedTSr = tsi, tsr
	sa.NegotiatedPairs = []tsPair{{
		I: tsSelector{Net: tsi, Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: sadSelPort}, Proto: sadSelProtoUDP},
		R: tsSelector{Net: tsr, Proto: sadSelProtoUDP},
	}}
	group := ipsec.ESPGroup{
		Name:      "esp-gcm",
		Lifetime:  3600,
		PFS:       ipsec.PFSDisable,
		Proposals: []ipsec.ESPProposal{{Number: 1, Encryption: ipsec.EncryptionAES256GCM}},
	}
	children := make([]*ChildSA, 0, 2)
	for i := range 2 {
		// Each parallel SA carries its own peer-chosen outbound SPI, as a second
		// CREATE_CHILD_SA answer would.
		sa.ChildOutboundSPI = 0x0c000001 + uint32(i)
		child, err := createFirstChildSA(sa, group, sadSelLocalOuter, sadSelRemoteOuter, 0, dataplane.Get(), slogutil.DiscardLogger())
		if err != nil {
			t.Fatalf("createFirstChildSA: %v", err)
		}
		if !child.ESPInstalled {
			t.Fatal("a parallel Child SA was not installed into XFRM")
		}
		t.Cleanup(child.Clear)
		children = append(children, child)
	}
	if children[0].InboundSPI == children[1].InboundSPI {
		t.Fatalf("the two Child SAs share inbound SPI %#x; they are not two SAs", children[0].InboundSPI)
	}
	return children[0], children[1]
}

// TestRFC4301ParallelSAsWithOneSelectorAreEachReceived proves the receiver half of RFC 4301
// Section 4.1 at the stack. Method: two Child SAs on one selector pair; an inner packet
// inside the selectors is sent on the first SA, then on the second, then on the first again,
// and each is delivered.
func TestRFC4301ParallelSAsWithOneSelectorAreEachReceived(t *testing.T) {
	// RFC requirement: RFC4301-4.1-9 positive -- two Child SAs installed on the same
	// selectors each have an inner packet inside those selectors decrypted and delivered,
	// sent first, second, then first again after its sibling.
	// RFC requirement: RFC4301-4.4.1-10 positive -- inbound traffic arriving on an SA that
	// is consistent with the inbound SPD entry Ze installs for it is delivered, on each of
	// two SAs sharing that entry.
	if !sadSelOwnNamespace(t) {
		return
	}
	sadSelNetns(t)
	first, second := parallelChildPair(t)
	senders := []*sadSelSender{newSadSelSender(t, first), newSadSelSender(t, second)}
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)

	for i, which := range []int{0, 1, 0} {
		senders[which].send(t, sadSelProtoIPv4, sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(sadSelPort)))
		if !sadSelDelivered(t, inside) {
			t.Errorf("send %d on SA %d (inbound spi %#x): the packet was not delivered", i, which, senders[which].spi)
		}
	}
}

// TestRFC4301ParallelSAsAreHeldToTheOneSharedPolicy proves that neither parallel SA is
// processed differently from its sibling. Method: the same two Child SAs; an inner packet
// whose source lies outside the shared selector is sent on each SA, and on each it is
// dropped by the inbound policy (XfrmInNoPols moves by one) and never delivered.
func TestRFC4301ParallelSAsAreHeldToTheOneSharedPolicy(t *testing.T) {
	// RFC requirement: RFC4301-4.1-9 negative -- an inner packet outside the shared
	// selectors is dropped on each of the two parallel SAs alike (XfrmInNoPols moves by one
	// per SA, nothing delivered), so neither SA is exempt from the policy its sibling obeys.
	// RFC requirement: RFC4301-4.4.1-10 negative -- inbound traffic arriving on an SA that
	// is inconsistent with the SPD (inner source outside the selector) is refused by the
	// kernel's SPD search after decryption (XfrmInNoPols) and not delivered.
	if !sadSelOwnNamespace(t) {
		return
	}
	sadSelNetns(t)
	first, second := parallelChildPair(t)
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)

	for which, child := range []*ChildSA{first, second} {
		sender := newSadSelSender(t, child)
		before := sadSelStat(t, sadSelStatNoPols)
		sender.send(t, sadSelProtoIPv4, sadSelIPv4(sadSelInnerStray, sadSelProtoUDP, sadSelUDP(sadSelPort)))
		if sadSelDelivered(t, inside) {
			t.Errorf("SA %d: a packet from outside the shared selector was delivered", which)
		}
		if got := sadSelStat(t, sadSelStatNoPols) - before; got != 1 {
			t.Errorf("SA %d: %s moved by %d, want 1 (dropped by the shared inbound policy)", which, sadSelStatNoPols, got)
		}
	}
}
