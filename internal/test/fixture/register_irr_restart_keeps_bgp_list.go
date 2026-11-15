// Design: docs/architecture/irr-owner.md -- a restart enforces the stored BGP IRR list at once
// Related: irr_owner_fixture.go -- the restart observers and helpers this one shares
// Related: fixture.go -- observeConfigured owns startup, assertion reporting and shutdown
package fixture

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/irr-restart-keeps-bgp-list", irrRestartKeepsBGPList, StopsDaemon)
}

// irrRestartKeepsBGPList observes test/plugin/irr-restart-keeps-bgp-list.ci
// (AC-1). Its goal is that a restart enforces the stored list at once. The
// first run waits for the stored dataset, as every restart test does. The
// restart requires `show bgp irr` to report the stored AS-TEST enforced
// (status ok, the mock's IPv4 count), the peer's UPDATE announcing a listed
// prefix to arrive accepted, and then the checks every restart test makes
// (same refreshed-at, no IRR query). The .ci refuses the filter's reject line
// in the restart, which is how a hold starts, so the UPDATE was accepted with
// no hold.
func irrRestartKeepsBGPList(ctx context.Context, _ []string) error {
	const name = "plugin/irr-restart-keeps-bgp-list"
	updates := make(chan string, 4)
	setup := func(p *sdk.Plugin) error {
		p.OnEvent(func(event string) error {
			var decoded map[string]any
			if json.Unmarshal([]byte(event), &decoded) != nil {
				return nil //nolint:nilerr // a malformed event is skipped, and failing the handler would end the session
			}
			bgp, _ := decoded["bgp"].(map[string]any)
			message, _ := bgp["message"].(map[string]any)
			if message["direction"] != directionReceived {
				return nil
			}
			update, _ := bgp["update"].(map[string]any)
			if _, hasNLRI := update["nlri"]; !hasNLRI {
				return nil
			}
			select {
			case updates <- text07(update):
			default:
			}
			return nil
		})
		p.SetStartupSubscriptions([]string{eventUpdate}, nil, "parsed")
		return nil
	}
	scenario := func(ctx context.Context, p *sdk.Plugin) error {
		first, restart, err := irrReadFirstRun()
		if err != nil {
			return err
		}
		if !restart {
			return irrFirstRunScenario(ctx, p, "show bgp irr", false)
		}
		if err := irrBGPEnforcesStored(ctx, p); err != nil {
			return fmt.Errorf("restart: %w", err)
		}
		if err := irrListedUpdateAccepted(ctx, updates); err != nil {
			return fmt.Errorf("restart: %w", err)
		}
		return irrSecondRunScenario(ctx, p, "show bgp irr", first)
	}
	return observeConfigured(ctx, name, &sdk.Registration{}, setup, scenario)
}
