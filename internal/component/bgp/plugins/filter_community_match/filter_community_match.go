// Design: docs/architecture/core-design.md -- community match filter plugin
// Detail: match.go -- the community entry types
// Detail: condition.go -- the community condition of a match instance
// Detail: config.go -- policy/community-match config parsing
//
// Package filter_community_match implements the bgp-filter-community-match plugin.
//
// The package owns the community-match type: its YANG under
// policy { community-match NAME { entry COMMUNITY { type T; action A; } } }
// and the community condition a match instance names (condition.go). A BGP
// chain reaches a list only through a match instance, judged in process over
// the route view, so the plugin answers no filter-update. The plugin process
// takes part in the config transaction: it parses the lists at Stage 2 and on
// every applied reload (config_tx.go), and refuses a delivery that does not
// parse.
//
// Separate from the tag/strip community plugin (bgp-filter-community) because
// intent differs: this condition filters (accept/reject), that plugin
// modifies (tag/strip). A match instance and that plugin's filter can coexist
// in the same filter chain.
package filter_community_match

import (
	"errors"
	"net"

	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errFilterCommunityMatchInvalidBgpConfig = errors.New("filter-community-match: invalid bgp config JSON")

var logger = slogutil.LazyLogger("bgp.filter.community.match")

const configRootBGP = "bgp"

// runFilterCommunityMatch runs the community match filter plugin using the SDK RPC protocol.
func runFilterCommunityMatch(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-community-match", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	newCommunityMatchConfigTx().Register(p)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{
		WantsConfig: []string{configRootBGP},
	}); err != nil {
		logger().Error("filter-community-match plugin failed", "error", err)
		return 1
	}
	return 0
}
