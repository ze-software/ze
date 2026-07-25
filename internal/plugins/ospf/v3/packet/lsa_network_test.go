// VALIDATES: spec-ospfv3-2-wire AC-9 -- the Network-LSA round-trips Options and
// the attached Router IDs, and carries no network mask. The header's Link State
// ID (the DR's Interface ID) is preserved verbatim.
// PREVENTS: treating the Type 2 Link State ID as a prefix or adding a mask.

package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv3NetworkLSARoundTrip(t *testing.T) {
	want := LSA{
		Header: sampleLSAHeader(t, types.LSTypeNetwork, "0.0.0.5"), // LS ID = DR interface ID
		Network: &NetworkLSA{
			Options:         mustOptions(t, uint32(types.OptV6|types.OptE|types.OptR)),
			AttachedRouters: []types.RouterID{mustRouterID(t, "10.0.0.1"), mustRouterID(t, "10.0.0.2"), mustRouterID(t, "10.0.0.3")},
		},
	}
	wire := encodeLSA(t, want)

	decoded, err := DecodeLSA(wire)
	if err != nil {
		t.Fatalf("DecodeLSA network: %v", err)
	}
	if decoded.Header.LinkStateID != want.Header.LinkStateID {
		t.Fatalf("LinkStateID = %v, want %v (DR interface ID preserved)", decoded.Header.LinkStateID, want.Header.LinkStateID)
	}
	body, err := decoded.DecodeNetwork()
	if err != nil {
		t.Fatalf("DecodeNetwork: %v", err)
	}
	if body.Options != want.Network.Options {
		t.Fatalf("options = %#06x, want %#06x", uint32(body.Options), uint32(want.Network.Options))
	}
	if len(body.AttachedRouters) != len(want.Network.AttachedRouters) {
		t.Fatalf("attached count = %d, want %d", len(body.AttachedRouters), len(want.Network.AttachedRouters))
	}
	for i := range want.Network.AttachedRouters {
		if body.AttachedRouters[i] != want.Network.AttachedRouters[i] {
			t.Fatalf("attached[%d] = %v, want %v", i, body.AttachedRouters[i], want.Network.AttachedRouters[i])
		}
	}
	// Body is 4 (reserved+options) + 4*N: no 4-octet network mask.
	wantBodyLen := 4 + len(want.Network.AttachedRouters)*4
	if len(decoded.Body) != wantBodyLen {
		t.Fatalf("network body length = %d, want %d (no mask)", len(decoded.Body), wantBodyLen)
	}
}
