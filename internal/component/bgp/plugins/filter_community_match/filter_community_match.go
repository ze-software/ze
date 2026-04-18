// Design: docs/architecture/core-design.md -- community match filter plugin
// Detail: match.go -- community matching algorithm and field extraction
// Detail: config.go -- bgp/policy/community-match config parsing
//
// Package filter_community_match implements the bgp-filter-community-match plugin.
//
// The plugin loads named community-match definitions from
// bgp { policy { community-match NAME { entry COMMUNITY { type T; action A; } } } }
// at OnConfigure (Stage 2). At runtime, peer filter chains reference a list
// as bgp-filter-community-match:NAME or community-match:NAME. The engine
// dispatches each match via CallFilterUpdate (filter-update RPC); the plugin
// handles it in OnFilterUpdate by extracting community fields from the update
// text and checking for presence of the entry's community value.
//
// Separate from the tag/strip community plugin (bgp-filter-community) because
// intent differs: this plugin filters (accept/reject), that one modifies
// (tag/strip). They can coexist in the same filter chain.
//
// The plugin declares ZERO filters at Stage 1: filter names come from config
// (Stage 2), not from compile-time registration.
package filter_community_match

import (
	"fmt"
	"net"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/configjson"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

const (
	filterActionAccept = "accept"
	filterActionReject = "reject"
)

var logger = slogutil.LazyLogger("bgp.filter.community.match")

// listsByName is the runtime-loaded set of community-match definitions.
// Updated atomically on every OnConfigure delivery.
var listsByName atomic.Pointer[map[string]*communityList]

// RunFilterCommunityMatch runs the community match filter plugin using the SDK RPC protocol.
func RunFilterCommunityMatch(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-community-match", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			bgpCfg, ok := configjson.ParseBGPSubtree(section.Data)
			if !ok {
				return fmt.Errorf("filter-community-match: invalid bgp config JSON")
			}
			lists, err := parseCommunityLists(bgpCfg)
			if err != nil {
				return fmt.Errorf("filter-community-match: %w", err)
			}
			listsByName.Store(&lists)
			logger().Debug("configured", "community-lists", len(lists))
		}
		return nil
	})

	p.OnFilterUpdate(func(in *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
		return handleFilterUpdate(in), nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"bgp"},
	}); err != nil {
		logger().Error("filter-community-match plugin failed", "error", err)
		return 1
	}
	return 0
}

// handleFilterUpdate dispatches a single filter-update RPC.
func handleFilterUpdate(in *sdk.FilterUpdateInput) *sdk.FilterUpdateOutput {
	listsP := listsByName.Load()
	if listsP == nil {
		logger().Warn("filter-update before configure", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: filterActionReject}
	}
	lists := *listsP
	list, ok := lists[in.Filter]
	if !ok {
		logger().Warn("unknown community-match", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: filterActionReject}
	}

	result := evaluateCommunities(list.entries, in.Update)

	if result == actionAccept {
		logger().Info("community-match accept", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: filterActionAccept}
	}

	logger().Info("community-match reject", "filter", in.Filter, "peer", in.Peer)
	return &sdk.FilterUpdateOutput{Action: filterActionReject}
}
