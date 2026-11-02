package configjson

import (
	"net/netip"
	"slices"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// selectorTree holds a standalone peer with a remote IP, a grouped peer and a
// dynamic group's template: the shapes a capability declaration reads.
const selectorTree = `{"bgp":{
	"peer":{"edge":{"connection":{"remote":{"ip":"192.0.2.1"}}}},
	"group":{
		"core":{"peer":{"rr1":{}}},
		"ix":{"connection":{"remote":{"ip":"dynamic"}}}
	}
}}`

// TestPeerSelectors checks the one selector set the reactor injects from: name,
// then address, then the dynamic group, each skipped when absent, and an
// address that is the name listed once.
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
// answers the declarations the bgp section's extract returns, and refuses a
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
	if got := declaration.Capabilities[0]; got.Code != 75 || !slices.Equal(got.Peers, []string{"edge"}) {
		t.Errorf("capability = %+v, want code 75 for edge", got)
	}
	if _, err := extract([]sdk.ConfigSection{{Root: "bgp", Data: `not json`}}); err == nil {
		t.Error("a bgp section that is not JSON was accepted")
	}
}
