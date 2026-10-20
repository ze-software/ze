// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the negotiated pair reaches the policy exactly
// RFC: rfc/short/rfc4301.md -- RFC4301-4.4.2-2

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// pprPort80 is the single port both tests here negotiate on the TSr half.
var pprPort80 = ipsec.PortSelector{Form: ipsec.PortSingle, Port: 80}

// pprProtocols returns the upper-layer protocol of every policy the Child SA installs,
// inbound first, then outbound.
func pprProtocols(child *ChildSA) []uint8 {
	var out []uint8
	for _, dir := range []dataplane.SADir{dataplane.SADirIn, dataplane.SADirOut} {
		for _, sp := range childPolicies(child, dir) {
			out = append(out, sp.UpperProto)
		}
	}
	return out
}

// pprWantOnly fails unless protocols holds exactly one inbound and one outbound policy,
// both for want.
func pprWantOnly(t *testing.T, side string, protocols []uint8, want uint8) {
	t.Helper()
	if len(protocols) != 2 {
		t.Fatalf("%s: %d policies installed for one negotiated pair, want one per direction", side, len(protocols))
	}
	for i, got := range protocols {
		if got != want {
			t.Errorf("%s: policy %d installs protocol %d, want %d, the intersection of the pair's two halves", side, i, got, want)
		}
	}
}

// TestRFC4301MixedProtocolPairInstallsTheIntersection drives the two producers of a
// negotiated pair, recordInitiatorSelectors and narrowChildSelectors, then childPolicies,
// which turns each pair into the policies the dataplane installs.
//
// VALIDATES: a pair whose TSi carries protocol 0 (any) and whose TSr carries TCP installs
// TCP in both directions, on the initiator and on the responder; the mirror pair (TSi
// UDP, TSr any) installs UDP.
// PREVENTS: the policy reading the TSi half alone, which installed "any protocol" for a
// pair whose packets can only be TCP, wider than the negotiation.
func TestRFC4301MixedProtocolPairInstallsTheIntersection(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2-2 positive -- a negotiated pair of TSi protocol any and TSr TCP port 80 installs one inbound and one outbound policy, each for TCP, on both the initiator and the responder; a pair of TSi UDP and TSr any installs UDP.
	// RFC requirement: RFC4301-4.4.2-2 negative -- no policy of a pair of TSi protocol any and TSr TCP carries protocol any, so the installed entry is never wider than the negotiated value.
	tsi := selectorsToWire([]tsSelector{{Net: mustNet(t, "10.1.0.0/16"), Port: ipsec.AnyPort(), Proto: 0}}, wire.PayloadTypeTSi)
	tsr := selectorsToWire([]tsSelector{{Net: mustNet(t, "10.2.0.0/16"), Port: pprPort80, Proto: 6}}, wire.PayloadTypeTSr)

	initiator := &SA{IsInitiator: true}
	if err := recordInitiatorSelectors(initiator, tsi, tsr, nil); err != nil {
		t.Fatalf("the initiator refused an answer equal to its offer: %v", err)
	}
	pprWantOnly(t, "initiator", pprProtocols(&ChildSA{Selectors: initiator.NegotiatedPairs, SelectorsLocalIsTSi: true}), 6)

	responder := &SA{PeerCfg: ipsec.SiteToSitePeer{Name: "ppr"}}
	if err := narrowChildSelectors(responder, tsi, tsr, nil); err != nil {
		t.Fatalf("the responder refused a proposal its empty policy accepts: %v", err)
	}
	pprWantOnly(t, "responder", pprProtocols(&ChildSA{Selectors: responder.NegotiatedPairs}), 6)

	udp := []tsPair{{
		I: tsSelector{Net: mustNet(t, "10.1.0.0/16"), Port: ipsec.AnyPort(), Proto: 17},
		R: tsSelector{Net: mustNet(t, "10.2.0.0/16"), Port: ipsec.AnyPort(), Proto: 0},
	}}
	pprWantOnly(t, "mirror", pprProtocols(&ChildSA{Selectors: udp, SelectorsLocalIsTSi: true}), 17)
}
