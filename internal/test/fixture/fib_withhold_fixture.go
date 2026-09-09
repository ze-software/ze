// Design: docs/architecture/core-design.md -- selection, and the FIB write that follows it
// Related: ../../component/sysrib/fibimport.go -- the gate this scenario drives
// Related: static_distance_fixture.go -- ribWinner02, the `show rib` read reused here
// Related: register_fib_withhold.go -- registers the driver below

package fixture

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// The scenario installs one route for each of two protocols, over the
// protocol-agnostic route-install RPC, so it needs no OSPF adjacency and no BGP
// peer to put an OSPF route and a BGP route in the system RIB.
//
// The next-hop is on-link on a dummy device the scenario creates, because the
// kernel refuses a route whose next-hop it cannot reach and the test would then
// prove nothing about the withhold.
const (
	fibWithholdDevice    = "zefibw0"
	fibWithholdAddress   = "192.0.2.254/24"
	fibWithholdNextHop   = "192.0.2.1"
	fibWithholdPrefix    = "10.98.0.0/24"
	fibWithholdKept      = "10.97.0.0/24"
	fibWithholdProtocol  = "bgp"
	fibWithholdKeptProto = "ospf"
)

// fibWithholdOneProtocol drives the owner's own example: `rib { fib-withhold [
// bgp ] }` with an OSPF route beside the BGP one. Both routes win their own
// prefix in the system RIB, and only the OSPF one reaches the kernel.
//
// The BGP half is the assertion that matters, and it has two parts that MUST
// both hold: no kernel entry for the prefix, and the prefix still in the RIB. A
// test that checked only the kernel would pass just as well against a Ze that
// dropped the route on the floor, which is the failure the whole design exists
// to avoid.
func fibWithholdOneProtocol(ctx context.Context, plugin *sdk.Plugin) error {
	commandIgnore(ctx, "ip", "link", "del", fibWithholdDevice)
	commandIgnore(ctx, "ip", "link", "add", fibWithholdDevice, "type", "dummy")
	commandIgnore(ctx, "ip", "link", "set", fibWithholdDevice, "up")
	commandIgnore(ctx, "ip", "addr", "add", fibWithholdAddress, "dev", fibWithholdDevice)
	defer commandIgnore(context.WithoutCancel(ctx), "ip", "link", "del", fibWithholdDevice)

	routes := []rpc.RouteInstallEntry{
		fibWithholdRoute(fibWithholdProtocol, fibWithholdPrefix, 20),
		fibWithholdRoute(fibWithholdKeptProto, fibWithholdKept, 110),
	}
	installed, err := plugin.RouteInstall(ctx, routes)
	if err != nil {
		return err
	}
	if int(installed) != len(routes) {
		var tb textbuf.Buffer
		tb.Str("route-install accepted ").Uint(uint64(installed)).Str(" of ").Uint(uint64(len(routes)))
		tb.Str(" routes: the daemon must register both ").Str(fibWithholdProtocol)
		tb.Str(" and ").Str(fibWithholdKeptProto)
		return errors.New(tb.String())
	}

	if err := fibWithholdRIBHolds(ctx, plugin, fibWithholdPrefix, fibWithholdProtocol); err != nil {
		return err
	}
	if err := fibWithholdRIBHolds(ctx, plugin, fibWithholdKept, fibWithholdKeptProto); err != nil {
		return err
	}

	// The kept protocol first: it proves the chain from route-install to the
	// kernel works in this environment, so the absence checked after it is the
	// withhold rather than a chain that never programmed anything.
	var kernel string
	programmed := Poll(ctx, 40, 250*time.Millisecond, func() bool {
		kernel = fibWithholdZeRoutes(ctx)
		return strings.Contains(kernel, fibWithholdKept)
	})
	if !programmed {
		var tb textbuf.Buffer
		tb.Str("the kept protocol ").Str(fibWithholdKeptProto).Str(" never programmed ")
		tb.Str(fibWithholdKept).Str("; proto 250 routes: ").Str(kernel)
		return errors.New(tb.String())
	}
	if strings.Contains(kernel, fibWithholdPrefix) {
		var tb textbuf.Buffer
		tb.Str("the withheld protocol ").Str(fibWithholdProtocol).Str(" programmed ")
		tb.Str(fibWithholdPrefix).Str("; proto 250 routes: ").Str(kernel)
		return errors.New(tb.String())
	}

	var ok textbuf.Buffer
	ok.Str("OK: ").Str(fibWithholdPrefix).Str(" is won by ").Str(fibWithholdProtocol)
	ok.Str(" and is not programmed, ").Str(fibWithholdKept).Str(" is won by ")
	ok.Str(fibWithholdKeptProto).Str(" and is programmed").Byte('\n')
	return ok.StdErr()
}

// fibWithholdRoute builds one route-install entry. The distance is the
// protocol's own, so the two prefixes are ranked the way a running router ranks
// them.
func fibWithholdRoute(protocol, prefix string, distance uint8) rpc.RouteInstallEntry {
	return rpc.RouteInstallEntry{
		Protocol:      protocol,
		AFI:           1,
		SAFI:          1,
		Prefix:        prefix,
		NextHop:       fibWithholdNextHop,
		AdminDistance: distance,
		Metric:        10,
	}
}

// fibWithholdRIBHolds waits for the system RIB to report the prefix as won by
// the protocol, which is the half of the assertion that says a withheld route is
// still a selected route.
func fibWithholdRIBHolds(ctx context.Context, plugin *sdk.Plugin, prefix, protocol string) error {
	var winner, seen string
	if Poll(ctx, 40, 250*time.Millisecond, func() bool {
		winner, seen = ribWinner02(ctx, plugin, prefix)
		return winner == protocol
	}) {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str(prefix).Str(": system RIB winner is ").Str(winner).Str(", want ").Str(protocol)
	tb.Str("; show rib: ").Str(seen)
	return errors.New(tb.String())
}

// fibWithholdZeRoutes lists only the routes Ze's kernel FIB plugin owns, which
// it writes under RTPROT_ZE (250), so a route the kernel created itself is never
// read as one of Ze's.
func fibWithholdZeRoutes(ctx context.Context) string {
	out, _ := netfilterCommandOutput(ctx, "ip", "route", "show", "proto", "250")
	return out
}
