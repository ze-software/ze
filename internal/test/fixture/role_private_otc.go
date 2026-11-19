// Design: docs/guide/bgp-role.md -- public egress refuses Private Use OTC without rewriting it.
// Related: plugin_fixture_14.go -- shared peer observations and quiescence.

package fixture

import (
	"context"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/role-private-otc-boundary", plugin14Observer("test-private-otc", rolePrivateOTCBoundary), StopsDaemon)
}

// rolePrivateOTCBoundary keeps both sessions established until the two source
// updates have been processed. The peer script checks the public OTC's exact
// wire and rejects either NLRI form of the private-OTC route. This observer
// independently requires both received routes and exactly one forwarded update.
func rolePrivateOTCBoundary(ctx context.Context, plugin *sdk.Plugin) error {
	if err := p12WaitPeersEOR(ctx, plugin, "peer1", "dest-peer"); err != nil {
		return err
	}
	_, _, err := plugin14PollCommand(ctx, plugin, 60, "show bgp peer * detail", func(status string, value any) bool {
		if status != rpc.StatusDone {
			return false
		}
		rows := plugin14PeerRows(plugin14Map(value))
		source := plugin14Map(rows["127.0.0.1"])
		return plugin14Number(source["updates-received"]) >= 2
	})
	if err != nil {
		return fmt.Errorf("the two public/private OTC inputs did not arrive: %w", err)
	}
	if err := plugin14Quiesce(ctx, plugin); err != nil {
		return fmt.Errorf("the OTC forwarding decisions did not settle: %w", err)
	}
	_, detail, err := plugin14DispatchMap(ctx, plugin, "show bgp peer * detail")
	if err != nil {
		return err
	}
	rows := plugin14PeerRows(detail)
	dest := plugin14Map(rows["127.0.0.2"])
	updates := plugin14Number(dest["updates-sent"]) - plugin14Number(dest["eor-sent"])
	if updates != 1 {
		return fmt.Errorf("public egress sent %d route updates, want exactly the public-OTC control", updates)
	}
	status, adj, err := plugin14DispatchMap(ctx, plugin, "show bgp adj-rib-in status")
	if err != nil {
		return err
	}
	if status != rpc.StatusDone {
		return fmt.Errorf("cannot observe the two accepted source routes: status %q", status)
	}
	if total := plugin14Number(adj["total-routes"]); total != 2 {
		return fmt.Errorf("Adj-RIB-In holds %d routes, want both OTC inputs before egress", total)
	}
	return nil
}
