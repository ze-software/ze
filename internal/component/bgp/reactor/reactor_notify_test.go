// Overview: reactor_notify.go — notifyMessageReceiver, the sent/received callback fan-out

package reactor

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPeerSessionSentPathRace reproduces the unlocked double-read of peer.session on the
// sent-message path (reactor_notify.go). A sent-direction callback reads peer.session
// (nil-check then field access) with no lock, while the peer run goroutine sets and clears
// peer.session under peer.mu (peer_run.go runOnce). Under -race this is a data race on the
// peer.session pointer; in production it is a non-nil-then-nil panic, or a read of a
// reconnected session's writeMu-guarded sentSourcePeerStr without its writeMu.
//
// After the fix the source-peer string travels as a MessageCallback argument
// captured at the write site under writeMu. Socket metadata is read from the
// session's immutable atomic snapshot after one guarded peer.session load.
//
// VALIDATES: AC-2 — no data race / nil-deref window on the sent path when the session is
// nilled mid-send.
// PREVENTS: the unlocked peer.session re-read returning to notifyMessageReceiver.
func TestPeerSessionSentPathRace(t *testing.T) {
	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	r := New(cfg)

	peerAddr := mustParseAddr("10.0.0.1")
	require.NoError(t, r.AddPeer(NewPeerSettings(peerAddr, 65000, 65001, 0x01010101)))
	r.setMessageReceiver(&testDeliveryReceiver{})

	r.mu.RLock()
	peer, ok := r.findPeerByAddr(peerAddr)
	r.mu.RUnlock()
	require.True(t, ok)

	session := NewSession(peer.settings)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Writer: emulate runOnce setting/clearing p.session under p.mu.
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			peer.mu.Lock()
			peer.session = session
			peer.mu.Unlock()
			peer.mu.Lock()
			peer.session = nil
			peer.mu.Unlock()
		}
	})

	// Reader: a sent-direction KEEPALIVE notification. On the unfixed code this reads
	// peer.session unlocked at the sent-path branch of notifyMessageReceiver.
	body := []byte{} // KEEPALIVE has no body
	for range 4000 {
		r.notifyMessageReceiver(peerAddr, msgtype.TypeKEEPALIVE, body, nil, 0, rpc.DirectionSent, BufHandle{}, nil, "", 0)
	}

	close(stop)
	wg.Wait()
}

// TestKeepaliveNamesTheNegotiatedASOfADynamicPeer proves that the message event
// of a dynamic peer names the AS its OPEN negotiated, both ways, while the
// session is still before Established. Method: a peer configured with no AS
// (dynamic), a session whose negotiation resolved AS 65002, and one sent and
// one received KEEPALIVE through notifyMessageReceiver. The peer's settings
// learn the AS only at Established, so reading them reported AS 0.
func TestKeepaliveNamesTheNegotiatedASOfADynamicPeer(t *testing.T) {
	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	r := New(cfg)

	peerAddr := mustParseAddr("10.0.0.1")
	require.NoError(t, r.AddPeer(NewPeerSettings(peerAddr, 65001, 0, 0x01010101)))
	var sentAS, receivedAS []uint32
	r.setMessageReceiver(&testDeliveryReceiver{
		onSent: func(p plugin.PeerInfo, _ bgptypes.RawMessage) {
			peerAS, _ := p.PeerAS.ASN()
			sentAS = append(sentAS, peerAS)
		},
		onReceived: func(p plugin.PeerInfo, _ bgptypes.RawMessage) {
			peerAS, _ := p.PeerAS.ASN()
			receivedAS = append(receivedAS, peerAS)
		},
	})

	r.mu.RLock()
	peer, ok := r.findPeerByAddr(peerAddr)
	r.mu.RUnlock()
	require.True(t, ok)
	require.Zero(t, peer.PeerAS(), "the peer must be dynamic: no AS before Established")

	session := NewSession(peer.settings)
	session.negotiated = capability.Negotiate(
		[]capability.Capability{&capability.ASN4{ASN: 65001}},
		[]capability.Capability{&capability.ASN4{ASN: 65002}},
		capability.PeerIdentity{LocalASN: 65001, PeerASN: 65002},
	)
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	r.notifyMessageReceiver(peerAddr, msgtype.TypeKEEPALIVE, []byte{}, nil, 0, rpc.DirectionSent, BufHandle{}, nil, "", 0)
	r.notifyMessageReceiver(peerAddr, msgtype.TypeKEEPALIVE, []byte{}, nil, 0, rpc.DirectionReceived, BufHandle{}, nil, "", 0)

	require.Equal(t, []uint32{65002}, sentAS, "sent KEEPALIVE must name the negotiated AS")
	require.Equal(t, []uint32{65002}, receivedAS, "received KEEPALIVE must name the negotiated AS")
}

// TestPreNegotiationEventsDoNotNameAS0 proves that a dynamic peer's message
// events written before the OPENs are exchanged say the peer's AS is not yet
// known, in every encoding, rather than naming AS 0. Method: a peer configured
// with no AS (dynamic) and no session, the OPEN Ze sends and a NOTIFICATION
// sent in OpenSent notified through notifyMessageReceiver, and the PeerInfo
// each one carried rendered by the JSON and text formatters plugins receive.
// RFC 7607 Section 2 makes AS 0 impossible as a peer's AS, so a 0 here is a
// value the event invented.
func TestPreNegotiationEventsDoNotNameAS0(t *testing.T) {
	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	r := New(cfg)

	peerAddr := mustParseAddr("10.0.0.1")
	require.NoError(t, r.AddPeer(NewPeerSettings(peerAddr, 65001, 0, 0x01010101)))
	var sent []plugin.PeerInfo
	r.setMessageReceiver(&testDeliveryReceiver{
		onSent: func(p plugin.PeerInfo, _ bgptypes.RawMessage) { sent = append(sent, p) },
	})

	r.notifyMessageReceiver(peerAddr, msgtype.TypeOPEN, []byte{4, 0xfd, 0xe9, 0, 90, 1, 1, 1, 1, 0}, nil, 0, rpc.DirectionSent, BufHandle{}, nil, "", 0)
	r.notifyMessageReceiver(peerAddr, msgtype.TypeNOTIFICATION, []byte{6, 2}, nil, 0, rpc.DirectionSent, BufHandle{}, nil, "", 0)
	require.Len(t, sent, 2)

	enc := format.NewJSONEncoder("")
	notify := format.DecodedNotification{ErrorCode: 6, ErrorSubcode: 2}
	for i, out := range []string{
		enc.Open(&sent[0], format.DecodedOpen{ASN: 65001}, rpc.DirectionSent, 1),
		enc.Notification(&sent[1], notify, rpc.DirectionSent, 2),
		enc.Keepalive(&sent[1], rpc.DirectionSent, 3),
	} {
		var event struct {
			BGP struct {
				Peer struct {
					Remote map[string]any `json:"remote"`
				} `json:"peer"`
			} `json:"bgp"`
		}
		require.NoError(t, json.Unmarshal([]byte(out), &event), out)
		require.NotContains(t, event.BGP.Peer.Remote, "as", "JSON event %d names an AS the peer has not told yet: %s", i, out)
	}

	for i, line := range []string{
		string(format.AppendNotification(nil, &sent[1], notify, rpc.DirectionSent, 2)),
		string(format.AppendKeepalive(nil, &sent[1], rpc.DirectionSent, 3)),
		string(format.AppendConnected(nil, &sent[1], plugin.EncodingText)),
	} {
		require.True(t, strings.HasPrefix(line, "peer 10.0.0.1 remote as unknown "), "text event %d must say the AS is unknown: %q", i, line)
	}
	connected := string(format.AppendConnected(nil, &sent[1], plugin.EncodingJSON))
	require.NotContains(t, connected, `"as":0`, "JSON connected event names AS 0: %s", connected)
}
