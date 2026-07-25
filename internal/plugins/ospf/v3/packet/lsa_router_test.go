// VALIDATES: spec-ospfv3-2-wire AC-8 -- the Router-LSA round-trips the W/V/E/B
// flags and the 16-octet address-free link records, with the link count derived
// from the LSA Length.
// PREVENTS: re-introducing OSPFv2 IP addresses or a #links field into the
// OSPFv3 Router-LSA.

package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv3RouterLSARoundTrip(t *testing.T) {
	want := sampleRouterLSA(t)
	wire := encodeLSA(t, want)

	decoded, err := DecodeLSA(wire)
	if err != nil {
		t.Fatalf("DecodeLSA router: %v", err)
	}
	body, err := decoded.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	if body.Flags != RouterFlagB|RouterFlagE {
		t.Fatalf("flags = %#x, want %#x", body.Flags, RouterFlagB|RouterFlagE)
	}
	if body.Options != want.Router.Options {
		t.Fatalf("options = %#06x, want %#06x", uint32(body.Options), uint32(want.Router.Options))
	}
	if len(body.Links) != len(want.Router.Links) {
		t.Fatalf("link count = %d, want %d (derived from Length)", len(body.Links), len(want.Router.Links))
	}
	for i := range want.Router.Links {
		if body.Links[i] != want.Router.Links[i] {
			t.Fatalf("link[%d] = %+v, want %+v", i, body.Links[i], want.Router.Links[i])
		}
	}
	// Each link record is exactly 16 octets; there is no #links field, so the body
	// is 4 (flags+options) + 16*N.
	wantBodyLen := 4 + len(want.Router.Links)*16
	if len(decoded.Body) != wantBodyLen {
		t.Fatalf("router body length = %d, want %d", len(decoded.Body), wantBodyLen)
	}
}

// VALIDATES: spec-ospf-ext-7 A-1 -- the OSPFv3 Router-LSA round-trips the V-bit and a
// RouterLinkTypeVirtual record (Interface ID, Neighbor Interface ID, Neighbor Router ID,
// metric) byte-for-byte (RFC 5340 App A.4.3).
func TestV6RouterLSAVirtualRecordRoundTrip(t *testing.T) {
	want := RouterLSA{
		Flags:   RouterFlagV | RouterFlagB,
		Options: mustOptions(t, uint32(types.OptV6|types.OptR)),
		Links: []RouterLink{{
			Type:                RouterLinkTypeVirtual,
			Metric:              25,
			InterfaceID:         types.InterfaceID(42),
			NeighborInterfaceID: types.InterfaceID(99),
			NeighborRouterID:    types.RouterID{172, 30, 0, 1},
		}},
	}
	buf := make([]byte, want.EncodedLen())
	want.WriteTo(buf, 0)
	got, err := DecodeRouterLSA(buf)
	if err != nil {
		t.Fatalf("DecodeRouterLSA: %v", err)
	}
	if got.Flags&RouterFlagV == 0 {
		t.Fatalf("V-bit lost: flags = %#x", got.Flags)
	}
	if len(got.Links) != 1 || got.Links[0] != want.Links[0] {
		t.Fatalf("virtual link record = %+v, want %+v", got.Links, want.Links)
	}
}

func TestOSPFv3RouterLSARejectsMisalignedLinks(t *testing.T) {
	// A body that is the fixed 4 octets plus a partial (non-16) link record must be
	// rejected, not silently truncated.
	body := make([]byte, 4+8)
	opts := mustOptions(t, uint32(types.OptV6))
	opts.WriteTo(body, 1)
	if _, err := DecodeRouterLSA(body); err == nil {
		t.Fatalf("DecodeRouterLSA accepted a misaligned link record")
	}
}
