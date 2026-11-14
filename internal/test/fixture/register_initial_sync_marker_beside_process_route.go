// Design: docs/architecture/behavior/peer-lifecycle.md -- what the End-of-RIB
// marker orders, and what it does not: the marker waits for no process.
// Related: test/plugin/initial-sync-marker-beside-process-route.ci -- the peer
// that requires this route and ze's marker, neither first.
// Related: internal/test/fixture/plugin_fixture_09.go -- passiveObserve09, the
// process shape this driver shares with the announce-withdraw drivers.
package fixture

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/initial-sync-marker-beside-process-route", markerBesideProcessRoute)
}

// markerBesideProcessRoute is a route-pushing process that pushes one route and
// nothing else, as soon as every plugin is ready, then drains the forward pool
// so the route is on its way before the driver returns. It never waits for the
// session or for ze's End-of-RIB: the push races the session's initial sync on
// purpose, so the route reaches the peer before or after the marker as timing
// decides, and the .ci beside it accepts either order.
func markerBesideProcessRoute(ctx context.Context, _ []string) error {
	return passiveObserve09(ctx, "announce-routes", &sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		const announce = "update text origin igp local-preference 200 nhop 10.0.1.254 nlri add 10.0.1.0/24"
		announced, withdrawn, err := p.UpdateRoute(ctx, "*", announce)
		if err != nil {
			return fmt.Errorf("update %q: %w", announce, err)
		}
		if announced+withdrawn == 0 {
			return errors.New("the announce acknowledged no routes")
		}
		if quiesced := dispatch09(ctx, p, "request quiesce"); !done09(quiesced) {
			return fmt.Errorf("quiesce after the announce: status=%s data=%s", quiesced.status, quiesced.text)
		}
		return nil
	})
}
