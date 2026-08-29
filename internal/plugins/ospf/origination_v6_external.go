// Design: docs/architecture/ospf/ospf-af-unify.md -- OSPFv3 (IPv6) AS-External origination (ASBR redistribution).
// RFC: rfc/short/rfc5340.md (App A.4.7 AS-External-LSA), rfc/short/rfc2328.md (sec 12.4.4)
//
// The IPv6 side of redistribution: when a redistributed IPv6 route is injected, this router
// (as an OSPFv3 ASBR) originates either an AS-External-LSA (0x4005) into the AS-wide store
// or an NSSA-LSA (0x2007) into each attached NSSA. Unlike OSPFv2 (where the Link State ID is
// the network address), the OSPFv3 external Link State ID is an arbitrary index, assigned
// once per prefix and remembered (engine.redistV6) for re-origination and withdrawal.

package ospf

import (
	"fmt"
	"net/netip"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// v6ExternalSelfTypes are the OSPFv3 self-LSA types the redistribution path owns; the
// stale-flush on withdrawal sweeps AS-External and NSSA externals, leaving the self Router /
// Network / Intra-Area-Prefix and inter-area summaries untouched.
var v6ExternalSelfTypes = map[types.LSType]struct{}{
	types.LSType(ospfv3types.LSTypeASExternal): {},
	types.LSType(ospfv3types.LSTypeNSSA):       {},
}

// v6InjectExternal originates (or refreshes) an OSPFv3 external LSA for a redistributed IPv6
// prefix and re-originates the Router-LSA so its E-bit reflects ASBR status. The LSID is
// assigned once per prefix and remembered for withdrawal.
func (e *engine) v6InjectExternal(prefix netip.Prefix, source string) error {
	// Serialize with the NSSA Type-7->Type-5 translation flush (translateNSSAV6, also under
	// nssaMu): that per-second pass snapshots redistV6 into a keep-set, then FlushStaleSelfLSAs
	// purges any self AS-External not in it. Without this lock an injection that lands in the
	// snapshot->flush window is purged and stays withdrawn until the next redistribution event.
	// Lock order is nssaMu -> e.mu, matching translateNSSA.
	e.nssaMu.Lock()
	defer e.nssaMu.Unlock()
	e.mu.Lock()
	cfg := e.cfg
	db := e.lsdb
	if db == nil || cfg.RouterID == (types.RouterID{}) {
		e.mu.Unlock()
		return errEngineNotReady
	}
	router := cfg.RouterID
	e.mu.Unlock()

	// Validate the prefix BEFORE assigning a Link State ID, so an unusable prefix does not leak a
	// value from the monotonic redistV6Next counter.
	prefix = prefix.Masked()
	wirePrefix, ok := netipToV6Prefix(prefix, 0)
	if !ok {
		return fmt.Errorf("ospf: external prefix %q is not a usable IPv6 prefix", prefix)
	}

	e.mu.Lock()
	lsid, ok := e.redistV6[prefix]
	if !ok {
		e.redistV6Next++
		lsid = v6SummaryLSID(e.redistV6Next)
		e.redistV6[prefix] = lsid
	}
	e.mu.Unlock()

	type2, metric, tag := externalParams(cfg, source)
	nssas, canType5 := e.externalScopeV6()
	for _, n := range nssas {
		propagate := !canType5 && n.hasFA
		e.v6OriginateNSSALSA(n.area, router, lsid, wirePrefix, type2, metric, n.fa, n.hasFA, tag, propagate)
	}
	if canType5 {
		e.v6OriginateExternalLSA(router, lsid, wirePrefix, type2, metric, tag)
	}
	e.originateSelfLSAs()
	e.refreshExternalMetrics(db, router)
	return nil
}

// v6WithdrawExternal MaxAge-purges the OSPFv3 AS-External/NSSA-LSA previously originated for
// prefix and re-originates the Router-LSA (clearing the E-bit when the last external is gone).
func (e *engine) v6WithdrawExternal(prefix netip.Prefix) (bool, error) {
	// Same nssaMu serialization as v6InjectExternal: the withdrawal builds its own keep-set and
	// flushes, so it must not interleave with the NSSA translation flush (nssaMu -> e.mu order).
	e.nssaMu.Lock()
	defer e.nssaMu.Unlock()
	e.mu.Lock()
	cfg := e.cfg
	db := e.lsdb
	prefix = prefix.Masked()
	delete(e.redistV6, prefix)
	router := cfg.RouterID
	redist := make([]types.LinkStateID, 0, len(e.redistV6))
	for _, lsid := range e.redistV6 {
		redist = append(redist, lsid)
	}
	translations := make([]types.LinkStateID, 0, len(e.translations))
	for lsid := range e.translations {
		translations = append(translations, types.LinkStateID(lsid))
	}
	e.mu.Unlock()
	if db == nil || router == (types.RouterID{}) {
		return false, nil
	}
	nssas, _ := e.externalScopeV6()
	keep := make(map[ospflsdb.SelfLSARef]struct{}, len(redist)*(len(nssas)+1)+len(translations))
	for _, lsid := range redist {
		keep[ospflsdb.SelfLSARef{Area: types.BackboneArea, Key: v6ExternalKey(router, lsid)}] = struct{}{}
		for _, n := range nssas {
			keep[ospflsdb.SelfLSARef{Area: n.area, Key: v6NSSAKey(router, lsid)}] = struct{}{}
		}
	}
	for _, lsid := range translations {
		keep[ospflsdb.SelfLSARef{Area: types.BackboneArea, Key: v6ExternalKey(router, lsid)}] = struct{}{}
	}
	// The per-area NSSA default belongs to applyNSSADefaults, not to redistribution, and its
	// LSID is not in redistV6. Without this it would be swept by the withdrawal of an
	// unrelated redistributed prefix, taking the RFC 3101 Section 2.4 border-router default
	// with it, and only the next reconciliation tick would put it back.
	for _, attachment := range nssas {
		keep[ospflsdb.SelfLSARef{Area: attachment.area, Key: v6NSSAKey(router, v6NSSADefaultLSID)}] = struct{}{}
	}
	n := db.FlushStaleSelfLSAs(router, v6ExternalSelfTypes, keep)
	if n > 0 {
		e.originateSelfLSAs()
		e.refreshExternalMetrics(db, router)
	}
	return n > 0, nil
}

// v6ExternalKey is the LSDB key for an OSPFv3 AS-External-LSA this router originates.
func v6ExternalKey(router types.RouterID, lsid types.LinkStateID) types.LSAKey {
	return types.LSAKey{Type: types.LSType(ospfv3types.LSTypeASExternal), LinkStateID: lsid, AdvertisingRouter: router}
}

// v6OriginateExternalLSA builds and installs one OSPFv3 AS-External-LSA (AS-wide store) for a
// prefix with the given metric type / metric / route tag.
func (e *engine) v6OriginateExternalLSA(router types.RouterID, lsid types.LinkStateID, prefix ospfv3packet.Prefix, type2 bool, metric, tag uint32) bool {
	body := ospfv3packet.ExternalLSA{
		ExternalType2: type2,
		// Clamp to the 24-bit metric field (RFC 5340 sec A.4.7), matching the NSSA path and the
		// OSPFv2 encoder; an unmasked value would silently truncate on the wire.
		Metric:           metric & packet.ExternalMetricMax,
		Prefix:           prefix,
		ExternalRouteTag: tag,
		HasRouteTag:      tag != 0,
	}
	bodyBytes := make([]byte, body.EncodedLen())
	body.WriteTo(bodyBytes, 0)
	key := v6ExternalKey(router, lsid)
	id := lsid
	b := body
	_, ok := e.lsdb.OriginateSelf(types.BackboneArea, key, bodyBytes, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header:   v6OriginHeader(ospfv3types.LSTypeASExternal, ospfv3types.LinkStateID(id), router, seq, purge),
			External: &b,
		})
	})
	return ok
}
