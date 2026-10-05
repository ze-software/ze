// Design: docs/architecture/plugin/rib-storage-design.md -- RIB storage internals
// RFC: rfc/short/rfc7911.md -- Section 2 and 3, the path identifier held per path
// Related: familyrib.go -- FamilyRIB, which dispatches every non-CIDR family here
// Related: pathset.go -- the per-path value layer the ADD-PATH map holds

package storage

import (
	"encoding/binary"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// opaqueFrameOctetsInitial sizes the buffer an ADD-PATH opaque walk frames each
// key into: a 4-octet path identifier and a route key. VPN, EVPN and MVPN keys
// fit; a longer key (a FlowSpec rule, a BGP-LS descriptor) grows it once.
const opaqueFrameOctetsInitial = 64

// opaqueRouteKey splits one wire NLRI of a non-CIDR family into the path
// identifier its session sent, the route key that names the route, and the
// route's own wire NLRI without the identifier. Without ADD-PATH the
// identifier is zero. withdraw says the NLRI came from a withdrawal, whose
// framing RouteKey reads differently. Returns false for an ADD-PATH NLRI too
// short to carry its identifier.
//
// RFC 7911 Section 3: "the assignment of the Path Identifier for a path by a
// BGP speaker is purely a local matter", and Section 2 identifies a path by
// "the combination of the address prefix and the Path Identifier". The
// identifier names a path of the route, so it is held in the value layer and
// never in the map key: two paths of one route on one session, or one route
// from sessions with and without ADD-PATH, are then one map entry.
func (r *FamilyRIB) opaqueRouteKey(nlriBytes []byte, withdraw bool) (uint32, string, []byte, bool) {
	var pathID uint32
	route := nlriBytes
	if r.addPath {
		if len(nlriBytes) < 4 {
			return 0, "", nil, false
		}
		pathID = binary.BigEndian.Uint32(nlriBytes)
		route = nlriBytes[4:]
	}
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	return pathID, string(RouteKey(r.fam, route, scratch[:], withdraw)), route, true
}

// RouteKey returns the identity of one route of a non-CIDR family: its wire
// NLRI without the path identifier, less every field the family's registered
// key operation (nlrisplit.GetPrefixKey) says does not identify the route. A
// VPN route loses its label stack, an EVPN route its labels, ESI and gateway.
// withdraw says route came from a withdrawal, whose label field the key
// operation reads as a Compatibility field. The result aliases route or
// scratch, which MUST hold nlrisplit.PrefixKeyScratchSize octets.
//
// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field
// MUST be ignored." RFC 8277 Section 2.5 binds a new label to the same prefix
// when the label changes, so the label never names a second route.
//
// FlowSpec keeps its own canonical form, which stays valid wire NLRI: RFC 8955
// Section 4 lets a rule under 240 octets use either length field, and both
// name one rule, so a replacement or a withdrawal in the other framing MUST
// reach the stored route. A route the key operation refuses keeps its whole
// wire form as its identity: the splitter already accepted its framing, and no
// field of it can then be shown not to identify it.
func RouteKey(fam family.Family, route, scratch []byte, withdraw bool) []byte {
	if ribevents.IsFlowSpec(fam) {
		// FlowSpecKey answers "" for a malformed length. The splitter already
		// refused such an NLRI, so the whole wire form stays its own identity.
		canonical := ribevents.FlowSpecKey(route)
		if canonical == "" {
			return route
		}
		return []byte(canonical)
	}
	key, err := nlrisplit.GetPrefixKey(fam)(route, scratch, withdraw)
	if err != nil {
		return route
	}
	return key
}

// opaquePath names one stored path of a non-CIDR route: its route key and its
// path identifier, zero without ADD-PATH.
type opaquePath struct {
	key    string
	pathID uint32
}

// routeNLRI returns the wire NLRI the path (pathID, key) was received with,
// without its path identifier. It is the key itself for a route every field of
// which identifies it, which holds no entry in r.wire.
func (r *FamilyRIB) routeNLRI(pathID uint32, key string) string {
	if len(r.wire) == 0 {
		return key
	}
	if route, ok := r.wire[opaquePath{key: key, pathID: pathID}]; ok {
		return route
	}
	return key
}

// setRouteNLRI records the wire NLRI the path (pathID, key) was last received
// with. A route equal to its key needs no record, and any earlier one goes.
//
// RFC 8277 Section 2.5: "If I1 is the same as I2, UPDATE U2 MUST be
// interpreted as meaning that L2 is now bound to P at N1 and that L1 is no
// longer bound to P at N1." The latest UPDATE's labels replace the stored ones.
func (r *FamilyRIB) setRouteNLRI(pathID uint32, key string, route []byte) {
	path := opaquePath{key: key, pathID: pathID}
	if string(route) == key {
		delete(r.wire, path)
		return
	}
	if stored, ok := r.wire[path]; ok && stored == string(route) {
		return
	}
	r.wire[path] = string(route)
}

// appendFramedKey appends to dst the NLRI an ADD-PATH session names one path
// by: the path identifier, then the route.
func appendFramedKey(dst []byte, pathID uint32, route string) []byte {
	dst = binary.BigEndian.AppendUint32(dst, pathID)
	return append(dst, route...)
}

// insertOpaqueNoOp checks whether the path (pathID, key) is stored with a
// matching fingerprint and length and was received as the same wire route. If
// so, it refreshes stale state and received ownership and reports true: the
// UPDATE re-announced the same route. A new label stack under equal attributes
// is a replacement (RFC 8277 Section 2.5), never a no-op.
func (r *FamilyRIB) insertOpaqueNoOp(pathID uint32, key string, route []byte, fp uint64, attrLen uint32, messageID uint64) bool {
	if r.routeNLRI(pathID, key) != string(route) {
		return false
	}
	if r.addPath {
		ps, exists := r.opaqueMulti[key]
		if !exists {
			return false
		}
		oldEntry, have := ps.lookup(pathID)
		if !have {
			return false
		}
		if !fingerprintMatches(oldEntry, fp, attrLen) {
			return false
		}
		ps.refresh(pathID, messageID)
		return true
	}
	oldEntry, exists := r.opaque[key]
	if !exists {
		return false
	}
	if !fingerprintMatches(oldEntry, fp, attrLen) {
		return false
	}
	if oldEntry.StaleLevel != StaleLevelFresh || oldEntry.MsgID != messageID {
		oldEntry.StaleLevel = StaleLevelFresh
		oldEntry.MsgID = messageID
		r.opaque[key] = oldEntry
	}
	return true
}

// fingerprintMatches reports whether entry was stored from attribute bytes with
// fingerprint fp and length attrLen. A zero fingerprint marks an entry stored
// before fingerprinting, which matches nothing.
func fingerprintMatches(entry RouteEntry, fp uint64, attrLen uint32) bool {
	return entry.AttrFingerprint != 0 && entry.AttrFingerprint == fp && entry.AttrLen == attrLen
}

// insertOpaque upserts newEntry as the path (pathID, key), received as the
// wire route. Equal attributes keep the stored entry and release the new one,
// the same short-circuit the CIDR backends apply; the wire route is recorded
// either way, because it carries the path's labels.
func (r *FamilyRIB) insertOpaque(pathID uint32, key string, route []byte, newEntry RouteEntry) {
	r.setRouteNLRI(pathID, key, route)
	if r.addPath {
		ps := r.opaqueMulti[key]
		if oldEntry, have := ps.lookup(pathID); have && entriesEqual(oldEntry, newEntry) {
			ps.refresh(pathID, newEntry.MsgID)
			newEntry.Release()
			return
		}
		replaced, ok := ps.upsert(pathID, newEntry)
		r.opaqueMulti[key] = ps
		if ok {
			replaced.Release()
		}
		return
	}
	if oldEntry, exists := r.opaque[key]; exists {
		if entriesEqual(oldEntry, newEntry) {
			oldEntry.StaleLevel = StaleLevelFresh
			oldEntry.MsgID = newEntry.MsgID
			r.opaque[key] = oldEntry
			newEntry.Release()
			return
		}
		oldEntry.Release()
	}
	r.opaque[key] = newEntry
}

// removeOpaque removes one path of a non-CIDR route. withdraw says nlriBytes
// came from a withdrawal (opaqueRouteKey). Returns true when the path existed.
func (r *FamilyRIB) removeOpaque(nlriBytes []byte, withdraw bool) bool {
	pathID, key, _, ok := r.opaqueRouteKey(nlriBytes, withdraw)
	if !ok {
		return false
	}
	delete(r.wire, opaquePath{key: key, pathID: pathID})
	if !r.addPath {
		entry, exists := r.opaque[key]
		if !exists {
			return false
		}
		entry.Release()
		delete(r.opaque, key)
		return true
	}
	ps, exists := r.opaqueMulti[key]
	if !exists {
		return false
	}
	removed, ok := ps.remove(pathID)
	if !ok {
		return false
	}
	removed.Release()
	if ps.len() == 0 {
		delete(r.opaqueMulti, key)
		return true
	}
	r.opaqueMulti[key] = ps
	return true
}

// lookupOpaque returns a copy of the entry stored for one path of a non-CIDR
// route, with lookupEntry's contract.
func (r *FamilyRIB) lookupOpaque(nlriBytes []byte) (RouteEntry, bool) {
	pathID, key, _, ok := r.opaqueRouteKey(nlriBytes, false)
	if !ok {
		return RouteEntry{}, false
	}
	if !r.addPath {
		e, ok := r.opaque[key]
		return e, ok
	}
	ps, exists := r.opaqueMulti[key]
	if !exists {
		return RouteEntry{}, false
	}
	return ps.lookup(pathID)
}

// appendKeyPathsRetained appends retained paths stored for the route key to
// dst. key is a RouteKey result: no path identifier,
// whatever framing the session that triggered the lookup uses, and no label.
// Each path carries the wire route it was received with. Without ADD-PATH the
// route holds at most one path, under path identifier zero. A CIDR family
// appends nothing: its routes are asked by prefix (appendPrefixPathsRetained).
//
// Caller MUST hold PeerRIB.mu. Each appended path owns its handles until Release.
func (r *FamilyRIB) appendKeyPathsRetained(key []byte, dst []PrefixPath) []PrefixPath {
	if r.cidr {
		return dst
	}
	identity := string(key)
	if !r.addPath {
		if entry, ok := r.opaque[identity]; ok {
			path := PrefixPath{Entry: entry, Route: r.routeNLRI(0, identity), Labels: attrpool.InvalidHandle}
			if path.retain() {
				dst = append(dst, path)
			}
		}
		return dst
	}
	ps, ok := r.opaqueMulti[identity]
	if !ok {
		return dst
	}
	for i := range ps.entries {
		entry := &ps.entries[i]
		path := PrefixPath{
			PathID: entry.pathID, Entry: entry.entry,
			Route: r.routeNLRI(entry.pathID, identity), Labels: attrpool.InvalidHandle,
		}
		if path.retain() {
			dst = append(dst, path)
		}
	}
	return dst
}

// opaqueLen returns the number of non-CIDR paths stored: one per route without
// ADD-PATH, one per path identifier with it.
func (r *FamilyRIB) opaqueLen() int {
	if !r.addPath {
		return len(r.opaque)
	}
	n := 0
	for _, ps := range r.opaqueMulti {
		n += ps.len()
	}
	return n
}

// modifyAllOpaque calls fn with each stored path's NLRI and a pointer to its
// entry, and writes the entry back. The NLRI is the wire route the path was
// received with, labels included; under ADD-PATH it is framed with the path
// identifier, the form the session sent it in. The NLRI is valid for the
// duration of that callback only. fn MUST NOT add or remove routes.
func (r *FamilyRIB) modifyAllOpaque(fn func(nlriBytes []byte, entry *RouteEntry)) {
	if !r.addPath {
		for key, entry := range r.opaque {
			fn([]byte(r.routeNLRI(0, key)), &entry)
			r.opaque[key] = entry
		}
		return
	}
	frame := make([]byte, 0, opaqueFrameOctetsInitial)
	for key, ps := range r.opaqueMulti {
		for i := range ps.entries {
			pathID := ps.entries[i].pathID
			frame = appendFramedKey(frame[:0], pathID, r.routeNLRI(pathID, key))
			fn(frame, &ps.entries[i].entry)
		}
	}
}

// releaseOpaque releases every non-CIDR path and empties every map.
func (r *FamilyRIB) releaseOpaque() {
	clear(r.wire)
	for key, entry := range r.opaque {
		entry.Release()
		delete(r.opaque, key)
	}
	for key, ps := range r.opaqueMulti {
		ps.releaseAll()
		delete(r.opaqueMulti, key)
	}
}

// modifyOpaque calls fn with a pointer to the entry for one path of a
// non-CIDR route. Returns false if the path is not stored.
func (r *FamilyRIB) modifyOpaque(nlriBytes []byte, fn func(entry *RouteEntry)) bool {
	pathID, key, _, ok := r.opaqueRouteKey(nlriBytes, false)
	if !ok {
		return false
	}
	if !r.addPath {
		e, ok := r.opaque[key]
		if !ok {
			return false
		}
		fn(&e)
		r.opaque[key] = e
		return true
	}
	ps, exists := r.opaqueMulti[key]
	if !exists {
		return false
	}
	return ps.modify(pathID, fn)
}

// purgeStaleOpaque deletes every non-CIDR path whose StaleLevel is above
// fresh, releasing its pool handles. Returns the number of paths purged.
// Deleting during a range is defined in Go, so no victim list is needed.
func (r *FamilyRIB) purgeStaleOpaque() int {
	purged := 0
	for key, entry := range r.opaque {
		if entry.StaleLevel > StaleLevelFresh {
			entry.Release()
			delete(r.opaque, key)
			delete(r.wire, opaquePath{key: key})
			purged++
		}
	}
	for key, ps := range r.opaqueMulti {
		// Backward, because remove swaps the last path into the removed slot,
		// and every path behind the cursor has already been judged.
		for _, path := range slices.Backward(ps.entries) {
			if path.entry.StaleLevel <= StaleLevelFresh {
				continue
			}
			removed, ok := ps.remove(path.pathID)
			if ok {
				removed.Release()
				delete(r.wire, opaquePath{key: key, pathID: path.pathID})
				purged++
			}
		}
		if ps.len() == 0 {
			delete(r.opaqueMulti, key)
			continue
		}
		r.opaqueMulti[key] = ps
	}
	return purged
}

// iterateOpaque calls fn with each stored path's NLRI and a copy of its entry,
// framed as modifyAllOpaque frames it, and stops when fn returns false. It
// only reads, so it is safe under the PeerRIB read lock, where
// modifyAllOpaque's write-back is not.
func (r *FamilyRIB) iterateOpaque(fn func(nlriBytes []byte, entry RouteEntry) bool) {
	if !r.addPath {
		for key, entry := range r.opaque {
			if !fn([]byte(r.routeNLRI(0, key)), entry) {
				return
			}
		}
		return
	}
	frame := make([]byte, 0, opaqueFrameOctetsInitial)
	for key, ps := range r.opaqueMulti {
		for i := range ps.entries {
			pathID := ps.entries[i].pathID
			frame = appendFramedKey(frame[:0], pathID, r.routeNLRI(pathID, key))
			if !fn(frame, ps.entries[i].entry) {
				return
			}
		}
	}
}
