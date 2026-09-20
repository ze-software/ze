// Design: docs/architecture/wire/attributes.md — path attribute encoding
// Related: partial.go — the two walks under test
// Related: rfc4271_test.go — the tagged units that carry the RFC claims

package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClearPartialWalksTheSameShapesTheStampDoes feeds ClearPartialOnWellKnownAndNonTransitive
// the three section shapes its walk has a branch for, and checks the bytes it leaves behind.
//
// VALIDATES: an extended-length attribute is stepped over by its two-octet length, a section
// that stops parsing clears what it read and returns, and a clean section is left untouched.
// PREVENTS: a walk that mis-steps an extended-length header and then reads a value byte as a
// flags octet, which would clear a bit in the middle of an attribute's value.
func TestClearPartialWalksTheSameShapesTheStampDoes(t *testing.T) {
	t.Run("extended length", func(t *testing.T) {
		// ORIGIN with Partial and Extended Length set, a two-octet length of 1, then a MED
		// with Partial set. Reaching the MED at all requires the 4-octet header step.
		section := []byte{
			0x70, 0x01, 0x00, 0x01, 0x00,
			0xa0, 0x04, 0x04, 0x00, 0x00, 0x00, 0x0a,
		}
		require.Equal(t, 2, ClearPartialOnWellKnownAndNonTransitive(section))
		assert.Equal(t, byte(0x50), section[0], "the ORIGIN keeps its Extended Length bit")
		assert.Equal(t, byte(0x80), section[5], "the MED is reached through the 4-octet header")
	})

	t.Run("truncated value", func(t *testing.T) {
		// A well-known attribute, then one whose length runs past the end.
		section := []byte{
			0x60, 0x01, 0x01, 0x00,
			0x60, 0x05, 0x40, 0x00,
		}
		require.Equal(t, 1, ClearPartialOnWellKnownAndNonTransitive(section))
		assert.Equal(t, byte(0x40), section[0], "the attribute read before the truncation is cleared")
		assert.Equal(t, byte(0x60), section[4], "the truncated attribute is left as it arrived")
	})

	t.Run("nothing to clear", func(t *testing.T) {
		section := []byte{
			0x40, 0x01, 0x01, 0x00, // ORIGIN, already conformant
			0xc0, 0x08, 0x04, 0xff, 0xff, 0xff, 0x01, // optional transitive, Partial clear
		}
		before := append([]byte(nil), section...)
		assert.Zero(t, ClearPartialOnWellKnownAndNonTransitive(section))
		assert.Equal(t, before, section, "a section with no forbidden Partial bit is not rewritten")
	})
}

// TestPartialWalksActOnDisjointClasses runs both walks over one section, in both orders, and
// requires the same bytes out.
//
// VALIDATES: publishBase's claim that the stamp and the clear never touch the same attribute,
// which is what makes their order free.
// PREVENTS: the clear undoing the Section 5 bit the stamp just set, or the stamp re-setting a
// bit the clear removed, either of which would make the published bytes depend on call order.
func TestPartialWalksActOnDisjointClasses(t *testing.T) {
	const unknownCode = 250
	require.False(t, AttributeCode(unknownCode).Recognized(),
		"the fixture only tests the disjointness while ze has no meaning for this code")

	fixture := func() []byte {
		return []byte{
			0x60, 0x01, 0x01, 0x00, // ORIGIN, Partial set: the clear's
			0xa0, 0x04, 0x04, 0x00, 0x00, 0x00, 0x0a, // MED, Partial set: the clear's
			0xc0, unknownCode, 0x02, 0x01, 0x02, // unrecognized optional transitive: the stamp's
			0xe0, 0x08, 0x04, 0xff, 0xff, 0xff, 0x01, // COMMUNITIES, Partial from a previous AS: neither's
		}
	}

	stampFirst := fixture()
	SetPartialOnUnrecognizedTransitive(stampFirst)
	ClearPartialOnWellKnownAndNonTransitive(stampFirst)

	clearFirst := fixture()
	ClearPartialOnWellKnownAndNonTransitive(clearFirst)
	SetPartialOnUnrecognizedTransitive(clearFirst)

	assert.Equal(t, stampFirst, clearFirst, "the two walks commute, so publishBase's order is free")
	assert.Equal(t, byte(0x40), stampFirst[0], "the well-known attribute loses the Partial bit")
	assert.Equal(t, byte(0x80), stampFirst[4], "the optional non-transitive attribute loses it")
	assert.Equal(t, byte(0xe0), stampFirst[11], "the unrecognized transitive attribute gains it")
	assert.Equal(t, byte(0xe0), stampFirst[16], "the previous AS's Partial bit survives both walks")
}
