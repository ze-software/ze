// Tests for the peer-up Adj-RIB-Out replay guard: which sessions get their
// Adj-RIB-Out re-advertised, and which must not.

package rib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// captureRIBLog installs a debug-level capturing logger for the test's duration.
func captureRIBLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := logger()
	SetLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { SetLogger(prev) })
	return &buf
}

// replayTestPeer is the single peer these tests exercise.
const replayTestPeer = "10.0.0.1"

// stateEventFor builds a state event for the text rail.
func stateEventFor(t *testing.T, state string, session uint64) *Event {
	t.Helper()
	return &Event{PeerOwner: 1, DestinationSession: session, Peer: mustMarshal(t, map[string]any{
		"state":  state,
		"remote": map[string]any{"address": replayTestPeer, "as": uint32(65001)},
	})}
}

// oneRouteRibOut pre-populates the peer's Adj-RIB-Out with a single route.
func oneRouteRibOut(t *testing.T, r *RIBManager) {
	t.Helper()
	r.peerMeta[1] = &peerMetadata{Address: netip.MustParseAddr(replayTestPeer)}
	r.ribOut[1] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {
			"10.0.0.0/24": {MsgID: 9, Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		},
	})
	// This fixture represents a locally injected route, not a source-less
	// forwarded route. Production sent receipts name that origin explicitly.
	for key, entry := range r.ribOut[1][family.IPv4Unicast] {
		entry.LocalOrigin = true
		r.ribOut[1][family.IPv4Unicast][key] = entry
	}
}

// TestPeerUpReplaySkippedOnFirstSession is the regression for the second duplicate
// test/plugin/llgr-readvertise-multipeer.ci caught.
//
// VALIDATES: a peer coming up for the FIRST time gets no Adj-RIB-Out replay, even
// when entries are already recorded for it.
// PREVENTS: a second copy of a route the live forward rail has already put on the
// wire. RFC 4271 Section 3.2: Adj-RIB-Out holds what has been advertised to a peer,
// and a session that has only just been established has been advertised nothing --
// so an entry present here was recorded from a send made on THIS session, because
// the sent event and the state event are produced by different goroutines and the
// send won. The replayed copy travels the announce rail, so it carries neither the
// RFC 4456 reflection attributes nor the RFC 9494 stale depreference the live copy
// had: two copies that do not even agree.
func TestPeerUpReplaySkippedOnFirstSession(t *testing.T) {
	r := newTestRIBManager(t)
	buf := captureRIBLog(t)

	oneRouteRibOut(t, r)
	r.handleState(stateEventFor(t, "up", 11))

	out := buf.String()
	assert.Contains(t, out, "adj-rib-out replay skipped on a peer's first session",
		"the suppressed duplicate must be visible; a guard that neither denies nor speaks does not exist")
	assert.NotContains(t, out, "update-route failed",
		"no route command may be attempted: attempting one IS the duplicate")
	assert.Len(t, r.ribOut[1], 1,
		"declining to replay must not discard the Adj-RIB-Out itself")
}

// TestPeerUpReplayRunsOnReestablishedSession keeps the feature the guard must not
// break.
//
// VALIDATES: a peer that has been up before and comes up again DOES get its
// Adj-RIB-Out re-advertised.
// PREVENTS: turning the duplicate fix into a route loss. A re-established session
// has genuinely been advertised those routes on a previous session and starts with
// an empty table, so it needs every one of them back.
func TestPeerUpReplayRunsOnReestablishedSession(t *testing.T) {
	r := newTestRIBManager(t)

	// First session, then teardown: the peer is now known to have been up.
	r.handleState(stateEventFor(t, "up", 11))
	r.handleState(stateEventFor(t, "down", 11))

	buf := captureRIBLog(t)
	oneRouteRibOut(t, r)
	r.handleState(stateEventFor(t, "up", 12))

	out := buf.String()
	assert.NotContains(t, out, "adj-rib-out replay skipped",
		"a re-established session is exactly the case the replay exists for")
	assert.Contains(t, out, "update-route failed",
		"route commands must be attempted (the test SDK connection is closed, so each one logs)")
}

// TestPeerUpReplayFirstSessionQuietWhenNothingRecorded keeps the guard from
// crying wolf.
//
// VALIDATES: the skip is silent when the Adj-RIB-Out is empty, which is the
// ordinary state of a first session.
// PREVENTS: a WARN on every peer-up, which would train operators to ignore the one
// line that reports a real event reordering.
func TestPeerUpReplayFirstSessionQuietWhenNothingRecorded(t *testing.T) {
	r := newTestRIBManager(t)
	buf := captureRIBLog(t)

	r.handleState(stateEventFor(t, "up", 11))

	assert.NotContains(t, buf.String(), "adj-rib-out replay skipped",
		"an empty Adj-RIB-Out on a first session is normal and says nothing")
}

// TestCollectPeerUpReplayGuard pins the guard's own decision table.
//
// VALIDATES: collectPeerUpReplay returns nothing for an unseen peer and the full
// grouped Adj-RIB-Out for a known one.
// PREVENTS: an inverted seenBefore, which would either restore the duplicate or
// silence every re-established peer's replay.
func TestCollectPeerUpReplayGuard(t *testing.T) {
	for _, tc := range []struct {
		name        string
		seenBefore  bool
		wantGroups  int
		wantSkipLog bool
	}{
		{"first session", false, 0, true},
		{"re-established session", true, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRIBManager(t)
			buf := captureRIBLog(t)
			oneRouteRibOut(t, r)

			groups := r.collectPeerUpReplay(1, tc.seenBefore)

			require.Len(t, groups, tc.wantGroups)
			assert.Equal(t, tc.wantSkipLog,
				strings.Contains(buf.String(), "adj-rib-out replay skipped"))
		})
	}
}

// TestPeerUpReplayAsyncDoesNotDeadlockDirectBridge proves peer-up replay runs
// off the event-delivery goroutine once background workers are started.
//
// VALIDATES: replaying more UPDATEs than a bounded delivery channel holds does
// not self-deadlock when each replayed update-route synchronously produces a
// sent event back into the plugin's own event queue (the DirectBridge path).
// PREVENTS: Process(bgp-rib).deliveryLoop blocking on itself after 64 replayed
// UPDATEs, which stalled deliverPeerState and bgp-rs replay on peer reconnect.
func TestPeerUpReplayAsyncDoesNotDeadlockDirectBridge(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() {
		bridge.CloseCallbacks()
		_ = pluginEnd.Close()
		_ = engineEnd.Close()
	})

	// Tiny bounded queue mimicking Process.deliveryLoop's 64-slot eventChan.
	// Every replayed update-route pushes back onto eventChan; if replay ran
	// synchronously inside handleStructuredState on the delivery goroutine, the
	// 5th RPC would deadlock forever because nothing is draining eventChan.
	const replayGroups = 20
	eventChan := make(chan int, 4)
	var sentCount atomic.Int32

	bridge.SetDispatchRPC(func(rpcCtx context.Context, method string, raw json.RawMessage) (json.RawMessage, error) {
		if method == rpc.MethodUpdateRoute {
			var input rpc.UpdateRouteInput
			if err := json.Unmarshal(raw, &input); err != nil {
				return nil, err
			}
			if strings.Contains(input.Command, " add ") {
				select {
				case eventChan <- int(sentCount.Add(1)):
				case <-rpcCtx.Done():
					return nil, rpcCtx.Err()
				}
			}
			return json.Marshal(rpc.UpdateRouteOutput{Announced: 1})
		}
		return json.Marshal(rpc.DispatchCommandOutput{Status: "done"})
	})
	bridge.SetReady()

	p := sdk.NewWithConn("rib-test", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { _ = p.Close() })
	r := newRIBManager(p)
	require.NoError(t, r.startRIBWorkers(ctx))
	t.Cleanup(func() { r.shutdownReceivedSources() })

	// Establish once then tear down so peer 1 is known and eligible for replay.
	r.handleStructuredState(&rpc.StructuredEvent{
		PeerAddress: replayTestPeer, PeerOwner: 1, DestinationSession: 11, State: rpc.SessionStateUp,
	})
	r.handleStructuredState(&rpc.StructuredEvent{
		PeerAddress: replayTestPeer, PeerOwner: 1, DestinationSession: 11, State: rpc.SessionStateDown,
	})

	// Seed 20 distinct next-hop groups so replay emits 20 separate update-route RPCs.
	routes := make(map[string]*Route, replayGroups)
	for i := range replayGroups {
		pfx := fmt.Sprintf("10.0.%d.0/24", i)
		routes[pfx] = &Route{
			MsgID:   uint64(100 + i),
			Family:  family.IPv4Unicast,
			Prefix:  pfx,
			NextHop: fmt.Sprintf("1.1.1.%d", i+1),
		}
	}
	r.peerMu.Lock()
	r.peerMeta[1] = &peerMetadata{Address: netip.MustParseAddr(replayTestPeer)}
	r.ribOut[1] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: routes,
	})
	for key, entry := range r.ribOut[1][family.IPv4Unicast] {
		entry.LocalOrigin = true
		r.ribOut[1][family.IPv4Unicast][key] = entry
	}
	r.peerMu.Unlock()

	// Run handleStructuredState on the same goroutine that drains eventChan.
	// Returning promptly proves replay was handed off to backgroundWG.
	r.handleStructuredState(&rpc.StructuredEvent{
		PeerAddress: replayTestPeer, PeerOwner: 1, DestinationSession: 12, State: rpc.SessionStateUp,
	})

	drained := 0
	for drained < replayGroups {
		select {
		case <-eventChan:
			drained++
		case <-ctx.Done():
			t.Fatalf("deadlocked or timed out after draining %d/%d replayed UPDATEs", drained, replayGroups)
		}
	}
	assert.Equal(t, replayGroups, drained, "all replayed UPDATEs must complete without deadlocking deliveryLoop")
}
