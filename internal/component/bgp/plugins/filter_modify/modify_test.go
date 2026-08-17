package filter_modify

import (
	"strings"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: AC-1 -- local-preference set in delta.
// VALIDATES: AC-2 -- med set in delta.
// VALIDATES: AC-3 -- origin set in delta.
// VALIDATES: AC-4 -- next-hop set in delta.
// VALIDATES: AC-7 -- only declared attributes in delta.
// VALIDATES: AC-8 -- multiple attributes in one modifier.
// PREVENTS: Wrong delta format; missing attributes; extra attributes.
func TestBuildDelta(t *testing.T) {
	tests := []struct {
		name     string
		setBlock map[string]any
		want     string
	}{
		{
			name:     "local_preference_only",
			setBlock: map[string]any{"local-preference": float64(200)},
			want:     "local-preference 200",
		},
		{
			name:     "med_only",
			setBlock: map[string]any{"med": float64(50)},
			want:     "med 50",
		},
		{
			name:     "origin_igp",
			setBlock: map[string]any{"origin": "igp"},
			want:     "origin igp",
		},
		{
			name:     "origin_incomplete",
			setBlock: map[string]any{"origin": "incomplete"},
			want:     "origin incomplete",
		},
		{
			name:     "next_hop",
			setBlock: map[string]any{"next-hop": "10.0.0.1"},
			want:     "next-hop 10.0.0.1",
		},
		{
			name: "multiple_attributes",
			setBlock: map[string]any{
				"local-preference": float64(200),
				"med":              float64(50),
				"origin":           "igp",
			},
			want: "local-preference 200 med 50 origin igp",
		},
		{
			name:     "empty_set_block",
			setBlock: map[string]any{},
			want:     "",
		},
		{
			name:     "nil_values_ignored",
			setBlock: map[string]any{"local-preference": nil, "med": nil},
			want:     "",
		},
		{
			name:     "string_numeric",
			setBlock: map[string]any{"local-preference": "300"},
			want:     "local-preference 300",
		},
		{
			name:     "local_preference_zero",
			setBlock: map[string]any{"local-preference": float64(0)},
			want:     "local-preference 0",
		},
		{
			name:     "local_preference_max",
			setBlock: map[string]any{"local-preference": float64(4294967295)},
			want:     "local-preference 4294967295",
		},
		{
			name:     "med_max",
			setBlock: map[string]any{"med": float64(4294967295)},
			want:     "med 4294967295",
		},
		{
			name:     "as_path_prepend",
			setBlock: map[string]any{"as-path-prepend": float64(3)},
			want:     "as-path-prepend 3",
		},
		{
			name:     "multiple_with_prepend",
			setBlock: map[string]any{"local-preference": float64(200), "as-path-prepend": float64(2)},
			want:     "local-preference 200 as-path-prepend 2",
		},
		{
			name:     "prepend_zero_ignored",
			setBlock: map[string]any{"as-path-prepend": float64(0)},
			want:     "",
		},
		{
			name:     "prepend_over_32_ignored",
			setBlock: map[string]any{"as-path-prepend": float64(33)},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDelta(tt.setBlock)
			if got != tt.want {
				t.Errorf("buildDelta() = %q, want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: Config parsing for modify definitions.
// PREVENTS: Invalid config silently accepted.
func TestParseModifyDefs(t *testing.T) {
	tests := []struct {
		name      string
		bgpCfg    map[string]any
		wantCount int
		wantDelta string // for single-def tests
		wantErr   bool
		errSubstr string
	}{
		{
			name: "single_def",
			bgpCfg: map[string]any{
				"policy": map[string]any{
					"modify": map[string]any{
						"PREFER-LOCAL": map[string]any{
							"set": map[string]any{
								"local-preference": float64(200),
							},
						},
					},
				},
			},
			wantCount: 1,
			wantDelta: "local-preference 200",
		},
		{
			name: "multiple_defs",
			bgpCfg: map[string]any{
				"policy": map[string]any{
					"modify": map[string]any{
						"A": map[string]any{"set": map[string]any{"local-preference": float64(100)}},
						"B": map[string]any{"set": map[string]any{"med": float64(50)}},
					},
				},
			},
			wantCount: 2,
		},
		{
			name:      "no_policy",
			bgpCfg:    map[string]any{},
			wantCount: 0,
		},
		{
			name: "no_operations",
			bgpCfg: map[string]any{
				"policy": map[string]any{
					"modify": map[string]any{
						"BAD": map[string]any{},
					},
				},
			},
			wantErr:   true,
			errSubstr: "no operations defined",
		},
		{
			name: "empty_set_no_other_ops",
			bgpCfg: map[string]any{
				"policy": map[string]any{
					"modify": map[string]any{
						"BAD": map[string]any{
							"set": map[string]any{},
						},
					},
				},
			},
			wantErr:   true,
			errSubstr: "no operations defined",
		},
		{
			name: "name_too_long",
			bgpCfg: map[string]any{
				"policy": map[string]any{
					"modify": map[string]any{
						strings.Repeat("x", maxNameLen+1): map[string]any{
							"set": map[string]any{"local-preference": float64(100)},
						},
					},
				},
			},
			wantErr:   true,
			errSubstr: "exceeds maximum length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defs, err := parseModifyDefs(tt.bgpCfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(defs) != tt.wantCount {
				t.Fatalf("expected %d defs, got %d", tt.wantCount, len(defs))
			}
			if tt.wantDelta != "" {
				for _, def := range defs {
					if def.delta != tt.wantDelta {
						t.Errorf("delta = %q, want %q", def.delta, tt.wantDelta)
					}
				}
			}
		})
	}
}

// VALIDATES: handleFilterUpdate returns modify with pre-built delta.
// PREVENTS: Wrong action or missing delta.
func TestHandleFilterUpdate(t *testing.T) {
	defs := map[string]*modifyDef{
		"PREFER-LOCAL": {name: "PREFER-LOCAL", delta: "local-preference 200"},
	}
	defsByName.Store(&defs)
	defer defsByName.Store(nil)

	tests := []struct {
		name       string
		filterName string
		wantAction sdk.FilterAction
		wantDelta  string
	}{
		{
			name:       "known_modifier",
			filterName: "PREFER-LOCAL",
			wantAction: sdk.FilterModify,
			wantDelta:  "local-preference 200",
		},
		{
			name:       "unknown_modifier",
			filterName: "NONEXISTENT",
			wantAction: sdk.FilterReject,
			wantDelta:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := &sdk.FilterUpdateInput{
				Filter: tt.filterName,
				Peer:   "127.0.0.1",
				Update: "origin igp as-path 65001 nlri ipv4/unicast add 10.0.0.0/24",
			}
			out := handleFilterUpdate(in)
			if out.Action != tt.wantAction {
				t.Errorf("action = %s, want %s", out.Action, tt.wantAction)
			}
			if out.Update != tt.wantDelta {
				t.Errorf("delta = %q, want %q", out.Update, tt.wantDelta)
			}
		})
	}
}

// VALIDATES: AC-1 -- increment local-preference produces correct absolute value.
// VALIDATES: AC-3 -- increment saturates at uint32 max.
func TestBuildDynamicDeltaIncrement(t *testing.T) {
	tests := []struct {
		name       string
		def        *modifyDef
		updateText string
		wantPart   string
	}{
		{
			name:       "increment_lp_100_by_50",
			def:        &modifyDef{increments: []incdec{{attr: "local-preference", value: 50}}},
			updateText: "origin igp local-preference 100 as-path 65001",
			wantPart:   "local-preference 150",
		},
		{
			name:       "increment_med_by_10",
			def:        &modifyDef{increments: []incdec{{attr: "med", value: 10}}},
			updateText: "origin igp med 90 as-path 65001",
			wantPart:   "med 100",
		},
		{
			name:       "increment_saturates_at_max",
			def:        &modifyDef{increments: []incdec{{attr: "local-preference", value: 50}}},
			updateText: "origin igp local-preference 4294967280 as-path 65001",
			wantPart:   "local-preference 4294967295",
		},
		{
			name:       "increment_missing_attr_treats_as_zero",
			def:        &modifyDef{increments: []incdec{{attr: "local-preference", value: 50}}},
			updateText: "origin igp as-path 65001",
			wantPart:   "local-preference 50",
		},
		{
			name:       "increment_aigp",
			def:        &modifyDef{increments: []incdec{{attr: "aigp", value: 100}}},
			updateText: "origin igp aigp 500 as-path 65001",
			wantPart:   "aigp 600",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDynamicDelta(tt.def, tt.updateText)
			if !strings.Contains(got, tt.wantPart) {
				t.Errorf("buildDynamicDelta() = %q, want to contain %q", got, tt.wantPart)
			}
		})
	}
}

// VALIDATES: AC-2 -- decrement floors at zero.
func TestBuildDynamicDeltaDecrement(t *testing.T) {
	tests := []struct {
		name       string
		def        *modifyDef
		updateText string
		wantPart   string
	}{
		{
			name:       "decrement_med_90_by_30",
			def:        &modifyDef{decrements: []incdec{{attr: "med", value: 30}}},
			updateText: "origin igp med 90 as-path 65001",
			wantPart:   "med 60",
		},
		{
			name:       "decrement_floors_at_zero",
			def:        &modifyDef{decrements: []incdec{{attr: "med", value: 30}}},
			updateText: "origin igp med 20 as-path 65001",
			wantPart:   "med 0",
		},
		{
			name:       "decrement_exact_zero",
			def:        &modifyDef{decrements: []incdec{{attr: "med", value: 50}}},
			updateText: "origin igp med 50 as-path 65001",
			wantPart:   "med 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDynamicDelta(tt.def, tt.updateText)
			if !strings.Contains(got, tt.wantPart) {
				t.Errorf("buildDynamicDelta() = %q, want to contain %q", got, tt.wantPart)
			}
		})
	}
}

// VALIDATES: AC-4 -- community-add directive in delta.
// VALIDATES: AC-6 -- large-community-add directive in delta.
func TestBuildDynamicDeltaCommunityOps(t *testing.T) {
	tests := []struct {
		name     string
		def      *modifyDef
		wantPart string
	}{
		{
			name: "community_add",
			def: &modifyDef{commOps: []commOp{
				{directive: "community-add", values: "65000:200"},
			}},
			wantPart: "community-add 65000:200",
		},
		{
			name: "community_remove",
			def: &modifyDef{commOps: []commOp{
				{directive: "community-remove", values: "65000:100"},
			}},
			wantPart: "community-remove 65000:100",
		},
		{
			name: "large_community_add",
			def: &modifyDef{commOps: []commOp{
				{directive: "large-community-add", values: "65000:100:200"},
			}},
			wantPart: "large-community-add 65000:100:200",
		},
		{
			name: "combined_static_and_dynamic",
			def: &modifyDef{
				delta: "local-preference 200",
				commOps: []commOp{
					{directive: "community-add", values: "65000:200"},
				},
			},
			wantPart: "community-add 65000:200",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDynamicDelta(tt.def, "origin igp as-path 65001")
			if !strings.Contains(got, tt.wantPart) {
				t.Errorf("buildDynamicDelta() = %q, want to contain %q", got, tt.wantPart)
			}
		})
	}
}

// VALIDATES: AC-13 -- set and increment for same attr rejected at parse time.
func TestParseModifyDefsConflict(t *testing.T) {
	bgpCfg := map[string]any{
		"policy": map[string]any{
			"modify": map[string]any{
				"BAD": map[string]any{
					"set":       map[string]any{"local-preference": float64(200)},
					"increment": map[string]any{"local-preference": float64(50)},
				},
			},
		},
	}
	_, err := parseModifyDefs(bgpCfg)
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "conflicts") {
		t.Errorf("error %q does not mention conflicts", err.Error())
	}
}

// VALIDATES: increment config parsing accepts valid input.
func TestParseModifyDefsIncrement(t *testing.T) {
	bgpCfg := map[string]any{
		"policy": map[string]any{
			"modify": map[string]any{
				"INC-LP": map[string]any{
					"increment": map[string]any{"local-preference": float64(50)},
				},
			},
		},
	}
	defs, err := parseModifyDefs(bgpCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	def := defs["INC-LP"]
	if len(def.increments) != 1 {
		t.Fatalf("expected 1 increment, got %d", len(def.increments))
	}
	if def.increments[0].attr != "local-preference" || def.increments[0].value != 50 {
		t.Errorf("increment = %+v, want local-preference 50", def.increments[0])
	}
}

// VALIDATES: community-add config parsing.
func TestParseModifyDefsCommunityAdd(t *testing.T) {
	bgpCfg := map[string]any{
		"policy": map[string]any{
			"modify": map[string]any{
				"TAG": map[string]any{
					"set": map[string]any{
						"community-add": []any{"65000:100", "65000:200"},
					},
				},
			},
		},
	}
	defs, err := parseModifyDefs(bgpCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	def := defs["TAG"]
	if len(def.commOps) != 1 {
		t.Fatalf("expected 1 commOp, got %d", len(def.commOps))
	}
	if def.commOps[0].directive != "community-add" {
		t.Errorf("directive = %q, want community-add", def.commOps[0].directive)
	}
	if def.commOps[0].values != "65000:100 65000:200" {
		t.Errorf("values = %q, want %q", def.commOps[0].values, "65000:100 65000:200")
	}
}

// VALIDATES: handleFilterUpdate with dynamic increment modifier.
func TestHandleFilterUpdateIncrement(t *testing.T) {
	defs := map[string]*modifyDef{
		"INC-LP": {
			name:       "INC-LP",
			increments: []incdec{{attr: "local-preference", value: 50}},
		},
	}
	defsByName.Store(&defs)
	defer defsByName.Store(nil)

	in := &sdk.FilterUpdateInput{
		Filter: "INC-LP",
		Peer:   "127.0.0.1",
		Update: "origin igp local-preference 100 as-path 65001 nlri ipv4/unicast add 10.0.0.0/24",
	}
	out := handleFilterUpdate(in)
	if out.Action != sdk.FilterModify {
		t.Fatalf("action = %v, want modify", out.Action)
	}
	if !strings.Contains(out.Update, "local-preference 150") {
		t.Errorf("delta = %q, want to contain 'local-preference 150'", out.Update)
	}
}

func TestExtractUint32Attr(t *testing.T) {
	tests := []struct {
		name       string
		updateText string
		attr       string
		want       uint32
	}{
		{"present", "origin igp local-preference 100 as-path 65001", "local-preference", 100},
		{"absent", "origin igp as-path 65001", "local-preference", 0},
		{"at_end", "origin igp med 50", "med", 50},
		{"zero_value", "origin igp local-preference 0 as-path 65001", "local-preference", 0},
		{"max_value", "origin igp local-preference 4294967295 as-path 65001", "local-preference", 4294967295},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractUint32Attr(tt.updateText, tt.attr)
			if got != tt.want {
				t.Errorf("extractUint32Attr(%q, %q) = %d, want %d", tt.updateText, tt.attr, got, tt.want)
			}
		})
	}
}

// rfc-test-change-approved: 2026-08-17 Thomas approved replacing tests of the
// removed, unreleased set { med-remove true; } syntax with del { med; }.
//
// VALIDATES: spec-rfc4271-med-across-as AC-5 -- the public del { med; }
// configuration for RFC 4271 Section 5.1.4's configured removal parses, and
// refuses to be combined with the opposite instruction about the same
// attribute.
//
// RFC requirement: RFC4271-5.1.4-4 positive -- "A BGP speaker MUST implement a
// mechanism (based on local configuration) that allows the MULTI_EXIT_DISC
// attribute to be removed from a route" (Section 5.1.4). The configuration is
// bgp { policy { modify NAME { del { med; } } } }, and a definition that states
// nothing else is a complete definition.
// RFC requirement: RFC4271-5.1.4-4 negative -- the mechanism is opt-in. A
// definition without del leaves the metric alone.
func TestParseModifyDefsMEDRemove(t *testing.T) {
	parseDef := func(defMap map[string]any) (*modifyDef, error) {
		defs, err := parseModifyDefs(map[string]any{
			"policy": map[string]any{"modify": map[string]any{"DROP-MED": defMap}},
		})
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

	got, err = parseDef(map[string]any{"set": map[string]any{"local-preference": float64(200)}})
	if err != nil {
		t.Fatalf("definition without del: unexpected error: %v", err)
	}
	if got.medRemove {
		t.Error("a definition without del must not remove MED")
	}

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

	// Removing the attribute and writing one are opposite instructions, and the
	// delta text records neither's precedence over the other.
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
// emitted on an import chain and refused on an export chain.
//
// RFC requirement: RFC4271-5.1.4-2 positive -- "If a BGP speaker is configured
// to remove the MULTI_EXIT_DISC attribute from a route, then this removal MUST
// be done prior to determining the degree of preference of the route and prior
// to performing route selection (Decision Process phases 1 and 2)" (Section
// 5.1.4). The import chain runs before those phases; the export chain runs
// after them, so the directive is emitted for the first and withheld from the
// second.
// RFC requirement: RFC4271-5.1.4-2 negative -- withholding it is what keeps RFC
// 4271 Section 9.1.2.2 answerable, which says that including the
// MULTI_EXIT_DISC of an EBGP-learned route in the comparison with an
// IBGP-learned route, then removing the attribute and advertising the route,
// has been proven to cause route loops.
func TestHandleFilterUpdateMEDRemoveIsImportOnly(t *testing.T) {
	defs := map[string]*modifyDef{
		"DROP-MED":    {name: "DROP-MED", medRemove: true},
		"DROP-AND-LP": {name: "DROP-AND-LP", delta: "local-preference 200", medRemove: true},
	}
	defsByName.Store(&defs)
	defer defsByName.Store(nil)

	for _, tt := range []struct {
		name      string
		filter    string
		direction string
		want      string
	}{
		{"import_alone", "DROP-MED", "import", "med-remove"},
		{"import_with_a_set", "DROP-AND-LP", "import", "local-preference 200 med-remove"},
		{"export_refused", "DROP-MED", "export", ""},
		{"export_keeps_the_rest", "DROP-AND-LP", "export", "local-preference 200"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out := handleFilterUpdate(&sdk.FilterUpdateInput{
				Filter:    tt.filter,
				Direction: tt.direction,
				Peer:      "127.0.0.1",
				Update:    "origin igp med 100 as-path 65001 nlri ipv4/unicast add 10.0.0.0/24",
			})
			if out.Action != sdk.FilterModify {
				t.Fatalf("action = %s, want %s", out.Action, sdk.FilterModify)
			}
			if out.Update != tt.want {
				t.Errorf("delta = %q, want %q", out.Update, tt.want)
			}
		})
	}
}
