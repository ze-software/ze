// Design: docs/architecture/mrt.md — capture the actual local OPEN identity per connection.
package reactor

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/netip"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/mrt"
	mrtplugin "github.com/ze-software/ze/internal/plugins/mrt"
)

// RFC requirement: RFC8050-x-4 positive -- actual migration-fallback OPENs supply the recorded local ASN and mixed withdrawal decoding context in both directions; original packets, exact prefixes and Path Identifiers survive, including through a retained outbound writer after the Session transport changes.
func TestMRTMigrationEpochUsesActualLocalOPEN(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), migrationLocalAS, migrationLocalAS, 0x0a000002)
	require.NoError(t, setMigrationAS(settings, migrationLegacyAS))
	settings.Capabilities = []capability.Capability{
		&capability.Multiprotocol{AFI: 1, SAFI: 1}, &capability.Multiprotocol{AFI: 2, SAFI: 1},
		&capability.AddPath{Families: []capability.AddPathFamily{{AFI: 1, SAFI: 1, Mode: capability.AddPathSend}, {AFI: 2, SAFI: 1, Mode: capability.AddPathReceive}}},
	}
	path := filepath.Join(t.TempDir(), "migration.mrt")
	recorder := mrtplugin.New(mrtplugin.Config{AllPath: path}, nil)
	recorder.Start(nil)
	stop := sync.OnceFunc(recorder.Stop)
	t.Cleanup(stop)
	r := New(&Config{})
	r.addMessageObserver(recorder)
	peer := plugin.PeerInfo{Address: settings.Address, LocalAS: migrationLocalAS, PeerAS: migrationLocalAS}
	var fallback atomic.Bool
	connect := func() (*Session, net.Conn, []byte) {
		s := NewSession(settings)
		s.asMigrationFallback = &fallback
		s.onWireMessage = func(wire []byte, id bgpctx.ContextID, sent bool, transport *sessionTransport) {
			r.dispatchObservedWire(&peer, wire, id, sent, transport)
		}
		startSession(t, s)
		t.Cleanup(s.timers.StopAll)
		t.Cleanup(s.stopSendHoldTimer)
		listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer func() {
			if err := listener.Close(); err != nil {
				t.Errorf("close listener: %v", err)
			}
		}()
		client, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", listener.Addr().String())
		require.NoError(t, err)
		t.Cleanup(func() {
			if err := client.Close(); err != nil {
				t.Errorf("close client: %v", err)
			}
		})
		server, err := listener.Accept()
		require.NoError(t, err)
		t.Cleanup(func() {
			// Session teardown may already have closed its accepted socket.
			if err := server.Close(); err != nil {
				if !errors.Is(err, net.ErrClosed) {
					t.Errorf("close server: %v", err)
				}
			}
		})
		require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))
		require.NoError(t, s.Accept(server))
		return s, client, readMRTTestPacket(t, client)
	}
	first, client, firstOpen := connect()
	require.Equal(t, uint16(migrationLocalAS), uint16(firstOpen[20])<<8|uint16(firstOpen[21]))
	_, err := client.Write(message.PackTo(&message.Notification{ErrorCode: message.NotifyOpenMessage, ErrorSubcode: message.NotifyOpenBadPeerAS}, nil))
	require.NoError(t, err)
	require.Error(t, first.ReadAndProcess())
	require.True(t, fallback.Load(), "real Bad Peer AS must select the next connection's fallback")

	s, client, localOpen := connect()
	require.Equal(t, uint16(migrationLegacyAS), uint16(localOpen[20])<<8|uint16(localOpen[21]))
	caps := []capability.Capability{&capability.ASN4{ASN: migrationLocalAS}, &capability.Multiprotocol{AFI: 1, SAFI: 1}, &capability.Multiprotocol{AFI: 2, SAFI: 1}, &capability.AddPath{Families: []capability.AddPathFamily{{AFI: 1, SAFI: 1, Mode: capability.AddPathReceive}, {AFI: 2, SAFI: 1, Mode: capability.AddPathSend}}}}
	params, extended := buildOptionalParams(caps)
	remoteOpen := message.PackTo(&message.Open{Version: 4, MyAS: migrationLocalAS, ASN4: migrationLocalAS, HoldTime: 90, BGPIdentifier: 0x0a000001, OptionalParams: params, ExtendedParams: extended}, nil)
	_, err = client.Write(remoteOpen)
	require.NoError(t, err)
	require.NoError(t, s.ReadAndProcess())
	require.Equal(t, byte(4), readMRTTestPacket(t, client)[18])
	_, err = client.Write(message.PackTo(message.NewKeepalive(), nil))
	require.NoError(t, err)
	require.NoError(t, s.ReadAndProcess())

	// A standalone Session has no Peer to publish encoding contexts on
	// Established. Use the negotiation produced by these actual OPENs, just
	// as Peer.setEncodingContexts does, before observing either UPDATE.
	negotiated := s.Negotiated()
	require.NotNil(t, negotiated)
	recvID, err := bgpctx.Registry.Register(bgpctx.FromNegotiatedRecv(negotiated))
	require.NoError(t, err)
	sendID, err := bgpctx.Registry.Register(bgpctx.FromNegotiatedSend(negotiated))
	require.NoError(t, err)
	s.setRecvCtxID(recvID)
	s.setSendCtxID(sendID)

	// Distinct classic and MP withdrawals need no NEXT_HOP or mandatory
	// announcement attributes, and exercise opposite direction-specific modes.
	incoming := buildUpdateMsg([]byte{0, 4, 24, 10, 1, 0, 0, 15, 0x80, 15, 12, 0, 2, 1, 1, 2, 3, 4, 32, 0x20, 1, 0x0d, 0xb9})
	_, err = client.Write(incoming)
	require.NoError(t, err)
	require.NoError(t, s.ReadAndProcess())
	outgoing := buildUpdateMsg([]byte{0, 8, 5, 6, 7, 8, 24, 10, 2, 0, 0, 11, 0x80, 15, 8, 0, 2, 1, 32, 0x20, 1, 0x0d, 0xba})
	old := s.wireWriter
	// A retained writer must not consult the replacement transport or mutable
	// migration decision when it finally accepts the old epoch's packet.
	fallback.Store(false)
	s.transport.Store(&sessionTransport{local: netip.MustParseAddr("192.0.2.99")})
	s.writeMu.Lock()
	_, err = old.Write(outgoing)
	s.writeMu.Unlock()
	require.NoError(t, err)
	require.Equal(t, outgoing, readMRTTestPacket(t, client))
	stop()

	var opens [][]byte
	updates := 0
	require.NoError(t, mrt.ReadFile(path, &mrt.Handler{OnMessage: func(h mrt.Header, _ uint32, record *mrt.MessageRecord) error {
		if record.BGPMessage.Bytes[18] == 1 {
			wantAS := uint32(migrationLegacyAS)
			if bytes.Equal(record.BGPMessage.Bytes, firstOpen) {
				wantAS = migrationLocalAS
			}
			require.Equal(t, wantAS, record.LocalAS)
			opens = append(opens, bytes.Clone(record.BGPMessage.Bytes))
			return nil
		}
		if record.BGPMessage.Bytes[18] != 2 {
			return nil
		}
		updates++
		require.Equal(t, uint32(migrationLegacyAS), record.LocalAS, "record identity must match the actual fallback OPEN, not configured LocalAS")
		parsed, err := mrt.ParseBGPMessage(record.BGPMessage)
		require.NoError(t, err)
		u := parsed.Update
		mp, err := mrt.ParseMPUnreach(mrt.FindAttribute(u.Attributes, 15).Value, u.AddPathFor(2, 1))
		require.NoError(t, err)
		if h.Subtype == mrt.BGP4MPMessageAS4AP {
			require.Equal(t, incoming, record.BGPMessage.Bytes)
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("10.1.0.0/24")}, u.WithdrawnPrefixes)
			require.Empty(t, u.WithdrawnPathIDs)
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("2001:db9::/32")}, mp.Prefixes)
			require.Equal(t, []uint32{0x01020304}, mp.PathIDs)
		} else {
			require.Equal(t, mrt.BGP4MPMessageAS4LocalAP, h.Subtype)
			require.Equal(t, outgoing, record.BGPMessage.Bytes)
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("10.2.0.0/24")}, u.WithdrawnPrefixes)
			require.Equal(t, []uint32{0x05060708}, u.WithdrawnPathIDs)
			require.Equal(t, []netip.Prefix{netip.MustParsePrefix("2001:dba::/32")}, mp.Prefixes)
			require.Empty(t, mp.PathIDs)
		}
		return nil
	}}))
	require.Equal(t, 2, updates)
	require.Equal(t, [][]byte{firstOpen, localOpen, remoteOpen}, opens)
}

func readMRTTestPacket(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	header := make([]byte, 19)
	_, err := io.ReadFull(conn, header)
	require.NoError(t, err)
	length := int(header[16])<<8 | int(header[17])
	require.GreaterOrEqual(t, length, 19)
	header = append(header, make([]byte, length-19)...)
	_, err = io.ReadFull(conn, header[19:])
	require.NoError(t, err)
	return header
}
