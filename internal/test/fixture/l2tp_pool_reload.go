// Design: docs/functional-tests.md -- pool reload through SIGHUP and the daemon's real pool handlers
// Related: reload_fixture_live_stages.go -- the processed-reload generation fence

package fixture

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("reload/l2tp-pool-range", l2tpPoolReload, StopsDaemon)
}

// l2tpPoolReload proves the operator's committed range reaches allocation:
// one subscriber consumes the boot range, the reload widens it, and another
// subscriber receives the added address while the first reservation survives.
// Equal numeric transport pairs then retain independent owners through another
// pool edit and either transport's subscriber teardown on the daemon bus.
func l2tpPoolReload(ctx context.Context, _ []string) error {
	return Observe(ctx, "l2tp-pool-reload", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := l2tpPoolReloadState(ctx, plugin, 1, 0); err != nil {
			return err
		}
		if err := l2tpPoolReloadAddress(ctx, plugin, "id 1 pool default", "10.100.0.10", "10.100.0.1"); err != nil {
			return err
		}
		if err := liveStagesReload(ctx, plugin, "config2.conf"); err != nil {
			return err
		}
		if err := l2tpPoolReloadState(ctx, plugin, 2, 1); err != nil {
			return err
		}
		if err := l2tpPoolReloadAddress(ctx, plugin, "id 2 pool default", "10.100.0.11", "10.100.0.1"); err != nil {
			return err
		}
		if err := l2tpPoolReloadAddress(ctx, plugin, "id 3 pool gold", "10.101.0.10", "10.101.0.1"); err != nil {
			return err
		}
		var delegation struct {
			Prefix string `json:"prefix"`
		}
		status, err := Dispatch(ctx, plugin, "request fakel2tp pool prefix id 4 pool default", &delegation)
		if err != nil {
			return err
		}
		if status != statusDone {
			return fmt.Errorf("prefix request returned %s", status)
		}
		if delegation.Prefix != "fd00:100::/64" {
			return fmt.Errorf("reloaded IPv6 pool delegated %q, want fd00:100::/64", delegation.Prefix)
		}
		var namedDelegation struct {
			Prefix string `json:"prefix"`
		}
		status, err = Dispatch(ctx, plugin, "request fakel2tp pool prefix id 5 pool gold6", &namedDelegation)
		if err != nil {
			return err
		}
		if status != statusDone {
			return fmt.Errorf("named prefix request returned %s", status)
		}
		if namedDelegation.Prefix != "fd00:101::/64" {
			return fmt.Errorf("reloaded named IPv6 pool delegated %q, want fd00:101::/64", namedDelegation.Prefix)
		}
		if err := l2tpPoolReloadNamedState(ctx, plugin); err != nil {
			return err
		}
		if err := l2tpPoolReloadIdentity(ctx, plugin); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: equal L2TP and PPPoE numeric ids retain distinct addresses through reload and either teardown") //nolint:errcheck // the .ci requires this marker
		fmt.Fprintln(os.Stderr, "OK: reloaded pool allocated added address 10.100.0.11, named address and IPv6 prefix")              //nolint:errcheck // the .ci requires this marker
		fmt.Fprintln(os.Stderr, "OK: show l2tp pool reports named IPv4 and IPv6 reservations")                                       //nolint:errcheck // the .ci requires this marker
		return nil
	})
}

// l2tpPoolReloadNamedState reads the daemon after both named allocations
// answered. The handler calls completed synchronously, so this needs no poll.
func l2tpPoolReloadNamedState(ctx context.Context, plugin *sdk.Plugin) error {
	var state struct {
		IPv4 map[string]struct {
			Start     string `json:"range-start"`
			End       string `json:"range-end"`
			Total     uint32 `json:"total"`
			Allocated uint32 `json:"allocated"`
			Available uint32 `json:"available"`
		} `json:"named-pools"`
		IPv6 map[string]struct {
			Block     string `json:"block"`
			Length    int    `json:"delegation-length"`
			Total     uint32 `json:"total"`
			Allocated uint32 `json:"allocated"`
			Available uint32 `json:"available"`
		} `json:"named-ipv6-pools"`
	}
	status, err := Dispatch(ctx, plugin, "show l2tp pool", &state)
	if err != nil {
		return err
	}
	if status != statusDone {
		return fmt.Errorf("named pool read returned %s", status)
	}
	address, found := state.IPv4["gold"]
	if !found {
		return fmt.Errorf("show l2tp pool omitted named-pools.gold: %v", state.IPv4)
	}
	if address.Start != "10.101.0.10" {
		return fmt.Errorf("named pool gold starts at %q, want 10.101.0.10", address.Start)
	}
	if address.End != "10.101.0.11" {
		return fmt.Errorf("named pool gold ends at %q, want 10.101.0.11", address.End)
	}
	if address.Total != 2 {
		return fmt.Errorf("named pool gold total=%d, want 2", address.Total)
	}
	if address.Allocated != 1 {
		return fmt.Errorf("named pool gold allocated=%d, want 1", address.Allocated)
	}
	if address.Available != 1 {
		return fmt.Errorf("named pool gold available=%d, want 1", address.Available)
	}
	prefix, found := state.IPv6["gold6"]
	if !found {
		return fmt.Errorf("show l2tp pool omitted named-ipv6-pools.gold6: %v", state.IPv6)
	}
	if prefix.Block != "fd00:101::/56" {
		return fmt.Errorf("named IPv6 pool gold6 block=%q, want fd00:101::/56", prefix.Block)
	}
	if prefix.Length != 64 {
		return fmt.Errorf("named IPv6 pool gold6 delegation-length=%d, want 64", prefix.Length)
	}
	if prefix.Total != 256 {
		return fmt.Errorf("named IPv6 pool gold6 total=%d, want 256", prefix.Total)
	}
	if prefix.Allocated != 1 {
		return fmt.Errorf("named IPv6 pool gold6 allocated=%d, want 1", prefix.Allocated)
	}
	if prefix.Available != 255 {
		return fmt.Errorf("named IPv6 pool gold6 available=%d, want 255", prefix.Available)
	}
	return nil
}

// l2tpPoolReloadState waits only on runtime pool state, never elapsed time.
func l2tpPoolReloadState(ctx context.Context, plugin *sdk.Plugin, total, allocated uint32) error {
	var state struct {
		Total     uint32 `json:"total"`
		Allocated uint32 `json:"allocated"`
	}
	var last error
	if !Poll(ctx, 30, 100*time.Millisecond, func() bool {
		status, err := Dispatch(ctx, plugin, "show l2tp pool", &state)
		last = err
		if err != nil {
			return false
		}
		if status != statusDone {
			return false
		}
		if state.Total != total {
			return false
		}
		return state.Allocated == allocated
	}) {
		mismatch := fmt.Errorf("pool after reload: total=%d allocated=%d, want total=%d allocated=%d",
			state.Total, state.Allocated, total, allocated)
		if last != nil {
			return fmt.Errorf("%w; last dispatch error: %w", mismatch, last)
		}
		return mismatch
	}
	return nil
}

// l2tpPoolReloadAddress dispatches once: retrying an allocation mutates state.
func l2tpPoolReloadAddress(ctx context.Context, plugin *sdk.Plugin, request, address, gateway string) error {
	return l2tpPoolReloadAddressCommand(ctx, plugin, "request fakel2tp pool address", request, address, gateway)
}

// l2tpPoolReloadAddressCommand preserves the address and gateway assertions for
// both explicit transport commands; every allocation is dispatched only once.
func l2tpPoolReloadAddressCommand(ctx context.Context, plugin *sdk.Plugin, command, request, address, gateway string) error {
	var answer struct {
		Address string `json:"address"`
		Gateway string `json:"gateway"`
	}
	status, err := Dispatch(ctx, plugin, command+" "+request, &answer)
	if err != nil {
		return err
	}
	if status != statusDone {
		return fmt.Errorf("address request %s returned %s", request, status)
	}
	if answer.Address != address {
		return fmt.Errorf("address request %s received %q, want %s", request, answer.Address, address)
	}
	if answer.Gateway != gateway {
		return fmt.Errorf("address request %s received gateway %q, want %s", request, answer.Gateway, gateway)
	}
	return nil
}
