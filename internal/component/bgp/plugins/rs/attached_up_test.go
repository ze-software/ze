// Tests for the attached up: a reload that attaches the route server to a
// session already established.

package rs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRSSkipsReplayOnAttachedUp proves that an up carrying reason attached marks
// the peer up for the route server and starts no replay, on both event paths.
//
// VALIDATES: an attached up sets Up and StateSeen, moves the cut to the newest
// MessageID seen, shuts no replay gate (replayDone stays nil), dispatches no
// adj-rib-in replay, and bumps the replay generation so a replay a previous up
// still runs stops sending.
// PREVENTS: the route server replaying every stored route into a session a
// reload only attached it to, and signaling a session ready the engine
// refuses (an attached up carries no initial-replay receipt).
//
// Method: the dispatch hook blocks until the test ends, so a replay goroutine
// that did start would still hold its gate shut when the assertions run.
func TestRSSkipsReplayOnAttachedUp(t *testing.T) {
	const target = "10.0.0.2"
	deliver := map[string]func(rs *routeServer){
		"text": func(rs *routeServer) {
			rs.dispatchText("peer " + target + " remote as 65001 state up reason " + rpc.ReasonAttached + "\n")
		},
		"structured": func(rs *routeServer) {
			ev := parseStructuredState(&rpc.StructuredEvent{
				EventType:   rpc.EventKindState,
				PeerAddress: target,
				PeerAS:      rpc.RemoteASOf(65001),
				State:       rpc.SessionStateUp,
				Reason:      rpc.ReasonAttached,
			})
			require.NotNil(t, ev)
			rs.handleState(ev)
		},
	}
	for name, send := range deliver {
		t.Run(name, func(t *testing.T) {
			rs := newTestRouteServer(t)
			release := make(chan struct{})
			t.Cleanup(func() { close(release) })
			dispatched := make(chan string, 8)
			rs.dispatchCommandHook = func(cmd string, _ []string, _ string) (string, json.RawMessage, error) {
				dispatched <- cmd
				<-release
				return "", nil, nil
			}

			stale := make(chan struct{})
			rs.mu.Lock()
			rs.seenMsgID = 9
			rs.peers[target] = &PeerState{Address: target, ReplayGen: 3, replayDone: stale}
			rs.mu.Unlock()

			send(rs)

			rs.mu.Lock()
			peer := rs.peers[target]
			up, seen, cut, gen, gate := peer.Up, peer.StateSeen, peer.ForwardFrom, peer.ReplayGen, peer.replayDone
			rs.mu.Unlock()

			assert.True(t, up, "the session is up for the route server")
			assert.True(t, seen, "the peer's state is known")
			assert.Equal(t, uint64(9), cut, "the cut is the newest MessageID seen when the plugin attached")
			assert.Equal(t, uint64(4), gen, "the generation moves on, so a previous up's replay stops sending")
			assert.Nil(t, gate, "an attached up shuts no replay gate")
			select {
			case cmd := <-dispatched:
				t.Fatalf("an attached up dispatched %q; it must start no replay", cmd)
			default:
			}
		})
	}
}
