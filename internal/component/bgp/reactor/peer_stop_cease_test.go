package reactor

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// RFC 4486 Section 4: "If a BGP speaker decides to de-configure a peer, then
// the speaker SHOULD send a NOTIFICATION message with the Error Code Cease and
// the Error Subcode "Peer De-configured"." The same section gives "Other
// Configuration Change" to a reset caused by any other configuration change.
//
// TestStopWithCeaseTellsThePeerWhy drives the stop a config reload uses for a
// removed peer and for a restarted one. The peer MUST read a Cease with the
// subcode the caller chose before the socket closes.
//
// PREVENTS: a reload that closes TCP with no NOTIFICATION. Peer.Stop only
// cancels a context, and the session's cancel goroutine then closes the
// connection with nothing on the wire.
func TestStopWithCeaseTellsThePeerWhy(t *testing.T) {
	for _, subcode := range []uint8{message.NotifyCeasePeerDeconfigured, message.NotifyCeaseOtherConfigChange} {
		t.Run(message.CeaseSubcodeString(subcode), func(t *testing.T) {
			session, client := shutdownTestSession(t, fsm.StateEstablished)
			peer := NewPeer(session.settings)
			peer.mu.Lock()
			peer.session = session
			peer.mu.Unlock()

			got := readOne(client)
			peer.stopWithCease(subcode)

			// Marker, length 21 (header plus code and subcode, no data:
			// RFC 8203 data is only for subcodes 2 and 4), type NOTIFICATION.
			want := append(bytes.Repeat([]byte{0xFF}, 16), 0x00, 0x15, 0x03, byte(message.NotifyCease), subcode)
			select {
			case msg, ok := <-got:
				require.True(t, ok, "socket closed with no NOTIFICATION on it")
				require.Equal(t, want, msg)
			case <-time.After(5 * time.Second):
				t.Fatal("the peer was stopped without being told why")
			}
		})
	}
}
