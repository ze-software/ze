// Design: plan/learned/1029-ospf-ext-1-opaque-framework.md -- RFC 5250 opaque-LSA carrier.
// RFC: rfc/short/rfc5250.md -- Type 9/10/11 scope, LS-ID split, reception delivery.
//
// This file holds the AS-wide (Type 11) opaque store's origination and the
// scope-agnostic opaque reception-delivery hook and self-origination entry point. The
// LSDB stays consumer-agnostic: it stores, floods, and reflood opaque LSAs by scope and
// hands each newer install to a delivery callback the engine sets; it never interprets
// an opaque body nor consults the consumer registry.

package lsdb

import (
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// OpaqueDelivery is one newer opaque LSA the LSDB hands to the engine after install.
// The engine maps the Opaque Type to a registered consumer, applies the RFC 5250 §5
// Type-11 reachability gate, and invokes the consumer callback. Body is a view valid
// only for the duration of the callback; a consumer that retains it must copy.
type OpaqueDelivery struct {
	Scope             types.LSType
	Area              types.AreaID
	Interface         string
	AdvertisingRouter types.RouterID
	OpaqueType        uint8
	OpaqueID          uint32
	Body              []byte
	// Age is the installed LSA's LS age in seconds (RFC 2328 §12.1.1), surfaced so a consumer
	// that needs the age clock (the RFC 3623 Grace-LSA helper's remaining-grace computation)
	// can read it. Consumers that ignore it are unaffected (it defaults to zero).
	Age uint16
	// Withdrawn is true when the newly installed LSA is a MaxAge purge (RFC 2328 §14): the
	// instance is being removed, not updated. A consumer that keeps derived state (a TED)
	// removes the corresponding entry rather than parsing the body as a fresh advertisement.
	Withdrawn bool
}

// SetOpaqueDelivery wires the engine callback invoked on every newer opaque-LSA install
// (types 9/10/11). It runs after the store + flood, outside the LSDB lock.
func (d *LSDB) SetOpaqueDelivery(fn func(OpaqueDelivery)) {
	d.mu.Lock()
	d.opaqueDelivery = fn
	d.mu.Unlock()
}

// deliverOpaqueOnNewer hands a newly installed opaque LSA to the engine delivery
// callback. It is a no-op for non-opaque LSAs and when no callback is wired.
func (d *LSDB) deliverOpaqueOnNewer(in ReceiveInput, lsa packet.LSA) {
	if !lsa.Header.Type.IsOpaque() {
		return
	}
	d.mu.RLock()
	fn := d.opaqueDelivery
	d.mu.RUnlock()
	if fn == nil {
		return
	}
	fn(OpaqueDelivery{
		Scope:             lsa.Header.Type,
		Area:              in.AreaID,
		Interface:         in.Interface,
		AdvertisingRouter: lsa.Header.AdvertisingRouter,
		OpaqueType:        lsa.OpaqueType(),
		OpaqueID:          lsa.OpaqueID(),
		Body:              lsa.Body,
		Age:               lsa.Header.Age.Age(),
		Withdrawn:         lsa.Header.Age.IsMaxAge(),
	})
}

// OpaqueOriginateInput is one self-originated opaque LSA the framework builds and floods.
type OpaqueOriginateInput struct {
	Router     types.RouterID
	OpaqueType uint8
	OpaqueID   uint32
	// Scope is the opaque LS type: 9 link-local, 10 area, 11 AS-wide (RFC 5250 §3).
	Scope types.LSType
	// Area is the target area for scope 10 and the sequence-bookkeeping key for scope 11
	// (the backbone area, mirroring OriginateExternal); ignored for scope 9.
	Area types.AreaID
	// Interface is the target interface for scope 9 (link-local); ignored otherwise.
	Interface string
	Options   types.Options
	Body      []byte
	// Withdraw MaxAge-flushes a previously originated instance through the purge path.
	Withdraw bool
}

// OriginateOpaque installs and floods one self-originated opaque LSA, reusing the
// self-origination machinery (sequence assignment, MinLSInterval rate-limit, install,
// flood) rather than adding a new path. It builds the LSA header with the LS type from
// the scope and the Link State ID from the Opaque Type + Opaque ID (RFC 5250 §3 /
// App A.2). Withdraw routes through the existing MaxAge purge path so peers withdraw it.
func (d *LSDB) OriginateOpaque(in OpaqueOriginateInput) (packet.LSAHeader, bool) {
	if in.Router == (types.RouterID{}) || !in.Scope.IsOpaque() {
		return packet.LSAHeader{}, false
	}
	lsid := packet.OpaqueLinkStateID(in.OpaqueType, in.OpaqueID)
	key := types.LSAKey{Type: in.Scope, LinkStateID: lsid, AdvertisingRouter: in.Router}

	if in.Scope == types.LSTypeOpaqueLink {
		if in.Interface == "" {
			return packet.LSAHeader{}, false
		}
		if in.Withdraw {
			return d.flushSelfLinkOpaque(in.Interface, key)
		}
		enc := opaqueEncoder(key, in.Options, in.Body)
		return d.OriginateLinkSelf(in.Interface, in.Area, key, in.Body, enc)
	}

	// Type 10 uses the target area; Type 11 uses the backbone area as the fixed
	// sequence key while dbForLocked routes it to the AS-wide opaque store.
	seqArea := in.Area
	if in.Scope == types.LSTypeOpaqueAS {
		seqArea = types.BackboneArea
	}
	if in.Withdraw {
		ok := d.flushSelfLSA(seqArea, key)
		h, _ := d.Lookup(seqArea, key)
		return h, ok
	}
	enc := opaqueEncoder(key, in.Options, in.Body)
	return d.OriginateSelf(seqArea, key, in.Body, enc)
}

// opaqueEncoder returns a SelfLSAEncoder that emits the opaque LSA (header + body) for
// the sequence the LSDB assigns. A purge stamps LS Age = MaxAge.
func opaqueEncoder(key types.LSAKey, opts types.Options, body []byte) SelfLSAEncoder {
	return func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		h := packet.LSAHeader{
			Age:               0,
			Options:           opts,
			Type:              key.Type,
			LinkStateID:       key.LinkStateID,
			AdvertisingRouter: key.AdvertisingRouter,
			Sequence:          seq,
		}
		if purge {
			h.Age = types.LSAge(types.MaxAge)
		}
		return packet.LSA{Header: h, Opaque: &packet.OpaqueLSA{Type: key.Type, Data: body}}
	}
}

// flushSelfLinkOpaque MaxAge-flushes a self-originated Type-9 opaque LSA on iface: it
// re-stamps the stored bytes to MaxAge with the next sequence, floods the purge out the
// link, and deletes the entry once acknowledged (RFC 2328 §14.1, link-scope variant).
func (d *LSDB) flushSelfLinkOpaque(iface string, key types.LSAKey) (packet.LSAHeader, bool) {
	lsa, ok := d.LookupLinkLSA(iface, key)
	if !ok || lsa.Header.Age.IsMaxAge() || len(lsa.RawBytes) == 0 {
		return packet.LSAHeader{}, false
	}
	seq, ok, _ := d.nextLinkOwnSequence(iface, key)
	if !ok {
		return packet.LSAHeader{}, false
	}
	raw := make([]byte, len(lsa.RawBytes))
	copy(raw, lsa.RawBytes)
	cksum, ok := packet.RefreshLSAInPlace(raw, types.LSAge(types.MaxAge), seq)
	if !ok {
		return packet.LSAHeader{}, false
	}
	h := lsa.Header
	h.Age = types.LSAge(types.MaxAge)
	h.Sequence = seq
	h.Checksum = cksum
	rh, ok := d.installLinkOriginated(iface, d.linkAreaOf(iface), packet.LSA{Header: h, RawBytes: raw}, key)
	if ok {
		d.deletePurgedLinkIfAcked(iface, d.linkAreaOf(iface), key)
	}
	return rh, ok
}

// linkAreaOf returns the area recorded for a link store (set at install time).
func (d *LSDB) linkAreaOf(iface string) types.AreaID {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.linkAreas[iface]
}

// OpaqueLSAView is one stored opaque LSA (identity + a copy of its body) for a consumer
// that decodes bodies (e.g. a TE decode of `show ospf database opaque-area`). Body is a
// copy owned by the caller, unlike the reception-hook Body view.
type OpaqueLSAView struct {
	Scope             types.LSType
	Area              types.AreaID
	Interface         string
	AdvertisingRouter types.RouterID
	OpaqueType        uint8
	OpaqueID          uint32
	Age               uint16
	Body              []byte
}

// OpaqueLSAsByType returns every stored opaque LSA whose Opaque Type matches, across the
// per-area (Type 10), AS-wide (Type 11), and per-interface (Type 9) stores, with a copy of
// each body. The carrier interprets no body; this is a generic query a consumer uses to
// decode its own bodies (ai/rules/plugin-self-containment.md).
func (d *LSDB) OpaqueLSAsByType(opaqueType uint8) []OpaqueLSAView {
	d.mu.RLock()
	defer d.mu.RUnlock()
	now := d.now()
	var out []OpaqueLSAView
	collect := func(store *areaDB, area types.AreaID, iface string) {
		for _, key := range store.sorted {
			if !key.Type.IsOpaque() || packet.OpaqueTypeOf(key.LinkStateID) != opaqueType {
				continue
			}
			lsa, ok := store.entries[key].LSA(now)
			if !ok {
				continue
			}
			body := make([]byte, len(lsa.Body))
			copy(body, lsa.Body)
			out = append(out, OpaqueLSAView{
				Scope: key.Type, Area: area, Interface: iface,
				AdvertisingRouter: key.AdvertisingRouter,
				OpaqueType:        packet.OpaqueTypeOf(key.LinkStateID),
				OpaqueID:          packet.OpaqueIDOf(key.LinkStateID),
				Age:               lsa.Header.Age.Age(), Body: body,
			})
		}
	}
	for area, store := range d.areas {
		collect(store, area, "")
	}
	collect(d.asOpaque, types.BackboneArea, "")
	for name, store := range d.links {
		collect(store, d.linkAreas[name], name)
	}
	return out
}

// OpaqueLSACount is one bucket of the current opaque-LSA population, by scope + Opaque
// Type, for the ze_ospf_opaque_lsas gauge.
type OpaqueLSACount struct {
	Scope      types.LSType
	OpaqueType uint8
	Count      int
}

// OpaqueLSACounts returns the current opaque-LSA counts grouped by scope (Type 9/10/11)
// and Opaque Type across every store: Type 9 in the per-interface link stores, Type 10
// in the per-area stores, and Type 11 in the AS-wide opaque store.
func (d *LSDB) OpaqueLSACounts() []OpaqueLSACount {
	d.mu.RLock()
	defer d.mu.RUnlock()
	type bucket struct {
		scope      types.LSType
		opaqueType uint8
	}
	counts := make(map[bucket]int)
	tally := func(store *areaDB) {
		for key := range store.entries {
			if !key.Type.IsOpaque() {
				continue
			}
			counts[bucket{scope: key.Type, opaqueType: packet.OpaqueTypeOf(key.LinkStateID)}]++
		}
	}
	for _, store := range d.areas {
		tally(store)
	}
	tally(d.asOpaque)
	for _, store := range d.links {
		tally(store)
	}
	out := make([]OpaqueLSACount, 0, len(counts))
	for b, n := range counts {
		out = append(out, OpaqueLSACount{Scope: b.scope, OpaqueType: b.opaqueType, Count: n})
	}
	return out
}
