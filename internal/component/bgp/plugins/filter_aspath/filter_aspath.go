// Design: docs/architecture/core-design.md -- AS-path regex filter plugin
// Detail: match.go -- regex matching algorithm
// Detail: config.go -- bgp/policy/as-path-list config parsing
// Related: internal/component/bgp/filtertext -- the reader of the filter text format
//
// Package filter_aspath implements the bgp-filter-aspath plugin.
//
// The plugin loads named as-path-list definitions from
// bgp { policy { as-path-list NAME { entry REGEX { action A; } } } }
// at OnConfigure (Stage 2). At runtime, peer filter chains reference a list
// as bgp-filter-aspath:NAME. The engine dispatches each match via
// CallFilterUpdate (filter-update RPC); the plugin handles it in OnFilterUpdate
// by reading the as-path field out of the update text with filtertext.ASPath
// and matching the space-separated decimal string against the list's ordered
// regex entries.
//
// The plugin declares ZERO filters at Stage 1: filter names come from config
// (Stage 2), not from compile-time registration. CallFilterUpdate does not
// gate on declared filters; FilterInfo lookup miss returns the safe defaults
// (no declared attributes, raw=false), and FilterOnError defaults to fail-closed.
package filter_aspath

import (
	"errors"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/filtertext"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errFilterAspathInvalidBgpConfigJson = errors.New("filter-aspath: invalid bgp config JSON")

var logger = slogutil.LazyLogger("bgp.filter.aspath")

const configRootBGP = "bgp"

// listsByName is the runtime-loaded set of as-path-list definitions, keyed by
// the YANG list name. Updated atomically on every OnConfigure delivery so
// the hot path can read without a lock.
var listsByName atomic.Pointer[map[string]*aspathList]

// runFilterAsPath runs the AS-path filter plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func runFilterAsPath(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-aspath", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != configRootBGP {
				continue
			}
			bgpCfg, ok := configjson.ParseBGPSubtree(section.Data)
			if !ok {
				return errFilterAspathInvalidBgpConfigJson
			}
			lists, err := parseAsPathLists(bgpCfg)
			if err != nil {
				return fmt.Errorf("filter-aspath: %w", err)
			}
			listsByName.Store(&lists)
			logger().Debug("configured", "as-path-lists", len(lists))
		}
		return nil
	})

	p.OnFilterUpdate(func(in *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
		return handleFilterUpdate(in), nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{configRootBGP},
	}); err != nil {
		logger().Error("filter-aspath plugin failed", "error", err)
		return 1
	}
	return 0
}

// handleFilterUpdate dispatches a single filter-update RPC.
// Looks up the named list by in.Filter, extracts the as-path field from the
// update text, normalizes it, and runs the regex evaluator. Outcomes:
//
//   - AS-path matches an accept entry: accept
//   - AS-path matches a reject entry: reject
//   - No entry matches: reject (implicit deny)
//   - Unknown filter name: reject (fail-closed)
//   - No AS-path attribute in update: match against empty string ""
func handleFilterUpdate(in *sdk.FilterUpdateInput) *sdk.FilterUpdateOutput {
	listsP := listsByName.Load()
	if listsP == nil {
		logger().Warn("filter-update before configure", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	lists := *listsP
	list, ok := lists[in.Filter]
	if !ok {
		logger().Warn("unknown as-path-list", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	asPathStr := filtertext.ASPath(in.Update)
	result := evaluateASPath(list.entries, asPathStr)

	if result == actionAccept {
		logger().Info("as-path-list accept", "filter", in.Filter, "peer", in.Peer, "as-path", asPathStr)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
	}

	logger().Info("as-path-list reject", "filter", in.Filter, "peer", in.Peer, "as-path", asPathStr)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
}
