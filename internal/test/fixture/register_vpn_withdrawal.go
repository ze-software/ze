// Design: docs/guide/route-reflection.md -- VPN withdrawal inventory lifecycle.
// RFC 8277 Section 2.4 -- see rfc/short/rfc8277.md.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"time"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/vpn-withdrawal-inventory", vpnWithdrawalInventory)
}

// vpnWithdrawalInventory releases each source action only after the destination
// was sent the preceding UPDATE. Exact route assertions live in the peer script.
func vpnWithdrawalInventory(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("vpn-withdrawal-inventory requires rs or rr")
	}
	role := args[0]
	if role != "rs" {
		if role != "rr" {
			return fmt.Errorf("unsupported inventory role %q", role)
		}
	}
	peerCommand := "show bgp rs peers"
	if role == "rr" {
		peerCommand = "show rr peers"
	}
	return runPlugin13(ctx, "vpn-withdrawal", nil, func(ctx context.Context, plugin *sdk.Plugin) error {
		if !Poll(ctx, 100, 250*time.Millisecond, func() bool {
			result := command13(ctx, plugin, peerCommand)
			if !done13(result) {
				return false
			}
			rows, ok := result.object()["peers"].([]any)
			if !ok {
				return false
			}
			up := make(map[string]bool)
			for _, value := range rows {
				row, ok := value.(map[string]any)
				if !ok {
					return false
				}
				address, _ := row["address"].(string)
				up[address], _ = row["up"].(bool)
			}
			return up["127.0.0.1"] && up["127.0.0.2"]
		}) {
			return errors.New("inventory owner never reported both VPN peers up")
		}
		for index, prefix := range []string{"192.0.2.0/24", prefixTestNet2, "203.0.113.0/24"} {
			if index > 0 {
				if !vpnWithdrawalWaitSent(ctx, plugin, index) {
					return fmt.Errorf("receiver was not sent VPN UPDATE %d", index)
				}
			}
			command := "update text origin igp nhop 1.1.1.1 nlri ipv4/unicast add " + prefix
			if _, _, err := plugin.UpdateRoute(ctx, "127.0.0.1", command); err != nil {
				return err
			}
		}
		if !vpnWithdrawalWaitSent(ctx, plugin, 4) {
			return errors.New("receiver was not sent both retained VPN withdrawals after source departure")
		}
		return nil
	}, true)
}

func vpnWithdrawalWaitSent(ctx context.Context, plugin *sdk.Plugin, count int) bool {
	return Poll(ctx, 100, 250*time.Millisecond, func() bool {
		result := command13(ctx, plugin, "show bgp peer 127.0.0.2 detail")
		return peerCounter13(result, "updates-sent")-peerCounter13(result, "eor-sent") >= count
	})
}
