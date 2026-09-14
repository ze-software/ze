package parse

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
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

// TestOriginReadsTheAttributeTable proves both parsers answer from the
// attribute package's origin table rather than from a copy of it.
//
// PREVENTS: a name changed in the attribute table that the config parser keeps
// refusing, or a name the parser accepts that no UPDATE can carry
// (ai/rules/principles.md).
func TestOriginReadsTheAttributeTable(t *testing.T) {
	names := attribute.OriginTextNames()
	require.NotEmpty(t, names, "attribute.OriginTextNames answered no name, so there is nothing to compare against")

	for _, name := range names {
		want, ok := attribute.OriginFromText(name)
		require.True(t, ok, "the attribute table spells %q and OriginFromText refuses it", name)

		got, err := Origin(name)
		require.NoError(t, err, "the attribute table spells %q and Origin refuses it", name)
		assert.Equal(t, uint8(want), got)

		got, err = Origin(strings.ToUpper(name))
		require.NoError(t, err)
		assert.Equal(t, uint8(want), got, "Origin is case-insensitive")

		assert.Equal(t, name, OriginString(uint8(want)))
	}

	// A word the table does not hold is refused, and the refusal offers the
	// table's words rather than a list written here.
	_, err := Origin("sideways")
	require.Error(t, err)
	for _, name := range names {
		assert.Contains(t, err.Error(), name)
	}

	// A value past the table is named by its number, not by a table word.
	past := uint8(len(names))
	_, ok := attribute.OriginFromText(OriginString(past))
	assert.False(t, ok, "OriginString(%d) answered a table word for a value the table does not hold", past)
}
