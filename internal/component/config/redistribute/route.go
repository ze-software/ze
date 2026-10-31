// Design: docs/architecture/core-design.md -- redistribution route types and loop prevention
// Related: registry.go -- source registry used for protocol lookup

package redistribute

import (
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// RedistRoute represents a route flowing through the redistribution engine.
// The Origin field is set once when a route first enters redistribution and
// is never modified. This is the key to loop prevention: a route is never
// redistributed back into its origin protocol.
type RedistRoute struct {
	Origin string // Protocol that originated this route ("bgp", "ospf", "connected", ...)
	Source string // Specific source name ("ibgp", "ebgp", "ospf", ...)
	// Route is the view an import rule's chain judges: family, prefix, next
	// hop, metric, the route's own tag and its BGP class.
	Route ribpolicy.Route
}

// ImportRule represents a parsed redistribution import entry from config.
// Corresponds to one entry in the YANG list import { key "source"; } under a
// `destination <proto>` block.
type ImportRule struct {
	Source string // Source name from config ("ebgp", "ospf", "connected", ...)
	// Destination is the protocol this import feeds ("bgp", "ospf", "isis"),
	// taken from the enclosing `destination <proto>` block. An empty
	// Destination is destination-agnostic: it accepts any importing protocol.
	// The config loader always populates it from the `destination` key.
	Destination string
	// Filters names the policy filter instances of the rule's chain, in order,
	// as `filters [ ... ]` holds them.
	Filters []string
	// Final is the chain's decision after its last filter, "accept" or
	// "reject" as `final` holds it. Empty means the rule has no chain: every
	// route of the source moves.
	Final string
	// Chain is the compiled chain of Filters and Final, against the attachment
	// "redistribute import". The config loader cannot compile it (the policy
	// library imports this package), so the redistribute orchestrator does
	// before it installs the rule. A rule with a Final and no Chain is a Ze
	// defect, and Accept panics on it.
	Chain ribpolicy.Chain
}

// mustNameImportingProtocol refuses an empty importing protocol.
//
// An empty name is not a verdict about anything. It misses every rule the
// config loader builds, because the loader writes the enclosing `destination`
// key into Destination on every one of them, and it matches every rule whose
// Destination is empty. So a caller with no importing protocol in hand reads
// back `false` (or `true`) for a question it never asked, and neither answer
// can be told from a rule that legitimately did not match
// (`ai/rules/principles.md`).
//
// Ze shipped that caller. The `bgp-redistribute` ingress filter
// (`internal/component/bgp/plugins/redistribute_ingress`, retired 2026-09-04)  // <!-- doc-links: ignore (the retirement this paragraph records: `1ec5b741f8` deleted the plugin on 2026-09-04) -->
// asked this package whether to KEEP a route a peer had just announced. That
// question holds no importing protocol, so the filter passed "", every rule
// rejected, and one `redistribute` block anywhere in the config discarded every
// route from every peer, with no log line. Its unit tests were green because
// they built ImportRule values with an empty Destination, which no config
// produces.
//
// The name reaches this package from the consumer registry or from a replay
// target, never from the wire, so an empty one is a Ze defect and a peer cannot
// reach this panic. `redistevents.RegisterProtocol` refuses an empty protocol
// name for the same reason.
func mustNameImportingProtocol(importingProtocol string) {
	if importingProtocol != "" {
		return
	}
	panic("BUG: redistribute: the importing protocol has no name, and an empty name is not a verdict")
}

// Accept checks whether a route should be accepted by this import rule, and
// answers the sets the rule's chain applied (a tag) when it is.
// A route is rejected if:
//   - its origin protocol matches the importing protocol (loop prevention)
//   - the rule has a Destination that does not match the importing protocol
//     (destination scoping: an import under `destination bgp` feeds only BGP)
//   - neither its specific source nor umbrella origin matches the rule's source
//   - the rule's chain does not accept it
//
// importingProtocol MUST name a protocol. An empty name panics.
func (r *ImportRule) Accept(route *RedistRoute, importingProtocol string) (ribpolicy.Sets, bool) {
	mustNameImportingProtocol(importingProtocol)

	// Loop prevention: one shared definition of the invariant (redistevents.WouldLoop),
	// also enforced at the two runtime guards in the redistribute orchestrator.
	if redistevents.WouldLoop(route.Origin, importingProtocol) {
		return ribpolicy.Sets{}, false
	}
	if !r.covers(route, importingProtocol) {
		return ribpolicy.Sets{}, false
	}
	// refuse-by-default: guards ImportRule.Final -- a rule with no final has no chain, so every route of its source moves
	if r.Final == "" {
		return ribpolicy.Sets{}, true
	}
	if r.Chain == nil {
		panic("BUG: redistribute: import " + r.Source + " has a final and no compiled chain")
	}
	verdict := r.Chain.Evaluate(route.Route)
	return verdict.Sets, verdict.Action == ribpolicy.ActionAccept
}

// covers answers whether the rule imports route's source into
// importingProtocol, before its chain has a say.
func (r *ImportRule) covers(route *RedistRoute, importingProtocol string) bool {
	// Destination scoping (R-3): a rule parsed under `destination <proto>` is
	// accepted only by that protocol.
	if r.Destination != "" && r.Destination != importingProtocol {
		return false
	}
	return route.Source == r.Source || route.Origin == r.Source
}

// evaluate checks a route against a set of import rules for a given importing
// protocol. Returns true if any rule accepts the route, with the sets of the
// first rule that does. Package-internal: the
// exported surface is Evaluator.Accept, which delegates here.
func evaluate(route *RedistRoute, rules []ImportRule, importingProtocol string) (ribpolicy.Sets, bool) {
	for i := range rules {
		if sets, ok := rules[i].Accept(route, importingProtocol); ok {
			return sets, true
		}
	}
	return ribpolicy.Sets{}, false
}
