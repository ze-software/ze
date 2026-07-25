package l2tp

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// connectedPair builds the shape the kernel worker hands to the listener: a UDP
// socket connected to a peer (the "tunnel socket", mirroring
// connectedUDPSocket in genl_linux.go) plus the peer socket that talks to it.
// Returns the tunnel socket's raw fd, the tunnel socket's own address, and the
// peer conn. Both are closed on cleanup.
func connectedPair(t *testing.T) (tunnelFD int, tunnelAddr netip.AddrPort, peer *net.UDPConn) {
	t.Helper()

	peer, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	t.Cleanup(func() { _ = peer.Close() })
	peerAddr, ok := peer.LocalAddr().(*net.UDPAddr)
	require.True(t, ok, "peer local addr is not *net.UDPAddr")

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = unix.Close(fd) })

	require.NoError(t, unix.Bind(fd, &unix.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}, Port: 0}))
	require.NoError(t, unix.Connect(fd, &unix.SockaddrInet4{
		Addr: [4]byte{127, 0, 0, 1}, Port: peerAddr.Port,
	}))

	sa, err := unix.Getsockname(fd)
	require.NoError(t, err)
	in4, ok := sa.(*unix.SockaddrInet4)
	require.True(t, ok, "tunnel socket is not AF_INET")

	return fd, netip.AddrPortFrom(netip.AddrFrom4(in4.Addr), uint16(in4.Port)), peer
}

// VALIDATES: the premise the adopt path depends on -- a datagram the peer sends
// to the connected tunnel socket is delivered to THAT socket and readable from
// it. Asserted with a raw recvfrom on the caller's own fd, so a failure here
// means the harness (or the platform) is wrong, not AdoptTunnelSocket.
// PREVENTS: chasing a delivery bug in the adopt path when the socket pair itself
// never carried the datagram.
func TestListener_connectedTunnelSocketReceives(t *testing.T) {
	fd, tunnelAddr, peer := connectedPair(t)

	_, err := peer.WriteToUDP([]byte("premise"), &net.UDPAddr{
		IP: net.IPv4(127, 0, 0, 1), Port: int(tunnelAddr.Port()),
	})
	require.NoError(t, err)

	buf := make([]byte, 128)
	deadline := time.Now().Add(5 * time.Second)
	for {
		n, _, rerr := unix.Recvfrom(fd, buf, unix.MSG_DONTWAIT)
		if rerr == nil {
			assert.Equal(t, "premise", string(buf[:n]))
			return
		}
		if !errors.Is(rerr, unix.EAGAIN) && !errors.Is(rerr, unix.EWOULDBLOCK) {
			t.Fatalf("recvfrom on connected tunnel socket: %v", rerr)
		}
		if time.Now().After(deadline) {
			t.Fatal("connected tunnel socket never received the peer's datagram")
		}
		// poll interval; the loop returns as soon as recvfrom yields the datagram
		time.Sleep(10 * time.Millisecond)
	}
}

// VALIDATES: a control frame arriving on an ADOPTED per-tunnel socket is
// delivered to the listener's RX channel, with the sending peer's address, so
// the reactor handles it through its one existing path.
// PREVENTS: the regression this method exists to fix -- once the kernel worker
// creates a tunnel's connected socket, Linux delivers that peer's datagrams to
// it instead of the listener (compute_score ranks a connected socket above an
// unconnected one), and the kernel passes control frames back up. With nothing
// reading that socket, ze went deaf to the peer's control plane: a second ICRQ,
// CDN, HELLO, or StopCCN was silently dropped once the first session came up.
func TestListener_AdoptedTunnelSocketDeliversToRX(t *testing.T) {
	ln := NewUDPListener(ephemeralBind(t), nil)
	require.NoError(t, ln.Start(context.Background()))
	t.Cleanup(func() { _ = ln.Stop() })

	fd, tunnelAddr, peer := connectedPair(t)
	require.NoError(t, ln.AdoptTunnelSocket(7, fd))

	payload := []byte("control-frame")
	_, err := peer.WriteToUDP(payload, &net.UDPAddr{
		IP: net.IPv4(127, 0, 0, 1), Port: int(tunnelAddr.Port()),
	})
	require.NoError(t, err)

	select {
	case pkt := <-ln.RX():
		assert.Equal(t, payload, pkt.bytes, "adopted socket payload must reach RX intact")
		peerAddr, ok := peer.LocalAddr().(*net.UDPAddr)
		require.True(t, ok)
		assert.Equal(t, uint16(peerAddr.Port), pkt.from.Port(),
			"RX must report the SENDING peer, not the tunnel socket")
		pkt.release()
	case <-time.After(5 * time.Second):
		t.Fatal("no packet on RX from the adopted tunnel socket")
	}
}

// VALIDATES: ReleaseTunnelSocket stops the reader and is idempotent, and the
// caller's own fd stays valid afterwards (the listener only ever closes its dup).
// PREVENTS: a double close of the kernel worker's connFD, and a reader goroutine
// left running on a tunnel that was torn down.
func TestListener_ReleaseTunnelSocketKeepsCallerFD(t *testing.T) {
	ln := NewUDPListener(ephemeralBind(t), nil)
	require.NoError(t, ln.Start(context.Background()))
	t.Cleanup(func() { _ = ln.Stop() })

	fd, _, _ := connectedPair(t)
	require.NoError(t, ln.AdoptTunnelSocket(9, fd))

	ln.ReleaseTunnelSocket(9)
	ln.ReleaseTunnelSocket(9) // idempotent
	ln.ReleaseTunnelSocket(4242)

	// The caller's descriptor is untouched: the listener closed only its dup.
	_, err := unix.Getsockname(fd)
	assert.NoError(t, err, "listener must not close the caller's fd")
}

// VALIDATES: adopting the same tunnel id twice is a no-op that does not start a
// second reader, and adopting on a stopped listener is refused.
// PREVENTS: duplicate delivery of every control frame (one per reader), and a
// goroutine started against a listener that is already shutting down.
func TestListener_AdoptTunnelSocketGuards(t *testing.T) {
	ln := NewUDPListener(ephemeralBind(t), nil)
	require.NoError(t, ln.Start(context.Background()))

	fd, tunnelAddr, peer := connectedPair(t)
	require.NoError(t, ln.AdoptTunnelSocket(3, fd))
	require.NoError(t, ln.AdoptTunnelSocket(3, fd), "re-adopting the same tid must be a no-op")

	_, err := peer.WriteToUDP([]byte("once"), &net.UDPAddr{
		IP: net.IPv4(127, 0, 0, 1), Port: int(tunnelAddr.Port()),
	})
	require.NoError(t, err)

	select {
	case pkt := <-ln.RX():
		pkt.release()
	case <-time.After(5 * time.Second):
		t.Fatal("no packet on RX")
	}
	// A second reader would deliver the same datagram twice.
	select {
	case pkt := <-ln.RX():
		pkt.release()
		t.Fatal("duplicate delivery: re-adopting started a second reader")
	case <-time.After(250 * time.Millisecond):
		// no duplicate; correct
	}

	require.NoError(t, ln.Stop())
	assert.ErrorIs(t, ln.AdoptTunnelSocket(5, fd), errListenerNotStarted,
		"adopting on a stopped listener must be refused")
}
