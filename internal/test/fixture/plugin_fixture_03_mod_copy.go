// Design: docs/architecture/bgp/fanout-dedup.md — cached forwarding copy isolation.
// Related: plugin_fixture_03.go — fixture registration and peer-detail decoding.
package fixture

import (
	"context"
	"fmt"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// bgpRSModCopy03 releases the source only after every session's initial sync,
// then waits for both intended recipients rather than counting any peer's EOR.
func bgpRSModCopy03(ctx context.Context, p *sdk.Plugin) error {
	if !Poll(ctx, 32, 200*time.Millisecond, func() bool {
		rows, err := peerRows03(ctx, p, "")
		if err != nil {
			return false
		}
		for _, address := range []string{"127.0.0.1", addrLoopbackSecond, "127.0.0.3"} {
			if number03(rows[address]["eor-sent"]) < 1 {
				return false
			}
		}
		return true
	}) {
		return fmt.Errorf("mod-copy: source, edited recipient and internal control must each send EOR")
	}

	// This fence is addressed only to the source, never to either counted
	// recipient. The peer sends the subjects only after receiving this route.
	if _, _, err := p.UpdateRoute(ctx, "127.0.0.1", "update text origin igp nhop 1.1.1.1 nlri ipv4/unicast add 192.0.2.0/24"); err != nil {
		return fmt.Errorf("mod-copy: release source after initial sync: %w", err)
	}
	if !Poll(ctx, 32, 200*time.Millisecond, func() bool {
		rows, err := peerRows03(ctx, p, "")
		if err != nil {
			return false
		}
		for _, address := range []string{addrLoopbackSecond, "127.0.0.3"} {
			row := rows[address]
			if number03(row["updates-sent"])-number03(row["eor-sent"]) < 2 {
				return false
			}
		}
		return true
	}) {
		return fmt.Errorf("mod-copy: edited recipient and internal control must each receive both non-EOR UPDATEs")
	}
	if _, _, err := p.UpdateRoute(ctx, "127.0.0.1", "update text origin igp nhop 1.1.1.1 nlri ipv4/unicast add 192.0.3.0/24"); err != nil {
		return fmt.Errorf("mod-copy: complete source after recipient delivery: %w", err)
	}
	return nil
}
