// VALIDATES: spec-ospfv3-2-wire AC-13 -- the Intra-Area-Prefix-LSA round-trips
// the 16-bit prefix count, the referenced (LS Type, Link State ID, Advertising
// Router) triple, and each prefix's 16-bit metric.
// PREVENTS: dropping the per-prefix metric or mis-reading the referenced triple.

package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv3IntraAreaPrefixRoundTrip(t *testing.T) {
	want := LSA{
		Header: sampleLSAHeader(t, types.LSTypeIntraAreaPrefix, "0.0.0.1"),
		IntraAreaPfx: &IntraAreaPrefixLSA{
			ReferencedLSType:      types.LSTypeRouter,
			ReferencedLinkStateID: mustLSID(t, "0.0.0.0"),
			ReferencedAdvRouter:   mustRouterID(t, "10.0.0.1"),
			Prefixes: []Prefix{
				makePrefix(t, 64, types.OptPrefixLA, 10),     // metric 10
				makePrefix(t, 128, 0, 1),                     // host route, metric 1
				makePrefix(t, 48, types.OptPrefixNU, 0xffff), // max 16-bit metric
			},
		},
	}
	wire := encodeLSA(t, want)

	decoded, err := DecodeLSA(wire)
	if err != nil {
		t.Fatalf("DecodeLSA intra-area-prefix: %v", err)
	}
	body, err := decoded.DecodeIntraAreaPrefix()
	if err != nil {
		t.Fatalf("DecodeIntraAreaPrefix: %v", err)
	}
	if body.ReferencedLSType != want.IntraAreaPfx.ReferencedLSType ||
		body.ReferencedLinkStateID != want.IntraAreaPfx.ReferencedLinkStateID ||
		body.ReferencedAdvRouter != want.IntraAreaPfx.ReferencedAdvRouter {
		t.Fatalf("referenced triple: got %+v want %+v", body, *want.IntraAreaPfx)
	}
	if len(body.Prefixes) != len(want.IntraAreaPfx.Prefixes) {
		t.Fatalf("prefix count = %d, want %d", len(body.Prefixes), len(want.IntraAreaPfx.Prefixes))
	}
	for i := range want.IntraAreaPfx.Prefixes {
		assertPrefixEqual(t, body.Prefixes[i], want.IntraAreaPfx.Prefixes[i])
		if body.Prefixes[i].Field16 != want.IntraAreaPfx.Prefixes[i].Field16 {
			t.Fatalf("prefix[%d] metric = %d, want %d", i, body.Prefixes[i].Field16, want.IntraAreaPfx.Prefixes[i].Field16)
		}
	}
	// The prefix count is a 16-bit field at body offset 0.
	if got := readUint16(decoded.Body, intraAreaPrefixCountOff); int(got) != len(want.IntraAreaPfx.Prefixes) {
		t.Fatalf("on-wire prefix count = %d, want %d", got, len(want.IntraAreaPfx.Prefixes))
	}
}

func TestOSPFv3IntraAreaPrefixRejectsMalformed(t *testing.T) {
	// Regression (I6): an Intra-Area-Prefix-LSA whose 16-bit prefix count exceeds what the body
	// can hold must be rejected before allocating (the over-long-count OOM vector); a body
	// shorter than the fixed header is truncated.
	want := LSA{
		Header: sampleLSAHeader(t, types.LSTypeIntraAreaPrefix, "0.0.0.1"),
		IntraAreaPfx: &IntraAreaPrefixLSA{
			ReferencedLSType:      types.LSTypeRouter,
			ReferencedLinkStateID: mustLSID(t, "0.0.0.0"),
			ReferencedAdvRouter:   mustRouterID(t, "10.0.0.1"),
			Prefixes:              []Prefix{makePrefix(t, 64, types.OptPrefixLA, 10)},
		},
	}
	decoded, err := DecodeLSA(encodeLSA(t, want))
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	body := append([]byte(nil), decoded.Body...)
	writeUint16(body, intraAreaPrefixCountOff, 0xFFFF)
	if _, err := decodeIntraAreaPrefixLSA(body); err == nil {
		t.Fatal("decodeIntraAreaPrefixLSA accepted an over-long 16-bit prefix count (OOM vector)")
	}
	if _, err := decodeIntraAreaPrefixLSA(body[:intraAreaPrefixListOff-1]); err == nil {
		t.Fatal("decodeIntraAreaPrefixLSA accepted a body shorter than the fixed header")
	}
}
