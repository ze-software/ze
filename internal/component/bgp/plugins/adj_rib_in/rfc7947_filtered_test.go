// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 7947 Section 2.1 import-filtered Adj-RIB-In entries
// RFC: rfc/short/rfc7947.md -- Section 2.1
// Related: rib.go, rib_commands.go, rib_eligibility.go, rib_replay_path.go
package adj_rib_in

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func ipv4FilteredBody(nlri []byte, withdrawn []byte) []byte {
	// ORIGIN(IGP)=40010100, NEXT_HOP(10.0.0.1)=4003040a000001
	attrs := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x03, 0x04, 10, 0, 0, 1}
	if len(nlri) == 0 {
		attrs = nil
	}
	body := make([]byte, 0, 2+len(withdrawn)+2+len(attrs)+len(nlri))
	body = append(body, byte(len(withdrawn)>>8), byte(len(withdrawn)))
	body = append(body, withdrawn...)
	body = append(body, byte(len(attrs)>>8), byte(len(attrs)))
	body = append(body, attrs...)
	body = append(body, nlri...)
	return body
}

func structuredFilteredEvent(t *testing.T, ctxID bgpctx.ContextID, peer string, owner, connID, msgID uint64, kind rpc.EventKind, step string, body []byte) *rpc.StructuredEvent {
	t.Helper()
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, err := wu.Attrs()
	require.NoError(t, err)
	return &rpc.StructuredEvent{
		EventType:    kind,
		PeerAddress:  peer,
		PeerOwner:    owner,
		ConnectionID: connID,
		MessageID:    msgID,
		RawMessage: &bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			MessageID:  msgID,
			WireUpdate: wu,
			AttrsWire:  attrs,
			Filtered:   kind == rpc.EventKindUpdateFiltered,
			FilterStep: step,
		},
	}
}

func statusMap(t *testing.T, r *AdjRIBInManager) map[string]any {
	t.Helper()
	m, ok := r.status().(map[string]any)
	require.True(t, ok, "status must return map[string]any")
	return m
}

// TestAdjRIBInFilteredLifecycleAndViews covers structured & JSON filtered ingest,
// show/status payloads (including json/yaml/table pipe rendering), cursor advance
// across filtered entries without stalling, single-path replay withdrawal,
// ingestPosition advance, explicit withdrawal removal, and peer-down cleanup.
//
// VALIDATES: AC-4, AC-6, AC-7, AC-8, AC-9, and A-7 of spec-rfc7947-adj-rib-in-accepts-filtered-updates.
func TestAdjRIBInFilteredLifecycleAndViews(t *testing.T) {
	declareShapes(t)
	r := newTestManager(t)
	r.ingestTracked = true

	const peer = "10.0.0.1"
	const owner uint64 = 11
	const conn uint64 = 1
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, nil))
	require.NoError(t, err)

	r.handleStructuredState(&rpc.StructuredEvent{
		EventType:    rpc.EventKindState,
		PeerAddress:  peer,
		PeerOwner:    owner,
		ConnectionID: conn,
		State:        rpc.SessionStateUp,
	})

	// 1. Accepted route msgID=10 (10.0.0.0/24).
	r.handleReceivedStructured(structuredFilteredEvent(t, ctxID, peer, owner, conn, 10,
		rpc.EventKindUpdate, "", ipv4FilteredBody([]byte{24, 10, 0, 0}, nil)))
	// 2. Filtered route msgID=11 (10.0.1.0/24).
	r.handleReceivedStructured(structuredFilteredEvent(t, ctxID, peer, owner, conn, 11,
		rpc.EventKindUpdateFiltered, "loop", ipv4FilteredBody([]byte{24, 10, 0, 1}, nil)))
	// 3. Accepted route msgID=12 (10.0.2.0/24).
	r.handleReceivedStructured(structuredFilteredEvent(t, ctxID, peer, owner, conn, 12,
		rpc.EventKindUpdate, "", ipv4FilteredBody([]byte{24, 10, 0, 2}, nil)))

	// A-7: ingestPosition advances across filtered events.
	pos, tracked := r.ingestPosition()
	assert.True(t, tracked)
	assert.Equal(t, uint64(12), pos)

	// AC-9: status reports filtered-routes separately while total-routes counts all.
	st := statusMap(t, r)
	assert.Equal(t, 3, st["total-routes"])
	assert.Equal(t, 1, st["filtered-routes"])

	// AC-8: show distinguishes filtered vs accepted in json, yaml, and table pipes.
	for _, pipe := range []string{"| json", "| yaml", "| table"} {
		rendered := answerOf(t, r, "show bgp adj-rib-in "+pipe)
		assert.Contains(t, rendered, "filtered")
		assert.Contains(t, rendered, "loop")
	}

	// AC-4 + cursor advance: buildReplayRoutes skips filtered entry and advances cursor past it.
	dest := netip.MustParseAddr("10.0.0.99")
	routes1, cursor1 := r.buildReplayRoutes(dest, 0, replayCut{})
	require.Len(t, routes1, 2, "only the two accepted routes may be replayed")
	for _, rt := range routes1 {
		assert.NotEqual(t, "180a0001", rt.NLRIHex, "filtered route must never appear in replay")
	}
	assert.GreaterOrEqual(t, cursor1, uint64(3), "cursor must advance past the filtered entry")

	// Incremental replay from cursor1 yields nothing (no stall).
	routes2, _ := r.buildReplayRoutes(dest, cursor1, replayCut{})
	assert.Empty(t, routes2)

	// ReplayPathCommand answers Withdraw=true for the filtered route.
	_, rawPath, err := r.replayPathCommand([]string{"10.0.0.99", peer, "ipv4/unicast", "10.0.1.0/24", "0", "11", "1"})
	require.NoError(t, err)
	page, ok := rawPath.(ribevents.ReplayPage)
	require.True(t, ok)
	require.Len(t, page.Routes, 1)
	assert.True(t, page.Routes[0].Withdraw)

	// Eligibility returns false for the filtered route and true for accepted route.
	assert.False(t, r.routeEligible(ribevents.ValidationRoute{
		Peer: netip.MustParseAddr(peer), PeerOwner: owner, ConnectionID: conn, Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.0.1.0/24"),
	}, 0))
	assert.True(t, r.routeEligible(ribevents.ValidationRoute{
		Peer: netip.MustParseAddr(peer), PeerOwner: owner, ConnectionID: conn, Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.0.0.0/24"),
	}, 0))

	// AC-6: explicit withdrawal removes the filtered entry.
	r.handleReceivedStructured(structuredFilteredEvent(t, ctxID, peer, owner, conn, 13,
		rpc.EventKindUpdate, "", ipv4FilteredBody(nil, []byte{24, 10, 0, 1})))
	st = statusMap(t, r)
	assert.Equal(t, 2, st["total-routes"])
	assert.Equal(t, 0, st["filtered-routes"])

	// Re-add via JSON fallback path (forked plugin rail) and verify peer-down clears it (AC-7).
	r.handleReceived(&bgp.Event{
		Type:          rpc.EventKindUpdateFiltered.String(),
		TypeKind:      rpc.EventKindUpdateFiltered,
		FilterStep:    "policy",
		PeerOwner:     owner,
		ConnectionID:  conn,
		Message:       &bgp.MessageInfo{Type: rpc.EventKindUpdateFiltered, ID: 14},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[family.Family]string{family.IPv4Unicast: "180a0009"},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv4Unicast: {
				{NextHop: "10.0.0.1", Action: routeaction.Add, NLRIs: []any{"10.0.9.0/24"}},
			},
		},
	})
	rawShow, err := json.Marshal(r.show("*"))
	require.NoError(t, err)
	assert.Contains(t, string(rawShow), `"filter-step":"policy"`)
	assert.Equal(t, 1, statusMap(t, r)["filtered-routes"])

	// AC-7: peer DOWN removes filtered entries along with the rest of the peer's store.
	r.handleStructuredState(&rpc.StructuredEvent{
		EventType:    rpc.EventKindState,
		PeerAddress:  peer,
		PeerOwner:    owner,
		ConnectionID: conn,
		State:        rpc.SessionStateDown,
	})
	st = statusMap(t, r)
	assert.Equal(t, 0, st["total-routes"])
	assert.Equal(t, 0, st["filtered-routes"])
}
