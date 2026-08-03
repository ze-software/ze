// Design: plan/learned/1029-ospf-ext-1-opaque-framework.md -- RFC 5250 opaque consumer registry.
// RFC: rfc/short/rfc5250.md -- §9 Opaque Type registry; §3 origination/reception hooks.
//
// The opaque framework is a registration API: a consumer module (the future ext-2 TE,
// ext-3 Router-Information, ext-4 Extended-Link/Prefix, ext-9 Grace-LSA specs) claims an
// Opaque Type from its own init() and supplies origination and reception hooks. The
// carrier here names NO consumer and interprets NO opaque body (ai/rules/plugins.md);
// removing a consumer removes its registerOpaqueConsumer call and all its behavior,
// leaving the carrier intact. The registry is process-global (populated at init) and the
// engine discovers it at startup, exactly as the guide describes FRR's opaque framework.

package ospf

import (
	"errors"
	"sync"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// OpaqueScope is the RFC 5250 §3 flooding scope of an opaque LSA. Its underlying value is
// the LS type that carries the scope: 9 link-local, 10 area-local, 11 AS-wide.
type OpaqueScope uint8

const (
	// OpaqueScopeLink is the RFC 5250 Type-9 link-local flooding scope.
	OpaqueScopeLink OpaqueScope = OpaqueScope(types.LSTypeOpaqueLink)
	// OpaqueScopeArea is the RFC 5250 Type-10 area-local flooding scope.
	OpaqueScopeArea OpaqueScope = OpaqueScope(types.LSTypeOpaqueArea)
	// OpaqueScopeAS is the RFC 5250 Type-11 AS-wide flooding scope.
	OpaqueScopeAS OpaqueScope = OpaqueScope(types.LSTypeOpaqueAS)
)

// lsType returns the opaque LS type this scope selects.
func (s OpaqueScope) lsType() types.LSType { return types.LSType(s) }

// valid reports whether s is one of the three RFC 5250 opaque scopes.
func (s OpaqueScope) valid() bool { return s.lsType().IsOpaque() }

// String returns a stable lowercase scope name.
func (s OpaqueScope) String() string {
	switch s {
	case OpaqueScopeLink:
		return extRegistryLink
	case OpaqueScopeArea:
		return scopeAreaName
	case OpaqueScopeAS:
		return scopeASName
	default:
		return "unknown"
	}
}

// opaqueOrigination is one opaque LSA a consumer asks the framework to originate. The
// framework assigns the sequence number, builds the LSA header (LS type from the
// consumer's registered scope, Link State ID from the Opaque Type + this OpaqueID),
// installs it, and floods it to opaque-capable neighbors.
type opaqueOrigination struct {
	// OpaqueID is the low 24 bits of the Link State ID (the consumer's instance id).
	OpaqueID uint32
	// Area is the target area for an area-scope (Type 10) consumer; ignored otherwise.
	Area types.AreaID
	// Interface is the target interface for a link-scope (Type 9) consumer; ignored otherwise.
	Interface string
	// Body is the opaque information (the bytes after the 20-byte LSA header).
	Body []byte
	// Withdraw MaxAge-flushes a previously originated instance through the purge path.
	Withdraw bool
	// Scope optionally overrides the consumer's registered flooding scope for THIS
	// origination. Zero means "use the registered scope". A consumer that floods some of
	// its instances at a different scope than others (e.g. RFC 5392 inter-AS TE choosing
	// Type 10 vs Type 11 per link) sets it; single-scope consumers leave it zero.
	Scope OpaqueScope
}

// opaqueOriginateFunc returns the set of opaque LSAs a consumer currently wants
// originated for router. The framework calls it on each self-LSA origination pass; an
// unchanged return floods nothing (origination is idempotent).
type opaqueOriginateFunc func(router types.RouterID) []opaqueOrigination

// opaqueReceived is one received opaque LSA delivered to its registered consumer after a
// newer install. Body is a view valid only during the callback; a consumer that retains
// it MUST copy.
type opaqueReceived struct {
	OpaqueType        uint8
	OpaqueID          uint32
	Scope             OpaqueScope
	Area              types.AreaID
	Interface         string
	AdvertisingRouter types.RouterID
	Body              []byte
	// Age is the received LSA's LS age in seconds (RFC 2328 §12.1.1). It surfaces the grace
	// clock the RFC 3623 Grace-LSA helper needs (remaining grace = Grace Period - LS age); it
	// is additive and zero for consumers that do not consult it (TE/RI/Extended-Prefix/Link).
	Age uint16
	// Reachable is the RFC 5250 §5 originator reachability. It is always true for Type 9
	// and Type 10; for a Type-11 (AS-wide) LSA it is false when the originating router is
	// not reachable in the route table, and the consumer must not use the LSA.
	Reachable bool
	// Withdrawn is true when the delivered LSA is a MaxAge purge (RFC 2328 §14): the
	// originator or a MinLSInterval-aged instance is being removed. A consumer that keeps
	// derived state (e.g. a TED) removes the corresponding entry instead of upserting.
	Withdrawn bool
}

// opaqueReceiveFunc is invoked after a newer opaque LSA of the consumer's Opaque Type is
// installed (RFC 5250 §3). It is never called for an equal or older duplicate.
type opaqueReceiveFunc func(opaqueReceived)

type opaqueConsumer struct {
	opaqueType  uint8
	scope       OpaqueScope
	onOriginate opaqueOriginateFunc
	onReceive   opaqueReceiveFunc
}

var (
	opaqueMu        sync.RWMutex
	opaqueConsumers = map[uint8]opaqueConsumer{}
)

// ErrOpaqueScopeInvalid is returned when a consumer registers with a scope that is not
// one of the three RFC 5250 opaque scopes (link/area/AS).
var ErrOpaqueScopeInvalid = errors.New("ospf: invalid opaque scope (must be link/area/AS)")

// ErrOpaqueTypeRegistered is returned when a consumer registers an Opaque Type that
// another consumer already owns. RFC 5250 §9: an Opaque Type identifies one application.
var ErrOpaqueTypeRegistered = errors.New("ospf: opaque type already registered")

// registerOpaqueConsumer registers a consumer for one Opaque Type at one flooding scope.
// A consumer calls it from its own init(); the OSPF engine discovers the registration at
// startup and invokes onOriginate (self-LSA origination) and onReceive (after a newer
// install) at the correct times. Registering a duplicate Opaque Type is rejected -- an
// Opaque Type is owned by exactly one application (RFC 5250 §9). Either callback may be
// nil (an origination-only or reception-only consumer).
func registerOpaqueConsumer(opaqueType uint8, scope OpaqueScope, onOriginate opaqueOriginateFunc, onReceive opaqueReceiveFunc) error {
	if !scope.valid() {
		return ErrOpaqueScopeInvalid
	}
	opaqueMu.Lock()
	defer opaqueMu.Unlock()
	if _, dup := opaqueConsumers[opaqueType]; dup {
		return ErrOpaqueTypeRegistered
	}
	opaqueConsumers[opaqueType] = opaqueConsumer{opaqueType: opaqueType, scope: scope, onOriginate: onOriginate, onReceive: onReceive}
	return nil
}

// lookupOpaqueConsumer returns the consumer registered for opaqueType.
func lookupOpaqueConsumer(opaqueType uint8) (opaqueConsumer, bool) {
	opaqueMu.RLock()
	defer opaqueMu.RUnlock()
	c, ok := opaqueConsumers[opaqueType]
	return c, ok
}

// opaqueConsumerSnapshot returns a copy of every registered consumer, for the engine's
// per-pass origination discovery.
func opaqueConsumerSnapshot() []opaqueConsumer {
	opaqueMu.RLock()
	defer opaqueMu.RUnlock()
	out := make([]opaqueConsumer, 0, len(opaqueConsumers))
	for _, c := range opaqueConsumers {
		out = append(out, c)
	}
	return out
}

// resetOpaqueConsumers clears the registry. Test-only helper; production never calls it.
func resetOpaqueConsumers() {
	opaqueMu.Lock()
	opaqueConsumers = map[uint8]opaqueConsumer{}
	opaqueMu.Unlock()
}

// safeOpaqueCall runs a consumer callback, recovering from a panic so a single bad
// consumer cannot crash the engine or wedge the LSDB lock (RFC 5250 framework isolation,
// AC-15/R-9). onPanic increments ze_ospf_opaque_consumer_errors_total.
func safeOpaqueCall(onPanic, fn func()) {
	defer func() {
		if r := recover(); r != nil && onPanic != nil {
			onPanic()
		}
	}()
	fn()
}
