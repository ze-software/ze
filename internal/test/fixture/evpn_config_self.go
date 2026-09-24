// Design: docs/architecture/wire/nlri-evpn.md -- connected next-hop self origination.
// Related: test/plugin/evpn-config-self.ci -- independent collector wire checks.
package fixture

import (
	"context"
	"fmt"
	"os"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/evpn-config-self", func(ctx context.Context, _ []string) error {
		return Observe(ctx, "plugin/evpn-config-self", sdk.Registration{}, evpnConfigSelf)
	})
}

func evpnConfigSelf(ctx context.Context, p *sdk.Plugin) error {
	if err := waitEOR07(ctx, p, 1); err != nil {
		return err
	}
	row, err := peerRow07(ctx, p, "127.0.0.2")
	if err != nil {
		return err
	}
	// Initial sync writes the configured IMET before its EOR. The peer checks
	// the route's actual next hop and independent originator, then lingers until
	// this socket-write fence lets Observe request daemon shutdown.
	if number07(row["updates-sent"]) != 2 || number07(row["eor-sent"]) != 1 {
		return fmt.Errorf("native EVPN initial sync did not emit exactly IMET and EOR: %v", row)
	}
	fmt.Fprintln(os.Stderr, "OK native EVPN self route and EOR reached collector")
	return nil
}
