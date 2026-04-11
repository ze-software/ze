// Design: docs/architecture/core-design.md -- prefix-list filter plugin
// Detail: match.go -- prefix matching algorithm and update evaluator
// Detail: config.go -- bgp/policy/prefix-list config parsing
//
// Package filter_prefix implements the bgp-filter-prefix plugin.
//
// The plugin loads named prefix-list definitions from
// bgp { policy { prefix-list NAME { entry P { ge G; le L; action A; } } } }
// at OnConfigure (Stage 2). At runtime, peer filter chains reference a list
// as bgp-filter-prefix:NAME. The engine dispatches each match via
// CallFilterUpdate (filter-update RPC); the plugin handles it in OnFilterUpdate
// by extracting the nlri field from the update text and applying the named
// list's match entries to every prefix (strict whole-update mode).
//
// The plugin declares ZERO filters at Stage 1: filter names come from config
// (Stage 2), not from compile-time registration. CallFilterUpdate does not
// gate on declared filters; FilterInfo lookup miss returns the safe defaults
// (no declared attributes, raw=false), and FilterOnError defaults to fail-closed.
package filter_prefix

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/configjson"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// Filter action wire values consumed by reactor/filter_chain.go parsePolicyAction.
const (
	filterActionAccept = "accept"
	filterActionReject = "reject"
)

var logger = slogutil.LazyLogger("bgp.filter.prefix")

// listsByName is the runtime-loaded set of prefix-list definitions, keyed by
// the YANG list name. Updated atomically on every OnConfigure delivery so
// the hot path can read without a lock.
var listsByName atomic.Pointer[map[string]*prefixList]

// RunFilterPrefix runs the prefix-list filter plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunFilterPrefix(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-prefix", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			bgpCfg, ok := configjson.ParseBGPSubtree(section.Data)
			if !ok {
				return fmt.Errorf("filter-prefix: invalid bgp config JSON")
			}
			lists, err := parsePrefixLists(bgpCfg)
			if err != nil {
				return fmt.Errorf("filter-prefix: %w", err)
			}
			listsByName.Store(&lists)
			logger().Debug("configured", "prefix-lists", len(lists))
		}
		return nil
	})

	p.OnFilterUpdate(func(in *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
		return handleFilterUpdate(in), nil
	})

	ctx := context.Background()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"bgp"},
	}); err != nil {
		logger().Error("filter-prefix plugin failed", "error", err)
		return 1
	}
	return 0
}

// handleFilterUpdate dispatches a single filter-update RPC.
// Looks up the named list by in.Filter, extracts the nlri field from the
// update text, and runs the strict whole-update evaluator. Unknown filter
// names fail closed (reject). The plugin never modifies the update -- it
// only returns accept or reject.
func handleFilterUpdate(in *sdk.FilterUpdateInput) *sdk.FilterUpdateOutput {
	listsP := listsByName.Load()
	if listsP == nil {
		logger().Warn("filter-update before configure", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: filterActionReject}
	}
	lists := *listsP
	list, ok := lists[in.Filter]
	if !ok {
		logger().Warn("unknown prefix-list", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: filterActionReject}
	}

	nlriField := extractNLRIField(in.Update)
	if list.evaluateUpdate(nlriField) {
		logger().Info("prefix-list accept", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField)
		return &sdk.FilterUpdateOutput{Action: filterActionAccept}
	}
	logger().Info("prefix-list reject", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField)
	return &sdk.FilterUpdateOutput{Action: filterActionReject}
}
