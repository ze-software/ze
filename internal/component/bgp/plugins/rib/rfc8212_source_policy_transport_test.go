// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection
// Related: source_policy.go -- the handler these tests reach through the SDK
// Related: ../../../../test/pluginengine/pluginengine.go -- the engine side of the connection

package rib

import (
	"context"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The peers of the transport tests: a source and an alternative that both
// announce 10.1.0.0/24, and a destination holding the source's forwarded copy.
const (
	transportSource      = "10.0.0.1"
	transportAlternative = "10.0.0.2"
	transportDestination = "10.0.0.3"
	transportShared      = "10.1.0.0/24"
	transportOnly        = "10.2.0.0/24"
)

// transportStateUp is the JSON state event that brings peer's session up.
func transportStateUp(peer string, as uint32) string {
	return fmt.Sprintf(`{"type":"state","peer":{"address":%q,"remote":{"address":%q,"as":%d}},"state":"up"}`, peer, peer, as)
}

// transportReceived is the format=full JSON event of one received UPDATE
// from peer, admitted under policy epoch, announcing prefix.
func transportReceived(peer string, id, epoch uint64, prefix, nlriHex string) string {
	return fmt.Sprintf(`{"type":"bgp","bgp":{"message":{"type":"update","id":%d,"direction":"received"},`+
		`"peer":{"address":%q,"remote":{"address":%q,"as":65001}},"source-policy-epoch":%d,`+
		`"raw":{"attributes":"40010100","nlri":{"ipv4/unicast":%q}},`+
		`"update":{"attr":{"origin":"igp"},"ipv4/unicast":[{"next-hop":%q,"action":"add","nlri":[%q]}]}}}`,
		id, peer, peer, epoch, nlriHex, peer, prefix)
}

// transportSent is the JSON sent event of the source's message 100 forwarded
// to the destination, which fills the sent inventory a denial withdraws.
func transportSent() string {
	return fmt.Sprintf(`{"type":"sent","msg-id":900,"source-policy-epoch":5,`+
		`"peer":{"address":%q,"remote":{"address":%q,"as":65003}},`+
		`"route-meta":{"source-peer":%q,"source-message-id":100},"origin":"igp",`+
		`"ipv4/unicast":[{"next-hop":%q,"action":"add","nlri":[%q]}]}`,
		transportDestination, transportDestination, transportSource, transportSource, transportShared)
}

// startRIBOverTransport runs the real bgp-rib entry point on transport and
// feeds it, through its callback loop, the sessions, the received UPDATEs
// (all at epoch 5) and the forwarded copy. It returns the engine and the
// running RIBManager.
func startRIBOverTransport(t *testing.T, ctx context.Context, transport pluginengine.Transport) (*pluginengine.Engine, *RIBManager) {
	t.Helper()
	engine := pluginengine.Start(t, transport, runRIBPlugin)
	r := activeManager.Load()
	require.NotNil(t, r, "runRIBPlugin publishes its manager before startup")
	for _, event := range []string{
		transportStateUp(transportSource, 65001),
		transportStateUp(transportAlternative, 65002),
		transportStateUp(transportDestination, 65003),
		transportReceived(transportSource, 100, 5, transportShared, "180a0100"),
		transportReceived(transportSource, 101, 5, transportOnly, "180a0200"),
		transportReceived(transportAlternative, 102, 5, transportShared, "180a0100"),
		transportSent(),
	} {
		require.NoError(t, engine.Deliver(ctx, event))
	}
	return engine, r
}

// transportCandidates counts the source's candidates for prefix in r.
func transportCandidates(t *testing.T, r *RIBManager, prefix netip.Prefix) int {
	t.Helper()
	nlri := append([]byte{byte(prefix.Bits())}, prefix.Addr().AsSlice()[:(prefix.Bits()+7)/8]...)
	source := netip.MustParseAddr(transportSource)
	count := 0
	for _, c := range gatherCandidatesHeld(t, r, family.IPv4Unicast, nlri, false) {
		if c.PeerIP == source {
			count++
		}
	}
	return count
}

// transportRequest is one lifecycle request of transaction tx1 for the
// source captured at epoch 5, as the coordinator sends it.
func transportRequest(t *testing.T, op rpc.SourcePolicyOperation, next uint64) rpc.SourcePolicyInput {
	t.Helper()
	return sourcePolicyRequest(t, op, "tx1", netip.MustParseAddr(transportSource), 5, next)
}

// transportPhase sends input and returns its receipt; the harness has
// already refused a receipt that does not echo the request's identity.
func transportPhase(t *testing.T, ctx context.Context, engine *pluginengine.Engine, input rpc.SourcePolicyInput) rpc.SourcePolicyOutput {
	t.Helper()
	out, err := engine.SourcePolicy(ctx, input)
	require.NoError(t, err)
	return out
}

// destinationUpdates is the update-route RPCs for the destination after the
// first skip.
func destinationUpdates(t *testing.T, engine *pluginengine.Engine, skip int) []rpc.UpdateRouteInput {
	t.Helper()
	var updates []rpc.UpdateRouteInput
	for _, update := range engine.RouteUpdates(t)[skip:] {
		if update.PeerSelector == transportDestination {
			updates = append(updates, update)
		}
	}
	return updates
}

// TestRFC8212SourcePolicyRIBTransport drives bgp-rib's lifecycle through the
// real SDK source-policy callback on the wire MuxConn and on DirectBridge.
//
// VALIDATES: the plugin's own entry point admits each request on its callback
// loop and answers a receipt echoing the request's identity; a foreign
// SessionGeneration is refused with the source still eligible and nothing
// written; apply makes the source's earlier paths ineligible and its forwarded
// copy's withdrawal reaches the engine before the receipt; abort restores the
// paths and re-announces the copy under NextPolicyEpoch before its receipt;
// finalize after abort is refused.
// PREVENTS: an owner reached only by a direct call, a receipt sent before the
// destination settled, and a rollback that leaves the destination without
// the route.
func TestRFC8212SourcePolicyRIBTransport(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			engine, r := startRIBOverTransport(t, ctx, transport)
			shared, only := netip.MustParsePrefix(transportShared), netip.MustParsePrefix(transportOnly)
			require.Equal(t, 1, transportCandidates(t, r, shared))
			require.Equal(t, 1, transportCandidates(t, r, only))

			out := transportPhase(t, ctx, engine, transportRequest(t, rpc.SourcePolicyPrepare, 0))
			require.Equal(t, rpc.StatusOK, out.Status, out.Error)

			written := len(engine.RouteUpdates(t))
			foreign := transportRequest(t, rpc.SourcePolicyApply, 6)
			foreign.Source.SessionGeneration = 4
			out = transportPhase(t, ctx, engine, foreign)
			require.Equal(t, rpc.StatusError, out.Status, "a replacement session's token is refused")
			require.Equal(t, 1, transportCandidates(t, r, shared), "the refusal changed no eligibility")
			require.Len(t, engine.RouteUpdates(t), written, "the refusal wrote nothing")

			out = transportPhase(t, ctx, engine, transportRequest(t, rpc.SourcePolicyApply, 6))
			require.Equal(t, rpc.StatusOK, out.Status, out.Error)
			require.Equal(t, 0, transportCandidates(t, r, shared), "a denied path is no candidate")
			require.Equal(t, 0, transportCandidates(t, r, only))
			withdrawn := destinationUpdates(t, engine, written)
			require.Len(t, withdrawn, 1, "the forwarded copy's withdrawal reached the engine before the receipt")
			require.Contains(t, withdrawn[0].Command, transportShared)
			require.Equal(t, true, withdrawn[0].Meta["rib-lifecycle"], "the withdrawal is RIB lifecycle feedback")

			written = len(engine.RouteUpdates(t))
			out = transportPhase(t, ctx, engine, transportRequest(t, rpc.SourcePolicyAbort, 7))
			require.Equal(t, rpc.StatusOK, out.Status, out.Error)
			require.Equal(t, 1, transportCandidates(t, r, shared), "abort restores the path")
			require.Equal(t, 1, transportCandidates(t, r, only))
			announced := destinationUpdates(t, engine, written)
			require.Len(t, announced, 1, "the copy's re-announcement reached the engine before the receipt")
			require.Contains(t, announced[0].Command, transportShared)
			require.NotEqual(t, withdrawn[0].Command, announced[0].Command, "abort announces, it does not withdraw again")
			require.Equal(t, uint64(7), announced[0].SourcePolicyEpoch, "the copy is announced under NextPolicyEpoch")

			out = transportPhase(t, ctx, engine, transportRequest(t, rpc.SourcePolicyFinalize, 0))
			require.Equal(t, rpc.StatusError, out.Status, "finalize after abort is refused")
			require.Contains(t, out.Error, "was not applied")
			require.Equal(t, 1, transportCandidates(t, r, shared), "the refused finalize removed nothing")
		})
	}
}

// TestRFC8212SourcePolicyRIBTransportFinalize proves, on both transports, that
// finalize after apply releases the denied paths and writes nothing more.
//
// VALIDATES: the source's paths leave its Adj-RIB-In at finalize while the
// alternative stays, and no destination write follows the apply's withdrawal.
// PREVENTS: an accepted denial that keeps denied state for a later recovery.
func TestRFC8212SourcePolicyRIBTransportFinalize(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			engine, r := startRIBOverTransport(t, ctx, transport)
			for _, input := range []rpc.SourcePolicyInput{
				transportRequest(t, rpc.SourcePolicyPrepare, 0),
				transportRequest(t, rpc.SourcePolicyApply, 6),
			} {
				out := transportPhase(t, ctx, engine, input)
				require.Equal(t, rpc.StatusOK, out.Status, out.Error)
			}
			written := len(engine.RouteUpdates(t))
			out := transportPhase(t, ctx, engine, transportRequest(t, rpc.SourcePolicyFinalize, 0))
			require.Equal(t, rpc.StatusOK, out.Status, out.Error)

			r.peerMu.RLock()
			sourceHeld := r.bgpPeers[netip.MustParseAddr(transportSource)].Len()
			alternativeHeld := r.bgpPeers[netip.MustParseAddr(transportAlternative)].Len()
			released := r.sourcePolicy[netip.MustParseAddr(transportSource)].sent == nil
			r.peerMu.RUnlock()
			require.Equal(t, 0, sourceHeld, "denied paths are released, not kept for a later recovery")
			require.Equal(t, 1, alternativeHeld, "the alternative is untouched")
			require.True(t, released, "finalize releases the withdrawn copies")
			require.Empty(t, destinationUpdates(t, engine, written), "finalize writes nothing more")
		})
	}
}
