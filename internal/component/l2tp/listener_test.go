package l2tp

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ephemeralBind returns a loopback AddrPort with an OS-chosen port.
func ephemeralBind(t *testing.T) netip.AddrPort {
	t.Helper()
	return netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0)
}

// TestListener_BindAndClose exercises the lifecycle without any I/O.
//
// VALIDATES: AC-2 (partial) -- listener binds and closes cleanly; port
// is an ephemeral value chosen by the kernel.
func TestListener_BindAndClose(t *testing.T) {
	ln := NewUDPListener(ephemeralBind(t), nil)
	require.NoError(t, ln.Start(context.Background()))
	addr := ln.Addr()
	assert.NotEqual(t, uint16(0), addr.Port(), "bound port should be non-zero")
	require.NoError(t, ln.Stop())
}

// TestListener_DoubleStart rejects the second Start.
func TestListener_DoubleStart(t *testing.T) {
	ln := NewUDPListener(ephemeralBind(t), nil)
	require.NoError(t, ln.Start(context.Background()))
	defer ln.Stop() //nolint:errcheck // test cleanup
	err := ln.Start(context.Background())
	require.Error(t, err)
}

// TestListener_StopIdempotent calls Stop twice.
func TestListener_StopIdempotent(t *testing.T) {
	ln := NewUDPListener(ephemeralBind(t), nil)
	require.NoError(t, ln.Start(context.Background()))
	require.NoError(t, ln.Stop())
	require.NoError(t, ln.Stop())
}

// TestListener_SendReceive round-trips bytes through the listener's RX
// channel using an external UDP client. Proves the slot-pool release
// path and the Send helper.
//
// VALIDATES: AC-2 -- external client can reach the bound port.
func TestListener_SendReceive(t *testing.T) {
	ln := NewUDPListener(ephemeralBind(t), nil)
	require.NoError(t, ln.Start(context.Background()))
	defer ln.Stop() //nolint:errcheck // test cleanup

	// External client socket.
	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	defer client.Close() //nolint:errcheck // test cleanup

	// Client -> listener.
	srvAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(ln.Addr().Port())}
	payload := []byte{0xC8, 0x02, 0x00, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err = client.WriteToUDP(payload, srvAddr)
	require.NoError(t, err)

	// Wait for packet on listener's RX.
	select {
	case pkt := <-ln.RX():
		assert.Equal(t, payload, pkt.bytes)
		pkt.release()
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for packet on listener.RX()")
	}

	// Listener -> client via Send.
	clientLocal, ok := client.LocalAddr().(*net.UDPAddr)
	require.True(t, ok, "client.LocalAddr() should be *net.UDPAddr")
	peer := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(clientLocal.Port))
	require.NoError(t, ln.Send(peer, []byte{0x01, 0x02, 0x03}))

	buf := make([]byte, 16)
	require.NoError(t, client.SetReadDeadline(time.Now().Add(2*time.Second)))
	n, _, err := client.ReadFromUDP(buf)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, buf[:n])
}

// TestListener_SendBeforeStart returns errListenerNotStarted.
func TestListener_SendBeforeStart(t *testing.T) {
	ln := NewUDPListener(ephemeralBind(t), nil)
	peer := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 1)
	err := ln.Send(peer, []byte{0x00})
	require.Error(t, err)
}
