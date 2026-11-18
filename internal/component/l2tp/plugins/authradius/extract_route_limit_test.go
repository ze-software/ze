// Design: docs/guide/l2tp.md -- atomic refusal of oversized framed-route profiles.
// Related: extract.go -- extractAuthMetadata must preserve a bounded overflow witness.
package l2tpauthradius

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/radius"
)

// TestAuthMetadataPreservesFramedRouteOverflowWitness checks the real metadata
// producer: exactly64 stays useful and the65th valid route reaches the owner
// that refuses the whole update. Inputs larger than65 stay bounded at65.
// MUTATION: returning at >=64 loses the witness and fails the overflow cases.
func TestAuthMetadataPreservesFramedRouteOverflowWitness(t *testing.T) {
	for _, input := range []struct {
		name      string
		v4        int
		v6        int
		malformed bool
	}{
		{name: "ipv4_64", v4: 64},
		{name: "ipv4_65", v4: 65},
		{name: "ipv4_80", v4: 80},
		{name: "ipv6_64", v6: 64},
		{name: "ipv6_65", v6: 65},
		{name: "ipv6_80", v6: 80},
		{name: "mixed_64", v4: 32, v6: 32},
		{name: "mixed_65", v4: 32, v6: 33},
		{name: "mixed_66_malformed", v4: 63, v6: 3, malformed: true},
		{name: "malformed_does_not_consume_capacity", v4: 32, v6: 32, malformed: true},
	} {
		t.Run(input.name, func(t *testing.T) {
			packet := &radius.Packet{Code: radius.CodeAccessAccept}
			var expected []l2tp.FramedRoute
			for family, count := range []int{input.v4, input.v6} {
				attribute := byte(radius.AttrFramedRoute)
				for index := range count {
					prefix := fmt.Sprintf("10.64.%d.0/24", index)
					gateway := "0.0.0.0"
					if family == 1 {
						attribute = radius.AttrFramedIPv6Route
						prefix = fmt.Sprintf("2001:db8:%x::/64", index)
						gateway = "::"
					}
					if input.malformed {
						packet.Attrs = append(packet.Attrs, radius.Attr{Type: attribute, Value: []byte("invalid")})
					}
					packet.Attrs = append(packet.Attrs, radius.Attr{Type: attribute, Value: []byte(prefix + " " + gateway + " 7")})
					expected = append(expected, l2tp.FramedRoute{Prefix: netip.MustParsePrefix(prefix), Metric: 7})
				}
			}
			metadata := extractAuthMetadata(packet)
			require.NotNil(t, metadata)
			require.Equal(t, expected[:min(len(expected), 65)], metadata.FramedRoutes)
		})
	}
}
