package configjson

import (
	"slices"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// selectorTree holds a standalone peer, a grouped peer and a dynamic group's
// template: the three shapes a capability selector names.
const selectorTree = `{"bgp":{
	"peer":{"edge":{}},
	"group":{
		"core":{"peer":{"rr1":{}}},
		"ix":{"connection":{"remote":{"ip":"dynamic"}}}
	}
}}`

// TestCapabilitySelectors checks the candidate peer set is the selector of
// every ForEachPeer visit, in the CapabilitySelector form the extractors write.
func TestCapabilitySelectors(t *testing.T) {
	bgpTree, ok := ParseBGPSubtree(selectorTree)
	if !ok {
		t.Fatal("selectorTree does not parse")
	}
	got := CapabilitySelectors(bgpTree)
	slices.Sort(got)
	want := []string{"edge", CapabilityGroupKey("ix"), "rr1"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("CapabilitySelectors = %v, want %v", got, want)
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
	if len(declaration.Peers) != 3 {
		t.Errorf("peers = %v, want the three selectors of the tree", declaration.Peers)
	}
	if _, err := extract([]sdk.ConfigSection{{Root: "bgp", Data: `not json`}}); err == nil {
		t.Error("a bgp section that is not JSON was accepted")
	}
}
