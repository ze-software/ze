package filter_modify

import (
	"strings"
	"testing"

	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
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
			name: "missing_set",
			bgpCfg: map[string]any{
				"policy": map[string]any{
					"modify": map[string]any{
						"BAD": map[string]any{},
					},
				},
			},
			wantErr:   true,
			errSubstr: "missing 'set'",
		},
		{
			name: "empty_set",
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
			errSubstr: "no attributes",
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
	// Set up a known modifier.
	defs := map[string]*modifyDef{
		"PREFER-LOCAL": {name: "PREFER-LOCAL", delta: "local-preference 200"},
	}
	defsByName.Store(&defs)
	defer defsByName.Store(nil)

	tests := []struct {
		name       string
		filterName string
		wantAction string
		wantDelta  string
	}{
		{
			name:       "known_modifier",
			filterName: "PREFER-LOCAL",
			wantAction: "modify",
			wantDelta:  "local-preference 200",
		},
		{
			name:       "unknown_modifier",
			filterName: "NONEXISTENT",
			wantAction: "reject",
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
				t.Errorf("action = %q, want %q", out.Action, tt.wantAction)
			}
			if out.Update != tt.wantDelta {
				t.Errorf("delta = %q, want %q", out.Update, tt.wantDelta)
			}
		})
	}
}
