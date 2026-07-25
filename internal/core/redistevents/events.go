// Design: docs/architecture/core-design.md -- cross-protocol route-change events
// Detail: pool.go -- batch pool allocation
// Detail: registry.go -- protocol ID registration
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

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/replay"
)

// EventType is the canonical event-type string under each protocol's
// namespace. Producers and consumers use it when calling
// events.Register[*RouteChangeBatch](<protocol>, redistevents.EventType).
const EventType = "route-change"

// ReplayNamespace and ReplayRequestEventType name the SHARED replay-request
// broadcast. Unlike RouteChange, which is per-producer (each producer emits
// under its own protocol namespace), the replay-request is a single event the
// redistribute orchestrator emits and every producer subscribes to. One fixed
// (namespace, eventType) pair lets all producers agree without the orchestrator
// having to emit per-producer.
const (
	ReplayNamespace        = "redistribute"
	ReplayRequestEventType = "replay-request"
)

// ReplayRequest is the payload of the (redistribute, replay-request) event.
// It aliases the shared replay.Request: the redistribute orchestrator allocates
// a per-peer correlation token on a BGP peer's down->up edge, and each producer
// echoes the token verbatim into the ReplayID of the RouteChangeBatch(es) it
// re-emits. The producer never learns the peer -- the orchestrator alone holds
// the ReplayID -> peer mapping, so the returning batch stays peer-agnostic.
//
// The identical replay.Request type is the request payload for the two
// broadcast hops (bgp-rib, system-rib), where it carries replay.Broadcast and
// the handler ignores the token; see internal/core/replay.
type ReplayRequest = replay.Request

// ReplayRequestEvent is the shared typed handle for (redistribute,
// replay-request). The orchestrator calls ReplayRequestEvent.Emit(bus, ...);
// producers call ReplayRequestEvent.Subscribe(bus, ...). Two Register calls
// with the same (namespace, eventType, T) are idempotent, so importers agree.
var ReplayRequestEvent = events.Register[*ReplayRequest](ReplayNamespace, ReplayRequestEventType)

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
	Table   uint32

	// OriginAS is the per-entry origin AS the redistributed route carries into a
	// consumer that models locally-originated routes with an AS_PATH (BGP emits
	// `origin igp origin-as <OriginAS>`). It exists because a batch has one
	// OriginASN but BGP best-paths each have their own origin AS, so the single
	// batch field cannot express them; the consumer prefers this per-entry value
	// when nonzero and falls back to the batch OriginASN otherwise. Zero -- the
	// default for every non-BGP producer -- preserves the batch-level behavior.
	// Value type; reset by the pool's clear(); no allocation.
	OriginAS uint32
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

	// ReplayID correlates a re-emitted batch to the orchestrator's replay
	// request. 0 means a normal incremental change (the default for every
	// existing producer emit -- additive, no wire/behavior change). A nonzero
	// value is the opaque token from a ReplayRequest the producer is answering;
	// the orchestrator maps it back to the single peer whose establishment
	// triggered the replay and targets only that peer. It is a value type (an
	// opaque token, NOT a peer) so the batch stays peer-agnostic.
	ReplayID uint64

	// OriginASN, when nonzero, is the origin AS the redistributed route carries
	// into BGP as a single-ASN AS_PATH: the consumer emits
	// `origin igp origin-as <OriginASN>` (a locally-originated route) instead of
	// the default `origin incomplete` with no AS_PATH. Generic capability: any
	// source may set it to model itself as a virtual router with its own ASN
	// (the first user is as112). Zero -- the default for every existing producer
	// -- preserves the legacy `origin incomplete`, no-AS_PATH wire output. Value
	// type; no allocation.
	OriginASN uint32

	// Community, when non-nil, is the list of standard BGP communities (each
	// packed as asn<<16|value per RFC 1997) the redistributed route carries: the
	// consumer emits `community [ <a>:<b> ... ]`. Generic: any source may set it.
	// Nil -- the default for every existing producer -- omits the COMMUNITIES
	// attribute entirely and adds no allocation. Like Entries, the slice is
	// read-only during synchronous dispatch and MUST NOT be retained past it;
	// ReleaseBatch drops the reference but never clears the backing array, which
	// the producer (typically config) owns.
	Community []uint32
}

// IsReplay reports whether this batch answers a replay request (nonzero
// ReplayID) rather than carrying a normal incremental change. Derived from the
// shared replay.IsReplay predicate over ReplayID, so there is no second source
// of truth (a separate Replay bool) to keep consistent.
func (b *RouteChangeBatch) IsReplay() bool { return replay.IsReplay(b.ReplayID) }
