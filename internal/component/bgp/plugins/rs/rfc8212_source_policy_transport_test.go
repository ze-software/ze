// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC naming: untagged -- tests machinery RFC 8212 default reject relies on, not the Section 3 verdict itself, which rfc8212_default_reject_test.go and the test/plugin/rfc8212-* carriers prove.
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection
// Related: source_policy.go -- the handler these tests reach through the SDK
// Related: ../../../../test/pluginengine/pluginengine.go -- the engine side of the connection

package rs

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// forwardedID matches message 102 in an engine RPC's ID list.
var forwardedID = regexp.MustCompile(`[\[,]102[\],]`)

// rsTransportReceived is the text event of one received UPDATE from the
// source, admitted under policy epoch, announcing prefix.
func rsTransportReceived(id, epoch uint64, prefix string) string {
	return fmt.Sprintf("peer %s remote as 65001 received update %d source-policy-epoch %d "+
		"origin igp next-hop 192.0.2.1 nlri ipv4/unicast add prefix %s", policySource, id, epoch, prefix)
}

// rsTransportPhase sends one lifecycle request of transaction tx1 for the
// source, captured at epoch 5, and returns its receipt.
func rsTransportPhase(t *testing.T, ctx context.Context, engine *pluginengine.Engine, op rpc.SourcePolicyOperation, next uint64) rpc.SourcePolicyOutput {
	t.Helper()
	input := rpc.SourcePolicyInput{TransactionID: "tx1", Operation: op,
		Source: rpc.SourcePolicyIdentity{Peer: policySource, PeerOwner: 1, SessionGeneration: 3,
			PolicyEpoch: 5, ReceiveFrontier: 9, TemplateGeneration: 1},
		NextPolicyEpoch: next, DeadlineMS: time.Now().Add(time.Minute).UnixMilli()}
	require.NoError(t, input.Validate(time.Now()))
	out, err := engine.SourcePolicy(ctx, input)
	require.NoError(t, err)
	return out
}

// rsTransportDeparture waits for the source's departure and returns its
// commands joined: the departure is sent on the route server's own workers.
func rsTransportDeparture(t *testing.T, engine *pluginengine.Engine) string {
	t.Helper()
	var departure string
	require.Eventually(t, func() bool {
		var commands []string
		for _, update := range engine.RouteUpdates(t) {
			if source, _ := update.Meta["recovery-source"].(string); source == policySource {
				commands = append(commands, update.Command)
			}
		}
		departure = strings.Join(commands, "\n")
		return departure != ""
	}, 10*time.Second, 10*time.Millisecond)
	return departure
}

// TestRFC8212SourcePolicyRSTransport drives bgp-rs's lifecycle through the
// real RunRouteServer entry point and the SDK source-policy callback on
// DirectBridge. bgp-rs refuses to start on the wire transport (RunRouteServer,
// TestRefusesExternalProcess), so DirectBridge is its only transport.
//
// VALIDATES: the plugin's own entry point admits each request and answers a
// receipt; finalize after apply drops the source's denied routes, so the
// departure of its session owes only the route admitted after the change; an
// abort after apply keeps them, so the departure owes both.
// PREVENTS: an owner reached only by a direct call, and a route server that
// keeps or loses denied routes against the transaction's outcome.
func TestRFC8212SourcePolicyRSTransport(t *testing.T) {
	for _, finalize := range []bool{true, false} {
		name := "abort"
		if finalize {
			name = "finalize"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			engine := pluginengine.Start(t, pluginengine.TransportBridge, RunRouteServer)
			for _, event := range []string{
				"peer " + policySource + " remote as 65001 state up",
				rsTransportReceived(100, 5, "10.1.0.0/24"),
				rsTransportReceived(101, 5, "10.2.0.0/24"),
			} {
				require.NoError(t, engine.Deliver(ctx, event))
			}

			out := rsTransportPhase(t, ctx, engine, rpc.SourcePolicyPrepare, 0)
			require.Equal(t, rpc.StatusOK, out.Status, out.Error)
			out = rsTransportPhase(t, ctx, engine, rpc.SourcePolicyApply, 6)
			require.Equal(t, rpc.StatusOK, out.Status, out.Error)
			if finalize {
				out = rsTransportPhase(t, ctx, engine, rpc.SourcePolicyFinalize, 0)
			} else {
				out = rsTransportPhase(t, ctx, engine, rpc.SourcePolicyAbort, 7)
			}
			require.Equal(t, rpc.StatusOK, out.Status, out.Error)

			require.NoError(t, engine.Deliver(ctx, rsTransportReceived(102, 7, "10.3.0.0/24")))
			// A forward still queued when the down arrives is dropped
			// (processForward), so the down waits for this one to reach the engine.
			require.Eventually(t, func() bool {
				for _, call := range engine.RPCs() {
					if forwardedID.Match(call.Params) {
						return true
					}
				}
				return false
			}, 10*time.Second, 10*time.Millisecond)
			require.NoError(t, engine.Deliver(ctx, "peer "+policySource+" remote as 65001 state down"))
			departure := rsTransportDeparture(t, engine)
			require.Contains(t, departure, "10.3.0.0/24", "the route admitted after the change departs")
			if finalize {
				require.NotContains(t, departure, "10.1.0.0/24", "a finalized denial owes no departure")
				require.NotContains(t, departure, "10.2.0.0/24", "a finalized denial owes no departure")
				return
			}
			require.Contains(t, departure, "10.1.0.0/24", "an aborted denial keeps the route")
			require.Contains(t, departure, "10.2.0.0/24", "an aborted denial keeps the route")
		})
	}
}
