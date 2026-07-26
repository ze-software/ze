// Tests for the peer-up Adj-RIB-Out replay guard: which sessions get their
// Adj-RIB-Out re-advertised, and which must not.

package rib

import (
	"bytes"
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
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
func stateEventFor(t *testing.T, state string) *Event {
	t.Helper()
	return &Event{Peer: mustMarshal(t, map[string]any{
		"state":  state,
		"remote": map[string]any{"address": replayTestPeer, "as": uint32(65001)},
	})}
}

// oneRouteRibOut pre-populates the peer's Adj-RIB-Out with a single route.
func oneRouteRibOut(t *testing.T, r *RIBManager) {
	t.Helper()
	r.ribOut[netip.MustParseAddr(replayTestPeer)] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {
			"10.0.0.0/24": {MsgID: 9, Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
		},
	})
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
	r.handleState(stateEventFor(t, "up"))

	out := buf.String()
	assert.Contains(t, out, "adj-rib-out replay skipped on a peer's first session",
		"the suppressed duplicate must be visible; a guard that neither denies nor speaks does not exist")
	assert.NotContains(t, out, "update-route failed",
		"no route command may be attempted: attempting one IS the duplicate")
	assert.Len(t, r.ribOut[netip.MustParseAddr(replayTestPeer)], 1,
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
	r.handleState(stateEventFor(t, "up"))
	r.handleState(stateEventFor(t, "down"))

	buf := captureRIBLog(t)
	oneRouteRibOut(t, r)
	r.handleState(stateEventFor(t, "up"))

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

	r.handleState(stateEventFor(t, "up"))

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

			groups := r.collectPeerUpReplay(netip.MustParseAddr(replayTestPeer), tc.seenBefore)

			require.Len(t, groups, tc.wantGroups)
			assert.Equal(t, tc.wantSkipLog,
				strings.Contains(buf.String(), "adj-rib-out replay skipped"))
		})
	}
}
