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
		{Name: "local-address", Source: SourceConfig},
		{Name: "local-as", Source: SourceConfig},
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
		{Name: "peer-as", Source: SourceConfig},
	}

	groups := FindCollisions(siblings, 1)

	assert.Empty(t, groups, "no collisions when all first chars unique")
}

// VALIDATES: AC-3 -- --min-prefix filtering.
// PREVENTS: Reporting collisions below the threshold.
func TestPrefixCollisionsMinPrefix(t *testing.T) {
	siblings := []SiblingInfo{
		{Name: "link-local", Source: SourceConfig},
		{Name: "local-address", Source: SourceConfig},
		{Name: "local-as", Source: SourceConfig},
		{Name: "listen", Source: SourceConfig},
	}

	// With min-prefix 1: "l" group has 4 members
	groups1 := FindCollisions(siblings, 1)
	assert.Len(t, groups1, 1)

	// With min-prefix 3: only report if 3+ chars needed to disambiguate
	// "li" vs "lo" disambiguates at 2 chars, so min-prefix=3 should still report
	// because "local-address" vs "local-as" needs 7 chars
	groups3 := FindCollisions(siblings, 3)
	assert.Len(t, groups3, 1, "local-address vs local-as needs 7 chars, above threshold 3")

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
