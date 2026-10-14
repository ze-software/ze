// Design: docs/guide/flowspec-route-reflector.md -- the families key of the peer rows.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/peer-families-show", peerFamiliesShow)
}

// commandShowRSPeers is the route server's peer command.
const commandShowRSPeers = "show bgp rs peers"

// peerFamiliesShowPeer is the ze-peer session the observer reads.
const peerFamiliesShowPeer = "127.0.0.1"

// peerFamiliesShowCommands maps the role argument to the plugin's peer command.
var peerFamiliesShowCommands = map[string]string{
	"rr": "show rr peers",
	"rs": commandShowRSPeers,
}

// peerFamiliesShow polls the rr or rs peer command until the 127.0.0.1 row is
// up and carries a families field, then passes when that field is exactly
// ipv6/unicast and fails naming what it read otherwise. A row that is up
// without the field keeps the poll going, so the verdict does not rest on the
// order of the OPEN and the state-up events.
func peerFamiliesShow(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("peer-families-show requires rs or rr")
	}
	peerCommand, known := peerFamiliesShowCommands[args[0]]
	if !known {
		return fmt.Errorf("unsupported peer-families-show role %q", args[0])
	}
	return runPlugin13(ctx, "peer-families-show", nil, func(ctx context.Context, plugin *sdk.Plugin) error {
		var families []any
		var last any
		found := Poll(ctx, 120, 250*time.Millisecond, func() bool {
			result := command13(ctx, plugin, peerCommand)
			if !done13(result) {
				return false
			}
			rows, ok := result.object()["peers"].([]any)
			if !ok {
				return false
			}
			for _, value := range rows {
				row, ok := value.(map[string]any)
				if !ok {
					continue
				}
				if address, _ := row["address"].(string); address != peerFamiliesShowPeer {
					continue
				}
				last = row
				if up, _ := row["up"].(bool); !up {
					return false
				}
				list, present := row["families"].([]any)
				if !present {
					return false
				}
				families = list
				return true
			}
			return false
		})
		if !found {
			return fmt.Errorf("%s: the %s row never came up with a families field (last row %v)", peerCommand, peerFamiliesShowPeer, last)
		}
		if len(families) != 1 {
			return fmt.Errorf("%s: families %v, want [ipv6/unicast]", peerCommand, families)
		}
		if name, _ := families[0].(string); name != "ipv6/unicast" {
			return fmt.Errorf("%s: families %v, want [ipv6/unicast]", peerCommand, families)
		}
		fmt.Fprintln(os.Stderr, "PASS:", peerCommand, "families ipv6/unicast")
		return nil
	}, true)
}
