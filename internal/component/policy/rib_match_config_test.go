package policy_test

import (
	"net/netip"
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_modify"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	staticevents "github.com/ze-software/ze/internal/plugins/static/events"
)

// importChain renders `<root> { filter { import { filters [ name ]; final accept; } } }`.
func importChain(root, name string) string {
	return root + " {\n\tfilter {\n\t\timport {\n\t\t\tfilters [ " + name + " ];\n\t\t\tfinal accept;\n\t\t}\n\t}\n}\n"
}

const matchPolicy = `
policy {
	prefix-list ALL {
		entry 0.0.0.0/0 { le 32; action accept; }
	}
	modify T100 {
		match { tag [ 100 ]; }
		set { distance 90; }
	}
	modify COMMUNITY {
		set { community-add [ 65000:1 ]; }
	}
	match LIST-TAG {
		prefix-list ALL;
		tag [ 4242 ];
	}
	match REJECT-4242 {
		action reject;
		tag [ 4242 ];
	}
	match L2 {
		level [ 2 ];
	}
	match NO-LIST {
		prefix-list MISSING;
	}
}
`

// TestConfigModifySetsDistanceByTag is AC-9 at config level, through the real
// YANG of policy, modify and ospf: `ospf filter import` naming a modify that
// matches tag 100 and sets distance 90 ranks a tagged OSPF path at 90 and an
// untagged one at the declared OSPF distance; a commit without the chain
// returns the tagged path to the declared distance. The same instance on
// `isis filter import` is refused at load, because IS-IS routes carry no tag.
func TestConfigModifySetsDistanceByTag(t *testing.T) {
	rib := locrib.Default()
	if rib == nil {
		t.Skip("forked plugin process: no Loc-RIB")
	}
	t.Cleanup(func() { locrib.PublishPolicy(nil) })
	tagged := netip.MustParsePrefix("203.0.113.0/24")
	untagged := netip.MustParsePrefix("203.0.114.0/24")
	ospf := ospfspf.ProtocolID()
	rib.Insert(family.IPv4Unicast, tagged, locrib.Path{Source: ospf, NextHop: netip.MustParseAddr("198.51.100.1"), Tag: 100, HasTag: true})
	rib.Insert(family.IPv4Unicast, untagged, locrib.Path{Source: ospf, NextHop: netip.MustParseAddr("198.51.100.1")})
	t.Cleanup(func() {
		rib.Remove(family.IPv4Unicast, tagged, ospf, 0)
		rib.Remove(family.IPv4Unicast, untagged, ospf, 0)
	})
	declared, ok := rib.Best(family.IPv4Unicast, untagged)
	if !ok {
		t.Fatal("untagged OSPF path not selected before any chain")
	}
	if declared.AdminDistance == 90 {
		t.Fatal("the declared OSPF distance is 90 itself, so the test cannot tell the set from it")
	}

	if err := policy.ApplyRIBImportsForTest(configSections(t, matchPolicy+importChain("ospf", "T100"))); err != nil {
		t.Fatalf("ospf import T100 refused: %v", err)
	}
	if best, _ := rib.Best(family.IPv4Unicast, tagged); best.AdminDistance != 90 {
		t.Errorf("tagged OSPF path at distance %d, want 90", best.AdminDistance)
	}
	if best, _ := rib.Best(family.IPv4Unicast, untagged); best.AdminDistance != declared.AdminDistance {
		t.Errorf("untagged OSPF path at distance %d, want the declared %d", best.AdminDistance, declared.AdminDistance)
	}

	if err := policy.ApplyRIBImportsForTest(configSections(t, matchPolicy)); err != nil {
		t.Fatal(err)
	}
	if best, _ := rib.Best(family.IPv4Unicast, tagged); best.AdminDistance != declared.AdminDistance {
		t.Errorf("after removing the chain the tagged path is at %d, want the declared %d", best.AdminDistance, declared.AdminDistance)
	}

	err := policy.ApplyRIBImportsForTest(configSections(t, matchPolicy+importChain("isis", "T100")))
	if err == nil || !strings.Contains(err.Error(), "reads tag") {
		t.Fatalf("isis import T100: err = %v, want a refusal naming tag", err)
	}
}

// TestConfigMatchStaticTag is AC-11 in-process, at config level through the
// real YANG: a static path's tag is what the match filter reads, and the
// action leaf decides which side is hidden. On the rib chain with final
// accept, `match REJECT-4242 { action reject; tag [ 4242 ]; }` hides the
// static path tagged 4242 and leaves the untagged one selected; the default
// action, in `match LIST-TAG { prefix-list ALL; tag [ 4242 ]; }`, does the
// opposite.
func TestConfigMatchStaticTag(t *testing.T) {
	rib := locrib.Default()
	if rib == nil {
		t.Skip("forked plugin process: no Loc-RIB")
	}
	t.Cleanup(func() { locrib.PublishPolicy(nil) })
	tagged := netip.MustParsePrefix("203.0.115.0/24")
	untagged := netip.MustParsePrefix("203.0.116.0/24")
	src := staticevents.ProtocolID
	rib.Insert(family.IPv4Unicast, tagged, locrib.Path{Source: src, NextHop: netip.MustParseAddr("198.51.100.1"), Tag: 4242, HasTag: true})
	rib.Insert(family.IPv4Unicast, untagged, locrib.Path{Source: src, NextHop: netip.MustParseAddr("198.51.100.1"), Tag: 0, HasTag: false})
	t.Cleanup(func() {
		rib.Remove(family.IPv4Unicast, tagged, src, 0)
		rib.Remove(family.IPv4Unicast, untagged, src, 0)
	})
	if err := policy.ApplyRIBImportsForTest(configSections(t, matchPolicy+importChain("rib", "REJECT-4242"))); err != nil {
		t.Fatalf("rib import REJECT-4242 refused: %v", err)
	}
	if best, ok := rib.Best(family.IPv4Unicast, tagged); ok {
		t.Errorf("the static path tagged 4242 passed `match { action reject; tag [ 4242 ]; }`: %+v", best)
	}
	if _, ok := rib.Best(family.IPv4Unicast, untagged); !ok {
		t.Error("the untagged static path was hidden by a reject on tag 4242")
	}

	if err := policy.ApplyRIBImportsForTest(configSections(t, matchPolicy+importChain("rib", "LIST-TAG"))); err != nil {
		t.Fatalf("rib import LIST-TAG refused: %v", err)
	}
	if _, ok := rib.Best(family.IPv4Unicast, tagged); !ok {
		t.Error("the static path tagged 4242 was hidden by a match on tag 4242")
	}
	if best, ok := rib.Best(family.IPv4Unicast, untagged); ok {
		t.Errorf("the untagged static path passed a match on tag 4242: %+v", best)
	}
}

// TestConfigMatchFieldRefusals: each instance is checked against the fields
// it names, at the attachment that names it.
func TestConfigMatchFieldRefusals(t *testing.T) {
	t.Cleanup(func() { locrib.PublishPolicy(nil) })
	cases := []struct {
		root, name, want string // want "" means accepted
	}{
		{"static", "LIST-TAG", ""},
		{"ospf", "LIST-TAG", ""},
		{"rib", "LIST-TAG", ""},
		{"isis", "LIST-TAG", "reads tag"},
		{"connected", "LIST-TAG", "reads tag"},
		{"isis", "L2", ""},
		{"ospf", "L2", "reads level"},
		{"rib", "COMMUNITY", "sets community"},
		{"rib", "NO-LIST", `"MISSING" is not a prefix-list`},
	}
	for _, tc := range cases {
		t.Run(tc.root+"/"+tc.name, func(t *testing.T) {
			err := policy.ApplyRIBImportsForTest(configSections(t, matchPolicy+importChain(tc.root, tc.name)))
			if tc.want == "" {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestMatchAndModifyBoundaries: the YANG range of each numeric match and set
// leaf, from the spec Boundary table: the last valid value parses and the
// first invalid one is refused.
func TestMatchAndModifyBoundaries(t *testing.T) {
	cases := []struct {
		name, body string
		valid      bool
	}{
		{"match tag max", "match M { tag [ 4294967295 ]; }", true},
		{"match tag above", "match M { tag [ 4294967296 ]; }", false},
		{"set distance max", "modify M { set { distance 255; } }", true},
		{"set distance above", "modify M { set { distance 256; } }", false},
		{"set tag max", "modify M { set { tag 4294967295; } }", true},
		{"set tag above", "modify M { set { tag 4294967296; } }", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.ParseTreeWithYANG("policy {\n\t"+tc.body+"\n}\n", nil)
			if tc.valid && err != nil {
				t.Fatalf("refused a valid value: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("accepted a value outside the range")
			}
		})
	}
}

// TestMatchLevelBoundaries: the level condition takes 1 or 2. The parse does
// not apply a leaf-list's range, so the refusal is the one a commit meets, at
// verify: level 2 attaches to isis import, 0 and 3 are refused naming the fix.
func TestMatchLevelBoundaries(t *testing.T) {
	t.Cleanup(func() { locrib.PublishPolicy(nil) })
	for _, tc := range []struct {
		level string
		valid bool
	}{{"2", true}, {"0", false}, {"3", false}} {
		t.Run("level "+tc.level, func(t *testing.T) {
			text := "policy {\n\tmatch M { level [ " + tc.level + " ]; }\n}\n" + importChain("isis", "M")
			err := policy.ApplyRIBImportsForTest(configSections(t, text))
			if tc.valid && err != nil {
				t.Fatalf("refused a valid level: %v", err)
			}
			if !tc.valid && (err == nil || !strings.Contains(err.Error(), "use 1 or 2")) {
				t.Fatalf("err = %v, want a refusal naming levels 1 and 2", err)
			}
		})
	}
}
