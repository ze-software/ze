// Design: plan/learned/955-ospf-1-types.md -- LSType known, opaque, and out-of-scope values

package types

import "testing"

// VALIDATES: AC-12 - LSType serializes as one byte and classifies in-scope and opaque values.
// PREVENTS: accepting unknown LSA types as implemented OSPFv2 LSAs.
func TestLSTypeKnownValues(t *testing.T) {
	inScope := []LSType{
		LSTypeRouter,
		LSTypeNetwork,
		LSTypeSummaryNetwork,
		LSTypeSummaryASBR,
		LSTypeASExternal,
		LSTypeNSSA,
	}
	for _, typ := range inScope {
		if !typ.Known() {
			t.Fatalf("LSType %d not known", typ)
		}
		if !typ.InScope() {
			t.Fatalf("LSType %d not in scope", typ)
		}
		if typ.IsOpaque() {
			t.Fatalf("LSType %d unexpectedly opaque", typ)
		}
	}

	for _, typ := range []LSType{LSTypeOpaqueLink, LSTypeOpaqueArea, LSTypeOpaqueAS} {
		if !typ.Known() {
			t.Fatalf("opaque LSType %d not known", typ)
		}
		if typ.InScope() {
			t.Fatalf("opaque LSType %d unexpectedly in scope", typ)
		}
		if !typ.IsOpaque() {
			t.Fatalf("opaque LSType %d not opaque", typ)
		}
	}

	if LSType(0).Known() {
		t.Fatalf("LSType 0 reported known")
	}
	if LSType(8).Known() {
		t.Fatalf("LSType 8 reported known")
	}
	var buf [1]byte
	if n := LSTypeNSSA.WriteTo(buf[:], 0); n != 1 || buf[0] != byte(LSTypeNSSA) {
		t.Fatalf("LSType.WriteTo wrote n=%d byte=%d, want n=1 byte=%d", n, buf[0], LSTypeNSSA)
	}
}

// VALIDATES: spec-ospf-af-unify -- ASExternal classifies the AS-wide-flooded LSA types so
// the LSDB routes them to its AS-wide store: the OSPFv2 Type 5 and the OSPFv3 AS-scope LS
// Types (0x4005). PREVENTS: an OSPFv3 AS-External (0x4005) landing in a per-area store, and
// (regression) any OSPFv2 type other than Type 5 being mis-classified as AS-External.
func TestLSTypeASExternal(t *testing.T) {
	asWide := []LSType{LSTypeASExternal, 0x4005}
	for _, typ := range asWide {
		if !typ.ASExternal() {
			t.Errorf("LSType %#x ASExternal() = false, want true", uint16(typ))
		}
	}
	notASWide := []LSType{
		LSTypeRouter, LSTypeNetwork, LSTypeSummaryNetwork, LSTypeSummaryASBR, LSTypeNSSA,
		LSTypeOpaqueLink, LSTypeOpaqueArea, LSTypeOpaqueAS, // Opaque-AS (Type 11) is not AS-External
		0x2001, 0x2002, 0x2003, 0x2004, 0x2007, 0x2009, 0x0008, // OSPFv3 area/link-scope types
	}
	for _, typ := range notASWide {
		if typ.ASExternal() {
			t.Errorf("LSType %#x ASExternal() = true, want false", uint16(typ))
		}
	}
}

func TestLSTypeNSSA(t *testing.T) {
	nssa := []LSType{LSTypeNSSA, 0x2007} // OSPFv2 Type 7 and the OSPFv3 area-scoped NSSA-LSA
	for _, typ := range nssa {
		if !typ.NSSA() {
			t.Errorf("LSType %#x NSSA() = false, want true", uint16(typ))
		}
	}
	notNSSA := []LSType{
		LSTypeRouter, LSTypeNetwork, LSTypeSummaryNetwork, LSTypeSummaryASBR, LSTypeASExternal,
		LSTypeOpaqueLink, LSTypeOpaqueArea, LSTypeOpaqueAS,
		0x2001, 0x2002, 0x2003, 0x2004, 0x2009, 0x4005, 0x0008, // other OSPFv3 types are not NSSA
	}
	for _, typ := range notNSSA {
		if typ.NSSA() {
			t.Errorf("LSType %#x NSSA() = true, want false", uint16(typ))
		}
	}
}

// VALIDATES: spec-ospfv3-4-link-lsa / spec-ospfv3-5-nssa-redist -- OSPFv3
// scope-typed LS Types render with the same stable names the OSPF CLI/database
// filters use, including Link-LSA (0x0008), NSSA-LSA (0x2007), AS-External
// (0x4005), and Intra-Area-Prefix (0x2009).
// PREVENTS: OSPFv3 LSDB snapshots showing every v3 LSA as "unknown" and making
// `show ip ospf database <type>` miss v3 entries.
func TestLSTypeStringOSPFv3(t *testing.T) {
	tests := []struct {
		typ  LSType
		want string
	}{
		{0x2001, "router"},
		{0x2002, "network"},
		{0x2003, "summary-network"},
		{0x2004, "summary-asbr"},
		{0x4005, "as-external"},
		{0x2007, "nssa"},
		{0x0008, "link"},
		{0x2009, "intra-area-prefix"},
	}
	for _, tt := range tests {
		if got := tt.typ.String(); got != tt.want {
			t.Errorf("LSType %#x String() = %q, want %q", uint16(tt.typ), got, tt.want)
		}
	}
}
