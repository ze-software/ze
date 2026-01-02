package api

import (
	"testing"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
)

// TestParseAttributeFilterValid verifies valid parsing.
//
// VALIDATES: Known names map to correct codes.
// PREVENTS: Typos causing silent failures.
func TestParseAttributeFilterValid(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMode  FilterMode
		wantCodes []attribute.AttributeCode
	}{
		{
			name:     "all keyword",
			input:    "all",
			wantMode: FilterModeAll,
		},
		{
			name:     "empty string defaults to all",
			input:    "",
			wantMode: FilterModeAll,
		},
		{
			name:     "none keyword",
			input:    "none",
			wantMode: FilterModeNone,
		},
		{
			name:      "single attribute",
			input:     "origin",
			wantMode:  FilterModeSelective,
			wantCodes: []attribute.AttributeCode{attribute.AttrOrigin},
		},
		{
			name:      "multiple attributes",
			input:     "origin as-path next-hop",
			wantMode:  FilterModeSelective,
			wantCodes: []attribute.AttributeCode{attribute.AttrOrigin, attribute.AttrASPath, attribute.AttrNextHop},
		},
		{
			name:      "med and local-pref",
			input:     "med local-pref",
			wantMode:  FilterModeSelective,
			wantCodes: []attribute.AttributeCode{attribute.AttrMED, attribute.AttrLocalPref},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAttributeFilter(tt.input)
			if err != nil {
				t.Fatalf("ParseAttributeFilter(%q) error = %v", tt.input, err)
			}
			if got.Mode != tt.wantMode {
				t.Errorf("Mode = %v, want %v", got.Mode, tt.wantMode)
			}
			if tt.wantCodes != nil {
				if len(got.Codes) != len(tt.wantCodes) {
					t.Errorf("len(Codes) = %d, want %d", len(got.Codes), len(tt.wantCodes))
				}
				for i, code := range tt.wantCodes {
					if i < len(got.Codes) && got.Codes[i] != code {
						t.Errorf("Codes[%d] = %v, want %v", i, got.Codes[i], code)
					}
				}
			}
		})
	}
}

// TestParseAttributeFilterNumeric verifies attr-N syntax.
//
// VALIDATES: attr-99 parses to code 99.
// PREVENTS: Unknown attributes inaccessible.
func TestParseAttributeFilterNumeric(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCodes []attribute.AttributeCode
	}{
		{
			name:      "single numeric",
			input:     "attr-99",
			wantCodes: []attribute.AttributeCode{99},
		},
		{
			name:      "mixed named and numeric",
			input:     "origin attr-99 as-path",
			wantCodes: []attribute.AttributeCode{attribute.AttrOrigin, 99, attribute.AttrASPath},
		},
		{
			name:      "attr-0",
			input:     "attr-0",
			wantCodes: []attribute.AttributeCode{0},
		},
		{
			name:      "attr-255",
			input:     "attr-255",
			wantCodes: []attribute.AttributeCode{255},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAttributeFilter(tt.input)
			if err != nil {
				t.Fatalf("ParseAttributeFilter(%q) error = %v", tt.input, err)
			}
			if got.Mode != FilterModeSelective {
				t.Errorf("Mode = %v, want FilterModeSelective", got.Mode)
			}
			if len(got.Codes) != len(tt.wantCodes) {
				t.Errorf("len(Codes) = %d, want %d", len(got.Codes), len(tt.wantCodes))
			}
			for i, code := range tt.wantCodes {
				if i < len(got.Codes) && got.Codes[i] != code {
					t.Errorf("Codes[%d] = %v, want %v", i, got.Codes[i], code)
				}
			}
		})
	}
}

// TestParseAttributeFilterInvalid verifies error handling.
//
// VALIDATES: Unknown names rejected with helpful error.
// PREVENTS: Silent acceptance of typos.
func TestParseAttributeFilterInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unknown name", "unknown-attr"},
		{"typo in origin", "orign"},
		{"invalid numeric negative", "attr--1"},
		{"invalid numeric too large", "attr-256"},
		{"invalid numeric format", "attr-abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAttributeFilter(tt.input)
			if err == nil {
				t.Errorf("ParseAttributeFilter(%q) should return error", tt.input)
			}
		})
	}
}

// TestParseAttributeFilterStructuralRejected verifies MP_REACH/UNREACH rejected.
//
// VALIDATES: attr-14 and attr-15 fail config parsing.
// PREVENTS: Structural attributes being filtered.
func TestParseAttributeFilterStructuralRejected(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"attr-14 MP_REACH", "attr-14"},
		{"attr-15 MP_UNREACH", "attr-15"},
		{"mixed with structural", "origin attr-14 as-path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAttributeFilter(tt.input)
			if err == nil {
				t.Errorf("ParseAttributeFilter(%q) should return error for structural attr", tt.input)
			}
		})
	}
}

// TestParseAttributeFilterCaseInsensitive verifies case handling.
//
// VALIDATES: "AS-PATH" == "as-path".
// PREVENTS: Case sensitivity surprises.
func TestParseAttributeFilterCaseInsensitive(t *testing.T) {
	tests := []struct {
		input     string
		wantCodes []attribute.AttributeCode
	}{
		{"ORIGIN", []attribute.AttributeCode{attribute.AttrOrigin}},
		{"Origin", []attribute.AttributeCode{attribute.AttrOrigin}},
		{"AS-PATH", []attribute.AttributeCode{attribute.AttrASPath}},
		{"As-Path", []attribute.AttributeCode{attribute.AttrASPath}},
		{"NEXT-HOP", []attribute.AttributeCode{attribute.AttrNextHop}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseAttributeFilter(tt.input)
			if err != nil {
				t.Fatalf("ParseAttributeFilter(%q) error = %v", tt.input, err)
			}
			if len(got.Codes) != len(tt.wantCodes) {
				t.Errorf("len(Codes) = %d, want %d", len(got.Codes), len(tt.wantCodes))
			}
			for i, code := range tt.wantCodes {
				if i < len(got.Codes) && got.Codes[i] != code {
					t.Errorf("Codes[%d] = %v, want %v", i, got.Codes[i], code)
				}
			}
		})
	}
}

// TestParseAttributeFilterDedupe verifies duplicate handling.
//
// VALIDATES: "as-path as-path" deduplicates silently.
// PREVENTS: Duplicate entries in codes slice.
func TestParseAttributeFilterDedupe(t *testing.T) {
	got, err := ParseAttributeFilter("as-path as-path origin origin")
	if err != nil {
		t.Fatalf("ParseAttributeFilter() error = %v", err)
	}
	if len(got.Codes) != 2 {
		t.Errorf("len(Codes) = %d, want 2 (deduplicated)", len(got.Codes))
	}
}

// TestParseAttributeFilterBothForms verifies singular/plural accepted.
//
// VALIDATES: "community" and "communities" both map to AttrCommunity.
// PREVENTS: Unnecessarily strict config parsing.
func TestParseAttributeFilterBothForms(t *testing.T) {
	tests := []struct {
		input    string
		wantCode attribute.AttributeCode
	}{
		{"community", attribute.AttrCommunity},
		{"communities", attribute.AttrCommunity},
		{"extended-community", attribute.AttrExtCommunity},
		{"extended-communities", attribute.AttrExtCommunity},
		{"large-community", attribute.AttrLargeCommunity},
		{"large-communities", attribute.AttrLargeCommunity},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseAttributeFilter(tt.input)
			if err != nil {
				t.Fatalf("ParseAttributeFilter(%q) error = %v", tt.input, err)
			}
			if len(got.Codes) != 1 || got.Codes[0] != tt.wantCode {
				t.Errorf("Codes = %v, want [%v]", got.Codes, tt.wantCode)
			}
		})
	}
}
