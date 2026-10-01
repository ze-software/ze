// Design: docs/architecture/route-selection.md -- received NEXT_HOP semantics
// Related: session_next_hop.go -- invalidReceiveNextHop, the semantic NEXT_HOP check
// Related: session_read.go -- readAndProcessMessage, which ignores the route and logs it

package reactor

import (
	"encoding/binary"
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// receiveWithNextHop sends one UPDATE announcing 203.0.113.0/24 with the given
// NEXT_HOP over an established one-hop EBGP session (peer 192.0.2.1 in AS
// 65002, local AS 65001, connected subnet 192.0.2.2/24), and answers the
// announced NLRI and the withdrawn routes the session delivers.
func receiveWithNextHop(t *testing.T, nextHop [4]byte) (nlri, withdrawn []byte) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	s, client := firstASSession(t, settings, nil, nil)
	s.nextHopScope.Store(newReceiveNextHopScope([]netip.Prefix{netip.MustParsePrefix("192.0.2.2/24")}, client, settings))

	attrs := firstASAttrs(2, 65002)
	copy(attrs[len(attrs)-4:], nextHop[:])
	wu := firstASReceive(t, s, client, makeUpdateBody(nil, attrs, []byte{24, 203, 0, 113}))
	nlri, err := wu.NLRI()
	require.NoError(t, err)
	length := int(binary.BigEndian.Uint16(wu.Payload()[:2]))
	return nlri, wu.Payload()[2 : 2+length]
}

// TestRFC4271SemanticallyIncorrectNextHopIsLoggedAndIgnored drives the two
// semantic NEXT_HOP errors of RFC 4271 Section 6.3 through a live session
// read: a NEXT_HOP that is the receiving speaker's own address (criterion a),
// and, on a one-hop EBGP session, a NEXT_HOP outside the subnet the speaker
// shares with it (criterion b). The session logger is captured at Debug,
// the level the per-UPDATE error diagnostics are written at.
//
// VALIDATES: for each error the announced prefix is not delivered (NLRI
// empty, 203.0.113.0/24 moved to the withdrawn routes) and one diagnostics
// line names the "invalid-next-hop" event; a NEXT_HOP on the shared subnet is
// delivered and logs no such line.
// PREVENTS: a semantically incorrect NEXT_HOP installed as a usable route, or
// ignored with no trace an operator can find.
//
// RFC requirement: RFC4271-6.3-2 positive -- a NEXT_HOP equal to the receiving speaker's address, or off the shared subnet of a one-hop EBGP session, is logged as an invalid-next-hop diagnostics line and its route is ignored (withdrawn, not delivered).
// RFC requirement: RFC4271-6.3-2 negative -- a NEXT_HOP on the shared subnet is delivered and logs no invalid-next-hop line.
func TestRFC4271SemanticallyIncorrectNextHopIsLoggedAndIgnored(t *testing.T) {
	prefix := []byte{24, 203, 0, 113}
	for _, tc := range []struct {
		name    string
		nextHop [4]byte
		invalid bool
	}{
		{"receiving speaker's own address", [4]byte{192, 0, 2, 2}, true},
		{"off the shared subnet", [4]byte{198, 51, 100, 1}, true},
		{"on the shared subnet", [4]byte{192, 0, 2, 7}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buf := captureSessionLog(t, slog.LevelDebug)
			nlri, withdrawn := receiveWithNextHop(t, tc.nextHop)
			logged := strings.Contains(buf.String(), "event=invalid-next-hop")
			if tc.invalid {
				assert.Empty(t, nlri, "the route is ignored")
				assert.Equal(t, prefix, withdrawn, "the ignored route is withdrawn")
				assert.True(t, logged, "the error is logged: %s", buf.String())
				return
			}
			assert.Equal(t, prefix, nlri, "a valid NEXT_HOP is delivered")
			assert.Empty(t, withdrawn)
			assert.False(t, logged, "a valid NEXT_HOP logs no error: %s", buf.String())
		})
	}
}
