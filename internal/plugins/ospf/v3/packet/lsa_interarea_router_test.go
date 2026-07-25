// VALIDATES: spec-ospfv3-2-wire AC-10 -- the Inter-Area-Router-LSA round-trips
// the 24-bit Options, the 24-bit metric, and the destination Router ID in a
// fixed 12-octet body.
// PREVENTS: a misaligned reserved octet shifting the metric or destination.

package packet

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv3InterAreaRouterRoundTrip(t *testing.T) {
	want := LSA{
		Header: sampleLSAHeader(t, types.LSTypeInterAreaRouter, "0.0.0.1"),
		InterAreaRtr: &InterAreaRouterLSA{
			Options:           mustOptions(t, uint32(types.OptV6|types.OptE)),
			Metric:            0x00abcd,
			DestinationRouter: mustRouterID(t, "192.0.2.50"),
		},
	}
	wire := encodeLSA(t, want)

	decoded, err := DecodeLSA(wire)
	if err != nil {
		t.Fatalf("DecodeLSA inter-area-router: %v", err)
	}
	if len(decoded.Body) != 12 {
		t.Fatalf("inter-area-router body length = %d, want 12", len(decoded.Body))
	}
	body, err := decoded.DecodeInterAreaRouter()
	if err != nil {
		t.Fatalf("DecodeInterAreaRouter: %v", err)
	}
	if body != *want.InterAreaRtr {
		t.Fatalf("decoded = %+v, want %+v", body, *want.InterAreaRtr)
	}
}
