// Design: docs/architecture/testing/ci-format.md -- compiled .ci fixture helpers
// Related: plugin_fixture_14.go -- plugin14PollCommand, plugin14Quiesce, plugin14PeerRows

package fixture

import (
	"context"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/rfc7999-blackhole-readvertise-agreement", plugin14Observer("test-blackhole-agreement", blackholeReadvertiseFence))
}

// blackholeReadvertiseSessions is the number of sessions the RFC 7999
// re-advertisement scenario opens: one source and five destinations.
const blackholeReadvertiseSessions = 6

// blackholeReadvertiseFence holds the daemon until every session has its
// initial-sync End-of-RIB, the source's two UPDATEs (the BLACKHOLE-tagged route
// and the untagged fence) have reached ze, and the forwarding decision for both
// has been made. The wire verdict belongs to the lingering ze-peers, which
// never exit by themselves; returning here makes Observe request the clean
// shutdown that ends the test. Without the fence the daemon would stop before
// the egress gate ran, and every destination's expectation would be vacuous.
func blackholeReadvertiseFence(ctx context.Context, plugin *sdk.Plugin) error {
	_, _, err := plugin14PollCommand(ctx, plugin, 100, "show bgp peer * detail", func(status string, value any) bool {
		if status != rpc.StatusDone {
			return false
		}
		rows := plugin14PeerRows(plugin14Map(value))
		synced := 0
		for _, row := range rows {
			if plugin14Number(plugin14Map(row)["eor-sent"]) >= 1 {
				synced++
			}
		}
		if synced < blackholeReadvertiseSessions {
			return false
		}
		return plugin14Number(plugin14Map(rows["127.0.0.1"])["updates-received"]) >= 2
	})
	if err != nil {
		return fmt.Errorf("the source's two UPDATEs never reached a fully synced daemon: %w", err)
	}
	if err := plugin14Quiesce(ctx, plugin); err != nil {
		return fmt.Errorf("forwarding decision not yet made: %w", err)
	}
	return nil
}
