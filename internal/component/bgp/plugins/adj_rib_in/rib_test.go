// Related: rib.go -- handleState and handleStructuredState, the peer-up replay under test

package adj_rib_in

import (
	"testing"

	"github.com/stretchr/testify/assert"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAdjRIBInSkipsReplayOnAttachedUp proves AC-16 for adj-rib-in: an up with
// reason attached, which a reload sends when it attaches this plugin to a
// session already up, relays nothing toward the peer on either ingest path,
// while an ordinary up still replays.
//
// VALIDATES: handleState and handleStructuredState return before the replay
// when the reason is rpc.ReasonAttached.
// PREVENTS: a live attach re-sending stored routes into a session whose initial
// routing update already went out, and the "peer-up replay failed" log a
// process without a send grant would raise.
func TestAdjRIBInSkipsReplayOnAttachedUp(t *testing.T) {
	const dest = "10.0.0.2"
	jsonUp := func(reason string) *bgp.Event {
		return &bgp.Event{
			Type:        "state",
			State:       "up",
			Reason:      reason,
			UnheldRoles: []string{claimPeerUpReplay},
			Peer: mustMarshal(t, bgp.PeerInfoJSON{
				Remote: bgp.PeerRemoteInfo{Address: dest, AS: 65002},
			}),
		}
	}
	structuredUp := func(reason string) *rpc.StructuredEvent {
		return &rpc.StructuredEvent{
			EventType:   rpc.EventKindState,
			PeerAddress: dest,
			State:       rpc.SessionStateUp,
			Reason:      reason,
			UnheldRoles: []string{claimPeerUpReplay},
		}
	}

	r, got := claimedManagerWithRoute(t)
	r.handleState(jsonUp(rpc.ReasonAttached))
	assert.Empty(t, *got, "json ingest: an attached up relays nothing")
	r.handleStructuredState(structuredUp(rpc.ReasonAttached))
	assert.Empty(t, *got, "structured ingest: an attached up relays nothing")

	r, got = claimedManagerWithRoute(t)
	r.handleStructuredState(structuredUp(""))
	assert.Equal(t, []string{dest}, *got, "an ordinary up still replays")
}
