// Design: docs/architecture/core-design.md -- ProtocolID registry for redistribute
// Related: events.go -- payload types referenced by the registry
//
// The registry stores VALUE TYPES ONLY -- no handle pointers, no producer-
// allocated pointers of any kind. Producers and consumers each build their
// own typed handles locally via events.Register[*RouteChangeBatch] in their
// own package; the events registry's duplicate-registration guard accepts
// independent Register calls from different packages with the same
// (namespace, eventType, T) tuple because that guard is a CONTRACT check,
// not shared mutable state.
//
// Concurrency: writers serialize through writeMu. Readers (Producers,
// ProtocolName, ProtocolIDOf) take a read lock. Registration happens at
// init() time, which is sequential in Go, but tests reset and re-register;
// the lock keeps that race-free.

package redistevents

import (
	"slices"
	"sync"
)

// protocolEntry is the value-typed record stored for each registered
// protocol. No pointers escape the registry.
type protocolEntry struct {
	id          ProtocolID
	name        string
	hasProducer bool
}

var (
	mu sync.RWMutex
	// entries[0] is a sentinel so allocated ProtocolIDs start at 1.
	entries = []protocolEntry{{}}
	byName  = map[string]ProtocolID{}
)

// RegisterProtocol declares a route-producing protocol by canonical name and
// returns its allocated ProtocolID. Idempotent on name -- a second call with
// the same name returns the existing ID.
//
// Allocates IDs starting at 1; ID 0 is reserved as ProtocolUnspecified.
// Panics with the "BUG:" prefix if name is empty (programmer error: the
// caller must supply a non-empty canonical name at init time).
func RegisterProtocol(name string) ProtocolID {
	if name == "" {
		panic("BUG: redistevents.RegisterProtocol: empty name")
	}
	mu.Lock()
	defer mu.Unlock()
	if id, ok := byName[name]; ok {
		return id
	}
	// Guard against silent uint16 truncation. ProtocolID space is 1..65535;
	// entry 0 is the sentinel slot, so the first 65535 valid IDs come from
	// entries[1..65535]. Reject the 65536th allocation rather than wrap to
	// ProtocolUnspecified.
	if len(entries) >= 1<<16 {
		panic("BUG: redistevents.RegisterProtocol: ProtocolID space exhausted (>65535 protocols)")
	}
	id := ProtocolID(len(entries)) // first allocation: len==1, id=1
	entries = append(entries, protocolEntry{id: id, name: name})
	byName[name] = id
	return id
}

// RegisterProducer marks the named protocol as having a producer. Called
// from a producer's init() AFTER its events.Register call so consumers know
// to subscribe. Idempotent: calling twice for the same ID is a no-op.
//
// Panics with the "BUG:" prefix if id is unknown (programmer error: the
// caller must register the protocol via RegisterProtocol first, in the
// same init).
func RegisterProducer(id ProtocolID) {
	mu.Lock()
	defer mu.Unlock()
	if int(id) <= 0 || int(id) >= len(entries) {
		panic("BUG: redistevents.RegisterProducer: unknown ProtocolID")
	}
	entries[id].hasProducer = true
}

// Producers returns a fresh slice of ProtocolIDs that have a registered
// producer. Returned slice is independent of the registry; callers may sort
// or modify it without affecting subsequent calls.
//
// Order is sorted by ProtocolID (ascending) for deterministic consumer
// startup behavior.
func Producers() []ProtocolID {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]ProtocolID, 0, len(entries))
	for i := 1; i < len(entries); i++ {
		if entries[i].hasProducer {
			out = append(out, entries[i].id)
		}
	}
	slices.Sort(out)
	return out
}

// ProtocolName returns a copy of the canonical name for id, or the empty
// string if id is unregistered or ProtocolUnspecified. Diagnostic / startup
// path -- not on the per-event hot path.
func ProtocolName(id ProtocolID) string {
	mu.RLock()
	defer mu.RUnlock()
	if int(id) <= 0 || int(id) >= len(entries) {
		return ""
	}
	return entries[id].name
}

// ProtocolIDOf returns the ProtocolID for name, or (ProtocolUnspecified,
// false) if name is unknown. Used by consumers at startup to learn the
// canonical IDs of protocols they care about (e.g. to filter out their own
// protocol via ProtocolIDOf("bgp")).
func ProtocolIDOf(name string) (ProtocolID, bool) {
	mu.RLock()
	defer mu.RUnlock()
	id, ok := byName[name]
	return id, ok
}

// ResetForTest clears the registry. Tests call this to start from a clean
// slate. NOT for production use.
func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	entries = []protocolEntry{{}}
	byName = map[string]ProtocolID{}
}
