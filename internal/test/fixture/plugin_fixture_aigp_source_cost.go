// Design: docs/architecture/route-selection.md -- AIGP session policy.
// Related: test/draft/plugin/aigp-source-cost-recovery.ci -- recipient wire assertions.
package fixture

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	registerPlugin08("plugin/aigp-source-cost-recovery", "aigp-cost-test", aigpSourceCostRecovery)
}

// aigpSourceCostRecovery changes the engine's real metric through the external
// plugin RPC. Only the recipient's byte assertions publish the two receipt
// files; sent counters are used solely to establish the initial-session barrier.
func aigpSourceCostRecovery(ctx context.Context, p *sdk.Plugin) error {
	const ready = "aigp-source-cost.ready"
	if !waitPeerCounter08(ctx, p, "127.0.0.2", "eor-sent", 1, 40) {
		return fmt.Errorf("AIGP recipient never completed initial sync")
	}
	if _, err := requireDone08(ctx, p, "request quiesce"); err != nil {
		return err
	}
	if err := os.WriteFile(ready, []byte("recipient ready"), 0o600); err != nil {
		return err
	}
	defer os.Remove(ready) //nolint:errcheck // private fixture marker removed again with the work directory
	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		_, err := os.Stat("aigp-source-cost.withheld")
		return err == nil
	}) {
		return fmt.Errorf("recipient did not acknowledge AIGP107 and the unknown-cost withdrawal")
	}
	installed, err := p.RouteInstall(ctx, []rpc.RouteInstallEntry{{
		Protocol: "ospf", AFI: 1, SAFI: 1, Prefix: "198.18.233.1/32", AdminDistance: 110, Metric: 11,
	}})
	if err != nil {
		return fmt.Errorf("install AIGP next-hop distance through route RPC: %w", err)
	}
	if installed != 1 {
		return fmt.Errorf("route RPC installed %d next-hop metrics, want 1", installed)
	}
	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		_, err := os.Stat("aigp-source-cost.recovered")
		return err == nil
	}) {
		return fmt.Errorf("recipient did not acknowledge AIGP111 after metric RPC without a source UPDATE")
	}
	fmt.Fprintln(os.Stderr, "AIGP source7 destination43 produced107; metric RPC recovered withheld route as111")
	return nil
}
