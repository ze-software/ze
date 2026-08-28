package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// RFC requirement: RFC7606-5.3-6 negative -- MP_REACH length 4 is below the minimum 5.
func TestRFC7606MPReachLengthFourIsIncorrect(t *testing.T) {
	// AFI(2), SAFI(1), and NextHopLen(1) fill four octets. FlowSpec has no next hop,
	// so the only missing octet is MP_REACH's reserved field and the only applicable
	// rule is Section 5.3's five-octet minimum.
	mp := []byte{0x00, 0x01, 0x85, 0x00}
	pathAttrs := sJoin(sOrigin, sASPath, append([]byte{0x80, 0x0e, byte(len(mp))}, mp...))

	result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
	require.Equal(t, RFC7606ActionSessionReset, result.Action)
	require.Equal(t, uint8(14), result.AttrCode)
	require.Contains(t, result.Description, "length 4 < 5",
		"the minimum-length rule itself must reject the boundary below five")
}
