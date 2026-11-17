// Design: docs/architecture/route-selection.md -- prospective next-hop admission
// Related: rfc4271_prospective_readers.go -- exact received and forwarding state
// Related: rfc4271_next_hop_daemon_fixture.go -- real BGP transport and kernel readers
package fixture

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	rfc4271ProspectiveCover        = "198.19.0.0/16"
	rfc4271ProspectiveDirect       = "198.19.152.0/24"
	rfc4271ProspectiveIndirect     = "198.19.153.0/24"
	rfc4271ProspectiveProtected    = "198.19.154.0/24"
	rfc4271ProspectiveHost         = "198.19.154.1/32"
	rfc4271ProspectiveRecursive    = "198.18.231.0/24"
	rfc4271ProspectiveDirectHop    = "198.19.152.1"
	rfc4271ProspectiveIndirectHop  = "198.18.231.1"
	rfc4271ProspectiveProtectedHop = "198.19.154.1"
	rfc4271ProspectiveGateway      = "198.18.230.1"
)

// Only interior routes use RouteInstall. Every subject is first selected from
// real TCP ingress and installed in Linux before its received next hop changes.
func rfc4271ProspectiveKernelDriver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("prospective kernel fixture needs placement and BGP listen address")
	}
	if err := importPlacementArgs(args[:1], 1); err != nil {
		return err
	}
	capture := newRFC4271ProspectiveCapture()
	return observeConfigured(ctx, "rfc4271-prospective-test", &sdk.Registration{}, capture.setup, func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := sysribExecutionPlacement(ctx, plugin, "internal"); err != nil {
			return err
		}
		if err := sysribExecutionPluginPlacement(ctx, plugin, "bgp-rib", "bgp-rib", args[0]); err != nil {
			return err
		}
		if err := importCreateLink(ctx, rfc4271NextHopLink, "198.18.230.254/24"); err != nil {
			return err
		}
		if err := waitRIBWinner(ctx, plugin, rfc4271NextHopConnected, "connected"); err != nil {
			return err
		}
		cover := rpc.RouteInstallEntry{Protocol: protocolOSPF, AFI: 1, SAFI: 1, Prefix: rfc4271ProspectiveCover,
			NextHop: rfc4271ProspectiveGateway, Interface: rfc4271NextHopLink, OnLink: true, Metric: 10, RouteType: 1}
		host := cover
		host.Prefix = rfc4271ProspectiveHost
		recursive := rpc.RouteInstallEntry{Protocol: protocolOSPF, AFI: 1, SAFI: 1, Prefix: rfc4271ProspectiveRecursive,
			NextHop: "198.19.153.1", Metric: 5, MetricRecursive: true, RouteType: 1}
		if err := sysribInstall(ctx, plugin, cover, host, recursive); err != nil {
			return err
		}
		for _, prefix := range []string{cover.Prefix, host.Prefix, recursive.Prefix} {
			if err := waitRIBWinner(ctx, plugin, prefix, protocolOSPF); err != nil {
				return err
			}
		}
		peer, err := rfc4271NextHopConnect(ctx, plugin, args[1], rfc4271NextHopPeerA, 0x02020202)
		if err != nil {
			return err
		}
		defer peer.Close() //nolint:errcheck // Transport closes after all state assertions.
		control, err := rfc4271NextHopConnect(ctx, plugin, args[1], rfc4271NextHopPeerB, 0x03030303)
		if err != nil {
			return err
		}
		defer control.Close() //nolint:errcheck // Transport closes after all state assertions.
		for _, prefix := range []string{rfc4271ProspectiveDirect, rfc4271ProspectiveIndirect, rfc4271ProspectiveProtected} {
			if err := rfc4271NextHopUpdate(peer, prefix, rfc4271NextHopDirect, 200); err != nil {
				return err
			}
			if err := rfc4271NextHopWinner(ctx, plugin, prefix, rfc4271NextHopPeerA, rfc4271NextHopDirect, rfc4271NextHopDirect); err != nil {
				return fmt.Errorf("subject was not installed before replacement: %w", err)
			}
		}
		if err := rfc4271NextHopUpdate(control, rfc4271NextHopControl, rfc4271NextHopDirect, 100); err != nil {
			return err
		}
		if err := rfc4271ProspectiveSurvivors(ctx, plugin, false); err != nil {
			return err
		}
		expected := rfc4271ProspectiveInitialReceived()
		if err := rfc4271ProspectiveReceived(ctx, plugin, expected); err != nil {
			return err
		}
		for _, subject := range []struct{ prefix, nextHop string }{
			{rfc4271ProspectiveDirect, rfc4271ProspectiveDirectHop},
			{rfc4271ProspectiveIndirect, rfc4271ProspectiveIndirectHop},
		} {
			// Current-table resolution MUST succeed before the replacement. Its
			// failure would test ordinary unreachable admission, not the preview.
			if err := rfc4271ProspectiveResolved(ctx, plugin, subject.nextHop); err != nil {
				return err
			}
			if err := capture.arm(ctx, subject.prefix); err != nil {
				return err
			}
			if err := rfc4271NextHopUpdate(peer, subject.prefix, subject.nextHop, 300); err != nil {
				return err
			}
			expected[subject.prefix] = rfc4271ProspectiveReceivedPath{Peer: rfc4271NextHopPeerA, NextHop: subject.nextHop, Preference: 300}
			if err := rfc4271ProspectiveReceived(ctx, plugin, expected); err != nil {
				return err
			}
			if err := capture.withdrawn(ctx, subject.prefix); err != nil {
				return err
			}
			if err := rfc4271ProspectiveAbsent(ctx, plugin, subject.prefix); err != nil {
				return err
			}
		}
		if err := rfc4271ProspectiveResolved(ctx, plugin, rfc4271ProspectiveProtectedHop); err != nil {
			return err
		}
		if err := rfc4271NextHopUpdate(peer, rfc4271ProspectiveProtected, rfc4271ProspectiveProtectedHop, 300); err != nil {
			return err
		}
		expected[rfc4271ProspectiveProtected] = rfc4271ProspectiveReceivedPath{Peer: rfc4271NextHopPeerA, NextHop: rfc4271ProspectiveProtectedHop, Preference: 300}
		if err := rfc4271ProspectiveReceived(ctx, plugin, expected); err != nil {
			return err
		}
		if err := rfc4271ProspectiveSurvivors(ctx, plugin, true); err != nil {
			return err
		}
		for _, prefix := range []string{rfc4271ProspectiveDirect, rfc4271ProspectiveIndirect} {
			if err := capture.withdrawn(ctx, prefix); err != nil {
				return err
			}
			if err := rfc4271ProspectiveAbsent(ctx, plugin, prefix); err != nil {
				return err
			}
		}
		return importSay("real " + args[0] + " BGP replacements reject direct and indirect prospective self-capture after installed seeds, retain exact received attributes, remove best/Loc-RIB/all-table kernel subjects, and preserve interior covers, a longer host route and an independent direct control")
	})
}
