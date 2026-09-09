// Design: docs/architecture/core-design.md -- selection, and the FIB write that follows it
// Related: fib_withhold_fixture.go -- the one-prefix scenario this one scales up
// Related: ../../component/sysrib/fibimport.go -- the gate this scenario drives
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

// The controller deployment: a box that holds every peer's routes in the RIB
// and serves them on the plugin API, and forwards no traffic, so a kernel route
// for each prefix is work with no reader.
//
// The scale is the point. Every other scenario for this feature carries one or
// two prefixes, so a gate that programs one prefix in a hundred passes them all.
//
// The next-hop is on-link on a dummy device the scenario creates, because the
// kernel refuses a route whose next-hop it cannot reach and the absence this
// test reads would then say nothing about the withhold. The address is TEST-NET-2
// and the device name is this scenario's own, so a sibling scenario running
// beside it shares neither.
const (
	fibControllerDevice    = "zefibc0"
	fibControllerAddress   = "198.51.100.254/24"
	fibControllerNextHop   = "198.51.100.1"
	fibControllerKept      = "10.121.0.0/24"
	fibControllerProtocol  = "bgp"
	fibControllerKeptProto = "ospf"
	fibControllerRoutes    = 200
)

// fibWithholdController drives the deployment the feature exists for: `rib {
// fib-withhold [ bgp ] }` with a whole BGP table in the RIB. Every prefix is
// won by bgp in the system RIB, and the kernel holds not one of them.
//
// The OSPF prefix beside the table is the control. It is programmed through the
// same batch, so the absence checked after it is the withhold rather than a
// chain that programmed nothing at all.
func fibWithholdController(ctx context.Context, plugin *sdk.Plugin) error {
	commandIgnore(ctx, "ip", "link", "del", fibControllerDevice)
	commandIgnore(ctx, "ip", "link", "add", fibControllerDevice, "type", "dummy")
	commandIgnore(ctx, "ip", "link", "set", fibControllerDevice, "up")
	commandIgnore(ctx, "ip", "addr", "add", fibControllerAddress, "dev", fibControllerDevice)
	defer commandIgnore(context.WithoutCancel(ctx), "ip", "link", "del", fibControllerDevice)

	table := fibControllerTable()
	routes := make([]rpc.RouteInstallEntry, 0, len(table)+1)
	for _, prefix := range table {
		routes = append(routes, fibControllerRoute(fibControllerProtocol, prefix, 20))
	}
	routes = append(routes, fibControllerRoute(fibControllerKeptProto, fibControllerKept, 110))

	installed, err := plugin.RouteInstall(ctx, routes)
	if err != nil {
		return err
	}
	if int(installed) != len(routes) {
		var tb textbuf.Buffer
		tb.Str("route-install accepted ").Uint(uint64(installed)).Str(" of ").Uint(uint64(len(routes)))
		tb.Str(" routes: the daemon must register both ").Str(fibControllerProtocol)
		tb.Str(" and ").Str(fibControllerKeptProto)
		return errors.New(tb.String())
	}

	if err := fibControllerRIBHoldsTable(ctx, plugin, table); err != nil {
		return err
	}

	// The kept protocol first: it proves the chain from route-install to the
	// kernel works in this environment.
	var kernel string
	programmed := Poll(ctx, 40, 250*time.Millisecond, func() bool {
		kernel = fibWithholdZeRoutes(ctx)
		return strings.Contains(kernel, fibControllerKept)
	})
	if !programmed {
		var tb textbuf.Buffer
		tb.Str("the kept protocol ").Str(fibControllerKeptProto).Str(" never programmed ")
		tb.Str(fibControllerKept).Str("; proto 250 routes: ").Str(kernel)
		return errors.New(tb.String())
	}

	if err := fibControllerKernelHoldsNone(table, kernel); err != nil {
		return err
	}

	var ok textbuf.Buffer
	ok.Str("OK: ").Uint(uint64(len(table))).Str(" ").Str(fibControllerProtocol)
	ok.Str(" prefixes are in the rib and none is programmed, ").Str(fibControllerKept)
	ok.Str(" is won by ").Str(fibControllerKeptProto).Str(" and is programmed").Byte('\n')
	return ok.StdErr()
}

// fibControllerTable builds the withheld table. Each prefix takes its own third
// octet, so no prefix in it is a substring of another and the kernel read below
// cannot answer about the wrong route.
func fibControllerTable() []string {
	prefixes := make([]string, 0, fibControllerRoutes)
	for i := range fibControllerRoutes {
		var tb textbuf.Buffer
		tb.Str("10.120.").Uint(uint64(i)).Str(".0/24")
		prefixes = append(prefixes, tb.String())
	}
	return prefixes
}

// fibControllerRoute builds one route-install entry over the scenario's shared
// on-link gateway, with the protocol's own administrative distance.
func fibControllerRoute(protocol, prefix string, distance uint8) rpc.RouteInstallEntry {
	return rpc.RouteInstallEntry{
		Protocol:      protocol,
		AFI:           1,
		SAFI:          1,
		Prefix:        prefix,
		NextHop:       fibControllerNextHop,
		AdminDistance: distance,
		Metric:        10,
	}
}

// fibControllerRIBHoldsTable waits for the system RIB to report every prefix of
// the table as won by the withheld protocol, which is the half of the assertion
// that says a withheld table is still a selected table.
func fibControllerRIBHoldsTable(ctx context.Context, plugin *sdk.Plugin, table []string) error {
	var held int
	var absent, raw string
	if Poll(ctx, 40, 250*time.Millisecond, func() bool {
		var winners map[string]string
		winners, raw = fibControllerRIBWinners(ctx, plugin)
		held, absent = 0, ""
		for _, prefix := range table {
			if winners[prefix] == fibControllerProtocol {
				held++
				continue
			}
			if absent == "" {
				absent = prefix
			}
		}
		return held == len(table)
	}) {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("the system RIB holds ").Uint(uint64(held)).Str(" of the ").Uint(uint64(len(table)))
	tb.Str(" prefixes as won by ").Str(fibControllerProtocol).Str("; the first one it does not is ")
	tb.Str(absent).Str("; show rib: ").Str(raw)
	return errors.New(tb.String())
}

// fibControllerRIBWinners reads `show rib` once and answers the winner for each
// prefix, so one command serves a whole table. It returns the raw answer beside
// the map, which is what the caller reports when the map is short.
func fibControllerRIBWinners(ctx context.Context, plugin *sdk.Plugin) (winners map[string]string, raw string) {
	data, err := requireDone02(ctx, plugin, "show rib")
	if err != nil {
		return nil, err.Error()
	}
	raw = text02(data)
	var rows []struct {
		Prefix   string `json:"prefix"`
		Protocol string `json:"protocol"`
	}
	if decodeErr := decode02(data, &rows); decodeErr != nil {
		return nil, raw
	}
	winners = make(map[string]string, len(rows))
	for _, row := range rows {
		winners[row.Prefix] = row.Protocol
	}
	return winners, raw
}

// fibControllerKernelHoldsNone reads the withheld table back out of the routes
// ze owns in the kernel. Not one of them is owed an entry, and the count says
// how wide a failure is: one prefix programmed is a gate that misses a case,
// and the whole table programmed is a gate that never ran.
func fibControllerKernelHoldsNone(table []string, kernel string) error {
	var programmed int
	var first string
	for _, prefix := range table {
		if !strings.Contains(kernel, prefix) {
			continue
		}
		if first == "" {
			first = prefix
		}
		programmed++
	}
	if programmed == 0 {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("the withheld protocol ").Str(fibControllerProtocol).Str(" programmed ")
	tb.Uint(uint64(programmed)).Str(" of its ").Uint(uint64(len(table))).Str(" prefixes, the first ")
	tb.Str(first).Str("; proto 250 routes: ").Str(kernel)
	return errors.New(tb.String())
}
