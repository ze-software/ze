// Design: docs/architecture/plugin/rib-storage-design.md -- RIB storage internals
// RFC: rfc/short/rfc8277.md -- Sections 2.2 and 2.3, the labeled NLRI a walk rebuilds
// Related: familyrib.go -- IterateEntry, the walk by stored key, and the label side-data
// Related: familyrib_opaque.go -- iterateOpaque, which already yields each path's wire route

package storage

import (
	"encoding/binary"
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/store"
)

// pathIDOctets is the size of the RFC 7911 Path Identifier that frames an
// ADD-PATH NLRI.
const pathIDOctets = 4

// labeledWireOctetsMax sizes the buffer a walk rebuilds one labeled unicast
// NLRI into: a Path Identifier and the longest NLRI its Length field allows.
const labeledWireOctetsMax = pathIDOctets + nlri.LabeledNLRIOctetsMax

// prefixNLRIOctetsMax is the longest [prefix length][prefix] of a CIDR family:
// one length octet and an IPv6 address.
const prefixNLRIOctetsMax = 1 + 16

// IterateFamilyWire calls fn for each path of fam with the NLRI an UPDATE
// carries for it, and stops when fn returns false. It is IterateFamily for a
// reader that writes the route for another parser (the MRT RIB snapshot): the
// fields the stored key leaves out are back in place. The NLRI is valid for
// the duration of that callback only, so fn MUST copy what it keeps.
//
// fn runs with this type's read lock held, and owes it the same obligation
// Iterate states.
func (r *PeerRIB) IterateFamilyWire(fam family.Family, fn func(nlriBytes []byte, entry RouteEntry) bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rib, exists := r.families[fam]
	if !exists {
		return
	}
	rib.IterateWire(fn)
}

// IterateWire calls fn for each stored path with the NLRI an UPDATE carries
// for it, framed with its Path Identifier under ADD-PATH, and stops when fn
// returns false.
//
// Three storage shapes answer it:
//
//   - IPv4/IPv6 unicast and multicast key a route by its whole NLRI, so the
//     key IterateEntry yields is the wire NLRI.
//   - A non-CIDR family (VPN, EVPN, FlowSpec, FlowSpec VPN, MVPN, BGP-LS) keys a
//     route by RouteKey, which drops labels, ESI and gateway, and keeps the NLRI
//     each path was received with beside it (FamilyRIB.wire): IterateEntry
//     already yields that wire route through iterateOpaque.
//   - Labeled unicast (SAFI 4) keys a route by its bare prefix and holds the
//     label stack as side-data, so its NLRI is rebuilt here from the two.
//
// A labeled path with no label stack bound is left out: the ingest stores the
// route and binds its labels in two steps (rib.insertLabeledEntry), so a walk
// between them meets a path whose announcement is not stored whole yet, and
// no labeled NLRI exists for it. The walk then reads as one taken before that
// announcement arrived.
func (r *FamilyRIB) IterateWire(fn func(nlriBytes []byte, entry RouteEntry) bool) {
	if !r.labelsApart() {
		r.IterateEntry(fn)
		return
	}
	var wire [labeledWireOctetsMax]byte
	if !r.addPath {
		r.direct.Iterate(func(pfx netip.Prefix, entry RouteEntry) bool {
			wireNLRI, ok := r.labeledWireNLRI(wire[:], 0, pfx, r.LookupLabels(0, pfx))
			if !ok {
				return true
			}
			return fn(wireNLRI, entry)
		})
		return
	}
	r.multi.Iterate(func(pfx netip.Prefix, ps pathSet) bool {
		for i := range ps.entries {
			path := &ps.entries[i]
			wireNLRI, ok := r.labeledWireNLRI(wire[:], path.pathID, pfx, path.labels)
			if !ok {
				continue
			}
			if !fn(wireNLRI, path.entry) {
				return false
			}
		}
		return true
	})
}

// labelsApart reports whether this family stores a route's label stack apart
// from its key: labeled unicast in CIDR storage. A labeled SAFI of another AFI
// is stored opaque, with its wire route whole.
func (r *FamilyRIB) labelsApart() bool {
	if !r.labeled {
		return false
	}
	return r.cidr
}

// labeledWireNLRI writes into dst the labeled unicast NLRI of the path
// (pathID, pfx) bound to the label stack h, and returns it:
//
//	ADD-PATH:  [Path Identifier (4, big-endian)][Length][labels][prefix]
//	otherwise: [Length][labels][prefix]
//
// The labeled part is nlri.WriteLabeledNLRI's, the one declaration of the RFC
// 8277 Section 2.3 layout. It answers false for a path with no label stack
// (IterateWire says when that happens) and for a dst too short. dst MUST hold
// labeledWireOctetsMax octets. It allocates nothing for a stack of at most
// nlri.LabeledNLRILabelsMax labels, the most a received NLRI can carry.
func (r *FamilyRIB) labeledWireNLRI(dst []byte, pathID uint32, pfx netip.Prefix, h attrpool.Handle) ([]byte, bool) {
	var stack [nlri.LabeledNLRILabelsMax]uint32
	labels := pool.AppendLabels(stack[:0], h)
	var key [prefixNLRIOctetsMax]byte
	prefixNLRI := store.PrefixToNLRIInto(pfx, key[:])
	head := 0
	if r.addPath {
		if len(dst) < pathIDOctets {
			return nil, false
		}
		// RFC 8277 Section 2.2: "If the procedures of [RFC7911] are being used,
		// a four-octet "path identifier" (as defined in Section 3 of [RFC7911])
		// is part of the NLRI and precedes the Length field."
		binary.BigEndian.PutUint32(dst, pathID)
		head = pathIDOctets
	}
	// RFC 8277 Section 2.3
	octets, ok := nlri.WriteLabeledNLRI(dst, head, prefixNLRI, labels)
	if !ok {
		return nil, false
	}
	return dst[:head+octets], true
}
