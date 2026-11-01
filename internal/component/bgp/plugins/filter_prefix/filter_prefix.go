// Design: docs/architecture/core-design.md -- prefix-list filter plugin
// Detail: match.go -- prefix matching algorithm
// Detail: condition.go -- the prefix-list condition of a match instance
// Detail: config.go -- policy/prefix-list config parsing
// Detail: config_tx.go -- the prefix lists at boot and on reload
//
// Package filter_prefix implements the bgp-filter-prefix plugin.
//
// The package owns the prefix-list type: its YANG under
// policy { prefix-list NAME { entry P { ge G; le L; action A; } } },
// the prefix-list condition a match instance names (condition.go), and the
// Loc-RIB import gate's evaluator (rib_evaluator.go). A BGP chain reaches a
// list only through a match instance, judged in process over the route view,
// so the plugin answers no filter-update. The plugin process takes part in the
// config transaction: it parses the lists at Stage 2 and on every reload, and
// refuses a delivery that does not parse.
package filter_prefix

import (
	"errors"
	"net"

	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errFilterPrefixInvalidBgpConfigJson = errors.New("filter-prefix: invalid bgp config JSON")

var logger = slogutil.LazyLogger("bgp.filter.prefix")

const configRootBGP = "bgp"

// RunFilterPrefix runs the prefix-list filter plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunFilterPrefix(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-prefix", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	newPrefixConfigTx().Register(p)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{
		WantsConfig: []string{configRootBGP},
	}); err != nil {
		logger().Error("filter-prefix plugin failed", "error", err)
		return 1
	}
	return 0
}
