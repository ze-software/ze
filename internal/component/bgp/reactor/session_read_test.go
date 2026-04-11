package reactor

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
)

// TestSessionReadWithBufio verifies messages parse correctly through bufio.Reader.
// VALIDATES: AC-1 — Messages parsed correctly through bufio.Reader.
// PREVENTS: Regression if bufio.Reader breaks message framing.
func TestSessionReadWithBufio(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive

	session := NewSession(settings)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	t.Cleanup(func() { client.Close() }) //nolint:errcheck // test cleanup
	t.Cleanup(func() { server.Close() }) //nolint:errcheck // test cleanup

	_ = acceptWithReader(t, session, server, client)

	// bufReader must be initialized after connection establishment.
	require.NotNil(t, session.bufReader, "bufReader must be set after Accept")

	// Send peer's OPEN + drain KEEPALIVE response.
	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x01020302,
	}
	openBytes := message.PackTo(peerOpen, nil)

	go func() {
		if _, err := client.Write(openBytes); err != nil {
			return
		}
		buf := make([]byte, 4096)
		if _, err := client.Read(buf); err != nil {
			return
		}
	}()

	err := session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateOpenConfirm, session.State())

	// Send KEEPALIVE to transition to Established.
	keepaliveBytes := message.PackTo(message.NewKeepalive(), nil)

	go func() {
		if _, err := client.Write(keepaliveBytes); err != nil {
			return
		}
	}()

	err = session.ReadAndProcess()
	require.NoError(t, err)
	require.Equal(t, fsm.StateEstablished, session.State())
}

// TestSessionReadDeadlineWithBufio verifies SetReadDeadline propagates through bufio.Reader.
// VALIDATES: AC-2 — Read deadline fires through bufio.Reader
// PREVENTS: Regression if deadline doesn't propagate through buffered layer.
func TestSessionReadDeadlineWithBufio(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	settings.Connection = ConnectionPassive

	session := NewSession(settings)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	t.Cleanup(func() { client.Close() }) //nolint:errcheck // test cleanup
	t.Cleanup(func() { server.Close() }) //nolint:errcheck // test cleanup

	_ = acceptWithReader(t, session, server, client)

	// bufReader must be initialized.
	require.NotNil(t, session.bufReader, "bufReader must be set after Accept")

	// Set a short deadline on the underlying conn.
	// bufio.Reader reads from this conn — deadline must propagate.
	require.NoError(t, server.SetReadDeadline(time.Now().Add(50*time.Millisecond)))

	// Call readAndProcessMessage directly (ReadAndProcess overwrites deadline to 5s).
	// Don't send any data — should timeout through the bufio.Reader layer.
	// Capture bufReader under RLock to match the production discipline (the
	// Run loop and ReadAndProcess both capture conn + bufReader under s.mu.RLock
	// so they pass a consistent pair to readAndProcessMessage).
	session.mu.RLock()
	bufReader := session.bufReader
	session.mu.RUnlock()
	err := session.readAndProcessMessage(server, bufReader)
	require.Error(t, err)
	var netErr net.Error
	require.ErrorAs(t, err, &netErr, "deadline must propagate as net.Error through bufio.Reader")
	require.True(t, netErr.Timeout(), "error must be a timeout")
}
