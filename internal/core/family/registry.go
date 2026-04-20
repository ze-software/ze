// Design: docs/architecture/wire/nlri.md -- family registration and string cache
// Overview: family.go -- Family type definition

package family

import (
	"errors"
	"fmt"
	"maps"
	"strconv"
	"sync"
	"sync/atomic"
	"unsafe"
)

// Registration errors.
var (
	ErrEmptyName    = errors.New("family: empty AFI or SAFI name")
	ErrAFIConflict  = errors.New("family: AFI name conflict")
	ErrSAFIConflict = errors.New("family: SAFI name conflict")
)

// familyAFISlots is the number of known AFI values for the family string cache.
const familyAFISlots = 4

// afiSlot maps known AFI values to compact indices for the family string cache.
// Returns -1 for unknown AFIs (rare, not on hot path).
func afiSlot(a AFI) int {
	switch a { //nolint:exhaustive // only 4 known AFIs need cache slots
	case AFIIPv4:
		return 0
	case AFIIPv6:
		return 1
	case AFIL2VPN:
		return 2
	case AFIBGPLS:
		return 3
	}
	return -1
}

// registry is the single immutable snapshot of all registry state, swapped
// atomically on each registration. The hot path (Family.String, AFI.String,
// SAFI.String) and the cold path (LookupFamily, RegisteredFamilyNames) read
// the current snapshot via state.Load() and never take a mutex. Writers
// serialize through writeMu, build a fresh snapshot, and atomically swap.
//
// Old snapshots are kept alive by the GC as long as any string returned via
// unsafe.String references their pack buffer.
type registry struct {
	pack         []byte                     // [spans N*4][string data] contiguous
	idx          [familyAFISlots][256]uint8 // [afiSlot][SAFI] -> 1-based span index
	afiNames     map[AFI]string             // immutable snapshot for AFI.String()
	safiNames    map[SAFI]string            // immutable snapshot for SAFI.String()
	familyByName map[string]Family          // immutable snapshot for LookupFamily()
	afiByName    map[string]AFI             // immutable snapshot for LookupAFI()
	safiByName   map[string]SAFI            // immutable snapshot for LookupSAFI()
}

// familyRegistration holds one entry collected by RegisterFamily.
type familyRegistration struct {
	afi     AFI
	safi    SAFI
	afiStr  string
	safiStr string
}

var (
	// writeMu serializes concurrent RegisterFamily calls. Readers never take it
	// -- they read the current state snapshot via state.Load().
	writeMu sync.Mutex

	// registrations is the write-only workspace used to rebuild the packed buffer.
	// Only mutated under writeMu.
	registrations []familyRegistration

	// state is the current immutable snapshot. Swapped atomically on each
	// successful RegisterFamily.
	state atomic.Pointer[registry]
)

// emptyState is stored at package init so readers never dereference nil.
var _ = initEmptyState()

func initEmptyState() bool {
	state.Store(newEmptyState())
	return true
}

func newEmptyState() *registry {
	return &registry{
		afiNames:     map[AFI]string{},
		safiNames:    map[SAFI]string{},
		familyByName: map[string]Family{},
		afiByName:    map[string]AFI{},
		safiByName:   map[string]SAFI{},
	}
}

// Base families defined by RFC 4760 itself, registered at package init.
//
// These are the families with universal AFI/SAFI numbers that every BGP
// implementation supports. They live in the family package (not in a plugin)
// because they are protocol-defined, not feature-defined.
//
// Plugin-specific families (FlowSpec, EVPN, MVPN, etc.) live in their plugins
// per the registration ownership pattern.
//
// Declaration order matters: these vars must be initialized AFTER initEmptyState
// above so that MustRegister sees a non-nil state pointer.
var (
	IPv4Unicast   = MustRegister(AFIIPv4, SAFIUnicast, "ipv4", "unicast")
	IPv6Unicast   = MustRegister(AFIIPv6, SAFIUnicast, "ipv6", "unicast")
	IPv4Multicast = MustRegister(AFIIPv4, SAFIMulticast, "ipv4", "multicast")
	IPv6Multicast = MustRegister(AFIIPv6, SAFIMulticast, "ipv6", "multicast")
)

// RegisterFamily registers a family with its AFI/SAFI names. Returns the Family value.
//
// The canonical family string is derived as afiStr + "/" + safiStr.
// Re-registration with identical values is a no-op.
// Re-registration with conflicting AFI or SAFI names returns an error.
//
// Called from plugin init() for internal plugins, and at runtime for external plugins.
//
// Concurrency: writers serialize through writeMu. Readers never block on
// RegisterFamily because they read state.Load() instead of taking writeMu.
func RegisterFamily(afi AFI, safi SAFI, afiStr, safiStr string) (Family, error) {
	writeMu.Lock()
	defer writeMu.Unlock()

	if afiStr == "" || safiStr == "" {
		return Family{}, fmt.Errorf("%w: AFI %d SAFI %d", ErrEmptyName, afi, safi)
	}

	cur := state.Load()

	if existing, ok := cur.afiNames[afi]; ok {
		if existing != afiStr {
			return Family{}, fmt.Errorf("%w: AFI %d is %q, got %q", ErrAFIConflict, afi, existing, afiStr)
		}
	}

	if existing, ok := cur.safiNames[safi]; ok {
		if existing != safiStr {
			return Family{}, fmt.Errorf("%w: SAFI %d is %q, got %q", ErrSAFIConflict, safi, existing, safiStr)
		}
	}

	f := Family{AFI: afi, SAFI: safi}
	canonical := afiStr + "/" + safiStr
	if _, ok := cur.familyByName[canonical]; ok {
		return f, nil
	}

	// Record in the write-only workspace, then build a fresh snapshot.
	registrations = append(registrations, familyRegistration{afi: afi, safi: safi, afiStr: afiStr, safiStr: safiStr})

	next := &registry{
		afiNames:     maps.Clone(cur.afiNames),
		safiNames:    maps.Clone(cur.safiNames),
		familyByName: maps.Clone(cur.familyByName),
		afiByName:    maps.Clone(cur.afiByName),
		safiByName:   maps.Clone(cur.safiByName),
	}
	next.afiNames[afi] = afiStr
	next.safiNames[safi] = safiStr
	next.familyByName[canonical] = f
	next.afiByName[afiStr] = afi
	next.safiByName[safiStr] = safi
	next.pack, next.idx = buildPack(registrations)

	state.Store(next)
	return f, nil
}

// MustRegister wraps RegisterFamily and panics on error. Use from package init()
// where any registration error indicates a programming bug (conflicting names,
// empty strings) that must abort startup.
func MustRegister(afi AFI, safi SAFI, afiStr, safiStr string) Family {
	f, err := RegisterFamily(afi, safi, afiStr, safiStr)
	if err != nil {
		panic("BUG: family.MustRegister: " + err.Error())
	}
	return f
}

// RegisteredFamilyNames returns all registered canonical family names.
// Lock-free: reads from the current state snapshot.
func RegisteredFamilyNames() []string {
	cur := state.Load()
	names := make([]string, 0, len(cur.familyByName))
	for name := range cur.familyByName {
		names = append(names, name)
	}
	return names
}

// LookupFamily looks up a canonical family name and returns the Family value.
// Returns zero Family and false if the name is not registered.
// Lock-free: reads from the current state snapshot.
func LookupFamily(s string) (Family, bool) {
	f, ok := state.Load().familyByName[s]
	return f, ok
}

// LookupAFI looks up an AFI by its registered name (e.g., "ipv4", "ipv6",
// "l2vpn", "bgp-ls"). Returns the AFI value and true on hit, zero and false
// on miss. Lock-free: reads from the current state snapshot.
func LookupAFI(name string) (AFI, bool) {
	a, ok := state.Load().afiByName[name]
	return a, ok
}

// LookupSAFI looks up a SAFI by its registered name (e.g., "unicast",
// "multicast", "evpn"). Returns the SAFI value and true on hit, zero and
// false on miss. Lock-free: reads from the current state snapshot.
func LookupSAFI(name string) (SAFI, bool) {
	s, ok := state.Load().safiByName[name]
	return s, ok
}

// buildPack builds the packed string buffer + AFI/SAFI index from a slice of
// registrations. Used by RegisterFamily to build the next snapshot.
func buildPack(regs []familyRegistration) ([]byte, [familyAFISlots][256]uint8) {
	type span struct{ pos, size uint16 }
	var spans []span
	var strBuf []byte
	var idx [familyAFISlots][256]uint8

	for _, r := range regs {
		slot := afiSlot(r.afi)
		if slot < 0 {
			continue
		}
		s := r.afiStr + "/" + r.safiStr
		pos := uint16(len(strBuf))
		strBuf = append(strBuf, s...)
		spans = append(spans, span{pos: pos, size: uint16(len(s))})
		idx[slot][r.safi] = uint8(len(spans)) // 1-based
	}

	spanBytes := uint16(len(spans) * 4)
	pack := make([]byte, int(spanBytes)+len(strBuf))
	for i, sp := range spans {
		off := i * 4
		absPos := spanBytes + sp.pos
		pack[off] = byte(absPos)
		pack[off+1] = byte(absPos >> 8)
		pack[off+2] = byte(sp.size)
		pack[off+3] = byte(sp.size >> 8)
	}
	copy(pack[spanBytes:], strBuf)

	return pack, idx
}

// lookupFamilyString returns the cached string for a Family, or empty string if not found.
// Lock-free: reads from the current state snapshot.
func lookupFamilyString(f Family) string {
	cur := state.Load()
	slot := afiSlot(f.AFI)
	if slot < 0 {
		return ""
	}
	idx := cur.idx[slot][f.SAFI]
	if idx == 0 {
		return ""
	}
	off := int(idx-1) * 4
	pos := int(cur.pack[off]) | int(cur.pack[off+1])<<8
	size := int(cur.pack[off+2]) | int(cur.pack[off+3])<<8
	return unsafe.String(&cur.pack[pos], size) //nolint:gosec // audited: pack is immutable after Store
}

// lookupAFIName returns the registered name for an AFI, or empty string.
// Lock-free: reads from the current state snapshot.
func lookupAFIName(a AFI) string {
	return state.Load().afiNames[a]
}

// lookupSAFIName returns the registered name for a SAFI, or empty string.
// Lock-free: reads from the current state snapshot.
func lookupSAFIName(s SAFI) string {
	return state.Load().safiNames[s]
}

// afiStringFallback formats an unregistered AFI as "afi-N".
func afiStringFallback(a AFI) string {
	var buf [20]byte
	b := append(buf[:0], "afi-"...)
	b = strconv.AppendUint(b, uint64(a), 10)
	return string(b)
}

// safiStringFallback formats an unregistered SAFI as "safi-N".
func safiStringFallback(s SAFI) string {
	var buf [20]byte
	b := append(buf[:0], "safi-"...)
	b = strconv.AppendUint(b, uint64(s), 10)
	return string(b)
}

// ResetRegistry clears all registrations. Only for use in tests.
func ResetRegistry() {
	writeMu.Lock()
	defer writeMu.Unlock()
	registrations = nil
	state.Store(newEmptyState())
}
