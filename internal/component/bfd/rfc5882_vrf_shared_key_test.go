// Design: rfc/short/rfc5882.md -- one session per remote system, whatever asks for it

package bfd

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/engine"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
)

// multiHopServiceIn builds the BFD runtime for one VRF with its multi-hop loop
// already in place, so the real applyPinned and pluginService.EnsureSession
// run end to end without binding the RFC 5883 port in a unit test. The loop is
// never started: EnsureSession only registers the session, which is the part
// under test.
func multiHopServiceIn(t *testing.T, vrf string, pinned map[string]any) (*runtimeState, *engine.Loop) {
	t.Helper()
	tr, _ := transport.Pair(api.MultiHop, netip.MustParseAddr("172.30.9.2"), netip.MustParseAddr("203.0.113.9"))
	loop := engine.NewLoop(tr, clock.RealClock{})
	state := newRuntimeState()
	key := loopKey{vrf: vrf, mode: api.MultiHop}
	state.loops[key] = loop
	state.loopDevices[key] = loopDeviceFor(api.SessionRequest{VRF: vrf})

	entry, err := parseMultiHopSession("203.0.113.9", pinned, nil)
	if err != nil {
		t.Fatalf("parseMultiHopSession: %v", err)
	}
	if err := state.applyPinned(&pluginConfig{enabled: true, sessions: []sessionConfig{entry}}); err != nil {
		t.Fatalf("applyPinned: %v", err)
	}
	return state, loop
}

// RFC requirement: RFC5882-4.4-1 positive -- "If multiple control protocols
// wish to establish BFD sessions with the same remote system for the same data
// protocol, all MUST share a single BFD session" (RFC 5882 sec 4.4). A pinned
// multi-hop-session to 203.0.113.9 in VRF red names its local address, and a
// BGP multi-hop peer to the same address names none, which is the request
// bfdRequestFor builds for a peer with no `connection local ip`. Both reach
// the engine through their real entry points, applyPinned and
// pluginService.EnsureSession, and the engine holds ONE session at refcount 2.
//
// Goal and method: the pinned entry is parsed by parseMultiHopSession and
// applied by applyPinned; the BGP request goes through
// pluginService.EnsureSession, which canonicalizes it and hands it to
// Loop.EnsureSession. The session count and the refcount are read from the
// loop's Snapshot, which is what `show bfd session` prints.
//
// MUTATION: make Loop.sharedEntryLocked return (nil, false) and the BGP
// request opens a second session, so this test goes red.
func TestMultiHopClientsInAVRFShareOneSession(t *testing.T) {
	state, loop := multiHopServiceIn(t, "red", map[string]any{"local": "172.30.9.2", "vrf": "red"})

	bgp := api.SessionRequest{Peer: netip.MustParseAddr("203.0.113.9"), VRF: "red", Mode: api.MultiHop}
	if _, err := (&pluginService{state: state}).EnsureSession(bgp); err != nil {
		t.Fatalf("EnsureSession (bgp): %v", err)
	}

	snap := loop.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("one remote system in VRF red, %d sessions: %+v", len(snap), snap)
	}
	if snap[0].Refcount != 2 {
		t.Errorf("shared session refcount = %d, want 2: one for the pinned entry, one for the BGP peer", snap[0].Refcount)
	}
}

// RFC requirement: RFC5882-4.4-1 negative -- the sharing is for "the same
// remote system" (RFC 5882 sec 4.4), so a BGP multi-hop peer to a DIFFERENT
// remote system, 203.0.113.8, in the same VRF does not join the pinned
// session to 203.0.113.9, although it names no local address and so matches
// that session on every other field. Same entry points as the positive test.
//
// MUTATION: drop the Peer comparison from sharesSession and the BGP request
// joins the pinned session, so this test goes red.
func TestMultiHopClientsToTwoRemoteSystemsGetTwoSessions(t *testing.T) {
	state, loop := multiHopServiceIn(t, "red", map[string]any{"local": "172.30.9.2", "vrf": "red"})

	other := api.SessionRequest{Peer: netip.MustParseAddr("203.0.113.8"), VRF: "red", Mode: api.MultiHop}
	if _, err := (&pluginService{state: state}).EnsureSession(other); err != nil {
		t.Fatalf("EnsureSession (bgp to another system): %v", err)
	}

	snap := loop.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("two remote systems, %d sessions, want 2: %+v", len(snap), snap)
	}
	for i := range snap {
		if snap[i].Refcount != 1 {
			t.Errorf("session to %s refcount = %d, want 1", snap[i].Peer, snap[i].Refcount)
		}
	}
}
