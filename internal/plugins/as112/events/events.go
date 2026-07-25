// Design: docs/architecture/core-design.md -- redistribute producer registration

// Package as112events holds the as112 plugin's redistribute producer identity:
// its numeric ProtocolID and the LOCAL typed EventBus handle for
// (as112, route-change). Kept in a subpackage (connected/events, static/events,
// l2tp/events precedent) so both the producer and any importer can obtain the
// same ProtocolID without pulling in the full as112 plugin.
package as112events

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// Namespace is the redistribute source/protocol name for as112. It MUST match
// the RouteSource Name/Protocol registered in the as112 plugin so the
// orchestrator resolves the ProtocolID by the same string.
const Namespace = "as112"

// CoveringPrefixesV4 / CoveringPrefixesV6 are the four fixed AS112 COVERING
// prefixes (RFC 7534 Section 3.4 Direct Delegation, RFC 7535 Section 3.1 DNAME
// Redirection) -- not operator-configurable. Defined here, in the shared leaf
// package, as the single source for the producer (internal/plugins/as112) and
// the fakeas112 test producer, so the announced set cannot drift between them.
// (The doctor keeps its own mirror by design -- it is the neutral coordination
// home and must not import this plugin; see checks_as112_coordination.go.)
// Read-only: callers iterate but must NOT mutate these slices.
var (
	CoveringPrefixesV4 = []netip.Prefix{
		netip.MustParsePrefix("192.175.48.0/24"), // Direct Delegation
		netip.MustParsePrefix("192.31.196.0/24"), // DNAME Redirection
	}
	CoveringPrefixesV6 = []netip.Prefix{
		netip.MustParsePrefix("2620:4f:8000::/48"), // Direct Delegation
		netip.MustParsePrefix("2001:4:112::/48"),   // DNAME Redirection
	}
)

// ProtocolID is allocated once and shared; the producer fills it into
// RouteChangeBatch.Protocol. RegisterProtocol is idempotent by name.
var ProtocolID = redistevents.RegisterProtocol(Namespace)

// registerProducer marks as112 as having a producer so the redistribute
// orchestrator discovers it via redistevents.Producers(). RegisterProducer
// panics on an unknown ProtocolID, so ProtocolID above must initialize first
// (package-var order guarantees it).
var _ = registerProducer()

func registerProducer() bool {
	redistevents.RegisterProducer(ProtocolID)
	return true
}

// RouteChange is the LOCAL typed handle for (as112, route-change). The events
// registry is idempotent on (namespace, eventType, T), so the orchestrator's
// own Register call with the same tuple agrees.
var RouteChange = events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)
