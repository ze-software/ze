package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
)

// VALIDATES: WriteAnnounceUpdate settles both halves of the Next Hop field before
// it writes its first byte, so a refusal returns zero bytes and leaves the
// caller's buffer untouched.
// PREVENTS: a half-built UPDATE in a session write buffer, and a future caller of
// this exported writer reaching the panic or the invented address that
// Peer.SendAnnounce is guarded against.

// zeroBuf reports whether the first n bytes of buf are still zero, which is what
// a buffer looks like when the encoder refused before writing.
func zeroBuf(buf []byte, n int) bool {
	for _, b := range buf[:n] {
		if b != 0 {
			return false
		}
	}
	return true
}

func TestWriteAnnounceUpdateRefusesBeforeWriting(t *testing.T) {
	v4Prefix := netip.MustParsePrefix("10.0.0.0/24")
	v6Prefix := netip.MustParsePrefix("2001:db8:1::/64")

	cases := []struct {
		name      string
		prefix    netip.Prefix
		nextHop   netip.Addr
		linkLocal netip.Addr
		wantErr   bool
	}{
		{"v4 route, v4 next hop", v4Prefix, netip.MustParseAddr("192.0.2.1"), netip.Addr{}, false},
		{"v4 route, mapped next hop", v4Prefix, netip.MustParseAddr("::ffff:192.0.2.1"), netip.Addr{}, false},
		{"v4 route, unset next hop", v4Prefix, netip.Addr{}, netip.Addr{}, true},
		{"v4 route, v6 next hop", v4Prefix, netip.MustParseAddr("2001:db8::1"), netip.Addr{}, true},
		{"v6 route, v6 next hop", v6Prefix, netip.MustParseAddr("2001:db8::1"), netip.Addr{}, false},
		{"v6 route, v6 next hop and link-local", v6Prefix, netip.MustParseAddr("2001:db8::1"), netip.MustParseAddr("fe80::2"), false},
		{"v6 route, v4 next hop", v6Prefix, netip.MustParseAddr("192.0.2.1"), netip.Addr{}, true},
		{"v6 route, mapped next hop", v6Prefix, netip.MustParseAddr("::ffff:192.0.2.1"), netip.Addr{}, true},
		{"v6 route, unset next hop", v6Prefix, netip.Addr{}, netip.Addr{}, true},
		{"v6 route, link-local next hop", v6Prefix, netip.MustParseAddr("fe80::1"), netip.Addr{}, true},
		{"v6 route, global second address", v6Prefix, netip.MustParseAddr("2001:db8::1"), netip.MustParseAddr("2001:db8::2"), true},
		{"v6 route, mapped second address", v6Prefix, netip.MustParseAddr("2001:db8::1"), netip.MustParseAddr("::ffff:192.0.2.1"), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, 512)
			route := bgptypes.RouteSpec{
				Prefix:  tc.prefix,
				NextHop: bgptypes.NewNextHopExplicit(tc.nextHop),
			}

			n := writeAnnounceUpdate(buf, 0, route, tc.linkLocal, 65000, false, true, false)
			err := validateAnnounceNextHop(route, tc.linkLocal)

			if !tc.wantErr {
				require.NoError(t, err)
				assert.Positive(t, n, "an accepted next hop still produces a message")
				return
			}
			require.ErrorIs(t, err, ErrNextHopUnencodable)
			assert.Zero(t, n, "the writer refuses by reporting no bytes written")
			assert.True(t, zeroBuf(buf, len(buf)), "a refusal leaves the caller's buffer untouched")
		})
	}
}
