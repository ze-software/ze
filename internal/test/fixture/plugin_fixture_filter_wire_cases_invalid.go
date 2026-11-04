// Design: docs/plugin-development/protocol.md -- The Sets of a filter-update answer.
// Related: ../plugins/filterwirecases/invalid.go -- the refused-set evaluator.

package fixture

import (
	"context"
	"fmt"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func filterWireCasesInvalidFence(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("filter-wire-cases-invalid-fence takes no arguments")
	}
	return Observe(ctx, "filter-wire-invalid-fence", &sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		if !Poll(ctx, 80, 100*time.Millisecond, func() bool { return everyPeerSentEndOfRIB(ctx, p, 1) }) {
			return fmt.Errorf("invalid-set source never reached its post-filter End-of-RIB")
		}
		if err := requireSeries04(ctx, p, []string{`ze_policy_route_set_refusals_total{filter="filter-wire-cases-forked:invalid"} 4`}); err != nil {
			return err
		}
		filterSubjectSaid("OK: refused sets counted exactly ", []byte("4"))
		return nil
	})
}
