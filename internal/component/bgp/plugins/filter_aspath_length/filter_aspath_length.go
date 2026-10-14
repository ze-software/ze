// Design: docs/architecture/core-design.md -- AS-path length policy filter
// Related: aspath_length.go -- path length evaluation
// Related: config.go -- bgp/policy/as-path-length config parsing

package filter_aspath_length

import (
	"errors"
	"net"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/filtertext"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errAsPathLengthInvalidBgpConfigJSON = errors.New("filter-aspath-length: invalid bgp config JSON")

var logger = slogutil.LazyLogger("bgp.filter.aspath-length")

const configRootBGP = "bgp"

var defsByName atomic.Pointer[map[string]*asPathLengthDef]

func RunFilterAsPathLength(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-aspath-length", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	newAsPathLengthConfigTx().Register(p)

	p.OnFilterUpdate(func(in *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
		return handleFilterUpdate(in), nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{configRootBGP},
	}); err != nil {
		logger().Error("filter-aspath-length plugin failed", "error", err)
		return 1
	}
	return 0
}

func handleFilterUpdate(in *sdk.FilterUpdateInput) *sdk.FilterUpdateOutput {
	defsP := defsByName.Load()
	if defsP == nil {
		logger().Warn("filter-update before configure", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	defs := *defsP
	def, ok := defs[in.Filter]
	if !ok {
		logger().Warn("unknown as-path-length filter", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	pathLen := countASPathHops(filtertext.ASPath(in.Update))

	if evaluateASPathLength(pathLen, def) {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
	}

	logger().Info("as-path-length reject", "filter", in.Filter, "peer", in.Peer,
		"length", pathLen, "max", def.max, "min", def.min)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
}
