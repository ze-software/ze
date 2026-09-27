// Design: docs/architecture/ospf/ospf-11-stub-nssa.md -- Type 7 NSSA-LSA origination.
// RFC: rfc/short/rfc3101.md -- sec 2 Type 7 NSSA-LSA (P-bit, forwarding address)

package lsdb

import (
	"bytes"
	"slices"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// OriginateNSSA builds and installs this router's Type 7 NSSA-LSA into the NSSA area
// store. RFC 3101 Section 2: the body is the Type 5 AS-External body (Network Mask,
// E-bit + metric, Forwarding Address, External Route Tag); the P (propagate) bit rides
// in the LSA-header Options field (OptionNP). Type 7 is area-scoped -- flooded only
// within the NSSA, never installed into the AS-wide Type 5 store.
func (d *LSDB) OriginateNSSA(area types.AreaID, router types.RouterID, network, mask [4]byte, type2 bool, metric uint32, fwd [4]byte, tag uint32, propagate bool) (packet.LSAHeader, bool) {
	if router == (types.RouterID{}) || area == types.BackboneArea {
		return packet.LSAHeader{}, false
	}
	// RFC 3101 §2.3 / §2.4: enforce the Type 7 P-bit policy at the origination boundary so no
	// caller can bypass it. The P-bit (propagate) requires a non-zero forwarding address, and
	// it MUST be clear when this router also originates a Type 5 AS-External LSA for the same
	// network (the Type 5 already carries the route into the backbone).
	if propagate {
		if d.selfOriginatesType5(network, router) {
			propagate = false
		} else if fwd == ([4]byte{}) {
			changed := d.PurgeNSSA(area, router, network)
			return packet.LSAHeader{}, changed
		}
	}
	key := types.LSAKey{Type: types.LSTypeNSSA, LinkStateID: types.LinkStateID(network), AdvertisingRouter: router}
	body := packet.ExternalLSA{
		NetworkMask:      mask,
		ExternalType2:    type2,
		Metric:           metric & packet.ExternalMetricMax,
		ForwardingAddr:   fwd,
		ExternalRouteTag: tag,
	}
	var opts types.Options
	if propagate {
		opts = opts.Set(types.OptionNP)
	}
	// Re-origination short-circuit: skip only when BOTH the body AND the P-bit are
	// unchanged (existingSelfBodyUnchanged compares the body alone, so a P-bit toggle
	// on an otherwise identical body must still re-originate).
	if h, same := d.existingSelfBodyUnchanged(area, key, encodedExternalBody(body)); same && h.Options.Has(types.OptionNP) == propagate {
		return h, false
	}
	seq, ok, purge := d.nextOwnSequence(area, key)
	if !ok {
		h, _ := d.Lookup(area, key)
		return h, false
	}
	h := packet.LSAHeader{Age: 0, Options: opts, Type: types.LSTypeNSSA, LinkStateID: key.LinkStateID, AdvertisingRouter: router, Sequence: seq}
	if purge {
		h.Age = types.LSAge(types.MaxAge)
	}
	return d.installOriginated(area, packet.LSA{Header: h, External: &body}, key, purge)
}

// selfOriginatesType5 reports whether this router currently originates a non-purged Type 5
// AS-External LSA for network. RFC 3101 §2.4: when the same network is advertised as a Type 5,
// the corresponding Type 7's P-bit must be clear so the route is not translated twice.
func (d *LSDB) selfOriginatesType5(network [4]byte, router types.RouterID) bool {
	key := types.LSAKey{Type: types.LSTypeASExternal, LinkStateID: types.LinkStateID(network), AdvertisingRouter: router}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.asExternal == nil {
		return false
	}
	e := d.asExternal.entries[key]
	return e != nil && e.self && !e.purged
}

// HigherRIDTranslatorExternals returns every non-purged AS-External LSA of type typ that a
// router in translators, with a Router ID strictly greater than self, advertises. It serves
// the translator-side test of RFC 3101 Section 3.2 step (2): a Type-5 is generated when "the
// calculating router has the highest router ID amongst NSSA translators that have originated a
// functionally equivalent Type-5 LSA (i.e. same destination, cost and non-zero forwarding
// address)". The caller judges equivalence, because OSPFv2 carries the destination in the Link
// State ID and Network Mask while OSPFv3 carries it in the body's prefix. translators MUST hold
// only the NSSA's border routers: a Type-5 from any other router says nothing about which
// translator yields.
func (d *LSDB) HigherRIDTranslatorExternals(typ types.LSType, translators []types.RouterID, self types.RouterID) []packet.LSA {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.asExternal == nil {
		return nil
	}
	now := d.now()
	var externals []packet.LSA
	for key, e := range d.asExternal.entries {
		if key.Type != typ {
			continue
		}
		if e.purged {
			continue
		}
		if bytes.Compare(key.AdvertisingRouter[:], self[:]) <= 0 {
			continue
		}
		if !slices.Contains(translators, key.AdvertisingRouter) {
			continue
		}
		lsa, ok := e.LSA(now)
		if !ok {
			continue // a stored entry shorter than its header is not an LSA to compare
		}
		externals = append(externals, lsa)
	}
	return externals
}

// PurgeNSSA MaxAge-purges this router's self-originated OSPFv2 Type 7 for network in the
// NSSA area, reporting whether a non-purged self LSA existed.
func (d *LSDB) PurgeNSSA(area types.AreaID, router types.RouterID, network [4]byte) bool {
	key := types.LSAKey{Type: types.LSTypeNSSA, LinkStateID: types.LinkStateID(network), AdvertisingRouter: router}
	return d.PurgeNSSAKey(area, key)
}

// PurgeNSSAKey MaxAge-purges this router's self-originated NSSA-LSA identified by key
// (RFC 2328 Section 14 premature aging), reporting whether a non-purged self LSA existed.
// It is the address-family-neutral door: key.Type must be an NSSA LS type, the OSPFv2 0x0007
// or the OSPFv3 0x2007, so no caller can flush an unrelated self LSA through it.
func (d *LSDB) PurgeNSSAKey(area types.AreaID, key types.LSAKey) bool {
	if !key.Type.NSSA() {
		return false
	}
	return d.flushSelfLSA(area, key)
}

func (d *LSDB) selfNSSACount(area types.AreaID, router types.RouterID) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	adb := d.areas[area]
	if adb == nil {
		return 0
	}
	n := 0
	for key, e := range adb.entries {
		if key.Type == types.LSTypeNSSA && key.AdvertisingRouter == router && e.self && !e.purged {
			n++
		}
	}
	return n
}
