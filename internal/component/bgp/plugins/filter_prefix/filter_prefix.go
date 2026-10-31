// Design: docs/architecture/core-design.md -- prefix-list filter plugin
// Detail: match.go -- prefix matching algorithm and update evaluator
// Detail: config.go -- policy/prefix-list config parsing
// Detail: config_tx.go -- the prefix lists at boot and on reload
//
// Package filter_prefix implements the bgp-filter-prefix plugin.
//
// The plugin loads named prefix-list definitions from
// policy { prefix-list NAME { entry P { ge G; le L; action A; } } }
// at Stage 2, and installs a reload's lists at config-apply. At runtime, peer filter chains reference a list
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
	"errors"
	"net"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/filtertext"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errFilterPrefixInvalidBgpConfigJson = errors.New("filter-prefix: invalid bgp config JSON")

var logger = slogutil.LazyLogger("bgp.filter.prefix")

const configRootBGP = "bgp"

// listsByName is the runtime-loaded set of prefix-list definitions, keyed by
// the YANG list name. Stored whole by every install (boot, apply, rollback,
// config_tx.go) so the hot path reads without a lock and never a mix.
var listsByName atomic.Pointer[map[string]*prefixList]

// RunFilterPrefix runs the prefix-list filter plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func RunFilterPrefix(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-prefix", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	newPrefixConfigTx().Register(p)

	p.OnFilterUpdate(func(in *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
		return handleFilterUpdate(in), nil
	})

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
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	lists := *listsP
	list, ok := lists[in.Filter]
	if !ok {
		logger().Warn("unknown prefix-list", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	nlriField := extractNLRIField(in.Update)
	partition := list.partitionUpdate(nlriField)

	// No prefixes (empty nlri or attrs-only update): accept. Preserves the
	// semantic that routes with no matchable reachability pass through.
	if len(partition.accepted) == 0 && len(partition.rejected) == 0 && !partition.hadParseError {
		logger().Info("prefix-list accept", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
	}

	// Malformed prefix in the text protocol -> fail-closed. Same contract
	// as evaluateUpdate had before the partition refactor.
	if partition.hadParseError {
		logger().Info("prefix-list reject", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField, "reason", "parse-error")
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	// All denied: reject the whole update.
	if len(partition.accepted) == 0 {
		logger().Info("prefix-list reject", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	// All accepted: accept without modification.
	if len(partition.rejected) == 0 {
		logger().Info("prefix-list accept", "filter", in.Filter, "peer", in.Peer, "nlri", nlriField)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
	}

	// The engine rewrites only the block of a CIDR family (filtertext.CIDRFamily)
	// and refuses a changed block of any other family (reactor
	// filter_nlri_delta.go). FlowSpec, for one, exposes its destination
	// component, not a replacement NLRI. A rejected prefix in such a block, or
	// in a block whose family this process does not know, rejects the complete
	// UPDATE: fail closed.
	for i := range partition.blocks {
		if !rewritableBlock(&partition.blocks[i]) {
			return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
		}
	}

	// Mixed: modify. Emit a delta whose nlri blocks contain only the accepted
	// subset of each block. The engine replaces the nlri value verbatim
	// (applyFilterDelta) and rewrites each section from its own block: the
	// legacy NLRI and Withdrawn Routes, and each MP_REACH_NLRI and
	// MP_UNREACH_NLRI (reactor filter_nlri_delta.go).
	delta := buildModifyDelta(partition)
	logger().Info(
		"prefix-list modify",
		"filter", in.Filter, "peer", in.Peer,
		"accepted", len(partition.accepted), "rejected", len(partition.rejected),
		"nlri", nlriField,
	)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Update: delta}
}

// rewritableBlock reports whether a modify delta can drop the rejected prefixes
// of blk. A block with no rejected prefix is echoed and needs no rewrite.
func rewritableBlock(blk *prefixBlock) bool {
	if len(blk.rejected) == 0 {
		return true
	}
	fam, known := family.LookupFamily(blk.family)
	if !known {
		return false
	}
	return filtertext.CIDRFamily(fam)
}

// buildModifyDelta renders the modify delta returned to the engine when some
// prefixes pass and some don't. The delta is intentionally minimal -- it
// only carries the nlri blocks, so applyFilterDelta in the engine leaves every
// other attribute untouched. A block whose every prefix was refused is left
// out, which the engine reads as "keep none" for that section; a block that
// carried no prefix (a non-CIDR family marker) is echoed as it was.
func buildModifyDelta(partition partitionResult) string {
	parts := make([]string, 0, 4*len(partition.blocks)+len(partition.accepted))
	for i := range partition.blocks {
		blk := &partition.blocks[i]
		if len(blk.accepted) == 0 && len(blk.rejected) > 0 {
			continue
		}
		parts = append(parts, blk.head...)
		parts = append(parts, blk.accepted...)
	}
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
