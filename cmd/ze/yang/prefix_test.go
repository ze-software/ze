package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// VALIDATES: AC-1 -- prefix collision detection groups siblings by shared first character.
// PREVENTS: Collisions going undetected when siblings share a prefix.
func TestPrefixCollisions(t *testing.T) {
	siblings := []SiblingInfo{
		{Name: "link-local", Source: SourceConfig},
		{Name: "local", Source: SourceConfig},
		{Name: "listen", Source: SourceConfig},
		{Name: "hold-time", Source: SourceConfig},
	}

	groups := FindCollisions(siblings, 1)

	assert.Len(t, groups, 1, "should find 1 collision group (l-prefix)")
	assert.Equal(t, "l", groups[0].Prefix)
	assert.Len(t, groups[0].Siblings, 3)
}

// VALIDATES: AC-1 -- no false positives when all siblings have unique first chars.
// PREVENTS: Reporting non-collisions as collisions.
func TestPrefixCollisionsNone(t *testing.T) {
	siblings := []SiblingInfo{
		{Name: "add-path", Source: SourceConfig},
		{Name: "hold-time", Source: SourceConfig},
		{Name: "remote", Source: SourceConfig},
	}

	groups := FindCollisions(siblings, 1)

	assert.Empty(t, groups, "no collisions when all first chars unique")
}

// VALIDATES: AC-3 -- --min-prefix filtering.
// PREVENTS: Reporting collisions below the threshold.
func TestPrefixCollisionsMinPrefix(t *testing.T) {
	siblings := []SiblingInfo{
		{Name: "link-local", Source: SourceConfig},
		{Name: "local", Source: SourceConfig},
		{Name: "listen", Source: SourceConfig},
		{Name: "log-level", Source: SourceConfig},
	}

	// With min-prefix 1: "l" group has 4 members
	groups1 := FindCollisions(siblings, 1)
	assert.Len(t, groups1, 1)

	// With min-prefix 3: only report if 3+ chars needed to disambiguate
	// "li" vs "lo" disambiguates at 2 chars, so min-prefix=3 should still report
	// because "link-local" vs "listen" needs 4 chars, "local" vs "log-level" needs 3
	groups3 := FindCollisions(siblings, 3)
	assert.Len(t, groups3, 1, "link-local vs listen needs 4 chars, above threshold 3")

	// With min-prefix 10: nothing needs 10+ chars
	groups10 := FindCollisions(siblings, 10)
	assert.Empty(t, groups10, "no collision needs 10+ chars to disambiguate")
}

// VALIDATES: AC-1 -- reports minimum chars needed to disambiguate each group.
// PREVENTS: Incorrect disambiguation depth calculation.
func TestPrefixCollisionDepth(t *testing.T) {
	siblings := []SiblingInfo{
		{Name: "raw", Source: SourceCommand},
		{Name: "refresh", Source: SourceCommand},
		{Name: "remove", Source: SourceCommand},
		{Name: "resume", Source: SourceCommand},
	}

	groups := FindCollisions(siblings, 1)

	assert.Len(t, groups, 1)
	g := groups[0]
	assert.Equal(t, "r", g.Prefix)
	// raw=2 (ra unique), refresh=3 (ref unique), remove=3 (rem unique), resume=3 (res unique)
	// MinChars = 2 (raw), MaxChars = 3 (refresh/remove/resume)
	assert.Equal(t, 2, g.MinChars, "raw needs 2 chars")
	assert.Equal(t, 3, g.MaxChars, "refresh/remove/resume need 3 chars")
}

// VALIDATES: AC-3 -- boundary test for --min-prefix.
// PREVENTS: Off-by-one in threshold filtering.
func TestPrefixCollisionMinPrefixBoundary(t *testing.T) {
	siblings := []SiblingInfo{
		{Name: "add", Source: SourceConfig},
		{Name: "adj", Source: SourceConfig},
	}

	// add vs adj: need 3 chars to disambiguate (add vs adj)
	groups2 := FindCollisions(siblings, 2)
	assert.Len(t, groups2, 1, "needs 3 chars, threshold 2 should include")

	groups3 := FindCollisions(siblings, 3)
	assert.Len(t, groups3, 1, "needs 3 chars, threshold 3 should include")

	groups4 := FindCollisions(siblings, 4)
	assert.Empty(t, groups4, "needs 3 chars, threshold 4 should exclude")
}

// PREVENTS: Crash on empty input.
func TestPrefixCollisionsEmpty(t *testing.T) {
	assert.Empty(t, FindCollisions(nil, 1), "nil input")
	assert.Empty(t, FindCollisions([]SiblingInfo{}, 1), "empty input")
	assert.Empty(t, FindCollisions([]SiblingInfo{{Name: "solo"}}, 1), "single sibling")
}

// PREVENTS: Crash or incorrect results when a sibling has empty name.
func TestPrefixCollisionsEmptyName(t *testing.T) {
	siblings := []SiblingInfo{
		{Name: "", Source: SourceConfig},
		{Name: "add", Source: SourceConfig},
		{Name: "adj", Source: SourceConfig},
	}
	groups := FindCollisions(siblings, 1)
	assert.Len(t, groups, 1, "empty-name sibling should be skipped, add/adj collide")
	assert.Len(t, groups[0].Siblings, 2, "only add and adj, not the empty one")
}

// PREVENTS: Incorrect depth for one-char names that are a prefix of another.
func TestPrefixCollisionSubstringName(t *testing.T) {
	siblings := []SiblingInfo{
		{Name: "a", Source: SourceConfig},
		{Name: "ab", Source: SourceConfig},
		{Name: "abc", Source: SourceConfig},
	}
	groups := FindCollisions(siblings, 1)
	assert.Len(t, groups, 1)
	// "a" is LCP 1 with "ab" -> depth 2, but "a" is only 1 char -> disambig = min(2, 1) = 1
	// "ab" is LCP 2 with "abc" -> depth 3, but "ab" is only 2 chars -> disambig = min(3, 2) = 2
	// longestCommonPrefix("a", "ab") = 1, so depth for "a" = 1+1 = 2. But len("a") = 1.
	// disambigPrefix clips to len(name). So "a" gets prefix "a".
	// This tests that we don't panic or go out of bounds.
	assert.Equal(t, 1, groups[0].MinChars)
}
