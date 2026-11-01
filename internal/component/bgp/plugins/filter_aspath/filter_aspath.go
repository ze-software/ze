// Design: docs/architecture/core-design.md -- AS-path regex filter plugin
// Detail: match.go -- regex matching algorithm
// Detail: condition.go -- the as-path condition of a match instance
// Detail: config.go -- policy/as-path-list config parsing
//
// Package filter_aspath implements the bgp-filter-aspath plugin.
//
// The package owns the as-path-list type: its YANG under
// policy { as-path-list NAME { entry REGEX { action A; } } } and the as-path
// condition a match instance names (condition.go). A BGP chain reaches a list
// only through a match instance, judged in process over the route view, so
// the plugin answers no filter-update. The plugin process takes part in the
// config transaction: it parses the lists at Stage 2 and on every applied
// reload (config_tx.go), and refuses a delivery that does not parse.
package filter_aspath

import (
	"errors"
	"net"

	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errFilterAspathInvalidBgpConfigJson = errors.New("filter-aspath: invalid bgp config JSON")

var logger = slogutil.LazyLogger("bgp.filter.aspath")

const configRootBGP = "bgp"

// runFilterAsPath runs the AS-path filter plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func runFilterAsPath(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-aspath", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	newAsPathConfigTx().Register(p)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{
		WantsConfig: []string{configRootBGP},
	}); err != nil {
		logger().Error("filter-aspath plugin failed", "error", err)
		return 1
	}
	return 0
}
