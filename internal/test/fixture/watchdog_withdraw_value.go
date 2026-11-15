// Design: docs/architecture/testing/ci-format.md -- state-based watchdog wire proof
// Related: plugin_fixture_11.go -- runPlugin11 keeps a route driver connected until its peer completes

package fixture

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/watchdog-withdraw-value", watchdogWithdrawValue)
}

// watchdogWithdrawValue waits for the ordinary and initially announced routes
// before releasing the held pool. The scripted peer judges every route on wire.
func watchdogWithdrawValue(ctx context.Context, _ []string) error {
	return runPlugin11(ctx, "service-watchdog", nil, func(ctx context.Context, p *sdk.Plugin) error {
		if !peerDetailReady(ctx, p, 40, "eor-sent", func(value any) bool { return number09(value) >= 1 }) {
			return errors.New("watchdog value proof: initial End-of-RIB was not sent")
		}
		// group-updates is disabled in the CI: three data UPDATEs and EOR.
		if !peerDetailReady(ctx, p, 40, "updates-sent", func(value any) bool { return number09(value) >= 4 }) {
			return errors.New("watchdog value proof: the ordinary, omitted and false startup routes were not sent")
		}
		if err := quiesce11(ctx, p); err != nil {
			return err
		}
		for range 3 {
			for _, command := range []string{cmdWatchdogAnnounceDNSR, cmdWatchdogWithdrawDNSR} {
				status, _, err := p.DispatchCommand(ctx, command)
				if err != nil {
					return fmt.Errorf("watchdog value proof %s: %w", command, err)
				}
				if status != statusDone {
					return fmt.Errorf("watchdog value proof %s returned %q, want done", command, status)
				}
				if err := quiesce11(ctx, p); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
