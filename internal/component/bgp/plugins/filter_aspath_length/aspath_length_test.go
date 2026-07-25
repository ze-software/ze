package filter_aspath_length

import (
	"strings"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: AC-9 -- as-path-length max rejects long paths.
// VALIDATES: AC-10 -- as-path-length max accepts short paths.
func TestAsPathLengthMaxAcceptReject(t *testing.T) {
	tests := []struct {
		name    string
		pathLen int
		max     int
		want    bool
	}{
		{"under_max", 25, 30, true},
		{"at_max", 30, 30, true},
		{"over_max", 35, 30, false},
		{"zero_length_allowed", 0, 30, true},
		{"max_one_accepts_one", 1, 1, true},
		{"max_one_rejects_two", 2, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := &asPathLengthDef{max: tt.max, min: -1}
			got := evaluateASPathLength(tt.pathLen, def)
			if got != tt.want {
				t.Errorf("evaluateASPathLength(%d, max=%d) = %v, want %v", tt.pathLen, tt.max, got, tt.want)
			}
		})
	}
}

// VALIDATES: AC-11 -- as-path-length min rejects short paths.
// VALIDATES: AC-12 -- as-path-length min accepts long paths.
func TestAsPathLengthMinAcceptReject(t *testing.T) {
	tests := []struct {
		name    string
		pathLen int
		min     int
		want    bool
	}{
		{"above_min", 3, 2, true},
		{"at_min", 2, 2, true},
		{"below_min", 1, 2, false},
		{"zero_min_accepts_zero", 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := &asPathLengthDef{max: -1, min: tt.min}
			got := evaluateASPathLength(tt.pathLen, def)
			if got != tt.want {
				t.Errorf("evaluateASPathLength(%d, min=%d) = %v, want %v", tt.pathLen, tt.min, got, tt.want)
			}
		})
	}
}

func TestAsPathLengthMinMaxCombined(t *testing.T) {
	def := &asPathLengthDef{max: 30, min: 2}

	tests := []struct {
		name    string
		pathLen int
		want    bool
	}{
		{"in_range", 15, true},
		{"at_min", 2, true},
		{"at_max", 30, true},
		{"below_min", 1, false},
		{"above_max", 31, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateASPathLength(tt.pathLen, def)
			if got != tt.want {
				t.Errorf("evaluateASPathLength(%d, min=2, max=30) = %v, want %v", tt.pathLen, got, tt.want)
			}
		})
	}
}

func TestCountASPathHops(t *testing.T) {
	tests := []struct {
		name      string
		asPathStr string
		want      int
	}{
		{"empty", "", 0},
		{"single_asn", "65001", 1},
		{"two_asns_bracketed", "[65001 65002]", 2},
		{"five_asns", "[65001 65002 65003 65004 65005]", 5},
		{"spaces_only", "   ", 0},
		{"brackets_empty", "[]", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countASPathHops(tt.asPathStr)
			if got != tt.want {
				t.Errorf("countASPathHops(%q) = %d, want %d", tt.asPathStr, got, tt.want)
			}
		})
	}
}

func TestExtractASPathField(t *testing.T) {
	tests := []struct {
		name       string
		updateText string
		want       string
	}{
		{"with_single_asn", "origin igp as-path 65001 next-hop 10.0.0.1", "65001"},
		{"with_multi_asn", "origin igp as-path [65001 65002] next-hop 10.0.0.1", "[65001 65002]"},
		{"no_as_path", "origin igp next-hop 10.0.0.1", ""},
		{"as_path_at_end", "origin igp as-path 65001", "65001"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractASPathField(tt.updateText)
			if got != tt.want {
				t.Errorf("extractASPathField(%q) = %q, want %q", tt.updateText, got, tt.want)
			}
		})
	}
}

func TestParseAsPathLengthDefs(t *testing.T) {
	tests := []struct {
		name      string
		bgpCfg    map[string]any
		wantCount int
		wantErr   bool
		errSubstr string
	}{
		{
			name: "max_only",
			bgpCfg: map[string]any{
				"policy": map[string]any{
					"as-path-length": map[string]any{
						"REJECT-LONG": map[string]any{"max": float64(30)},
					},
				},
			},
			wantCount: 1,
		},
		{
			name: "min_only",
			bgpCfg: map[string]any{
				"policy": map[string]any{
					"as-path-length": map[string]any{
						"REJECT-SHORT": map[string]any{"min": float64(2)},
					},
				},
			},
			wantCount: 1,
		},
		{
			name: "both",
			bgpCfg: map[string]any{
				"policy": map[string]any{
					"as-path-length": map[string]any{
						"RANGE": map[string]any{"min": float64(2), "max": float64(30)},
					},
				},
			},
			wantCount: 1,
		},
		{
			name: "neither",
			bgpCfg: map[string]any{
				"policy": map[string]any{
					"as-path-length": map[string]any{
						"BAD": map[string]any{},
					},
				},
			},
			wantErr:   true,
			errSubstr: "at least one",
		},
		{
			name: "min_exceeds_max",
			bgpCfg: map[string]any{
				"policy": map[string]any{
					"as-path-length": map[string]any{
						"BAD": map[string]any{"min": float64(50), "max": float64(30)},
					},
				},
			},
			wantErr:   true,
			errSubstr: "min (50) exceeds max (30)",
		},
		{
			name:      "no_policy",
			bgpCfg:    map[string]any{},
			wantCount: 0,
		},
		{
			name: "name_too_long",
			bgpCfg: map[string]any{
				"policy": map[string]any{
					"as-path-length": map[string]any{
						strings.Repeat("x", maxNameLen+1): map[string]any{"max": float64(30)},
					},
				},
			},
			wantErr:   true,
			errSubstr: "exceeds maximum length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defs, err := parseAsPathLengthDefs(tt.bgpCfg)
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
		})
	}
}

func TestHandleFilterUpdate(t *testing.T) {
	defs := map[string]*asPathLengthDef{
		"REJECT-LONG":  {name: "REJECT-LONG", max: 3, min: -1},
		"REJECT-SHORT": {name: "REJECT-SHORT", max: -1, min: 2},
	}
	defsByName.Store(&defs)
	defer defsByName.Store(nil)

	tests := []struct {
		name       string
		filterName string
		update     string
		wantAction sdk.FilterAction
	}{
		{
			name:       "accept_short_path",
			filterName: "REJECT-LONG",
			update:     "origin igp as-path [65001 65002] next-hop 10.0.0.1",
			wantAction: sdk.FilterAccept,
		},
		{
			name:       "reject_long_path",
			filterName: "REJECT-LONG",
			update:     "origin igp as-path [65001 65002 65003 65004] next-hop 10.0.0.1",
			wantAction: sdk.FilterReject,
		},
		{
			name:       "accept_at_max",
			filterName: "REJECT-LONG",
			update:     "origin igp as-path [65001 65002 65003] next-hop 10.0.0.1",
			wantAction: sdk.FilterAccept,
		},
		{
			name:       "reject_too_short",
			filterName: "REJECT-SHORT",
			update:     "origin igp as-path 65001 next-hop 10.0.0.1",
			wantAction: sdk.FilterReject,
		},
		{
			name:       "accept_long_enough",
			filterName: "REJECT-SHORT",
			update:     "origin igp as-path [65001 65002 65003] next-hop 10.0.0.1",
			wantAction: sdk.FilterAccept,
		},
		{
			name:       "unknown_filter_rejects",
			filterName: "NONEXISTENT",
			update:     "origin igp as-path 65001",
			wantAction: sdk.FilterReject,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := &sdk.FilterUpdateInput{
				Filter: tt.filterName,
				Peer:   "127.0.0.1",
				Update: tt.update,
			}
			out := handleFilterUpdate(in)
			if out.Action != tt.wantAction {
				t.Errorf("action = %v, want %v", out.Action, tt.wantAction)
			}
		})
	}
}
