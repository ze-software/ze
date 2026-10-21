// Design: docs/architecture/core-design.md — peer run loop and session lifecycle
// Overview: peer_admin.go — the administrative hold these tests drive

package reactor

import (
	"context"
	"io"
	"net"
	"net/netip"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// countingListener accepts and closes every connection, counting them.
func countingListener(t *testing.T) (uint16, *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() }) //nolint:errcheck // test cleanup
	var conns atomic.Int32
	go acceptAndCount(ln, &conns)
	_, portText, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	port, err := strconv.ParseUint(portText, 10, 16)
	require.NoError(t, err)
	return uint16(port), &conns
}

func acceptAndCount(ln net.Listener, conns *atomic.Int32) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		conns.Add(1)
		c.Close() //nolint:errcheck // test: the count is all that matters
	}
}

// TestPeerShutdownContainerHoldsUntilEnable checks the hold end to end on the
// run loop: a peer configured with the shutdown container dials nothing, and
// Enable makes it dial. The positive half proves the silence was the hold and
// not a peer that never started.
func TestPeerShutdownContainerHoldsUntilEnable(t *testing.T) {
	port, conns := countingListener(t)

	settings := NewPeerSettings(mustParseAddr("127.0.0.1"), 65000, 65000, 0x01010101)
	settings.Port = port
	settings.Connection = ConnectionActive
	settings.AdminShutdown = AdminShutdown{Held: true, Message: "maintenance"}

	peer := NewPeer(settings)
	peer.setReconnectDelay(20*time.Millisecond, 40*time.Millisecond)
	stopPeer := startAndStop(t, peer)
	defer stopPeer()

	require.Eventually(t, func() bool { return peer.State() == PeerStateIdleHold },
		2*time.Second, 10*time.Millisecond, "a held peer names itself idle-hold")
	assert.Never(t, func() bool { return conns.Load() > 0 },
		300*time.Millisecond, 20*time.Millisecond, "a held peer must not dial")

	peer.Enable()
	require.Eventually(t, func() bool { return conns.Load() > 0 },
		2*time.Second, 10*time.Millisecond, "an enabled peer dials")

	require.NoError(t, peer.Disable("back soon"))
	assert.True(t, peer.AdminDown())
	require.Eventually(t, func() bool { return peer.State() == PeerStateIdleHold },
		2*time.Second, 10*time.Millisecond, "a disabled peer parks again")
	dialled := conns.Load()
	assert.Never(t, func() bool { return conns.Load() > dialled },
		300*time.Millisecond, 20*time.Millisecond, "a disabled peer must not dial")
}

// TestHoldAdminDownRejectsInboundWithCease checks that a connection reaching a
// held peer gets Cease, Connection Rejected (RFC 4486 Section 4) and that
// Enable ends the hold.
func TestHoldAdminDownRejectsInboundWithCease(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.7"), 65000, 65001, 0x01010101)
	settings.AdminShutdown.Held = true
	peer := NewPeer(settings)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	peer.ctx = ctx

	done := make(chan struct{})
	go func() {
		defer close(done)
		peer.holdAdminDown()
	}()

	client, server := net.Pipe()
	defer client.Close() //nolint:errcheck // test cleanup
	peer.setInboundConnection(server)

	require.NoError(t, client.SetReadDeadline(time.Now().Add(2*time.Second)))
	got, err := io.ReadAll(client)
	require.NoError(t, err)
	require.Len(t, got, message.HeaderLen+2, "one NOTIFICATION with no data")
	assert.Equal(t, byte(msgtype.TypeNOTIFICATION), got[18])
	assert.Equal(t, byte(message.NotifyCease), got[19])
	assert.Equal(t, byte(message.NotifyCeaseConnectionRejected), got[20])

	peer.Enable()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Enable must end the hold")
	}
}

// TestParsePeerSettingsShutdownContainer checks that the presence container
// holds the peer and carries its message, and that its absence holds nothing.
func TestParsePeerSettingsShutdownContainer(t *testing.T) {
	ip := netip.MustParseAddr("192.0.2.1")
	conn := func() map[string]any {
		return map[string]any{"local": map[string]any{"ip": "auto"}}
	}
	held, err := parsePeerSettings("p", map[string]any{
		"connection": conn(),
		"shutdown":   map[string]any{"message": "back soon"},
	}, ip, 65001, 65000, 0x01010101)
	require.NoError(t, err)
	assert.Equal(t, AdminShutdown{Held: true, Message: "back soon"}, held.AdminShutdown)

	bare, err := parsePeerSettings("p", map[string]any{"connection": conn(), "shutdown": map[string]any{}}, ip, 65001, 65000, 0x01010101)
	require.NoError(t, err)
	assert.Equal(t, AdminShutdown{Held: true}, bare.AdminShutdown)

	free, err := parsePeerSettings("p", map[string]any{"connection": conn()}, ip, 65001, 65000, 0x01010101)
	require.NoError(t, err)
	assert.Equal(t, AdminShutdown{}, free.AdminShutdown)
}

// TestAdminShutdownChangeFollowsPresenceOnly checks the reload rule: adding the
// container holds, removing it releases, and an unchanged container leaves a
// runtime enable alone.
func TestAdminShutdownChangeFollowsPresenceOnly(t *testing.T) {
	peer := NewPeer(NewPeerSettings(mustParseAddr("192.0.2.8"), 65000, 65001, 0x01010101))
	peer.applyAdminShutdownChange(AdminShutdown{}, AdminShutdown{Held: true, Message: "x"})
	assert.True(t, peer.AdminDown(), "a commit adding the container holds the peer")

	peer.Enable()
	peer.applyAdminShutdownChange(AdminShutdown{Held: true}, AdminShutdown{Held: true, Message: "y"})
	assert.False(t, peer.AdminDown(), "an unchanged container must not undo a runtime enable")

	require.NoError(t, peer.Disable(""))
	peer.applyAdminShutdownChange(AdminShutdown{Held: true}, AdminShutdown{})
	assert.False(t, peer.AdminDown(), "a commit removing the container releases the peer")
}
