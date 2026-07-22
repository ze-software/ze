// Overview: reactor_notify.go — notifyMessageReceiver, the sent/received callback fan-out

package reactor

import (
	"sync"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/msgtype"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestPeerSessionSentPathRace reproduces the unlocked double-read of peer.session on the
// sent-message path (reactor_notify.go). A sent-direction callback reads peer.session
// (nil-check then field access) with no lock, while the peer run goroutine sets and clears
// peer.session under peer.mu (peer_run.go runOnce). Under -race this is a data race on the
// peer.session pointer; in production it is a non-nil-then-nil panic, or a read of a
// reconnected session's writeMu-guarded sentSourcePeerStr without its writeMu.
//
// After the fix the source-peer string travels as a MessageCallback argument captured at
// the write site under writeMu, so notifyMessageReceiver never re-reads peer.session and
// the race disappears.
//
// VALIDATES: AC-2 — no data race / nil-deref window on the sent path when the session is
// nilled mid-send.
// PREVENTS: the unlocked peer.session re-read returning to notifyMessageReceiver.
func TestPeerSessionSentPathRace(t *testing.T) {
	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	r := New(cfg)

	peerAddr := mustParseAddr("10.0.0.1")
	require.NoError(t, r.AddPeer(NewPeerSettings(peerAddr, 65000, 65001, 0x01010101)))
	r.SetMessageReceiver(&testDeliveryReceiver{})

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
		r.notifyMessageReceiver(peerAddr, msgtype.TypeKEEPALIVE, body, nil, 0, rpc.DirectionSent, BufHandle{}, nil, "")
	}

	close(stop)
	wg.Wait()
}
