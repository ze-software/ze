package configjson

import (
	"maps"
	"net/netip"
	"slices"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// selectorTree holds a standalone peer with a remote IP, a grouped peer and a
// dynamic group's template: the shapes a capability selector names.
const selectorTree = `{"bgp":{
	"peer":{"edge":{"connection":{"remote":{"ip":"192.0.2.1"}}}},
	"group":{
		"core":{"peer":{"rr1":{}}},
		"ix":{"connection":{"remote":{"ip":"dynamic"}}}
	}
}}`

// TestCapabilitySelectors checks the candidate peer set holds every selector of
// every ForEachPeer visit: a peer's name and its remote IP (role declares under
// the IP), and a dynamic group's template under its group selector, never the
// group's "dynamic" placeholder address.
func TestCapabilitySelectors(t *testing.T) {
	bgpTree, ok := ParseBGPSubtree(selectorTree)
	if !ok {
		t.Fatal("selectorTree does not parse")
	}
	got := CapabilitySelectors(bgpTree)
	slices.Sort(got)
	want := []string{"edge", "192.0.2.1", CapabilityGroupKey("ix"), "rr1"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("CapabilitySelectors = %v, want %v", got, want)
	}
}

// TestPeerSelectors checks the one selector set the reactor injects from and a
// reload candidate offers: name, then address, then the dynamic group, each
// skipped when absent, and an address that is the name listed once.
func TestPeerSelectors(t *testing.T) {
	addr := netip.MustParseAddr("192.0.2.1")
	cases := []struct {
		name, group string
		addr        netip.Addr
		want        []string
	}{
		{"edge", "", addr, []string{"edge", "192.0.2.1"}},
		{"edge", "", netip.Addr{}, []string{"edge"}},
		{"192.0.2.1", "", addr, []string{"192.0.2.1"}},
		{"dyn-192.0.2.1", "ix", addr, []string{"dyn-192.0.2.1", "192.0.2.1", CapabilityGroupKey("ix")}},
		{"", "ix", netip.Addr{}, []string{CapabilityGroupKey("ix")}},
	}
	for _, c := range cases {
		if got := PeerSelectors(c.name, c.addr, c.group); !slices.Equal(got, c.want) {
			t.Errorf("PeerSelectors(%q, %v, %q) = %v, want %v", c.name, c.addr, c.group, got, c.want)
		}
	}
}

// TestBGPCapabilityExtractor checks the extractor reads only bgp sections,
// reports every selector of the tree as a candidate peer, and refuses a
// section whose data is not JSON.
func TestBGPCapabilityExtractor(t *testing.T) {
	extract := BGPCapabilityExtractor(func(string) ([]sdk.CapabilityDecl, error) {
		return []sdk.CapabilityDecl{{Code: 75, Peers: []string{"edge"}}}, nil
	})
	declaration, err := extract([]sdk.ConfigSection{
		{Root: "exabgp", Data: `not json`},
		{Root: "bgp", Data: selectorTree},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(declaration.Capabilities) != 1 {
		t.Errorf("capabilities = %v, want the one the bgp section declares", declaration.Capabilities)
	}
	if len(declaration.Peers) != 4 {
		t.Errorf("peers = %v, want the four selectors of the tree", declaration.Peers)
	}
	wantLabels := map[string]string{CapabilityGroupKey("ix"): "peers of dynamic group ix"}
	if !maps.Equal(declaration.Labels, wantLabels) {
		t.Errorf("labels = %v, want only the dynamic group's template named as its group", declaration.Labels)
	}
	if _, err := extract([]sdk.ConfigSection{{Root: "bgp", Data: `not json`}}); err == nil {
		t.Error("a bgp section that is not JSON was accepted")
	}
}
