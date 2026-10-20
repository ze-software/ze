package update

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/rtc" // blank import: registers the RTC NLRI encoder
	"github.com/ze-software/ze/internal/core/family"
)

// TestUpdateTextRTCSection checks that an `mp ipv4/rtc` section reaches the rtc
// encoder whole. Its first word `origin-as` is also the top-level origin-as
// directive, and ending the section there left it empty
// ("nlri section has no prefixes"). Method: the group's NLRI bytes are the RFC
// 4684 Section 4 encoding, 96-bit and zero-length, for add and del.
func TestUpdateTextRTCSection(t *testing.T) {
	membership := []byte{0x60, 0x00, 0x00, 0xFD, 0xE8, 0x00, 0x02, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x01}
	cases := []struct {
		name     string
		args     string
		want     []byte
		withdraw bool
	}{
		{"membership", "nhop 192.0.2.1 mp ipv4/rtc add origin-as 65000 route-target 65000:1", membership, false},
		{"default", "nhop 192.0.2.1 mp ipv4/rtc add default", []byte{0x00}, false},
		{"membership withdrawn", "mp ipv4/rtc del origin-as 65000 route-target 65000:1", membership, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseUpdateText(strings.Fields(tc.args))
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)
			group := result.Groups[0]
			assert.Equal(t, family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIRTC}, group.Family)
			nlris := group.Announce
			if tc.withdraw {
				nlris = group.Withdraw
			}
			require.Len(t, nlris, 1)
			assert.Equal(t, tc.want, nlris[0].Bytes())
		})
	}

	// The top-level directive before a section keeps its meaning.
	result, err := ParseUpdateText(strings.Fields("origin-as 65001 nhop 192.0.2.1 mp ipv4/rtc add default"))
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, uint32(65001), result.Groups[0].OriginAS)
}
