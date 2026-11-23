// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 7947 Section 2.1 import-filtered Adj-RIB-In entries
// RFC: rfc/short/rfc7947.md -- Section 2.1
// Related: rib.go, rib_commands.go, rib_eligibility.go, rib_replay_path.go
// RFC naming: untagged -- unit tests of Adj-RIB-In filtered lifecycle, views, replay skip, and Prometheus gauge; RFC7947-2.1-1 conformance tags live in reactor/rfc7947_filtered_conformance_test.go.
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
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func ipv4FilteredBody(nlri, withdrawn []byte) []byte {
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

//nolint:unparam // peer parameter kept explicit so tests can add multi-peer cases
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

type fakeGauge struct {
	val float64
}

func (g *fakeGauge) Set(v float64) { g.val = v }
func (g *fakeGauge) Inc()          { g.val++ }
func (g *fakeGauge) Dec()          { g.val-- }
func (g *fakeGauge) Add(v float64) { g.val += v }

type fakeGaugeVec struct {
	values map[string]*fakeGauge
}

func (v *fakeGaugeVec) With(labelValues ...string) metrics.Gauge {
	key := ""
	if len(labelValues) > 0 {
		key = labelValues[0]
	}
	if v.values == nil {
		v.values = make(map[string]*fakeGauge)
	}
	g, ok := v.values[key]
	if !ok {
		g = &fakeGauge{}
		v.values[key] = g
	}
	return g
}

func (v *fakeGaugeVec) Delete(labelValues ...string) bool {
	if len(labelValues) == 0 || v.values == nil {
		return false
	}
	_, ok := v.values[labelValues[0]]
	delete(v.values, labelValues[0])
	return ok
}

type fakeMetricsRegistry struct {
	metrics.NopRegistry
	gauges map[string]*fakeGaugeVec
}

func (r *fakeMetricsRegistry) GaugeVec(name, _ string, _ []string) metrics.GaugeVec {
	if r.gauges == nil {
		r.gauges = make(map[string]*fakeGaugeVec)
	}
	v := &fakeGaugeVec{values: make(map[string]*fakeGauge)}
	r.gauges[name] = v
	return v
}

// TestAdjRIBInFilteredRoutesMetric verifies that ze_adjribin_routes_filtered
// tracks import-filtered route inserts, replacements, withdrawals, and peer down.
//
// VALIDATES: Prometheus filtered-route gauge in spec-rfc7947-adj-rib-in-accepts-filtered-updates.
func TestAdjRIBInFilteredRoutesMetric(t *testing.T) {
	reg := &fakeMetricsRegistry{}
	SetMetricsRegistry(reg)
	t.Cleanup(func() { SetMetricsRegistry(nil) })

	r := newTestManager(t)
	const peer = "10.0.0.1"
	const owner uint64 = 21
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

	gaugeVal := func() float64 {
		gv := reg.gauges[metricRoutesFiltered]
		require.NotNil(t, gv)
		g := gv.values[peer]
		if g == nil {
			return 0
		}
		return g.val
	}

	// Filtered insert -> 1.
	r.handleReceivedStructured(structuredFilteredEvent(t, ctxID, peer, owner, conn, 1,
		rpc.EventKindUpdateFiltered, "loop", ipv4FilteredBody([]byte{24, 10, 0, 1}, nil)))
	assert.Equal(t, float64(1), gaugeVal())

	// Accepted insert -> still 1.
	r.handleReceivedStructured(structuredFilteredEvent(t, ctxID, peer, owner, conn, 2,
		rpc.EventKindUpdate, "", ipv4FilteredBody([]byte{24, 10, 0, 2}, nil)))
	assert.Equal(t, float64(1), gaugeVal())

	// Replace accepted with filtered -> 2.
	r.handleReceivedStructured(structuredFilteredEvent(t, ctxID, peer, owner, conn, 3,
		rpc.EventKindUpdateFiltered, "peer-policy-chain", ipv4FilteredBody([]byte{24, 10, 0, 2}, nil)))
	assert.Equal(t, float64(2), gaugeVal())

	// Replace filtered with accepted -> 1.
	r.handleReceivedStructured(structuredFilteredEvent(t, ctxID, peer, owner, conn, 4,
		rpc.EventKindUpdate, "", ipv4FilteredBody([]byte{24, 10, 0, 2}, nil)))
	assert.Equal(t, float64(1), gaugeVal())

	// Explicit withdrawal of filtered route -> 0.
	r.handleReceivedStructured(structuredFilteredEvent(t, ctxID, peer, owner, conn, 5,
		rpc.EventKindUpdate, "", ipv4FilteredBody(nil, []byte{24, 10, 0, 1})))
	assert.Equal(t, float64(0), gaugeVal())

	// Re-insert filtered route then peer DOWN -> 0.
	r.handleReceivedStructured(structuredFilteredEvent(t, ctxID, peer, owner, conn, 6,
		rpc.EventKindUpdateFiltered, "loop", ipv4FilteredBody([]byte{24, 10, 0, 3}, nil)))
	assert.Equal(t, float64(1), gaugeVal())

	r.handleStructuredState(&rpc.StructuredEvent{
		EventType:    rpc.EventKindState,
		PeerAddress:  peer,
		PeerOwner:    owner,
		ConnectionID: conn,
		State:        rpc.SessionStateDown,
	})
	assert.Equal(t, float64(0), gaugeVal())
}

// TestAdjRIBInSuspendForRevalidationKeepsImportFiltered proves that RPKI
// revalidation refresh (`enable-validation refresh`) never suspends
// ImportFiltered routes into r.pending, never exposes them in the RPKI
// snapshot, and preserves ze_adjribin_routes_filtered (RFC 7947 Section 2.1).
func TestAdjRIBInSuspendForRevalidationKeepsImportFiltered(t *testing.T) {
	reg := &fakeMetricsRegistry{}
	SetMetricsRegistry(reg)
	t.Cleanup(func() { SetMetricsRegistry(nil) })

	r := newTestManager(t)
	const peer = "10.0.0.1"
	const owner uint64 = 33
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

	// Install one accepted route and one import-filtered route.
	r.handleReceivedStructured(structuredFilteredEvent(t, ctxID, peer, owner, conn, 1,
		rpc.EventKindUpdate, "", ipv4FilteredBody([]byte{24, 10, 0, 1}, nil)))
	r.handleReceivedStructured(structuredFilteredEvent(t, ctxID, peer, owner, conn, 2,
		rpc.EventKindUpdateFiltered, "peer-policy-chain", ipv4FilteredBody([]byte{24, 10, 0, 2}, nil)))

	gv := reg.gauges[metricRoutesFiltered]
	require.NotNil(t, gv)
	require.Equal(t, float64(1), gv.values[peer].val)

	// Trigger enable-validation refresh (calls suspendForRevalidation).
	status, data, cmdErr := r.enableValidationCommand([]string{"refresh"})
	require.NoError(t, cmdErr)
	require.Equal(t, statusDone, status)

	snap, ok := data.(map[string]any)
	require.True(t, ok)
	routes, ok := snap["routes"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, routes, 1, "only the accepted route should be suspended for RPKI revalidation")
	assert.Equal(t, "10.0.1.0/24", routes[0]["prefix"])

	// The import-filtered route must remain in r.ribIn, absent from r.pending,
	// and keep ze_adjribin_routes_filtered == 1.
	r.mu.RLock()
	pendingCount := len(r.pending)
	r.mu.RUnlock()
	assert.Equal(t, 1, pendingCount, "only accepted route moves to pending")
	assert.Equal(t, float64(1), gv.values[peer].val, "filtered gauge must stay 1 across RPKI refresh")

	st, ok := r.status().(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, st["total-routes"], "import-filtered route stays visible in Adj-RIB-In")
	assert.Equal(t, 1, st["filtered-routes"])
}
