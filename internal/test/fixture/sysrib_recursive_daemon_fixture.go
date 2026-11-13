// Design: docs/architecture/rib/unified-locrib.md -- common selected LPM and cascade.
// Related: sysrib_forwarding_capture_fixture.go -- real SDK producer and FIB observations.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	sysribRecursiveLink    = "zesrrec"
	sysribRecursiveV4      = "198.18.244.0/24"
	sysribRecursiveV6      = "2001:db8:244::/64"
	sysribRecursiveCross   = "198.18.245.0/24"
	sysribRecursiveControl = "198.18.246.0/24"
)

func sysribRecursiveDriver(ctx context.Context, args []string) error {
	if err := importPlacementArgs(args, 1); err != nil {
		return err
	}
	capture := newSysribFIBCapture(sysribRecursiveV4, sysribRecursiveV6, sysribRecursiveCross, sysribRecursiveControl)
	return observeConfigured(ctx, "sysrib-recursive-test", &sdk.Registration{}, capture.setup, func(ctx context.Context, p *sdk.Plugin) (result error) {
		if err := sysribExecutionPlacement(ctx, p, args[0]); err != nil {
			return err
		}
		if err := importCreateLink(ctx, sysribRecursiveLink, "198.18.240.254/24", "2001:db8:240::fe/64"); err != nil {
			return err
		}
		defer func() { result = errors.Join(result, importDeleteLink(ctx, sysribRecursiveLink)) }()
		for _, prefix := range []string{"198.18.240.0/24", "2001:db8:240::/64"} {
			if err := waitRIBWinner(ctx, p, prefix, "connected"); err != nil {
				return err
			}
		}
		cover4 := rpc.RouteInstallEntry{Protocol: "ospf", AFI: 1, SAFI: 1, Prefix: "10.242.0.0/16", NextHop: "198.18.240.1", Interface: sysribRecursiveLink, OnLink: true, Metric: 13, RouteType: 1}
		cover6 := rpc.RouteInstallEntry{Protocol: "ospf", AFI: 2, SAFI: 1, Prefix: "2001:db8:242::/48", NextHop: "2001:db8:240::1", Interface: sysribRecursiveLink, OnLink: true, Metric: 17, RouteType: 1}
		hidden4 := cover4
		hidden4.Prefix = "10.242.1.0/24"
		hidden4.HasTag = true
		hidden4.Tag = 4242
		hidden4.Metric = 99
		hidden6 := cover6
		hidden6.Prefix = "2001:db8:242:1::/64"
		hidden6.HasTag = true
		hidden6.Tag = 4242
		hidden6.Metric = 99
		control := rpc.RouteInstallEntry{Protocol: "static", AFI: 1, SAFI: 1, Prefix: sysribRecursiveControl, RouteType: 6}
		subjects := []rpc.RouteInstallEntry{
			{Protocol: "static", AFI: 1, SAFI: 1, Prefix: sysribRecursiveV4, NextHop: "10.242.1.5", Metric: 5, MetricRecursive: true, RouteType: 1},
			{Protocol: "static", AFI: 2, SAFI: 1, Prefix: sysribRecursiveV6, NextHop: "2001:db8:242:1::5", Metric: 5, MetricRecursive: true, RouteType: 1},
			{Protocol: "static", AFI: 1, SAFI: 1, Prefix: sysribRecursiveCross, NextHop: "2001:db8:242:1::5", Metric: 5, MetricRecursive: true, RouteType: 1},
		}
		// The more-specific candidates are present and hidden before their
		// dependent subjects arrive; selection must find each broader survivor.
		if err := sysribInstall(ctx, p, cover4, cover6, hidden4, hidden6, control); err != nil {
			return err
		}
		tag := uint32(4242)
		for _, prefix := range []string{hidden4.Prefix, hidden6.Prefix} {
			if err := importCandidateState(ctx, p, prefix, "ospf", 110, &tag, false, "central", "rib import"); err != nil {
				return err
			}
		}
		if err := sysribInstall(ctx, p, subjects...); err != nil {
			return err
		}
		if err := sysribRecursiveResolved(ctx, p, capture, 0); err != nil {
			return err
		}
		before, err := sysribRecursiveMetrics(ctx, p, true, 13, 17)
		if err != nil {
			return err
		}
		// The producer offers no subject again after this point. Cost updates,
		// cover loss and recovery must all come from dependency changes.
		cover4.Metric = 23
		if err := sysribInstall(ctx, p, cover4); err != nil {
			return err
		}
		after, err := sysribRecursiveMetrics(ctx, p, true, 23, 17)
		if err != nil {
			return err
		}
		if after == before {
			return fmt.Errorf("topology metric change retained routing revision %d", before)
		}
		if err := sysribRemove(ctx, p, cover4, cover6); err != nil {
			return err
		}
		if _, err := sysribRecursiveMetrics(ctx, p, false, 0, 0); err != nil {
			return err
		}
		for _, prefix := range []string{sysribRecursiveV4, sysribRecursiveV6, sysribRecursiveCross} {
			if err := capture.await(ctx, prefix, 0, sysribevents.BestChangeEntry{Action: routeaction.Withdraw, Prefix: netip.MustParsePrefix(prefix)}); err != nil {
				return err
			}
		}
		if err := importKernel(ctx, sysribRecursiveV4, 0); err != nil {
			return err
		}
		if err := sysribKernel6(ctx, sysribRecursiveV6, 0); err != nil {
			return err
		}
		if err := importKernel(ctx, sysribRecursiveCross, 0); err != nil {
			return err
		}
		if err := sysribRecursiveSurvivors(ctx); err != nil {
			return err
		}
		if err := sysribInstall(ctx, p, cover4, cover6); err != nil {
			return err
		}
		if err := sysribRecursiveResolved(ctx, p, capture, 0); err != nil {
			return err
		}
		if _, err := sysribRecursiveMetrics(ctx, p, true, 23, 17); err != nil {
			return err
		}
		replayAt, err := capture.requestReplay(ctx, p)
		if err != nil {
			return err
		}
		if err := sysribRecursiveResolved(ctx, p, capture, replayAt); err != nil {
			return err
		}
		return importSay("recursive IPv4, IPv6 and cross-family outputs use broader selectable covers, withdraw on proved loss, recover without reinjection and retain current replay and metric revision")
	})
}

func sysribRecursiveResolved(ctx context.Context, p *sdk.Plugin, capture *sysribFIBCapture, replayAt uint64) error {
	for _, value := range []struct{ prefix, gateway string }{
		{sysribRecursiveV4, "198.18.240.1"}, {sysribRecursiveV6, "2001:db8:240::1"}, {sysribRecursiveCross, "2001:db8:240::1"},
	} {
		if err := waitRIBWinner(ctx, p, value.prefix, "static"); err != nil {
			return err
		}
		want := sysribevents.BestChangeEntry{Prefix: netip.MustParsePrefix(value.prefix), Protocol: "static", NextHop: netip.MustParseAddr(value.gateway), Interface: sysribRecursiveLink, OnLink: true, Metric: 5, RouteType: 1}
		if replayAt != 0 {
			want.Action = routeaction.Add
		}
		if err := capture.await(ctx, value.prefix, replayAt, want); err != nil {
			return err
		}
		if want.Prefix.Addr().Is6() {
			if err := sysribKernel6(ctx, value.prefix, 1, value.gateway+" ", "dev "+sysribRecursiveLink+" ", "proto 250 ", "metric 5 "); err != nil {
				return err
			}
		} else if err := importKernel(ctx, value.prefix, 1, value.gateway+" ", "dev "+sysribRecursiveLink+" ", "proto 250 ", "metric 5 "); err != nil {
			return err
		}
	}
	return sysribRecursiveSurvivors(ctx)
}

func sysribRecursiveMetrics(ctx context.Context, p *sdk.Plugin, resolved bool, cost4, cost6 uint64) (uint64, error) {
	addresses := []string{"10.242.1.5", "2001:db8:242:1::5", "198.18.240.1"}
	out, err := p.RouteMetrics(ctx, addresses)
	if err != nil {
		return 0, err
	}
	if len(out.Distances) != 3 {
		return 0, fmt.Errorf("route-metrics returned %d values for three inputs", len(out.Distances))
	}
	want := []rpc.RouteMetric{{Cost: cost4, Resolved: resolved}, {Cost: cost6, Resolved: resolved}, {Resolved: true}}
	for i := range want {
		if out.Distances[i] != want[i] {
			return 0, fmt.Errorf("route-metrics %s: got %+v, want %+v", addresses[i], out.Distances[i], want[i])
		}
	}
	return out.Revision, nil
}

func sysribRecursiveSurvivors(ctx context.Context) error {
	if err := importKernel(ctx, sysribRecursiveControl, 1, "6 "+sysribRecursiveControl+" ", "proto 250 "); err != nil {
		return err
	}
	if err := importKernel(ctx, "198.18.240.0/24", 1, "dev "+sysribRecursiveLink+" ", "proto 2 "); err != nil {
		return err
	}
	return sysribKernel6(ctx, "2001:db8:240::/64", 1, "dev "+sysribRecursiveLink+" ", "proto 2 ")
}

func sysribRecursiveKernelEmpty(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("recursive kernel-empty fixture takes no arguments")
	}
	for _, prefix := range []string{sysribRecursiveV4, sysribRecursiveCross, sysribRecursiveControl, "198.18.240.0/24", "10.242.0.0/16", "10.242.1.0/24"} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	for _, prefix := range []string{sysribRecursiveV6, "2001:db8:240::/64", "2001:db8:242::/48", "2001:db8:242:1::/64"} {
		if err := sysribKernel6(ctx, prefix, 0); err != nil {
			return err
		}
	}
	return importSay("recursive fixture prefixes are absent from every kernel table")
}
