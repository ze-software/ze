// Design: docs/architecture/route-selection.md -- Step 0, stale depreference
// Related: plugin_fixture_02.go -- the registration, requireDone02, map02
// Related: ../../../test/plugin/llgr-stale-received-depreferenced.ci -- the scenario
// Related: terminal_cover_fixture.go -- terminalCovers, the next-hop topology

package fixture

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	llgrReceivedDevice   = "llgr-fixture0"
	llgrReceivedInstance = 9494
)

// llgrReceivedVerdict is what one prefix of the scenario must show.
type llgrReceivedVerdict struct {
	prefix string // the contested prefix
	winner string // the peer address that must be best
	step   string // the decision step that must decide it
}

// llgrStaleReceived02 checks RFC 9494 Section 4.3 through the daemon: a route
// carrying LLGR_STALE from the session whose OPEN carried the LLGR Capability
// loses Step 0 to a fresh route at a lower LOCAL_PREF, and the same community
// from the session whose OPEN did not carry it is not depreferenced, so its
// higher LOCAL_PREF wins.
func llgrStaleReceived02(ctx context.Context, plugin *sdk.Plugin) error {
	if err := llgrReceivedTopology(ctx, plugin); err != nil {
		return err
	}
	if err := p12WaitPeersEOR(ctx, plugin, "llgr", "plain"); err != nil {
		return err
	}
	verdicts := []llgrReceivedVerdict{
		{prefix: prefixTen8, winner: addrLoopbackV6, step: "stale-level"},
		{prefix: prefixTen1Of16, winner: addrLoopbackV6, step: stepLocalPreference},
	}
	entries := make(map[string]map[string]any, len(verdicts))
	if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
		raw, err := requireDone02(ctx, plugin, "show bgp rib best | reason")
		if err != nil {
			return false
		}
		data, err := map02(raw)
		if err != nil {
			return false
		}
		list, _ := data["best-path-reason"].([]any)
		for _, value := range list {
			entry, _ := value.(map[string]any)
			candidates, _ := entry["candidates"].([]any)
			prefix, _ := entry["prefix"].(string)
			if len(candidates) >= 2 {
				entries[prefix] = entry
			}
		}
		return len(entries) >= len(verdicts)
	}) {
		return fmt.Errorf("two candidates never stored for both prefixes, saw %v", entries)
	}
	for _, want := range verdicts {
		entry, ok := entries[want.prefix]
		if !ok {
			return fmt.Errorf("%s: no entry with two candidates, saw %v", want.prefix, entries)
		}
		if entry["winner-peer"] != want.winner {
			return fmt.Errorf("%s: best is %v, want %s: %v", want.prefix, entry["winner-peer"], want.winner, entry)
		}
		steps, _ := entry["steps"].([]any)
		if len(steps) == 0 {
			return fmt.Errorf("%s: no decision steps: %v", want.prefix, entry)
		}
		first, _ := steps[0].(map[string]any)
		if first["step"] != want.step {
			return fmt.Errorf("%s: decided by %v, want %s: %v", want.prefix, first["step"], want.step, entry)
		}
		fmt.Fprintf(os.Stderr, "OK: llgr received %s best %s by %s\n", want.prefix, want.winner, want.step)
	}
	return nil
}

// llgrReceivedTopology gives the unchanged wire next hops independent terminal
// paths in the real system RIB (terminalCovers).
func llgrReceivedTopology(ctx context.Context, p *sdk.Plugin) error {
	return terminalCovers{label: "LLGR", device: llgrReceivedDevice, instance: llgrReceivedInstance,
		addresses: []string{addrPeerOne, addrPeerTwo}}.install(ctx, p)
}
