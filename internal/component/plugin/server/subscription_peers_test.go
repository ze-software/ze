// Design: docs/architecture/api/process-protocol.md -- subscription peer selection.
// Related: startup_subscription_test.go -- real deliver-batch transport probe.

package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSubscriptionPeerSelectorsDelivery verifies every selector reaches the real
// registration, grant intersection and deliver-batch wire. Exact selection and
// configured grants independently exclude peers, and overlapping selections
// deliver one copy. No subscription is inserted by the test itself.
func TestSubscriptionPeerSelectorsDelivery(t *testing.T) {
	for _, tc := range []struct {
		name   string
		peers  []string
		events []string
		want   [4]bool
	}{
		{"multiple", []string{"192.0.2.2", "192.0.2.3", "192.0.2.5"}, []string{"update direction received"}, [4]bool{true, true, false, false}},
		{"empty", nil, []string{"update direction received"}, [4]bool{true, true, true, false}},
		{"single", []string{"192.0.2.3"}, []string{"update direction received"}, [4]bool{false, true, false, false}},
		{"duplicate", []string{"192.0.2.3", "192.0.2.3"}, []string{"update direction received"}, [4]bool{false, true, false, false}},
		{"overlap", []string{"192.0.2.3", "*"}, []string{"update direction received"}, [4]bool{true, true, true, false}},
		{"exclusion", []string{"!192.0.2.2"}, []string{"update direction received"}, [4]bool{false, true, true, false}},
		{"wildcard-events", []string{"192.0.2.2", "192.0.2.3", "192.0.2.5"}, []string{"*", "update direction received"}, [4]bool{true, true, false, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSubscriptionServer()
			proc, batches := newDeliveryProbe(t, "selector-probe")
			subscriptionPeerCleanup(t, proc)
			peers := subscriptionPeerGrants(proc.Name())
			s.UpdateDeliveryGraph(graphNS, peers)
			s.registerSubscriptions(proc, &rpc.SubscribeEventsInput{
				Namespace: graphNS,
				Events:    tc.events,
				Peers:     tc.peers,
				Format:    "hex",
				Encoding:  "json",
			})

			for i, peer := range peers {
				payload := `{"peer":"` + peer.Addr + `","event":"update"}`
				subscriptionPeerDelivery(t, s, batches, peer.Addr, graphUpdate, events.DirectionReceived, payload, tc.want[i])
				// All attached peers grant both directions. Only the event
				// wildcard declares sent, so this checks declaration direction.
				wantSent := false
				if tc.name == "wildcard-events" {
					wantSent = tc.want[i]
				}
				subscriptionPeerDelivery(t, s, batches, peer.Addr, graphUpdate, events.DirectionSent, payload, wantSent)
				// State is a distinct registered type, granted only as received.
				// The wildcard must expand for the second selector too.
				subscriptionPeerDelivery(t, s, batches, peer.Addr, graphState, events.DirectionReceived, `{"state":"up"}`, wantSent)
				subscriptionPeerDelivery(t, s, batches, peer.Addr, graphState, events.DirectionSent, `{"state":"sent"}`, false)
			}

			// The second peer is selected in every case. Its final receipt is
			// a FIFO witness after all negative attempts, not a timed silence.
			subscriptionPeerDelivery(t, s, batches, "192.0.2.3", graphUpdate, events.DirectionReceived, `{"fence":true}`, true)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := proc.DrainEventsApplied(ctx); err != nil {
				t.Fatalf("drain delivered events: %v", err)
			}
			select {
			case extra := <-batches:
				t.Fatalf("unexpected extra delivery: %v", extra)
			default:
			}
		})
	}
}

// TestSubscriptionPeerSelectorMatching preserves IP/name and exclusion semantics
// through the same PeerScopedProcs boundary used by BGP delivery. Each selector
// is independent: alternatives union, rather than making exclusions global.
func TestSubscriptionPeerSelectorMatching(t *testing.T) {
	ns, update, _ := graphIDs(t)
	for _, tc := range []struct {
		name  string
		peers []string
		want  [4]bool
	}{
		{"second-name", []string{"first", "second"}, [4]bool{true, true, false, false}},
		{"mixed", []string{"192.0.2.2", "second"}, [4]bool{true, true, false, false}},
		{"name-overlap", []string{"192.0.2.3", "second"}, [4]bool{false, true, false, false}},
		{"single-name", []string{"second"}, [4]bool{false, true, false, false}},
		{"exclude-name", []string{"!first"}, [4]bool{false, true, true, false}},
		{"exclusion-alternative", []string{"!first", "first"}, [4]bool{true, true, true, false}},
		{"two-exclusions", []string{"!first", "!second"}, [4]bool{true, true, true, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSubscriptionServer()
			proc := process.NewProcess(plugin.PluginConfig{Name: "named-selector-probe"})
			s.registerSubscriptions(proc, &rpc.SubscribeEventsInput{
				Namespace: graphNS,
				Events:    []string{"update direction received"},
				Peers:     tc.peers,
			})
			// Exercise declaration-before-graph as well as the reverse order
			// used by the wire test: both are real startup sequences.
			peers := subscriptionPeerGrants(proc.Name())
			s.UpdateDeliveryGraph(graphNS, peers)
			for i, peer := range peers {
				got := s.PeerScopedProcs(ns, update, events.DirReceived, peer.Addr, peer.Name)
				if !tc.want[i] {
					if len(got) != 0 {
						t.Fatalf("peer %s: unexpected recipients %v", peer.Name, procNames(got))
					}
					continue
				}
				if len(got) != 1 {
					t.Fatalf("peer %s: recipients = %v, want one observer", peer.Name, procNames(got))
				}
				if got[0] != proc {
					t.Fatalf("peer %s: delivered to wrong process %s", peer.Name, got[0].Name())
				}
				declared, direction := s.subscriptions.declaredDirection(proc, ns, update, peer.Addr, peer.Name)
				if !declared {
					t.Fatalf("peer %s: reconciliation lost its declaration", peer.Name)
				}
				if direction != events.DirReceived {
					t.Fatalf("peer %s: declared direction = %v, want received", peer.Name, direction)
				}
			}
		})
	}
}

// TestSubscriptionPeerSelectorsRuntime exercises the subscribe-events RPC with
// multiple peers and verifies the second peer's enveloped bytes on the wire.
func TestSubscriptionPeerSelectorsRuntime(t *testing.T) {
	s := newSubscriptionServer()
	proc, batches := newDeliveryProbe(t, "runtime-selector-probe")
	subscriptionPeerCleanup(t, proc)
	s.UpdateDeliveryGraph(graphNS, subscriptionPeerGrants(proc.Name()))
	params, err := json.Marshal(&rpc.SubscribeEventsInput{
		Namespace: graphNS,
		Events:    []string{"update direction received"},
		Peers:     []string{"192.0.2.2", "192.0.2.3"},
		Format:    "hex",
		Encoding:  "json",
		Envelope:  true,
	})
	if err != nil {
		t.Fatalf("marshal subscription: %v", err)
	}
	if _, err := s.opSubscribeEvents(context.Background(), proc, params); err != nil {
		t.Fatalf("runtime subscribe: %v", err)
	}
	const payload = `{"peer":"192.0.2.3","receipt":"accepted"}`
	delivered, err := s.deliverEvent(nil, graphNS, graphUpdate, events.DirectionReceived, "192.0.2.3", payload)
	if err != nil {
		t.Fatalf("deliver second peer: %v", err)
	}
	if delivered != 1 {
		t.Fatalf("delivered to %d processes, want one", delivered)
	}
	batch := waitBatch(t, batches)
	if len(batch) != 1 {
		t.Fatalf("delivered batch = %v, want one envelope", batch)
	}
	envelope, err := rpc.ParseEventEnvelope(batch[0])
	if err != nil {
		t.Fatalf("parse delivered envelope: %v", err)
	}
	if envelope.Namespace != graphNS {
		t.Fatalf("delivered namespace = %q, want %q", envelope.Namespace, graphNS)
	}
	if envelope.Event != graphUpdate {
		t.Fatalf("delivered event = %q, want %q", envelope.Event, graphUpdate)
	}
	if string(envelope.Payload) != payload {
		t.Fatalf("delivered payload = %q, want %q", envelope.Payload, payload)
	}
}

// TestSubscriptionPeerSelectorsRejectedNamespace proves a rejected plural block
// neither adds peers nor changes existing format, encoding or envelope state.
// The runtime RPC uses the same registration producer as startup.
func TestSubscriptionPeerSelectorsRejectedNamespace(t *testing.T) {
	s := newSubscriptionServer()
	proc, batches := newDeliveryProbe(t, "namespace-selector-probe")
	subscriptionPeerCleanup(t, proc)
	s.UpdateDeliveryGraph(graphNS, subscriptionPeerGrants(proc.Name()))
	s.registerSubscriptions(proc, &rpc.SubscribeEventsInput{
		Namespace: graphNS,
		Events:    []string{"update direction received"},
		Peers:     []string{"192.0.2.3"},
		Format:    "hex",
		Encoding:  "json",
	})
	params, err := json.Marshal(&rpc.SubscribeEventsInput{
		Namespace: "subscription-peer-unknown-namespace",
		Events:    []string{"*"},
		Peers:     []string{"192.0.2.2", "192.0.2.4"},
		Format:    "parsed",
		Encoding:  "text",
		Envelope:  true,
	})
	if err != nil {
		t.Fatalf("marshal subscription: %v", err)
	}
	if _, err := s.opSubscribeEvents(context.Background(), proc, params); err != nil {
		t.Fatalf("runtime subscribe: %v", err)
	}
	if proc.Format() != "hex" {
		t.Fatalf("rejected block changed format to %q", proc.Format())
	}
	if proc.Encoding() != "json" {
		t.Fatalf("rejected block changed encoding to %q", proc.Encoding())
	}
	if proc.Envelope() {
		t.Fatal("rejected block enabled envelope")
	}
	subscriptionPeerDelivery(t, s, batches, "192.0.2.2", graphUpdate, events.DirectionReceived, `{"unexpected":"first"}`, false)
	subscriptionPeerDelivery(t, s, batches, "192.0.2.4", graphUpdate, events.DirectionReceived, `{"unexpected":"third"}`, false)
	subscriptionPeerDelivery(t, s, batches, "192.0.2.3", graphUpdate, events.DirectionReceived, `{"existing":true}`, true)
}

// subscriptionPeerGrants grants both UPDATE directions and received state to
// three peers. The fourth has no receive grant, even when explicitly selected.
func subscriptionPeerGrants(name string) []DeliveryPeer {
	peers := []DeliveryPeer{
		{Addr: "192.0.2.2", Name: "first"},
		{Addr: "192.0.2.3", Name: "second"},
		{Addr: "192.0.2.4", Name: "third"},
		{Addr: "192.0.2.5", Name: "ungranted"},
	}
	for i := range 3 {
		peers[i].Bindings = []plugin.PeerProcessBinding{{
			PluginName: name,
			Receive: map[string]events.Direction{
				graphUpdate: events.DirBoth,
				graphState:  events.DirReceived,
			},
		}}
	}
	return peers
}

func subscriptionPeerDelivery(t *testing.T, s *Server, batches <-chan []string, peer, event, direction, payload string, want bool) {
	t.Helper()
	delivered, err := s.deliverEvent(nil, graphNS, event, direction, peer, payload)
	if err != nil {
		t.Fatalf("deliver %s %s for %s: %v", event, direction, peer, err)
	}
	if !want {
		if delivered != 0 {
			t.Fatalf("delivered %s %s for excluded peer %s to %d processes", event, direction, peer, delivered)
		}
		return
	}
	if delivered != 1 {
		t.Fatalf("delivered %s %s for %s to %d processes, want one", event, direction, peer, delivered)
	}
	batch := waitBatch(t, batches)
	if len(batch) != 1 {
		t.Fatalf("batch for %s = %v, want exactly one payload", peer, batch)
	}
	if batch[0] != payload {
		t.Fatalf("delivered bytes for %s = %q, want %q", peer, batch[0], payload)
	}
}

func subscriptionPeerCleanup(t *testing.T, proc *process.Process) {
	t.Helper()
	t.Cleanup(func() {
		proc.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := proc.Wait(ctx); err != nil {
			t.Errorf("stop delivery process: %v", err)
		}
	})
}
