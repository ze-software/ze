// Design: docs/architecture/plugin/rib-storage-design.md -- RIB process ownership.
package rib

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The engine closes each actual SDK connection at subtest cleanup and waits
// for runRIBPlugin. The retained manager then exposes what shutdown released.
func TestRIBShutdownReleasesOriginalSourceReferences(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		for cycle := range 2 {
			var manager *RIBManager
			var references []*storage.SourceReference
			var retained []*storage.PeerRIB
			t.Run(fmt.Sprintf("%s/%d", transport, cycle), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				engine, r := startRIBOverTransport(t, ctx, transport)
				manager = r
				_, _, err := r.markStaleCommand([]string{transportSource, "60"})
				require.NoError(t, err)
				out := transportPhase(t, ctx, engine, transportRequest(t, rpc.SourcePolicyPrepare, 0))
				require.Equal(t, rpc.StatusOK, out.Status, out.Error)
				out = transportPhase(t, ctx, engine, transportRequest(t, rpc.SourcePolicyApply, 6))
				require.Equal(t, rpc.StatusOK, out.Status, out.Error)
				r.peerMu.RLock()
				require.NotEmpty(t, r.sourcePolicy[netip.MustParseAddr(transportSource)].sent,
					"an applied transaction must still own its removed sent entry")
				for _, routes := range r.bgpPeers {
					retained = append(retained, routes)
				}
				r.peerMu.RUnlock()
				r.sourceLeases.mu.Lock()
				for _, lease := range r.sourceLeases.leases {
					references = append(references, lease.ref)
					require.Positive(t, lease.ref.References())
				}
				r.sourceLeases.mu.Unlock()
				require.NotEmpty(t, references)
			})
			require.NotNil(t, manager)
			for _, ref := range references {
				require.Zero(t, ref.References(), "an internal restart must return every original-source reference")
			}
			for _, routes := range retained {
				require.Zero(t, routes.Len(), "received storage must release its attribute owners")
			}
			require.Empty(t, manager.bgpPeers)
			require.Empty(t, manager.ribOut)
			require.Empty(t, manager.sourcePolicy)
			require.Empty(t, manager.grState)
			require.Empty(t, manager.sourceLeases.leases)
			require.Empty(t, manager.sourceLeases.reported)
		}
	}
}

func TestRIBShutdownJoinsReplacedGRCallback(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr("192.0.2.151")
	entered, release := make(chan struct{}), make(chan struct{})
	closing := make(chan struct{})
	r.backgroundCancel = func() { close(closing) }
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	old, current := &peerGRState{}, &peerGRState{}
	r.peerMu.Lock()
	r.startGRExpiryLocked(old, time.Nanosecond, func() { close(entered); <-release })
	r.grState[peer] = old
	r.peerMu.Unlock()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("the original GR callback did not start")
	}
	r.peerMu.Lock()
	r.stopGRExpiryLocked(old)
	r.startGRExpiryLocked(current, time.Hour, func() { t.Error("a canceled GR timer ran") })
	r.grState[peer] = current
	r.peerMu.Unlock()
	stopped := make(chan struct{})
	go func() { r.shutdownReceivedSources(); close(stopped) }()
	// Reader cancellation follows closure of admission and precedes the join.
	select {
	case <-closing:
	case <-ctx.Done():
		t.Fatal("shutdown did not close timer admission")
	}
	r.peerMu.RLock()
	closed := r.stopping
	r.peerMu.RUnlock()
	require.True(t, closed)
	select {
	case <-stopped:
		t.Fatal("shutdown returned before its replaced GR callback finished")
	default:
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal("shutdown did not join its GR callback")
	}
	_, _, err := r.markStaleCommand([]string{peer.String(), "60"})
	require.Error(t, err, "an ended owner cannot schedule another timer")
	require.Empty(t, r.grState)
}

func TestRIBShutdownJoinsReadersBeforeSourceRelease(t *testing.T) {
	work, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	f := newSourceFailureFixture(t)
	r, s := f.r, f.r.sourceLeases
	peer := netip.MustParseAddr("192.0.2.152")
	ref, err := s.acquire(t.Context(), peer, 701, 81)
	require.NoError(t, err)
	entry := storage.NewRouteEntry()
	entry.SetSourceReference(ref)
	routes := storage.NewPeerRIB(peer.String())
	routes.InsertEntry(family.IPv4Unicast, entry, 0, 0, []byte{24, 198, 18, 152})
	entry.Release()
	ref.Release()
	r.bgpPeers[peer] = routes
	ctx, cancel := context.WithCancel(work)
	r.backgroundCancel = cancel
	r.backgroundWG.Add(1)
	readerCanceled, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	go func() {
		defer r.backgroundWG.Done()
		<-ctx.Done()
		close(readerCanceled)
		<-release
	}()
	stopped := make(chan struct{})
	go func() { r.shutdownReceivedSources(); close(stopped) }()
	select {
	case <-readerCanceled:
	case <-work.Done():
		t.Fatal("shutdown did not cancel the owned reader")
	}
	require.Positive(t, ref.References(), "a reader still owns access to received storage")
	select {
	case <-stopped:
		t.Fatal("shutdown released payloads while its reader was active")
	default:
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case <-stopped:
	case <-work.Done():
		t.Fatal("shutdown did not finish after the reader returned")
	}
	require.Zero(t, ref.References())
	_, err = s.acquire(t.Context(), peer, 701, 82)
	require.Error(t, err, "late intake must be refused before any new source authority is requested")
	require.Empty(t, s.leases)
	r.shutdownReceivedSources() // Repeated lifecycle cleanup owns no second release.
}

func TestRIBSourceLeaseCachedIntakeWakesOnlyOnChange(t *testing.T) {
	f := newSourceFailureFixture(t)
	s := f.r.sourceLeases
	peer := netip.MustParseAddr("192.0.2.153")
	first, err := s.acquire(t.Context(), peer, 701, 83)
	require.NoError(t, err)
	require.Empty(t, s.wake, "a live new lease owes no settlement")
	for range 128 {
		ref, acquireErr := s.acquire(t.Context(), peer, 701, 83)
		require.NoError(t, acquireErr)
		ref.Release()
	}
	require.Empty(t, s.wake, "cached UPDATE intake must not scan all source sessions")
	s.forgetAdmission(peer)
	require.Empty(t, s.wake, "clearing no fault changes nothing")
	s.setAdmission(peer, errors.New("controlled admission failure"))
	require.Len(t, s.wake, 1)
	<-s.wake
	second, err := s.acquire(t.Context(), peer, 701, 83)
	require.NoError(t, err)
	require.Len(t, s.wake, 1, "clearing a real warning must wake its report owner")
	<-s.wake
	second.Release()
	require.Empty(t, s.wake)
	first.Release()
	require.Len(t, s.wake, 1, "the last reference still owes an engine release")
	require.NoError(t, s.settle(t.Context()))
}

func TestRIBSourceLeaseFailureProjectionIsStable(t *testing.T) {
	f := newSourceFailureFixture(t)
	s := f.r.sourceLeases
	peer := netip.MustParseAddr("192.0.2.154")
	s.admission[peer] = errors.New("admission")
	s.leases[11] = &ribSourceLease{peer: peer, failure: errors.New("receipt 11")}
	s.leases[2] = &ribSourceLease{peer: peer, failure: errors.New("receipt 2")}
	for range 128 {
		require.EqualError(t, s.failures()[peer], "admission\nreceipt 2\nreceipt 11",
			"unchanged failures must not generate a new report because map iteration changed")
	}
}

// TestRIBShutdownPreservesIndependentSharedPools reconstructs B's real retained
// route after A releases its equal pooled attributes and original-source refs.
func TestRIBShutdownPreservesIndependentSharedPools(t *testing.T) {
	first := newSourceFailureFixture(t).r
	other := newSourceFailureFixture(t).r
	electionFixtureLocRIB(t, first, netip.MustParseAddr("10.0.0.1"))
	electionFixtureLocRIB(t, other, netip.MustParseAddr("10.0.0.1"))
	t.Cleanup(first.shutdownReceivedSources)
	t.Cleanup(other.shutdownReceivedSources)
	peer := netip.MustParseAddr("192.0.2.155")
	sourceLeaseReceive(t, first, peer, 701, 91, false, false)
	sourceLeaseReceive(t, other, peer, 701, 92, false, false)
	before, err := other.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, before, 1, "the independent owner has a useful elected route")
	other.sourceLeases.mu.Lock()
	ref := other.sourceLeases.leases[92].ref
	other.sourceLeases.mu.Unlock()
	first.shutdownReceivedSources()
	require.Empty(t, first.bgpPeers)
	require.Empty(t, first.sourceLeases.leases)
	require.Positive(t, ref.References(), "B still owns its separate original-source reference")
	// A changed stale projection forces a new extraction from B's pooled
	// storage; reading only the earlier owned snapshot would miss pool damage.
	_, _, err = other.markStaleCommand([]string{peer.String(), "0"})
	require.NoError(t, err)
	after, err := other.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, after, 1)
	require.Greater(t, after[0].revision, before[0].revision)
	require.Equal(t, before[0].route.Attributes, after[0].route.Attributes)
	require.Equal(t, uint64(92), after[0].route.SourceReceipt)
	require.Positive(t, other.bgpPeers[peer].Len())
	other.shutdownReceivedSources()
	require.Zero(t, ref.References(), "B returns its reference at its own shutdown only")
}
