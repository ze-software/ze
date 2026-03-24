package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMatchesPrefixTokensEmpty verifies empty tokens match any line.
//
// VALIDATES: Empty token list is a universal match (vacuous truth).
// PREVENTS: False negative when no search tokens are provided.
func TestMatchesPrefixTokensEmpty(t *testing.T) {
	assert.True(t, matchesPrefixTokens("set remote as 100", nil), "nil tokens should match any line")
	assert.True(t, matchesPrefixTokens("set remote as 100", []string{}), "empty tokens should match any line")
	assert.True(t, matchesPrefixTokens("anything at all", []string{}), "empty tokens should match any line")
}

// TestMatchesPrefixTokensEmptyLine verifies empty line with tokens returns false.
//
// VALIDATES: Non-empty tokens cannot match an empty line.
// PREVENTS: False positive on empty or whitespace-only lines.
func TestMatchesPrefixTokensEmptyLine(t *testing.T) {
	assert.False(t, matchesPrefixTokens("", []string{"r"}), "empty line should not match any token")
	assert.False(t, matchesPrefixTokens("   ", []string{"r"}), "whitespace-only line should not match any token")
	assert.False(t, matchesPrefixTokens("\t", []string{"set"}), "tab-only line should not match any token")
}

// TestMatchesPrefixTokensCaseInsensitive verifies case-insensitive prefix matching.
//
// VALIDATES: Uppercase tokens match lowercase words in the line.
// PREVENTS: Search failing when user types in different case than config values.
func TestMatchesPrefixTokensCaseInsensitive(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		tokens []string
		want   bool
	}{
		{
			name:   "uppercase tokens match lowercase line",
			line:   "set remote as 100",
			tokens: []string{"R", "A"},
			want:   true,
		},
		{
			name:   "lowercase tokens match uppercase line",
			line:   "SET REMOTE AS 100",
			tokens: []string{"r", "a"},
			want:   true,
		},
		{
			name:   "mixed case tokens match mixed case line",
			line:   "Set Remote As 100",
			tokens: []string{"rEm", "aS"},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesPrefixTokens(tt.line, tt.tokens)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestMatchesPrefixTokensPartialPrefix verifies partial word prefix matching.
//
// VALIDATES: A token that is a prefix of a word in the line matches.
// PREVENTS: Search requiring exact full-word match instead of prefix.
func TestMatchesPrefixTokensPartialPrefix(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		tokens []string
		want   bool
	}{
		{
			name:   "single partial prefix matches",
			line:   "set remote as 100",
			tokens: []string{"rem"},
			want:   true,
		},
		{
			name:   "full word also matches",
			line:   "set remote as 100",
			tokens: []string{"remote"},
			want:   true,
		},
		{
			name:   "single char prefix matches",
			line:   "set remote as 100",
			tokens: []string{"s"},
			want:   true,
		},
		{
			name:   "non-matching prefix does not match",
			line:   "set remote as 100",
			tokens: []string{"xyz"},
			want:   false,
		},
		{
			name:   "prefix longer than any word does not match",
			line:   "set remote as 100",
			tokens: []string{"remotely"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesPrefixTokens(tt.line, tt.tokens)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestMatchesPrefixTokensOrderMatters verifies tokens must appear in order.
//
// VALIDATES: Tokens must match words in left-to-right order within the line.
// PREVENTS: Out-of-order tokens producing false positive matches.
func TestMatchesPrefixTokensOrderMatters(t *testing.T) {
	// "as" matches "as" at position 3, but then "rem" has no match after position 3
	assert.False(t, matchesPrefixTokens("set remote as 100", []string{"as", "rem"}),
		"tokens in wrong order should not match")

	// Correct order should match
	assert.True(t, matchesPrefixTokens("set remote as 100", []string{"rem", "as"}),
		"tokens in correct order should match")

	// Another out-of-order case
	assert.False(t, matchesPrefixTokens("set remote as 100", []string{"100", "set"}),
		"reversed tokens should not match")
}

// TestMatchesPrefixTokensNonAdjacent verifies tokens can skip intermediate words.
//
// VALIDATES: Tokens match non-adjacent words as long as order is preserved.
// PREVENTS: Search requiring tokens to match consecutive words only.
func TestMatchesPrefixTokensNonAdjacent(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		tokens []string
		want   bool
	}{
		{
			name:   "skip intermediate word ip",
			line:   "set remote ip as 100",
			tokens: []string{"r", "a"},
			want:   true,
		},
		{
			name:   "skip multiple intermediate words",
			line:   "set bgp peer remote ip address as 65001",
			tokens: []string{"s", "a"},
			want:   true,
		},
		{
			name:   "match first and last word skipping all middle",
			line:   "set remote ip as 100",
			tokens: []string{"s", "1"},
			want:   true,
		},
		{
			name:   "three non-adjacent tokens",
			line:   "set bgp peer remote ip address as 65001",
			tokens: []string{"s", "r", "a"},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesPrefixTokens(tt.line, tt.tokens)
			assert.Equal(t, tt.want, got)
		})
	}
}
