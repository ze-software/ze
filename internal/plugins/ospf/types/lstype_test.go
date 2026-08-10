// Design: docs/architecture/ospf/ospf-1-types.md -- LSType known, opaque, and out-of-scope values

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
		if !typ.inScope() {
			t.Fatalf("LSType %d not in scope", typ)
		}
		// RFC requirement: RFC5250-3-1 negative -- standard OSPFv2 LS types are not classified as opaque
		if typ.IsOpaque() {
			t.Fatalf("LSType %d unexpectedly opaque", typ)
		}
	}

	for _, typ := range []LSType{LSTypeOpaqueLink, LSTypeOpaqueArea, LSTypeOpaqueAS} {
		if !typ.Known() {
			t.Fatalf("opaque LSType %d not known", typ)
		}
		if typ.inScope() {
			t.Fatalf("opaque LSType %d unexpectedly in scope", typ)
		}
		// RFC requirement: RFC5250-3-1 positive -- LS types 9/10/11 are recognized as opaque LSAs
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
// `show ospf database <type>` miss v3 entries.
// VALIDATES: spec-ospf-af-unify -- ASWide is the LSDB store-routing / flooding-scope test:
// the OSPFv2 Type 5 or ANY OSPFv3 AS-scope LS Type (S2/S1=0b10), which INCLUDES the RFC 7770
// AS-scope Router Information LSA (0xC00C) even though it is not an AS-External.
// PREVENTS: an AS-scope RI LSA being routed to a per-area store, or an area/link-scope LSA
// (or the Opaque-AS Type 11, which carries no scope bits) being mis-routed to the AS-wide store.
func TestLSTypeASWide(t *testing.T) {
	asWide := []LSType{
		LSTypeASExternal, // OSPFv2 Type 5
		0x4005,           // OSPFv3 AS-External (AS scope)
		0xC00C,           // OSPFv3 AS-scope Router Information LSA (U-bit set, AS scope) -- AS-wide but not AS-External
		0x4025,           // OSPFv3 E-AS-External (AS scope)
	}
	for _, typ := range asWide {
		if !typ.ASWide() {
			t.Errorf("LSType %#x ASWide() = false, want true", uint16(typ))
		}
	}
	notASWide := []LSType{
		LSTypeRouter, LSTypeNetwork, LSTypeSummaryNetwork, LSTypeSummaryASBR, LSTypeNSSA,
		LSTypeOpaqueLink, LSTypeOpaqueArea, LSTypeOpaqueAS, // Opaque-AS (Type 11) has no scope bits
		0x2001, 0x2004, 0x2007, 0x2009, 0x0008, // OSPFv3 area/link-scope types
		0x6001, // reserved scope (S2/S1=0b11) is not AS scope
	}
	for _, typ := range notASWide {
		if typ.ASWide() {
			t.Errorf("LSType %#x ASWide() = true, want false", uint16(typ))
		}
	}
	// ASWide is strictly WIDER than ASExternal: 0xC00C is AS-wide but must not be AS-External.
	if LSType(0xC00C).ASExternal() {
		t.Errorf("LSType 0xC00C ASExternal() = true, want false (RI LSA is not AS-External)")
	}
}

// VALIDATES: spec-ospf-af-unify -- InterAreaRouter identifies the ASBR-summary LSA suppressed
// from stub/NSSA areas: OSPFv2 Type 4 or OSPFv3 area-scoped Inter-Area-Router-LSA 0x2004.
// PREVENTS: an OSPFv3 Inter-Area-Router-LSA being treated as a plain summary, or any other
// type being mis-classified as an ASBR summary.
func TestLSTypeInterAreaRouter(t *testing.T) {
	iar := []LSType{LSTypeSummaryASBR, 0x2004}
	for _, typ := range iar {
		if !typ.InterAreaRouter() {
			t.Errorf("LSType %#x InterAreaRouter() = false, want true", uint16(typ))
		}
	}
	notIAR := []LSType{
		LSTypeRouter, LSTypeNetwork, LSTypeSummaryNetwork, LSTypeASExternal, LSTypeNSSA,
		LSTypeOpaqueLink, LSTypeOpaqueArea, LSTypeOpaqueAS,
		0x2001, 0x2003, 0x2007, 0x4005, 0x0008,
	}
	for _, typ := range notIAR {
		if typ.InterAreaRouter() {
			t.Errorf("LSType %#x InterAreaRouter() = true, want false", uint16(typ))
		}
	}
}

// VALIDATES: spec-ospfv3-4-link-lsa -- String renders every named branch, including the
// internal Grace-LSA sentinel (0x800B) and the three RFC 5250 opaque scopes, and returns
// "unknown" for an unrecognized type code.
// PREVENTS: `show ospf database <type>` rendering a Grace or Opaque LSA as "unknown" or a
// stale label, and a partial switch silently mislabeling one scope.
func TestLSTypeStringAllNames(t *testing.T) {
	tests := []struct {
		typ  LSType
		want string
	}{
		{LSTypeRouter, "router"},
		{LSTypeNetwork, "network"},
		{LSTypeSummaryNetwork, "summary-network"},
		{LSTypeSummaryASBR, "summary-asbr"},
		{LSTypeASExternal, "as-external"},
		{LSTypeNSSA, "nssa"},
		{LSTypeLink, "link"},
		{LSTypeGraceV6, "grace"},                            // 0x800B internal sentinel
		{areaScope | LSTypeOpaqueLink, "intra-area-prefix"}, // 0x2009
		{LSTypeOpaqueLink, "opaque-link"},                   // Type 9
		{LSTypeOpaqueArea, "opaque-area"},                   // Type 10
		{LSTypeOpaqueAS, "opaque-as"},                       // Type 11
		{LSType(0x1234), "unknown"},                         // unrecognized
		{LSType(0), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.typ.String(); got != tt.want {
			t.Errorf("LSType %#x String() = %q, want %q", uint16(tt.typ), got, tt.want)
		}
	}
}

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
