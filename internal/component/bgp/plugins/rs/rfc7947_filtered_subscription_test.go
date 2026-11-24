// Design: docs/architecture/api/process-protocol.md -- subscription isolation of update-filtered
// Related: register.go -- rsBusReceives
// Related: server.go -- handleStructuredEvent
// RFC naming: untagged -- subscription isolation unit test complementing reactor/rfc7947_filtered_conformance_test.go.
package rs

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRouteServerTakesNoDeliveryOfFilteredUpdates verifies A-6: bgp-rs's real
// Stage 1 Receives declaration excludes update-filtered, and its structured
// handler ignores EventKindUpdateFiltered without mutating route-server state.
//
// VALIDATES: A-6 of spec-rfc7947-adj-rib-in-accepts-filtered-updates.
func TestRouteServerTakesNoDeliveryOfFilteredUpdates(t *testing.T) {
	receives := rsBusReceives()
	require.NotEmpty(t, receives)
	assert.True(t, slices.ContainsFunc(receives, func(r rpc.MessageReceiveDecl) bool {
		return r.Message == bgpevents.Namespace+":"+bgpevents.EventUpdate &&
			len(r.Tags) == 1 &&
			r.Tags[0].Name == events.TagDirection &&
			slices.Equal(r.Tags[0].Values, []string{events.DirectionReceived})
	}), "positive control: bgp-rs must still receive ordinary received updates")
	filteredMsg := bgpevents.Namespace + ":" + bgpevents.EventUpdateFiltered
	assert.False(t, slices.ContainsFunc(receives, func(r rpc.MessageReceiveDecl) bool {
		return r.Message == filteredMsg
	}), "bgp-rs must not declare reception of %s", filteredMsg)
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
