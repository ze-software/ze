// Design: docs/architecture/plugin/rib-storage-design.md -- retained FlowSpec attributes.
package reactor

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// RFC 8955 Section 7: "All Traffic Filtering Actions are specified as transitive
// BGP Extended Communities." The first forward is deliberately ineligible;
// the only subsequent input is its covering unicast route, never a second rule.
// This forces authorization recovery to reconstruct the retained attributes.
func TestFlowSpecRouteServerReplayKeepsActions(t *testing.T) {
	peers := flowForwardLiveRouter(t, "self")
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	raw := mustHex(t, "050118c00002")
	key := ribevents.ValidationRoute{Peer: peers[0].peer.Settings().Address, Family: fam, NLRI: string(raw)}
	peers[0].send(t, flowForwardFrame(flowForwardPayload(t, fam, raw, mustHex(t, "c0000202"))))
	lowEventually(t, func() bool { return ribevents.RoutePresent(key) }, "retained unauthorized FlowSpec rule")
	if ribevents.RouteEligible(key, 0) {
		t.Fatal("FlowSpec rule authorized without a covering route")
	}
	// RFC 4271 Section 3.1: the recipient was never advertised the pending
	// rule, so it is never sent the rule's withdrawal either.
	withdrawnRule := func() bool {
		peer := peers[1]
		peer.mu.Lock()
		defer peer.mu.Unlock()
		for _, frame := range peer.frames {
			if frame[18] != 2 {
				continue
			}
			update, err := message.UnpackUpdate(frame[message.HeaderLen:])
			if err != nil {
				continue
			}
			_, _, mp, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
			if found && bytes.Equal(mp, mustHex(t, "000185050118c00002")) {
				return true
			}
		}
		return false
	}
	// Causal marker: an unrelated unicast route the source sends after the
	// rule. One source's UPDATEs are forwarded in receipt order, so the marker
	// reaching the recipient proves the rule's forward was already decided,
	// and the absence check below is not a race with that decision.
	// 198.51.100.0/24 does not cover the rule's destination 192.0.2.0/24, so
	// it cannot authorize it (RFC 8955 Section 6).
	marker := mustHex(t, "18c63364")
	peers[0].send(t, flowForwardFrame(mustHex(t, "000000144001010040020602010000fdea400304c0000201"+"18c63364")))
	lowEventually(t, func() bool {
		peer := peers[1]
		peer.mu.Lock()
		defer peer.mu.Unlock()
		for _, frame := range peer.frames {
			if frame[18] == 2 && bytes.HasSuffix(frame, marker) {
				return true
			}
		}
		return false
	}, "marker route forwarded after the pending rule")
	if flowForwardReceived(peers[1], fam) != nil {
		t.Fatal("unauthorized FlowSpec announcement reached recipient")
	}
	peers[0].send(t, flowForwardFrame(mustHex(t, "000000144001010040020602010000fdea400304c000020118c00002")))
	lowEventually(t, func() bool { return flowForwardReceived(peers[1], fam) != nil }, "retained FlowSpec authorization recovery")
	assertFlowForwardWire(t, flowForwardReceived(peers[1], fam), fam, raw)
	if withdrawnRule() {
		t.Fatal("recipient was sent the withdrawal of a FlowSpec rule it was never advertised")
	}
}
