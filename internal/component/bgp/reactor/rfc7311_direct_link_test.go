package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
)

// TestRFC7311DirectLinkCostNeverWrapsOrDisappears exercises next-hop-self on a
// direct link with no IGP resolution, including an absent configured cost.
// RFC 7311 Section 3.4.3: "Then, when R1 changes the next hop of a route from R2
// to R1, the AIGP TLV value MUST be increased by a non-zero amount."
// RFC requirement: RFC7311-3.4.3-6 positive -- an unresolved direct link with configured cost 7 advertises received metric 100 as 107 after next-hop-self.
// RFC requirement: RFC7311-3.4.3-6 negative -- an unresolved direct link with no configured cost withdraws the route instead of announcing next-hop-self without its required increment.
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
		if mods.IsWithdraw() != (tc.link == 0) {
			t.Fatalf("link %d: withdrawal=%v", tc.link, mods.IsWithdraw())
		}
		if mods.IsWithdraw() {
			rebuilt = make([]byte, len(body))
			n := buildWithdrawalPayload(body, rebuilt)
			if n == 0 {
				t.Fatal("cost-withheld withdrawal could not be built")
			}
			rebuilt = rebuilt[:n]
		}
		// RFC 7311 Section 3.4.3: carry the received path and policy withdrawal intent.
		conn := aigpForwardToWire(t, body, rebuilt, mods.IsWithdraw())
		metric, present := aigpReceivedMetric(t, conn.written()[message.HeaderLen:])
		if present != tc.present || metric != tc.want {
			t.Fatalf("metric %d link %d: wire %d present=%v, want %d present=%v", tc.metric, tc.link, metric, present, tc.want, tc.present)
		}
		wireBody := conn.written()[message.HeaderLen:]
		if tc.link == 0 {
			if !bytes.Equal(wireBody, []byte{0, 4, 24, 10, 20, 0, 0, 0}) {
				t.Fatalf("cost-withheld route did not reach the recipient as an exact withdrawal: %x", wireBody)
			}
		}
	}
}

// aigpForwardToWire sends a received path through the real final writer with
// negotiated framing and captured source/destination ownership. A policy
// withdrawal retains the original announcement as its synthesized provenance.
func aigpForwardToWire(t *testing.T, original, transformed []byte, synthesized bool) *recordingConn {
	t.Helper()
	peer, conn := newAnnouncePeer(t, "192.0.2.2")
	source, _ := newAnnouncePeer(t, "192.0.2.1")
	for _, established := range []*Peer{source, peer} {
		established.session.localOpen = &message.Open{MyAS: 65000, HoldTime: 90}
		established.session.peerOpen = &message.Open{MyAS: 65001, HoldTime: 90}
		established.session.negotiateWith(nil, nil)
		established.setEncodingContexts(established.session.negotiated)
		t.Cleanup(established.clearEncodingContexts)
	}
	peer.session.settings.AIGPSession = new(true)
	item := fwdItem{
		peer: peer, session: peer.currentSession(), authority: adjOutForwarded,
		rawBodies: [][]byte{transformed}, sourceMessageID: 1,
		receivedPeer: source, receivedGeneration: source.forwardGeneration.Load(),
		sourcePeerStr: source.addrString,
	}
	var pool fwdPool
	defer pool.releaseItem(&item)
	received := wireu.NewWireUpdate(original, source.recvContextID())
	rebuilt := wireu.NewWireUpdate(transformed, source.recvContextID())
	// RFC 7911 Section 5: preserve ingress identity and synthesized withdrawal intent.
	if err := prepareFwdProvenance(&item, received, rebuilt, synthesized); err != nil {
		t.Fatal(err)
	}
	// RFC 7311 Section 3.4.3: write the post-policy metric or its withheld withdrawal.
	fwdBatchHandler(fwdKey{}, []fwdItem{item})
	return conn
}
