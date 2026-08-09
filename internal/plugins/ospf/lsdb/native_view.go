// Design: docs/architecture/ospf/ospf-ext-3-router-information.md -- native-LSA-by-type body query.
// RFC: rfc/short/rfc5340.md (OSPFv3 native LSAs), rfc/short/rfc7770.md (RI LSA function code 12).
//
// OpaqueLSAsByType (opaque_as.go) serves OSPFv2 consumers that decode opaque bodies by
// Opaque Type. Its OSPFv3 counterpart is a query by native LS Type: an OSPFv3 extension
// (the RFC 7770 Router Information LSA) is a native LSA identified by its 16-bit LS Type,
// not an opaque payload, so a consumer that renders or counts it queries the LSDB for the
// scope-typed LS Type. The LSDB interprets no body; this is a generic query
// (ai/rules/plugins.md).

package lsdb

import (
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// NativeLSAView is one stored LSA (identity + a copy of its body) matched by native LS
// Type. Body is a copy owned by the caller. It mirrors OpaqueLSAView but keys on the
// 16-bit LS Type (which for OSPFv3 carries the flooding scope) rather than an Opaque Type.
type NativeLSAView struct {
	Type              types.LSType
	Area              types.AreaID
	Interface         string
	LinkStateID       types.LinkStateID
	AdvertisingRouter types.RouterID
	Age               uint16
	Body              []byte
	// RawBytes is a copy of the full on-wire LSA (20-octet header + body). The OSPFv3
	// debug detail decode (spec-ospf-ext-14) re-parses it through the v3 codec, which needs
	// the header the OSPFv2 body decoders do not.
	RawBytes []byte
}

// AllLSAViews returns every stored LSA (any LS Type) across the per-area stores, the
// AS-wide store, and the per-interface link stores, each with a copy of its body. The
// OSPFv3 debug database detail / per-scope views (spec-ospf-ext-14) call it once and then
// filter/decode by native LS Type + RFC 5340 Section A.4.2.1 flooding scope in the engine;
// the LSDB interprets no body (ai/rules/plugins.md).
func (d *LSDB) AllLSAViews() []NativeLSAView {
	d.mu.RLock()
	defer d.mu.RUnlock()
	now := d.now()
	var out []NativeLSAView
	collect := func(store *areaDB, area types.AreaID, iface string) {
		if store == nil {
			return
		}
		for _, key := range store.sorted {
			lsa, ok := store.entries[key].LSA(now)
			if !ok {
				continue
			}
			body := make([]byte, len(lsa.Body))
			copy(body, lsa.Body)
			raw := make([]byte, len(lsa.RawBytes))
			copy(raw, lsa.RawBytes)
			out = append(out, NativeLSAView{
				Type:              key.Type,
				Area:              area,
				Interface:         iface,
				LinkStateID:       key.LinkStateID,
				AdvertisingRouter: key.AdvertisingRouter,
				Age:               lsa.Header.Age.Age(),
				Body:              body,
				RawBytes:          raw,
			})
		}
	}
	for area, store := range d.areas {
		collect(store, area, "")
	}
	collect(d.asExternal, types.BackboneArea, "")
	collect(d.asOpaque, types.BackboneArea, "")
	for name, store := range d.links {
		collect(store, d.linkAreas[name], name)
	}
	return out
}

// LSAViewsByType returns every stored LSA whose LS Type equals t, across the per-area
// stores, the AS-wide store (where OSPFv3 AS-scope LSAs live), and the per-interface link
// stores, with a copy of each body. A consumer that renders an OSPFv3 extension LSA (e.g.
// the RFC 7770 Router Information LSA at area scope 0xA00C or AS scope 0xC00C) calls it once
// per scoped LS Type.
func (d *LSDB) LSAViewsByType(t types.LSType) []NativeLSAView {
	d.mu.RLock()
	defer d.mu.RUnlock()
	now := d.now()
	var out []NativeLSAView
	collect := func(store *areaDB, area types.AreaID, iface string) {
		if store == nil {
			return
		}
		for _, key := range store.sorted {
			if key.Type != t {
				continue
			}
			lsa, ok := store.entries[key].LSA(now)
			if !ok {
				continue
			}
			body := make([]byte, len(lsa.Body))
			copy(body, lsa.Body)
			raw := make([]byte, len(lsa.RawBytes))
			copy(raw, lsa.RawBytes)
			out = append(out, NativeLSAView{
				Type:              key.Type,
				Area:              area,
				Interface:         iface,
				LinkStateID:       key.LinkStateID,
				AdvertisingRouter: key.AdvertisingRouter,
				Age:               lsa.Header.Age.Age(),
				Body:              body,
				RawBytes:          raw,
			})
		}
	}
	for area, store := range d.areas {
		collect(store, area, "")
	}
	collect(d.asExternal, types.BackboneArea, "")
	for name, store := range d.links {
		collect(store, d.linkAreas[name], name)
	}
	return out
}
