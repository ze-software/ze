// Detail: summary.go -- handleBgpOverview, the argument grammar of `show bgp`

package peer

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/family"
)

// showBgpDispatcher answers `show bgp` the way the daemon does: the dispatcher
// matches the registered key and hands the handler every token it did not
// consume (matchBuiltinTokens, internal/component/plugin/server/command.go).
func showBgpDispatcher(t *testing.T) (*pluginserver.Dispatcher, *pluginserver.CommandContext) {
	t.Helper()

	reactor := &mockReactor{
		peers: []plugin.PeerInfo{{
			Address:            netip.MustParseAddr("192.0.2.1"),
			PeerAS:             65001,
			State:              plugin.PeerStateEstablished,
			NegotiatedFamilies: []family.Family{family.IPv4Unicast},
		}},
		stats: plugin.ReactorStats{PeerCount: 1},
	}
	d := pluginserver.NewDispatcher()
	d.Register("show bgp", handleBgpOverview, "BGP overview")
	return d, newTestContext(reactor)
}

// TestShowBgpRefusesATokenAfterTheFamily drives the operator's own text through
// the dispatcher, which is where the trailing token comes from.
//
// VALIDATES: a token after the address family is reported as the unknown
// command it is, and the accepted forms still answer.
// PREVENTS:  `show bgp ipv4 rubbish` answering the ipv4 summary, which hands
// the operator a different question's answer and never says that `rubbish`
// reached nobody (plan/journal/silent-fall-through.md, 2026-08-21).
func TestShowBgpRefusesATokenAfterTheFamily(t *testing.T) {
	d, ctx := showBgpDispatcher(t)

	t.Run("a token after the family", func(t *testing.T) {
		resp, err := d.Dispatch(ctx, "show bgp ipv4 rubbish")
		require.Error(t, err)
		assert.ErrorIs(t, err, pluginserver.ErrUnknownCommand)
		require.NotNil(t, resp)
		assert.Equal(t, plugin.StatusError, resp.Status)
		assert.Nil(t, resp.Data, "a refused command must answer no summary")
		assert.Contains(t, resp.Error, "show bgp ipv4 rubbish", "the refusal names the path typed")
		assert.NotContains(t, resp.Error, "invalid family", "the family was valid; the token after it was not")
	})

	t.Run("an oversized token is bounded in the message", func(t *testing.T) {
		resp, err := d.Dispatch(ctx, "show bgp ipv4 "+strings.Repeat("z", 4096))
		require.Error(t, err)
		assert.ErrorIs(t, err, pluginserver.ErrUnknownCommand)
		assert.Less(t, len(resp.Error), 160, "operator input must not reach the envelope unbounded")
	})

	t.Run("the family alone still answers", func(t *testing.T) {
		resp, err := d.Dispatch(ctx, "show bgp ipv4")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, plugin.StatusDone, resp.Status)
		data, ok := resp.Data.(plugin.Map)
		require.True(t, ok)
		assert.Equal(t, "ipv4/unicast", data["family"])
	})

	t.Run("the bare command still answers", func(t *testing.T) {
		resp, err := d.Dispatch(ctx, "show bgp")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, plugin.StatusDone, resp.Status)
	})
}
