// VALIDATES: spec-ospf-ext-4 -- Extended Prefix reception: the N-Flag is ignored on a
// non-host prefix (AC-5), the same prefix across LSAs resolves to the lowest Opaque ID (AC-9),
// a duplicate prefix in one LSA uses the first instance (AC-10), a Type-11 LSA from an
// unreachable originator is present-but-unusable (AC-14), and a malformed body is counted and
// not applied (AC-7 metric).
// PREVENTS: a lost host-route flag on a non-host prefix, the wrong LSA's attributes winning,
// an unreachable-originator LSA being used, or a malformed LSA polluting state.
package ospf

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/transport"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

func extPrefixBody(routeType, plen, flags uint8, addr [4]byte) []byte {
	return packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{{
		RouteType: routeType, PrefixLength: plen, AF: packet.ExtPrefixAFIPv4Unicast, Flags: flags, AddressPrefix: addr,
	}}})
}

func extRecvEngine(t *testing.T) *engine {
	t.Helper()
	return newEngine(transport.New(&fakeBackend{}))
}

// TestExtPrefixSameOpaqueIDRefreshUpdates is the ext-4 review regression: a refresh of the SAME
// Extended Prefix Opaque LSA (same Opaque ID, delivered only on a newer install) MUST overwrite
// the resolved attributes, not be dropped as a lower-or-equal duplicate. The cross-LSA dedup
// keeps the strictly-lower Opaque ID; the equal case is a refresh and must win.
func TestExtPrefixSameOpaqueIDRefreshUpdates(t *testing.T) {
	r := newExtReceiver()
	adv := types.RouterID{3, 3, 3, 3}
	prefix := [5]byte{10, 2, 2, 2, 32}
	r.applyPrefix(adv, 1, packet.ExtRouteTypeIntraArea, 0, prefix, OpaqueScopeArea, true)
	// Same Opaque ID 1, refreshed: now inter-area with the N-Flag. Must overwrite.
	r.applyPrefix(adv, 1, packet.ExtRouteTypeInterArea, packet.ExtPrefixFlagN, prefix, OpaqueScopeArea, true)
	e, ok := r.lookupPrefix(adv, prefix)
	if !ok {
		t.Fatalf("prefix not stored")
	}
	if e.flags&packet.ExtPrefixFlagN == 0 || e.routeType != packet.ExtRouteTypeInterArea {
		t.Fatalf("same-Opaque-ID refresh dropped: flags=%#x routeType=%d", e.flags, e.routeType)
	}
	// AC-9 preserved: a strictly-lower existing Opaque ID still wins over a higher incoming one.
	r.applyPrefix(adv, 5, packet.ExtRouteTypeIntraArea, 0, prefix, OpaqueScopeArea, true)
	if e2, _ := r.lookupPrefix(adv, prefix); e2.opaqueID != 1 {
		t.Fatalf("lowest Opaque ID must win: got %d want 1", e2.opaqueID)
	}
}

func TestExtPrefixNFlagIgnoredNonHost(t *testing.T) {
	eng := extRecvEngine(t)
	adv := types.RouterID{2, 2, 2, 2}
	// N-Flag set on a /24 (non-host): it MUST be ignored (RFC 7684 sec 2.1), not malformed.
	eng.extPrefixOnReceive(opaqueReceived{
		OpaqueType: packet.ExtPrefixOpaqueType, OpaqueID: 1, Scope: OpaqueScopeArea, AdvertisingRouter: adv,
		Body: extPrefixBody(packet.ExtRouteTypeIntraArea, 24, packet.ExtPrefixFlagN, [4]byte{10, 1, 1, 0}), Reachable: true,
	})
	e, ok := eng.extRecv.lookupPrefix(adv, [5]byte{10, 1, 1, 0, 24})
	if !ok {
		t.Fatalf("prefix not stored")
	}
	// RFC requirement: RFC7684-2.1-1 negative -- the N-Flag set on a /24 non-host prefix is
	// cleared on receive (extNormalizeFlags, ext_prefix.go:265-271), so it is never honored.
	if e.flags&packet.ExtPrefixFlagN != 0 {
		t.Fatalf("N-Flag must be ignored on a /24 non-host prefix, flags=%#x", e.flags)
	}
	// The same flag on a /32 host IS kept.
	eng.extPrefixOnReceive(opaqueReceived{
		OpaqueType: packet.ExtPrefixOpaqueType, OpaqueID: 2, Scope: OpaqueScopeArea, AdvertisingRouter: adv,
		Body: extPrefixBody(packet.ExtRouteTypeIntraArea, 32, packet.ExtPrefixFlagN, [4]byte{10, 9, 9, 9}), Reachable: true,
	})
	h, _ := eng.extRecv.lookupPrefix(adv, [5]byte{10, 9, 9, 9, 32})
	// RFC requirement: RFC7684-2.1-1 positive -- the N-Flag set on a /32 host prefix is
	// retained on receive; normalization is confined to non-host prefixes.
	if h.flags&packet.ExtPrefixFlagN == 0 {
		t.Fatalf("N-Flag must be kept on a /32 host prefix, flags=%#x", h.flags)
	}
}

func TestExtPrefixLowestOpaqueIDWins(t *testing.T) {
	eng := extRecvEngine(t)
	adv := types.RouterID{2, 2, 2, 2}
	key := [5]byte{10, 2, 2, 0, 24}
	recv := func(id uint32, flags uint8) {
		eng.extPrefixOnReceive(opaqueReceived{
			OpaqueType: packet.ExtPrefixOpaqueType, OpaqueID: id, Scope: OpaqueScopeArea, AdvertisingRouter: adv,
			Body: extPrefixBody(packet.ExtRouteTypeIntraArea, 24, flags, [4]byte{10, 2, 2, 0}), Reachable: true,
		})
	}
	recv(5, packet.ExtPrefixFlagA) // higher Opaque ID first
	recv(2, 0)                     // lower Opaque ID wins
	e, ok := eng.extRecv.lookupPrefix(adv, key)
	if !ok || e.opaqueID != 2 {
		t.Fatalf("lowest Opaque ID must win, got %+v ok=%v", e, ok)
	}
	// A later higher Opaque ID does not displace the lower one.
	recv(9, 0)
	e, _ = eng.extRecv.lookupPrefix(adv, key)
	if e.opaqueID != 2 {
		t.Fatalf("higher Opaque ID displaced the lower, got %d", e.opaqueID)
	}
}

func TestExtPrefixDuplicateInLSAFirstWins(t *testing.T) {
	eng := extRecvEngine(t)
	adv := types.RouterID{2, 2, 2, 2}
	// One LSA with two Extended Prefix TLVs for the same prefix: first RT intra, second RT
	// inter. The first instance is used (RFC 7684 sec 2.1).
	body := packet.EncodeExtPrefixLSA(packet.ExtPrefixLSA{Prefixes: []packet.ExtPrefixTLV{
		{RouteType: packet.ExtRouteTypeIntraArea, PrefixLength: 24, AddressPrefix: [4]byte{10, 3, 3, 0}},
		{RouteType: packet.ExtRouteTypeInterArea, PrefixLength: 24, AddressPrefix: [4]byte{10, 3, 3, 0}},
	}})
	eng.extPrefixOnReceive(opaqueReceived{
		OpaqueType: packet.ExtPrefixOpaqueType, OpaqueID: 1, Scope: OpaqueScopeArea, AdvertisingRouter: adv, Body: body, Reachable: true,
	})
	e, ok := eng.extRecv.lookupPrefix(adv, [5]byte{10, 3, 3, 0, 24})
	if !ok || e.routeType != packet.ExtRouteTypeIntraArea {
		t.Fatalf("first Extended Prefix TLV instance must win, got %+v ok=%v", e, ok)
	}
}

func TestExtPrefixType11UnreachableUnusable(t *testing.T) {
	eng := extRecvEngine(t)
	adv := types.RouterID{2, 2, 2, 2}
	key := [5]byte{198, 51, 100, 0, 24}
	body := extPrefixBody(packet.ExtRouteTypeASExternal, 24, 0, [4]byte{198, 51, 100, 0})
	// Type-11 (AS scope) with an unreachable originator -> present but unusable (RFC 5250 sec 5).
	eng.extPrefixOnReceive(opaqueReceived{
		OpaqueType: packet.ExtPrefixOpaqueType, OpaqueID: 1, Scope: OpaqueScopeAS, AdvertisingRouter: adv, Body: body, Reachable: false,
	})
	e, ok := eng.extRecv.lookupPrefix(adv, key)
	if !ok {
		t.Fatalf("Type-11 prefix not stored")
	}
	if e.usable {
		t.Fatalf("Type-11 prefix from unreachable originator must be unusable")
	}
	// Once reachable, a fresh (lower/equal Opaque ID) instance is usable.
	eng2 := extRecvEngine(t)
	eng2.extPrefixOnReceive(opaqueReceived{
		OpaqueType: packet.ExtPrefixOpaqueType, OpaqueID: 1, Scope: OpaqueScopeAS, AdvertisingRouter: adv, Body: body, Reachable: true,
	})
	r, _ := eng2.extRecv.lookupPrefix(adv, key)
	if !r.usable {
		t.Fatalf("Type-11 prefix from reachable originator must be usable")
	}
}

func TestExtPrefixMalformedCounted(t *testing.T) {
	reg := &opaqueMetricRegistry{counts: map[string]int{}}
	eng := extRecvEngine(t)
	eng.setMetrics(reg)
	adv := types.RouterID{2, 2, 2, 2}
	// A top-level TLV Length that overruns the body is malformed (RFC 7684 sec 5).
	eng.extPrefixOnReceive(opaqueReceived{
		OpaqueType: packet.ExtPrefixOpaqueType, OpaqueID: 1, Scope: OpaqueScopeArea, AdvertisingRouter: adv,
		Body: []byte{0x00, 0x01, 0x00, 0xff, 0x01, 0x20, 0x00, 0x00}, Reachable: true,
	})
	// RFC requirement: RFC7684-5-1 negative -- a top-level TLV Length overrunning the body is
	// detected by the bound-checked decoder, counted (ze_ospf_ext_malformed_total), and no
	// prefix attribute is stored, rather than crashing the routing process (§5).
	if reg.counts["ze_ospf_ext_malformed_total|7"] == 0 {
		t.Fatalf("malformed body did not increment ze_ospf_ext_malformed_total: %v", reg.counts)
	}
	if _, ok := eng.extRecv.lookupPrefix(adv, [5]byte{}); ok {
		t.Fatalf("malformed LSA must not store any prefix attribute")
	}
}
