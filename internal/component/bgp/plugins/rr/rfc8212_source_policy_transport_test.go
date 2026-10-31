// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC naming: untagged -- tests machinery RFC 8212 default reject relies on, not the Section 3 verdict itself, which rfc8212_default_reject_test.go and the test/plugin/rfc8212-* carriers prove.
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection
// Related: source_policy.go -- the handler these tests reach through the SDK
// Related: ../../../../test/pluginengine/pluginengine.go -- the engine side of the connection

package rr

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// rrTransportSource is the source whose import policy the transport test
// withdraws. The owner sends nothing downstream for a phase, so the fake
// engine, which answers only update-route, needs no store.
const rrTransportSource = "10.0.0.1"

// runRouteReflectorCallbacks runs bgp-rr's real callback registration on conn
// without runRouteReflector's in-process check, which refuses the wire
// transport by design (retention.Family is a same-process call).
func runRouteReflectorCallbacks(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-rr", conn)
	defer func() { _ = p.Close() }()
	return serveRouteReflector(p)
}

// rrTransportPhase sends one lifecycle request of transaction tx for the
// source, captured at epoch 5, and returns its receipt; the harness has
// already refused a receipt that does not echo the request's identity.
func rrTransportPhase(t *testing.T, ctx context.Context, engine *pluginengine.Engine, tx string, op rpc.SourcePolicyOperation, next uint64) rpc.SourcePolicyOutput {
	t.Helper()
	input := rpc.SourcePolicyInput{TransactionID: tx, Operation: op,
		Source: rpc.SourcePolicyIdentity{Peer: rrTransportSource, PeerOwner: 1, SessionGeneration: 3,
			PolicyEpoch: 5, ReceiveFrontier: 9, TemplateGeneration: 1},
		NextPolicyEpoch: next, DeadlineMS: time.Now().Add(time.Minute).UnixMilli()}
	if err := input.Validate(time.Now()); err != nil {
		t.Fatal(err)
	}
	out, err := engine.SourcePolicy(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestRFC8212RouteReflectorSourcePolicyTransport drives bgp-rr's lifecycle
// through the real SDK source-policy callback on the wire MuxConn and on
// DirectBridge.
//
// VALIDATES: the handler serveRouteReflector registers is reached on both
// transports and answers receipts echoing each request: a prepare of a
// source whose session is not up is refused; once the source is up and has
// announced a route through the text event path, prepare, apply and finalize
// succeed; a second transaction prepared and aborted with target zero
// succeeds, and its finalize is then refused.
// PREVENTS: a declared owner whose handler is reachable only by a direct
// call, which the engine's preflight would admit and then fail.
func TestRFC8212RouteReflectorSourcePolicyTransport(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			engine := pluginengine.Start(t, transport, runRouteReflectorCallbacks)

			out := rrTransportPhase(t, ctx, engine, "tx0", rpc.SourcePolicyPrepare, 0)
			if out.Status != rpc.StatusError {
				t.Fatal("prepare of a source with no session was accepted")
			}

			for _, event := range []string{
				"peer " + rrTransportSource + " remote as 65001 state up\n",
				"peer " + rrTransportSource + " remote as 65001 received update 100 source-policy-epoch 5 " +
					"origin igp next " + rrTransportSource + " nlri ipv4/unicast add 10.1.0.0/24\n",
			} {
				if err := engine.Deliver(ctx, event); err != nil {
					t.Fatal(err)
				}
			}
			for _, step := range []struct {
				op   rpc.SourcePolicyOperation
				next uint64
			}{{rpc.SourcePolicyPrepare, 0}, {rpc.SourcePolicyApply, 6}, {rpc.SourcePolicyFinalize, 0}} {
				out = rrTransportPhase(t, ctx, engine, "tx1", step.op, step.next)
				if out.Status != rpc.StatusOK {
					t.Fatalf("tx1 operation %d: %s", step.op, out.Error)
				}
			}
			out = rrTransportPhase(t, ctx, engine, "tx2", rpc.SourcePolicyPrepare, 0)
			if out.Status != rpc.StatusOK {
				t.Fatal(out.Error)
			}
			out = rrTransportPhase(t, ctx, engine, "tx2", rpc.SourcePolicyAbort, 0)
			if out.Status != rpc.StatusOK {
				t.Fatal(out.Error)
			}
			out = rrTransportPhase(t, ctx, engine, "tx2", rpc.SourcePolicyFinalize, 0)
			if out.Status != rpc.StatusError {
				t.Fatal("finalize after abort was accepted")
			}
		})
	}
}
