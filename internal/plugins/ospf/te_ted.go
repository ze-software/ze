// Design: plan/learned/1030-ospf-ext-2-traffic-engineering.md -- the Traffic Engineering Database.
// RFC: rfc/short/rfc3630.md sec 1 (the TED is passive; no SPF), rfc/short/rfc5250.md sec 5
// (Type-11 originator-reachability gate).
//
// The TED is a passive, link-keyed store of the TE topology learned from received TE LSAs
// (RFC 3630 Opaque type 1 and RFC 5392 Opaque type 6). It never triggers SPF and installs
// no routes. It is fed by the TE consumer's OnReceive and read by `show ospf te-database`,
// the TE metrics, and a future rsvpte admission consumer through the value-typed Snapshot
// and LookupLink (no cross-boundary pointers, ai/rules/plugins.md). Each entry keys
// on the LSA instance (advertising router + Opaque type + Opaque ID) so a withdraw removes
// exactly the right link; the decoded link also carries its Link ID + local address for
// the rsvpte lookup.

package ospf

import (
	"sort"
	"sync"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// defaultTEDMax bounds the TED so a flood of distinct TE LSAs cannot grow it without limit
// (R-10). It mirrors the order of magnitude of the opaque LSA store; a TE LSA evicted here
// (or withdrawn) removes its TED entry.
const defaultTEDMax = 65536

// tedKey identifies one TE LSA instance: its advertising router plus the Opaque type/ID.
// RFC 3630 sec 2.4.2 carries one Link TLV per LSA, so this also identifies one TE link.
type tedKey struct {
	adv        types.RouterID
	opaqueType uint8
	opaqueID   uint32
}

// tedLink is one stored TE link entry.
type tedLink struct {
	adv        types.RouterID
	area       types.AreaID
	scope      OpaqueScope
	opaqueType uint8
	opaqueID   uint32
	reachable  bool // last delivered RFC 5250 sec 5 flag (fallback when no live seam)
	seq        uint64
	link       packet.TELink
}

// tedRA is one stored Router-Address instance: the advertised address plus the insertion
// sequence, used to evict the oldest when the Router-Address table reaches its bound (R-10).
type tedRA struct {
	addr [4]byte
	seq  uint64
}

// TEDRouterAddress is one advertising router's RFC 3630 sec 2.4.1 Router Address.
type TEDRouterAddress struct {
	Router  types.RouterID `json:"router"`
	Address [4]byte        `json:"address"`
}

// TEDLink is a read-only, value-typed view of one TED link entry for a consumer (the
// show handler or the future rsvpte admission consumer). It carries the LSA identity, the
// flooding scope, the RFC 5250 sec 5 usability, and a copy of the decoded link attributes.
type TEDLink struct {
	AdvertisingRouter types.RouterID
	Area              types.AreaID
	Scope             OpaqueScope
	OpaqueType        uint8
	OpaqueID          uint32
	Usable            bool
	Link              packet.TELink
}

// tedSnapshot is a consistent, value-typed view of the whole TED.
type tedSnapshot struct {
	RouterAddresses []TEDRouterAddress
	Links           []TEDLink
}

// ted is the Traffic Engineering Database. All methods are safe for concurrent use: the
// reception path writes, the show/metrics path reads.
type ted struct {
	mu        sync.Mutex
	links     map[tedKey]*tedLink
	routerRA  map[tedKey]tedRA           // Router-Address instances by LSA key
	routerAdr map[types.RouterID][4]byte // effective Router Address per router (derived)
	max       int
	seq       uint64
	reachable func(types.RouterID) bool // RFC 5250 sec 5 live seam; nil -> use stored flag
}

func newTED() *ted {
	return &ted{
		links:     make(map[tedKey]*tedLink),
		routerRA:  make(map[tedKey]tedRA),
		routerAdr: make(map[types.RouterID][4]byte),
		max:       defaultTEDMax,
	}
}

// setReachable installs the RFC 5250 sec 5 reachability seam (production: SPF reachability).
func (d *ted) setReachable(fn func(types.RouterID) bool) {
	d.mu.Lock()
	d.reachable = fn
	d.mu.Unlock()
}

// setMax bounds the number of stored link entries (R-10).
func (d *ted) setMax(n int) {
	d.mu.Lock()
	if n > 0 {
		d.max = n
	}
	d.mu.Unlock()
}

// applyLSA upserts a decoded TE LSA into the TED. A Router-Address TLV updates the
// originator's Router Address; a Link TLV upserts a link entry. reachable is the RFC 5250
// sec 5 flag delivered by the carrier (meaningful for Type-11 only). It never triggers SPF.
func (d *ted) applyLSA(adv types.RouterID, area types.AreaID, scope OpaqueScope, opaqueType uint8, opaqueID uint32, lsa packet.TELSA, reachable bool) {
	key := tedKey{adv: adv, opaqueType: opaqueType, opaqueID: opaqueID}
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case lsa.IsRouterAddress:
		// Bound the Router-Address table the same way as the link table (R-10): a flood of
		// distinct advertising routers cannot grow it without limit; evict the oldest instance.
		if _, exists := d.routerRA[key]; !exists && len(d.routerRA) >= d.max {
			d.evictOldestRALocked()
		}
		d.seq++
		d.routerRA[key] = tedRA{addr: lsa.RouterAddress, seq: d.seq}
		d.routerAdr[adv] = lsa.RouterAddress
	case lsa.IsLink:
		if _, exists := d.links[key]; !exists && len(d.links) >= d.max {
			d.evictOldestLocked()
		}
		d.seq++
		d.links[key] = &tedLink{
			adv: adv, area: area, scope: scope, opaqueType: opaqueType, opaqueID: opaqueID,
			reachable: reachable, seq: d.seq, link: copyTELink(lsa.Link),
		}
	}
}

// withdraw removes the TED entry for one LSA instance on a MaxAge/withdraw (RFC 2328
// sec 14) or an eviction from the opaque store (R-10).
func (d *ted) withdraw(adv types.RouterID, opaqueType uint8, opaqueID uint32) {
	key := tedKey{adv: adv, opaqueType: opaqueType, opaqueID: opaqueID}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.routerRA[key]; ok {
		delete(d.routerRA, key)
		d.recomputeRouterAddrLocked(adv)
		return
	}
	delete(d.links, key)
}

// recomputeRouterAddrLocked rebuilds the effective Router Address for adv after a
// Router-Address instance was withdrawn or evicted.
func (d *ted) recomputeRouterAddrLocked(adv types.RouterID) {
	for k, v := range d.routerRA {
		if k.adv == adv {
			d.routerAdr[adv] = v.addr
			return
		}
	}
	delete(d.routerAdr, adv)
}

// evictOldestLocked removes the lowest-sequence (oldest inserted) link entry so a bounded
// TED can accept a newer one (R-10).
func (d *ted) evictOldestLocked() {
	var (
		oldestKey tedKey
		oldestSeq uint64
		found     bool
	)
	for k, e := range d.links {
		if !found || e.seq < oldestSeq {
			oldestKey, oldestSeq, found = k, e.seq, true
		}
	}
	if found {
		delete(d.links, oldestKey)
	}
}

// evictOldestRALocked removes the lowest-sequence (oldest inserted) Router-Address entry so a
// bounded TED can accept a new one (R-10), then recomputes the affected router's effective
// address (a later instance for that router, if any, becomes effective).
func (d *ted) evictOldestRALocked() {
	var (
		oldestKey tedKey
		oldestSeq uint64
		found     bool
	)
	for k, e := range d.routerRA {
		if !found || e.seq < oldestSeq {
			oldestKey, oldestSeq, found = k, e.seq, true
		}
	}
	if found {
		delete(d.routerRA, oldestKey)
		d.recomputeRouterAddrLocked(oldestKey.adv)
	}
}

// usableLocked applies the RFC 5250 sec 5 gate: Type-9/10 entries are always usable;
// a Type-11 (AS-wide) entry is usable only when its originator is reachable.
func (d *ted) usableLocked(e *tedLink) bool {
	if e.scope != OpaqueScopeAS {
		return true
	}
	if d.reachable != nil {
		return d.reachable(e.adv)
	}
	return e.reachable
}

// Snapshot returns a consistent, value-typed view of the TED. Slices are freshly built and
// the decoded link is deep-copied, so a caller may mutate the result freely (AC-20).
func (d *ted) Snapshot() tedSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := tedSnapshot{
		RouterAddresses: make([]TEDRouterAddress, 0, len(d.routerAdr)),
		Links:           make([]TEDLink, 0, len(d.links)),
	}
	for r, a := range d.routerAdr {
		out.RouterAddresses = append(out.RouterAddresses, TEDRouterAddress{Router: r, Address: a})
	}
	for _, e := range d.links {
		out.Links = append(out.Links, TEDLink{
			AdvertisingRouter: e.adv, Area: e.area, Scope: e.scope,
			OpaqueType: e.opaqueType, OpaqueID: e.opaqueID,
			Usable: d.usableLocked(e), Link: copyTELink(e.link),
		})
	}
	sort.Slice(out.RouterAddresses, func(i, j int) bool {
		return lessRouterID(out.RouterAddresses[i].Router, out.RouterAddresses[j].Router)
	})
	sort.Slice(out.Links, func(i, j int) bool {
		if out.Links[i].AdvertisingRouter != out.Links[j].AdvertisingRouter {
			return lessRouterID(out.Links[i].AdvertisingRouter, out.Links[j].AdvertisingRouter)
		}
		return out.Links[i].OpaqueID < out.Links[j].OpaqueID
	})
	return out
}

// LookupLink finds a link entry by (advertising router, Link ID, local address), the key
// the future rsvpte admission consumer uses. It returns a value copy (AC-20).
func (d *ted) LookupLink(adv types.RouterID, linkID, localAddr [4]byte) (TEDLink, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, e := range d.links {
		if e.adv != adv || !e.link.HasLinkID || e.link.LinkID != linkID {
			continue
		}
		if len(e.link.LocalIPs) == 0 || e.link.LocalIPs[0] != localAddr {
			continue
		}
		return TEDLink{
			AdvertisingRouter: e.adv, Area: e.area, Scope: e.scope,
			OpaqueType: e.opaqueType, OpaqueID: e.opaqueID,
			Usable: d.usableLocked(e), Link: copyTELink(e.link),
		}, true
	}
	return TEDLink{}, false
}

// linkCountByArea returns the stored link count per area (for the ze_ospf_te_database_links
// gauge labeled by area).
func (d *ted) linkCountByArea() map[types.AreaID]int {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[types.AreaID]int, len(d.links))
	for _, e := range d.links {
		out[e.area]++
	}
	return out
}

// unreachableCount returns the number of Type-11 entries currently held unusable because
// their originator is unreachable (ze_ospf_te_unreachable_originators, RFC 5250 sec 5).
func (d *ted) unreachableCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := 0
	for _, e := range d.links {
		if e.scope == OpaqueScopeAS && !d.usableLocked(e) {
			n++
		}
	}
	return n
}

// copyTELink deep-copies a decoded link so the TED and any snapshot own independent slice
// backing (no cross-boundary aliasing to the received body or between snapshots).
func copyTELink(l packet.TELink) packet.TELink {
	out := l
	if len(l.LocalIPs) > 0 {
		out.LocalIPs = append([][4]byte(nil), l.LocalIPs...)
	}
	if len(l.RemoteIPs) > 0 {
		out.RemoteIPs = append([][4]byte(nil), l.RemoteIPs...)
	}
	return out
}

// lessRouterID orders two Router IDs by their 4-octet big-endian value.
func lessRouterID(a, b types.RouterID) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
