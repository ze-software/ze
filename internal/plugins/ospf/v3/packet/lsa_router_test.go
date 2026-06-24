// VALIDATES: spec-ospfv3-2-wire AC-8 -- the Router-LSA round-trips the W/V/E/B
// flags and the 16-octet address-free link records, with the link count derived
// from the LSA Length.
// PREVENTS: re-introducing OSPFv2 IP addresses or a #links field into the
// OSPFv3 Router-LSA.

package packet

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/v3/types"
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
