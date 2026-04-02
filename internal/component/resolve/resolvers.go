// Design: docs/architecture/resolve.md -- Resolution component container
//
// Package resolve provides the Resolvers container that holds DNS, Cymru,
// PeeringDB, and IRR resolver instances. Created once at hub startup and
// passed to consumers.
//
// Each resolver keeps its own typed API. Consumers call methods directly
// on the specific resolver they need (e.g., resolvers.Cymru.LookupASNName).
package resolve

import (
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/cymru"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/dns"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/irr"
	"codeberg.org/thomas-mangin/ze/internal/component/resolve/peeringdb"
)

// Resolvers holds all resolution service instances. Created once at hub startup
// with explicit configuration. Consumers receive the whole struct or individual
// resolvers as needed.
//
// Zero-value fields mean the resolver is not available (consumer should handle nil).
type Resolvers struct {
	DNS       *dns.Resolver
	Cymru     *cymru.CymruResolver
	PeeringDB *peeringdb.PeeringDB
	IRR       *irr.IRR
}

// Close releases resources held by all resolvers. Caller MUST call Close
// when the Resolvers struct is no longer needed.
func (r *Resolvers) Close() {
	if r.DNS != nil {
		r.DNS.Close()
	}
}
