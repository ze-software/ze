package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
)

// TestRFC7311DirectLinkCostNeverWrapsOrDisappears exercises next-hop-self on a
// direct link with no IGP resolution, including an absent configured cost.
// RFC 7311 Section 3.4.3: "Then, when R1 changes the next hop of a route from R2
// to R1, the AIGP TLV value MUST be increased by a non-zero amount."
// RFC requirement: RFC7311-3.4.3-6 positive -- an unresolved direct link with configured cost 7 advertises received metric 100 as 107 after next-hop-self.
// RFC requirement: RFC7311-3.4.3-6 negative -- an unresolved direct link with cost zero removes AIGP instead of advertising its unchanged value.
// RFC requirement: RFC7311-3.4.3-2 positive -- adding a direct-link cost above the available metric range saturates at the uint64 maximum.
// RFC requirement: RFC7311-3.4.3-2 negative -- adding a direct-link cost within range emits its exact sum.
func TestRFC7311DirectLinkCostNeverWrapsOrDisappears(t *testing.T) {
	igpcost.Set(func(netip.Addr) igpcost.Distance { return igpcost.Distance{} })
	t.Cleanup(func() { igpcost.Set(nil) })
	for _, tc := range []struct {
		metric, link, want uint64
		present            bool
	}{{100, 7, 107, true}, {100, 0, 0, false}, {^uint64(0) - 5, 7, ^uint64(0), true}} {
		body := aigpTestBody(tc.metric)
		facts := &peerForwardFacts{aigpEnabled: true, localAddr: netip.MustParseAddr("192.0.2.9")}
		var mods filterapi.ModAccumulator
		mods.Op(uint8(attribute.AttrNextHop), filterapi.AttrModSet, []byte{192, 0, 2, 9})
		// RFC 7311 Section 3.4.3: production next-hop-self metric calculation.
		applyFactsAIGP(facts, payloadAIGP(body), payloadNextHop(body), body, netip.MustParseAddr("192.0.2.1"), tc.link, &mods)
		rebuilt, _, failure := buildModifiedPayload(body, &mods, attrModHandlersWithDefaults(), nil, nil)
		if failure != modifyFailureNone || rebuilt == nil {
			t.Fatalf("rebuild failed: %v", failure)
		}
		peer, conn := newAnnouncePeer(t, "192.0.2.2")
		peer.session.settings.AIGPSession = new(true)
		fwdBatchHandler(fwdKey{}, []fwdItem{{peer: peer, rawBodies: [][]byte{rebuilt}, sourceMessageID: 1}})
		metric, present := aigpReceivedMetric(t, conn.written()[message.HeaderLen:])
		if present != tc.present || metric != tc.want {
			t.Fatalf("metric %d link %d: wire %d present=%v, want %d present=%v", tc.metric, tc.link, metric, present, tc.want, tc.present)
		}
	}
}
