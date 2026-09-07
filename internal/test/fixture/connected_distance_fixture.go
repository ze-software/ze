// Design: docs/architecture/core-design.md -- connected prefixes in the shared Loc-RIB
// Related: ../../plugins/connected/locrib.go -- the path this scenario makes visible
// Related: register_connected_distance.go -- registers the drivers below

package fixture

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// connectedDevice is the dummy interface this scenario creates, and
// connectedPrefix is the prefix its address covers. The BGP route injected below
// names the same prefix, so the two compete for it.
const (
	connectedDevice  = "zeconn0"
	connectedAddress = "10.9.0.1/24"
	connectedPrefix  = "10.9.0.0/24"
)

// connectedDistanceWinner02 builds the scenario that reads which protocol won the
// contested prefix, and then reads the kernel to check that Ze programmed a route
// for it only where Ze was supposed to.
//
// The address is assigned while the daemon is RUNNING, on purpose. The interface
// monitor emits `interface`/`addr-added` from a netlink event, so an address that
// was already there when the daemon started produces no event and the connected
// plugin would never see the prefix. Assigning it here drives the entry point the
// feature actually has.
//
// wantProtocol is the winner the configured distance selects. wantZeRoute says
// whether an RTPROT_ZE entry should exist for the prefix afterwards: false when
// connected wins, because the kernel already holds its own route and a second
// entry from Ze would be the collision this arbitration removes.
func connectedDistanceWinner02(wantProtocol string, wantZeRoute bool) ObserverScenario {
	return func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := apiRIBReady02(ctx, plugin); err != nil {
			return err
		}
		var inject textbuf.Buffer
		inject.Str("request bgp rib inject 10.0.0.99 ipv4/unicast ").Str(connectedPrefix)
		inject.Str(" origin igp aspath 64500 nexthop 198.51.100.1")
		if _, err := requireDone02(ctx, plugin, inject.String()); err != nil {
			return err
		}
		commandIgnore(ctx, "ip", "link", "add", connectedDevice, "type", "dummy")
		commandIgnore(ctx, "ip", "link", "set", connectedDevice, "up")
		commandIgnore(ctx, "ip", "addr", "add", connectedAddress, "dev", connectedDevice)
		defer commandIgnore(context.WithoutCancel(ctx), "ip", "link", "del", connectedDevice)

		var winner, seen string
		if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
			winner, seen = ribWinner02(ctx, plugin, connectedPrefix)
			return winner == wantProtocol
		}) {
			var tb textbuf.Buffer
			tb.Str(connectedPrefix).Str(": system RIB winner is ").Str(winner)
			tb.Str(", want ").Str(wantProtocol).Str("; show rib: ").Str(seen)
			return errors.New(tb.String())
		}

		if err := checkZeRoute02(ctx, wantZeRoute); err != nil {
			return err
		}
		var ok textbuf.Buffer
		ok.Str("OK: ").Str(connectedPrefix).Str(" is won by ").Str(wantProtocol)
		ok.Str(", ze-programmed=").Str(boolText02(wantZeRoute)).Byte('\n')
		return ok.StdErr()
	}
}

// checkZeRoute02 reads the kernel and answers whether it agrees about who
// programmed the contested prefix. It lists only the routes fib-kernel owns, so
// the kernel's own connected route for the same prefix is never mistaken for one
// of Ze's.
func checkZeRoute02(ctx context.Context, want bool) error {
	var routes string
	Poll(ctx, 20, 100*time.Millisecond, func() bool {
		routes = zeRoutesOutput02(ctx)
		return strings.Contains(routes, connectedPrefix) == want
	})
	if strings.Contains(routes, connectedPrefix) == want {
		return nil
	}
	var tb textbuf.Buffer
	if want {
		tb.Str("no Ze-programmed route for ").Str(connectedPrefix)
		tb.Str(", but the winner is a protocol Ze installs; proto 250 routes: ").Str(routes)
	} else {
		tb.Str("a Ze-programmed route for ").Str(connectedPrefix)
		tb.Str(" survives beside the kernel's own connected route; proto 250 routes: ").Str(routes)
	}
	return errors.New(tb.String())
}

// zeRoutesOutput02 lists only the routes Ze's kernel FIB plugin owns, which it
// writes under RTPROT_ZE (250).
func zeRoutesOutput02(ctx context.Context) string {
	out, _ := netfilterCommandOutput(ctx, "ip", "route", "show", "proto", "250")
	return out
}

func boolText02(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
