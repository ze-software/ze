// Design: docs/architecture/route-selection.md -- authoritative interior costs.
// Related: ../../component/bgp/plugins/rib/rib_remote.go -- actual remote caller.
// Related: sysrib_recursive_daemon_fixture.go -- observer-only metric controls.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	sysribBGPMetricLink        = "zesrmetric"
	sysribBGPMetricTerminal    = "10.248.0.0/24"
	sysribBGPMetricStatic      = "10.249.1.0/24"
	sysribBGPMetricReceived    = "10.249.2.0/24"
	sysribBGPMetricMissing     = "10.249.3.0/24"
	sysribBGPMetricZero        = "10.249.4.0/24"
	sysribBGPMetricSubject     = "198.19.35.0/24"
	sysribBGPMetricAIGPSubject = "198.19.36.0/24"
	sysribBGPMetricControl     = "198.19.37.0/24"
	sysribBGPMetricConnected   = "198.19.30.0/24"
)

func sysribBGPMetricDriver(ctx context.Context, args []string) error {
	if err := importPlacementArgs(args, 1); err != nil {
		return err
	}
	return Observe(ctx, "sysrib-bgp-metric-test", &sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) (result error) {
		if err := sysribExecutionPlacement(ctx, p, "forked"); err != nil {
			return err
		}
		if err := sysribExecutionPluginPlacement(ctx, p, "bgp-rib", "bgp-rib", args[0]); err != nil {
			return err
		}
		if err := apiRIBReady02(ctx, p); err != nil {
			return err
		}
		if err := importCreateLink(ctx, sysribBGPMetricLink, "198.19.30.254/24"); err != nil {
			return err
		}
		defer func() { result = errors.Join(result, importDeleteLink(ctx, sysribBGPMetricLink)) }()
		if err := waitRIBWinner(ctx, p, sysribBGPMetricConnected, "connected"); err != nil {
			return err
		}
		terminal := rpc.RouteInstallEntry{Protocol: "ospf", AFI: 1, SAFI: 1, Prefix: sysribBGPMetricTerminal, Interface: sysribBGPMetricLink, Metric: 11, RouteType: 1}
		static := rpc.RouteInstallEntry{Protocol: "static", AFI: 1, SAFI: 1, Prefix: sysribBGPMetricStatic, NextHop: "10.248.0.1", Metric: 999, MetricRecursive: true, RouteType: 1}
		received := rpc.RouteInstallEntry{Protocol: "bgp", AFI: 1, SAFI: 1, Prefix: sysribBGPMetricReceived, NextHop: "10.248.0.2", Metric: 999, IsBGP: true, IsEBGP: true, AIGP: 7, AIGPPresent: true, RouteType: 1}
		missing := rpc.RouteInstallEntry{Protocol: "bgp", AFI: 1, SAFI: 1, Prefix: sysribBGPMetricMissing, NextHop: "10.248.0.3", Metric: 999, IsBGP: true, IsEBGP: true, RouteType: 1}
		zero := rpc.RouteInstallEntry{Protocol: "ospf", AFI: 1, SAFI: 1, Prefix: sysribBGPMetricZero, Interface: sysribBGPMetricLink, RouteType: 1}
		control := rpc.RouteInstallEntry{Protocol: "static", AFI: 1, SAFI: 1, Prefix: sysribBGPMetricControl, RouteType: 6}
		if err := sysribInstall(ctx, p, terminal, static, received, missing, zero, control); err != nil {
			return err
		}
		initial, err := sysribBGPMetricValues(ctx, p, 11, 0, 0, false, false)
		if err != nil {
			return err
		}
		// Each subject is offered once per peer. No subsequent topology step
		// reinjects these routes: both real RIB placements must reselect them.
		for _, offer := range []struct{ prefix, peer, hop string }{
			{sysribBGPMetricSubject, "10.0.0.1", "10.249.1.1"},
			{sysribBGPMetricSubject, "10.0.0.2", "10.249.4.1"},
			{sysribBGPMetricAIGPSubject, "10.0.0.3", "10.249.2.1"},
			{sysribBGPMetricAIGPSubject, "10.0.0.4", "10.249.3.1"},
		} {
			command := "request bgp rib inject " + offer.peer + " ipv4/unicast " + offer.prefix + " origin igp localpref 100 aspath 64500 nexthop " + offer.hop
			if _, err := requireDone02(ctx, p, command); err != nil {
				return err
			}
		}
		if err := sysribBGPMetricWinner(ctx, p, sysribBGPMetricSubject, "10.0.0.2", "10.249.4.1"); err != nil {
			return fmt.Errorf("resolved zero must beat recursive cost 11: %w", err)
		}
		if err := sysribBGPMetricWinner(ctx, p, sysribBGPMetricAIGPSubject, "10.0.0.4", "10.249.3.1"); err != nil {
			return fmt.Errorf("missing recursive AIGP is reachable cost 11 beside received cost 18: %w", err)
		}
		if err := sysribRemove(ctx, p, zero); err != nil {
			return err
		}
		if _, err := sysribBGPMetricValues(ctx, p, 11, 0, 0, false, true); err != nil {
			return err
		}
		if err := sysribBGPMetricWinner(ctx, p, sysribBGPMetricSubject, "10.0.0.1", "10.249.1.1"); err != nil {
			return fmt.Errorf("lost zero-cost reachability must invalidate cached winner: %w", err)
		}
		zero.Metric = 23
		if err := sysribInstall(ctx, p, zero); err != nil {
			return err
		}
		if _, err := sysribBGPMetricValues(ctx, p, 11, 23, 0, false, false); err != nil {
			return err
		}
		if err := sysribBGPMetricWinner(ctx, p, sysribBGPMetricSubject, "10.0.0.1", "10.249.1.1"); err != nil {
			return err
		}
		terminal.Metric = 31
		if err := sysribInstall(ctx, p, terminal); err != nil {
			return err
		}
		revised, err := sysribBGPMetricValues(ctx, p, 31, 23, 0, false, false)
		if err != nil {
			return err
		}
		if revised <= initial {
			return fmt.Errorf("topology mutation did not advance metric revision: %d -> %d", initial, revised)
		}
		if err := sysribBGPMetricWinner(ctx, p, sysribBGPMetricSubject, "10.0.0.2", "10.249.4.1"); err != nil {
			return fmt.Errorf("recursive terminal cost change must reselect retained BGP inputs: %w", err)
		}
		missing.AIGPPresent = true
		missing.AIGP = 30
		if err := sysribInstall(ctx, p, missing); err != nil {
			return err
		}
		if _, err := sysribBGPMetricValues(ctx, p, 31, 23, 30, true, false); err != nil {
			return err
		}
		if err := sysribBGPMetricWinner(ctx, p, sysribBGPMetricAIGPSubject, "10.0.0.3", "10.249.2.1"); err != nil {
			return fmt.Errorf("recursive received AIGP change must move the actual BGP winner: %w", err)
		}
		missing.AIGPPresent = false
		missing.AIGP = 0
		terminal.Metric = 0
		if err := sysribInstall(ctx, p, missing, terminal); err != nil {
			return err
		}
		if _, err := sysribBGPMetricValues(ctx, p, 0, 23, 0, false, false); err != nil {
			return err
		}
		if err := sysribBGPMetricWinner(ctx, p, sysribBGPMetricSubject, "10.0.0.1", "10.249.1.1"); err != nil {
			return fmt.Errorf("resolved recursive zero must remain usable: %w", err)
		}
		if err := sysribBGPMetricWinner(ctx, p, sysribBGPMetricAIGPSubject, "10.0.0.4", "10.249.3.1"); err != nil {
			return err
		}
		// A real received zero is present and must clear MissingAIGP without
		// making this equally reachable recursive path disappear.
		missing.AIGPPresent = true
		if err := sysribInstall(ctx, p, missing); err != nil {
			return err
		}
		if _, err := sysribBGPMetricValues(ctx, p, 0, 23, 0, true, false); err != nil {
			return err
		}
		if err := sysribBGPMetricWinner(ctx, p, sysribBGPMetricAIGPSubject, "10.0.0.4", "10.249.3.1"); err != nil {
			return err
		}
		if err := importKernel(ctx, sysribBGPMetricControl, 1, "6 "+sysribBGPMetricControl+" ", "proto 250 "); err != nil {
			return err
		}
		if err := importKernel(ctx, sysribBGPMetricConnected, 1, "dev "+sysribBGPMetricLink+" ", "proto 2 "); err != nil {
			return err
		}
		return importSay("actual " + args[0] + " BGP RIB resolves zero, recursive cost and received/missing AIGP through the daemon authority with external system RIB, and topology-only changes reselect retained routes into the kernel")
	})
}

func sysribBGPMetricValues(ctx context.Context, p *sdk.Plugin, terminal, zero, missingAIGP uint64, missingPresent, zeroAbsent bool) (uint64, error) {
	addresses := []string{"10.249.1.1", "10.249.2.1", "10.249.3.1", "10.249.4.1", "10.249.5.1"}
	want := []rpc.RouteMetric{
		{Cost: terminal, Resolved: true, Recursive: true},
		{Cost: terminal + 7, Resolved: true, Recursive: true},
		{Cost: terminal + missingAIGP, Resolved: true, Recursive: true, MissingAIGP: !missingPresent},
		{Cost: zero, Resolved: !zeroAbsent},
		{},
	}
	out, err := p.RouteMetrics(ctx, addresses)
	if err != nil {
		return 0, err
	}
	if len(out.Distances) != len(want) {
		return 0, fmt.Errorf("authoritative metrics returned %d entries, want %d", len(out.Distances), len(want))
	}
	for i, expected := range want {
		if out.Distances[i] != expected {
			return 0, fmt.Errorf("authoritative metric %s: got %+v, want %+v", addresses[i], out.Distances[i], expected)
		}
	}
	return out.Revision, nil
}

func sysribBGPMetricWinner(ctx context.Context, p *sdk.Plugin, prefix, peer, hop string) error {
	var last string
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		data, err := requireDone02(ctx, p, "show bgp rib best")
		if err != nil {
			last = err.Error()
			return false
		}
		last = text02(data)
		var answer struct {
			BestPath []struct {
				Prefix   string `json:"prefix"`
				BestPeer string `json:"best-peer"`
			} `json:"best-path"`
		}
		if decode02(data, &answer) != nil {
			return false
		}
		for _, row := range answer.BestPath {
			if row.Prefix == prefix {
				return row.BestPeer == peer
			}
		}
		return false
	}) {
		return fmt.Errorf("actual BGP winner %s is not %s: %s", prefix, peer, last)
	}
	if _, err := waitCandidates(ctx, p, prefix, func(row candidateRow) bool {
		path, found := candidatePathOf(row, "bgp")
		if !found {
			return false
		}
		if !path.Selected {
			return false
		}
		return path.NextHop == hop
	}); err != nil {
		return err
	}
	return importKernel(ctx, prefix, 1, "dev "+sysribBGPMetricLink+" ", "proto 250 ")
}

func sysribBGPMetricKernelEmpty(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("BGP metric kernel-empty fixture takes no arguments")
	}
	for _, prefix := range []string{sysribBGPMetricTerminal, sysribBGPMetricStatic, sysribBGPMetricReceived, sysribBGPMetricMissing, sysribBGPMetricZero, sysribBGPMetricSubject, sysribBGPMetricAIGPSubject, sysribBGPMetricControl, sysribBGPMetricConnected} {
		if err := importKernel(ctx, prefix, 0); err != nil {
			return err
		}
	}
	return importSay("BGP metric fixture prefixes are absent from every kernel table")
}
