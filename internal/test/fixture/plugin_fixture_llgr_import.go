// Design: docs/guide/graceful-restart.md -- NO_LLGR is evaluated after import policy.
package fixture

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/llgr-import-no-llgr", llgrImportNoLLGR)
}

func llgrImportNoLLGR(ctx context.Context, _ []string) error {
	// The .ci stderr fence owns successful shutdown; a second stop from the
	// observer would race the runner's signal and force an unclean exit.
	return runPlugin13(ctx, "observer", nil, func(ctx context.Context, p *sdk.Plugin) error {
		count := func(filter string, want int) bool {
			r := command13(ctx, p, "show bgp rib received "+filter+" count")
			got, ok := countFrom13(r)
			return done13(r) && ok && got == want
		}
		if !Poll(ctx, 60, 100*time.Millisecond, func() bool {
			return count("prefix 10.0.0.0/24 community 65535:7", 1) &&
				count("prefix 10.0.1.0/24", 1) &&
				count("prefix 10.0.1.0/24 community 65535:7", 0)
		}) {
			return fmt.Errorf("import policy did not add NO_LLGR to exactly the marked source route")
		}
		// A marker on each source's real TCP stream releases its close action
		// only AFTER the policy-mutated RIB state was observed. No timer or
		// sleep stands in for delivery of the original UPDATEs.
		for _, peer := range []string{"marked", "plain"} {
			r := command13(ctx, p, "send bgp "+peer+" update text origin igp local-preference 100 nhop 1.1.1.1 nlri ipv4/unicast add 198.51.100.0/24")
			if !done13(r) {
				return fmt.Errorf("release source %s: %s", peer, r.text())
			}
		}
		if !Poll(ctx, 120, 100*time.Millisecond, func() bool {
			return count("prefix 10.0.0.0/24", 0) &&
				count("prefix 10.0.1.0/24 community 65535:6", 1)
		}) {
			return fmt.Errorf("LLGR did not remove policy-added NO_LLGR while retaining and marking the unmodified control: %s", command13(ctx, p, "show bgp rib received").text())
		}
		fmt.Fprintln(os.Stderr, "OK: import-added NO_LLGR removed on LLGR entry; unmarked control retained with LLGR_STALE")
		return nil
	}, false)
}
