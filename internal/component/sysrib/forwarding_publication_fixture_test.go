// Design: docs/architecture/rib/unified-locrib.md -- publication order for the forwarding fixtures.
// Related: forwarding_order.go -- orderForwarding, the stage every publication runs.
// Related: route_view_worker_output.go -- publish, the worker's only route to the output.

package sysrib

import (
	"net/netip"
	"sync/atomic"
	"testing"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/replay"
)

// forwardingBus is the bus the forwarding fixtures record publications on. The
// daemon's output emits on the plugin server's bus (routeViewAuthority.publish),
// so production holds no bus of its own; a test installs one with
// useForwardingBus. With none installed a publication is ordered and dropped,
// which is what a test that reads no bus asks for.
var forwardingBus atomic.Pointer[testEventBus]

// useForwardingBus installs bus for the calling test and removes it when the
// test ends. A test that installs one MUST NOT run in parallel with another.
func useForwardingBus(t *testing.T, bus *testEventBus) {
	t.Helper()
	forwardingBus.Store(bus)
	t.Cleanup(func() { forwardingBus.Store(nil) })
}

// publishForwarding is one consumer step's publication. It runs the changes
// through orderForwarding, the stage worker.publish runs on every live,
// cascade, configuration and replay publication, and records each ordered batch
// on the test's bus with the typed handle the daemon output emits with. It
// stands in only for the acknowledged transport between the two (publishBatch
// and routeViewAuthority.publish), which the route_view_runtime,
// route_view_publication_order and locrib_feed tests drive. A replayID of zero
// is live.
func publishForwarding(t *testing.T, changes map[family.Family][]outgoingChange, replayID uint64) {
	t.Helper()
	batches, err := orderForwarding(changes, replayID)
	if err != nil {
		t.Fatalf("order the publication: %v", err)
	}
	bus := forwardingBus.Load()
	if bus == nil {
		return
	}
	for i := range batches {
		if _, err := sysribevents.BestChange.Emit(bus, &batches[i]); err != nil {
			t.Fatalf("record the publication: %v", err)
		}
	}
}

// publishFamily publishes one family's changes as one live publication.
func publishFamily(t *testing.T, fam family.Family, changes []outgoingChange) {
	t.Helper()
	publishForwarding(t, map[family.Family][]outgoingChange{fam: changes}, 0)
}

// publishCascade publishes the cascade the worker's apply computes when the
// covering routes of nhs change.
func (s *sysRIB) publishCascade(t *testing.T, nhs []netip.Addr) {
	t.Helper()
	publishForwarding(t, s.cascadeChanges(nhs), 0)
}

// publishPermissions installs permit and publishes the sweep it owes, as the
// worker's configure collects and flushes it.
func (s *sysRIB) publishPermissions(t *testing.T, permit map[string]bool) {
	t.Helper()
	publishForwarding(t, s.applyFIBImport(permit), 0)
}

// orderedReplay answers the batches worker.replay publishes for replayID.
func (s *sysRIB) orderedReplay(t *testing.T, replayID uint64) []outgoingBatch {
	t.Helper()
	batches, err := orderForwarding(s.replayChanges(), replayID)
	if err != nil {
		t.Fatalf("order the replay: %v", err)
	}
	return batches
}

// publishReplay publishes the replay worker.replay emits for a broadcast
// request, the only replay identity the daemon admits.
func (s *sysRIB) publishReplay(t *testing.T) {
	t.Helper()
	publishForwarding(t, s.replayChanges(), replay.Broadcast)
}
