// Design: docs/architecture/core-design.md -- AS-path length policy filter
// Related: aspath_length.go -- path length evaluation
// Related: condition.go -- the as-path-length condition of a match instance
// Related: config.go -- policy/as-path-length config parsing

package filter_aspath_length

import (
	"errors"
	"net"

	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errAsPathLengthInvalidBgpConfigJSON = errors.New("filter-aspath-length: invalid bgp config JSON")

var logger = slogutil.LazyLogger("bgp.filter.aspath-length")

const configRootBGP = "bgp"

// RunFilterAsPathLength runs the plugin process, which takes part in the config
// transaction: it parses the definitions at Stage 2 and on every reload, and
// refuses a delivery that does not parse.
func RunFilterAsPathLength(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-aspath-length", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	newAsPathLengthConfigTx().Register(p)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{
		WantsConfig: []string{configRootBGP},
	}); err != nil {
		logger().Error("filter-aspath-length plugin failed", "error", err)
		return 1
	}
	return 0
}
