// Design: docs/architecture/wire/messages.md -- UPDATE receive acceptance
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSessionRFC4271SamePrefixWithdrawAndAnnounceIsProcessed exercises the
// receiver's mandatory support for a prefix in both legacy UPDATE fields.
//
// VALIDATES: on an established, unfiltered legacy eBGP Session, an initial
// ordinary announcement, a same-prefix mixed UPDATE and a following ordinary
// announcement each complete ReadAndProcess, keep Established and deliver one
// semantic UPDATE through firstASReceive. The ordinary controls also retain
// their announced NLRI.
//
// PREVENTS: rejecting or silently dropping an otherwise valid UPDATE solely
// because the same prefix appears in WITHDRAWN ROUTES and NLRI.
//
// RFC requirement: RFC4271-4.3-5 positive -- the established Session processes
// the same-prefix shape through its receive consumer boundary. The callback's
// field representation and the RIB winner are unrestricted here; the default
// announce-wins treatment is separately covered by RFC4271-4.3-7.
func TestSessionRFC4271SamePrefixWithdrawAndAnnounceIsProcessed(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	s, client := firstASSession(t, settings, nil, nil)
	prefix := []byte{24, 203, 0, 113}
	attrs := firstASAttrs(2, 65002)
	for _, tc := range []struct {
		name      string
		withdrawn []byte
	}{
		{"initial ordinary", nil},
		{"same prefix", prefix},
		{"following ordinary", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wu := firstASReceive(t, s, client, makeUpdateBody(tc.withdrawn, attrs, prefix))
			if len(tc.withdrawn) != 0 {
				return
			}
			nlri, err := wu.NLRI()
			require.NoError(t, err)
			require.Equal(t, prefix, nlri)
		})
	}
}
