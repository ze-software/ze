package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC 7606 Section 4: when an attribute's declared length conflicts with the data
// remaining, "the Total Attribute Length MUST be relied upon to enable the beginning of
// the NLRI field to be located". The attribute walk abandons at the conflict, and the
// treat-as-withdraw synthesis then reads the NLRI from the offset the Total Attribute
// Length names, never from where the overrunning attribute claimed to end.

// TestRFC7606NLRILocatedByTotalAttributeLength pins that offset.
//
// VALIDATES: RFC7606-4-3, the NLRI field starts at Total Attribute Length under a
// length conflict, so every announced prefix becomes a withdrawal.
// PREVENTS: locating the NLRI from the overrunning attribute's declared length, which
// would drop or truncate the withdrawn prefixes.
//
// RFC requirement: RFC7606-4-3 positive -- with a COMMUNITY that declares 8 octets where 4 remain, the synthesized withdrawal names both prefixes found at the Total Attribute Length offset (§4).
// RFC requirement: RFC7606-4-3 negative -- the withdrawal never carries the truncated tail that the overrunning attribute's own length would have pointed at (§4).
func TestRFC7606NLRILocatedByTotalAttributeLength(t *testing.T) {
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0xc0, 0x08, 0x08, 0xfd, 0xe8, 0x00, 0x64, // COMMUNITY declares 8 octets, 4 follow
	}
	nlri := []byte{24, 10, 0, 0, 24, 192, 0, 2} // 10.0.0.0/24, 192.0.2.0/24
	body := buildBody(nil, pathAttrs, nlri)

	// The conflict is real: the validator abandons the walk at COMMUNITY.
	verdict := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, verdict.Action, verdict.Description)
	require.Equal(t, uint8(8), verdict.AttrCode)

	out, changed := SynthesizeWithdraw(body)
	require.True(t, changed, "an UPDATE announcing two prefixes has routes to withdraw")

	withdrawn, attrs, remaining := splitBody(t, out)
	assert.Equal(t, nlri, withdrawn, "the NLRI at Total Attribute Length is what gets withdrawn")
	assert.Empty(t, attrs)
	assert.Empty(t, remaining)

	// Where COMMUNITY's own length would have placed the NLRI: 4 declared-but-absent
	// octets past the attribute section, inside the real NLRI. That tail is not a
	// prefix list and must never be what the withdrawal carries.
	overrun := 8 - 4
	wrongTail := nlri[overrun:]
	assert.NotEqual(t, wrongTail, withdrawn, "the NLRI is not located from the overrunning attribute")
}
