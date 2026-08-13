// VALIDATES: a modify definition applies its operations only to a route that
// meets its match condition, and leaves every other route unchanged.
// PREVENTS: the only way to condition a modify being a match filter earlier in
// the chain, which rejects every route that does not match and so cannot express
// "rewrite this one, forward the rest untouched".

package filter_modify

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filtertext"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// modifyDefsFromConfig parses one modify definition out of the config shape the
// plugin receives, so each test states its input as the operator writes it.
func modifyDefsFromConfig(t *testing.T, name string, def map[string]any) *modifyDef {
	t.Helper()
	defs, err := parseModifyDefs(map[string]any{
		"policy": map[string]any{
			"modify": map[string]any{name: def},
		},
	})
	if err != nil {
		t.Fatalf("parseModifyDefs: %v", err)
	}
	d, ok := defs[name]
	if !ok {
		t.Fatalf("modify %q absent from %v", name, defs)
	}
	return d
}

func TestParseModifyMatch(t *testing.T) {
	t.Run("standard community", func(t *testing.T) {
		d := modifyDefsFromConfig(t, "RTBH", map[string]any{
			"match": map[string]any{"community": []any{"65001:666"}},
			"set":   map[string]any{"next-hop": "192.0.2.1"},
		})
		if d.match.empty() {
			t.Fatal("match condition parsed as empty")
		}
		if got, want := len(d.match.communities), 1; got != want {
			t.Fatalf("communities = %d, want %d", got, want)
		}
		if got := d.match.communities[0]; got.value != "65001:666" || got.kind != filtertext.CommunityStandard {
			t.Fatalf("community = %+v, want 65001:666 standard", got)
		}
	})

	t.Run("a well-known value is normalized to the name the formatter emits", func(t *testing.T) {
		// The filter text renders 0xFFFF029A as "blackhole", never as
		// "65535:666". Without normalization an operator writing the numeric
		// form configures a condition that can never fire.
		d := modifyDefsFromConfig(t, "RTBH", map[string]any{
			"match": map[string]any{"community": []any{"65535:666"}},
			"set":   map[string]any{"next-hop": "192.0.2.1"},
		})
		if got := d.match.communities[0].value; got != "blackhole" {
			t.Fatalf("community = %q, want %q", got, "blackhole")
		}
	})

	t.Run("the RFC name is accepted as written", func(t *testing.T) {
		d := modifyDefsFromConfig(t, "RTBH", map[string]any{
			"match": map[string]any{"community": "blackhole"},
			"set":   map[string]any{"next-hop": "192.0.2.1"},
		})
		if got := d.match.communities[0].value; got != "blackhole" {
			t.Fatalf("community = %q, want %q", got, "blackhole")
		}
	})

	t.Run("large and extended", func(t *testing.T) {
		d := modifyDefsFromConfig(t, "M", map[string]any{
			"match": map[string]any{
				"large-community":    []any{"65001:100:200"},
				"extended-community": []any{"target:65001:1"},
			},
			"set": map[string]any{"local-preference": 200},
		})
		kinds := map[filtertext.CommunityKind]string{}
		for _, c := range d.match.communities {
			kinds[c.kind] = c.value
		}
		if kinds[filtertext.CommunityLarge] != "65001:100:200" {
			t.Errorf("large = %q", kinds[filtertext.CommunityLarge])
		}
		if kinds[filtertext.CommunityExtended] == "" {
			t.Error("extended community absent")
		}
	})

	t.Run("an unknown key inside match is refused", func(t *testing.T) {
		_, err := parseModifyDefs(map[string]any{
			"policy": map[string]any{"modify": map[string]any{
				"M": map[string]any{
					"match": map[string]any{"prefix": "10.0.0.0/8"},
					"set":   map[string]any{"med": 10},
				},
			}},
		})
		if err == nil {
			t.Fatal("an unknown match key was accepted; a condition nobody evaluates reads as in force and is not")
		}
	})

	t.Run("an unparseable community value is refused", func(t *testing.T) {
		_, err := parseModifyDefs(map[string]any{
			"policy": map[string]any{"modify": map[string]any{
				"M": map[string]any{
					"match": map[string]any{"community": "not-a-community"},
					"set":   map[string]any{"med": 10},
				},
			}},
		})
		if err == nil {
			t.Fatal("an unparseable community was accepted; it would silently never match")
		}
	})

	t.Run("a match with no operations is still refused", func(t *testing.T) {
		_, err := parseModifyDefs(map[string]any{
			"policy": map[string]any{"modify": map[string]any{
				"M": map[string]any{"match": map[string]any{"community": "blackhole"}},
			}},
		})
		if err == nil {
			t.Fatal("a modify with a condition and nothing to do was accepted")
		}
	})
}

func TestMatchCondMatches(t *testing.T) {
	cond := matchCond{communities: []matchCommunity{
		{kind: filtertext.CommunityStandard, value: "blackhole"},
		{kind: filtertext.CommunityLarge, value: "65001:1:2"},
	}}

	for _, tt := range []struct {
		name string
		text string
		want bool
	}{
		{"standard value present", "community [blackhole no-export] med 10", true},
		{"large value present", "large-community 65001:1:2", true},
		{"neither present", "community 65001:100 med 10", false},
		{"no community attribute at all", "origin igp med 10", false},
		{"standard value present only in the large attribute", "large-community blackhole", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := cond.matches(tt.text); got != tt.want {
				t.Fatalf("matches(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}

	// An absent condition applies to every route, which is what every modify
	// definition written before the match container did.
	var none matchCond
	if !none.empty() {
		t.Fatal("a zero matchCond must read as absent")
	}
	if !none.matches("origin igp") {
		t.Fatal("an absent condition must apply to every route")
	}
}

// RFC requirement: RFC7999-3.3-2 positive
// RFC 7999 Section 3.3 lets a speaker honor a BLACKHOLE announcement only when
// "the receiving party agreed to honor the BLACKHOLE community on that
// particular BGP session". The agreement IS the configuration: a modify naming
// the community, on that peer's import chain. Here it is present, so the route
// carrying the community has its next-hop rewritten to the discard address the
// operator named.
func TestBlackholeCommunityRewritesNextHopWhenAgreed(t *testing.T) {
	defs := map[string]*modifyDef{}
	defs["RTBH"] = modifyDefsFromConfig(t, "RTBH", map[string]any{
		"match": map[string]any{"community": "blackhole"},
		"set":   map[string]any{"next-hop": "192.0.2.1"},
	})
	defsByName.Store(&defs)

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "RTBH",
		Peer:   "10.0.0.2",
		Update: "origin igp next-hop 10.0.0.2 community blackhole nlri ipv4 unicast add 10.100.0.1/32",
	})
	if out.Action != sdk.FilterModify {
		t.Fatalf("action = %v, want modify", out.Action)
	}
	if out.Update != "next-hop 192.0.2.1" {
		t.Fatalf("delta = %q, want %q", out.Update, "next-hop 192.0.2.1")
	}
}

// RFC requirement: RFC7999-3.3-2 negative
// The same speaker, the same announcement, and no agreement for that community:
// the operator's modify names a different community, so nothing was agreed for
// BLACKHOLE. The route passes through with the next-hop the peer announced, and
// is forwarded normally.
func TestBlackholeCommunityLeavesNextHopAloneWithoutAgreement(t *testing.T) {
	defs := map[string]*modifyDef{}
	defs["RTBH"] = modifyDefsFromConfig(t, "RTBH", map[string]any{
		"match": map[string]any{"community": "65001:100"},
		"set":   map[string]any{"next-hop": "192.0.2.1"},
	})
	defsByName.Store(&defs)

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "RTBH",
		Peer:   "10.0.0.2",
		Update: "origin igp next-hop 10.0.0.2 community blackhole nlri ipv4 unicast add 10.100.0.1/32",
	})
	if out.Action != sdk.FilterAccept {
		t.Fatalf("action = %v, want accept", out.Action)
	}
	if out.Update != "" {
		t.Fatalf("delta = %q, want empty: an accept carries no change", out.Update)
	}
}

// An operator's own community is not a special case of the well-known one. This
// is the case the previous design could not express at all, because it read one
// hardcoded value.
func TestOperatorOwnCommunityRewritesNextHop(t *testing.T) {
	defs := map[string]*modifyDef{}
	defs["RTBH"] = modifyDefsFromConfig(t, "RTBH", map[string]any{
		"match": map[string]any{"community": "65001:666"},
		"set":   map[string]any{"next-hop": "192.0.2.1"},
	})
	defsByName.Store(&defs)

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "RTBH",
		Peer:   "10.0.0.2",
		Update: "origin igp next-hop 10.0.0.2 community 65001:666 nlri ipv4 unicast add 10.100.0.1/32",
	})
	if out.Action != sdk.FilterModify {
		t.Fatalf("action = %v, want modify", out.Action)
	}
	if out.Update != "next-hop 192.0.2.1" {
		t.Fatalf("delta = %q, want %q", out.Update, "next-hop 192.0.2.1")
	}
}

// A definition with no match container keeps applying to every route. Existing
// deployments state no condition, and their modifiers must not start skipping
// routes.
func TestModifyWithoutMatchStillAppliesToEveryRoute(t *testing.T) {
	defs := map[string]*modifyDef{}
	defs["LP"] = modifyDefsFromConfig(t, "LP", map[string]any{
		"set": map[string]any{"local-preference": 200},
	})
	defsByName.Store(&defs)

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "LP",
		Peer:   "10.0.0.2",
		Update: "origin igp next-hop 10.0.0.2 nlri ipv4 unicast add 10.100.0.1/32",
	})
	if out.Action != sdk.FilterModify {
		t.Fatalf("action = %v, want modify", out.Action)
	}
	if out.Update != "local-preference 200" {
		t.Fatalf("delta = %q", out.Update)
	}
}
