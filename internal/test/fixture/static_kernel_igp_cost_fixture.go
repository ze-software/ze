// Design: docs/architecture/route-selection.md -- interior cost chooses between BGP paths and a cost change reselects
// Related: static_kernel_fixture.go -- the link, the daemon handle and the reload signal this scenario reuses
// Related: register_static_distance.go -- registers the driver below

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// The interior-cost scenario's addresses. Two static routes on the kernel link
// stand for the interior routes to two BGP next hops. They are interface-only,
// so the Loc-RIB ends its cost walk at them and their metric is the interior
// cost. The cheaper next hop is offered by the peer with the higher address, so
// the later tie-breakers alone would choose the other one.
const (
	igpCostPrefix       = "203.0.113.0/24"
	igpCostCheapPeer    = "10.0.0.2"
	igpCostCheapHop     = "10.91.0.1"
	igpCostDearPeer     = "10.0.0.1"
	igpCostDearHop      = "10.92.0.1"
	igpCostConfigBefore = "metric 10"
	igpCostConfigAfter  = "metric 30"
)

// staticKernelIGPCostReselect proves spec-fib-depth AC-1 and AC-13 through the
// operator's path. Two BGP paths for one prefix differ only in the interior
// cost of their next hops, which static routes supply: 10 to the cheap hop, 20
// to the dear one. The best path must use the cheap hop. The scenario then
// rewrites only the cheap hop's static metric to 30 and sends SIGHUP. No BGP
// route changes, so the best path can only move to the dear hop if the RIB
// reselects on the interior cost change.
func staticKernelIGPCostReselect(ctx context.Context, plugin *sdk.Plugin) error {
	if err := apiRIBReady02(ctx, plugin); err != nil {
		return err
	}
	for _, offer := range [][2]string{{igpCostDearPeer, igpCostDearHop}, {igpCostCheapPeer, igpCostCheapHop}} {
		var inject textbuf.Buffer
		inject.Str("request bgp rib inject ").Str(offer[0]).Str(" ipv4/unicast ").Str(igpCostPrefix)
		inject.Str(" origin igp localpref 100 aspath 64500 nexthop ").Str(offer[1])
		if _, err := requireDone02(ctx, plugin, inject.String()); err != nil {
			return err
		}
	}

	if err := igpCostAwaitBest(ctx, plugin, igpCostCheapPeer); err != nil {
		return fmt.Errorf("AC-1, interior cost 10 against 20: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK AC-1: interior cost chose "+igpCostCheapPeer+" via "+igpCostCheapHop)

	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	config, err := os.ReadFile("ze-bgp.conf")
	if err != nil {
		return fmt.Errorf("reading the daemon config to rewrite it: %w", err)
	}
	raised := strings.Replace(string(config), igpCostConfigBefore, igpCostConfigAfter, 1)
	if raised == string(config) {
		return fmt.Errorf("the daemon config holds no %q to raise: %q", igpCostConfigBefore, config)
	}
	if err := os.WriteFile("ze-bgp.conf", []byte(raised), 0o600); err != nil {
		return err
	}
	if err := signalProcess(pid, syscall.SIGHUP); err != nil {
		return err
	}

	if err := igpCostAwaitBest(ctx, plugin, igpCostDearPeer); err != nil {
		return fmt.Errorf("AC-13, after the cheap hop's interior cost rose to 30: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK AC-13: interior cost change moved the best path to "+igpCostDearPeer+" via "+igpCostDearHop)
	return nil
}

// igpCostAwaitBest waits, inside the kernel scenarios' ten-second bound, for
// the BGP best path of igpCostPrefix to come from wantPeer.
func igpCostAwaitBest(ctx context.Context, plugin *sdk.Plugin, wantPeer string) error {
	var seen string
	if Poll(ctx, kernelPolls, 100*time.Millisecond, func() bool {
		var peer string
		peer, seen = igpCostBestPeer(ctx, plugin)
		return peer == wantPeer
	}) {
		return nil
	}
	return fmt.Errorf("best path for %s is not from %s; show bgp rib best: %s", igpCostPrefix, wantPeer, seen)
}

// igpCostBestPeer reads the peer that bgp-rib selected for igpCostPrefix.
func igpCostBestPeer(ctx context.Context, plugin *sdk.Plugin) (peer, raw string) {
	data, err := requireDone02(ctx, plugin, "show bgp rib best")
	if err != nil {
		return "", err.Error()
	}
	raw = text02(data)
	var answer struct {
		BestPath []struct {
			Prefix   string `json:"prefix"`
			BestPeer string `json:"best-peer"`
		} `json:"best-path"`
	}
	if decodeErr := decode02(data, &answer); decodeErr != nil {
		return "", raw
	}
	for _, row := range answer.BestPath {
		if row.Prefix == igpCostPrefix {
			return row.BestPeer, raw
		}
	}
	return "", raw
}
