// VALIDATES: spec-ospfv3-2-wire AC-12 -- the Link-LSA round-trips the router
// priority, 24-bit Options, the 128-bit link-local interface address, the 32-bit
// prefix count, and the prefix list.
// PREVENTS: mis-sizing the link-local address or the 32-bit prefix count.

package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv3LinkLSARoundTrip(t *testing.T) {
	linkLocal := [16]byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0x02, 0x11, 0x22, 0xff, 0xfe, 0x33, 0x44, 0x55}
	want := LSA{
		Header: sampleLSAHeader(t, types.LSTypeLink, "0.0.0.3"),
		Link: &LinkLSA{
			RtrPriority:   1,
			Options:       mustOptions(t, uint32(types.OptV6|types.OptR)),
			LinkLocalAddr: linkLocal,
			Prefixes: []Prefix{
				makePrefix(t, 64, types.OptPrefixLA, 0),
				makePrefix(t, 0, 0, 0),   // default route, 0 address bytes
				makePrefix(t, 128, 0, 0), // host route, 16 address bytes
			},
		},
	}
	wire := encodeLSA(t, want)

	decoded, err := DecodeLSA(wire)
	if err != nil {
		t.Fatalf("DecodeLSA link: %v", err)
	}
	body, err := decoded.DecodeLink()
	if err != nil {
		t.Fatalf("DecodeLink: %v", err)
	}
	if body.RtrPriority != want.Link.RtrPriority || body.Options != want.Link.Options || body.LinkLocalAddr != want.Link.LinkLocalAddr {
		t.Fatalf("link scalars: got %+v want %+v", body, *want.Link)
	}
	if len(body.Prefixes) != len(want.Link.Prefixes) {
		t.Fatalf("prefix count = %d, want %d", len(body.Prefixes), len(want.Link.Prefixes))
	}
	for i := range want.Link.Prefixes {
		assertPrefixEqual(t, body.Prefixes[i], want.Link.Prefixes[i])
		// The Link-LSA 16-bit field is Reserved (0) on the wire.
		if body.Prefixes[i].Field16 != 0 {
			t.Fatalf("link prefix[%d] 16-bit field = %#x, want 0 (reserved)", i, body.Prefixes[i].Field16)
		}
	}
	// The 32-bit prefix count must be encoded at body offset 20.
	if got := readUint32(decoded.Body, linkPrefixCountOff); int(got) != len(want.Link.Prefixes) {
		t.Fatalf("on-wire prefix count = %d, want %d", got, len(want.Link.Prefixes))
	}
}

func TestOSPFv3LinkLSARejectsMalformed(t *testing.T) {
	// Regression (I6): a Link-LSA whose 32-bit prefix count exceeds what the body can hold must
	// be rejected before allocating (the over-long-count OOM vector); a body shorter than the
	// fixed header is truncated. Neither must panic or allocate on the attacker-controlled count.
	want := LSA{
		Header: sampleLSAHeader(t, types.LSTypeLink, "0.0.0.3"),
		Link:   &LinkLSA{RtrPriority: 1, Options: mustOptions(t, uint32(types.OptV6|types.OptR)), Prefixes: []Prefix{makePrefix(t, 64, types.OptPrefixLA, 0)}},
	}
	decoded, err := DecodeLSA(encodeLSA(t, want))
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	body := append([]byte(nil), decoded.Body...)
	writeUint32(body, linkPrefixCountOff, 0xFFFFFFFF)
	if _, err := decodeLinkLSA(body); err == nil {
		t.Fatal("decodeLinkLSA accepted an over-long 32-bit prefix count (OOM vector)")
	}
	if _, err := decodeLinkLSA(body[:linkPrefixListOff-1]); err == nil {
		t.Fatal("decodeLinkLSA accepted a body shorter than the fixed header")
	}
}
