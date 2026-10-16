package fixture

// plugin_fixture_pool_stats.go asks `show metrics pool` from inside a running
// daemon and checks the rows against the one declaration of the pools.
//
// Design: docs/architecture/pool-architecture.md -- pool-stats, the operator view of the attribute pools
// Related: register_pool_stats.go -- the scenario name
// Related: test/plugin/bgp-pool-stats.ci -- the run
// Related: plugin_fixture_04.go -- observe04, command04, waitPeerEOR04 and number04

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// poolStats proves an operator reads every attribute pool under its own name.
// The expected names come from pool.AllNamedPools, the list the handler reads,
// so a pool added there is expected here with no edit; a row named "unknown",
// a missing row, or a row out of order fails the run.
func poolStats(ctx context.Context, p *sdk.Plugin) error {
	// The peer block expects an End-of-RIB; returning before it is sent tears
	// the session down and fails the peer, not this scenario.
	if err := waitPeerEOR04(ctx, p); err != nil {
		return err
	}
	status, value, err := command04(ctx, p, "show metrics pool")
	if err != nil {
		return err
	}
	if status != statusDone {
		return fmt.Errorf("show metrics pool: status %q, answer %v", status, value)
	}
	data, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("show metrics pool: answer is %T, not an object", value)
	}
	rows, ok := data["pools"].([]any)
	if !ok {
		return fmt.Errorf("show metrics pool: no pools list in %v", data)
	}
	want := pool.AllNamedPools()
	if len(rows) != len(want) {
		return fmt.Errorf("show metrics pool: %d rows, %d pools declared", len(rows), len(want))
	}
	if int(number04(data["count"])) != len(rows) {
		return fmt.Errorf("show metrics pool: count %v, %d rows", data["count"], len(rows))
	}
	for i, raw := range rows {
		row, isRow := raw.(map[string]any)
		if !isRow {
			return fmt.Errorf("show metrics pool: row %d is %T", i, raw)
		}
		name, _ := row["name"].(string)
		if name != want[i].Name {
			return fmt.Errorf("show metrics pool: row %d named %q, want %q", i, name, want[i].Name)
		}
		if _, hasRate := row["dedup-rate"].(string); !hasRate {
			return errors.New("show metrics pool: row " + name + " has no dedup-rate")
		}
	}
	fmt.Fprintln(os.Stderr, "OK: show metrics pool named every pool")
	return nil
}
