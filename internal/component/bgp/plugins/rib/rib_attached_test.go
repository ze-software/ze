// Design: docs/architecture/plugin/rib-storage-design.md — an attached up replays nothing
// Related: rib.go — handleState and handleStructuredState, the up handlers under test

package rib

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRIBSkipsReplayOnAttachedUp proves the rib's two up handlers replay
// nothing toward a peer when the up carries reason attached: a reload attached
// the rib to a session that was already up, so there is no initial-replay token
// to answer and the engine refuses a ready for session 0. The peer is still
// recorded up, so its later down purges what the rib learns from it.
//
// VALIDATES: no "plugin session ready" dispatch on an attached up, on either
// the text or the structured path; one on a plain up (the control).
// PREVENTS: "request peer X plugin session ready session 0" refused by the
// engine on every receive-only attach (bgp-attach-grant-change-live.ci).
func TestRIBSkipsReplayOnAttachedUp(t *testing.T) {
	peer := netip.MustParseAddr("10.0.0.1")
	const attachedUp = `{"type":"state","peer":{"address":"10.0.0.1","remote":{"address":"10.0.0.1","as":65001}},"state":"up","reason":"attached"}`
	const plainUp = `{"type":"state","peer":{"address":"10.0.0.1","remote":{"address":"10.0.0.1","as":65001}},"state":"up","initial-replay":"1"}`

	for _, tc := range []struct {
		name  string
		up    func(*testing.T, *RIBManager)
		ready int
	}{
		{"text attached", func(t *testing.T, r *RIBManager) { r.handleState(mustParseEvent(t, attachedUp)) }, 0},
		{"structured attached", func(_ *testing.T, r *RIBManager) {
			r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peer.String(), State: rpc.SessionStateUp, Reason: rpc.ReasonAttached})
		}, 0},
		{"text plain up", func(t *testing.T, r *RIBManager) { r.handleState(mustParseEvent(t, plainUp)) }, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRIBManager(t)
			ready := 0
			r.dispatchHook = func(command string) {
				if strings.Contains(command, "plugin session ready") {
					ready++
				}
			}
			tc.up(t, r)
			if ready != tc.ready {
				t.Fatalf("ready markers = %d, want %d", ready, tc.ready)
			}
			r.peerMu.Lock()
			up := r.peerUp[peer]
			r.peerMu.Unlock()
			if !up {
				t.Fatal("the peer must be recorded up, so its later down purges its routes")
			}
		})
	}
}

func mustParseEvent(t *testing.T, raw string) *Event {
	t.Helper()
	event, err := parseEvent([]byte(raw))
	if err != nil {
		t.Fatalf("parse event: %v", err)
	}
	return event
}
