// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SA/SP installation
// Related: child.go -- installChildDataplane, which builds the outbound SAParams
// RFC: rfc/short/rfc4301.md -- Section 5.1.2.1 tunnel header DSCP

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: the peer's dscp-map reaches the outbound state the engine installs, and
// the inbound state keeps copy.
// PREVENTS: a mapping parsed into the peer and dropped before the dataplane.
// RFC requirement: RFC4301-5.1.2.1-1 positive -- a site-to-site peer's outer DSCP mapping is carried onto the outbound tunnel-mode SA the engine hands the dataplane, and the inbound SA keeps copy.
func TestOuterHeaderDSCPReachesTheBackend(t *testing.T) {
	ef, err := dataplane.FixedOuterDSCP(46)
	if err != nil {
		t.Fatal(err)
	}
	sa := testSA()
	sa.PeerCfg.OuterDSCP = ef
	dp := &mockDP{}
	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	defer child.Clear()
	seen := 0
	for _, p := range dp.sas {
		want := dataplane.OuterDSCPCopy
		if p.Dir == dataplane.SADirOut {
			want = ef
			seen++
		}
		if p.OuterDSCP != want {
			t.Errorf("SA dir %d carries outer DSCP %s, want %s", p.Dir, p.OuterDSCP, want)
		}
	}
	if seen != 1 {
		t.Fatalf("installed %d outbound SAs, want 1", seen)
	}
}
