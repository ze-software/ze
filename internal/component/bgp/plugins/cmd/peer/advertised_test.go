package peer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/route"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestRibClearAdvertisedSelector proves the handler hands the bound selector
// to the reactor, with AFI 0 for every family, and answers the count.
// Method: a mock reactor records the call.
func TestRibClearAdvertisedSelector(t *testing.T) {
	reactor := &mockReactor{advertisedCount: 3}
	ctx := newTestContext(reactor)
	ctx.Selectors = map[string]string{advertisedSelectorLeaf: "*"}

	resp, err := handleRibClearAdvertised(ctx, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status, resp.Error)
	require.Equal(t, []string{"*"}, reactor.advertisedCalls)
	require.Zero(t, reactor.advertisedAFI)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	require.Equal(t, 3, data["withdrawn"])
}

// TestRibClearAdvertisedRefusals proves the two refusals: no selector names
// the explicit forms and reaches no reactor, and a selector matching no peer
// is an error rather than an empty success.
func TestRibClearAdvertisedRefusals(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newTestContext(reactor)
	resp, err := handleRibClearAdvertised(ctx, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Error, "clear bgp rib advertised * for every peer")
	require.Empty(t, reactor.advertisedCalls)

	reactor.advertisedErr = route.ErrNoPeersMatch
	ctx.Selectors = map[string]string{advertisedSelectorLeaf: "192.0.2.9"}
	resp, err = handleRibClearAdvertised(ctx, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, resp.Status)
	require.Contains(t, resp.Error, "no peer matches 192.0.2.9")
}
