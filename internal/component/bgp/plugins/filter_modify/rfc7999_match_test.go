// VALIDATES: a modify definition applies its operations only to a route that
// meets its match condition, and leaves every other route unchanged.
// PREVENTS: the only way to condition a modify being a match filter earlier in
// the chain, which rejects every route that does not match and so cannot express
// "rewrite this one, forward the rest untouched".

package filter_modify

import (
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// communityFields is the route's fields object as the engine writes it for
// one community field: a list of the values the event formatter writes, each
// value produced by the attribute type's own text (filter_community json.go
// writes Community.String, LargeCommunity.String and
// ExtendedCommunity.AppendDecoded).
func communityFields(field string, values ...string) string {
	out := `{"` + field + `":[`
	for i, v := range values {
		if i > 0 {
			out += ","
		}
		out += `"` + v + `"`
	}
	return out + "]}"
}

// modifyDefsFromConfig parses one modify definition out of the config shape the
// plugin receives, so each test states its input as the operator writes it.
func modifyDefsFromConfig(t *testing.T, name string, def map[string]any) *modifyDef {
	t.Helper()
	defs, err := parseModifyDefs(map[string]any{"modify": map[string]any{name: def}})
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
	t.Run("an unknown key inside match is refused", func(t *testing.T) {
		_, err := parseModifyDefs(map[string]any{"modify": map[string]any{
			"M": map[string]any{
				"match": map[string]any{"prefix": "10.0.0.0/8"},
				"set":   map[string]any{"med": 10},
			},
		}})
		if err == nil {
			t.Fatal("an unknown match key was accepted; a condition nobody evaluates reads as in force and is not")
		}
	})

	for _, tc := range []struct {
		field, value string
	}{
		{"community", "not-a-community"},
		{"large-community", "1:2:4294967296"},
		{"extended-community", "target:65000:4294967296"},
		{"extended-community", "0002fde8000000gg"},
	} {
		t.Run("invalid "+tc.field+" "+tc.value, func(t *testing.T) {
			_, err := parseModifyDefs(map[string]any{"modify": map[string]any{
				"M": map[string]any{
					"match": map[string]any{tc.field: tc.value},
					"set":   map[string]any{"med": 10},
				},
			}})
			if err == nil {
				t.Fatal("an unparseable community match was accepted")
			}
		})
	}

	t.Run("a match with no operations is still refused", func(t *testing.T) {
		_, err := parseModifyDefs(map[string]any{"modify": map[string]any{
			"M": map[string]any{"match": map[string]any{"community": "blackhole"}},
		}})
		if err == nil {
			t.Fatal("a modify with a condition and nothing to do was accepted")
		}
	})
}

func TestConfiguredMatchModifiesOnlyMatchingRoute(t *testing.T) {
	target20 := attribute.ExtendedCommunity{0, 2, 0xfd, 0xe8, 0, 0, 0, 20}
	target10 := attribute.ExtendedCommunity{0, 2, 0xfd, 0xe8, 0, 0, 0, 10}
	origin20 := attribute.ExtendedCommunity{0, 3, 0xfd, 0xe8, 0, 0, 0, 20}
	decoded := func(e attribute.ExtendedCommunity) string { return string(e.AppendDecoded(nil)) }

	for _, tc := range []struct {
		name, field, configured, matching, other string
	}{
		{"numeric well-known", "community", "65535:666",
			attribute.Community(0xffff029a).String(), attribute.Community(0xffffff01).String()},
		{"large decimal", "large-community", "065001:00100:00200",
			attribute.LargeCommunity{GlobalAdmin: 65001, LocalData1: 100, LocalData2: 200}.String(),
			attribute.LargeCommunity{GlobalAdmin: 65001, LocalData1: 100, LocalData2: 201}.String()},
		{"route target", "extended-community", "target:65000:20", decoded(target20), decoded(target10)},
		{"site of origin", "extended-community", "origin:65000:20", decoded(origin20), decoded(target20)},
		{"raw extended hex", "extended-community", "0002FDE800000014", decoded(target20), decoded(target10)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defs := map[string]*modifyDef{
				"SERVICE": modifyDefsFromConfig(t, "SERVICE", map[string]any{
					"match": map[string]any{tc.field: tc.configured},
					"set":   map[string]any{"next-hop": "192.0.2.1"},
				}),
			}
			out, err := evaluate(t, defs, modifyRoute("SERVICE", directionImport, communityFields(tc.field, tc.matching)))
			if err != nil || out.Action != sdk.FilterModify {
				t.Fatalf("matching route was not rewritten: %+v, %v", out, err)
			}
			wantSets(t, out.Sets, `next-hop set "192.0.2.1"`)
			out, err = evaluate(t, defs, modifyRoute("SERVICE", directionImport, communityFields(tc.field, tc.other)))
			if err != nil || out.Action != sdk.FilterAccept || len(out.Sets) != 0 {
				t.Fatalf("nonmatching route did not pass unchanged: %+v, %v", out, err)
			}
		})
	}
}

func TestMatchCondMatches(t *testing.T) {
	cond := matchCond{communities: []matchCommunity{
		newMatchCommunity(attribute.CommunityStandard, "blackhole"),
		newMatchCommunity(attribute.CommunityLarge, "65001:1:2"),
	}}

	for _, tt := range []struct {
		name   string
		fields string
		want   bool
	}{
		{"standard value present", `{"community":["blackhole","no-export"],"med":10}`, true},
		{"large value present", `{"large-community":["65001:1:2"]}`, true},
		{"neither present", `{"community":["65001:100"],"med":10}`, false},
		{"no community attribute at all", `{"med":10}`, false},
		{"standard value present only in the large attribute", `{"large-community":["blackhole"]}`, false},
		{"a value is matched whole, never as a prefix", `{"large-community":["65001:1:20"]}`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := cond.matches([]byte(tt.fields)); got != tt.want {
				t.Fatalf("matches(%s) = %v, want %v", tt.fields, got, tt.want)
			}
		})
	}

	// An absent condition applies to every route, which is what every modify
	// definition written before the match container did.
	var none matchCond
	if !none.empty() {
		t.Fatal("a zero matchCond must read as absent")
	}
	if !none.matches([]byte(`{}`)) {
		t.Fatal("an absent condition must apply to every route")
	}
}

// RFC requirement: RFC7999-4-1 negative -- the counter-case of the row: an
// explicit configuration directive (a modify naming the BLACKHOLE community)
// is present, so the route carrying 65535:666 has its next-hop rewritten to the
// discard address 192.0.2.1 the operator named. The discard in this test
// follows from the directive, which is why the pass-through below follows from
// its absence.
// The route is announced with no prefix authorization in scope: this unit does
// not check the covering-prefix condition of RFC 7999 Section 3.3.
func TestBlackholeCommunityRewritesNextHopWhenAgreed(t *testing.T) {
	defs := map[string]*modifyDef{}
	defs["RTBH"] = modifyDefsFromConfig(t, "RTBH", map[string]any{
		"match": map[string]any{"community": "blackhole"},
		"set":   map[string]any{"next-hop": "192.0.2.1"},
	})

	out, err := evaluate(t, defs, modifyRoute("RTBH", directionImport,
		communityFields("community", attribute.Community(0xffff029a).String())))
	if err != nil {
		t.Fatal(err)
	}
	if out.Action != sdk.FilterModify {
		t.Fatalf("action = %v, want modify", out.Action)
	}
	wantSets(t, out.Sets, `next-hop set "192.0.2.1"`)
}

// RFC requirement: RFC7999-4-1 positive -- without an explicit configuration
// directive for BLACKHOLE (the operator's modify names 65001:100 only), the
// route carrying 65535:666 is accepted with no change: its next-hop stays the
// one the peer announced, so no traffic toward 10.100.0.1/32 is discarded.
func TestBlackholeCommunityLeavesNextHopAloneWithoutAgreement(t *testing.T) {
	defs := map[string]*modifyDef{}
	defs["RTBH"] = modifyDefsFromConfig(t, "RTBH", map[string]any{
		"match": map[string]any{"community": "65001:100"},
		"set":   map[string]any{"next-hop": "192.0.2.1"},
	})

	out, err := evaluate(t, defs, modifyRoute("RTBH", directionImport,
		communityFields("community", attribute.Community(0xffff029a).String())))
	if err != nil {
		t.Fatal(err)
	}
	if out.Action != sdk.FilterAccept {
		t.Fatalf("action = %v, want accept", out.Action)
	}
	if len(out.Sets) != 0 || out.Update != "" {
		t.Fatalf("answer = %+v, want no change: an accept carries none", out)
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

	out, err := evaluate(t, defs, modifyRoute("RTBH", directionImport, communityFields("community", "65001:666")))
	if err != nil {
		t.Fatal(err)
	}
	if out.Action != sdk.FilterModify {
		t.Fatalf("action = %v, want modify", out.Action)
	}
	wantSets(t, out.Sets, `next-hop set "192.0.2.1"`)
}

// A definition with no match container keeps applying to every route. Existing
// deployments state no condition, and their modifiers must not start skipping
// routes.
func TestModifyWithoutMatchStillAppliesToEveryRoute(t *testing.T) {
	defs := map[string]*modifyDef{}
	defs["LP"] = modifyDefsFromConfig(t, "LP", map[string]any{
		"set": map[string]any{"local-preference": 200},
	})

	out, err := evaluate(t, defs, modifyRoute("LP", directionImport, `{}`))
	if err != nil {
		t.Fatal(err)
	}
	if out.Action != sdk.FilterModify {
		t.Fatalf("action = %v, want modify", out.Action)
	}
	wantSets(t, out.Sets, "local-preference set 200")
}
