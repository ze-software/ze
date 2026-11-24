// Design: docs/architecture/bgp/structural-forwarding.md -- original fast-path handback targets.
// Related: server_forward.go -- the cached rail serves handed-back original destinations.
package rs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFastPathHandbackKeepsOriginalEndpoint checks the cached rail forwards
// both accepted target address forms while preserving exact owner/session scope.
func TestFastPathHandbackKeepsOriginalEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name    string
		target  rpc.ForwardTarget
		forward bool
	}{
		{name: "address", target: rpc.ForwardTarget{Destination: "10.0.0.2", DestinationOwner: 2, DestinationSession: 202}, forward: true},
		{name: "endpoint", target: rpc.ForwardTarget{Destination: "10.0.0.2:179", DestinationOwner: 2, DestinationSession: 202}, forward: true},
		{name: "different owner", target: rpc.ForwardTarget{Destination: "10.0.0.2:179", DestinationOwner: 3, DestinationSession: 202}},
		{name: "different session", target: rpc.ForwardTarget{Destination: "10.0.0.2:179", DestinationOwner: 2, DestinationSession: 203}},
		{name: "different address", target: rpc.ForwardTarget{Destination: "10.0.0.3:179", DestinationOwner: 2, DestinationSession: 202}},
		{name: "invalid endpoint", target: rpc.ForwardTarget{Destination: "invalid", DestinationOwner: 2, DestinationSession: 202}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rs := newTestRouteServer(t)
			source, err := newWorkerKey("10.0.0.1", 1, 101)
			require.NoError(t, err)
			destination, err := newWorkerKey("10.0.0.2", 2, 102)
			require.NoError(t, err)
			rs.peers[source] = &PeerState{Address: source.address.String(), Up: true, RSClient: true, StateSeen: true,
				DestinationSession: 201, InitialReplay: 20}
			rs.peers[destination] = &PeerState{Address: destination.address.String(), Up: true, RSClient: true, StateSeen: true,
				DestinationSession: 202, ForwardFrom: 10, InitialReplay: 20}
			var forwarded []string
			rs.forwardCachedHook = func(ids []uint64, targets []string) {
				require.Equal(t, []uint64{40}, ids)
				forwarded = append(forwarded, targets...)
			}
			rs.batchForwardUpdateSkipped(source, source.address.String(), 40,
				map[family.Family]bool{family.IPv4Unicast: true}, []rpc.ForwardTarget{tc.target})
			batch := rs.workerBatch(source)
			if tc.forward {
				require.Equal(t, []rpc.ForwardTarget{tc.target}, batch.targets, "handback must preserve original tuple")
			} else {
				require.Empty(t, batch.targets)
			}
			rs.flushWorkerBatch(source)
			if tc.forward {
				require.Equal(t, []string{tc.target.Destination}, forwarded)
			} else {
				require.Empty(t, forwarded)
			}
		})
	}
}
