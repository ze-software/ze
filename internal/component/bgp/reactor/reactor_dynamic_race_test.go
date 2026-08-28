// Overview: reactor_dynamic.go — dynamic peer settings resolution on establishment

package reactor

import (
	"net/netip"
	"strings"
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

// TestDynamicPeerSettingsFilterReadRacesTheReload proves resolveDynamicPeerSettings
// reads the filter pair under p.mu.
//
// Goal: ImportFilters and ExportFilters are two of the three hot-swappable settings
// fields, and applyHotSwappableSettings (peer_settings_apply.go) writes both from the
// config-reload goroutine under p.mu. resolveDynamicPeerSettings read them with no
// lock, on the establishment goroutine, which is a data race on a two-word slice
// header: the reader can take the new pointer with the old length.
//
// Method: one goroutine re-resolves the dynamic peer, as each reconnection does, while
// this one applies a reload. The verdict is the race detector, so the test runs
// under `./le test-unit bgp`. The closing assertions are the second half: a
// resolved chain still comes out of the accessor, and every name is one the two writers
// can actually produce, never a shape a torn header would leave.
//
// VALIDATES: the establishment read of ImportFilters/ExportFilters is serialized with
// the reload write of the same fields.
// PREVENTS: an unlocked read of a slice header the reload goroutine rewrites.
func TestDynamicPeerSettingsFilterReadRacesTheReload(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("185.1.69.11"), 65000, 0, 0x01020304)
	settings.IsDynamic = true
	settings.ImportFilters = []filterapi.FilterRef{{Name: "in-$remote_as"}}
	settings.ExportFilters = []filterapi.FilterRef{{Name: "out-$remote_as"}}

	peer := NewPeer(settings)
	session := NewSession(settings)
	session.mu.Lock()
	session.peerOpen = &message.Open{MyAS: 65001, BGPIdentifier: 0x0a0b0c0d}
	session.mu.Unlock()

	// Resolve once synchronously so the template is captured whatever order the two
	// goroutines below are scheduled in. Without it the reload can land first and the
	// capture would latch that chain instead, which is a different test.
	peer.resolveDynamicPeerSettings(session)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Writer: each reconnection of a dynamic peer re-resolves the filter chains.
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

	// The second writer: a config reload publishes a new filter pair onto the running
	// peer, through the same path peerSettingsSwapPlan chose for it.
	//
	// 1000 rather than the 4000 the test above uses, because each iteration here reaches
	// refreshForwardFactsIfLive and so reads the host interface table. This target runs at
	// -count=20, so a syscall per iteration is paid 20 times over.
	for i := range 1000 {
		next := NewPeerSettings(settings.Address, 65000, 0, 0x01020304)
		next.ImportFilters = []filterapi.FilterRef{{Name: "in-$remote_as"}, {Name: "reload-in"}}
		next.ExportFilters = []filterapi.FilterRef{{Name: "out-$remote_as"}}
		if i%2 == 0 {
			next.ImportFilters = next.ImportFilters[:1]
		}
		peer.applyHotSwappableSettings(next, hotSwappableSettings)
	}

	close(stop)
	wg.Wait()

	require.Equal(t, uint32(65001), peer.PeerAS(), "PeerAS resolved from the OPEN")
	for _, ref := range peer.ImportFilters() {
		require.NotEmpty(t, ref.Name)
		require.True(t,
			ref.Name == "in-65001" || ref.Name == "in-$remote_as" || ref.Name == "reload-in",
			"import filter name written by neither writer: %q", ref.Name)
	}
	for _, ref := range peer.ExportFilters() {
		require.NotEmpty(t, ref.Name)
		require.True(t,
			ref.Name == "out-65001" || strings.Contains(ref.Name, "$remote_as"),
			"export filter name written by neither writer: %q", ref.Name)
	}
}
