// Design: docs/architecture/api/process-protocol.md -- advisory automatic replay ownership.
package server

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// This uses actual structured and framed JSON delivery to an owner and an
// observer sharing one encoding. Advisory ownership must grant neither output
// eligibility nor a replay receipt to the observer.
func TestOrdinaryReplayOwnedDeliveryTransports(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "wire", true: "bridge"}[typed], func(t *testing.T) {
			snapshot := registry.Snapshot()
			t.Cleanup(func() { registry.Restore(snapshot) })
			s := newTestServer(t)
			const owner, observer = "ordinary-replay-owner", "ordinary-replay-observer"
			observations := make(chan selectedStateObservation, 2)
			var processes []*process.Process
			for _, name := range []string{owner, observer} {
				if typed {
					processes = append(processes, selectedStateBridgeProcess(t, name, observations))
					continue
				}
				processes = append(processes, selectedStateWireProcess(t, name, observations))
			}
			for _, scenario := range []struct {
				name  string
				state rpc.SessionState
				owner string
				token uint64
				want  bool
			}{
				{"ordinary-up", rpc.SessionStateUp, owner, 471, true},
				{"down", rpc.SessionStateDown, owner, 471, false},
				{"no-owner", rpc.SessionStateUp, "", 471, false},
				{"no-session", rpc.SessionStateUp, owner, 0, false},
			} {
				t.Run(scenario.name, func(t *testing.T) {
					peer := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.48"),
						DestinationSession: scenario.token, SelectedOwner: scenario.owner,
						OrdinaryReplayOwned: true}
					var cache formatCache
					var scratch [512]byte
					deliverPeerState(s, &peer, processes, nil, scenario.state, rpc.ReasonAttached, false, &cache, &scratch)
					for range len(processes) {
						select {
						case got := <-observations:
							require.Equal(t, scenario.want, got.ordinaryReplayOwned)
							selected := scenario.want && got.process == owner
							require.Equal(t, selected, got.selected)
							if selected {
								require.Equal(t, scenario.token, got.replay)
								continue
							}
							require.Zero(t, got.replay)
						case <-time.After(5 * time.Second):
							t.Fatal("advisory state did not reach both actual recipients")
						}
					}
				})
			}
		})
	}
}

func TestOrdinaryReplayOwnedStructuredReset(t *testing.T) {
	event := rpc.GetStructuredEvent()
	event.OrdinaryReplayOwned = true
	rpc.PutStructuredEvent(event)
	require.False(t, event.OrdinaryReplayOwned, "pooled advisory must not survive the previous UP")
}
