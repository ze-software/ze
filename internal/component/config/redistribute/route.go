// Design: docs/architecture/core-design.md -- redistribution route types and loop prevention
// Related: registry.go -- source registry used for protocol lookup

package redistribute

import (
	"slices"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
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
// Corresponds to one entry in the YANG list import { key "source"; }.
type ImportRule struct {
	Source   string          // Source name from config ("ebgp", "ospf", "connected", ...)
	Families []family.Family // Allowed families (empty = all families accepted)
}

// Accept checks whether a route should be accepted by this import rule.
// A route is rejected if:
//   - its origin protocol matches the importing protocol (loop prevention)
//   - its family is not in the allowed list (when families is non-empty)
//   - its source does not match the rule's source
func (r ImportRule) Accept(route RedistRoute, importingProtocol string) bool {
	if route.Origin == importingProtocol {
		return false
	}
	if route.Source != r.Source {
		return false
	}
	return len(r.Families) == 0 || slices.Contains(r.Families, route.Family)
}

// Evaluate checks a route against a set of import rules for a given
// importing protocol. Returns true if any rule accepts the route.
func Evaluate(route RedistRoute, rules []ImportRule, importingProtocol string) bool {
	for i := range rules {
		if rules[i].Accept(route, importingProtocol) {
			return true
		}
	}
	return false
}
