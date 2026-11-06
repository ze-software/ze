// Design: docs/architecture/core-design.md -- final-only export policy reconciliation.
package reactor

import (
	"bytes"
	"encoding/json"
	"net/netip"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestOrdinaryRefreshDoesNotDoublePrepend proves a route refresh re-advertises
// the AS_PATH the peer was first sent when the export policy has not changed.
//
// The RIB's Adj-RIB-Out holds the attributes as sent, after export policy, and a
// refresh replays them through the ordinary writer's export gate. This drives
// that: the first send's own attributes come back as a replay of its receipt.
//
// VALIDATES: RFC 2918 Section 4 re-advertisement of the same route.
// PREVENTS: an as-path prepend member applying again on every refresh, so the peer
// sees a longer path for a route nothing changed.
func TestOrdinaryRefreshDoesNotDoublePrepend(t *testing.T) {
	judge := func(rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
		return &rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{
			{Field: "as-path", Action: rpc.FilterSetActionPrepend, Value: json.RawMessage("1")},
		}}, nil
	}
	transport := &writerBoundaryTransport{answerTransport: &answerTransport{
		reads: []string{"prefix"}, sets: []string{"as-path"}}, conn: errorVerdictConnection(t, true, judge)}
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
	peer, conn := writerBoundaryPeer(t, r)
	peer.settings.ExportPolicy = answerPolicyAt(t, "bgp export", routepolicy.FinalAccept, "prepend")
	a := &reactorAPIAdapter{r: r}
	target := announceTarget{peer: peer, session: peer.session}
	nextHop := netip.MustParseAddr("10.0.0.1")

	send := func(batch bgptypes.NLRIBatch) []byte {
		t.Helper()
		facts := announceFactsFor(peer, batch.Family, nextHop, false, peer.negotiated.Load())
		update, err := a.buildBatchAnnounceUpdate(make([]byte, 4096), make([]byte, 4096), batch, facts)
		if err != nil {
			t.Fatal(err)
		}
		before := len(conn.written())
		if err := target.sendBatchUpdate(t.Context(), update, 4096, false, batch); err != nil {
			t.Fatal(err)
		}
		bodies := updateBodies(t, conn.written()[before:])
		if len(bodies) != 1 {
			t.Fatalf("output=%x, want one UPDATE", bodies)
		}
		return bodies[0]
	}

	first := staleReadvertiseBatch(t)
	first.Stale = 0
	sent := send(first)
	sections, err := wire.ParseUpdateSections(sent)
	if err != nil {
		t.Fatal(err)
	}
	attrs := sections.Attrs(sent)
	_, _, firstPath, found := attribute.AttrFind(attrs, attribute.AttrASPath)
	if !found {
		t.Fatalf("first send carried no AS_PATH: %x", sent)
	}

	refresh := first
	refresh.Attrs = nil
	refresh.Wire = attribute.NewAttributesWire(attrs, bgpctx.APIContextID)
	refresh.Replay = true
	refresh.SentOwnerMessage = peer.session.sentReceipt.MessageID()
	if refresh.SentOwnerMessage == 0 {
		t.Fatal("first send produced no ownership receipt")
	}
	again := send(refresh)
	sections, err = wire.ParseUpdateSections(again)
	if err != nil {
		t.Fatal(err)
	}
	_, _, refreshedPath, found := attribute.AttrFind(sections.Attrs(again), attribute.AttrASPath)
	if !found || !bytes.Equal(refreshedPath, firstPath) {
		t.Fatalf("refresh AS_PATH=%x, want the path first sent %x", refreshedPath, firstPath)
	}
}
