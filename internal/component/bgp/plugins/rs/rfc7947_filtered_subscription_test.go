// Design: docs/architecture/api/process-protocol.md -- subscription isolation of update-filtered
// Related: server.go -- routeServerStartupSubscriptions, handleStructuredEvent
// RFC naming: untagged -- subscription isolation unit test complementing reactor/rfc7947_filtered_conformance_test.go.
package rs

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRouteServerTakesNoDeliveryOfFilteredUpdates verifies A-6: bgp-rs's real
// startup subscriptions exclude update-filtered, and its structured handler
// ignores EventKindUpdateFiltered without mutating route-server state.
//
// VALIDATES: A-6 of spec-rfc7947-adj-rib-in-accepts-filtered-updates.
func TestRouteServerTakesNoDeliveryOfFilteredUpdates(t *testing.T) {
	subs := routeServerStartupSubscriptions()
	require.NotEmpty(t, subs)
	assert.True(t, slices.Contains(subs, eventUpdate+" direction received"),
		"positive control: bgp-rs must still subscribe to ordinary received updates")
	assert.False(t, slices.ContainsFunc(subs, func(s string) bool {
		return strings.HasPrefix(s, bgpevents.EventUpdateFiltered)
	}), "bgp-rs must not subscribe to %s", bgpevents.EventUpdateFiltered)
	assert.Equal(t, bgpevents.EventUpdateFiltered, rpc.EventKindUpdateFiltered.String())

	// Drive the real structured event handler with EventKindUpdateFiltered and
	// verify it neither dispatches nor mutates route-server tracking state.
	rs := &routeServer{
		peers:       make(map[workerKey]*PeerState),
		withdrawals: make(map[string]map[withdrawalKey]withdrawalEntry),
	}
	rs.handleStructuredEvent(&rpc.StructuredEvent{
		EventType:    rpc.EventKindUpdateFiltered,
		PeerAddress:  "10.0.0.1",
		PeerOwner:    7,
		ConnectionID: 1,
		MessageID:    42,
		RawMessage: &bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			MessageID:  42,
			Filtered:   true,
			FilterStep: "loop",
		},
	})
	assert.Zero(t, rs.seenMsgID, "EventKindUpdateFiltered must not advance bgp-rs seenMsgID")
	assert.Empty(t, rs.withdrawals, "EventKindUpdateFiltered must not record withdrawals in bgp-rs")
}
