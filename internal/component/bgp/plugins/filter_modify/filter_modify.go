// Design: docs/architecture/core-design.md -- route attribute modifier plugin
// Detail: modify.go -- delta building and attribute encoding
// Detail: config.go -- bgp/policy/modify config parsing
//
// Package filter_modify implements the bgp-filter-modify plugin.
//
// The plugin loads named modifier definitions from
// bgp { policy { modify NAME { set { local-preference 200; } } } }
// at OnConfigure (Stage 2). At runtime, peer filter chains reference a
// modifier as bgp-filter-modify:NAME or modify:NAME. The engine dispatches
// each call via CallFilterUpdate (filter-update RPC); the plugin returns
// action "modify" with a pre-built text delta. The engine merges the delta
// via applyFilterDelta (text overlay) and textDeltaToModOps ->
// buildModifiedPayload (wire-level rewriting).
//
// The modifier always returns "modify" -- it unconditionally sets declared
// attributes on every route that reaches it. For conditional modification,
// compose with match filters earlier in the chain.
//
// The plugin declares ZERO filters at Stage 1: modifier names come from
// config (Stage 2).
package filter_modify

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/configjson"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

const (
	filterActionModify = "modify"
	filterActionReject = "reject"
)

var logger = slogutil.LazyLogger("bgp.filter.modify")

// defsByName is the runtime-loaded set of modify definitions.
// Updated atomically on every OnConfigure delivery.
var defsByName atomic.Pointer[map[string]*modifyDef]

// RunFilterModify runs the route modify plugin using the SDK RPC protocol.
func RunFilterModify(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-modify", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			bgpCfg, ok := configjson.ParseBGPSubtree(section.Data)
			if !ok {
				return fmt.Errorf("filter-modify: invalid bgp config JSON")
			}
			defs, err := parseModifyDefs(bgpCfg)
			if err != nil {
				return fmt.Errorf("filter-modify: %w", err)
			}
			defsByName.Store(&defs)
			logger().Debug("configured", "modifiers", len(defs))
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
		logger().Error("filter-modify plugin failed", "error", err)
		return 1
	}
	return 0
}

// handleFilterUpdate dispatches a single filter-update RPC.
// Returns "modify" with the pre-built delta for known modifiers.
// Unknown modifier names fail closed with "reject".
func handleFilterUpdate(in *sdk.FilterUpdateInput) *sdk.FilterUpdateOutput {
	defsP := defsByName.Load()
	if defsP == nil {
		logger().Warn("filter-update before configure", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: filterActionReject}
	}
	defs := *defsP
	def, ok := defs[in.Filter]
	if !ok {
		logger().Warn("unknown modify", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: filterActionReject}
	}

	logger().Info("modify apply", "filter", in.Filter, "peer", in.Peer, "delta", def.delta)
	return &sdk.FilterUpdateOutput{Action: filterActionModify, Update: def.delta}
}
