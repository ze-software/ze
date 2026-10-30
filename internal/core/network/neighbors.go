// Design: docs/architecture/core-design.md — the kernel neighbor table, read through a registered reader
// Overview: connected.go — locally connected subnets, the other half of RFC 2545 Section 3's host facts
// RFC: rfc/short/rfc2545.md
//
// The neighbor table is how BGP learns the Link-Local of another router on a
// shared subnet without importing the interface component. The component
// registers one reader at init(); a caller asks ReadNeighbors and gets value
// types. With no reader linked the answer is ErrNeighborReaderNotRegistered,
// never an empty table (ai/rules/principles.md).

package network

import (
	"errors"
	"net/netip"
	"sync"
)

// NeighborBinding says whether a kernel neighbor entry binds its address to a
// link-layer address now. The zero value is unspecified, never a binding.
type NeighborBinding uint8

const (
	NeighborBindingUnspecified NeighborBinding = iota
	// NeighborBindingValid is REACHABLE, STALE, DELAY, PROBE or PERMANENT. Linux
	// holds a working neighbor in STALE most of the time, so STALE binds too.
	NeighborBindingValid
	// NeighborBindingInvalid is every other state: INCOMPLETE, FAILED, NOARP,
	// NONE, or a state the reader does not know. None of them names a working
	// link-layer address.
	NeighborBindingInvalid
)

// Neighbor is one entry of the kernel neighbor table, as the registered reader
// reports it. LinkAddress is the hardware address in one canonical spelling the
// reader chooses, compared only for equality, and empty when the kernel holds
// none.
type Neighbor struct {
	Address     netip.Addr
	LinkAddress string
	Device      string
	Binding     NeighborBinding
}

// NeighborReader reads the IPv6 entries of the kernel neighbor table.
type NeighborReader func() ([]Neighbor, error)

// ErrNeighborReaderNotRegistered is ReadNeighbors' answer in a build where no
// component registered a reader, which is distinct from a table that is empty.
var ErrNeighborReaderNotRegistered = errors.New("no neighbor table reader is registered: this build carries no interface component")

var neighborReader struct {
	sync.Mutex
	read NeighborReader
}

// RegisterNeighborReader installs the one reader of the neighbor table. The
// interface component calls it from its init(), so this package imports no
// component. A second registration is a build defect.
func RegisterNeighborReader(read NeighborReader) {
	neighborReader.Lock()
	defer neighborReader.Unlock()
	if neighborReader.read != nil {
		panic("BUG: neighbor table reader registered twice")
	}
	neighborReader.read = read
}

// ReadNeighbors reads the IPv6 neighbor table through the registered reader.
// It reads the kernel, so it belongs at a session or configuration boundary,
// or on a neighbor event, never on a per-message path. Safe for concurrent use.
func ReadNeighbors() ([]Neighbor, error) {
	neighborReader.Lock()
	read := neighborReader.read
	neighborReader.Unlock()
	if read == nil {
		return nil, ErrNeighborReaderNotRegistered
	}
	return read()
}

// LinkLocalOutcome is the answer to "which Link-Local does the router holding
// this global address use". The zero value is unspecified, never an answer.
type LinkLocalOutcome uint8

const (
	LinkLocalOutcomeUnspecified LinkLocalOutcome = iota
	// LinkLocalFound: exactly one resolved Link-Local entry shares the global
	// entry's device and link-layer address.
	LinkLocalFound
	// LinkLocalNoGlobalEntry: the table holds no entry for the global address.
	LinkLocalNoGlobalEntry
	// LinkLocalGlobalUnbound: the global's entry names no working link-layer
	// address (INCOMPLETE, FAILED, or no address recorded).
	LinkLocalGlobalUnbound
	// LinkLocalNoCandidate: no resolved Link-Local entry shares the global
	// entry's device and link-layer address.
	LinkLocalNoCandidate
	// LinkLocalAmbiguous: two or more Link-Locals share that link-layer
	// address, or the global's entries name different routers.
	LinkLocalAmbiguous
)

// String names the outcome for a log line; never compare with it.
func (o LinkLocalOutcome) String() string {
	switch o {
	case LinkLocalFound:
		return "found"
	case LinkLocalNoGlobalEntry:
		return "the neighbor table holds no entry for the next hop"
	case LinkLocalGlobalUnbound:
		return "the neighbor entry of the next hop is not resolved (incomplete or failed)"
	case LinkLocalNoCandidate:
		return "no resolved link-local neighbor entry has the link-layer address of the next hop"
	case LinkLocalAmbiguous:
		return "more than one link-local neighbor entry has the link-layer address of the next hop"
	case LinkLocalOutcomeUnspecified:
		return "unspecified"
	default:
		// The set is closed, but String also renders a corrupt value in a log.
		return "invalid"
	}
}

// NeighborLinkLocals maps each IPv6 global neighbor to the Link-Local address
// of the same router, derived from one read of the neighbor table.
//
// The kernel table maps an address to a device and a link-layer address. It
// never maps a global address to a Link-Local, so the pairing is derived: the
// global's entry gives the router's link-layer address on a device, and a
// fe80::/10 entry on the same device with the same link-layer address is that
// router's Link-Local.
//
// Immutable after NewNeighborLinkLocals, so safe for concurrent use. Its size is
// bounded by the table it was built from, which the kernel bounds
// (net.ipv6.neigh.default.gc_thresh3).
type NeighborLinkLocals struct {
	byGlobal map[netip.Addr]linkLocalAnswer
}

type linkLocalAnswer struct {
	linkLocal netip.Addr
	outcome   LinkLocalOutcome
}

// neighborLink is a router's identity on one device.
type neighborLink struct {
	device      string
	linkAddress string
}

// NewNeighborLinkLocals derives the global-to-Link-Local pairing from entries.
func NewNeighborLinkLocals(entries []Neighbor) *NeighborLinkLocals {
	candidates := make(map[neighborLink][]netip.Addr)
	for i := range entries {
		entry := &entries[i]
		address := entry.Address.Unmap()
		if !address.Is6() || !address.IsLinkLocalUnicast() {
			continue
		}
		if entry.Binding != NeighborBindingValid {
			continue
		}
		if entry.LinkAddress == "" {
			continue
		}
		key := neighborLink{device: entry.Device, linkAddress: entry.LinkAddress}
		candidates[key] = append(candidates[key], address.WithZone(""))
	}

	byGlobal := make(map[netip.Addr]linkLocalAnswer)
	for i := range entries {
		entry := &entries[i]
		address := entry.Address.Unmap()
		if !address.Is6() || !address.IsGlobalUnicast() {
			continue
		}
		answer := linkLocalAnswerFor(entry, candidates)
		if previous, seen := byGlobal[address]; seen && previous != answer {
			// One global on two devices naming two different routers: the
			// table does not say which one the next hop is.
			answer = linkLocalAnswer{outcome: LinkLocalAmbiguous}
		}
		byGlobal[address] = answer
	}
	return &NeighborLinkLocals{byGlobal: byGlobal}
}

// linkLocalAnswerFor pairs one global entry with the candidates on its link.
func linkLocalAnswerFor(entry *Neighbor, candidates map[neighborLink][]netip.Addr) linkLocalAnswer {
	if entry.Binding != NeighborBindingValid {
		return linkLocalAnswer{outcome: LinkLocalGlobalUnbound}
	}
	if entry.LinkAddress == "" {
		return linkLocalAnswer{outcome: LinkLocalGlobalUnbound}
	}
	found := candidates[neighborLink{device: entry.Device, linkAddress: entry.LinkAddress}]
	switch len(found) {
	case 0:
		return linkLocalAnswer{outcome: LinkLocalNoCandidate}
	case 1:
		return linkLocalAnswer{linkLocal: found[0], outcome: LinkLocalFound}
	default:
		return linkLocalAnswer{outcome: LinkLocalAmbiguous}
	}
}

// LinkLocalOf answers which Link-Local the router holding global uses. The
// address is valid only when the outcome is LinkLocalFound.
func (n *NeighborLinkLocals) LinkLocalOf(global netip.Addr) (netip.Addr, LinkLocalOutcome) {
	if n == nil {
		panic("BUG: LinkLocalOf on a nil NeighborLinkLocals")
	}
	answer, ok := n.byGlobal[global.Unmap()]
	if !ok {
		return netip.Addr{}, LinkLocalNoGlobalEntry
	}
	return answer.linkLocal, answer.outcome
}

// Equal reports whether two pairings answer every global the same way. Two nil
// pairings are equal; a nil and a built one are not.
func (n *NeighborLinkLocals) Equal(other *NeighborLinkLocals) bool {
	if n == nil {
		return other == nil
	}
	if other == nil {
		return false
	}
	if len(n.byGlobal) != len(other.byGlobal) {
		return false
	}
	for global, answer := range n.byGlobal {
		if other.byGlobal[global] != answer {
			return false
		}
	}
	return true
}
