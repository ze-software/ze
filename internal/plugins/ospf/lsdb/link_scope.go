// Design: docs/architecture/ospf/ospf-af-unify.md -- OSPFv3 link-local-scope LSDB (Link-LSA store)
// RFC: rfc/short/rfc5340.md (sec 4.4.3.8 Link-LSA, link-local flooding scope)

package lsdb

import (
	"bytes"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const linkLSALinkLocalRawOff = types.LSAHeaderLen + 4

// LinkLSARef identifies a self-originated OSPFv3 Link-LSA by local interface and key.
type LinkLSARef struct {
	Interface string
	Key       types.LSAKey
}

// isLinkLSAType reports whether t is a link-local-scope LSA that lives in the
// per-interface link store: the OSPFv3 Type-8 Link-LSA (RFC 5340) or the RFC 5250
// Type-9 link-local opaque LSA. Both are bound to the single interface they arrive
// on and are flooded only out that interface (floodLink), never area- or AS-wide.
// RFC 5250 Section 3.1: a Type-9 opaque LSA "is not flooded beyond the local
// (sub)network" -- routing it through installLink/floodLink enforces that bound.
// The OSPFv3 Grace-LSA (RFC 5187 sec 2.1, wire LS Type 0x000B, function code 11,
// link-local scope) is likewise link-scoped; the OSPFv3 codec maps it to the internal
// LSTypeGraceV6 sentinel (0x000B numerically collides with the OSPFv2 Type-11 Opaque-AS,
// so a distinct sentinel keeps the two apart), and it too routes through the link store.
func isLinkLSAType(t types.LSType) bool {
	return t == types.LSTypeLink || t == types.LSTypeOpaqueLink || t == types.LSTypeGraceV6
}

func (d *LSDB) linkForLocked(iface string) *areaDB {
	store := d.links[iface]
	if store == nil {
		store = newAreaDB()
		d.links[iface] = store
	}
	return store
}

func (d *LSDB) linkForReadLocked(iface string) *areaDB { return d.links[iface] }

func (d *LSDB) installLink(iface string, area types.AreaID, lsa packet.LSA, self, enforceMinArrival bool) (installResult, bool) {
	raw, h, ok := normaliseLSA(lsa)
	if !ok || !isLinkLSAType(h.Type) || iface == "" {
		return installResult{Freshness: Older}, false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.installLinkLocked(iface, area, raw, h, self, enforceMinArrival)
}

func (d *LSDB) installLinkLocked(iface string, area types.AreaID, raw []byte, h packet.LSAHeader, self, enforceMinArrival bool) (installResult, bool) {
	now := d.now()
	key := h.Key()
	store := d.linkForLocked(iface)
	d.linkAreas[iface] = area
	existing := store.entries[key]
	if existing != nil {
		fr := CompareHeaders(h, existing.Header(now))
		if fr == Older || fr == Equal {
			return installResult{Freshness: fr, Entry: existing, Previous: existing}, true
		}
		if enforceMinArrival && d.arrivedTooSoonLinkLocked(iface, key, now) {
			return installResult{Freshness: Equal, Entry: existing, Previous: existing}, true
		}
		entry := newEntry(h, raw, now, self)
		store.entries[key] = entry
		d.noteLinkArrivalLocked(iface, key, now)
		return installResult{Freshness: Newer, Stored: true, Entry: entry, Previous: existing}, true
	}
	if len(store.entries) >= MaxLSAsPerArea {
		return installResult{Freshness: Older}, false
	}
	if enforceMinArrival && d.arrivedTooSoonLinkLocked(iface, key, now) {
		return installResult{Freshness: Equal}, true
	}
	entry := newEntry(h, raw, now, self)
	store.entries[key] = entry
	store.rebuildSortedLocked()
	d.noteLinkArrivalLocked(iface, key, now)
	d.publishSizeMetricLocked(area, key.Type)
	return installResult{Freshness: Newer, Stored: true, Entry: entry}, true
}

func (d *LSDB) arrivedTooSoonLinkLocked(iface string, key types.LSAKey, now time.Time) bool {
	arrivals := d.linkArrival[iface]
	if arrivals == nil {
		return false
	}
	last, ok := arrivals[key]
	return ok && now.Sub(last) < d.timers.MinLSArrival
}

func (d *LSDB) noteLinkArrivalLocked(iface string, key types.LSAKey, now time.Time) {
	arrivals := d.linkArrival[iface]
	if arrivals == nil {
		arrivals = make(map[types.LSAKey]time.Time)
		d.linkArrival[iface] = arrivals
	}
	arrivals[key] = now
}

// LookupLink returns the current header for a link-scoped LSA on iface.
func (d *LSDB) LookupLink(iface string, key types.LSAKey) (packet.LSAHeader, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	store := d.linkForReadLocked(iface)
	if store == nil {
		return packet.LSAHeader{}, false
	}
	entry := store.entries[key]
	if entry == nil {
		return packet.LSAHeader{}, false
	}
	return entry.Header(d.now()), true
}

// LookupLinkLSA returns a lazy LSA view backed by an owned raw-byte copy from iface's link store.
func (d *LSDB) LookupLinkLSA(iface string, key types.LSAKey) (packet.LSA, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	store := d.linkForReadLocked(iface)
	if store == nil {
		return packet.LSA{}, false
	}
	entry := store.entries[key]
	if entry == nil {
		return packet.LSA{}, false
	}
	return entry.LSA(d.now())
}

// LinkLSAs returns every link-local-scope LSA stored for iface in stable key order.
func (d *LSDB) LinkLSAs(iface string) []packet.LSA {
	d.mu.RLock()
	defer d.mu.RUnlock()
	store := d.linkForReadLocked(iface)
	if store == nil {
		return nil
	}
	out := make([]packet.LSA, 0, len(store.entries))
	for _, key := range store.sorted {
		lsa, ok := store.entries[key].LSA(d.now())
		if ok {
			out = append(out, lsa)
		}
	}
	return out
}

// ReleaseLink drops all link-scoped state tied to iface, used when the interface is removed.
func (d *LSDB) ReleaseLink(iface string) int {
	if iface == "" {
		return 0
	}
	d.mu.Lock()
	count := 0
	if store := d.links[iface]; store != nil {
		count = len(store.entries)
	}
	delete(d.links, iface)
	delete(d.linkAreas, iface)
	delete(d.linkArrival, iface)
	delete(d.linkOwn, iface)
	delete(d.delayedAck, iface)
	for nbr := range d.retransmit {
		if nbr.Interface == iface {
			delete(d.retransmit, nbr)
		}
	}
	d.mu.Unlock()
	return count
}

// OriginateLinkSelf installs and floods one self-originated OSPFv3 Link-LSA on iface only.
func (d *LSDB) OriginateLinkSelf(iface string, area types.AreaID, key types.LSAKey, body []byte, enc SelfLSAEncoder) (packet.LSAHeader, bool) {
	if enc == nil || iface == "" || key.AdvertisingRouter == (types.RouterID{}) || !isLinkLSAType(key.Type) {
		return packet.LSAHeader{}, false
	}
	if h, same := d.existingLinkSelfBodyUnchanged(iface, key, body); same {
		return h, false
	}
	seq, ok, purge := d.nextLinkOwnSequence(iface, key)
	if !ok {
		h, _ := d.LookupLink(iface, key)
		return h, false
	}
	return d.installLinkOriginated(iface, area, enc(seq, purge), key)
}

func (d *LSDB) existingLinkSelfBodyUnchanged(iface string, key types.LSAKey, body []byte) (packet.LSAHeader, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	store := d.linkForReadLocked(iface)
	if store == nil {
		return packet.LSAHeader{}, false
	}
	entry := store.entries[key]
	if entry == nil || !entry.self || entry.purged || len(entry.raw) < types.LSAHeaderLen {
		return packet.LSAHeader{}, false
	}
	if !bytes.Equal(entry.raw[types.LSAHeaderLen:], body) {
		return packet.LSAHeader{}, false
	}
	return entry.Header(d.now()), true
}

// nextLinkOwnSequence returns the next sequence number for a self-originated Link-LSA, whether
// to (re)originate, and whether this is a MaxAge purge. It mirrors the area path
// (nextOwnSequence): at MaxSequenceNumber it MaxAge-flushes the instance (purge) instead of
// re-originating at the max, so the Link-LSA can later restart from InitialSequenceNumber
// (RFC 2328 sec 12.1.6). The link path previously returned no purge flag and stuck at the max.
func (d *LSDB) nextLinkOwnSequence(iface string, key types.LSAKey) (types.LSSequenceNumber, bool, bool) {
	now := d.now()
	d.mu.Lock()
	defer d.mu.Unlock()
	own := d.linkOwn[iface]
	if own == nil {
		own = make(map[types.LSAKey]ownRecord)
		d.linkOwn[iface] = own
	}
	rec, exists := own[key]
	if exists && !rec.last.IsZero() && now.Sub(rec.last) < d.timers.MinLSInterval {
		return rec.sequence, false, false
	}
	switch {
	case !exists || rec.sequence == 0:
		rec.sequence = types.InitialSequenceNumber
	case rec.sequence.IsMax():
		own[key] = ownRecord{sequence: rec.sequence, last: now}
		return rec.sequence, true, true
	default:
		rec.sequence = rec.sequence.Next()
	}
	rec.last = now
	own[key] = rec
	return rec.sequence, true, false
}

func (d *LSDB) installLinkOriginated(iface string, area types.AreaID, lsa packet.LSA, key types.LSAKey) (packet.LSAHeader, bool) {
	raw, hdr, ok := normaliseLSA(lsa)
	if !ok || !isLinkLSAType(hdr.Type) {
		return packet.LSAHeader{}, false
	}
	d.mu.Lock()
	res, ok := d.installLinkLocked(iface, area, raw, hdr, true, false)
	if !ok || res.Entry == nil {
		d.mu.Unlock()
		return packet.LSAHeader{}, false
	}
	h := res.Entry.Header(d.now())
	d.mu.Unlock()
	d.mOriginations.With(key.Type.String()).Inc()
	d.floodLink(iface, area, key)
	return h, true
}

func (d *LSDB) floodLink(ifaceName string, area types.AreaID, key types.LSAKey) {
	if ifaceName == "" {
		return
	}
	var iface InterfaceInfo
	found := false
	topology := d.topologySnapshot()
	for idx := range topology {
		if topology[idx].Name == ifaceName {
			iface = topology[idx]
			found = true
			break
		}
	}
	if !found {
		return
	}
	var entry *Entry
	d.mu.RLock()
	store := d.linkForReadLocked(ifaceName)
	if store != nil {
		entry = store.entries[key]
	}
	d.mu.RUnlock()
	if entry == nil {
		return
	}
	lsa, ok := entry.LSA(d.now())
	if !ok {
		return
	}
	raw := lsa.RawBytes
	queued := false
	for _, nbr := range iface.Neighbors {
		if !isFloodEligibleNeighborState(nbr.State) || nbr.RouterID == (types.RouterID{}) {
			continue
		}
		// RFC 5250 Section 3.1: Opaque LSAs are flooded only to opaque-capable neighbors
		// (those that set the O-bit in their DD). A Type-9 link-local opaque LSA queued for a
		// non-opaque neighbor would waste its LSDB and provoke acks; the area/AS flood path
		// (floodExcept) applies the same gate.
		if lsa.Header.Type.IsOpaque() && !nbr.OpaqueCapable {
			continue
		}
		if d.queueRetransmit(area, NeighborKey{Interface: iface.Name, RouterID: nbr.RouterID}, lsa.Header, raw) {
			queued = true
		}
	}
	if !queued {
		return
	}
	d.sendLSUpdate(iface.Name, floodDestination(iface), area, []packet.LSA{lsa})
}

func (d *LSDB) deletePurgedLinkIfAcked(iface string, area types.AreaID, key types.LSAKey) {
	d.mu.Lock()
	defer d.mu.Unlock()
	store := d.links[iface]
	if store == nil {
		return
	}
	entry := store.entries[key]
	if entry == nil || !entry.purged {
		return
	}
	for _, lsas := range d.retransmit {
		if rt, ok := lsas[key]; ok && rt.area == area {
			return
		}
	}
	delete(store.entries, key)
	store.rebuildSortedLocked()
	if own := d.linkOwn[iface]; own != nil {
		delete(own, key)
	}
}

func (d *LSDB) sendDirectLinkLSUpdate(iface string, dst netip.Addr, area types.AreaID, entry *Entry, transmitDelay uint16) {
	if !dst.IsValid() {
		return
	}
	lsa, ok := entry.LSA(d.now())
	if !ok {
		return
	}
	lsa.Header.Age = lsa.Header.Age.Add(transmitDelay)
	lsa.Header.Age.WriteTo(lsa.RawBytes, 0)
	d.sendLSUpdate(iface, dst, area, []packet.LSA{lsa})
}

// handleSelfLinkReceived implements the RFC 2328 sec 13.4 fight-back for link-scoped LSAs.
// A neighbor may flood a self-originated Link-LSA that is newer than our own record: a stale
// instance still circulating after this router restarted, or another router using our Router
// ID. We never install the foreign copy; instead we advance our own sequence record past it so
// the next origination re-originates a strictly newer instance that supersedes it across the
// link. (The area path reinstalls a re-stamped copy directly, but that goes through the neutral
// re-encode, which misparses an OSPFv3 Link-LSA -- so the link path reclaims via the next
// origination instead.) Returns true so the caller does not install the received self-LSA.
func (d *LSDB) handleSelfLinkReceived(iface string, lsa packet.LSA) bool {
	// RFC 3623 sec 2: during graceful restart the restarting router keeps (does not flush) its
	// received self-originated link-scope LSAs, including its own pre-restart Grace-LSAs.
	if d.selfFlushSuppressed() {
		return false
	}
	d.mu.RLock()
	self := d.selfRouter
	d.mu.RUnlock()
	if self == (types.RouterID{}) || lsa.Header.AdvertisingRouter != self {
		return false
	}
	key := lsa.Header.Key()
	local, have := d.LookupLink(iface, key)
	if have && CompareHeaders(lsa.Header, local) != Newer {
		return true // our instance is current or newer; ignore the received copy
	}
	d.mu.Lock()
	own := d.linkOwn[iface]
	if own == nil {
		own = make(map[types.LSAKey]ownRecord)
		d.linkOwn[iface] = own
	}
	// Bump our record to the received sequence (clearing last so the next origination is not
	// rate-limited) unless we already hold a strictly newer one, so the next OriginateLinkSelf
	// produces received.Sequence.Next() and reclaims the LSA. A zero record is "no sequence yet"
	// (0 is not a valid signed OSPF sequence, and would falsely compare newer than the negative
	// InitialSequenceNumber), so always bump in that case.
	if rec := own[key]; rec.sequence == 0 || !rec.sequence.NewerThan(lsa.Header.Sequence) {
		own[key] = ownRecord{sequence: lsa.Header.Sequence}
	}
	d.mu.Unlock()
	return true
}

// FlushStaleLinkSelfLSAs removes self Link-LSAs no longer present in the regenerated keep set.
func (d *LSDB) FlushStaleLinkSelfLSAs(router types.RouterID, keep map[LinkLSARef]struct{}) int {
	d.mu.RLock()
	var stale []LinkLSARef
	for iface, own := range d.linkOwn {
		for key := range own {
			if key.AdvertisingRouter != router || !isLinkLSAType(key.Type) {
				continue
			}
			ref := LinkLSARef{Interface: iface, Key: key}
			if _, ok := keep[ref]; !ok {
				stale = append(stale, ref)
			}
		}
	}
	d.mu.RUnlock()
	count := 0
	for _, ref := range stale {
		if d.deleteLinkLSA(ref.Interface, ref.Key) {
			count++
		}
	}
	return count
}

func (d *LSDB) deleteLinkLSA(iface string, key types.LSAKey) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	store := d.links[iface]
	if store == nil || store.entries[key] == nil {
		return false
	}
	delete(store.entries, key)
	store.rebuildSortedLocked()
	if own := d.linkOwn[iface]; own != nil {
		delete(own, key)
	}
	return true
}

func linkLocalFromRaw(raw []byte) (netip.Addr, bool) {
	if len(raw) < linkLSALinkLocalRawOff+16 {
		return netip.Addr{}, false
	}
	var a [16]byte
	copy(a[:], raw[linkLSALinkLocalRawOff:linkLSALinkLocalRawOff+16])
	addr := netip.AddrFrom16(a)
	return addr, addr.IsValid()
}
