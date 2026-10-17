package fixture

import (
	"context"
	"fmt"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// bgpRSFastpathReplayPrefix03 is the route bgp-rs-fastpath-replay.ci's peer
// announces before the observer enables the fast path.
const bgpRSFastpathReplayPrefix03 = "10.20.30.0/24"

// bgpRSFastpathReplay03 enables the fast path only once the peer's route is in
// the Loc-RIB, read through `show bgp rib rpf`, which answers from the Loc-RIB
// itself. Enable rebuilds the tracker's state from the Loc-RIB before it
// answers, so the enable answer itself must count that route.
func bgpRSFastpathReplay03(ctx context.Context, p *sdk.Plugin) error {
	var last any
	if !Poll(ctx, 60, 200*time.Millisecond, func() bool {
		value, err := done03(ctx, p, "show bgp rib rpf ipv4/unicast 10.20.30.1")
		if err != nil {
			last = err
			return false
		}
		last = value
		row, ok := value.(map[string]any)
		return ok && row["matched-prefix"] == bgpRSFastpathReplayPrefix03
	}) {
		return fmt.Errorf("the peer's route never reached the Loc-RIB, last answer: %v", last)
	}
	value, err := done03(ctx, p, "request bgp rib fastpath enable")
	if err != nil {
		return err
	}
	status, ok := value.(map[string]any)
	if !ok || status["enabled"] != true {
		return fmt.Errorf("fast path not enabled: %v", value)
	}
	if number03(status["prefixes"]) != 1 {
		return fmt.Errorf("enable recorded %v prefixes, want the 1 route learned before it: %v", status["prefixes"], value)
	}
	if number03(status["replayed"]) != 1 {
		return fmt.Errorf("enable replayed %v routes from the Loc-RIB, want 1: %v", status["replayed"], value)
	}
	return nil
}
