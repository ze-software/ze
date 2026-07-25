// Design: plan/learned/961-ospf-7-lsdb-flooding.md -- LSA aging, refresh, and purge.
// RFC 2328 Section 14: MaxAge LSAs are retained until acknowledged everywhere.

package lsdb

import (
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// Tick ages the database once. The caller supplies now so production can use a
// 1s ticker and tests can advance a fake monotonic clock.
func (d *LSDB) Tick(now time.Time) TickResult {
	var purges []struct {
		area  types.AreaID
		iface string
		key   types.LSAKey
		link  bool
	}
	d.mu.Lock()
	for area, store := range d.areas {
		for key, entry := range store.entries {
			if entry.purged {
				continue
			}
			if entry.Header(now).Age.IsMaxAge() {
				entry.markPurged(now)
				purges = append(purges, struct {
					area  types.AreaID
					iface string
					key   types.LSAKey
					link  bool
				}{area: area, key: key})
				d.mPurges.With(key.Type.String()).Inc()
			}
		}
	}
	for key, entry := range d.asExternal.entries {
		if entry.purged {
			continue
		}
		if entry.Header(now).Age.IsMaxAge() {
			entry.markPurged(now)
			purges = append(purges, struct {
				area  types.AreaID
				iface string
				key   types.LSAKey
				link  bool
			}{area: types.BackboneArea, key: key})
			d.mPurges.With(key.Type.String()).Inc()
		}
	}
	// RFC 5250 §3.1: Type-11 opaque LSAs live in the AS-wide opaque store, keyed under the
	// backbone area like Type 5; age and purge them on the same path.
	for key, entry := range d.asOpaque.entries {
		if entry.purged {
			continue
		}
		if entry.Header(now).Age.IsMaxAge() {
			entry.markPurged(now)
			purges = append(purges, struct {
				area  types.AreaID
				iface string
				key   types.LSAKey
				link  bool
			}{area: types.BackboneArea, key: key})
			d.mPurges.With(key.Type.String()).Inc()
		}
	}
	for iface, store := range d.links {
		area := d.linkAreas[iface]
		for key, entry := range store.entries {
			if entry.purged {
				continue
			}
			if entry.Header(now).Age.IsMaxAge() {
				entry.markPurged(now)
				purges = append(purges, struct {
					area  types.AreaID
					iface string
					key   types.LSAKey
					link  bool
				}{area: area, iface: iface, key: key, link: true})
				d.mPurges.With(key.Type.String()).Inc()
			}
		}
	}
	d.mu.Unlock()
	for _, p := range purges {
		d.notifyChange(p.area)
	}
	for _, p := range purges {
		if p.link {
			d.floodLink(p.iface, p.area, p.key)
			continue
		}
		d.floodExcept("", types.RouterID{}, p.area, p.key)
	}
	for _, p := range purges {
		if p.link {
			d.deletePurgedLinkIfAcked(p.iface, p.area, p.key)
			continue
		}
		d.deletePurgedIfAcked(p.area, p.key)
	}
	return TickResult{Purged: len(purges)}
}

// TickResult reports aging work performed by Tick.
type TickResult struct {
	Purged int
}

// RefreshSelf refreshes self-originated LSAs whose age has reached LSRefreshTime.
// It re-stamps the stored raw bytes in place, preserving OSPFv2 versus OSPFv3
// header encoding, then re-floods the refreshed instance.
func (d *LSDB) RefreshSelf(now time.Time) int {
	type refresh struct {
		area  types.AreaID
		iface string
		key   types.LSAKey
		lsa   packet.LSA
		link  bool
	}
	var todo []refresh
	d.mu.RLock()
	for area, store := range d.areas {
		for key, entry := range store.entries {
			if !entry.self || entry.purged || entry.Header(now).Age.Age() < types.LSRefreshTime {
				continue
			}
			lsa, ok := entry.LSA(now)
			if ok {
				todo = append(todo, refresh{area: area, key: key, lsa: lsa})
			}
		}
	}
	// AS-external (Type 5) LSAs live in the AS-wide store, not in d.areas. Refresh the
	// self-originated ones too -- originated under the backbone area key, matching Tick's
	// purge convention and origination. Without this, every redistributed external, the
	// `default-information originate` default, and every NSSA-translated Type 5 ages to
	// MaxAge and Tick purges it ~LSRefreshTime after the last topology change, blackholing
	// the routes domain-wide even though the redistribution is still active.
	for key, entry := range d.asExternal.entries {
		if !entry.self || entry.purged || entry.Header(now).Age.Age() < types.LSRefreshTime {
			continue
		}
		lsa, ok := entry.LSA(now)
		if ok {
			todo = append(todo, refresh{area: types.BackboneArea, key: key, lsa: lsa})
		}
	}
	// Self-originated Type-11 opaque LSAs live in the AS-wide opaque store; refresh them
	// on the same backbone-keyed path as Type 5 so they do not age to MaxAge and purge.
	for key, entry := range d.asOpaque.entries {
		if !entry.self || entry.purged || entry.Header(now).Age.Age() < types.LSRefreshTime {
			continue
		}
		lsa, ok := entry.LSA(now)
		if ok {
			todo = append(todo, refresh{area: types.BackboneArea, key: key, lsa: lsa})
		}
	}
	for iface, store := range d.links {
		area := d.linkAreas[iface]
		for key, entry := range store.entries {
			if !entry.self || entry.purged || entry.Header(now).Age.Age() < types.LSRefreshTime {
				continue
			}
			lsa, ok := entry.LSA(now)
			if ok {
				todo = append(todo, refresh{area: area, iface: iface, key: key, lsa: lsa, link: true})
			}
		}
	}
	d.mu.RUnlock()
	count := 0
	for idx := range todo {
		item := &todo[idx]
		raw := make([]byte, len(item.lsa.RawBytes))
		copy(raw, item.lsa.RawBytes)
		seq := item.lsa.Header.Sequence.Next()
		cksum, ok := packet.RefreshLSAInPlace(raw, 0, seq)
		if !ok {
			continue
		}
		h := item.lsa.Header
		h.Age = 0
		h.Sequence = seq
		h.Checksum = cksum
		var res installResult
		if item.link {
			res, ok = d.installLink(item.iface, item.area, packet.LSA{Header: h, RawBytes: raw}, true, false)
		} else {
			res, ok = d.install(item.area, packet.LSA{Header: h, RawBytes: raw}, true, false)
		}
		if ok && res.Entry != nil {
			d.mu.Lock()
			if item.link {
				own := d.linkOwn[item.iface]
				if own == nil {
					own = make(map[types.LSAKey]ownRecord)
					d.linkOwn[item.iface] = own
				}
				own[item.key] = ownRecord{sequence: h.Sequence, last: now}
			} else {
				own := d.own[item.area]
				if own == nil {
					own = make(map[types.LSAKey]ownRecord)
					d.own[item.area] = own
				}
				own[item.key] = ownRecord{sequence: h.Sequence, last: now}
			}
			d.mu.Unlock()
			d.mRefreshes.With(item.key.Type.String()).Inc()
			if item.link {
				d.floodLink(item.iface, item.area, item.key)
			} else {
				d.floodExcept("", types.RouterID{}, item.area, item.key)
			}
			d.notifyChange(item.area)
			count++
		}
	}
	return count
}
