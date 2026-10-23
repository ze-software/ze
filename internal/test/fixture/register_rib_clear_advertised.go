// Design: docs/architecture/api/commands.md -- clear bgp rib advertised
// Related: plugin_fixture_09.go -- observe09, dispatch09, the helpers this driver uses

package fixture

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/rib-clear-advertised", ribClearAdvertised)
}

// ribClearAdvertisedRoute is the one route the driver announces and then
// clears; test/plugin/rib-clear-advertised.ci holds its UPDATE and its
// withdrawal as the peer receives them.
const ribClearAdvertisedRoute = "origin igp local-preference 200 nhop 10.0.1.254 nlri add 10.0.1.0/24"

var errRibClearAdvertisedBare = errors.New("clear bgp rib advertised with no selector was accepted, and it must be refused")

// ribClearAdvertised drives `clear bgp rib advertised` through the command
// dispatcher, the operator's entry point. It announces one route, waits for
// it to reach the wire, then checks three answers: the bare command is
// refused (the selector is mandatory), `clear bgp rib advertised *` withdraws
// exactly that route, and a second clear withdraws nothing because the
// Adj-RIB-Out is now empty. The peer side of the .ci asserts the withdrawal
// UPDATE itself, so a clear that answered done without writing it fails there.
// observe09 then asks the daemon to shut down, and a failed check is reported
// with the ZE-OBSERVER-FAIL sentinel the .ci rejects.
func ribClearAdvertised(ctx context.Context, _ []string) error {
	return observe09(ctx, "rib-clear-advertised", func(ctx context.Context, p *sdk.Plugin) error {
		announced, _, err := p.UpdateRoute(ctx, "*", "update text "+ribClearAdvertisedRoute)
		if err != nil {
			return fmt.Errorf("announce %q: %w", ribClearAdvertisedRoute, err)
		}
		if announced != 1 {
			return fmt.Errorf("announce %q acknowledged %d routes, want 1", ribClearAdvertisedRoute, announced)
		}
		if quiesced := dispatch09(ctx, p, "request quiesce"); !done09(quiesced) {
			return fmt.Errorf("quiesce after the announce: status=%s data=%s", quiesced.status, quiesced.text)
		}

		if bare := dispatch09(ctx, p, "clear bgp rib advertised"); done09(bare) {
			return fmt.Errorf("%w: data=%s", errRibClearAdvertisedBare, bare.text)
		}

		if err := ribClearAdvertisedExpect(ctx, p, 1); err != nil {
			return err
		}
		if quiesced := dispatch09(ctx, p, "request quiesce"); !done09(quiesced) {
			return fmt.Errorf("quiesce after the clear: status=%s data=%s", quiesced.status, quiesced.text)
		}
		return ribClearAdvertisedExpect(ctx, p, 0)
	})
}

// ribClearAdvertisedExpect runs `clear bgp rib advertised *` and checks the
// count of withdrawn routes it answers.
func ribClearAdvertisedExpect(ctx context.Context, p *sdk.Plugin, want int) error {
	cleared := dispatch09(ctx, p, "clear bgp rib advertised *")
	if !done09(cleared) {
		return fmt.Errorf("clear bgp rib advertised *: status=%s data=%s", cleared.status, cleared.text)
	}
	withdrawn, present := map09(cleared.data)["withdrawn"]
	if !present {
		return fmt.Errorf("clear bgp rib advertised * answered no withdrawn count: data=%s", cleared.text)
	}
	if got := number09(withdrawn); got != want {
		return fmt.Errorf("clear bgp rib advertised * withdrew %d routes, want %d: data=%s", got, want, cleared.text)
	}
	return nil
}
