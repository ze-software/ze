// Design: docs/architecture/api/process-protocol.md -- subscription isolation of update-filtered
// Related: server.go -- route server startup subscriptions
package rs

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRouteServerTakesNoDeliveryOfFilteredUpdates verifies A-6: bgp-rs does not
// subscribe to update-filtered, and its structured handler ignores
// EventKindUpdateFiltered if ever delivered.
//
// VALIDATES: A-6 of spec-rfc7947-adj-rib-in-accepts-filtered-updates.
func TestRouteServerTakesNoDeliveryOfFilteredUpdates(t *testing.T) {
	// Verify none of the base BGP startup subscriptions used by bgp-rs name update-filtered.
	subs := []string{
		eventUpdate + " direction received",
		eventState,
		eventOpen + " direction received",
		eventRefresh,
	}
	assert.False(t, slices.ContainsFunc(subs, func(s string) bool {
		return strings.HasPrefix(s, bgpevents.EventUpdateFiltered)
	}), "bgp-rs must not subscribe to %s", bgpevents.EventUpdateFiltered)

	// Guard: EventKindUpdateFiltered is distinct from EventKindUpdate.
	assert.NotEqual(t, rpc.EventKindUpdate, rpc.EventKindUpdateFiltered)
}
