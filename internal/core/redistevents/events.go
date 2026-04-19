// Design: docs/architecture/core-design.md -- cross-protocol route-change events
//
// Package redistevents owns the shared, value-typed payload that protocol
// route producers (L2TP, connected, future static/OSPF/ISIS) publish on the
// EventBus and the bgp-redistribute consumer subscribes to.
//
// The package is intentionally minimal: type definitions, a uint16 ProtocolID
// registry, a "this protocol has a producer" presence bit, and a pooled
// allocator for the batch payload. There is no shared state across plugin
// boundaries -- producers and consumers each call events.Register[
// *RouteChangeBatch] in their OWN package to obtain a LOCAL typed handle bound
// to (<protocol>, "route-change"). The events registry is idempotent on
// (namespace, eventType, T), so independent Register calls from different
// packages with the same tuple agree.
//
// Hot-path constraints (see plan/spec-bgp-redistribute.md, "Pool semantics"):
//   - Payload fields are value types only -- no string fields, no pointers
//     into another plugin or component's memory. Strings would force a
//     per-event heap allocation; cross-boundary pointers would let the
//     consumer reach into producer-owned data, which is rejected.
//   - The batch and its Entries slice come from a sync.Pool seeded for peak
//     concurrent producers. Producer lifecycle: AcquireBatch -> fill -> Emit
//     -> ReleaseBatch.
//   - Per the EventBus contract, subscribers MUST treat the payload as
//     read-only and MUST NOT retain it past the dispatch call. ReleaseBatch
//     therefore runs unconditionally on the producer side after Emit returns.
package redistevents

import "net/netip"

// EventType is the canonical event-type string under each protocol's
// namespace. Producers and consumers use it when calling
// events.Register[*RouteChangeBatch](<protocol>, redistevents.EventType).
const EventType = "route-change"

// ProtocolID is the typed numeric identity of a route-producing protocol.
// Allocated at producer init via RegisterProtocol.
//
// uint16 because per rules/enum-over-string.md "Performance" row, every per-
// event payload field that crosses a component boundary must be a typed
// numeric identity. The zero value is ProtocolUnspecified, which is invalid
// and surfaces uninitialised payloads on the consumer side.
type ProtocolID uint16

// ProtocolUnspecified is the zero ProtocolID. A batch carrying this protocol
// is invalid (corruption or a producer bug); the consumer drops it with a
// warn.
const ProtocolUnspecified ProtocolID = 0

// RouteAction is the typed enum for an entry's lifecycle change.
//
// uint8 to keep the entry struct compact. Zero value is ActionUnspecified
// (invalid) so an uninitialised entry surfaces immediately rather than being
// silently treated as Add or Remove.
type RouteAction uint8

// Route action enumerants. Keep ActionUnspecified at zero so uninitialised
// entries are caught.
const (
	ActionUnspecified RouteAction = 0
	ActionAdd         RouteAction = 1
	ActionRemove      RouteAction = 2
)

// String returns a human-readable form for diagnostics. Never used for
// equality on the hot path.
func (a RouteAction) String() string {
	if a == ActionAdd {
		return "add"
	}
	if a == ActionRemove {
		return "remove"
	}
	return "unspecified"
}

// RouteChangeEntry is one route lifecycle event in a batch. Every field is a
// value type with a fixed in-memory size, so the entries slice's backing
// array stays stable in the pool and the payload carries no pointer into
// producer-owned memory.
//
// NextHop semantics: the zero netip.Addr means "no explicit next-hop, the
// consumer should emit `nhop self` and let the reactor substitute each
// peer's local session address". A non-zero Addr is passed through verbatim
// as `nhop <addr>`.
type RouteChangeEntry struct {
	Action  RouteAction
	Prefix  netip.Prefix
	NextHop netip.Addr
	Metric  uint32
}

// RouteChangeBatch is the payload of (<protocol>, "route-change"). One batch
// describes one (protocol, family) tuple of entries; producers may emit any
// number of entries per batch, including zero (a no-op for diagnostic /
// liveness purposes; the consumer skips it).
//
// Pointer-typed because the bus delivers `any`, and the typed handle wraps
// `T = *RouteChangeBatch`. The pointer comes from the bus, not from another
// plugin's memory.
type RouteChangeBatch struct {
	// Protocol identifies the producing protocol (numeric identity, registered
	// at producer init). Consumers compare on this uint16 to filter out their
	// own protocol or to dispatch by source.
	Protocol ProtocolID

	// AFI / SAFI together form the canonical ze address family. Stored as
	// raw integers (not family.Family) so the redistevents package stays a
	// true leaf with zero internal coupling -- producers and consumers
	// translate via family.Family{AFI: ..., SAFI: ...} at the boundary.
	AFI  uint16
	SAFI uint8

	// Entries is the slice of per-prefix changes. Pool-friendly: backing
	// array is recycled via sync.Pool. Producers cap each acquire at the
	// pool's seeded size; growth on the hot path is a sizing bug surfaced
	// by the burst test (AC-13).
	Entries []RouteChangeEntry
}
