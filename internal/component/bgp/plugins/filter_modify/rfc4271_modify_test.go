package filter_modify

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// The plugin answers a route with the definition's filter sets, and the
// engine applies them (policychain Sets). The tests here assert the answer:
// the sets a configuration builds and the action a route gets. What the
// engine writes for those sets is proven where it is computed:
//   - the arithmetic (an increment or a decrement from the route's own value,
//     the clamp at 0 and at 4294967295, an absent attribute starting from the
//     chain's base, and the base unused when the route carries the
//     attribute): policychain TestChainFoldsNumericSets and
//     TestExternalNumericSetsResolveAgainstTheRoute;
//   - the AIGP increment of a route with no AIGP writing nothing:
//     policychain TestExternalAIGPIncrementOfAbsentMetricLeavesTheRoute;
//   - the med removal on import, and an export route that answers one
//     failing: policychain TestExternalMEDRemove;
//   - the base read from bgp { defaults { attribute { } } }: bgp/config
//     TestParseAttributeBases.

// modifyRoute is the route the engine hands the evaluator: the header and
// the declared fields as the engine writes them.
func modifyRoute(filter, direction, fields string) sdk.FilterRoute {
	return rpc.InputFilterRoute(&rpc.FilterUpdateInput{
		Filter: filter, Direction: direction, Peer: "10.0.0.1", PeerAS: 65001, Fields: json.RawMessage(fields),
	})
}

// setsText renders sets as "field action value" lines, in order.
func setsText(sets []rpc.FilterSet) []string {
	out := make([]string, 0, len(sets))
	for _, set := range sets {
		out = append(out, set.Field+" "+string(set.Action)+" "+string(set.Value))
	}
	return out
}

// wantSets fails unless got holds exactly want, in order.
func wantSets(t *testing.T, got []rpc.FilterSet, want ...string) {
	t.Helper()
	text := setsText(got)
	if strings.Join(text, "\n") != strings.Join(want, "\n") {
		t.Fatalf("sets = %q, want %q", text, want)
	}
}

// parseOne parses one definition named name out of the policy block the
// plugin is delivered.
func parseOne(t *testing.T, name string, def map[string]any) *modifyDef {
	t.Helper()
	defs, err := parseModifyDefs(map[string]any{"modify": map[string]any{name: def}})
	if err != nil {
		t.Fatalf("parseModifyDefs: %v", err)
	}
	return defs[name]
}

// evaluate installs defs and answers the evaluator's verdict for route.
func evaluate(t *testing.T, defs map[string]*modifyDef, route sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	t.Helper()
	previous := defsByName.Load()
	t.Cleanup(func() { defsByName.Store(previous) })
	defsByName.Store(&defs)
	return evaluateModify(&route)
}

// VALIDATES: AC-1 -- local-preference set; AC-2 -- med set; AC-3 -- origin
// set; AC-4 -- next-hop set; AC-7 -- only the declared attributes; AC-8 --
// several attributes in one modifier. Each becomes one set of the answer, and
// the log line's delta text names each.
// PREVENTS: a wrong value, a missing attribute or an extra one in the answer.
func TestBuildSets(t *testing.T) {
	tests := []struct {
		name      string
		setBlock  map[string]any
		wantSets  []string
		wantDelta string
	}{
		{"local_preference_only", map[string]any{"local-preference": float64(200)},
			[]string{"local-preference set 200"}, "local-preference 200"},
		{"med_only", map[string]any{"med": float64(50)}, []string{"med set 50"}, "med 50"},
		{"origin_igp", map[string]any{"origin": "igp"}, []string{`origin set "igp"`}, "origin igp"},
		{"origin_incomplete", map[string]any{"origin": "incomplete"},
			[]string{`origin set "incomplete"`}, "origin incomplete"},
		{"next_hop", map[string]any{"next-hop": "10.0.0.1"}, []string{`next-hop set "10.0.0.1"`}, "next-hop 10.0.0.1"},
		{"multiple_attributes", map[string]any{"local-preference": float64(200), "med": float64(50), "origin": "igp"},
			[]string{"local-preference set 200", "med set 50", `origin set "igp"`}, "local-preference 200 med 50 origin igp"},
		{"string_numeric", map[string]any{"local-preference": "300"},
			[]string{"local-preference set 300"}, "local-preference 300"},
		{"local_preference_zero", map[string]any{"local-preference": float64(0)},
			[]string{"local-preference set 0"}, "local-preference 0"},
		{"local_preference_max", map[string]any{"local-preference": float64(4294967295)},
			[]string{"local-preference set 4294967295"}, "local-preference 4294967295"},
		{"med_max", map[string]any{"med": float64(4294967295)}, []string{"med set 4294967295"}, "med 4294967295"},
		{"as_path_prepend", map[string]any{"as-path-prepend": float64(3)},
			[]string{"as-path prepend 3"}, "as-path-prepend 3"},
		{"multiple_with_prepend", map[string]any{"local-preference": float64(200), "as-path-prepend": float64(2)},
			[]string{"local-preference set 200", "as-path prepend 2"}, "local-preference 200 as-path-prepend 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := parseOne(t, "M", map[string]any{"set": tt.setBlock})
			wantSets(t, def.sets, tt.wantSets...)
			if def.delta != tt.wantDelta {
				t.Fatalf("delta = %q, want %q", def.delta, tt.wantDelta)
			}
		})
	}
}

// VALIDATES: a set block that states nothing, and an as-path-prepend outside
// the YANG range 1 to 32, are refused rather than dropped.
// PREVENTS: a definition installed that changes nothing the operator wrote.
func TestBuildSetsRefuses(t *testing.T) {
	for _, tt := range []struct {
		name     string
		setBlock map[string]any
		errText  string
	}{
		{"empty_set_block", map[string]any{}, "no operations defined"},
		{"nil_values_ignored", map[string]any{"local-preference": nil, "med": nil}, "no operations defined"},
		{"prepend_zero_refused", map[string]any{"as-path-prepend": float64(0)}, "outside 1 to 32"},
		{"prepend_over_32_refused", map[string]any{"as-path-prepend": float64(33)}, "outside 1 to 32"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseModifyDefs(map[string]any{"modify": map[string]any{"M": map[string]any{"set": tt.setBlock}}})
			if err == nil || !strings.Contains(err.Error(), tt.errText) {
				t.Fatalf("error = %v, want one naming %q", err, tt.errText)
			}
		})
	}
}

// VALIDATES: an IPv6 next-hop is refused when the config is read. NEXT_HOP
// (RFC 4271 Section 5.1.3, type 3) carries an IPv4 address only, the
// engine's next-hop set writes that attribute and nothing else, and the text
// path before it logged "next-hop must be IPv4" and dropped the operation.
// PREVENTS: a definition the engine cannot apply being installed, so that
// every route it meets fails instead of the commit.
func TestBuildSetsRefusesAnIPv6NextHop(t *testing.T) {
	_, err := parseModifyDefs(map[string]any{"modify": map[string]any{
		"V6": map[string]any{"set": map[string]any{"next-hop": "2001:db8::1"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "IPv4") {
		t.Fatalf("error = %v, want the IPv6 next-hop refused naming IPv4", err)
	}
	_, err = parseModifyDefs(map[string]any{"modify": map[string]any{
		"BAD": map[string]any{"set": map[string]any{"next-hop": "not-an-address"}},
	}})
	if err == nil {
		t.Fatal("an unparseable next-hop was accepted")
	}
}

// VALIDATES: Config parsing for modify definitions.
// PREVENTS: Invalid config silently accepted.
func TestParseModifyDefs(t *testing.T) {
	tests := []struct {
		name      string
		policyCfg map[string]any
		wantCount int
		wantDelta string // for single-def tests
		errSubstr string
	}{
		{name: "single_def", wantCount: 1, wantDelta: "local-preference 200",
			policyCfg: map[string]any{"modify": map[string]any{
				"PREFER-LOCAL": map[string]any{"set": map[string]any{"local-preference": float64(200)}},
			}}},
		{name: "multiple_defs", wantCount: 2,
			policyCfg: map[string]any{"modify": map[string]any{
				"A": map[string]any{"set": map[string]any{"local-preference": float64(100)}},
				"B": map[string]any{"set": map[string]any{"med": float64(50)}},
			}}},
		{name: "no_policy", policyCfg: map[string]any{}, wantCount: 0},
		{name: "no_operations", errSubstr: "no operations defined",
			policyCfg: map[string]any{"modify": map[string]any{"BAD": map[string]any{}}}},
		{name: "name_too_long", errSubstr: "exceeds maximum length",
			policyCfg: map[string]any{"modify": map[string]any{
				strings.Repeat("x", maxNameLen+1): map[string]any{"set": map[string]any{"local-preference": float64(100)}},
			}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defs, err := parseModifyDefs(tt.policyCfg)
			if tt.errSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("error = %v, want one containing %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(defs) != tt.wantCount {
				t.Fatalf("expected %d defs, got %d", tt.wantCount, len(defs))
			}
			for _, def := range defs {
				if tt.wantDelta != "" && def.delta != tt.wantDelta {
					t.Errorf("delta = %q, want %q", def.delta, tt.wantDelta)
				}
			}
		})
	}
}

// VALIDATES: a configured modifier answers modify with its sets and no text;
// a name no definition holds, or a route before the configuration, is an
// error, so the route meets the declaration's on-error rather than passing
// unchanged.
// PREVENTS: an unconfigured modifier answering accept.
func TestEvaluateModify(t *testing.T) {
	defs := map[string]*modifyDef{"PREFER-LOCAL": parseOne(t, "PREFER-LOCAL",
		map[string]any{"set": map[string]any{"local-preference": float64(200)}})}

	out, err := evaluate(t, defs, modifyRoute("PREFER-LOCAL", directionImport, `{}`))
	if err != nil || out.Action != sdk.FilterModify {
		t.Fatalf("known = %+v, %v; want modify", out, err)
	}
	wantSets(t, out.Sets, "local-preference set 200")

	if out, err := evaluate(t, defs, modifyRoute("NONEXISTENT", directionImport, `{}`)); err == nil {
		t.Fatalf("unknown modifier = %+v, want an error", out)
	}
	defsByName.Store(nil)
	route := modifyRoute("PREFER-LOCAL", directionImport, `{}`)
	if out, err := evaluateModify(&route); err == nil {
		t.Fatalf("before configure = %+v, want an error", out)
	}
}

// VALIDATES: AC-1, AC-2 -- an increment and a decrement are answered as the
// operation, never as a number the plugin computed: the engine resolves them
// against the route's value (policychain TestChainFoldsNumericSets).
// AC-13 -- set and increment of one attribute are refused at parse time.
func TestParseModifyDefsIncrementDecrement(t *testing.T) {
	def := parseOne(t, "INC-LP", map[string]any{"increment": map[string]any{"local-preference": float64(50)}})
	wantSets(t, def.sets, "local-preference increment 50")
	if def.delta != "local-preference +50" {
		t.Fatalf("delta = %q, want %q", def.delta, "local-preference +50")
	}

	def = parseOne(t, "DEC-MED", map[string]any{"decrement": map[string]any{"med": float64(30)}})
	wantSets(t, def.sets, "med decrement 30")
	if def.delta != "med -30" {
		t.Fatalf("delta = %q, want %q", def.delta, "med -30")
	}

	def = parseOne(t, "INC-AIGP", map[string]any{"increment": map[string]any{"aigp": float64(100)}})
	wantSets(t, def.sets, "aigp increment 100")

	_, err := parseModifyDefs(map[string]any{"modify": map[string]any{"BAD": map[string]any{
		"set":       map[string]any{"local-preference": float64(200)},
		"increment": map[string]any{"local-preference": float64(50)},
	}}})
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("set and increment = %v, want a conflict error", err)
	}
	for _, zero := range []string{"increment", "decrement"} {
		_, err := parseModifyDefs(map[string]any{"modify": map[string]any{"BAD": map[string]any{
			zero: map[string]any{"med": float64(0)},
		}}})
		if err == nil {
			t.Fatalf("%s of 0 was accepted", zero)
		}
	}
}

// VALIDATES: AC-4, AC-6 -- each community value of an add or a remove leaf is
// one set of the answer, in the field the leaf names; an invalid value is
// refused at parse time.
func TestParseModifyDefsCommunityOps(t *testing.T) {
	def := parseOne(t, "TAG", map[string]any{"set": map[string]any{
		"local-preference":          float64(200),
		"community-add":             []any{"65000:100", "65000:200"},
		"community-remove":          []any{"65000:1"},
		"large-community-add":       []any{"65000:100:200"},
		"extended-community-remove": []any{"target:65000:7"},
	}})
	wantSets(t, def.sets,
		"local-preference set 200",
		`community add "65000:100"`, `community add "65000:200"`, `community remove "65000:1"`,
		`large-community add "65000:100:200"`, `extended-community remove "target:65000:7"`)
	want := "local-preference 200 community-add 65000:100 65000:200 community-remove 65000:1 " +
		"large-community-add 65000:100:200 extended-community-remove target:65000:7"
	if def.delta != want {
		t.Fatalf("delta = %q, want %q", def.delta, want)
	}

	for leaf, bad := range map[string]string{
		"community-add": "not-a-community", "large-community-add": "1:2:4294967296",
		"extended-community-add": "target:65000:4294967296",
	} {
		_, err := parseModifyDefs(map[string]any{"modify": map[string]any{"BAD": map[string]any{
			"set": map[string]any{leaf: []any{bad}},
		}}})
		if err == nil {
			t.Fatalf("%s %q was accepted", leaf, bad)
		}
	}
}

// VALIDATES: spec-rfc4271-med-across-as AC-5 -- the public del { med; }
// configuration for RFC 4271 Section 5.1.4's configured removal parses into
// the med remove set, and refuses to be combined with the opposite
// instruction about the same attribute.
//
// RFC requirement: RFC4271-5.1.4-4 positive -- "A BGP speaker MUST implement a
// mechanism (based on local configuration) that allows the MULTI_EXIT_DISC
// attribute to be removed from a route" (Section 5.1.4). The configuration is
// policy { modify NAME { del { med; } } }, and a definition that states
// nothing else is a complete definition whose import answer is the med remove
// set {"field":"med","action":"remove","value":null}.
// RFC requirement: RFC4271-5.1.4-4 negative -- the mechanism is opt-in. A
// definition without del answers no med remove set.
func TestParseModifyDefsMEDRemove(t *testing.T) {
	parseDef := func(defMap map[string]any) (*modifyDef, error) {
		defs, err := parseModifyDefs(map[string]any{"modify": map[string]any{"DROP-MED": defMap}})
		if err != nil {
			return nil, err
		}
		return defs["DROP-MED"], nil
	}

	got, err := parseDef(map[string]any{"del": map[string]any{"med": true}})
	if err != nil {
		t.Fatalf("del med: unexpected error: %v", err)
	}
	if !got.medRemove {
		t.Error("del med must set the removal")
	}
	wantSets(t, got.importSets, "med remove null")
	wantSets(t, got.sets)

	got, err = parseDef(map[string]any{"set": map[string]any{"local-preference": float64(200)}})
	if err != nil {
		t.Fatalf("definition without del: unexpected error: %v", err)
	}
	if got.medRemove || len(got.importSets) != 0 {
		t.Errorf("a definition without del must not remove MED: %+v", got.importSets)
	}
	wantSets(t, got.sets, "local-preference set 200")

	for _, invalid := range []struct {
		name string
		key  string
		def  map[string]any
	}{
		{"legacy_set_key", "med-remove", map[string]any{"set": map[string]any{"med-remove": true}}},
		{"unknown_del_key", "origin", map[string]any{"del": map[string]any{"origin": true}}},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			_, err := parseDef(invalid.def)
			if err == nil {
				t.Fatalf("expected %s to be rejected", invalid.name)
			}
			if !strings.Contains(err.Error(), "unknown key") || !strings.Contains(err.Error(), invalid.key) {
				t.Errorf("error %q does not identify unknown key %q", err.Error(), invalid.key)
			}
		})
	}

	// Removing the attribute and writing one are opposite instructions, and
	// the answer would carry both with no precedence between them.
	for _, conflict := range []map[string]any{
		{"del": map[string]any{"med": true}, "set": map[string]any{"med": float64(50)}},
		{"del": map[string]any{"med": true}, "increment": map[string]any{"med": float64(10)}},
		{"del": map[string]any{"med": true}, "decrement": map[string]any{"med": float64(10)}},
	} {
		_, err := parseDef(conflict)
		if err == nil {
			t.Fatalf("expected a conflict error for %v", conflict)
		}
		if !strings.Contains(err.Error(), "conflicts") {
			t.Errorf("error %q does not mention conflicts", err.Error())
		}
	}
}

// VALIDATES: spec-rfc4271-med-across-as AC-6 -- the configured removal is
// answered on an import chain and withheld on an export chain.
//
// RFC requirement: RFC4271-5.1.4-2 positive -- "If a BGP speaker is configured
// to remove the MULTI_EXIT_DISC attribute from a route, then this removal MUST
// be done prior to determining the degree of preference of the route and prior
// to performing route selection (Decision Process phases 1 and 2)" (Section
// 5.1.4). The import chain runs before those phases; the export chain runs
// after them, so the med remove set is answered for the first and withheld
// from the second.
// RFC requirement: RFC4271-5.1.4-2 negative -- withholding it is what keeps RFC
// 4271 Section 9.1.2.2 answerable, which says that including the
// MULTI_EXIT_DISC of an EBGP-learned route in the comparison with an
// IBGP-learned route, then removing the attribute and advertising the route,
// has been proven to cause route loops.
func TestHandleFilterUpdateMEDRemoveIsImportOnly(t *testing.T) {
	defs := map[string]*modifyDef{
		"DROP-MED": parseOne(t, "DROP-MED", map[string]any{"del": map[string]any{"med": true}}),
		"DROP-AND-LP": parseOne(t, "DROP-AND-LP", map[string]any{
			"del": map[string]any{"med": true}, "set": map[string]any{"local-preference": float64(200)},
		}),
	}
	for _, tt := range []struct {
		name, filter, direction string
		action                  sdk.FilterAction
		sets                    []string
	}{
		{"import_alone", "DROP-MED", "import", sdk.FilterModify, []string{"med remove null"}},
		{"import_with_a_set", "DROP-AND-LP", "import", sdk.FilterModify,
			[]string{"local-preference set 200", "med remove null"}},
		{"export_refused", "DROP-MED", "export", sdk.FilterAccept, nil},
		{"export_keeps_the_rest", "DROP-AND-LP", "export", sdk.FilterModify, []string{"local-preference set 200"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := evaluate(t, defs, modifyRoute(tt.filter, tt.direction, `{}`))
			if err != nil {
				t.Fatal(err)
			}
			if out.Action != tt.action {
				t.Fatalf("action = %s, want %s", out.Action, tt.action)
			}
			wantSets(t, out.Sets, tt.sets...)
		})
	}
}

// VALIDATES: the evaluator honors the fast-path contract on both transports
// (FR-9, AC-36): no blocking, the same answer twice, no view kept past the
// call, equal answers on the slot and over the batch, concurrent use.
// PREVENTS: an evaluator that is only correct when the bridge loop serialized it.
func TestModifyConformance(t *testing.T) {
	defs := map[string]*modifyDef{
		"LP":       parseOne(t, "LP", map[string]any{"set": map[string]any{"local-preference": float64(200)}}),
		"DROP-MED": parseOne(t, "DROP-MED", map[string]any{"del": map[string]any{"med": true}}),
		"RTBH": parseOne(t, "RTBH", map[string]any{
			"match": map[string]any{"community": "65001:666"},
			"set":   map[string]any{"next-hop": "192.0.2.1"},
		}),
	}
	previous := defsByName.Load()
	t.Cleanup(func() { defsByName.Store(previous) })
	defsByName.Store(&defs)
	route := func(filter, direction, fields string) sdk.ConformanceRoute {
		return sdk.ConformanceRoute{Input: rpc.FilterUpdateInput{
			Filter: filter, Direction: direction, Peer: "10.0.0.1", Fields: json.RawMessage(fields),
		}}
	}
	sdk.CheckFilterConformance(t, "modify", evaluateModify, []sdk.ConformanceRoute{
		route("LP", "import", `{}`),
		route("DROP-MED", "import", `{}`),
		route("DROP-MED", "export", `{}`),
		route("RTBH", "import", `{"community":["65001:666"]}`),
		route("RTBH", "import", `{"community":["65001:667"]}`),
	})
}
