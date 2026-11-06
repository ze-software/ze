// Design: docs/architecture/plugin/rib-storage-design.md -- validation publication ownership.
// Related: rib.go -- handleState and handleStructuredState, the peer-up replay under test
package adj_rib_in

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/seqmap"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPeerUpReplayDoesNotHoldMutationOwner proves a mutation is admitted while a
// peer-up replay is held at the relay, on both ingest paths, and takes effect: it
// rejects the very route the replay's snapshot is carrying.
//
// VALIDATES: handleState and handleStructuredState release mutationMu before the
// replay, whose relay waits on the engine for up to its own timeout, and a route
// rejected meanwhile leaves the store.
// PREVENTS: a source-policy or reject-routes command stalling behind one peer's
// replay, and a replay that needs the engine to finish such a command.
func TestPeerUpReplayDoesNotHoldMutationOwner(t *testing.T) {
	const source, dest = "10.0.0.1", "10.0.0.2"
	jsonUp := &bgp.Event{Type: "state", State: stateUp,
		Peer: mustMarshal(t, bgp.PeerInfoJSON{Remote: bgp.PeerRemoteInfo{Address: dest, AS: 65002}})}
	paths := map[string]func(*AdjRIBInManager){
		"json": func(r *AdjRIBInManager) { r.handleState(jsonUp) },
		"structured": func(r *AdjRIBInManager) {
			r.handleStructuredState(&rpc.StructuredEvent{EventType: rpc.EventKindState,
				State: rpc.SessionStateUp, PeerAddress: dest})
		},
	}
	for name, deliver := range paths {
		t.Run(name, func(t *testing.T) {
			r := newTestManager(t)
			// applyDecision answers nothing with validation off, and the reject below
			// is what proves the mutation ran rather than returned early.
			r.validationEnabled = true
			routes := seqmap.New[compactRouteKey, *RawRoute]()
			key := routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0)
			routes.Put(key, 1, &RawRoute{
				Family: family.IPv4Unicast, AttrHex: "40010100", NHopHex: "0a000001", NLRIHex: "180a0000",
			})
			r.ribIn[netip.MustParseAddr(source)] = routes

			relaying := make(chan struct{})
			release := make(chan struct{})
			r.routeRelayer = func(string, []rpc.StoredRoute) error {
				close(relaying)
				<-release
				return nil
			}
			replayed := make(chan struct{})
			go func() {
				defer close(replayed)
				deliver(r)
			}()
			defer func() {
				close(release)
				<-replayed
			}()
			select {
			case <-relaying:
			case <-time.After(5 * time.Second):
				t.Fatal("the peer-up replay never reached the relay")
			}

			admitted := make(chan struct{})
			go func() {
				defer close(admitted)
				// The command's answer also reports the publication this test has no
				// consumer for. What is under test is that it runs during the replay
				// and that the store changes, which the assertion below reads.
				_, _, _ = r.rejectRoutesCommand([]string{source, "ipv4/unicast", "10.0.0.0/24", "0"})
			}()
			select {
			case <-admitted:
			case <-time.After(5 * time.Second):
				t.Fatal("a mutation stayed blocked behind the peer-up replay")
			}
			r.mu.RLock()
			_, held := routes.Get(key)
			r.mu.RUnlock()
			if held {
				t.Fatal("a route rejected during the replay stayed in the store")
			}
		})
	}
}
