// Design: docs/architecture/ike/ipsec-3-data-model.md -- IPsec data model
// Related: outer_header.go -- parseDFBit and ValidateOuterHeader, the producers under test
// RFC: rfc/short/rfc4301.md -- Section 8.1 DF bit

package ipsec

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// dfBitTree is one site-to-site peer with the given remote address and, when df is
// not empty, a df-bit leaf.
func dfBitTree(remote, mode, df string) *config.Tree {
	tree := config.NewTree()
	ipsec := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")
	peer := config.NewTree()
	peer.Set("remote-address", remote)
	if mode != "" {
		peer.Set("mode", mode)
	}
	if df != "" {
		peer.Set("df-bit", df)
	}
	ipsec.GetOrCreateContainer("site-to-site").AddListEntry("peer", "branch", peer)
	return tree
}

// VALIDATES: each df-bit value an operator writes reaches the parsed peer as its own
// treatment, and an absent leaf reaches it as copy.
// PREVENTS: a treatment that commits and is read as the default, so the SA carries the
// kernel's copy while the operator's config says clear.
// RFC requirement: RFC4301-8.1-2 positive -- the df-bit leaf of a site-to-site peer is parsed as copy, clear or set for that peer's SAs, and an absent leaf as copy.
func TestDFBitIsConfigurablePerPeer(t *testing.T) {
	for _, tc := range []struct {
		leaf string
		want dataplane.DFTreatment
	}{
		{"", dataplane.DFCopy},
		{"copy", dataplane.DFCopy},
		{"clear", dataplane.DFClear},
		{"set", dataplane.DFSet},
	} {
		cfg, err := ParseIPsecConfig(dfBitTree("198.51.100.1", "", tc.leaf))
		if err != nil {
			t.Fatalf("df-bit %q: ParseIPsecConfig: %v", tc.leaf, err)
		}
		if got := cfg.Peers["branch"].DFBit; got != tc.want {
			t.Errorf("df-bit %q parsed as %s, want %s", tc.leaf, got, tc.want)
		}
	}
	if _, err := ParseIPsecConfig(dfBitTree("198.51.100.1", "", "sometimes")); err == nil {
		t.Error("df-bit sometimes was accepted")
	}
}

// VALIDATES: commit refuses a DF treatment no dataplane can apply exactly, and accepts
// clear on an IPv4 tunnel-mode peer.
// PREVENTS: a peer that commits and never brings a Child SA up, because every install
// is refused by the backend.
// RFC requirement: RFC4301-8.1-2 negative -- df-bit set, and df-bit clear on a transport-mode or IPv6 peer, are refused by ValidateOuterHeader, while clear on an IPv4 tunnel peer is accepted.
func TestDFBitValidation(t *testing.T) {
	for _, tc := range []struct {
		name, remote, mode, df string
		refused                bool
		mentions               string
	}{
		{"clear on IPv4 tunnel", "198.51.100.1", "", "clear", false, ""},
		{"set", "198.51.100.1", "", "set", true, "df-bit set"},
		{"clear on transport", "198.51.100.1", "transport", "clear", true, "transport-mode"},
		{"clear on IPv6", "2001:db8::1", "", "clear", true, "IPv6"},
		{"copy on IPv6", "2001:db8::1", "", "copy", false, ""},
	} {
		cfg, err := ParseIPsecConfig(dfBitTree(tc.remote, tc.mode, tc.df))
		if err != nil {
			t.Fatalf("%s: ParseIPsecConfig: %v", tc.name, err)
		}
		err = cfg.ValidateOuterHeader()
		if (err != nil) != tc.refused {
			t.Errorf("%s: ValidateOuterHeader() = %v, refused want %t", tc.name, err, tc.refused)
			continue
		}
		if err != nil && !strings.Contains(err.Error(), tc.mentions) {
			t.Errorf("%s: refusal %q does not name %q", tc.name, err, tc.mentions)
		}
	}
}

// VALIDATES: the df-bit YANG default and each enum name are the ones the Go side
// parses and prints, so a peer with no leaf takes the same treatment whether the config
// system delivers the default or not.
// PREVENTS: the YANG default and dataplane.DFCopy drifting apart (one fact, two
// declarations: YANG cannot name a Go constant).
func TestDFBitYANGMatchesTheGoTreatments(t *testing.T) {
	leaf := yangLeaf(t, "df-bit")
	if def := yangStatement(t, leaf, "default"); def != dataplane.DFCopy.String() {
		t.Errorf("the df-bit YANG default is %q, and the Go zero value prints %q", def, dataplane.DFCopy.String())
	}
	for _, df := range []dataplane.DFTreatment{dataplane.DFCopy, dataplane.DFClear, dataplane.DFSet} {
		if !strings.Contains(leaf, "enum "+df.String()+" {") {
			t.Errorf("the df-bit leaf has no enum %q", df.String())
		}
	}
}
