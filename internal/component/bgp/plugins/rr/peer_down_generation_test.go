// Design: docs/architecture/plugin/rib-storage-design.md -- Source-DOWN replacement and sent ordering
// Related: rr.go -- dispatchText, handleStructuredState, recoverRouteSel

package rr

import (
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPeerDownEventGenerationReachesDeparture proves the session a down
// names reaches the departure's recovery-generation meta on both event
// paths, text and structured. Method: the source holds one route; the down
// arrives as a text line or as a StructuredEvent with generation 9, and the
// request the engine receives must carry "9".
func TestPeerDownEventGenerationReachesDeparture(t *testing.T) {
	const source = "192.0.2.1"
	deliveries := map[string]func(rr *routeReflector){
		"text": func(rr *routeReflector) {
			rr.dispatchText("peer " + source + " remote as 65001 state down peer-owner 701 connection-id 41 reason tcp-failure session-generation 9\n")
		},
		"structured": func(rr *routeReflector) {
			rr.handleStructuredState(&rpc.StructuredEvent{PeerAddress: source, PeerOwner: 701, ConnectionID: 41,
				State: rpc.SessionStateDown, SessionGeneration: 9})
		},
	}
	for name, deliver := range deliveries {
		t.Run(name, func(t *testing.T) {
			rr, requests := newPeerDownReflector(t, rrOriginalFixture(t, "192.0.2.1", 701, 41))
			// Preserve the helper's explicit original UP and captured destination.
			origin := []byte{0x40, 1, 1, 0}
			rrOriginalWire(t, rr, rrOriginalFixture(t, source, 701, 41), 70, 80, rrAddPathUpdate(t, nil, origin, []byte{0, 0, 0, 11, 8, 10}))
			deliver(rr)
			var got peerDownRequest
			select {
			case got = <-requests:
			case <-time.After(5 * time.Second):
				t.Fatal("peer-down sent no request")
			}
			rr.stopDepartures()
			if got.meta["recovery-generation"] != "9" {
				t.Fatalf("peer-down meta = %v, want recovery-generation 9: the departure names the session its down ended", got.meta)
			}
		})
	}
}
