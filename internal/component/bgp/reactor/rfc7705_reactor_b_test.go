package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// Goal: prove a speaker configured with the RFC 7705 migration mechanism not
// only accepts the OPEN of its iBGP peer under either ASN but establishes the
// session, and establishes it as internal.
// Method: an OpenSent session whose LocalAS and PeerAS are the retained ASN and
// whose migration ASN is the legacy one receives the peer OPEN through
// handleOpen and the peer KEEPALIVE through handleKeepalive; the FSM state and
// the internal verdict for the ASN the peer presented are read.
//
// VALIDATES: RFC 7705 Section 4.2, accept the OPEN and establish an iBGP session.
// PREVENTS: a session that stops at OpenConfirm, or that establishes as eBGP
// because the peer presented the legacy ASN.
//
// RFC requirement: RFC7705-4.2-2 positive -- a peer OPEN whose My Autonomous System is the globally configured ASN, and one whose My Autonomous System is the locally configured (migration) ASN, each reach Established after the peer KEEPALIVE, draw no NOTIFICATION, and the ASN presented is judged internal (isIBGPWith true).
// RFC requirement: RFC7705-4.2-2 negative -- a peer OPEN presenting a third ASN is refused with OPEN Message Error / Bad Peer AS and never reaches Established, even after a KEEPALIVE.
func TestRFC7705MigratingPeerEstablishesAsIBGP(t *testing.T) {
	cases := []struct {
		name       string
		advertised uint16
		accept     bool
	}{
		{name: "globally-configured-asn", advertised: migrationLocalAS, accept: true},
		{name: "locally-configured-asn", advertised: migrationLegacyAS, accept: true},
		{name: "third-asn", advertised: migrationStrangerAS, accept: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session, written := migrationSession(t, migrationLocalAS, migrationLocalAS, migrationLegacyAS)
			openErr := session.handleOpen(openBodyWithIdentifier(tc.advertised, 0x0A000001))
			if !tc.accept {
				require.ErrorIs(t, openErr, ErrPeerASMismatch)
				code, subcode, found := notificationFrom(t, written)
				require.True(t, found)
				require.Equal(t, message.NotifyOpenMessage, message.NotifyErrorCode(code))
				require.Equal(t, message.NotifyOpenBadPeerAS, subcode)
				session.handleKeepalive() //nolint:errcheck // the refused session must not advance whatever this returns
				require.NotEqual(t, fsm.StateEstablished, session.State())
				return
			}
			require.NoError(t, openErr)
			require.NoError(t, session.handleKeepalive())
			require.Equal(t, fsm.StateEstablished, session.State())
			require.True(t, session.settings.isIBGPWith(uint32(tc.advertised)),
				"the session established under this ASN is internal")
			_, _, found := notificationFrom(t, written)
			require.False(t, found)
		})
	}
}
