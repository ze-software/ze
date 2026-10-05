// Design: docs/architecture/plugin/rib-storage-design.md -- RIB storage internals
// Related: pathset.go -- per-prefix path-id bookkeeping used under ADD-PATH (CIDR families)

package storage

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/store"
)

// FamilyRIB stores routes with per-attribute-type deduplication. Each route
// has its own RouteEntry with handles to individual attribute pools; routes
// sharing common attributes share pool entries.
//
// FamilyRIB picks its internal backend at construction from the (CIDR,
// ADD-PATH) pair:
//
//	                 | !addPath          | addPath
//	-----------------+-------------------+-----------------------
//	CIDR family      | direct (BART)     | multi (BART, pathSet)
//	non-CIDR family  | opaque (map)      | opaqueMulti (map, pathSet)
//
// CIDR families (IPv4/IPv6 unicast and multicast) have [prefix-len][addr]
// wire NLRIs that fit a netip.Prefix; BART gives longest-prefix match and
// compact memory. Path-id lives in the value layer (pathSet) so the BART
// key stays a bare prefix.
//
// Non-CIDR families (flow, EVPN, VPN, MVPN, MUP, RTC, bgp-ls) have NLRIs
// with arbitrary internal structure. They go in a plain map keyed by the
// wire bytes of the route without its path identifier (FlowSpec with its
// shortest length field, see opaqueKey). Under ADD-PATH the path identifier
// lives in a pathSet, as it does for a CIDR prefix, so every path of one route
// is one map entry whatever framing the session that sent it uses
// (familyrib_opaque.go).
// Specialised per-family indexes (e.g. EVPN route-type hashing, flowspec
// component decoding) can be added behind this same API without touching
// callers.
type FamilyRIB struct {
	fam         family.Family
	addPath     bool
	cidr        bool
	labeled     bool                          // SAFI 4: labels stored as side-data
	direct      *store.Store[RouteEntry]      // cidr && !addPath
	multi       *store.Store[pathSet]         // cidr && addPath
	opaque      map[string]RouteEntry         // !cidr && !addPath
	opaqueMulti map[string]pathSet            // !cidr && addPath: keyed by the route without its path id
	wire        map[opaquePath]string         // !cidr: the wire route of each path whose route key drops fields (labels, ESI)
	labels      *store.Store[attrpool.Handle] // labeled && cidr && !addPath: parallel BART for label handles; under ADD-PATH each pathEntry holds its own
}

// newFamilyRIB creates a FamilyRIB for the given address family.
func newFamilyRIB(fam family.Family, addPath bool) *FamilyRIB {
	r := &FamilyRIB{
		fam:     fam,
		addPath: addPath,
		cidr:    IsCIDRFamily(fam),
		labeled: fam.SAFI == family.SAFIMPLSLabel,
	}
	if !r.cidr {
		r.wire = make(map[opaquePath]string)
	}
	switch {
	case !r.cidr && addPath:
		r.opaqueMulti = make(map[string]pathSet)
	case !r.cidr:
		r.opaque = make(map[string]RouteEntry)
	case addPath:
		r.multi = store.NewStore[pathSet](fam)
	default:
		r.direct = store.NewStore[RouteEntry](fam)
	}
	if r.labeled && r.cidr && !addPath {
		r.labels = store.NewStore[attrpool.Handle](fam)
	}
	return r
}

// IsCIDRFamily reports whether fam uses the simple [prefix-len][addr]
// NLRI wire format that BART can key on: IPv4/IPv6 unicast, multicast and
// labeled unicast. Labeled unicast qualifies because the ingest path strips
// the label stack before the RIB sees the key (rib.insertLabeled), so what
// FamilyRIB stores is a bare prefix.
//
// It is exported because the BGP plugin's best-prev store must partition
// families exactly as the Adj-RIB-In does. A second copy of the predicate
// would let the two disagree, and the route would then be keyed one way on
// insert and looked up another.
func IsCIDRFamily(fam family.Family) bool {
	//exhaustive:ignore // Only plain or label-stripped prefix SAFIs can use CIDR storage.
	switch fam.SAFI {
	case family.SAFIUnicast, family.SAFIMulticast, family.SAFIMPLSLabel:
	default:
		return false
	}
	return fam.AFI == family.AFIIPv4 || fam.AFI == family.AFIIPv6
}

// parseNLRIKey splits CIDR wire NLRI bytes into (pathID, prefix). Under
// ADD-PATH the first 4 bytes carry a path-id (RFC 7911); otherwise the
// whole slice is prefix-len + address bytes. Returns ok=false when the
// bytes are malformed or the family does not map to a netip.Prefix -- in
// particular, this returns false for non-CIDR families, which should use
// the opaque map path instead.
func (r *FamilyRIB) parseNLRIKey(nlriBytes []byte) (uint32, netip.Prefix, bool) {
	if r.addPath {
		if len(nlriBytes) < 4 {
			return 0, netip.Prefix{}, false
		}
		pathID := uint32(nlriBytes[0])<<24 |
			uint32(nlriBytes[1])<<16 |
			uint32(nlriBytes[2])<<8 |
			uint32(nlriBytes[3])
		pfx, ok := store.NLRIToPrefix(r.fam, nlriBytes[4:])
		return pathID, pfx, ok
	}
	pfx, ok := store.NLRIToPrefix(r.fam, nlriBytes)
	return 0, pfx, ok
}

// buildNLRIBytes reconstructs wire NLRI bytes for (pathID, pfx) into buf.
// CIDR-only helper; non-CIDR families iterate their opaque map keys
// directly. Under ADD-PATH the first 4 bytes are the path-id. buf must be
// at least 21 bytes (4 path-id + 1 prefix-len + 16 IPv6). Returns nil if
// buf is too small or pfx is an invalid zero-value.
func (r *FamilyRIB) buildNLRIBytes(pathID uint32, pfx netip.Prefix, buf []byte) []byte {
	if !r.addPath {
		return store.PrefixToNLRIInto(pfx, buf)
	}
	if len(buf) < 4 {
		return nil
	}
	buf[0] = byte(pathID >> 24)
	buf[1] = byte(pathID >> 16)
	buf[2] = byte(pathID >> 8)
	buf[3] = byte(pathID)
	tail := store.PrefixToNLRIInto(pfx, buf[4:])
	if tail == nil {
		return nil
	}
	return buf[:4+len(tail)]
}

// Insert adds a route with its attributes to the RIB. Parses attributes into
// per-type pools for fine-grained deduplication. If the (prefix, path-id) or
// full NLRI bytes already exist, performs implicit withdraw (releases the
// old entry) unless the new attributes are bit-identical, in which case the
// new handles are released and the old entry is retained with its stale
// flag cleared.
// Fast path: when the existing route has a fingerprint matching the raw
// attribute bytes, ParseAttributes is skipped entirely. This eliminates
// 95%+ of allocation in no-op re-announcement scenarios (route refresh,
// peer churn replay, duplicate UPDATE storms).
func (r *FamilyRIB) Insert(attrBytes, nlriBytes []byte) {
	fp := attrFingerprint(attrBytes)
	attrLen := uint32(len(attrBytes))

	if !r.cidr {
		pathID, key, route, ok := r.opaqueRouteKey(nlriBytes, false)
		if !ok {
			return
		}
		if r.insertOpaqueNoOp(pathID, key, route, fp, attrLen, 0) {
			return
		}
		newEntry, err := ParseAttributes(attrBytes)
		if err != nil {
			return
		}
		newEntry.AttrFingerprint = fp
		newEntry.AttrLen = attrLen
		r.insertOpaque(pathID, key, route, newEntry)
		return
	}

	pathID, pfx, ok := r.parseNLRIKey(nlriBytes)
	if !ok {
		return
	}

	if r.addPath {
		if r.insertMultiNoOp(pfx, pathID, fp, attrLen, 0) {
			return
		}
		newEntry, err := ParseAttributes(attrBytes)
		if err != nil {
			return
		}
		newEntry.AttrFingerprint = fp
		newEntry.AttrLen = attrLen
		r.insertMulti(pfx, pathID, newEntry)
		return
	}

	if oldEntry, exists := r.direct.Lookup(pfx); exists {
		if oldEntry.AttrFingerprint != 0 && oldEntry.AttrFingerprint == fp && oldEntry.AttrLen == attrLen {
			if oldEntry.StaleLevel != StaleLevelFresh {
				oldEntry.StaleLevel = StaleLevelFresh
				r.direct.Insert(pfx, oldEntry)
			}
			return
		}
	}

	newEntry, err := ParseAttributes(attrBytes)
	if err != nil {
		return
	}
	newEntry.AttrFingerprint = fp
	newEntry.AttrLen = attrLen

	if oldEntry, exists := r.direct.Lookup(pfx); exists {
		if entriesEqual(oldEntry, newEntry) {
			if oldEntry.StaleLevel != StaleLevelFresh {
				oldEntry.StaleLevel = StaleLevelFresh
				r.direct.Insert(pfx, oldEntry)
			}
			newEntry.Release()
			return
		}
		oldEntry.Release()
	}
	r.direct.Insert(pfx, newEntry)
}

// InsertEntry adds a route using a caller-owned pre-parsed RouteEntry.
// The caller parsed attributes once via ParseRouteEntry and passes the same
// entry, fingerprint, and attribute length for each NLRI in the UPDATE.
// InsertEntry calls AddRef before storing so the caller can Release its
// copy after all inserts.
func (r *FamilyRIB) InsertEntry(nlriBytes []byte, entry RouteEntry, fp uint64, attrLen uint32) {
	if !r.cidr {
		pathID, key, route, ok := r.opaqueRouteKey(nlriBytes, false)
		if !ok {
			return
		}
		if r.insertOpaqueNoOp(pathID, key, route, fp, attrLen, entry.MsgID) {
			return
		}
		clone := entry
		if err := clone.AddRef(); err != nil {
			return
		}
		r.insertOpaque(pathID, key, route, clone)
		return
	}

	pathID, pfx, ok := r.parseNLRIKey(nlriBytes)
	if !ok {
		return
	}

	if r.addPath {
		if r.insertMultiNoOp(pfx, pathID, fp, attrLen, entry.MsgID) {
			return
		}
		clone := entry
		if err := clone.AddRef(); err != nil {
			return
		}
		r.insertMulti(pfx, pathID, clone)
		return
	}

	if oldEntry, exists := r.direct.Lookup(pfx); exists {
		if oldEntry.AttrFingerprint != 0 && oldEntry.AttrFingerprint == fp && oldEntry.AttrLen == attrLen {
			if oldEntry.StaleLevel != StaleLevelFresh || oldEntry.MsgID != entry.MsgID {
				oldEntry.StaleLevel = StaleLevelFresh
				oldEntry.MsgID = entry.MsgID
				r.direct.Insert(pfx, oldEntry)
			}
			return
		}
	}

	clone := entry
	if err := clone.AddRef(); err != nil {
		return
	}

	if oldEntry, exists := r.direct.Lookup(pfx); exists {
		if entriesEqual(oldEntry, clone) {
			if oldEntry.StaleLevel != StaleLevelFresh || oldEntry.MsgID != clone.MsgID {
				oldEntry.StaleLevel = StaleLevelFresh
				oldEntry.MsgID = clone.MsgID
				r.direct.Insert(pfx, oldEntry)
			}
			clone.Release()
			return
		}
		oldEntry.Release()
	}
	r.direct.Insert(pfx, clone)
}

// insertMulti upserts newEntry at pathID within the pathSet for pfx.
// Applies the same "equal-attributes retains old" short-circuit as the
// direct path.
func (r *FamilyRIB) insertMulti(pfx netip.Prefix, pathID uint32, newEntry RouteEntry) {
	if ps, exists := r.multi.Lookup(pfx); exists {
		if oldEntry, have := ps.lookup(pathID); have && entriesEqual(oldEntry, newEntry) {
			ps.refresh(pathID, newEntry.MsgID)
			r.multi.Insert(pfx, ps)
			newEntry.Release()
			return
		}
		replaced, ok := ps.upsert(pathID, newEntry)
		r.multi.Insert(pfx, ps)
		if ok {
			replaced.Release()
		}
		return
	}
	var ps pathSet
	ps.upsert(pathID, newEntry)
	r.multi.Insert(pfx, ps)
}

// Remove removes the path an NLRI in announcement framing names, the form a
// walk hands back. Returns true if the path existed.
func (r *FamilyRIB) Remove(nlriBytes []byte) bool {
	if !r.cidr {
		return r.removeOpaque(nlriBytes, false)
	}
	return r.removeCIDR(nlriBytes)
}

// Withdraw removes the path a received withdrawal names. Returns true if the
// path existed. It differs from Remove for a family whose withdrawal frames
// the route differently: RFC 8277 Section 2.4 puts one Compatibility field
// where the announcement carried its label stack, so a labeled unicast
// withdrawal reaches its prefix through LabeledWithdrawnPrefix and an opaque
// route through its route key. The quoted rule, RFC 8277 Section 2.4: "Upon
// reception, the value of the Compatibility field MUST be ignored." holds for
// both.
func (r *FamilyRIB) Withdraw(nlriBytes []byte) bool {
	if !r.cidr {
		return r.removeOpaque(nlriBytes, true)
	}
	if !r.labeled {
		return r.removeCIDR(nlriBytes)
	}
	var buf [4 + nlrisplit.PrefixKeyScratchSize]byte
	prefixNLRI, ok := LabeledWithdrawnPrefix(r.fam, nlriBytes, r.addPath, buf[:0])
	if !ok {
		return false
	}
	return r.removeCIDR(prefixNLRI)
}

// LabeledWithdrawnPrefix appends to dst the NLRI a labeled unicast (SAFI 4)
// route is stored under, read from one NLRI of a received withdrawal:
// [path-id(4)?][prefix length][prefix], the path identifier kept when addPath
// is set. nlri MUST come from withdrawal framing (nlrisplit.SplitWithdrawn).
// Returns false for an NLRI too short for its framing.
//
// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field
// MUST be ignored." The three octets after the Length are skipped whatever
// they hold, so 0x800000, with its S bit clear, reaches the route as any
// label value does.
func LabeledWithdrawnPrefix(fam family.Family, nlri []byte, addPath bool, dst []byte) ([]byte, bool) {
	route := nlri
	if addPath {
		if len(nlri) < 4 {
			return dst, false
		}
		dst = append(dst, nlri[:4]...)
		route = nlri[4:]
	}
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	key, err := nlrisplit.GetPrefixKey(fam)(route, scratch[:], true)
	if err != nil {
		return dst, false
	}
	return append(dst, key...), true
}

// removeCIDR removes the path of a CIDR prefix an NLRI names.
func (r *FamilyRIB) removeCIDR(nlriBytes []byte) bool {

	pathID, pfx, ok := r.parseNLRIKey(nlriBytes)
	if !ok {
		return false
	}

	if !r.addPath {
		entry, exists := r.direct.Lookup(pfx)
		if !exists {
			return false
		}
		entry.Release()
		r.removePrefixLabels(pfx)
		return r.direct.Delete(pfx)
	}

	ps, exists := r.multi.Lookup(pfx)
	if !exists {
		return false
	}
	removed, ok := ps.remove(pathID)
	if !ok {
		return false
	}
	removed.Release()
	if ps.len() == 0 {
		r.multi.Delete(pfx)
	} else {
		r.multi.Insert(pfx, ps)
	}
	return true
}

// lookupEntry finds the RouteEntry for an NLRI. Returns (entry, true) if
// found, (zero RouteEntry, false) otherwise. The returned entry is a copy --
// safe for read-only use.
func (r *FamilyRIB) lookupEntry(nlriBytes []byte) (RouteEntry, bool) {
	if !r.cidr {
		return r.lookupOpaque(nlriBytes)
	}
	pathID, pfx, ok := r.parseNLRIKey(nlriBytes)
	if !ok {
		return RouteEntry{}, false
	}
	return r.lookupPrefixPath(pathID, pfx)
}

// lookupPrefixPath is lookupEntry for a CIDR path named by its prefix and path
// identifier, which is ignored without ADD-PATH. A non-CIDR family answers
// false: its routes have no prefix.
func (r *FamilyRIB) lookupPrefixPath(pathID uint32, pfx netip.Prefix) (RouteEntry, bool) {
	if !r.cidr {
		return RouteEntry{}, false
	}
	if !r.addPath {
		return r.direct.Lookup(pfx)
	}
	ps, exists := r.multi.Lookup(pfx)
	if !exists {
		return RouteEntry{}, false
	}
	return ps.lookup(pathID)
}

// PrefixPath is one stored path of a route, a CIDR prefix or a non-CIDR route
// key: the path identifier it was received under and its route. PathID is zero
// for a family stored without ADD-PATH, where the route alone names the path.
//
// Route is the wire NLRI a non-CIDR path was received with, without its path
// identifier: the labels a route key drops live here, so every read of what
// the path carries goes through it. It is empty for a CIDR prefix.
type PrefixPath struct {
	PathID uint32
	// Labels belongs to a retained snapshot returned by PeerRIB's append
	// operations. It is InvalidHandle when the path has no label side-data.
	Labels attrpool.Handle
	Entry  RouteEntry
	Route  string
}

// Release gives back a snapshot returned by AppendPrefixPathsRetained or
// AppendKeyPathsRetained. The caller MUST release each returned path exactly
// once, or transfer both Entry and Labels to another owner.
func (p *PrefixPath) Release() {
	p.Entry.Release()
	if p.Labels.IsValid() {
		_ = pool.Labels.Release(p.Labels)
		p.Labels = attrpool.InvalidHandle
	}
}

// retain pins one snapshot while PeerRIB.mu is held. A failed acquisition
// rolls back its references, matching LookupRetained's failure contract.
func (p *PrefixPath) retain() bool {
	if err := p.Entry.AddRef(); err != nil {
		return false
	}
	if p.Labels.IsValid() {
		if err := pool.Labels.AddRef(p.Labels); err != nil {
			p.Entry.Release()
			return false
		}
	}
	return true
}

// appendPrefixPathsRetained appends retained paths stored for pfx to dst.
// A CIDR family stored without ADD-PATH holds at most one path
// per prefix; under ADD-PATH it holds one per path identifier (RFC 7911
// Section 2). A non-CIDR family appends nothing: its routes have no prefix.
//
// Caller MUST hold PeerRIB.mu. Each appended path owns its handles until
// Release; a reused or stack-backed dst avoids allocating the slice.
func (r *FamilyRIB) appendPrefixPathsRetained(pfx netip.Prefix, dst []PrefixPath) []PrefixPath {
	if !r.cidr {
		return dst
	}
	if !r.addPath {
		if entry, ok := r.direct.Lookup(pfx); ok {
			path := PrefixPath{Entry: entry, Labels: r.LookupLabels(0, pfx)}
			if path.retain() {
				dst = append(dst, path)
			}
		}
		return dst
	}
	ps, ok := r.multi.Lookup(pfx)
	if !ok {
		return dst
	}
	for i := range ps.entries {
		entry := &ps.entries[i]
		path := PrefixPath{PathID: entry.pathID, Entry: entry.entry, Labels: entry.labels}
		if path.retain() {
			dst = append(dst, path)
		}
	}
	return dst
}

// Len returns the total number of routes in the RIB. Under ADD-PATH, routes
// with different path-ids for the same prefix count separately.
func (r *FamilyRIB) Len() int {
	switch {
	case !r.cidr:
		return r.opaqueLen()
	case !r.addPath:
		return r.direct.Len()
	}
	n := 0
	r.multi.Iterate(func(_ netip.Prefix, ps pathSet) bool {
		n += ps.len()
		return true
	})
	return n
}

// IterateEntry calls fn for each route with its NLRI bytes and RouteEntry.
// Under ADD-PATH every (prefix, path-id) pair yields a separate callback.
// For non-CIDR families the nlriBytes passed to fn is the stored route key,
// framed with its path identifier under ADD-PATH, and is valid for the duration
// of that callback only. Callbacks MUST copy if they need to retain it.
func (r *FamilyRIB) IterateEntry(fn func(nlriBytes []byte, entry RouteEntry) bool) {
	r.iterateEntry(fn, false)
}

// iterateEntrySorted is like IterateEntry but CIDR families are visited in
// numerically sorted prefix order. Non-CIDR families have no natural order.
func (r *FamilyRIB) iterateEntrySorted(fn func(nlriBytes []byte, entry RouteEntry) bool) {
	r.iterateEntry(fn, true)
}

func (r *FamilyRIB) iterateEntry(fn func(nlriBytes []byte, entry RouteEntry) bool, sorted bool) {
	if !r.cidr {
		r.iterateOpaque(fn)
		return
	}
	if !r.addPath {
		var buf [21]byte
		iter := r.direct.Iterate
		if sorted {
			iter = r.direct.IterateSorted
		}
		iter(func(pfx netip.Prefix, entry RouteEntry) bool {
			nlri := r.buildNLRIBytes(0, pfx, buf[:])
			if nlri == nil {
				return true
			}
			return fn(nlri, entry)
		})
		return
	}
	var buf [21]byte
	iter := r.multi.Iterate
	if sorted {
		iter = r.multi.IterateSorted
	}
	iter(func(pfx netip.Prefix, ps pathSet) bool {
		for i := range ps.entries {
			nlri := r.buildNLRIBytes(ps.entries[i].pathID, pfx, buf[:])
			if nlri == nil {
				continue
			}
			if !fn(nlri, ps.entries[i].entry) {
				return false
			}
		}
		return true
	})
}

// Release frees all RouteEntry handles and clears the RIB.
func (r *FamilyRIB) Release() {
	switch {
	case !r.cidr:
		r.releaseOpaque()
	case !r.addPath:
		r.direct.ModifyAll(func(e *RouteEntry) { e.Release() })
		r.direct.Reset()
	default:
		r.multi.ModifyAll(func(ps *pathSet) { ps.releaseAll() })
		r.multi.Reset()
	}
	if r.labels != nil {
		r.labels.Iterate(func(_ netip.Prefix, h attrpool.Handle) bool {
			if h.IsValid() {
				_ = pool.Labels.Release(h)
			}
			return true
		})
		r.labels.Reset()
	}
}

// modifyEntry calls fn with a pointer to the entry for the given NLRI. fn
// may mutate the entry (e.g., update StaleLevel). Returns false if the NLRI
// does not exist.
func (r *FamilyRIB) modifyEntry(nlriBytes []byte, fn func(entry *RouteEntry)) bool {
	if !r.cidr {
		return r.modifyOpaque(nlriBytes, fn)
	}
	pathID, pfx, ok := r.parseNLRIKey(nlriBytes)
	if !ok {
		return false
	}
	if !r.addPath {
		return r.direct.Modify(pfx, fn)
	}
	return r.multi.Modify(pfx, func(ps *pathSet) {
		ps.modify(pathID, fn)
	})
}

// ModifyAll calls fn with a pointer to each entry. fn may mutate the entry.
func (r *FamilyRIB) ModifyAll(fn func(entry *RouteEntry)) {
	switch {
	case !r.cidr:
		r.modifyAllOpaque(func(_ []byte, entry *RouteEntry) { fn(entry) })
	case !r.addPath:
		r.direct.ModifyAll(fn)
	default:
		r.multi.ModifyAll(func(ps *pathSet) {
			for i := range ps.entries {
				fn(&ps.entries[i].entry)
			}
		})
	}
}

// ModifyAllKeyed calls fn with the NLRI key and a pointer to each entry.
func (r *FamilyRIB) ModifyAllKeyed(fn func(nlriBytes []byte, entry *RouteEntry)) {
	switch {
	case !r.cidr:
		r.modifyAllOpaque(fn)
	case !r.addPath:
		var buf [21]byte
		r.direct.ModifyAllKeyed(func(pfx netip.Prefix, entry *RouteEntry) {
			nlri := r.buildNLRIBytes(0, pfx, buf[:])
			if nlri != nil {
				fn(nlri, entry)
			}
		})
	default:
		var buf [21]byte
		r.multi.ModifyAllKeyed(func(pfx netip.Prefix, ps *pathSet) {
			for i := range ps.entries {
				nlri := r.buildNLRIBytes(ps.entries[i].pathID, pfx, buf[:])
				if nlri != nil {
					fn(nlri, &ps.entries[i].entry)
				}
			}
		})
	}
}

// Family returns the address family of this RIB.
func (r *FamilyRIB) Family() family.Family { return r.fam }

// HasAddPath returns whether ADD-PATH is enabled.
func (r *FamilyRIB) HasAddPath() bool { return r.addPath }

// isLabeled returns whether this is a labeled unicast family (SAFI 4).
func (r *FamilyRIB) isLabeled() bool { return r.labeled }

// SetLabels binds the MPLS label handle h to the path (pathID, pfx), as
// side-data beside its RouteEntry, and releases the handle that path held
// before. Without ADD-PATH, pathID is ignored and the prefix holds one binding.
// Returns false when the family is not labeled or, under ADD-PATH, the path is
// not stored: the caller then still owns h.
func (r *FamilyRIB) SetLabels(pathID uint32, pfx netip.Prefix, h attrpool.Handle) bool {
	if !r.labeled {
		return false
	}
	if r.labels != nil {
		if old, exists := r.labels.Lookup(pfx); exists && old.IsValid() {
			_ = pool.Labels.Release(old)
		}
		r.labels.Insert(pfx, h)
		return true
	}
	if r.multi == nil {
		return false
	}
	bound := false
	r.multi.Modify(pfx, func(ps *pathSet) { bound = ps.setLabels(pathID, h) })
	return bound
}

// LookupLabels returns the label handle bound to the path (pathID, pfx), or
// InvalidHandle. Without ADD-PATH, pathID is ignored.
func (r *FamilyRIB) LookupLabels(pathID uint32, pfx netip.Prefix) attrpool.Handle {
	if !r.labeled {
		return attrpool.InvalidHandle
	}
	if r.labels != nil {
		h, ok := r.labels.Lookup(pfx)
		if !ok {
			return attrpool.InvalidHandle
		}
		return h
	}
	if r.multi == nil {
		return attrpool.InvalidHandle
	}
	ps, exists := r.multi.Lookup(pfx)
	if !exists {
		return attrpool.InvalidHandle
	}
	return ps.lookupLabels(pathID)
}

// removePrefixLabels deletes and releases the one label binding a prefix holds
// without ADD-PATH. Under ADD-PATH r.labels is nil and pathSet releases each
// path's handle with the path.
func (r *FamilyRIB) removePrefixLabels(pfx netip.Prefix) {
	if r.labels == nil {
		return
	}
	if old, exists := r.labels.Lookup(pfx); exists && old.IsValid() {
		_ = pool.Labels.Release(old)
	}
	r.labels.Delete(pfx)
}

// MarkStale sets StaleLevel on all routes in this family.
func (r *FamilyRIB) MarkStale(level uint8) {
	r.ModifyAll(func(entry *RouteEntry) { entry.StaleLevel = level })
}

// PurgeStale deletes all routes where StaleLevel > 0, releasing pool handles.
// Returns the number of routes purged.
func (r *FamilyRIB) PurgeStale() int {
	if !r.cidr {
		return r.purgeStaleOpaque()
	}
	if !r.addPath {
		var stalePfx []netip.Prefix
		r.direct.Iterate(func(pfx netip.Prefix, entry RouteEntry) bool {
			if entry.StaleLevel > StaleLevelFresh {
				stalePfx = append(stalePfx, pfx)
			}
			return true
		})
		for _, pfx := range stalePfx {
			if entry, ok := r.direct.Lookup(pfx); ok {
				entry.Release()
				r.direct.Delete(pfx)
				r.removePrefixLabels(pfx)
			}
		}
		return len(stalePfx)
	}
	type staleKey struct {
		pfx    netip.Prefix
		pathID uint32
	}
	var stale []staleKey
	r.multi.Iterate(func(pfx netip.Prefix, ps pathSet) bool {
		for i := range ps.entries {
			if ps.entries[i].entry.StaleLevel > StaleLevelFresh {
				stale = append(stale, staleKey{pfx: pfx, pathID: ps.entries[i].pathID})
			}
		}
		return true
	})
	for _, k := range stale {
		ps, ok := r.multi.Lookup(k.pfx)
		if !ok {
			continue
		}
		removed, ok := ps.remove(k.pathID)
		if !ok {
			continue
		}
		removed.Release()
		if ps.len() == 0 {
			r.multi.Delete(k.pfx)
		} else {
			r.multi.Insert(k.pfx, ps)
		}
	}
	return len(stale)
}

// StaleCount returns the number of routes with StaleLevel > 0.
func (r *FamilyRIB) StaleCount() int {
	count := 0
	if !r.cidr {
		r.iterateOpaque(func(_ []byte, entry RouteEntry) bool {
			if entry.StaleLevel > StaleLevelFresh {
				count++
			}
			return true
		})
		return count
	}
	if !r.addPath {
		r.direct.Iterate(func(_ netip.Prefix, entry RouteEntry) bool {
			if entry.StaleLevel > StaleLevelFresh {
				count++
			}
			return true
		})
		return count
	}
	r.multi.Iterate(func(_ netip.Prefix, ps pathSet) bool {
		for i := range ps.entries {
			if ps.entries[i].entry.StaleLevel > StaleLevelFresh {
				count++
			}
		}
		return true
	})
	return count
}

// entriesEqual checks if two RouteEntries have the same attribute handles.
// Used for no-op detection (same NLRI + same attrs = skip).
func entriesEqual(a, b RouteEntry) bool {
	return a.Bundle == b.Bundle && a.ASPath == b.ASPath
}

// attrFingerprint computes an FNV-1a 64-bit hash of raw attribute bytes. Used
// with AttrLen as a composite guard for fast no-op detection.
// Never returns 0 (FNV offset basis is non-zero). The zero sentinel on
// RouteEntry.AttrFingerprint catches entries inserted before fingerprinting.
//
// The bytes alone identify the entry: every caller hands over attributes that
// are already four-octet, so two identical byte strings can no longer mean two
// different AS paths.
func attrFingerprint(attrBytes []byte) uint64 {
	h := uint64(14695981039346656037) // FNV offset basis
	for _, b := range attrBytes {
		h ^= uint64(b)
		h *= 1099511628211 // FNV prime
	}
	return h
}

// insertMultiNoOp checks if the multi (ADD-PATH) entry exists with a matching
// fingerprint+length. If so, refreshes stale state and received ownership.
func (r *FamilyRIB) insertMultiNoOp(pfx netip.Prefix, pathID uint32, fp uint64, attrLen uint32, messageID uint64) bool {
	if ps, exists := r.multi.Lookup(pfx); exists {
		if oldEntry, have := ps.lookup(pathID); have {
			if oldEntry.AttrFingerprint != 0 && oldEntry.AttrFingerprint == fp && oldEntry.AttrLen == attrLen {
				if oldEntry.StaleLevel != StaleLevelFresh || oldEntry.MsgID != messageID {
					ps.refresh(pathID, messageID)
					r.multi.Insert(pfx, ps)
				}
				return true
			}
		}
	}
	return false
}

// ToWireBytes reconstructs attribute wire bytes from the RouteEntry.
// Returns wire bytes in RFC 4271 format (concatenated attributes with headers).
//
// Attributes are written in type-code order per RFC 4271 Appendix F.3.
// OtherAttrs are merged into the correct position by type code.
//
// Limitation: For individually pooled attributes, flags are normalized to standard
// values (0x40 well-known, 0x80 optional, 0xC0 optional-transitive). The Partial
// flag (0x20) is NOT preserved. OtherAttrs preserve original flags.
// For exact wire reproduction, use msg-id cache forwarding instead.
func (e *RouteEntry) ToWireBytes() ([]byte, error) {
	var result []byte
	b := e.GetBundle()

	otherByType := make(map[uint8][]byte)
	if b.HasOtherAttrs() {
		data, err := pool.OtherAttrs.Get(b.OtherAttrs)
		if err != nil {
			return nil, err
		}
		otherByType = parseOtherAttrs(data)
	}

	writeAttr := func(code attribute.AttributeCode, flags byte, p *attrpool.Pool, h attrpool.Handle) error {
		if h.IsValid() {
			data, err := p.Get(h)
			if err != nil {
				return err
			}
			result = appendAttrWire(result, code, flags, data)
		} else if wire, ok := otherByType[byte(code)]; ok {
			result = append(result, wire...)
			delete(otherByType, byte(code))
		}
		return nil
	}

	if err := writeAttr(attribute.AttrOrigin, 0x40, pool.Origin, b.Origin); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrASPath, 0x40, pool.ASPath, e.ASPath); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrNextHop, 0x40, pool.NextHop, b.NextHop); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrMED, 0x80, pool.MED, b.MED); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrLocalPref, 0x40, pool.LocalPref, b.LocalPref); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrAtomicAggregate, 0x40, pool.AtomicAggregate, b.AtomicAggregate); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrAggregator, 0xC0, pool.Aggregator, b.Aggregator); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrCommunity, 0xC0, pool.Communities, b.Communities); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrOriginatorID, 0x80, pool.OriginatorID, b.OriginatorID); err != nil {
		return nil, err
	}
	if err := writeAttr(attribute.AttrClusterList, 0x80, pool.ClusterList, b.ClusterList); err != nil {
		return nil, err
	}
	// RFC 8955 Section 7: "All Traffic Filtering Actions are specified as
	// transitive BGP Extended Communities." Retained replay must not lose
	// those actions merely because they have their own pooled field.
	if err := writeAttr(attribute.AttrExtCommunity, 0xC0, pool.ExtCommunities, b.ExtCommunities); err != nil {
		return nil, err
	}

	var codes []uint8
	for code := range otherByType {
		codes = append(codes, code)
	}
	sortBytes(codes)
	for _, code := range codes {
		result = append(result, otherByType[code]...)
	}

	return result, nil
}

// parseOtherAttrs parses the OtherAttrs blob into a map by type code.
// Input format: [type(1)][flags(1)][length_16bit][value(n)]...
// Returns map of type_code -> complete wire bytes (flags + type + length + value).
func parseOtherAttrs(data []byte) map[uint8][]byte {
	result := make(map[uint8][]byte)
	off := 0
	for off+4 <= len(data) {
		typeCode := data[off]
		flags := data[off+1]
		length := int(data[off+2])<<8 | int(data[off+3])
		off += 4

		if off+length > len(data) {
			break // Malformed.
		}
		value := data[off : off+length]
		off += length

		// Reconstruct wire format: flags + type + length + value.
		var wire []byte
		if length > 255 {
			wire = append(wire, flags|0x10, typeCode, byte(length>>8), byte(length))
		} else {
			wire = append(wire, flags&^0x10, typeCode, byte(length))
		}
		wire = append(wire, value...)
		result[typeCode] = wire
	}
	return result
}

// sortBytes sorts a byte slice in ascending order.
func sortBytes(b []uint8) {
	for i := 1; i < len(b); i++ {
		for j := i; j > 0 && b[j-1] > b[j]; j-- {
			b[j-1], b[j] = b[j], b[j-1]
		}
	}
}

// appendAttrWire appends an attribute in wire format (header + value).
func appendAttrWire(dst []byte, code attribute.AttributeCode, flags byte, value []byte) []byte {
	if len(value) > 255 {
		// Extended length (2-byte length field).
		flags |= 0x10
		dst = append(dst, flags, byte(code), byte(len(value)>>8), byte(len(value)))
	} else {
		// Normal length (1-byte length field).
		dst = append(dst, flags, byte(code), byte(len(value)))
	}
	return append(dst, value...)
}
