// Design: docs/architecture/behavior/fsm-established.md — OPEN acceptance
// Overview: session_open_as.go — validateOpenPeerAS and openPeerAS, under test here
// RFC: rfc/short/rfc6793.md — the Four-octet AS capability replaces My AS only between NEW speakers

package reactor

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// openSentSessionASN4 is openSentSessionAS with the Four-octet AS advertisement chosen by
// the caller. With advertise false the session is built the way `capability { asn4
// disable; }` builds it (parseCapabilitiesFromTree sets DisableASN4), so the OPEN ze
// sends carries no code-65 capability and ze is an OLD speaker toward this peer.
func openSentSessionASN4(t *testing.T, peerAS uint32, advertise bool) (*Session, chan []byte) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, peerAS, 0x0A000002)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.DisableASN4 = !advertise
	settings.Capabilities = []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	session := NewSession(settings)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	written := make(chan []byte, 8)
	go func() {
		for {
			buf := make([]byte, 4096)
			n, err := client.Read(buf)
			if err != nil {
				close(written)
				return
			}
			written <- buf[:n]
		}
	}()

	require.NoError(t, session.Accept(server))
	require.Equal(t, fsm.StateOpenSent, session.State())

	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})

	return session, written
}

// TestOpenPeerASIsMyASWhenZeDoesNotAdvertiseASN4 drives handleOpen with the OPEN a NEW
// speaker in AS 4200000000 sends (My AS = AS_TRANS, capability value 4200000000) to a
// session ze holds two-octet.
//
// VALIDATES: RFC 6793 Section 4.1 uses the capability value "in lieu of" My AS only
// "When a NEW BGP speaker processes an OPEN message from another NEW BGP speaker". With
// the capability not advertised ze is an OLD speaker toward the peer, so the peer AS is
// the My Autonomous System field, AS_TRANS, and a session configured for remote 23456
// accepts it.
//
// PREVENTS: validateOpenPeerAS reading the capability whatever ze advertised, which sent
// NOTIFICATION 2/2 Bad Peer AS to BIRD in the interop scenario
// bgp-remove-private-as-as4path-frr.
//
// The other cases hold the gate to the local advertisement: a NEW session still judges by
// the capability, and a two-octet session does not accept the four-octet AS it never
// negotiated.
func TestOpenPeerASIsMyASWhenZeDoesNotAdvertiseASN4(t *testing.T) {
	const peerAS4 uint32 = 4200000000

	tests := []struct {
		name       string
		peerAS     uint32
		advertise  bool
		wantReject bool
	}{
		{"two-octet session accepts AS_TRANS", message.AS_TRANS, false, false},
		{"two-octet session refuses the unnegotiated four-octet AS", peerAS4, false, true},
		{"four-octet session accepts the capability AS", peerAS4, true, false},
		{"four-octet session refuses AS_TRANS", message.AS_TRANS, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, written := openSentSessionASN4(t, tt.peerAS, tt.advertise)

			err := session.handleOpen(openBodyWithASN4Capability(peerAS4))

			if !tt.wantReject {
				require.NoError(t, err, "the peer AS this session is configured for must be accepted")
				assert.Equal(t, fsm.StateOpenConfirm, session.State(), "the session advances")
				code, subcode, found := notificationFrom(t, written)
				assert.False(t, found,
					"an accepted OPEN must draw no NOTIFICATION, got %d/%d", code, subcode)
				return
			}

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrPeerASMismatch)
			code, subcode, found := notificationFrom(t, written)
			require.True(t, found, "RFC 4271 Section 6.2 requires a NOTIFICATION")
			assert.Equal(t, message.NotifyOpenMessage, message.NotifyErrorCode(code),
				"error code must be OPEN Message Error")
			assert.Equal(t, message.NotifyOpenBadPeerAS, subcode, "subcode must be Bad Peer AS")
		})
	}
}
