// Design: docs/architecture/static-routes.md -- a named-table static route keeps the direct data-plane write
// Related: ../../plugins/static/locrib.go -- inMainTable, the boundary this scenario observes
// Related: static_distance_fixture.go -- ribWinner02, the system RIB read shared with the distance drivers
// Related: register_static_distance.go -- registers the two drivers below

package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// The named-table scenario's addresses. The gateway subnet is assigned to a
// dummy link by the setup driver, so the kernel accepts a route through it in any
// table. One prefix sits in the main table and one in the named table, so the
// scenario can show the system RIB sees static routes before it asserts that it
// does not see the named-table one.
const (
	namedTableLink    = "zent171"
	namedTableAddress = "192.0.2.2/24"
	namedTableID      = "171"
	namedTablePrefix  = "198.18.71.0/24"
	mainTablePrefix   = "198.18.72.0/24"
	namedTableGateway = "192.0.2.1"

	// protocolStatic is the protocol name the system RIB reports for a static
	// Loc-RIB path.
	protocolStatic = "static"

	// namedTablePolls bounds each of the scenario's three waits to six seconds,
	// so a failing wait reports its own reason inside the .ci timeout instead
	// of the runner killing the daemon first.
	namedTablePolls = 60
)

// staticNamedTableSetup creates the link that makes the gateway reachable.
func staticNamedTableSetup(ctx context.Context, _ []string) error {
	return staticSetup(namedTableLink, namedTableAddress)(ctx, nil)
}

// staticNamedTableUnchanged proves the main-table boundary from the operator's
// side: a route in a named table is programmed straight into that table by the
// static plugin, stamped with the static protocol (251), and never becomes a
// Loc-RIB path, so the system RIB does not list it and the FIB plugin programs no
// main-table entry for it.
//
// Method: wait until the MAIN-table static route is both in the system RIB and in
// the kernel's main table. That proves the observer can see a static Loc-RIB path
// and a FIB-programmed route, so the absences asserted afterwards are evidence
// rather than an empty read. Then read the named table, the main table and the
// system RIB for the named-table prefix.
func staticNamedTableUnchanged(ctx context.Context, plugin *sdk.Plugin) error {
	var winner, seen string
	if !Poll(ctx, namedTablePolls, 100*time.Millisecond, func() bool {
		winner, seen = ribWinner02(ctx, plugin, mainTablePrefix)
		return winner == protocolStatic
	}) {
		return fmt.Errorf("main-table static %s never reached the system RIB (winner %q); show rib: %s",
			mainTablePrefix, winner, seen)
	}
	var mainRoute string
	if !Poll(ctx, namedTablePolls, 100*time.Millisecond, func() bool {
		mainRoute = kernelRoute(ctx, "main", mainTablePrefix)
		return strings.Contains(mainRoute, "proto 250")
	}) {
		return fmt.Errorf("main-table static %s was never programmed by the FIB plugin as proto 250; ip route: %q",
			mainTablePrefix, mainRoute)
	}

	var namedRoute string
	if !Poll(ctx, namedTablePolls, 100*time.Millisecond, func() bool {
		namedRoute = kernelRoute(ctx, namedTableID, namedTablePrefix)
		return namedRoute != ""
	}) {
		return fmt.Errorf("named-table static %s never reached table %s", namedTablePrefix, namedTableID)
	}
	if !strings.Contains(namedRoute, "via "+namedTableGateway) {
		return fmt.Errorf("table %s route %q does not go via %s", namedTableID, namedRoute, namedTableGateway)
	}
	if !strings.Contains(namedRoute, "proto 251") {
		return fmt.Errorf("table %s route %q is not stamped proto 251 by the static plugin", namedTableID, namedRoute)
	}
	if leaked := kernelRoute(ctx, "main", namedTablePrefix); leaked != "" {
		return fmt.Errorf("named-table static %s leaked into the main table: %q", namedTablePrefix, leaked)
	}
	if protocol, raw := ribWinner02(ctx, plugin, namedTablePrefix); protocol != "" {
		return fmt.Errorf("named-table static %s became a Loc-RIB path won by %q; show rib: %s",
			namedTablePrefix, protocol, raw)
	}
	fmt.Fprintln(os.Stderr, "OK: "+namedTablePrefix+" is in table "+namedTableID+" as proto 251, absent from the main table and the system RIB")
	return nil
}

// kernelRoute returns the kernel's entry for exactly prefix in table, trimmed,
// or "" when the table holds none. -N prints the protocol as its number, so an
// rt_protos name mapping on the host cannot hide 250 or 251.
func kernelRoute(ctx context.Context, table, prefix string) string {
	out, err := netfilterCommandOutput(ctx, "ip", "-N", "route", "show", "table", table, "exact", prefix)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
