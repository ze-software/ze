// Design: docs/guide/l2tp.md -- retained original listener inventory

package l2tp

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestListenerInventoryKeepsOriginalBoundRetainedRoot starts a real socket,
// changes the current configured endpoint, then reads the stopped retained root.
// A current-config reconstruction and duplicate root publication cannot pass.
func TestListenerInventoryKeepsOriginalBoundRetainedRoot(t *testing.T) {
	reactor := newReactorForSnapshot(t)
	reactor.listener = newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), reactor.logger)
	require.NoError(t, reactor.listener.Start(t.Context()))
	t.Cleanup(func() { require.NoError(t, reactor.listener.Stop()) })
	bound := reactor.listener.Addr()
	require.NotZero(t, bound.Port())
	require.NoError(t, reactor.listener.Stop())
	reactor.params.ServerName = "original"
	reactor.params.ConfiguredListen = netip.MustParseAddrPort("192.0.2.99:1701")
	subsystem := NewSubsystem(&Parameters{})
	subsystem.reactors = []*l2tpReactor{reactor}
	subsystem.retiredReactors = []*l2tpReactor{reactor}
	listeners := subsystem.Listeners()
	require.Equal(t, []ListenerSnapshot{{Name: "original", Addr: bound}}, listeners)
	require.NotEqual(t, reactor.params.ConfiguredListen, listeners[0].Addr)
}
