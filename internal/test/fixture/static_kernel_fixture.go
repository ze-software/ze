// Design: docs/architecture/static-routes.md -- static routes reach the FIB through the Loc-RIB
// Related: static_distance_fixture.go -- the system RIB read and the contested prefix these extend to the kernel
// Related: static_named_table_fixture.go -- kernelRoute, the single-table kernel read
// Related: register_static_distance.go -- registers the drivers below

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

// The kernel scenarios' addresses. One dummy link carries both gateway subnets,
// so the static next-hops and the BGP next-hop all resolve in a bare namespace
// and the FIB plugin can program whichever path wins.
const (
	kernelLink          = "zentk"
	kernelStaticAddress = "192.0.2.2/24"
	kernelBGPAddress    = "198.51.100.2/24"
	kernelStaticGateway = "192.0.2.1"
	kernelStaticSecond  = "192.0.2.3"
	kernelBGPGateway    = "198.51.100.1"
	kernelWeightedRoute = "198.18.73.0/24"
	kernelIfaceRoute    = "198.18.74.0/24"

	// kernelPolls bounds each wait to ten seconds, inside the .ci timeout, so a
	// failing wait reports its own reason before the runner kills the daemon.
	kernelPolls = 100
)

// staticKernelSetup creates the link that makes every gateway in the kernel
// scenarios reachable.
func staticKernelSetup(ctx context.Context, _ []string) error {
	return staticSetup(kernelLink, kernelStaticAddress, kernelBGPAddress)(ctx, nil)
}

// staticKernelDistanceWinner builds the scenario that reads the arbitration's
// outcome from the KERNEL: the system RIB must name wantProtocol for the
// contested prefix, and the kernel must then hold exactly one entry for it, in
// any table, stamped proto 250 by the FIB plugin and forwarding via wantGateway.
//
// One entry is the point. Before static main-table routes went through the
// Loc-RIB, static wrote its own proto 251 entry and the FIB plugin wrote proto
// 250 for the BGP winner, both at the default metric, so write order rather than
// the declared distance decided which one forwarded.
func staticKernelDistanceWinner(wantProtocol, wantGateway string) ObserverScenario {
	return func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := apiRIBReady02(ctx, plugin); err != nil {
			return err
		}
		var inject textbuf.Buffer
		inject.Str("request bgp rib inject 10.0.0.99 ipv4/unicast ").Str(arbitratedPrefix)
		inject.Str(" origin igp aspath 64500 nexthop ").Str(kernelBGPGateway)
		if _, err := requireDone02(ctx, plugin, inject.String()); err != nil {
			return err
		}

		var winner, seen string
		if !Poll(ctx, kernelPolls, 100*time.Millisecond, func() bool {
			winner, seen = ribWinner02(ctx, plugin, arbitratedPrefix)
			return winner == wantProtocol
		}) {
			return fmt.Errorf("%s: system RIB winner is %q, want %q; show rib: %s",
				arbitratedPrefix, winner, wantProtocol, seen)
		}

		var entries []string
		if !Poll(ctx, kernelPolls, 100*time.Millisecond, func() bool {
			entries = kernelEntries(ctx, arbitratedPrefix)
			return len(entries) == 1 &&
				strings.Contains(entries[0], "proto 250") &&
				strings.Contains(entries[0], "via "+wantGateway+" ")
		}) {
			return fmt.Errorf("%s won by %s: want exactly one kernel entry, proto 250 via %s; ip route show table all: %q",
				arbitratedPrefix, wantProtocol, wantGateway, entries)
		}
		fmt.Fprintln(os.Stderr, "OK: kernel holds one entry for "+arbitratedPrefix+", proto 250 via "+wantGateway)
		return nil
	}
}

// staticKernelWeightedMultipath proves AC-12 at the kernel: a main-table static
// route with two weighted next-hops reaches the kernel as one proto 250
// multipath entry whose hops carry the configured weights.
func staticKernelWeightedMultipath(ctx context.Context, _ *sdk.Plugin) error {
	want := []string{
		"nexthop via " + kernelStaticGateway + " dev " + kernelLink + " weight 3",
		"nexthop via " + kernelStaticSecond + " dev " + kernelLink + " weight 1",
	}
	var route string
	if !Poll(ctx, kernelPolls, 100*time.Millisecond, func() bool {
		route = strings.Join(strings.Fields(kernelRoute(ctx, "main", kernelWeightedRoute)), " ")
		if !strings.Contains(route, "proto 250") {
			return false
		}
		for _, hop := range want {
			if !strings.Contains(route, hop) {
				return false
			}
		}
		return true
	}) {
		return fmt.Errorf("%s: want a proto 250 multipath entry carrying %q; ip route: %q",
			kernelWeightedRoute, want, route)
	}
	fmt.Fprintln(os.Stderr, "OK: "+kernelWeightedRoute+" is multipath with weights 3 and 1, proto 250")
	return nil
}

// staticKernelInterfaceNextHop proves AC-13 at the kernel: a main-table static
// route whose only next-hop names an interface is programmed by the FIB plugin
// (proto 250) out of that interface, with no gateway.
func staticKernelInterfaceNextHop(ctx context.Context, _ *sdk.Plugin) error {
	var route string
	if !Poll(ctx, kernelPolls, 100*time.Millisecond, func() bool {
		route = strings.Join(strings.Fields(kernelRoute(ctx, "main", kernelIfaceRoute)), " ")
		return strings.Contains(route, "dev "+kernelLink+" ") && strings.Contains(route, "proto 250")
	}) {
		return fmt.Errorf("%s: want a proto 250 entry out of dev %s; ip route: %q",
			kernelIfaceRoute, kernelLink, route)
	}
	if strings.Contains(route, " via ") {
		return fmt.Errorf("%s: an interface-only next-hop was programmed with a gateway: %q", kernelIfaceRoute, route)
	}
	fmt.Fprintln(os.Stderr, "OK: "+kernelIfaceRoute+" is programmed out of dev "+kernelLink+", proto 250")
	return nil
}

// kernelEntries returns every kernel entry for exactly prefix across all tables,
// one string per entry. -N prints the protocol as its number, so an rt_protos
// mapping on the host cannot hide 250 or 251.
func kernelEntries(ctx context.Context, prefix string) []string {
	out, err := netfilterCommandOutput(ctx, "ip", "-N", "route", "show", "table", "all", "exact", prefix)
	if err != nil {
		return nil
	}
	var entries []string
	for line := range strings.SplitSeq(out, "\n") {
		// iproute2 pads some fields with a second space: an MPLS route
		// prints "<prefix>  encap mpls  100/200 via ...". Each word is
		// rejoined with one space so a caller matches the words, not the
		// padding.
		if fields := strings.Fields(line); len(fields) > 0 {
			entries = append(entries, strings.Join(fields, " ")+" ")
		}
	}
	return entries
}

// staticKernelDistanceReload proves the owner decision of 2026-10-08 at the
// kernel: the RIB owns administrative distance, so a reload that changes only
// `rib { distance { static } }` re-ranks the static route ALREADY installed.
// The scenario starts at `static 5`, waits for the kernel to forward on the
// static next-hop, rewrites the config to `static 250`, sends SIGHUP, and waits
// for the kernel to move to the BGP next-hop with no second entry left behind.
func staticKernelDistanceReload(ctx context.Context, plugin *sdk.Plugin) error {
	if err := staticKernelDistanceWinner(protocolStatic, kernelStaticGateway)(ctx, plugin); err != nil {
		return fmt.Errorf("before the reload: %w", err)
	}
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	config, err := os.ReadFile("ze-bgp.conf")
	if err != nil {
		return fmt.Errorf("reading the daemon config to rewrite it: %w", err)
	}
	raised := strings.Replace(string(config), "static 5", "static 250", 1)
	if raised == string(config) {
		return fmt.Errorf("the daemon config holds no `static 5` to raise: %q", config)
	}
	if err := os.WriteFile("ze-bgp.conf", []byte(raised), 0o600); err != nil {
		return err
	}
	if err := signalProcess(pid, syscall.SIGHUP); err != nil {
		return err
	}

	var entries []string
	if !Poll(ctx, kernelPolls, 100*time.Millisecond, func() bool {
		entries = kernelEntries(ctx, arbitratedPrefix)
		return len(entries) == 1 &&
			strings.Contains(entries[0], "proto 250") &&
			strings.Contains(entries[0], "via "+kernelBGPGateway+" ")
	}) {
		return fmt.Errorf("after the reload to static 250: want exactly one kernel entry, proto 250 via %s; ip route show table all: %q",
			kernelBGPGateway, entries)
	}
	if winner, seen := ribWinner02(ctx, plugin, arbitratedPrefix); winner != "bgp" {
		return fmt.Errorf("after the reload to static 250: system RIB winner is %q, want bgp; show rib: %s", winner, seen)
	}
	fmt.Fprintln(os.Stderr, "OK: reload re-ranked "+arbitratedPrefix+", kernel now via "+kernelBGPGateway)
	return nil
}

// overrideWitnessPrefix is the second contested prefix of the route-override
// scenario. Its static route carries no distance of its own, so it ranks at the
// declared static distance and shows when a reload of that distance took effect.
const overrideWitnessPrefix = "10.1.0.0/16"

// staticKernelDistanceRouteOverride proves AC-21 at the kernel: a static
// route's own `distance` leaf wins over `rib { distance { static } }`, and a
// reload of that declaration leaves the route at its own value.
//
// The config gives 10.0.0.0/8 its own `distance 3` and leaves 10.1.0.0/16
// without one, under `static 5`; eBGP offers both prefixes at 20. Before the
// reload the static next-hop forwards both. The reload raises the declaration
// to `static 250`. The witness 10.1.0.0/16 then moves to the BGP next-hop,
// which proves sysrib published the new declaration and the Loc-RIB re-ranked.
// 10.0.0.0/8 must stay on the static next-hop: its own 3 still beats eBGP 20,
// where the declared 250 would lose. An override the Loc-RIB ignored, or one
// the static plugin never sent, therefore moves 10.0.0.0/8 to BGP with the
// witness.
func staticKernelDistanceRouteOverride(ctx context.Context, plugin *sdk.Plugin) error {
	if err := apiRIBReady02(ctx, plugin); err != nil {
		return err
	}
	for _, prefix := range []string{arbitratedPrefix, overrideWitnessPrefix} {
		var inject textbuf.Buffer
		inject.Str("request bgp rib inject 10.0.0.99 ipv4/unicast ").Str(prefix)
		inject.Str(" origin igp aspath 64500 nexthop ").Str(kernelBGPGateway)
		if _, err := requireDone02(ctx, plugin, inject.String()); err != nil {
			return err
		}
	}
	for _, prefix := range []string{arbitratedPrefix, overrideWitnessPrefix} {
		if err := kernelForwardsVia(ctx, prefix, kernelStaticGateway); err != nil {
			return fmt.Errorf("before the reload: %w", err)
		}
	}

	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	config, err := os.ReadFile("ze-bgp.conf")
	if err != nil {
		return fmt.Errorf("reading the daemon config to rewrite it: %w", err)
	}
	raised := strings.Replace(string(config), "static 5", "static 250", 1)
	if raised == string(config) {
		return fmt.Errorf("the daemon config holds no `static 5` to raise: %q", config)
	}
	if err := os.WriteFile("ze-bgp.conf", []byte(raised), 0o600); err != nil {
		return err
	}
	if err := signalProcess(pid, syscall.SIGHUP); err != nil {
		return err
	}

	if err := kernelForwardsVia(ctx, overrideWitnessPrefix, kernelBGPGateway); err != nil {
		return fmt.Errorf("after the reload to static 250, the witness: %w", err)
	}
	if err := kernelForwardsVia(ctx, arbitratedPrefix, kernelStaticGateway); err != nil {
		return fmt.Errorf("after the reload to static 250, the route with its own distance 3: %w", err)
	}
	if winner, seen := ribWinner02(ctx, plugin, arbitratedPrefix); winner != protocolStatic {
		return fmt.Errorf("after the reload to static 250: %s system RIB winner is %q, want %q; show rib: %s",
			arbitratedPrefix, winner, protocolStatic, seen)
	}
	fmt.Fprintln(os.Stderr, "OK: "+arbitratedPrefix+" kept its own distance 3 across the reload, kernel via "+kernelStaticGateway)
	return nil
}

// kernelForwardsVia waits for the kernel to hold exactly one entry for prefix,
// programmed by the FIB plugin (proto 250) via gateway, and reports what it
// held when the wait runs out.
func kernelForwardsVia(ctx context.Context, prefix, gateway string) error {
	var entries []string
	if !Poll(ctx, kernelPolls, 100*time.Millisecond, func() bool {
		entries = kernelEntries(ctx, prefix)
		return len(entries) == 1 &&
			strings.Contains(entries[0], "proto 250") &&
			strings.Contains(entries[0], "via "+gateway+" ")
	}) {
		return fmt.Errorf("%s: want exactly one kernel entry, proto 250 via %s; ip route show table all: %q",
			prefix, gateway, entries)
	}
	return nil
}
