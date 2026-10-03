// Design: docs/architecture/plugin/rib-storage-design.md -- RIB storage internals
// RFC: rfc/short/rfc7911.md -- Section 2 and 3, the path identifier held per path
// Related: familyrib.go -- FamilyRIB, which dispatches every non-CIDR family here
// Related: pathset.go -- the per-path value layer the ADD-PATH map holds

package storage

import (
	"encoding/binary"
	"slices"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// opaqueFrameOctetsInitial sizes the buffer an ADD-PATH opaque walk frames each
// key into: a 4-octet path identifier and a route key. VPN, EVPN and MVPN keys
// fit; a longer key (a FlowSpec rule, a BGP-LS descriptor) grows it once.
const opaqueFrameOctetsInitial = 64

// opaqueRouteKey splits one wire NLRI of a non-CIDR family into the path
// identifier its session sent and the route key that names the route. Without
// ADD-PATH the identifier is zero and the whole NLRI is the route. Returns false
// for an ADD-PATH NLRI too short to carry its identifier.
//
// RFC 7911 Section 3: "the assignment of the Path Identifier for a path by a
// BGP speaker is purely a local matter", and Section 2 identifies a path by
// "the combination of the address prefix and the Path Identifier". The
// identifier names a path of the route, so it is held in the value layer and
// never in the map key: two paths of one route on one session, or one route
// from sessions with and without ADD-PATH, are then one map entry.
func (r *FamilyRIB) opaqueRouteKey(nlriBytes []byte) (uint32, string, bool) {
	if !r.addPath {
		return 0, opaqueKey(r.fam, nlriBytes), true
	}
	if len(nlriBytes) < 4 {
		return 0, "", false
	}
	return binary.BigEndian.Uint32(nlriBytes), opaqueKey(r.fam, nlriBytes[4:]), true
}

// opaqueKey returns the map identity of one route key, an NLRI without a path
// identifier. It is the whole key, except that FlowSpec takes the shortest
// length encoding: RFC 8955 Section 4 lets a rule under 240 octets use either
// length field, and both name one rule, so a replacement or a withdrawal in the
// other framing MUST reach the stored route. The identity stays valid wire
// NLRI, because a walk hands it back to callers as NLRI bytes.
func opaqueKey(fam family.Family, key []byte) string {
	if !ribevents.IsFlowSpec(fam) {
		return string(key)
	}
	// FlowSpecKey answers "" for a malformed length. The splitter already
	// refused such an NLRI, so the whole wire form stays its own identity.
	canonical := ribevents.FlowSpecKey(key)
	if canonical == "" {
		return string(key)
	}
	return canonical
}

// appendFramedKey appends to dst the NLRI an ADD-PATH session names one path
// by: the path identifier, then the route key.
func appendFramedKey(dst []byte, pathID uint32, key string) []byte {
	dst = binary.BigEndian.AppendUint32(dst, pathID)
	return append(dst, key...)
}

// insertOpaqueNoOp checks whether the path (pathID, key) is stored with a
// matching fingerprint and length. If so, it refreshes stale state and
// received ownership and reports true: the UPDATE re-announced the same route.
func (r *FamilyRIB) insertOpaqueNoOp(pathID uint32, key string, fp uint64, attrLen uint32, messageID uint64) bool {
	if r.addPath {
		ps, exists := r.opaqueMulti[key]
		if !exists {
			return false
		}
		oldEntry, have := ps.lookup(pathID)
		if !have || !fingerprintMatches(oldEntry, fp, attrLen) {
			return false
		}
		ps.refresh(pathID, messageID)
		return true
	}
	oldEntry, exists := r.opaque[key]
	if !exists || !fingerprintMatches(oldEntry, fp, attrLen) {
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

// insertOpaque upserts newEntry as the path (pathID, key). Equal attributes
// keep the stored entry and release the new one, the same short-circuit the
// CIDR backends apply.
func (r *FamilyRIB) insertOpaque(pathID uint32, key string, newEntry RouteEntry) {
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

// removeOpaque withdraws one path of a non-CIDR route. Returns true when the
// path existed.
func (r *FamilyRIB) removeOpaque(nlriBytes []byte) bool {
	pathID, key, ok := r.opaqueRouteKey(nlriBytes)
	if !ok {
		return false
	}
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
	pathID, key, ok := r.opaqueRouteKey(nlriBytes)
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

// appendKeyPaths appends every path stored for the route key to dst and
// returns the extended slice. key carries no path identifier, whatever framing
// the session that triggered the lookup uses. Without ADD-PATH the route holds
// at most one path, under path identifier zero. A CIDR family appends nothing:
// its routes are asked by prefix (appendPrefixPaths).
//
// The entries are copies whose pool handles are NOT retained, lookupEntry's
// contract.
func (r *FamilyRIB) appendKeyPaths(key []byte, dst []PrefixPath) []PrefixPath {
	if r.cidr {
		return dst
	}
	identity := opaqueKey(r.fam, key)
	if !r.addPath {
		if entry, ok := r.opaque[identity]; ok {
			dst = append(dst, PrefixPath{Entry: entry})
		}
		return dst
	}
	ps, ok := r.opaqueMulti[identity]
	if !ok {
		return dst
	}
	for i := range ps.entries {
		dst = append(dst, PrefixPath{PathID: ps.entries[i].pathID, Entry: ps.entries[i].entry})
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
// entry, and writes the entry back. Under ADD-PATH the NLRI is framed with the
// path identifier, the form the session sent it in. The NLRI is valid for the
// duration of that callback only. fn MUST NOT add or remove routes.
func (r *FamilyRIB) modifyAllOpaque(fn func(nlriBytes []byte, entry *RouteEntry)) {
	if !r.addPath {
		for key, entry := range r.opaque {
			fn([]byte(key), &entry)
			r.opaque[key] = entry
		}
		return
	}
	frame := make([]byte, 0, opaqueFrameOctetsInitial)
	for key, ps := range r.opaqueMulti {
		for i := range ps.entries {
			frame = appendFramedKey(frame[:0], ps.entries[i].pathID, key)
			fn(frame, &ps.entries[i].entry)
		}
	}
}

// releaseOpaque releases every non-CIDR path and empties both maps.
func (r *FamilyRIB) releaseOpaque() {
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
	pathID, key, ok := r.opaqueRouteKey(nlriBytes)
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
			if !fn([]byte(key), entry) {
				return
			}
		}
		return
	}
	frame := make([]byte, 0, opaqueFrameOctetsInitial)
	for key, ps := range r.opaqueMulti {
		for i := range ps.entries {
			frame = appendFramedKey(frame[:0], ps.entries[i].pathID, key)
			if !fn(frame, ps.entries[i].entry) {
				return
			}
		}
	}
}
