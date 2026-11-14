// Design: docs/architecture/rib/unified-locrib.md -- publication order and acknowledged forwarding output.
// Related: route_view_worker_output.go -- publish, which refuses a publication orderForwarding cannot order.
// Related: route_view_output.go -- publish, which refuses output without the daemon's event bus.

package sysrib

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// requireRouteViewRefusal requires err to be the operator refusal naming
// problem, with fix as its remedy.
func requireRouteViewRefusal(t *testing.T, err error, problem, fix string) {
	t.Helper()
	refusal, isOperator := errors.AsType[*opererr.Error](err)
	if !isOperator {
		t.Fatalf("error = %v (%T), want the operator refusal %q", err, err, problem)
	}
	if refusal.Problem() != problem {
		t.Fatalf("refusal problem = %q, want %q", refusal.Problem(), problem)
	}
	if refusal.Fix() != fix {
		t.Fatalf("refusal fix = %q, want %q", refusal.Fix(), fix)
	}
}

// Goal: a publication orderForwarding refuses reaches no FIB writer. publish
// returns the refusal before its first batch, so the daemon receives nothing
// and the worker's output identity is not consumed; the worker's run loop
// then fails with that error like any other publication error.
//
// Method: this tests publish's contract, not a reachable daemon state. Every
// producer carries each prefix once (applySelectedBatch and cascadeChanges
// dedupe their keys, the pending set and replayChanges are keyed by prefix),
// so no worker input yields the cycle; the cyclic map is handed to publish
// directly, on a worker built over the runtime feed's real JSON transport.
// The publication carries one prefix twice, first with a gateway inside its
// own prefix and then device-only: the device route must precede the gateway
// route (it provides the gateway), and the second entry for a prefix must
// follow the first, so no order exists. The control publishes the device
// route alone through the same worker and must reach the feed as operation 1,
// which also proves the refusal consumed no operation.
func TestRouteViewWorkerPublishRefusesUnorderablePublication(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	feed := newRouteViewRuntimeFeed(t, ctx)
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
	worker := &routeViewWorker{client: client, ctx: ctx}

	prefix := netip.MustParsePrefix("198.18.231.0/24")
	gatewayed := outgoingChange{Action: routeaction.Add, Prefix: prefix, Protocol: "static", NextHop: netip.MustParseAddr("198.18.231.9")}
	device := outgoingChange{Action: routeaction.Add, Prefix: prefix, Protocol: "static", Interface: "port1"}
	err := worker.publish(ctx, map[family.Family][]outgoingChange{family.IPv4Unicast: {gatewayed, device}}, 0)
	requireRouteViewRefusal(t, err, "A forwarding publication has no installable order", "ze request reload")
	if worker.outputOp != 0 {
		t.Fatalf("the refused publication consumed output operation %d, want none", worker.outputOp)
	}
	select {
	case batch := <-feed.published:
		t.Fatalf("the refused publication reached the daemon: %+v", batch)
	default:
	}

	// The feed refuses a publication before it served a complete snapshot;
	// the control publishes as a synchronized worker would.
	feed.complete = true
	if err := worker.publish(ctx, map[family.Family][]outgoingChange{family.IPv4Unicast: {device}}, 0); err != nil {
		t.Fatalf("control publication failed: %v", err)
	}
	select {
	case batch := <-feed.published:
		if len(batch.Changes) != 1 {
			t.Fatalf("control publication carried %d changes, want the device route alone: %+v", len(batch.Changes), batch)
		}
		if batch.Changes[0].Prefix != prefix {
			t.Fatalf("control publication carried %s, want %s", batch.Changes[0].Prefix, prefix)
		}
		if batch.Changes[0].Interface != "port1" {
			t.Fatalf("control publication carried device %q, want port1", batch.Changes[0].Interface)
		}
	case <-ctx.Done():
		t.Fatal("control publication never reached the daemon")
	}
	if worker.outputOp != 1 {
		t.Fatalf("control publication used output operation %d, want 1", worker.outputOp)
	}
}

// Goal: a publication with no daemon event bus is refused with the operator
// error that names the bus, and the refusal leaves the session as it was: no
// output ledger is created, no cleanup fence is registered, and the operation
// identity is not consumed, so the same operation published with a bus
// afterwards is acknowledged and delivered.
//
// Method: open a view, read its first page, publish operation 1 with a nil
// bus and require the refusal and the absence of session output, then publish
// the same operation with a bus and require its acknowledgment, exactly one
// delivered event, and a ledger holding exactly the published prefix.
func TestRouteViewOutputRefusesWithoutEventBus(t *testing.T) {
	redistevents.RegisterProtocol("static")
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	input := routeViewPublication{Session: opened.Session, Cursor: page.Cursor, Operation: 1, Batch: routeViewOutputBatch()}

	_, err := authority.publish(context.Background(), owner, input, nil)
	requireRouteViewRefusal(t, err, "The daemon forwarding event bus is unavailable", "ze request plugin restart "+owner.Name())
	authority.mu.Lock()
	session := authority.active
	authority.mu.Unlock()
	if session == nil {
		t.Fatal("the refusal closed the view")
	}
	session.requestMu.Lock()
	output := session.output
	session.requestMu.Unlock()
	if output != nil {
		t.Fatalf("the refused publication created session output: ledger %v", output.ledger)
	}

	bus := newTestEventBus()
	ack, err := authority.publish(context.Background(), owner, input, bus)
	if err != nil {
		t.Fatalf("publication with a bus after the refusal failed: %v", err)
	}
	if ack != (rpc.RouteViewPublishOutput{Session: opened.Session, Cursor: page.Cursor, Operation: 1}) {
		t.Fatalf("acknowledgment = %+v, want operation 1 at cursor %d", ack, page.Cursor)
	}
	if len(bus.events) != 1 {
		t.Fatalf("bus received %d events, want the one publication", len(bus.events))
	}
	session.requestMu.Lock()
	defer session.requestMu.Unlock()
	if session.output == nil {
		t.Fatal("the delivered publication created no output ledger")
	}
	if len(session.output.ledger) != 1 {
		t.Fatalf("ledger holds %d prefixes, want the one published", len(session.output.ledger))
	}
	key := prefixKey{family: family.IPv4Unicast, prefix: input.Batch.Changes[0].Prefix}
	if _, owned := session.output.ledger[key]; !owned {
		t.Fatalf("ledger %v does not own the published prefix %s", session.output.ledger, key.prefix)
	}
}
