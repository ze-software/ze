package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrigin verifies parsing of BGP ORIGIN attribute values.
//
// VALIDATES: All valid origin strings parse to correct uint8 values.
// PREVENTS: Regression in origin parsing when unifying config and API parsers.
func TestOrigin(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  uint8
	}{
		// RFC 4271 Section 5.1.1: ORIGIN values
		{"igp_lowercase", "igp", 0},
		{"egp_lowercase", "egp", 1},
		{"incomplete_lowercase", "incomplete", 2},

		// Case insensitivity
		{"igp_uppercase", "IGP", 0},
		{"egp_uppercase", "EGP", 1},
		{"incomplete_uppercase", "INCOMPLETE", 2},
		{"igp_mixed", "Igp", 0},
		{"egp_mixed", "Egp", 1},
		{"incomplete_mixed", "InComplete", 2},

		// Empty string = IGP (config behavior to preserve)
		{"empty_string_is_igp", "", 0},

		// "?" alias for incomplete (API behavior to preserve)
		{"question_mark_is_incomplete", "?", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Origin(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestOriginInvalid verifies that invalid origin strings are rejected.
//
// VALIDATES: Invalid inputs return descriptive error.
// PREVENTS: Silent acceptance of typos or unknown origin values.
func TestOriginInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"typo_igpp", "igpp"},
		{"typo_ig", "ig"},
		{"unknown_value", "unknown"},
		{"numeric_zero", "0"},
		{"numeric_one", "1"},
		{"numeric_two", "2"},
		{"whitespace", " igp"},
		{"whitespace_trailing", "igp "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Origin(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid origin")
		})
	}
}

// TestOriginString verifies the String() method for Origin values.
//
// VALIDATES: Origin values format correctly for display.
// PREVENTS: Wrong string representation in logs/output.
func TestOriginString(t *testing.T) {
	tests := []struct {
		name  string
		value uint8
		want  string
	}{
		{"igp", 0, "igp"},
		{"egp", 1, "egp"},
		{"incomplete", 2, "incomplete"},
		{"unknown_3", 3, "unknown(3)"},
		{"unknown_255", 255, "unknown(255)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OriginString(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}
