package filter_modify

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
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
			// Absent LOCAL_PREF starts from the declared base of 100, not from
			// 0. RFC 4271 Section 9.1.1 supplies no number, so this is local
			// policy, and 100 is what FRR and BIRD use. Until 2026-09-04 this
			// case expected 50, which was the zero reached by a failed lookup
			// rather than a value anybody chose.
			name:       "increment_absent_lp_starts_from_the_declared_base",
			def:        &modifyDef{increments: []incdec{{attr: "local-preference", value: 50}}},
			updateText: "origin igp as-path 65001",
			wantPart:   "local-preference 150",
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

// TestReadUint32AttrReportsAbsenceDistinctly covers AC-17. The reading this
// function returns is what separates a route carrying 0 from a route carrying
// nothing, and the old signature returned 0 for both.
func TestReadUint32AttrReportsAbsenceDistinctly(t *testing.T) {
	tests := []struct {
		name        string
		updateText  string
		attr        string
		wantValue   uint32
		wantReading attrReading
	}{
		{"present", "origin igp local-preference 100 as-path 65001", localPreferenceAttr, 100, attrPresent},
		{"absent", "origin igp as-path 65001", localPreferenceAttr, 0, attrAbsent},
		{"at_end", "origin igp med 50", medAttr, 50, attrPresent},
		{"a real zero is not an absence", "origin igp local-preference 0 as-path 65001", localPreferenceAttr, 0, attrPresent},
		{"max_value", "origin igp local-preference 4294967295 as-path 65001", localPreferenceAttr, 4294967295, attrPresent},
		{"wider than uint32 is not an absence", "origin igp aigp 4294967296 nlri x", aigpAttr, 0, attrUnreadable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reading := readUint32Attr(tt.updateText, tt.attr)
			require.Equal(t, tt.wantReading, reading, "the reading")
			require.Equal(t, tt.wantValue, got, "the value")
		})
	}
}

// TestDecrementMedOnAbsentAttributeMaterialisesZero covers AC-14. RFC 4271
// Section 9.1.2.2 supplies the 0 the subtraction starts from, the result floors,
// and the pair IS written, so the route gains a metric it did not carry. FRR
// reaches the same route: route_set_metric defaults to 0 on the presence flag
// and calls bgp_attr_set_med unconditionally.
func TestDecrementMedOnAbsentAttributeMaterialisesZero(t *testing.T) {
	def := &modifyDef{name: "DEC-MED", decrements: []incdec{{attr: medAttr, value: 30}}}
	got := buildDynamicDelta(def, "origin igp as-path 65001 nlri ipv4/unicast add 10.0.0.0/24")
	require.Equal(t, "med 0", got, "the RFC default floors, and the attribute is written")
}

// TestDecrementMedOnAPresentAttributeSubtracts covers AC-5, and is the case the
// renderer fix made reachable: before it the subject never named med, so this
// computed from 0 and answered "med 0" for every route.
func TestDecrementMedOnAPresentAttributeSubtracts(t *testing.T) {
	def := &modifyDef{name: "DEC-MED", decrements: []incdec{{attr: medAttr, value: 30}}}
	got := buildDynamicDelta(def, "origin igp med 100 as-path 65001 nlri ipv4/unicast add 10.0.0.0/24")
	require.Equal(t, "med 70", got)
}

// TestIncrementLocalPrefOnAbsentAttributeStartsFromTheDefault covers AC-15.
// RFC 4271 Section 9.1.1 supplies no number, so 100 is Ze's declared local
// policy, matching FRR's BGP_DEFAULT_LOCAL_PREF and BIRD's default_local_pref.
func TestIncrementLocalPrefOnAbsentAttributeStartsFromTheDefault(t *testing.T) {
	def := &modifyDef{name: "INC-LP", increments: []incdec{{attr: localPreferenceAttr, value: 50}}}
	got := buildDynamicDelta(def, "origin igp as-path 65001 nlri ipv4/unicast add 10.0.0.0/24")
	require.Equal(t, "local-preference 150", got, "100 plus 50")
}

// TestAigpArithmeticOnAbsentAttributeCreatesNothing covers AC-16, which is a
// conformance requirement rather than a preference. RFC 7311 Section 3.4.1: "A
// BGP speaker MUST NOT add the AIGP attribute to any route whose path leads
// outside the AIGP administrative domain to which the BGP speaker belongs."
// Section 4.1 is why no number could stand in for the absence either: a route
// with no AIGP TLV is removed from consideration rather than scored, so a
// substituted 0 would make it best where the RFC makes it lose.
func TestAigpArithmeticOnAbsentAttributeCreatesNothing(t *testing.T) {
	text := "origin igp as-path 65001 nlri ipv4/unicast add 10.0.0.0/24"

	inc := &modifyDef{name: "INC-AIGP", increments: []incdec{{attr: aigpAttr, value: 50}}}
	require.Empty(t, buildDynamicDelta(inc, text), "increment must not create an AIGP attribute")

	dec := &modifyDef{name: "DEC-AIGP", decrements: []incdec{{attr: aigpAttr, value: 30}}}
	require.Empty(t, buildDynamicDelta(dec, text), "decrement must not create one either")

	// A route that DOES carry one is arithmetic as usual.
	carried := "origin igp aigp 42 as-path 65001 nlri ipv4/unicast add 10.0.0.0/24"
	require.Equal(t, "aigp 92", buildDynamicDelta(inc, carried))
}

// TestAbsentValueTableCoversEveryArithmeticAttribute covers AC-18. Every leaf
// the YANG increment and decrement containers accept is named here with the
// decision taken for it, so a fourth leaf cannot arrive and silently inherit a
// zero nobody chose: it fails this test until somebody decides.
func TestAbsentValueTableCoversEveryArithmeticAttribute(t *testing.T) {
	decided := map[string]bool{
		localPreferenceAttr: true,  // base declared: 100
		medAttr:             true,  // base declared: 0
		aigpAttr:            false, // deliberately no base, RFC 7311 Section 3.4.1
	}

	declaredBase, err := parseAttributeDefaults(nil)
	require.NoError(t, err, "the schema declares the bases and must be readable")

	for attr, wantBase := range decided {
		_, declared := declaredBase[attr]
		require.Equalf(t, wantBase, declared, "%s: the table and the decision disagree", attr)

		_, run := currentForArithmetic("origin igp nlri ipv4/unicast add 10.0.0.0/24", attr)
		require.Equalf(t, wantBase, run, "%s: arithmetic on an absent attribute", attr)
	}

	require.Len(t, declaredBase, 2, "a new leaf under bgp/defaults/attribute needs a row in this test's decision map")

	_, run := currentForArithmetic("origin igp nlri ipv4/unicast add 10.0.0.0/24", "no-such-attribute")
	require.False(t, run, "an attribute with no declared base gets no arithmetic")
}

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

// medSubjectAttr wraps one attribute value in its wire header, which is what
// the parsers in knownAttrParsers read.
func medSubjectAttr(flags attribute.AttributeFlags, code attribute.AttributeCode, value []byte) []byte {
	buf := make([]byte, 3+len(value))
	attribute.WriteHeaderTo(buf, 0, flags, code, uint16(len(value))) //nolint:gosec // test data, bounded
	copy(buf[3:], value)
	return buf
}

// medSubjectFromWire renders the filter subject the reactor hands this plugin
// for a route carrying MULTI_EXIT_DISC, through the producer itself.
//
// The arithmetic below is measured against the text the daemon really emits. A
// hand-written subject cannot see a renderer that names no metric, and that is
// exactly the defect this spec repairs: appendSingleAttr named a pointer type
// no parser builds, so every med increment computed from an absent attribute.
func medSubjectFromWire(t *testing.T, med uint32) string {
	t.Helper()

	value := make([]byte, 4)
	binary.BigEndian.PutUint32(value, med)

	packed := medSubjectAttr(attribute.FlagTransitive, attribute.AttrOrigin, []byte{0})
	packed = append(packed, medSubjectAttr(attribute.FlagTransitive, attribute.AttrASPath,
		[]byte{byte(attribute.ASSequence), 1, 0x00, 0x00, 0xFD, 0xE9})...)
	packed = append(packed, medSubjectAttr(attribute.FlagTransitive, attribute.AttrNextHop,
		[]byte{10, 0, 0, 1})...)
	packed = append(packed, medSubjectAttr(attribute.FlagOptional, attribute.AttrMED, value)...)

	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)

	subject := string(reactor.AppendUpdateForFilter(nil,
		attribute.NewAttributesWire(packed, ctxID), nil, nil))
	require.Contains(t, subject, "med ",
		"the fixture is only a fixture if the renderer names the metric")
	return subject
}

// TestIncrementMedComputesFromTheRouteValue covers AC-4. The route carries 100,
// the operator asked for 50 more, and the delta says 150.
//
// The subject comes from AppendUpdateForFilter rather than from a string typed
// here, so the test fails when the renderer stops naming the metric. With the
// five pointer arms in place the subject named no med, currentForArithmetic
// took the absent base of 0, and the same configuration answered "med 50".
func TestIncrementMedComputesFromTheRouteValue(t *testing.T) {
	defs := map[string]*modifyDef{
		"INC-MED": {name: "INC-MED", increments: []incdec{{attr: medAttr, value: 50}}},
	}
	defsByName.Store(&defs)
	defer defsByName.Store(nil)

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter:    "INC-MED",
		Direction: "import",
		Peer:      "10.0.0.1",
		PeerAS:    65001,
		Update:    medSubjectFromWire(t, 100),
	})

	require.Equal(t, sdk.FilterModify, out.Action)
	require.Equal(t, "med 150", out.Update, "100 from the route plus the configured 50")
}

// TestDecrementMedComputesFromTheRouteValue covers AC-5, the same route through
// the opposite operation: 100 less 30 is 70.
//
// TestDecrementMedOnAPresentAttributeSubtracts asserts the same arithmetic over
// buildDynamicDelta from a hand-written subject. This one drives the plugin's
// entry point on the text the reactor produced, so it also fails when the
// renderer, the plugin dispatch, or the delta the plugin returns changes.
func TestDecrementMedComputesFromTheRouteValue(t *testing.T) {
	defs := map[string]*modifyDef{
		"DEC-MED": {name: "DEC-MED", decrements: []incdec{{attr: medAttr, value: 30}}},
	}
	defsByName.Store(&defs)
	defer defsByName.Store(nil)

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter:    "DEC-MED",
		Direction: "import",
		Peer:      "10.0.0.1",
		PeerAS:    65001,
		Update:    medSubjectFromWire(t, 100),
	})

	require.Equal(t, sdk.FilterModify, out.Action)
	require.Equal(t, "med 70", out.Update, "100 from the route less the configured 30")
}

// absentSubject is a route that carries neither MULTI_EXIT_DISC nor LOCAL_PREF,
// which is the case every test below is about.
const absentSubject = "origin igp as-path 65001 nlri ipv4/unicast add 10.0.0.0/24"

// configureAttributeDefaults installs one bgp { defaults { attribute { } } }
// block the way OnConfigure installs it: through parseAttributeDefaults, so the
// test drives the config path rather than a map written by hand. The previous
// base is restored, because the store is package state several tests share.
func configureAttributeDefaults(t *testing.T, attributeBlock map[string]any) {
	t.Helper()
	base, err := parseAttributeDefaults(map[string]any{
		"defaults": map[string]any{"attribute": attributeBlock},
	})
	require.NoError(t, err)
	storeAbsentBase(t, &base)
}

// storeAbsentBase installs a base for one test and restores what was there.
//
// The parameter is a POINTER because the store is one, and because nil is a
// state a test needs: it is what the plugin holds before the first configure.
//
//nolint:gocritic // ptrToRefParam: nil is the pre-configure state under test
func storeAbsentBase(t *testing.T, base *map[string]uint32) {
	t.Helper()
	previous := absentBase.Load()
	t.Cleanup(func() { absentBase.Store(previous) })
	absentBase.Store(base)
}

// TestAbsentBaseComesFromConfig covers AC-10: the two numbers are declared once,
// in ze-bgp-conf.yang, and the plugin keeps no copy of either.
//
// The second half is what makes the first half mean something. A plugin that
// still held the literals would answer 0 and 100 from a base that declares
// neither, and it would satisfy the schema comparison above by having been
// written to the same numbers.
func TestAbsentBaseComesFromConfig(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	base, err := parseAttributeDefaults(nil)
	require.NoError(t, err)

	for _, leaf := range []string{medAttr, localPreferenceAttr} {
		declared, err := config.SchemaDefaultInt(schema, attributeDefaultsPath+"/"+leaf)
		require.NoErrorf(t, err, "%s: the schema is the declaration and must carry a default", leaf)
		require.Equalf(t, uint32(declared), base[leaf], "%s: the base is not the schema's number", leaf) //nolint:gosec // G115: a YANG uint32 default
	}

	storeAbsentBase(t, &map[string]uint32{})
	for _, attr := range []string{medAttr, localPreferenceAttr} {
		_, run := currentForArithmetic(absentSubject, attr)
		require.Falsef(t, run, "%s: a base that declares nothing must leave no compiled-in number behind", attr)
	}
}

// TestConfiguredLocalPrefBaseFeedsTheArithmetic covers AC-2. RFC 4271 Section
// 9.1.1 calls the degree of preference "a local matter", so the number an
// operator writes is the number the arithmetic starts from.
//
// The value is the STRING form, which is what a YANG leaf is delivered as.
func TestConfiguredLocalPrefBaseFeedsTheArithmetic(t *testing.T) {
	configureAttributeDefaults(t, map[string]any{localPreferenceAttr: "80"})

	def := &modifyDef{name: "INC-LP", increments: []incdec{{attr: localPreferenceAttr, value: 50}}}
	require.Equal(t, "local-preference 130", buildDynamicDelta(def, absentSubject), "80 plus 50")
}

// TestConfiguredMedBaseFeedsTheArithmetic covers AC-3 and AC-4. The decrement
// still floors at 0, so a configured base does not let the arithmetic run under
// the smallest MULTI_EXIT_DISC the wire can carry.
//
// The value is the JSON number form, which is what the config section carries
// when it arrives as JSON rather than as a native tree.
func TestConfiguredMedBaseFeedsTheArithmetic(t *testing.T) {
	configureAttributeDefaults(t, map[string]any{medAttr: float64(50)})

	subtract30 := &modifyDef{name: "DEC-MED", decrements: []incdec{{attr: medAttr, value: 30}}}
	require.Equal(t, "med 20", buildDynamicDelta(subtract30, absentSubject), "50 less 30")

	subtract80 := &modifyDef{name: "DEC-MED-FLOOR", decrements: []incdec{{attr: medAttr, value: 80}}}
	require.Equal(t, "med 0", buildDynamicDelta(subtract80, absentSubject), "the decrement floors at 0")
}

// TestConfiguredBaseIsUnusedWhenTheRouteCarriesTheAttribute covers AC-5. A base
// answers for an attribute the route does NOT carry, and this is the case that
// tells a base that is read too widely from one that is read correctly.
func TestConfiguredBaseIsUnusedWhenTheRouteCarriesTheAttribute(t *testing.T) {
	configureAttributeDefaults(t, map[string]any{
		medAttr:             "50",
		localPreferenceAttr: "80",
	})

	carried := "origin igp med 100 local-preference 200 as-path 65001 nlri ipv4/unicast add 10.0.0.0/24"

	incMed := &modifyDef{name: "INC-MED", increments: []incdec{{attr: medAttr, value: 30}}}
	require.Equal(t, "med 130", buildDynamicDelta(incMed, carried), "100 from the route, not 50 from the config")

	incPref := &modifyDef{name: "INC-LP", increments: []incdec{{attr: localPreferenceAttr, value: 30}}}
	require.Equal(t, "local-preference 230", buildDynamicDelta(incPref, carried), "200 from the route, not 80")
}

// TestConfiguredBaseArithmeticHoldsAtTheBoundaries covers the boundary rows of
// the spec's numeric table. The base is a uint32, so the top of the range is a
// value an operator can write, and an increment on top of it saturates rather
// than wrapping to a small metric.
func TestConfiguredBaseArithmeticHoldsAtTheBoundaries(t *testing.T) {
	const uint32Max = "4294967295"

	configureAttributeDefaults(t, map[string]any{medAttr: uint32Max, localPreferenceAttr: uint32Max})

	incMed := &modifyDef{name: "INC-MED", increments: []incdec{{attr: medAttr, value: 1}}}
	require.Equal(t, "med "+uint32Max, buildDynamicDelta(incMed, absentSubject), "the sum saturates")

	decMed := &modifyDef{name: "DEC-MED", decrements: []incdec{{attr: medAttr, value: 1}}}
	require.Equal(t, "med 4294967294", buildDynamicDelta(decMed, absentSubject))

	configureAttributeDefaults(t, map[string]any{medAttr: "0", localPreferenceAttr: "0"})

	require.Equal(t, "med 1", buildDynamicDelta(incMed, absentSubject), "0 is a value an operator can write")
	require.Equal(t, "med 0", buildDynamicDelta(decMed, absentSubject), "and the decrement floors on it")
}

// TestDefaultsApplyBeforeConfigure covers AC-8 and validates assumption A-2. The
// ordering between the first filter update and the first configure delivery is
// not guaranteed, so the window is answered from the schema's own numbers.
// Nothing computes from a zero the operator did not choose.
func TestDefaultsApplyBeforeConfigure(t *testing.T) {
	storeAbsentBase(t, nil)

	incPref := &modifyDef{name: "INC-LP", increments: []incdec{{attr: localPreferenceAttr, value: 50}}}
	require.Equal(t, "local-preference 150", buildDynamicDelta(incPref, absentSubject), "100 plus 50")

	incMed := &modifyDef{name: "INC-MED", increments: []incdec{{attr: medAttr, value: 50}}}
	require.Equal(t, "med 50", buildDynamicDelta(incMed, absentSubject), "0 plus 50")

	incAigp := &modifyDef{name: "INC-AIGP", increments: []incdec{{attr: aigpAttr, value: 50}}}
	require.Empty(t, buildDynamicDelta(incAigp, absentSubject), "and aigp still gets no base")
}
