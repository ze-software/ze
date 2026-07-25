// VALIDATES: spec-ospf-ext-14 AC-2/AC-4/AC-8 -- `show ospf ipv6 database <type> detail`
// renders each native v3 LSA's scope-aware header + a decoded body (registered decoder ->
// named fields; no decoder -> header + body-hex; malformed -> raw hex + decode-error
// metric, no panic); `... database scope <link|area|as>` filters on the RFC 5340
// Section A.4.2.1 S2/S1 bits; a reserved scope is rejected.
// PREVENTS: a v6 detail view keyed on a flat OSPFv2 type number, one that panics on a
// malformed body, or a scope filter that ignores the S2/S1 bits.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func newV6DecodeEngine(t *testing.T, router types.RouterID) *engine {
	t.Helper()
	e := newEngineWithCodecAF(transport.New(&fakeBackend{}), v6Codec{}, afIPv6Unicast)
	e.cfg.RouterID = router
	e.lsdb.SetSelfRouterID(router)
	return e
}

func originateV6Router(t *testing.T, e *engine, router types.RouterID, area types.AreaID) {
	t.Helper()
	body := ospfv3packet.RouterLSA{}
	bb := make([]byte, body.EncodedLen())
	body.WriteTo(bb, 0)
	key := types.LSAKey{Type: types.LSType(ospfv3types.LSTypeRouter), AdvertisingRouter: router}
	_, ok := e.lsdb.OriginateSelf(area, key, bb, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header: v6OriginHeader(ospfv3types.LSTypeRouter, ospfv3types.LinkStateID{}, router, seq, purge),
			Router: &body,
		})
	})
	if !ok {
		t.Fatalf("OriginateSelf(v6 Router-LSA) returned false")
	}
}

// originateV6Raw originates a v6 LSA of the given wire LS Type carrying an opaque raw body
// (no typed pointer), so an unknown function code or a malformed body can be exercised.
func originateV6Raw(t *testing.T, e *engine, wire ospfv3types.LSType, router types.RouterID, area types.AreaID, body []byte) {
	t.Helper()
	key := types.LSAKey{Type: v6NeutralLSType(wire), AdvertisingRouter: router}
	_, ok := e.lsdb.OriginateSelf(area, key, body, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header: v6OriginHeader(wire, ospfv3types.LinkStateID{}, router, seq, purge),
			Body:   body,
		})
	})
	if !ok {
		t.Fatalf("OriginateSelf(v6 raw %#x) returned false", uint16(wire))
	}
}

func v3DetailRowsOf(t *testing.T, e *engine, scopeFilter string) []v3DetailLSA {
	t.Helper()
	out, err := e.v3DatabaseDetailSnapshot("", scopeFilter)
	if err != nil {
		t.Fatalf("v3DatabaseDetailSnapshot(%q): %v", scopeFilter, err)
	}
	for _, v := range out {
		if db, ok := v.(v3DetailDatabase); ok {
			return db.LSAs
		}
	}
	t.Fatalf("no v3DetailDatabase in %#v", out)
	return nil
}

func TestV3DecodeTypedDecoder(t *testing.T) {
	router := types.RouterID{10, 0, 0, 1}
	e := newV6DecodeEngine(t, router)
	originateV6Router(t, e, router, types.BackboneArea)

	rows := v3DetailRowsOf(t, e, "")
	if len(rows) != 1 {
		t.Fatalf("v3 detail rows = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.LSType != "router" || r.Decoder != "router" || r.Decoded == nil {
		t.Fatalf("router LSA decode = %+v", r)
	}
	if r.Scope != "area" {
		t.Fatalf("router LSA scope = %q, want area", r.Scope)
	}
	if !r.LocalOriginated {
		t.Fatalf("self-originated v6 LSA should be local (AC-23)")
	}
}

func TestV3DecodeFallbackNoDecoder(t *testing.T) {
	router := types.RouterID{10, 0, 0, 2}
	e := newV6DecodeEngine(t, router)
	// Area-scope LS Type with an unknown function code (0x0FF): no registered decoder.
	originateV6Raw(t, e, ospfv3types.LSType(0x20FF), router, types.BackboneArea, []byte{0xde, 0xad, 0xbe, 0xef})

	rows := v3DetailRowsOf(t, e, "")
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Decoder != "generic" || rows[0].BodyHex != "deadbeef" {
		t.Fatalf("generic fallback row = %+v", rows[0])
	}
	if rows[0].Scope != "area" {
		t.Fatalf("scope = %q, want area", rows[0].Scope)
	}
}

func TestV3DecodeMalformed(t *testing.T) {
	rec := withDebugMetrics(t)
	router := types.RouterID{10, 0, 0, 3}
	e := newV6DecodeEngine(t, router)
	// Router LS Type but a 5-byte body: DecodeRouterLSA requires 4 + 16k, so it errors.
	originateV6Raw(t, e, ospfv3types.LSTypeRouter, router, types.BackboneArea, []byte{1, 2, 3, 4, 5})

	rows := v3DetailRowsOf(t, e, "")
	if len(rows) != 1 || !rows[0].Malformed || rows[0].BodyHex == "" {
		t.Fatalf("malformed router body should render raw hex: %+v", rows[0])
	}
	if rec.get("ze_ospfv3_debug_decode_errors_total") == 0 {
		t.Fatalf("v6 decode-error metric not incremented")
	}
}

func TestV3DatabaseScopeFilter(t *testing.T) {
	router := types.RouterID{10, 0, 0, 4}
	e := newV6DecodeEngine(t, router)
	originateV6Router(t, e, router, types.BackboneArea)                                                          // area 0x2001
	originateV6Raw(t, e, ospfv3types.LSTypeASExternal, router, types.BackboneArea, []byte{0, 0, 0, 0})           // as 0x4005
	originateV6Raw(t, e, ospfv3types.LSType(0x0099), router, types.BackboneArea, []byte{0x11, 0x22, 0x33, 0x44}) // link-local

	area := v3DetailRowsOf(t, e, "area")
	if len(area) != 1 || area[0].Scope != "area" {
		t.Fatalf("scope=area rows = %+v", area)
	}
	as := v3DetailRowsOf(t, e, "as")
	if len(as) != 1 || as[0].Scope != "as" {
		t.Fatalf("scope=as rows = %+v", as)
	}
	// A reserved / unknown scope selector is rejected.
	if _, err := e.v3DatabaseDetailSnapshot("", "reserved"); err == nil {
		t.Fatalf("reserved scope should be rejected")
	}
}
