// VALIDATES: spec-ospf-af-unify -- an OSPFv3 AS-External LSA (16-bit scope-typed LS Type
// 0x4005) installs into the AS-wide store, so it is visible from every area rather than
// trapped in one area's store. PREVENTS: the LSDB routing a v6 AS-External by the OSPFv2
// Type-5 value only, which would mis-scope OSPFv3 externals to a per-area store.
package lsdb

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestOSPFLSDBV6ASExternalIsASWide(t *testing.T) {
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	adv := rid("9.9.9.9")

	// Hand-build an OSPFv3 AS-External LSA: the 16-bit LS Type 0x4005 sits at offset 2
	// (where OSPFv2 has Options + an 8-bit type). Body is opaque for this store test.
	raw := make([]byte, types.LSAHeaderLen+4)
	binary.BigEndian.PutUint16(raw[0:], 1)      // LS age
	binary.BigEndian.PutUint16(raw[2:], 0x4005) // scope-typed LS Type (AS-External)
	copy(raw[8:12], adv[:])                     // advertising router
	binary.BigEndian.PutUint32(raw[12:], uint32(types.InitialSequenceNumber))
	binary.BigEndian.PutUint16(raw[18:], uint16(len(raw))) // length
	packet.FinalizeLSAChecksum(raw)

	hdr := types.LSAHeader{
		Age:               1,
		Type:              0x4005,
		AdvertisingRouter: adv,
		Sequence:          types.InitialSequenceNumber,
		Length:            uint16(len(raw)),
	}
	if !db.Install(area("0.0.0.1"), packet.LSA{Header: hdr, RawBytes: raw}) {
		t.Fatal("install of OSPFv3 AS-External LSA failed")
	}

	// AS-wide: the LSA must be visible from a different area than it was installed against.
	key := types.LSAKey{Type: 0x4005, AdvertisingRouter: adv}
	if _, ok := db.Lookup(area("0.0.0.2"), key); !ok {
		t.Fatal("OSPFv3 AS-External LSA not visible cross-area: it was routed to a per-area store, not the AS-wide store")
	}
}

func TestOSPFShouldDropByAreaV6(t *testing.T) {
	// The receive/send area filters (RFC 2328 sec 3.6 / RFC 3101) must classify the OSPFv3
	// scope-typed LS Types the same as the OSPFv2 values they parallel: AS-External
	// (0x4005 ~ Type 5), Inter-Area-Router (0x2004 ~ Type 4), NSSA (0x2007 ~ Type 7).
	cases := []struct {
		name     string
		area     string
		typ      types.LSType
		wantDrop bool
	}{
		// Stub area drops AS-External + ASBR-summary (both AFs); intra/inter-prefix stay.
		{"v4-ext-stub", AreaTypeStub, types.LSTypeASExternal, true},
		{"v6-ext-stub", AreaTypeStub, 0x4005, true},
		{"v4-asbr-stub", AreaTypeStub, types.LSTypeSummaryASBR, true},
		{"v6-iar-stub", AreaTypeStub, 0x2004, true},
		{"v6-iaprefix-stub", AreaTypeStub, 0x2003, false}, // inter-area prefix IS allowed in stub
		{"v6-router-stub", AreaTypeStub, 0x2001, false},
		// Normal area keeps AS-External; a stray NSSA-LSA is invalid outside an NSSA.
		{"v6-ext-normal", AreaTypeNormal, 0x4005, false},
		{"v4-nssa-normal", AreaTypeNormal, types.LSTypeNSSA, true},
		{"v6-nssa-normal", AreaTypeNormal, 0x2007, true},
		// NSSA area drops AS-External but accepts the NSSA-LSA.
		{"v6-ext-nssa", AreaTypeNSSA, 0x4005, true},
		{"v4-nssa-nssa", AreaTypeNSSA, types.LSTypeNSSA, false},
		{"v6-nssa-nssa", AreaTypeNSSA, 0x2007, false},
	}
	for _, c := range cases {
		if got := shouldDropByArea(c.area, c.typ); got != c.wantDrop {
			t.Errorf("%s: shouldDropByArea(%s, %#x) = %v, want %v", c.name, c.area, uint16(c.typ), got, c.wantDrop)
		}
		// eligibleInterface is the send-side mirror: a type dropped on receive into an area
		// must not be flooded out an interface in that area (AS-External is AS-wide, so it
		// floods on non-stub interfaces regardless of the area match).
		iface := InterfaceInfo{AreaID: area("0.0.0.5"), AreaType: c.area}
		eligible := eligibleInterface(iface, area("0.0.0.5"), c.typ)
		if c.wantDrop && eligible {
			t.Errorf("%s: eligibleInterface = true for a type dropped by area %s", c.name, c.area)
		}
	}
}

func TestOSPFLSDBV6ASExternalDroppedFromStub(t *testing.T) {
	// RFC 2328 sec 3.6: a stub area never sees AS-External LSAs. The AS-wide store holds the
	// OSPFv3 AS-External (0x4005), but a Lookup against a stub area must hide it.
	clock := &fakeClock{now: time.Unix(0, 0)}
	db := newTestDB(clock)
	db.SetAreaTypes(map[types.AreaID]string{area("0.0.0.1"): AreaTypeStub})
	adv := rid("9.9.9.9")

	raw := make([]byte, types.LSAHeaderLen+4)
	binary.BigEndian.PutUint16(raw[0:], 1)
	binary.BigEndian.PutUint16(raw[2:], 0x4005)
	copy(raw[8:12], adv[:])
	binary.BigEndian.PutUint32(raw[12:], uint32(types.InitialSequenceNumber))
	binary.BigEndian.PutUint16(raw[18:], uint16(len(raw)))
	packet.FinalizeLSAChecksum(raw)
	hdr := types.LSAHeader{Age: 1, Type: 0x4005, AdvertisingRouter: adv, Sequence: types.InitialSequenceNumber, Length: uint16(len(raw))}
	if !db.Install(area("0.0.0.2"), packet.LSA{Header: hdr, RawBytes: raw}) {
		t.Fatal("install of OSPFv3 AS-External LSA failed")
	}

	key := types.LSAKey{Type: 0x4005, AdvertisingRouter: adv}
	if _, ok := db.Lookup(area("0.0.0.1"), key); ok {
		t.Error("OSPFv3 AS-External visible in a stub area: v6 receive-suppression not applied")
	}
	if _, ok := db.Lookup(area("0.0.0.2"), key); !ok {
		t.Error("OSPFv3 AS-External not visible in a normal area")
	}
}
