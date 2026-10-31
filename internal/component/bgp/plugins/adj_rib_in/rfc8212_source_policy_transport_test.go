// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC naming: untagged -- tests machinery RFC 8212 default reject relies on, not the Section 3 verdict itself, which rfc8212_default_reject_test.go and the test/plugin/rfc8212-* carriers prove.
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection
// Related: source_policy.go -- the handler these tests reach through the SDK
// Related: ../../../../test/pluginengine/pluginengine.go -- the engine side of the connection

package adj_rib_in

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The peers of the transport tests: the source whose policy changes, another
// source that stays eligible, and the replay target.
const (
	adjTransportSource = "10.0.0.1"
	adjTransportOther  = "10.0.0.2"
	adjTransportTarget = "10.0.0.3"
)

// adjTransportStateUp is the JSON state event that brings peer's session up.
func adjTransportStateUp(peer string) string {
	return fmt.Sprintf(`{"type":"state","peer":{"address":%q,"remote":{"address":%q,"as":65001}},"state":"up"}`, peer, peer)
}

// adjTransportReceived is the format=full JSON event of one received UPDATE
// from peer, admitted under policy epoch 5, announcing prefix.
func adjTransportReceived(peer string, id uint64, prefix, nlriHex string) string {
	return fmt.Sprintf(`{"type":"bgp","bgp":{"message":{"type":"update","id":%d,"direction":"received"},`+
		`"peer":{"address":%q,"remote":{"address":%q,"as":65001}},"source-policy-epoch":5,`+
		`"raw":{"attributes":"40010100","nlri":{"ipv4/unicast":%q}},`+
		`"update":{"attr":{"origin":"igp"},"ipv4/unicast":[{"next-hop":%q,"action":"add","nlri":[%q]}]}}}`,
		id, peer, peer, nlriHex, peer, prefix)
}

// startAdjRIBInOverTransport runs the real bgp-adj-rib-in entry point on
// transport and feeds it, through its callback loop, two routes of the source
// and one of the other peer, all admitted at epoch 5.
func startAdjRIBInOverTransport(t *testing.T, ctx context.Context, transport pluginengine.Transport) *pluginengine.Engine {
	t.Helper()
	engine := pluginengine.Start(t, transport, runAdjRIBInPlugin)
	for _, event := range []string{
		adjTransportStateUp(adjTransportSource),
		adjTransportStateUp(adjTransportOther),
		adjTransportStateUp(adjTransportTarget),
		adjTransportReceived(adjTransportSource, 100, "10.1.0.0/24", "180a0100"),
		adjTransportReceived(adjTransportSource, 101, "10.2.0.0/24", "180a0200"),
		adjTransportReceived(adjTransportOther, 102, "10.1.0.0/24", "180a0100"),
	} {
		require.NoError(t, engine.Deliver(ctx, event))
	}
	return engine
}

// adjTransportReplayed counts, per source peer, the routes a replay to the
// target carries, asked through the plugin's replay-routes command.
func adjTransportReplayed(t *testing.T, ctx context.Context, engine *pluginengine.Engine) map[string]int {
	t.Helper()
	document, err := engine.Execute(ctx, ribevents.ReplayRoutesCommand, []string{adjTransportTarget, "0"})
	require.NoError(t, err)
	var page ribevents.ReplayPage
	require.NoError(t, json.Unmarshal(document, &page), string(document))
	counts := make(map[string]int)
	for i := range page.Routes {
		counts[page.Routes[i].SourcePeer]++
	}
	return counts
}

// adjTransportHeld is the route count the plugin's status command reports
// for the source, which counts stored routes whatever their eligibility.
func adjTransportHeld(t *testing.T, ctx context.Context, engine *pluginengine.Engine) int {
	t.Helper()
	document, err := engine.Execute(ctx, "show bgp adj-rib-in status", nil)
	require.NoError(t, err)
	var status struct {
		Peers map[string]int `json:"peers"`
	}
	require.NoError(t, json.Unmarshal(document, &status), string(document))
	return status.Peers[adjTransportSource]
}

// adjTransportPhase sends one lifecycle request of transaction tx1 for the
// source, captured at epoch 5, and returns its receipt; the harness has
// already refused a receipt that does not echo the request's identity.
func adjTransportPhase(t *testing.T, ctx context.Context, engine *pluginengine.Engine, op rpc.SourcePolicyOperation, generation, next uint64) rpc.SourcePolicyOutput {
	t.Helper()
	input := rpc.SourcePolicyInput{TransactionID: "tx1", Operation: op,
		Source: rpc.SourcePolicyIdentity{Peer: adjTransportSource, PeerOwner: 1, SessionGeneration: generation,
			PolicyEpoch: 5, ReceiveFrontier: 9, TemplateGeneration: 1},
		NextPolicyEpoch: next, DeadlineMS: time.Now().Add(time.Minute).UnixMilli()}
	require.NoError(t, input.Validate(time.Now()))
	out, err := engine.SourcePolicy(ctx, input)
	require.NoError(t, err)
	return out
}

// TestRFC8212SourcePolicyAdjRIBInTransport drives bgp-adj-rib-in's lifecycle
// through the real SDK source-policy callback on the wire MuxConn and on
// DirectBridge.
//
// VALIDATES: the plugin's own entry point admits each request on its callback
// loop and answers a receipt echoing the request's identity; a foreign
// SessionGeneration is refused with every route still replayed; apply makes
// the source's earlier routes ineligible for replay while keeping them stored
// and leaving the other source replayed; abort restores them; finalize after
// abort is refused.
// PREVENTS: an owner reached only by a direct call, a denied route replayed
// to a peer, and a rollback that loses the source's routes.
func TestRFC8212SourcePolicyAdjRIBInTransport(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			engine := startAdjRIBInOverTransport(t, ctx, transport)
			all := map[string]int{adjTransportSource: 2, adjTransportOther: 1}
			require.Equal(t, all, adjTransportReplayed(t, ctx, engine))

			out := adjTransportPhase(t, ctx, engine, rpc.SourcePolicyPrepare, 3, 0)
			require.Equal(t, rpc.StatusOK, out.Status, out.Error)

			out = adjTransportPhase(t, ctx, engine, rpc.SourcePolicyApply, 4, 6)
			require.Equal(t, rpc.StatusError, out.Status, "a replacement session's token is refused")
			require.Equal(t, all, adjTransportReplayed(t, ctx, engine), "the refusal changed no eligibility")

			out = adjTransportPhase(t, ctx, engine, rpc.SourcePolicyApply, 3, 6)
			require.Equal(t, rpc.StatusOK, out.Status, out.Error)
			require.Equal(t, map[string]int{adjTransportOther: 1}, adjTransportReplayed(t, ctx, engine),
				"the source's earlier routes are not replayed; the other source's are")
			require.Equal(t, 2, adjTransportHeld(t, ctx, engine), "apply keeps the routes for abort")

			out = adjTransportPhase(t, ctx, engine, rpc.SourcePolicyAbort, 3, 7)
			require.Equal(t, rpc.StatusOK, out.Status, out.Error)
			require.Equal(t, all, adjTransportReplayed(t, ctx, engine), "abort restores the source's routes")

			out = adjTransportPhase(t, ctx, engine, rpc.SourcePolicyFinalize, 3, 0)
			require.Equal(t, rpc.StatusError, out.Status, "finalize after abort is refused")
			require.Contains(t, out.Error, "was not applied")
			require.Equal(t, all, adjTransportReplayed(t, ctx, engine), "the refused finalize removed nothing")
		})
	}
}

// TestRFC8212SourcePolicyAdjRIBInTransportFinalize proves, on both
// transports, that finalize after apply releases the denied routes and that a
// later abort cannot undo it.
//
// VALIDATES: the source's stored routes drop to none at finalize, the other
// source stays replayed, and a stale abort is refused with nothing restored.
// PREVENTS: an accepted denial that keeps denied state for a later recovery.
func TestRFC8212SourcePolicyAdjRIBInTransportFinalize(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			engine := startAdjRIBInOverTransport(t, ctx, transport)
			out := adjTransportPhase(t, ctx, engine, rpc.SourcePolicyPrepare, 3, 0)
			require.Equal(t, rpc.StatusOK, out.Status, out.Error)
			out = adjTransportPhase(t, ctx, engine, rpc.SourcePolicyApply, 3, 6)
			require.Equal(t, rpc.StatusOK, out.Status, out.Error)
			require.Equal(t, 2, adjTransportHeld(t, ctx, engine))

			out = adjTransportPhase(t, ctx, engine, rpc.SourcePolicyFinalize, 3, 0)
			require.Equal(t, rpc.StatusOK, out.Status, out.Error)
			require.Equal(t, 0, adjTransportHeld(t, ctx, engine), "denied routes are released, not kept for a later recovery")
			require.Equal(t, map[string]int{adjTransportOther: 1}, adjTransportReplayed(t, ctx, engine))

			out = adjTransportPhase(t, ctx, engine, rpc.SourcePolicyAbort, 3, 7)
			require.Equal(t, rpc.StatusError, out.Status, "a stale abort cannot undo a finalized denial")
			require.Equal(t, 0, adjTransportHeld(t, ctx, engine))
		})
	}
}
