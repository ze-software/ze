// Design: docs/architecture/route-selection.md
// RFC: rfc/short/rfc7311.md, Section 3.4.1 -- local next hops for originated AIGP.
package reactor

import (
	"bufio"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// VALIDATES: each announced section has a local governing next hop before an
// originated UPDATE can carry AIGP; unrelated NEXT_HOP attributes grant nothing.
// PREVENTS: a local legacy NEXT_HOP authorizing remote MP_REACH destinations.
func TestAIGPOriginChecksEveryAnnouncedSection(t *testing.T) {
	for _, tc := range []struct {
		name                                       string
		legacyNLRI, legacySelf, mpSelf, wantMetric bool
	}{
		{"mp remote with unrelated local legacy attribute", false, true, false, false},
		{"local legacy announcement and remote mp announcement", true, true, false, false},
		{"remote legacy announcement and local mp announcement", true, false, true, false},
		{"both announcements local", true, true, true, true},
		{"mp local with unrelated remote legacy attribute", false, false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer, conn := newAnnouncePeer(t, "192.0.2.2")
			enabled := true
			peer.session.settings.AIGPSession = &enabled
			peer.session.settings.AIGPOriginate = true
			peer.session.settings.LocalAddress = netip.MustParseAddr("192.0.2.1")
			local6 := netip.MustParseAddr("2001:db8::1")
			peer.session.nextHopScope.Store(&receiveNextHopScope{local: local6})
			body := aigpTestBody(99)
			sections, err := wire.ParseUpdateSections(body)
			require.NoError(t, err)
			attrs := append([]byte(nil), sections.Attrs(body)...)
			if !tc.legacySelf {
				_, _, nextHop, found := attribute.AttrFind(attrs, attribute.AttrNextHop)
				require.True(t, found)
				copy(nextHop, []byte{192, 0, 2, 9})
			}
			nextHop := netip.MustParseAddr("2001:db8::2")
			if tc.mpSelf {
				nextHop = local6
			}
			address := nextHop.As16()
			reach := append([]byte{0, 2, 1, 16}, address[:]...)
			reach = append(reach, 0, 48, 0x20, 0x01, 0x0d, 0xb8, 0, 0)
			attrs = append(attrs, 0x80, byte(attribute.AttrMPReachNLRI), byte(len(reach)))
			attrs = append(attrs, reach...)
			var legacy []byte
			if tc.legacyNLRI {
				legacy = sections.NLRI(body)
			}
			body = makeUpdateBody(nil, attrs, legacy)
			require.NoError(t, peer.session.SendRawMessage(uint8(msgtype.TypeUPDATE), body))
			out := conn.written()[message.HeaderLen:]
			metric, present := aigpReceivedMetric(t, out)
			require.Equal(t, tc.wantMetric, present)
			if present {
				require.Equal(t, uint64(99), metric)
			}
			written, err := wire.ParseUpdateSections(out)
			require.NoError(t, err)
			require.Equal(t, legacy, written.NLRI(out))
			_, _, sentReach, found := attribute.AttrFind(written.Attrs(out), attribute.AttrMPReachNLRI)
			require.True(t, found)
			require.Equal(t, reach, sentReach, "removing AIGP must preserve the MP announcement")
		})
	}
}

// VALIDATES: an originating writer can finish while teardown's session lock is held.
// PREVENTS: writeMu -> Negotiated's session.mu reversing closeConn's lock order.
func TestAIGPOriginWriteDoesNotDeadlockTeardown(t *testing.T) {
	peer, _ := newAnnouncePeer(t, "192.0.2.2")
	session := peer.session
	enabled := true
	session.settings.AIGPSession = &enabled
	session.settings.AIGPOriginate = true
	session.settings.LocalAddress = netip.MustParseAddr("192.0.2.1")
	writer, remote := net.Pipe()
	session.conn = writer
	session.bufWriter = bufio.NewWriterSize(writer, 4096)
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	session.egressRouteFilter = func([]byte) (bool, []byte) {
		close(entered)
		<-release
		return false, nil
	}
	body := aigpTestBody(99)
	sections, err := wire.ParseUpdateSections(body)
	require.NoError(t, err)
	written := make(chan error, 1)
	locked, completed := false, false
	defer func() {
		// Owning the first teardown lock in the test makes even the old
		// inversion recoverable: failure releases it before joining the writer.
		releaseOnce.Do(func() { close(release) })
		if locked {
			session.mu.Unlock()
		}
		_ = remote.Close()
		_ = writer.Close()
		if !completed {
			<-written
		}
		session.closeConn()
	}()
	go func() {
		written <- session.SendUpdate(&message.Update{PathAttributes: sections.Attrs(body), NLRI: sections.NLRI(body)})
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("originated writer did not enter its export filter")
	}
	// closeConn takes session.mu before waiting for writeMu. Hold that exact
	// first lock while the already-running writer reaches AIGP authorization.
	session.mu.Lock()
	locked = true
	require.NoError(t, remote.Close())
	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-written:
		completed = true
		require.Error(t, err, "the closed pipe must fail the actual socket flush")
	case <-time.After(5 * time.Second):
		t.Fatal("AIGP origin authorization waits on teardown's session lock")
	}
}
