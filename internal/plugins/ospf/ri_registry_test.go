// VALIDATES: spec-ospf-ext-3 AC-7/AC-8, R-3/R-6 -- registerRITLV stores a downstream
// consumer's TLV builder; the RI originator emits the type-1 Informational Capabilities TLV
// FIRST and then the registered builders in ascending TLV-type order; and a panicking builder
// is recovered, counted, and omitted while the RI LSA is still emitted.
// PREVENTS: a registered TLV emitted before the type-1 TLV (RFC 7770 sec 2.4 MUST violation),
// out-of-order registered TLVs, or a single bad consumer crashing OSPF origination.
package ospf

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// riTLVTypes decodes an RI body into its ordered TLV type list.
func riTLVTypes(t *testing.T, body []byte) []uint16 {
	t.Helper()
	decoded, err := packet.DecodeRITLVStream(body)
	if err != nil {
		t.Fatalf("decode RI body: %v", err)
	}
	types := make([]uint16, 0, len(decoded))
	for _, tlv := range decoded {
		types = append(types, tlv.Type)
	}
	return types
}

func TestRITLVRegistered(t *testing.T) {
	resetRITLVs()
	t.Cleanup(resetRITLVs)

	called := false
	if err := registerRITLV(8, OpaqueScopeArea, func(types.RouterID) []packet.RITLV {
		called = true
		return []packet.RITLV{{Type: 8, Value: []byte{1, 2, 3, 4}}}
	}); err != nil {
		t.Fatalf("registerRITLV: %v", err)
	}
	eng, router := newRedistEngine(t, riCfg(true, "area"))
	bodies := eng.buildRIInstances(OpaqueScopeArea, router)
	if len(bodies) != 1 {
		t.Fatalf("instances = %d, want 1", len(bodies))
	}
	if !called {
		t.Fatalf("registered builder not invoked")
	}
	got := riTLVTypes(t, bodies[0])
	// type-1 first (sec 2.4), then type-2 functional carrier, then the registered type-8.
	// RFC requirement: RFC7770-2.6-1 positive -- the type-2 Functional Capabilities TLV is carried
	// in the first instance (Instance 0) of the RI LSA, immediately after the type-1 TLV (§2.6).
	want := []uint16{packet.RITLVInformationalCapabilities, packet.RITLVFunctionalCapabilities, 8}
	if len(got) != len(want) {
		t.Fatalf("TLV types = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("TLV[%d] = %d, want %d (order %v)", i, got[i], want[i], got)
		}
	}
}

func TestRITLVRegisteredOrder(t *testing.T) {
	resetRITLVs()
	t.Cleanup(resetRITLVs)

	// Register out of type order; the originator must emit them ascending after type-1/2.
	for _, typ := range []uint16{10, 8} {
		if err := registerRITLV(typ, OpaqueScopeArea, func(types.RouterID) []packet.RITLV {
			return []packet.RITLV{{Type: typ, Value: []byte{0, 0, 0, 0}}}
		}); err != nil {
			t.Fatalf("registerRITLV %d: %v", typ, err)
		}
	}
	// Reserved types are rejected.
	if err := registerRITLV(packet.RITLVInformationalCapabilities, OpaqueScopeArea, nil); err == nil {
		t.Fatalf("registering reserved type 1 must fail")
	}
	// Duplicate is rejected.
	if err := registerRITLV(8, OpaqueScopeArea, nil); err == nil {
		t.Fatalf("duplicate registration must fail")
	}

	eng, router := newRedistEngine(t, riCfg(true, "area"))
	got := riTLVTypes(t, eng.buildRIInstances(OpaqueScopeArea, router)[0])
	want := []uint16{packet.RITLVInformationalCapabilities, packet.RITLVFunctionalCapabilities, 8, 10}
	if len(got) != len(want) {
		t.Fatalf("TLV order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("TLV[%d] = %d, want %d (%v)", i, got[i], want[i], got)
		}
	}
}

func TestRITLVScopeFiltered(t *testing.T) {
	resetRITLVs()
	t.Cleanup(resetRITLVs)
	// A builder registered for AS scope must NOT appear in an area-scope body.
	if err := registerRITLV(8, OpaqueScopeAS, func(types.RouterID) []packet.RITLV {
		return []packet.RITLV{{Type: 8, Value: []byte{1, 2, 3, 4}}}
	}); err != nil {
		t.Fatalf("registerRITLV: %v", err)
	}
	eng, router := newRedistEngine(t, riCfg(true, "area", "as"))
	area := riTLVTypes(t, eng.buildRIInstances(OpaqueScopeArea, router)[0])
	if len(area) != 2 {
		t.Fatalf("area body TLVs = %v, want just type-1 and type-2 (AS-scoped builder excluded)", area)
	}
	as := riTLVTypes(t, eng.buildRIInstances(OpaqueScopeAS, router)[0])
	if len(as) != 3 || as[2] != 8 {
		t.Fatalf("as body TLVs = %v, want type-1, type-2, type-8", as)
	}
}

func TestRITLVBuilderPanicIsolated(t *testing.T) {
	resetRITLVs()
	t.Cleanup(resetRITLVs)

	if err := registerRITLV(8, OpaqueScopeArea, func(types.RouterID) []packet.RITLV {
		panic("bad SR builder")
	}); err != nil {
		t.Fatalf("registerRITLV: %v", err)
	}
	eng, router := newRedistEngine(t, riCfg(true, "area"))
	var errCount int
	eng.ri.builderErrors = countingCounter{n: &errCount}

	bodies := eng.buildRIInstances(OpaqueScopeArea, router) // must not panic
	if len(bodies) != 1 {
		t.Fatalf("instances = %d, want 1", len(bodies))
	}
	got := riTLVTypes(t, bodies[0])
	// The RI LSA is still emitted with the type-1 TLV first; the panicking builder's TLV is absent.
	if len(got) != 2 || got[0] != packet.RITLVInformationalCapabilities {
		t.Fatalf("TLVs = %v, want the type-1/type-2 pair without the failed builder's TLV", got)
	}
	if errCount != 1 {
		t.Fatalf("builder error counter = %d, want 1", errCount)
	}
}
