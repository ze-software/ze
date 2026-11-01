// Design: docs/architecture/core-design.md -- route attribute modifier plugin
// Detail: modify.go -- the sets a definition answers, built at config time
// Detail: config.go -- policy/modify config parsing
// RFC: rfc/short/rfc4271.md -- Sections 5.1.4 and 9.1.2.2 govern the med-remove directive
//
// Package filter_modify implements the bgp-filter-modify plugin.
//
// The plugin loads named modifier definitions from
// policy { modify NAME { set { local-preference 200; } } }
// at Stage 2 and on every reload (config_tx.go). At runtime, peer filter chains reference a
// modifier as bgp-filter-modify:NAME or modify:NAME. One evaluator answers the
// route on both transports (sdk.OnFilter): the typed bridge slot when the
// plugin runs internal, the filter-update batch when it runs forked. It answers
// action "modify" with the definition's sets, built at config time, and the
// engine applies them (policychain Sets): an increment or a decrement resolves
// there against the route's value, or, for a route that carries none, against
// the chain's base from bgp { defaults { attribute { } } }.
//
// A definition CAN state its own condition in a match container. A route that
// meets it gets action "modify"; a route that does not gets "accept" and passes
// through unchanged. A definition that states no condition matches every route,
// which is what every definition written before the match container did.
//
// The plugin declares one filter pattern at Stage 1: modifier names come from
// config (Stage 2), and a name the config does not define fails the route.
package filter_modify

import (
	"errors"
	"net"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// directionImport is the wire form the engine passes on an ingress chain
	// (rpc.FilterUpdateInput.Direction, "import" or "export").
	directionImport = "import"
	// medRemoveText names the MULTI_EXIT_DISC removal in the delta text the
	// "modify apply" log line carries. The removal itself is the med remove
	// set (modifyDef.importSets).
	medRemoveText = "med-remove"
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

// The community leaves under the `set` block. Each one names the field and
// the action of the sets it builds (communitySetLeaves), and the delta text
// of the log line writes the leaf name.
const (
	communityAddDirective            = "community-add"
	communityRemoveDirective         = "community-remove"
	largeCommunityAddDirective       = "large-community-add"
	largeCommunityRemoveDirective    = "large-community-remove"
	extendedCommunityAddDirective    = "extended-community-add"
	extendedCommunityRemoveDirective = "extended-community-remove"
)

var (
	errFilterModifyInvalidBgpConfigJson = errors.New("filter-modify: invalid bgp config JSON")
	errFilterBeforeConfigure            = errors.New("filter-modify: a route arrived before the configuration")
	errUnknownModify                    = errors.New("filter-modify: no modify of this name is configured")
)

var logger = slogutil.LazyLogger("bgp.filter.modify")

const configRootBGP = "bgp"

// defsByName is the installed set of modify definitions, stored whole by
// installModifyDefs (Stage 2 and each reload's apply and rollback), so one
// load reads the modifiers of one config.
var defsByName atomic.Pointer[map[string]*modifyDef]

// runFilterModify runs the route modify plugin using the SDK RPC protocol.
func runFilterModify(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-modify", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	newModifyConfigTx().Register(p)

	// One evaluator for both transports (D-12): the typed bridge slot when
	// the plugin runs internal, the filter-update batch when it runs forked.
	p.OnFilter(evaluateModify)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{
		// The policy root is listed although the registry's ConfigReads names
		// it: a forked process has no registry row (registryRow refuses an
		// external program), so Stage 1 is the only declaration it carries,
		// and without it the definitions under policy never arrive.
		WantsConfig: []string{configRootBGP, policy.RootName},
		Filters: []sdk.FilterDecl{{
			NamePattern: ".+",
			Direction:   sdk.FilterBoth,
			// The match condition reads the three community fields, and a
			// declaration with no read would be served as a text member. The
			// sets need no read: the engine resolves an increment against the
			// route's own value.
			Reads: []string{policy.FieldCommunity.String(), policy.FieldLargeCommunity.String(),
				policy.FieldExtendedCommunity.String()},
			Sets: []string{policy.FieldLocalPreference.String(), policy.FieldMED.String(),
				policy.FieldOrigin.String(), policy.FieldNextHop.String(), policy.FieldASPath.String(),
				policy.FieldCommunity.String(), policy.FieldLargeCommunity.String(),
				policy.FieldExtendedCommunity.String(), policy.FieldAIGP.String()},
			OnError: sdk.OnErrorReject,
		}},
	}); err != nil {
		logger().Error("filter-modify plugin failed", "error", err)
		return 1
	}
	return 0
}

// evaluateModify answers one route: the definition's sets for a route that
// meets its condition, accept for one that does not. A name the config does
// not define, or a route before the first configuration, is an error, so the
// route meets the declaration's on-error rather than passing unchanged. It
// reads only the configured definitions, swapped whole by the config
// transaction (defsByName), so it is safe for concurrent use and never blocks.
func evaluateModify(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	defsP := defsByName.Load()
	if defsP == nil {
		return nil, errFilterBeforeConfigure
	}
	def, ok := (*defsP)[route.Filter()]
	if !ok {
		return nil, errUnknownModify
	}

	// A route that does not meet the definition's condition passes through
	// UNCHANGED. Accept, never reject: the chain drops a rejected route, and a
	// conditional modifier exists precisely so the routes it does not touch keep
	// flowing.
	if !def.match.empty() {
		fields, err := route.Fields()
		if err != nil {
			return nil, err
		}
		if !def.match.matches(fields) {
			return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
		}
	}

	sets, delta := def.sets, def.delta
	if def.medRemove {
		sets, delta = medRemoveFor(def, route)
	}
	if len(sets) == 0 {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
	}

	logger().Info("modify apply", "filter", route.Filter(), "peer", route.Peer(), "delta", delta)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: sets}, nil
}

// medRemoveFor answers the sets and the delta text of a definition that
// removes MULTI_EXIT_DISC (RFC 4271 Section 5.1.4), and the removal rides
// only on an import chain.
//
// Section 5.1.4 requires the removal to happen "prior to determining the degree
// of preference of the route and prior to performing route selection (Decision
// Process phases 1 and 2)", which the import chain satisfies and the export
// chain cannot: on export the decision process has already run. Section 9.1.2.2
// then makes an export-side removal actively wrong, because comparing on a
// MULTI_EXIT_DISC and afterwards advertising the route without it "has been
// proven to cause route loops".
//
// The engine fails an export route that answers the removal
// (policychain errMEDRemoveOnExport), so the export chain gets the other sets
// and a warning: silence would leave a configured removal that never happens
// and never says why.
func medRemoveFor(def *modifyDef, route *sdk.FilterRoute) ([]rpc.FilterSet, string) {
	if route.Direction() == directionImport {
		return def.importSets, def.importDelta
	}
	logger().Warn("del { med; } is ignored on export",
		"filter", route.Filter(), "direction", route.Direction(),
		"rfc", "RFC 4271 Section 5.1.4 requires configured removal before decision process phases 1 and 2")
	return def.sets, def.delta
}
