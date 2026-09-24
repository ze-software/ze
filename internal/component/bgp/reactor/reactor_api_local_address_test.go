// Design: docs/architecture/wire/nlri-bgpls.md -- native EPE session identity

package reactor

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func newConnectedLocalSession(t *testing.T, settings *PeerSettings, local string) (*Session, net.Conn) {
	t.Helper()
	listener, err := net.Listen("tcp4", net.JoinHostPort(local, "0"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	var dialer net.Dialer
	client, err := dialer.DialContext(t.Context(), "tcp4", listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.SetReadDeadline(time.Now().Add(10*time.Second)))
	connected, err := listener.Accept()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connected.Close() })
	session := NewSession(settings)
	require.NoError(t, session.Start())
	require.NoError(t, session.Accept(connected))
	t.Cleanup(session.closeConn)
	t.Cleanup(session.timers.StopAll)
	open, err := core4271ReadMessage(client)
	require.NoError(t, err)
	require.Equal(t, byte(msgtype.TypeOPEN), open[18])
	return session, client
}

// VALIDATES: a real TCP connection selected from local-ip auto publishes its
// actual local endpoint to native state-event consumers such as BGP EPE.
// PREVENTS: an empty configured address making a live native session unusable.
func TestEstablishedStateEventUsesConnectedLocalAddress(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0xc0000201)
	// LocalAddress remains invalid, exactly as local-ip auto configuration does.
	settings.LocalAddress = netip.Addr{}
	session, _ := newConnectedLocalSession(t, settings, "127.0.0.1")
	connected := session.Conn()
	peer := NewPeer(settings)
	peer.session = session
	peer.remoteRouterID.Store(0xc0000202)
	info := establishedPeerInfo(peer)
	wire := format.AppendStateChange(nil, &info, rpc.SessionStateUp, "", nil, plugin.EncodingJSON)
	var event struct {
		BGP struct {
			Peer struct {
				Local struct {
					Address string `json:"address"`
				} `json:"local"`
			} `json:"peer"`
		} `json:"bgp"`
	}
	require.NoError(t, json.Unmarshal(wire, &event))
	require.Equal(t, "127.0.0.1", event.BGP.Peer.Local.Address)
	// Structured in-process events consume the textual endpoint accessor instead.
	require.Equal(t, "127.0.0.1", info.LocalAddrStr())
	localEndpoint := connected.LocalAddr().(*net.TCPAddr).AddrPort()
	remoteEndpoint := connected.RemoteAddr().(*net.TCPAddr).AddrPort()
	require.Equal(t, localEndpoint.Port(), info.LocalPort)
	require.Equal(t, remoteEndpoint.Port(), info.RemotePort)

	// A consumer inspecting an already-established session needs the same
	// identity, including the peer's OPEN identifier rather than the local one.
	api := &reactorAPIAdapter{r: &Reactor{peers: map[netip.AddrPort]*Peer{settings.PeerKey(): peer}}}
	peers := api.Peers()
	require.Len(t, peers, 1)
	require.Equal(t, "127.0.0.1", peers[0].LocalAddrStr())
	require.Equal(t, uint32(0xc0000202), peers[0].RemoteRouterID)
	require.Equal(t, localEndpoint.Port(), peers[0].LocalPort)
	require.Equal(t, remoteEndpoint.Port(), peers[0].RemotePort)

	// Sent callbacks run while writeMu is held. They must carry the same
	// socket identity without acquiring the oppositely ordered session.mu.
	api.r.clock = clock.RealClock{}
	var delivered plugin.PeerInfo
	api.r.messageReceiver = &testDeliveryReceiver{
		onSent: func(peer plugin.PeerInfo, _ bgptypes.RawMessage) { delivered = peer },
	}
	session.SetMessageCallback(api.r.notifyMessageReceiver)
	require.NoError(t, session.sendKeepalive(connected))
	require.Equal(t, localEndpoint.Addr().Unmap(), delivered.LocalAddress)
	require.Equal(t, localEndpoint.Port(), delivered.LocalPort)
	require.Equal(t, remoteEndpoint.Port(), delivered.RemotePort)
}

// VALIDATES: both queue drains retain self as a policy across a connection
// replacement and emit the replacement socket's local endpoint.
// PREVENTS: a queued announcement silently carrying the previous connection's
// address, or a local-ip auto route being dropped before it can be resolved.
func TestQueuedSelfResolvesReplacementConnection(t *testing.T) {
	for _, finalDrain := range []bool{false, true} {
		name := "initial-drain"
		if finalDrain {
			name = "closing-drain"
		}
		t.Run(name, func(t *testing.T) {
			peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
			peer.settings.LocalAddress = netip.Addr{}
			oldSession, oldClient := newConnectedLocalSession(t, peer.settings, "127.0.0.1")
			peer.session = oldSession
			api := &reactorAPIAdapter{r: &Reactor{
				config:          &Config{LocalAS: 65000},
				attrModHandlers: attrModHandlersWithDefaults(),
				peers:           map[netip.AddrPort]*Peer{peer.settings.PeerKey(): peer},
			}}
			batch := bgptypes.NLRIBatch{
				Family:  family.IPv4Unicast,
				NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.20.0.0/24"), 0)},
				NextHop: bgptypes.NewNextHopSelf(),
			}
			require.NoError(t, api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender()))
			oldSession.closeConn()
			_, err := core4271ReadMessage(oldClient)
			require.ErrorIs(t, err, io.EOF, "the queued announcement must not reach the old connection")

			replacement, client := newConnectedLocalSession(t, peer.settings, "127.0.0.2")
			require.NoError(t, replacement.fsm.Event(fsm.EventBGPOpen))
			require.NoError(t, replacement.fsm.Event(fsm.EventKeepaliveMsg))
			peer.session = replacement
			if finalDrain {
				peer.drainAndCloseQueueGate(peer.addrString, message.MaxMsgLen)
			} else {
				peer.sendInitialRoutes()
			}
			packet, err := core4271ReadMessage(client)
			require.NoError(t, err)
			require.Equal(t, byte(msgtype.TypeUPDATE), packet[18])
			body := packet[message.HeaderLen:]
			attrLen := int(binary.BigEndian.Uint16(body[2:4]))
			attrs := body[4 : 4+attrLen]
			_, hop, found := findPathAttr(attrs, byte(attribute.AttrNextHop))
			require.True(t, found)
			require.Equal(t, []byte{127, 0, 0, 2}, hop)
			require.Equal(t, []byte{24, 10, 20, 0}, body[4+attrLen:])
		})
	}
}

type replacingSessionNLRI struct {
	nlri.NLRI
	once              sync.Once
	beforeReplacement int
	replace           func()
}

func (n *replacingSessionNLRI) WriteTo(buf []byte, off int) int {
	if n.beforeReplacement > 0 {
		n.beforeReplacement--
	} else {
		n.once.Do(n.replace)
	}
	return n.NLRI.WriteTo(buf, off)
}

// VALIDATES: a live self batch remains bound to the connection whose endpoint
// was resolved, including grouped, partially suppressed and stale delivery.
// PREVENTS: replacing that connection during encoding from leaking its local
// address into an UPDATE written to the replacement connection.
func TestBatchSelfDoesNotCrossReplacementSession(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		grouped, partial, stale bool
	}{
		{name: "direct"},
		{name: "grouped", grouped: true},
		{name: "partial", partial: true},
		{name: "stale", stale: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
			peer.settings.GroupUpdates = tc.partial
			peer.sendingInitialRoutes.Store(0)
			peer.initialSyncEOROwed.Store(false)
			oldSession, oldClient := newConnectedLocalSession(t, peer.settings, "127.0.0.1")
			replacement, client := newConnectedLocalSession(t, peer.settings, "127.0.0.2")
			for _, session := range []*Session{oldSession, replacement} {
				require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
				require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
			}
			peer.session = oldSession
			api := &reactorAPIAdapter{r: &Reactor{
				config:          &Config{LocalAS: 65000},
				attrModHandlers: attrModHandlersWithDefaults(),
				peers:           map[netip.AddrPort]*Peer{peer.settings.PeerKey(): peer},
				updateGroups:    newUpdateGroupIndex(tc.grouped),
			}}
			replaced := false
			route := &replacingSessionNLRI{
				NLRI: nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.21.0.0/24"), 0),
				replace: func() {
					peer.mu.Lock()
					peer.session = replacement
					peer.mu.Unlock()
					oldSession.closeConn()
					replaced = true
				},
			}
			batch := bgptypes.NLRIBatch{
				Family:  family.IPv4Unicast,
				NLRIs:   []nlri.NLRI{route},
				NextHop: bgptypes.NewNextHopSelf(),
			}
			if tc.partial {
				seed := batch
				seed.NLRIs = []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.20.0.0/24"), 0)}
				require.NoError(t, api.AnnounceNLRIBatch(t.Context(), selector.All(), seed, plugin.OperatorSender()))
				packet, err := core4271ReadMessage(oldClient)
				require.NoError(t, err)
				require.Equal(t, byte(msgtype.TypeUPDATE), packet[18])
				batch.NLRIs = append(seed.NLRIs, route)
				route.beforeReplacement = 1 // Replace during the partial rebuild, not the shared build.
			}
			if tc.stale {
				batch.Stale = 1
				api.r.readvertiseEgressFilters = append(api.r.readvertiseEgressFilters, keepStaleReadvertise)
			}
			err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender())
			require.True(t, replaced, "the build must reach the replacement boundary")
			require.Error(t, err, "the captured connection closed before this batch could be sent")
			replacement.closeConn()
			_, err = core4271ReadMessage(client)
			require.ErrorIs(t, err, io.EOF, "the replacement connection must receive no old-session UPDATE")
		})
	}
}

// VALIDATES: a static set, including a split grouped UPDATE, stays on the
// connection captured before its self next hops were resolved.
// PREVENTS: later routes or chunks moving to a replacement session while the
// static receipt still identifies the original connection.
func TestStaticSelfStaysOnCapturedSession(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		name := "ungrouped"
		if grouped {
			name = "grouped-split"
		}
		t.Run(name, func(t *testing.T) {
			peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
			session, client := newConnectedLocalSession(t, peer.settings, "127.0.0.1")
			replacement, replacementClient := newConnectedLocalSession(t, peer.settings, "127.0.0.2")
			for _, active := range []*Session{session, replacement} {
				require.NoError(t, active.fsm.Event(fsm.EventBGPOpen))
				require.NoError(t, active.fsm.Event(fsm.EventKeepaliveMsg))
			}
			peer.session = session
			replaced := false
			r := &Reactor{
				clock: clock.RealClock{},
				messageReceiver: &testDeliveryReceiver{
					onSent: func(plugin.PeerInfo, bgptypes.RawMessage) {
						peer.mu.Lock()
						peer.session = replacement
						peer.mu.Unlock()
						replaced = true
					},
				},
			}
			session.SetMessageCallback(r.notifyMessageReceiver)
			var routes []StaticRoute
			want := make(map[string]bool)
			for i := byte(0); i < 16; i++ {
				routes = append(routes, StaticRoute{
					Prefix:  netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 20, i, 0}), 24),
					NextHop: bgptypes.NewNextHopSelf(),
				})
				want[string([]byte{24, 10, 20, i})] = true
			}
			peer.sendStaticRoutes(session, routes, grouped, 80, false)
			require.True(t, replaced, "the first frame must trigger the session replacement")
			session.closeConn()
			replacement.closeConn()
			got := make(map[string]bool)
			frames := 0
			for {
				packet, err := core4271ReadMessage(client)
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				require.Equal(t, byte(msgtype.TypeUPDATE), packet[18])
				body := packet[message.HeaderLen:]
				require.GreaterOrEqual(t, len(body), 4)
				attrLen := int(binary.BigEndian.Uint16(body[2:4]))
				require.LessOrEqual(t, 4+attrLen, len(body))
				_, hop, found := findPathAttr(body[4:4+attrLen], byte(attribute.AttrNextHop))
				require.True(t, found)
				require.Equal(t, []byte{127, 0, 0, 1}, hop)
				for prefixes := body[4+attrLen:]; len(prefixes) > 0; prefixes = prefixes[4:] {
					require.GreaterOrEqual(t, len(prefixes), 4)
					require.Equal(t, byte(24), prefixes[0])
					got[string(prefixes[:4])] = true
				}
				frames++
			}
			require.Greater(t, frames, 1, "the replacement must occur before the remaining frames")
			require.Equal(t, want, got)
			_, err := core4271ReadMessage(replacementClient)
			require.ErrorIs(t, err, io.EOF, "no static frame belongs to the replacement connection")
		})
	}
}
