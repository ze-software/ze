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
		if line = strings.TrimSpace(line); line != "" {
			entries = append(entries, line+" ")
		}
	}
	return entries
}
