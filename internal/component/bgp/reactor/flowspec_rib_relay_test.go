// Design: docs/architecture/bgp/structural-forwarding.md -- mandatory FlowSpec authorization.
package reactor

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// forwardViaRIB does what a FlowSpec forwarder does with received UPDATE id.
// It forwards the UPDATE live, which carries every non-FlowSpec section and no
// FlowSpec rule (flowSpecSection), then relays each FlowSpec rule the way the
// route server replays the RIB's ValidationChange (rs processValidation): the
// received attribute block, the received generation as MsgID, one stored route
// per rule, and a withdrawal for each withdrawn rule. ADD-PATH is not framed.
func (f *aigpReplayFixture) forwardViaRIB(t *testing.T, id uint64) {
	t.Helper()
	api := &reactorAPIAdapter{r: f.r}
	sel, err := selector.Parse(f.destination.Settings().Address.String())
	require.NoError(t, err)
	sender := plugin.ProcessSender("aigp-forwarder")
	// Read the rules before the forward: a forward that dispatches nothing
	// lets the cache evict the entry.
	update, ok := f.r.recentUpdates.Get(id)
	require.True(t, ok, "received UPDATE is not cached")
	wu := wireu.NewWireUpdate(bytes.Clone(update.WireUpdate.Payload()), update.WireUpdate.SourceCtxID())
	payload := wu.Payload()
	withdrawn, err := wu.Withdrawn()
	require.NoError(t, err)
	attrStart := 4 + len(withdrawn)
	attrEnd := attrStart + int(binary.BigEndian.Uint16(payload[attrStart-2:attrStart]))
	attrs := bytes.Clone(payload[attrStart:attrEnd])
	source := f.source.Settings().Address.String()
	var routes []rpc.StoredRoute
	collect := func(data []byte, family string, withdraw bool) {
		_, err := nlrisplit.SplitFlowSpec(data, false, func(rule []byte) bool {
			route := rpc.StoredRoute{SourcePeer: source, Family: family, MsgID: id, Withdraw: withdraw,
				NLRIHex: hex.EncodeToString([]byte(ribevents.FlowSpecKey(rule))), NLRIFraming: rpc.NLRIFramingPrefixOnly}
			if !withdraw {
				route.AttrHex = hex.EncodeToString(attrs)
			}
			routes = append(routes, route)
			return true
		})
		require.NoError(t, err)
	}
	reach, err := wu.MPReach()
	require.NoError(t, err)
	if reach != nil && ribevents.IsFlowSpec(reach.Family()) {
		collect(reach.NLRIBytes(), reach.Family().String(), false)
	}
	unreach, err := wu.MPUnreach()
	require.NoError(t, err)
	if unreach != nil && ribevents.IsFlowSpec(unreach.Family()) {
		collect(unreach.WithdrawnBytes(), unreach.Family().String(), true)
	}
	if err := api.ForwardUpdate(sel, id, "aigp-forwarder", sender); err != nil && !errors.Is(err, errAllDestinationsSuppressed) {
		t.Fatal(err)
	}
	for i := range routes {
		require.NoError(t, api.RelayStoredRoute(f.destination.Settings().Address, routes[i:i+1], sender))
	}
}
