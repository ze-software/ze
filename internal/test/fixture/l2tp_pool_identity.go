// Design: docs/functional-tests.md -- transport identity through the daemon's pool and subscriber bus
// Related: l2tp_pool_reload.go -- existing default and named reload assertions

package fixture

import (
	"context"
	"fmt"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// l2tpPoolReloadIdentity retains the original named L2TP owner while a PPPoE
// owner uses its exact numeric pair. Reload and each teardown must preserve
// the other owner's address without creating a second reservation on retry.
func l2tpPoolReloadIdentity(ctx context.Context, plugin *sdk.Plugin) error {
	const request = "id 3 pool gold"
	const pppoe = "request fakel2tp pool pppoe-address"
	if err := l2tpPoolReloadAddressCommand(ctx, plugin, pppoe, request, "10.101.0.11", "10.101.0.1"); err != nil {
		return err
	}
	if err := liveStagesReload(ctx, plugin, "config3.conf"); err != nil {
		return err
	}
	if err := l2tpPoolReloadIdentityState(ctx, plugin, 2); err != nil {
		return err
	}
	if err := l2tpPoolReloadAddress(ctx, plugin, request, "10.101.0.10", "10.101.0.1"); err != nil {
		return err
	}
	if err := l2tpPoolReloadAddressCommand(ctx, plugin, pppoe, request, "10.101.0.11", "10.101.0.1"); err != nil {
		return err
	}
	if err := l2tpPoolReloadIdentityRelease(ctx, plugin, "pppoe", 1); err != nil {
		return err
	}
	if err := l2tpPoolReloadAddress(ctx, plugin, request, "10.101.0.10", "10.101.0.1"); err != nil {
		return err
	}
	if err := l2tpPoolReloadIdentityState(ctx, plugin, 1); err != nil {
		return err
	}
	if err := l2tpPoolReloadAddressCommand(ctx, plugin, pppoe, request, "10.101.0.11", "10.101.0.1"); err != nil {
		return err
	}
	if err := l2tpPoolReloadIdentityState(ctx, plugin, 2); err != nil {
		return err
	}
	if err := l2tpPoolReloadIdentityRelease(ctx, plugin, "l2tp", 1); err != nil {
		return err
	}
	if err := l2tpPoolReloadAddressCommand(ctx, plugin, pppoe, request, "10.101.0.11", "10.101.0.1"); err != nil {
		return err
	}
	if err := l2tpPoolReloadIdentityState(ctx, plugin, 1); err != nil {
		return err
	}
	if err := l2tpPoolReloadIdentityRelease(ctx, plugin, "pppoe", 0); err != nil {
		return err
	}
	return l2tpPoolReloadIdentityState(ctx, plugin, 0)
}

// l2tpPoolReloadIdentityRelease emits once, then waits on the real named-pool
// state; command delivery alone does not prove the pool subscriber ran.
func l2tpPoolReloadIdentityRelease(ctx context.Context, plugin *sdk.Plugin, access string, allocated uint32) error {
	var answer struct {
		ID string `json:"id"`
	}
	status, err := Dispatch(ctx, plugin, "request fakel2tp pool release access "+access+" id 3", &answer)
	if err != nil {
		return err
	}
	if status != statusDone {
		return fmt.Errorf("%s teardown returned %s", access, status)
	}
	if answer.ID != access+"-65000-3" {
		return fmt.Errorf("%s teardown identified %q, want %s-65000-3", access, answer.ID, access)
	}
	return l2tpPoolReloadIdentityState(ctx, plugin, allocated)
}

// l2tpPoolReloadIdentityState polls reservation counts rather than sleeping
// for the subscriber bus; it also refuses a missing or resized gold pool.
func l2tpPoolReloadIdentityState(ctx context.Context, plugin *sdk.Plugin, allocated uint32) error {
	var state struct {
		Pools map[string]struct {
			Total     uint32 `json:"total"`
			Allocated uint32 `json:"allocated"`
			Available uint32 `json:"available"`
		} `json:"named-pools"`
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
		pool, found := state.Pools["gold"]
		if !found {
			return false
		}
		if pool.Total != 3 {
			return false
		}
		if pool.Available != 3-allocated {
			return false
		}
		return pool.Allocated == allocated
	}) {
		mismatch := fmt.Errorf("transport identity gold pool=%+v, want total=3 allocated=%d available=%d",
			state.Pools["gold"], allocated, 3-allocated)
		if last != nil {
			return fmt.Errorf("%w; last dispatch error: %w", mismatch, last)
		}
		return mismatch
	}
	return nil
}
