// Design: docs/architecture/core-design.md -- route attribute modifier plugin
// Detail: modify.go -- delta building and attribute encoding
// Detail: config.go -- bgp/policy/modify config parsing
// RFC: rfc/short/rfc4271.md -- Sections 5.1.4 and 9.1.2.2 govern the med-remove directive
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
// A definition CAN state its own condition in a match container. A route that
// meets it gets action "modify"; a route that does not gets "accept" and passes
// through unchanged. A definition that states no condition matches every route,
// which is what every definition written before the match container did.
//
// The plugin declares ZERO filters at Stage 1: modifier names come from
// config (Stage 2).
package filter_modify

import (
	"errors"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// directionImport is the wire form the engine passes on an ingress chain
	// (rpc.FilterUpdateInput.Direction, "import" or "export").
	directionImport = "import"
	// medRemoveDirective is the delta token the engine converts into one
	// filterapi.AttrModSuppress on attribute 4 (ExtractMEDRemoveOps,
	// reactor/filter_delta.go). It carries no operand.
	medRemoveDirective = "med-remove"
)

// The attribute names of the modify vocabulary. Each one is a YANG leaf under
// the `set` block, and the three numeric ones are also the attribute names the
// `increment`, `decrement` and `del` blocks accept. Spell each name once here:
// a misspelled map index reads as an absent leaf, so the modification is
// silently dropped rather than refused.
const (
	localPreferenceAttr = "local-preference"
	medAttr             = "med"
	aigpAttr            = "aigp"
	originAttr          = "origin"
	nextHopAttr         = "next-hop"
	asPathPrependAttr   = "as-path-prepend"
)

// The community delta directives. Each token is both the YANG leaf under the
// `set` block and the directive the engine parses out of the delta string, so
// the two spellings cannot drift apart.
const (
	communityAddDirective            = "community-add"
	communityRemoveDirective         = "community-remove"
	largeCommunityAddDirective       = "large-community-add"
	largeCommunityRemoveDirective    = "large-community-remove"
	extendedCommunityAddDirective    = "extended-community-add"
	extendedCommunityRemoveDirective = "extended-community-remove"
)

var errFilterModifyInvalidBgpConfigJson = errors.New("filter-modify: invalid bgp config JSON")

var logger = slogutil.LazyLogger("bgp.filter.modify")

const configRootBGP = "bgp"

// defsByName is the runtime-loaded set of modify definitions.
// Updated atomically on every OnConfigure delivery.
var defsByName atomic.Pointer[map[string]*modifyDef]

// absentBase is the value the arithmetic starts from for an attribute the route
// does not carry, one entry per leaf bgp { defaults { attribute { } } }
// declares. Updated atomically on every OnConfigure delivery, so a reload
// changes what routes arriving after it compute from.
//
// nil until the first delivery, and absentBaseFor (modify.go) answers that
// window from the schema's own defaults rather than from a zero value.
var absentBase atomic.Pointer[map[string]uint32]

// runFilterModify runs the route modify plugin using the SDK RPC protocol.
func runFilterModify(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-modify", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != configRootBGP {
				continue
			}
			bgpCfg, ok := configjson.ParseBGPSubtree(section.Data)
			if !ok {
				return errFilterModifyInvalidBgpConfigJson
			}
			if err := applyBGPConfig(bgpCfg); err != nil {
				return err
			}
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
		logger().Error("filter-modify plugin failed", "error", err)
		return 1
	}
	return 0
}

// applyBGPConfig installs one delivery of the bgp config subtree: the named
// modifier definitions, and the base the arithmetic starts from for an
// attribute a route does not carry. Every delivery replaces both, so a reload
// changes what routes arriving after it compute from.
//
// The base is stored FIRST, and that ordering buys one property: an update that
// passes handleFilterUpdate's defsByName gate never finds a nil base. It does
// NOT pair the two stores. buildDynamicDelta loads the base once per attribute,
// so a reload landing mid-delta can give one delivery's modifier the next
// delivery's base. Both numbers are the operator's, a reload is not retroactive
// for a route already handled, and no caller needs more than the property above.
//
// A DELIVERY that does not parse installs NOTHING, and the previous
// configuration keeps running. Half a delivery would leave the modifiers of one
// config computing from the base of another for every route after it, which is
// the durable version of the race the paragraph above bounds to one delta.
func applyBGPConfig(bgpCfg map[string]any) error {
	base, err := parseAttributeDefaults(bgpCfg)
	if err != nil {
		return fmt.Errorf("filter-modify: %w", err)
	}
	defs, err := parseModifyDefs(bgpCfg)
	if err != nil {
		return fmt.Errorf("filter-modify: %w", err)
	}

	absentBase.Store(&base)
	defsByName.Store(&defs)

	logger().Debug("configured", "modifiers", len(defs),
		"absent-base-med", base[medAttr],
		"absent-base-local-preference", base[localPreferenceAttr])
	return nil
}

// handleFilterUpdate dispatches a single filter-update RPC.
// Static modifiers return the pre-built delta. Dynamic modifiers (inc/dec,
// community ops) read the current update text and compute the delta at runtime.
// Unknown modifier names fail closed with "reject".
func handleFilterUpdate(in *sdk.FilterUpdateInput) *sdk.FilterUpdateOutput {
	defsP := defsByName.Load()
	if defsP == nil {
		logger().Warn("filter-update before configure", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	defs := *defsP
	def, ok := defs[in.Filter]
	if !ok {
		logger().Warn("unknown modify", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	// A route that does not meet the definition's condition passes through
	// UNCHANGED. Accept, never reject: the chain drops a rejected route, and a
	// conditional modifier exists precisely so the routes it does not touch keep
	// flowing. A definition that states no condition matches every route, which
	// is what every definition written before the match container did.
	if !def.match.matches(in.Update) {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
	}

	var delta string
	if def.isDynamic() {
		delta = buildDynamicDelta(def, in.Update)
	} else {
		delta = def.delta
	}
	if def.medRemove {
		delta = appendMEDRemove(delta, in.Filter, in.Direction)
	}

	logger().Info("modify apply", "filter", in.Filter, "peer", in.Peer, "delta", delta)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Update: delta}
}

// appendMEDRemove adds RFC 4271 Section 5.1.4's removal directive to the delta,
// and only on an import chain.
//
// Section 5.1.4 requires the removal to happen "prior to determining the degree
// of preference of the route and prior to performing route selection (Decision
// Process phases 1 and 2)", which the import chain satisfies and the export
// chain cannot: on export the decision process has already run. Section 9.1.2.2
// then makes an export-side removal actively wrong, because comparing on a
// MULTI_EXIT_DISC and afterwards advertising the route without it "has been
// proven to cause route loops".
//
// The engine enforces this by converting the directive at the import site alone
// (ExtractMEDRemoveOps, reactor/filter_delta.go). This function is the half the
// operator can see: silence would leave a configured removal that never happens
// and never says why.
func appendMEDRemove(delta, filter, direction string) string {
	if direction != directionImport {
		logger().Warn("del { med; } is ignored on export",
			"filter", filter, "direction", direction,
			"rfc", "RFC 4271 Section 5.1.4 requires configured removal before decision process phases 1 and 2")
		return delta
	}
	if delta == "" {
		return medRemoveDirective
	}
	return joinValues([]string{delta, medRemoveDirective})
}
