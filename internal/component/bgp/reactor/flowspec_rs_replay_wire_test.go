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
	lowEventually(t, func() bool {
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
	}, "first forward withheld pending authorization")
	if flowForwardReceived(peers[1], fam) != nil {
		t.Fatal("unauthorized FlowSpec announcement reached recipient")
	}
	peers[0].send(t, flowForwardFrame(mustHex(t, "000000144001010040020602010000fdea400304c000020118c00002")))
	lowEventually(t, func() bool { return flowForwardReceived(peers[1], fam) != nil }, "retained FlowSpec authorization recovery")
	assertFlowForwardWire(t, flowForwardReceived(peers[1], fam), fam, raw)
}
