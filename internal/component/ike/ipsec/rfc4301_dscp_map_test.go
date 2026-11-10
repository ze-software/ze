// Design: docs/architecture/ike/ipsec-3-data-model.md -- IPsec data model
// Related: outer_header.go -- parseDSCPMap and ValidateOuterHeader, the producers under test
// RFC: rfc/short/rfc4301.md -- Section 5.1.2.1 tunnel header DSCP

package ipsec

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// VALIDATES: dscp-map parses copy, 0 and 63, takes copy when absent, and refuses 64
// and a word; commit refuses a mapping on a transport-mode peer.
// PREVENTS: a mapping that commits and reaches the peer as copy.
// RFC requirement: RFC4301-5.1.2.1-1 positive -- the dscp-map leaf of a site-to-site peer is parsed as copy or as a fixed DSCP 0 to 63 for that peer's SAs.
func TestDSCPMapIsConfigurablePerPeer(t *testing.T) {
	for _, tc := range []struct {
		leaf  string
		fixed bool
		value uint8
		bad   bool
	}{
		{"", false, 0, false}, {"copy", false, 0, false}, {"0", true, 0, false}, {"63", true, 63, false},
		{"64", false, 0, true}, {"af41", false, 0, true},
	} {
		got, err := parseDSCPMap("branch", tc.leaf, tc.leaf != "")
		if tc.bad {
			if err == nil {
				t.Errorf("dscp-map %q was accepted as %s", tc.leaf, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("dscp-map %q: %v", tc.leaf, err)
		}
		v, fixed := got.Fixed()
		if fixed != tc.fixed || v != tc.value {
			t.Errorf("dscp-map %q parsed as %s", tc.leaf, got)
		}
	}

	cfg, err := ParseIPsecConfig(dfBitTree("198.51.100.1", "transport", ""))
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	peer := cfg.Peers["branch"]
	peer.OuterDSCP, err = dataplane.FixedOuterDSCP(46)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Peers["branch"] = peer
	if err := cfg.ValidateOuterHeader(); err == nil {
		t.Error("dscp-map 46 on a transport-mode peer was accepted")
	}
	if def := yangStatement(t, yangLeaf(t, "dscp-map"), "default"); def != dataplane.OuterDSCPCopy.String() {
		t.Errorf("the dscp-map YANG default is %q, and the Go zero value prints %q", def, dataplane.OuterDSCPCopy.String())
	}
}
