// Overview: reactor_dynamic.go — dynamic peer settings resolution on establishment

package reactor

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestDynamicPeerSettingsRace reproduces the unlocked mutation of a dynamic peer's
// settings (reactor_dynamic.go resolveDynamicPeerSettings) racing a cross-goroutine
// reader. On establishment the peer writes settings.PeerAS and the ImportFilters/
// ExportFilters slice headers with no lock, while the OPEN-validation goroutine reads
// settings.PeerAS to scope the RFC 6286 Section 2.1 identifier claim (peer.go claimPeerAS).
// Under -race this is a data race on settings.PeerAS; the slice-header writes can tear a
// multi-word read.
//
// After the fix the establishment write holds p.mu and every cross-goroutine reader goes
// through the p.mu-guarded PeerAS()/ImportFilters()/ExportFilters() accessors.
//
// VALIDATES: AC-3 — no torn/stale PeerAS read; the conflict check sees old or new, never
// a mix.
// PREVENTS: the unlocked settings.PeerAS write and read racing across goroutines.
func TestDynamicPeerSettingsRace(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("185.1.69.10"), 65000, 0, 0x01020304)
	settings.IsDynamic = true
	settings.ImportFilters = []filterapi.FilterRef{{Name: "in-$remote_as"}}
	settings.ExportFilters = []filterapi.FilterRef{{Name: "out-$remote_as"}}

	peer := NewPeer(settings)
	session := NewSession(settings)
	session.mu.Lock()
	session.peerOpen = &message.Open{MyAS: 65001, BGPIdentifier: 0x0a0b0c0d}
	session.mu.Unlock()
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	r := &Reactor{peers: make(map[netip.AddrPort]*Peer)}
	r.peers[settings.PeerKey()] = peer

	// the "other peer key" argument existed only for checkRouterIDConflict's
	// self-exclusion. That scan is gone: the RFC 6286 Section 2.1 check is now a claim keyed
	// on the peer itself, so the reader below reads THIS peer's PeerAS -- the same field, the
	// same cross-goroutine race, one fewer indirection. No assertion is dropped.

	// Resolve once synchronously so PeerAS == 65001 regardless of how the writer goroutine
	// is scheduled. Without this the final assertion is scheduling-dependent: under -cpu=1,
	// if the writer is first scheduled after close(stop), its select takes the ready <-stop
	// and returns having never resolved, leaving PeerAS == 0. The writer loop below still
	// re-resolves concurrently with the reader for the -race exposure.
	peer.resolveDynamicPeerSettings(session)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Writer: dynamic establishment resolves PeerAS + filter slices, repeatedly (each
	// reconnection re-resolves).
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			peer.resolveDynamicPeerSettings(session)
		}
	})

	// Reader: OPEN validation scopes the RFC 6286 Section 2.1 identifier claim to the peer's
	// AS, reading PeerAS through the p.mu-guarded accessor (peer.go claimPeerAS) while the
	// writer above republishes it.
	remoteOpen := &message.Open{MyAS: 65001, BGPIdentifier: 0x0a0b0c0d}
	for range 4000 {
		peer.claimPeerAS(remoteOpen)
	}

	close(stop)
	wg.Wait()

	require.Equal(t, uint32(65001), peer.PeerAS(), "PeerAS resolved from the OPEN")
}
