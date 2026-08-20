// Design: route_community.go — which extended community form the `update text`
// vocabulary writes for an AS number, and which numbers that form can carry.
// Related: route_community.go, internal/core/bgp/attribute/extcomm_decoded.go

package route

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestParseExtendedCommunitiesFourByteASNumberLimit verifies that the route
// target parser refuses a number too wide for the type 0x02 local
// administrator.
//
// The goal is that no value an operator types into `update text` reaches the
// wire with octets removed, and the method is a byte comparison plus an error
// assertion. RFC 5668 Section 2 gives the four-octet AS specific extended
// community a 4-octet global administrator and a 2-octet local administrator,
// so a number above 65535 has no place in it, and RFC 4360 Section 3.1 gives
// the two-octet form the other split.
//
// VALIDATES: target:65536:65535 encodes as type 0x02, and target:65536:65536 is
// rejected with an error naming 65535.
//
// PREVENTS: the silent truncation this parser used to perform.
// `update text extended-community [target:65536:70000]` became
// 0x0202 00010000 1170, so the peer received route target 65536:4464 while the
// operator's command returned success.
func TestParseExtendedCommunitiesFourByteASNumberLimit(t *testing.T) {
	t.Parallel()

	accepted := []struct {
		name  string
		input string
		want  attribute.ExtendedCommunity
	}{
		{
			name:  "two-byte AS keeps the type 0x00 form and its 4-octet number",
			input: "target:65000:99999",
			want:  attribute.ExtendedCommunity{0x00, 0x02, 0xFD, 0xE8, 0x00, 0x01, 0x86, 0x9F},
		},
		{
			name:  "first four-byte AS takes the type 0x02 form",
			input: "target:65536:100",
			want:  attribute.ExtendedCommunity{0x02, 0x02, 0x00, 0x01, 0x00, 0x00, 0x00, 0x64},
		},
		{
			name:  "last number the type 0x02 form carries",
			input: "target:65536:65535",
			want:  attribute.ExtendedCommunity{0x02, 0x02, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xFF},
		},
	}

	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ecs, consumed, err := ParseExtendedCommunities([]string{tt.input})
			require.NoError(t, err)
			require.Equal(t, 1, consumed)
			require.Len(t, ecs, 1)
			assert.Equal(t, tt.want, ecs[0])
			// The decoder splits the value field by the type octet, so a
			// correctly typed community reads back as it was written.
			assert.Equal(t, tt.input, ecs[0].String())
		})
	}

	refused := []struct {
		name  string
		input string
	}{
		{name: "first number above the limit", input: "target:65536:65536"},
		{name: "number far above the limit", input: "target:4200000001:99999"},
	}

	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := ParseExtendedCommunities([]string{tt.input})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "65535")
		})
	}
}
