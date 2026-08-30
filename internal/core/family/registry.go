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

	"github.com/ze-software/ze/internal/core/textbuf"
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
	afi      AFI
	safi     SAFI
	afiName  string
	safiName string
}

// FamilyRegistration describes one runtime family registration request.
// RegisterFamilyBatch uses this value type so callers can validate and commit
// a plugin's declared families as one atomic batch.
type FamilyRegistration struct {
	AFI      AFI
	SAFI     SAFI
	AFIName  string
	SAFIName string
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
	IPv4Unicast   = MustRegister(AFIIPv4, SAFIUnicast, afiNameIPv4, "unicast")
	IPv6Unicast   = MustRegister(AFIIPv6, SAFIUnicast, afiNameIPv6, "unicast")
	IPv4Multicast = MustRegister(AFIIPv4, SAFIMulticast, afiNameIPv4, "multicast")
	IPv6Multicast = MustRegister(AFIIPv6, SAFIMulticast, afiNameIPv6, "multicast")
)

// RegisterFamily registers a family with its AFI/SAFI names. Returns the Family value.
//
// The canonical family string is derived as afiName + "/" + safiName.
// Re-registration with identical values is a no-op.
// Re-registration with conflicting AFI or SAFI names returns an error.
//
// Called from plugin init() for internal plugins, and at runtime for external plugins.
//
// Concurrency: writers serialize through writeMu. Readers never block on
// RegisterFamily because they read state.Load() instead of taking writeMu.
func RegisterFamily(afi AFI, safi SAFI, afiName, safiName string) (Family, error) {
	writeMu.Lock()
	defer writeMu.Unlock()

	if afiName == "" || safiName == "" {
		return Family{}, fmt.Errorf("%w: AFI %d SAFI %d", ErrEmptyName, afi, safi)
	}

	cur := state.Load()

	if existing, ok := cur.afiNames[afi]; ok {
		if existing != afiName {
			return Family{}, fmt.Errorf("%w: AFI %d is %q, got %q", ErrAFIConflict, afi, existing, afiName)
		}
	}

	if existing, ok := cur.safiNames[safi]; ok {
		if existing != safiName {
			return Family{}, fmt.Errorf("%w: SAFI %d is %q, got %q", ErrSAFIConflict, safi, existing, safiName)
		}
	}

	f := Family{AFI: afi, SAFI: safi}
	var tb textbuf.Buffer
	canonical := tb.Str(afiName).Byte('/').Str(safiName).String()
	if _, ok := cur.familyByName[canonical]; ok {
		return f, nil
	}

	// Record in the write-only workspace, then build a fresh snapshot.
	registrations = append(registrations, familyRegistration{afi: afi, safi: safi, afiName: afiName, safiName: safiName})

	next := &registry{
		afiNames:     maps.Clone(cur.afiNames),
		safiNames:    maps.Clone(cur.safiNames),
		familyByName: maps.Clone(cur.familyByName),
		afiByName:    maps.Clone(cur.afiByName),
		safiByName:   maps.Clone(cur.safiByName),
	}
	next.afiNames[afi] = afiName
	next.safiNames[safi] = safiName
	next.familyByName[canonical] = f
	next.afiByName[afiName] = afi
	next.safiByName[safiName] = safi
	next.pack, next.idx = buildPack(registrations)

	state.Store(next)
	return f, nil
}

// RegisterFamilyBatch registers a set of families atomically. Either every
// non-duplicate family becomes visible in one snapshot, or none of them do.
// It returns the registrations that were newly added so callers can roll them
// back if a later startup stage fails.
func RegisterFamilyBatch(reqs []FamilyRegistration) ([]FamilyRegistration, error) {
	writeMu.Lock()
	defer writeMu.Unlock()

	cur := state.Load()
	next := &registry{
		afiNames:     maps.Clone(cur.afiNames),
		safiNames:    maps.Clone(cur.safiNames),
		familyByName: maps.Clone(cur.familyByName),
		afiByName:    maps.Clone(cur.afiByName),
		safiByName:   maps.Clone(cur.safiByName),
	}
	nextRegistrations := append([]familyRegistration(nil), registrations...)
	added := make([]FamilyRegistration, 0, len(reqs))

	for _, req := range reqs {
		if req.AFIName == "" || req.SAFIName == "" {
			return nil, fmt.Errorf("%w: AFI %d SAFI %d", ErrEmptyName, req.AFI, req.SAFI)
		}
		if existing, ok := next.afiNames[req.AFI]; ok && existing != req.AFIName {
			return nil, fmt.Errorf("%w: AFI %d is %q, got %q", ErrAFIConflict, req.AFI, existing, req.AFIName)
		}
		if existing, ok := next.safiNames[req.SAFI]; ok && existing != req.SAFIName {
			return nil, fmt.Errorf("%w: SAFI %d is %q, got %q", ErrSAFIConflict, req.SAFI, existing, req.SAFIName)
		}

		f := Family{AFI: req.AFI, SAFI: req.SAFI}
		var tb textbuf.Buffer
		canonical := tb.Str(req.AFIName).Byte('/').Str(req.SAFIName).String()
		if _, ok := next.familyByName[canonical]; ok {
			continue
		}

		next.afiNames[req.AFI] = req.AFIName
		next.safiNames[req.SAFI] = req.SAFIName
		next.familyByName[canonical] = f
		next.afiByName[req.AFIName] = req.AFI
		next.safiByName[req.SAFIName] = req.SAFI
		nextRegistrations = append(nextRegistrations, familyRegistration{
			afi: req.AFI, safi: req.SAFI, afiName: req.AFIName, safiName: req.SAFIName,
		})
		added = append(added, req)
	}

	if len(added) == 0 {
		return nil, nil
	}
	next.pack, next.idx = buildPack(nextRegistrations)
	registrations = nextRegistrations
	state.Store(next)
	return added, nil
}

// UnregisterFamilyBatch removes registrations previously returned by
// RegisterFamilyBatch. It is intended for startup rollback of dynamic plugin
// family declarations.
func UnregisterFamilyBatch(reqs []FamilyRegistration) {
	if len(reqs) == 0 {
		return
	}
	writeMu.Lock()
	defer writeMu.Unlock()

	remove := make(map[FamilyRegistration]struct{}, len(reqs))
	for _, req := range reqs {
		remove[req] = struct{}{}
	}

	filtered := make([]familyRegistration, 0, len(registrations))
	changed := false
	for _, reg := range registrations {
		req := FamilyRegistration{AFI: reg.afi, SAFI: reg.safi, AFIName: reg.afiName, SAFIName: reg.safiName}
		if _, ok := remove[req]; ok {
			changed = true
			continue
		}
		filtered = append(filtered, reg)
	}
	if !changed {
		return
	}
	registrations = filtered
	state.Store(registryFromRegistrations(filtered))
}

func registryFromRegistrations(regs []familyRegistration) *registry {
	next := newEmptyState()
	for _, reg := range regs {
		f := Family{AFI: reg.afi, SAFI: reg.safi}
		var tb textbuf.Buffer
		canonical := tb.Str(reg.afiName).Byte('/').Str(reg.safiName).String()
		next.afiNames[reg.afi] = reg.afiName
		next.safiNames[reg.safi] = reg.safiName
		next.familyByName[canonical] = f
		next.afiByName[reg.afiName] = reg.afi
		next.safiByName[reg.safiName] = reg.safi
	}
	next.pack, next.idx = buildPack(regs)
	return next
}

// MustRegister wraps RegisterFamily and panics on error. Use from package init()
// where any registration error indicates a programming bug (conflicting names,
// empty strings) that must abort startup.
func MustRegister(afi AFI, safi SAFI, afiName, safiName string) Family {
	f, err := RegisterFamily(afi, safi, afiName, safiName)
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
		var tb textbuf.Buffer
		s := tb.Str(r.afiName).Byte('/').Str(r.safiName).String()
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
