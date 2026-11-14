// Design: docs/architecture/behavior/peer-lifecycle.md -- what the End-of-RIB
// marker orders, and what it does not: the marker waits for no process.
// Related: test/plugin/initial-sync-marker-beside-process-route.ci -- the peer
// that requires this route and ze's marker, neither first.
// Related: register_initial_sync_barrier.go -- initialSyncBarrierRaw, the same
// lingering-peer shape for a raw injection beside the marker.
// Related: plugin_fixture_10.go -- fixture10WaitEOR, the marker counter barrier.
package fixture

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/initial-sync-marker-beside-process-route", markerBesideProcessRoute, StopsDaemon)
}

// markerBesideProcessRoute is a route-pushing process that pushes one route and
// nothing else, as soon as every plugin is ready. It never waits for the
// session or for ze's End-of-RIB before the push: the push races the session's
// initial sync on purpose, so the route reaches the peer before or after the
// marker as timing decides, and the .ci beside it accepts either order.
//
// It ends the test itself. The peer lingers once both frames have arrived, so
// the peer's completion cannot stop the daemon while this driver still has an
// engine call in flight; the runner stops the daemon on its peers' exit only
// for a fixture that does not declare StopsDaemon (runOrchestrated). After the
// push the driver waits for ze's eor-sent counter, which only a marker written
// to the socket raises, then returns: Observe quiesces, so a route pushed after
// the sync left the forward pool, and requests the shutdown that closes the
// lingering peer's session. A route pushed during the sync drained before the
// marker, so both frames are on the wire before the shutdown.
func markerBesideProcessRoute(ctx context.Context, _ []string) error {
	return observe09(ctx, "announce-routes", func(ctx context.Context, p *sdk.Plugin) error {
		const announce = "update text origin igp local-preference 200 nhop 10.0.1.254 nlri add 10.0.1.0/24"
		announced, withdrawn, err := p.UpdateRoute(ctx, "*", announce)
		if err != nil {
			return fmt.Errorf("update %q: %w", announce, err)
		}
		if announced+withdrawn == 0 {
			return errors.New("the announce acknowledged no routes")
		}
		return fixture10WaitEOR(ctx, p, "peer1", 60)
	})
}
