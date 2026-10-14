// Design: docs/architecture/testing/ci-format.md -- compiled plugin fixtures
// Related: plugin_fixture_02.go -- observer02, eorSent02, the observer pattern this reuses
// Related: plugin_fixture_13.go -- routeServerReplay13, the shutdown-after-forward driver

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/srv6-invalid-sid-ineligible", observer02("srv6-ineligible-test", srv6InvalidSIDIneligible02))
	Register("plugin/prefixsid-malformed-receive", routeServerReplay13("shutdown-after-forward", "10.0.7.0/24", false))
}

// srv6InvalidSIDIneligible02 checks RFC 9252 Section 7 from the RIB: the
// peer sent three IPv4 VPN paths, P1 (10.0.1.0/24, an invalid SID Structure),
// P2 (10.0.2.0/24, a 24-bit transposition into the 20-bit label) and P3
// (10.0.3.0/24, a valid 20-bit transposition, the only one with LOCAL_PREF
// 300). Once the RIB stores all three, the best table must hold one IPv4 VPN
// path, P3. Paths are counted by family and P3 is named by its LOCAL_PREF, so
// the check reads nothing from how a VPN prefix is rendered.
func srv6InvalidSIDIneligible02(ctx context.Context, plugin *sdk.Plugin) error {
	if !eorSent02(ctx, plugin, "peer1", 1, 40) {
		return fmt.Errorf("ze never sent its End-of-RIB to peer1")
	}
	const vpn = `"family":"ipv4/mpls-vpn"`
	stored := ""
	if !Poll(ctx, 40, plugin01PollDelay, func() bool {
		raw, err := requireDone02(ctx, plugin, "show bgp rib")
		if err != nil {
			return false
		}
		stored = string(raw)
		return strings.Count(stored, vpn) >= 3
	}) {
		return fmt.Errorf("the RIB never stored the three IPv4 VPN paths: %.600s", stored)
	}
	raw, err := requireDone02(ctx, plugin, "show bgp rib best")
	if err != nil {
		return fmt.Errorf("show bgp rib best: %w", err)
	}
	best := string(raw)
	if n := strings.Count(best, vpn); n != 1 {
		return fmt.Errorf("%d IPv4 VPN best paths, want 1 (P3 only; P1 and P2 hold no valid SRv6 SID): %.800s", n, best)
	}
	if !strings.Contains(best, `"value":300`) {
		return fmt.Errorf("the IPv4 VPN best path is not P3 (LOCAL_PREF 300): %.800s", best)
	}
	fmt.Fprintln(os.Stderr, "OK: srv6 ineligible: of three stored paths only P3 is selected")
	return nil
}
