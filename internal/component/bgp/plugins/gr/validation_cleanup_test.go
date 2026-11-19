// Design: docs/guide/graceful-restart.md -- actual GR command completion.
package gr

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"testing"
)

// Actual source command production preserves original actor and keywords;
// only completed removal operations produce passive validation cleanup.
func TestGRSourceCleanupAfterAppliedCommand(t *testing.T) {
	source, err := newGRSource("192.0.2.50", 1701, 41)
	require.NoError(t, err)
	gp := &grPlugin{}
	var order []string
	gp.dispatchHook = func(command string, args ...string) {
		order = append(order, "command")
		require.Equal(t, []string{"owner", "1701", "connection", "41"}, args[:4])
	}
	var cleanups []retention.Cleanup
	gp.cleanupHook = func(payload string) {
		order = append(order, "cleanup")
		cleanup, err := retention.DecodeCleanup(rpc.EventEnvelope{Payload: json.RawMessage(payload)})
		require.NoError(t, err)
		cleanups = append(cleanups, cleanup)
	}
	gp.dispatchSource(source, "request bgp rib source mark-stale", "restart-time", "3600")
	require.Empty(t, cleanups)
	gp.dispatchSource(source, "request bgp rib source purge-stale", "family", "ipv4/unicast")
	require.Len(t, cleanups, 1)
	require.Equal(t, uint64(1701), cleanups[0].Owner())
	require.Equal(t, uint64(41), cleanups[0].Connection())
	require.True(t, cleanups[0].MatchesFamily(family.IPv4Unicast))
	require.False(t, cleanups[0].MatchesFamily(family.IPv6Unicast))
	gp.dispatchSource(source, "request bgp rib source delete-with-community", "family", "ipv4/unicast", "community", "ffff0007")
	require.Len(t, cleanups, 2)
	require.True(t, cleanups[1].NoLLGROnly())
	gp.dispatchSource(source, "request bgp rib source release-routes")
	require.Len(t, cleanups, 3)
	require.True(t, cleanups[2].MatchesFamily(family.IPv6Unicast))
	require.Equal(t, []string{"command", "cleanup", "command", "cleanup", "command", "cleanup", "command"}, order)
	gp.dispatchHook = nil
	gp.dispatchSource(source, "request bgp rib source release-routes")
	require.Len(t, cleanups, 3, "an unapplied command cannot claim metadata cleanup")
}
