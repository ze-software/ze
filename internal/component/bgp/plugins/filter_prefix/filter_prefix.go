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
	filterActionModify = "modify"
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
// update text, and runs the per-prefix partition evaluator. Outcomes:
//
//   - no prefixes: accept (nothing to evaluate -- preserves legacy behavior)
//   - all prefixes accepted: accept
//   - all prefixes rejected: reject
//   - some accepted, some rejected (mixed): modify with a delta whose nlri
//     block contains only the accepted subset. The engine rewrites the
//     IPv4-unicast legacy NLRI section to match.
//
// Malformed prefixes in the text protocol trip hadParseError and fall back
// to reject (fail-closed, same as the original strict evaluator). Unknown
// filter names also fail closed.
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
	partition := list.partitionUpdate(nlriField)

	// No prefixes (empty nlri or attrs-only update): accept. Preserves the
	// semantic that routes with no matchable reachability pass through.
	if len(partition.accepted) == 0 && len(partition.rejected) == 0 && !partition.hadParseError {
		logger().Info("prefix-list accept", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField)
		return &sdk.FilterUpdateOutput{Action: filterActionAccept}
	}

	// Malformed prefix in the text protocol -> fail-closed. Same contract
	// as evaluateUpdate had before the partition refactor.
	if partition.hadParseError {
		logger().Info("prefix-list reject", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField, "reason", "parse-error")
		return &sdk.FilterUpdateOutput{Action: filterActionReject}
	}

	// All denied: reject the whole update.
	if len(partition.accepted) == 0 {
		logger().Info("prefix-list reject", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField)
		return &sdk.FilterUpdateOutput{Action: filterActionReject}
	}

	// All accepted: accept without modification.
	if len(partition.rejected) == 0 {
		logger().Info("prefix-list accept", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField)
		return &sdk.FilterUpdateOutput{Action: filterActionAccept}
	}

	// Mixed: modify. Emit a delta whose nlri block contains only the
	// accepted subset. The engine picks this up in applyFilterDelta (which
	// replaces the nlri key verbatim) and in extractNLRIOverride (which
	// reencodes the accepted prefixes into wire bytes for the IPv4 legacy
	// NLRI section).
	delta := buildModifyDelta(partition)
	logger().Info(
		"prefix-list modify",
		"filter", in.Filter, "peer", in.Peer,
		"accepted", len(partition.accepted), "rejected", len(partition.rejected),
		"nlri", nlriField,
	)
	return &sdk.FilterUpdateOutput{Action: filterActionModify, Update: delta}
}

// buildModifyDelta renders the modify delta returned to the engine when some
// prefixes pass and some don't. The delta is intentionally minimal -- it
// only carries the new nlri block so applyFilterDelta in the engine leaves
// every other attribute untouched.
func buildModifyDelta(partition partitionResult) string {
	parts := make([]string, 0, 3+len(partition.accepted))
	parts = append(parts, "nlri", partition.family, partition.op)
	parts = append(parts, partition.accepted...)
	return joinWords(parts)
}

// joinWords is a local strings.Join with a single space separator. Kept as a
// helper so the modify-delta emission can grow (e.g., quoting) without
// touching every call site.
func joinWords(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	n := len(parts) - 1
	for _, p := range parts {
		n += len(p)
	}
	b := make([]byte, 0, n)
	b = append(b, parts[0]...)
	for _, p := range parts[1:] {
		b = append(b, ' ')
		b = append(b, p...)
	}
	return string(b)
}
