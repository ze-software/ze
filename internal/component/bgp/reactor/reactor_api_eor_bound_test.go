// Design: docs/architecture/api/process-protocol.md -- original destination EOR writes.
// Related: reactor_api_forward.go -- the actual queued and immediate EOR writers.
package reactor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestBoundEndOfRIBKeepsFinalDestination checks real EOR bytes on both writer
// paths. Replacing the original at final export admission must write neither
// its still-open old socket nor the successor; a current request remains useful.
func TestBoundEndOfRIBKeepsFinalDestination(t *testing.T) {
	for _, queued := range []bool{false, true} {
		name := "immediate"
		if queued {
			name = "queued"
		}
		t.Run(name, func(t *testing.T) {
			for _, replace := range []bool{false, true} {
				name := "current"
				if replace {
					name = "replaced"
				}
				t.Run(name, func(t *testing.T) {
					adapter, peer, originalWire := newEORGuardPeer(t)
					original := peer.session
					original.initialReplay.Store(71)
					peer.establishedSession.Store(original)
					peer.SetReactor(adapter.r)
					t.Cleanup(original.timers.StopAll)
					nextPeer, nextWire := newEstablishedRecordingPeer(t, "10.0.0.2", family.IPv4Unicast)
					next := nextPeer.session
					next.initialReplay.Store(72)
					t.Cleanup(next.timers.StopAll)
					var replaced atomic.Bool
					original.egressFinalAccepts = func(filterapi.ExportKind) bool {
						if replace && replaced.CompareAndSwap(false, true) {
							peer.mu.Lock()
							peer.session = next
							peer.establishedSession.Store(next)
							peer.initialSyncEOR = map[family.Family]bool{family.IPv4Unicast: true}
							peer.mu.Unlock()
						}
						return true
					}
					if queued {
						adapter.r.fwdPool = newFwdPool(fwdBatchHandler, fwdPoolConfig{})
						t.Cleanup(adapter.r.fwdPool.Stop)
					}
					sender, err := plugin.OperatorSender().WithDestination(peer.sourceOwner, 71)
					require.NoError(t, err)
					err = adapter.AnnounceEOR(selector.All(), uint16(family.AFIIPv4), uint8(family.SAFIUnicast), sender)
					if queued {
						require.NoError(t, err)
						ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
						defer cancel()
						require.NoError(t, adapter.r.fwdPool.barrierPeer(ctx, peer.settings.PeerKey()))
					}
					if replace {
						require.True(t, replaced.Load(), "replacement must reach final admission")
						if !queued {
							require.ErrorIs(t, err, errAdjOutSession)
						}
						require.Empty(t, originalWire.written(), "a retired original cannot emit its old marker")
						require.Empty(t, nextWire.written(), "a successor cannot inherit the old marker")
						peer.mu.RLock()
						claimed := peer.initialSyncEOR[family.IPv4Unicast]
						peer.mu.RUnlock()
						require.True(t, claimed, "refusing the old marker must preserve the successor's independent claim")
						return
					}
					require.NoError(t, err)
					require.Equal(t, eorWire(family.IPv4Unicast), originalWire.written())
					require.Empty(t, nextWire.written())
				})
			}
		})
	}
}

// TestBoundEndOfRIBClaimKeepsOriginalSession checks replacement before claim
// admission cannot insert the old marker into the successor's family map.
func TestBoundEndOfRIBClaimKeepsOriginalSession(t *testing.T) {
	_, peer, _ := newEORGuardPeer(t)
	original := peer.session
	t.Cleanup(original.timers.StopAll)
	target := announceTarget{peer: peer, session: original}
	claimed, err := target.claimEndOfRIB(family.IPv4Unicast)
	require.NoError(t, err)
	require.True(t, claimed)
	target.releaseEndOfRIBClaim(family.IPv4Unicast)
	nextPeer, _ := newEstablishedRecordingPeer(t, "10.0.0.2", family.IPv4Unicast)
	next := nextPeer.session
	t.Cleanup(next.timers.StopAll)
	peer.mu.Lock()
	peer.session = next
	peer.initialSyncEOR = map[family.Family]bool{family.IPv6Unicast: true}
	peer.mu.Unlock()
	claimed, err = target.claimEndOfRIB(family.IPv4Unicast)
	require.ErrorIs(t, err, errAdjOutSession)
	require.False(t, claimed)
	peer.mu.RLock()
	originalClaim := peer.initialSyncEOR[family.IPv4Unicast]
	nextClaim := peer.initialSyncEOR[family.IPv6Unicast]
	peer.mu.RUnlock()
	require.False(t, originalClaim)
	require.True(t, nextClaim)
}
