// Design: docs/architecture/core-design.md -- redistribution route types and loop prevention
// Related: registry.go -- source registry used for protocol lookup

package redistribute

import (
	"slices"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// RedistRoute represents a route flowing through the redistribution engine.
// The Origin field is set once when a route first enters redistribution and
// is never modified. This is the key to loop prevention: a route is never
// redistributed back into its origin protocol.
type RedistRoute struct {
	Origin string        // Protocol that originated this route ("bgp", "ospf", "connected", ...)
	Family family.Family // Address family (ipv4/unicast, ipv6/unicast, ...)
	Source string        // Specific source name ("ibgp", "ebgp", "ospf", ...)
}

// ImportRule represents a parsed redistribution import entry from config.
// Corresponds to one entry in the YANG list import { key "source"; } under a
// `destination <proto>` block.
type ImportRule struct {
	Source string // Source name from config ("ebgp", "ospf", "connected", ...)
	// Destination is the protocol this import feeds ("bgp", "ospf", "isis"),
	// taken from the enclosing `destination <proto>` block. An empty
	// Destination is destination-agnostic: it accepts any importing protocol.
	// Empty is the back-compat default for rules built without the field (unit
	// tests, callers that pre-date destination scoping); the config loader
	// always populates it from the `destination` key so production rules are
	// scoped.
	Destination string
	Families    []family.Family // Allowed families (empty = all families accepted)
}

// Accept checks whether a route should be accepted by this import rule.
// A route is rejected if:
//   - its origin protocol matches the importing protocol (loop prevention)
//   - the rule has a Destination that does not match the importing protocol
//     (destination scoping: an import under `destination bgp` feeds only BGP)
//   - neither its specific source nor umbrella origin matches the rule's source
//   - its family is not in the allowed list (when families is non-empty)
func (r ImportRule) Accept(route RedistRoute, importingProtocol string) bool {
	// Loop prevention: one shared definition of the invariant (redistevents.WouldLoop),
	// also enforced at the two runtime guards in redistribute_egress.
	if redistevents.WouldLoop(route.Origin, importingProtocol) {
		return false
	}
	// Destination scoping (R-3): a rule parsed under `destination <proto>` is
	// accepted only by that protocol. An empty Destination stays agnostic so
	// rules built without the field behave as before this change.
	if r.Destination != "" && r.Destination != importingProtocol {
		return false
	}
	if route.Source != r.Source && route.Origin != r.Source {
		return false
	}
	return len(r.Families) == 0 || slices.Contains(r.Families, route.Family)
}

// evaluate checks a route against a set of import rules for a given importing
// protocol. Returns true if any rule accepts the route. Package-internal: the
// exported surface is Evaluator.Accept, which delegates here.
func evaluate(route RedistRoute, rules []ImportRule, importingProtocol string) bool {
	for i := range rules {
		if rules[i].Accept(route, importingProtocol) {
			return true
		}
	}
	return false
}
