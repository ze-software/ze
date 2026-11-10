// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SA/SP installation
// Related: child.go -- installChildDataplane, which builds the outbound SAParams
// RFC: rfc/short/rfc4301.md -- Section 8.1 DF bit

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: the peer's df-bit reaches the OUTBOUND state of the Child SA the engine
// installs, the encapsulator RFC 4301 Section 8.1 speaks of, and the inbound state
// keeps the default because it emits no outer header.
// PREVENTS: a treatment parsed into the peer config and dropped on the way to the
// dataplane, so every tunnel copies the DF bit whatever the operator configured.
// RFC requirement: RFC4301-8.1-2 positive -- a site-to-site peer's DF treatment is carried onto the outbound tunnel-mode SA the engine hands the dataplane, and the inbound SA keeps copy.
func TestDFBitTreatmentReachesTheBackend(t *testing.T) {
	for _, df := range []dataplane.DFTreatment{dataplane.DFCopy, dataplane.DFClear} {
		sa := testSA()
		sa.PeerCfg.DFBit = df
		dp := &mockDP{}

		child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, slogutil.DiscardLogger())
		if err != nil {
			t.Fatalf("DF %s: createFirstChildSA: %v", df, err)
		}
		var outbound, inbound int
		for _, p := range dp.sas {
			switch p.Dir {
			case dataplane.SADirOut:
				outbound++
				if p.DF != df {
					t.Errorf("DF %s: the outbound SA carries %s", df, p.DF)
				}
			case dataplane.SADirIn:
				inbound++
				if p.DF != dataplane.DFCopy {
					t.Errorf("DF %s: the inbound SA carries %s, and a decapsulating state takes none", df, p.DF)
				}
			case dataplane.SADirFwd:
				t.Errorf("DF %s: a forward-direction SA was installed", df)
			}
		}
		if outbound != 1 || inbound != 1 {
			t.Fatalf("DF %s: installed %d outbound and %d inbound SAs, want one each", df, outbound, inbound)
		}
		if child.DFBit != df {
			t.Errorf("DF %s: the Child SA records %s, which a rekey copies from", df, child.DFBit)
		}
		child.Clear()
	}
}
