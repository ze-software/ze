// Design: docs/architecture/rib/forward-handle.md -- mirror and delivery lifetime.
package rib

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRIBShutdownDrainsStructuredPeerDown holds a real plugin's admitted DOWN
// at Loc-RIB removal, then closes its bridge. The plugin cannot detach its mirror
// before the withdrawal completes, and no later delivery can enter the handler.
func TestRIBShutdownDrainsStructuredPeerDown(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	done := make(chan int, 1)
	exited := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	var loc *locrib.RIB
	prefix := netip.MustParsePrefix("198.18.255.0/24")
	go func() {
		defer close(exited)
		done <- runRIBPlugin(rpc.NewBridgedConn(pluginEnd, bridge))
	}()
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		bridge.CloseCallbacks()
		if err := mux.Close(); err != nil {
			t.Logf("close startup transport: %v", err)
		}
		if err := engineEnd.Close(); err != nil {
			t.Logf("close engine pipe: %v", err)
		}
		select {
		case <-exited:
			if loc != nil {
				loc.Remove(family.IPv4Unicast, prefix, bgpProtocolID, 0)
			}
		case <-time.After(5 * time.Second):
			t.Error("RIB plugin did not join during cleanup")
		}
	})
	for stage := range 3 {
		req := readMuxRequestTimeout(t, ctx, mux)
		if err := mux.SendOK(ctx, req.ID); err != nil {
			t.Fatal(err)
		}
		switch stage {
		case 0:
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", &rpc.ConfigureInput{}); err != nil {
				t.Fatal(err)
			}
		case 1:
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:share-registry", &rpc.ShareRegistryInput{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	// This callback is answered only after the SDK installs the direct handler.
	if _, err := bridge.SendCallback(ctx, "ze-plugin-callback:post-startup", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	r := activeManager.Load()
	if r == nil {
		t.Fatal("the real RIB plugin did not publish its manager")
	}
	loc = r.locRIB.Load()
	if loc == nil {
		t.Fatal("the in-process RIB plugin has no Loc-RIB")
	}
	peer := netip.MustParseAddr("192.0.2.254")
	r.peerMu.Lock()
	purgeTestPeer(r, peer, 65001)
	r.bgpPeers[peer].Insert(family.IPv4Unicast, makeAttrBytes([4]byte{192, 0, 2, 254}), []byte{24, 198, 18, 255})
	r.peerMu.Unlock()
	if _, changed := r.checkBestPathChange(family.IPv4Unicast, []byte{24, 198, 18, 255}, false, nil); !changed {
		t.Fatal("seeded route did not become best")
	}
	var path locrib.Path
	present := loc.Inspect(family.IPv4Unicast, prefix, func(group locrib.PathGroup) {
		path = group.Paths[0]
	})
	if !present {
		t.Fatal("seeded route is absent from Loc-RIB")
	}
	// MUST release candidate extraction before stopping the plugin in cleanup.
	electionEntered := make(chan struct{})
	electionRelease := make(chan struct{})
	var electionEnteredOnce, electionReleaseOnce sync.Once
	igpcost.Set(func(netip.Addr) igpcost.Distance {
		electionEnteredOnce.Do(func() { close(electionEntered) })
		<-electionRelease
		return igpcost.Distance{Resolved: true}
	})
	t.Cleanup(func() {
		// MUST unblock the worker before plugin cleanup can drain DOWN.
		electionReleaseOnce.Do(func() { close(electionRelease) })
		igpcost.Set(nil)
	})
	// Wake the real reselection worker even if its initial scan already ran.
	// Its single-prefix scan must finish before DOWN can delete that peer.
	path.Metric ^= 1
	loc.Insert(family.IPv4Unicast, prefix, path)
	select {
	case <-electionEntered:
	case <-ctx.Done():
		t.Fatal("background election did not reach candidate extraction")
	}
	entered := make(chan struct{})
	r.purgeRemoveHook = func(family.Family, netip.Prefix) {
		// Re-enter a manager command after peerMu was released by the DOWN path.
		if _, _, err := r.fastpathCommand([]string{"status"}); err != nil {
			t.Errorf("reentrant tracker status: %v", err)
		}
		if r.locRIB.Load() != loc {
			t.Error("mirror detached before admitted withdrawal reached removal")
		}
		close(entered)
		<-release
		if r.locRIB.Load() != loc {
			t.Error("mirror detached while admitted withdrawal waited to finish")
		}
	}
	delivered := make(chan error, 1)
	deliveryDone := make(chan struct{})
	t.Cleanup(func() {
		// MUST release and join our delivery even when the SDK drain is broken.
		electionReleaseOnce.Do(func() { close(electionRelease) })
		releaseOnce.Do(func() { close(release) })
		bridge.CloseCallbacks()
		select {
		case <-deliveryDone:
		case <-time.After(5 * time.Second):
			t.Error("structured DOWN delivery did not join during cleanup")
		}
	})
	go func() {
		// MUST signal completion for test-owned cleanup independently of Run.
		defer close(deliveryDone)
		delivered <- bridge.DeliverStructured([]any{&rpc.StructuredEvent{
			EventType: rpc.EventKindState, PeerAddress: peer.String(), State: rpc.SessionStateDown,
		}})
	}()
	// The election holds a read admission. A refused read proves the real
	// structured DOWN handler has queued its writer, not merely been scheduled.
	for r.peerMu.TryRLock() {
		r.peerMu.RUnlock()
		if ctx.Err() != nil {
			t.Fatal("structured DOWN did not request peer write admission")
		}
		runtime.Gosched()
	}
	electionReleaseOnce.Do(func() { close(electionRelease) })
	select {
	case <-entered:
	case err := <-delivered:
		t.Fatalf("structured DOWN returned before its removal hook: %v", err)
	case code := <-done:
		t.Fatalf("plugin exited before the admitted removal: %d", code)
	case <-ctx.Done():
		t.Fatal("structured DOWN did not reach Loc-RIB removal")
	}
	bridge.CloseCallbacks()
	if err := bridge.DeliverStructured(nil); !errors.Is(err, rpc.ErrBridgeClosed) {
		t.Errorf("delivery after fence = %v, want ErrBridgeClosed", err)
	}
	// Negative wait observes whether the plugin exits while the admitted handler
	// is held. The release channel, rather than elapsed time, permits completion.
	select {
	case code := <-done:
		t.Fatalf("plugin exited with %d before the admitted withdrawal finished", code)
	case <-time.After(25 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-delivered:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("admitted structured DOWN did not finish")
	}
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("plugin exit = %d, want 0", code)
		}
	case <-ctx.Done():
		t.Fatal("plugin did not exit after delivery drained")
	}
	if _, present := loc.Lookup(family.IPv4Unicast, prefix); present {
		t.Fatal("shutdown detached the mirror before the withdrawal removed its route")
	}
	r.peerMu.RLock()
	_, retained := r.bgpPeers[peer]
	r.peerMu.RUnlock()
	if retained {
		t.Fatal("shutdown retained the departed peer's Adj-RIB-In")
	}
	if r.locRIB.Load() != nil {
		t.Fatal("plugin exit did not detach Loc-RIB")
	}
}

// TestRIBPeerDownFencesPendingElection queues DOWN while an election reads its
// candidate's IGP cost. A pending peer writer must not purge that candidate and
// then let the older election publish it again.
func TestRIBPeerDownFencesPendingElection(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	r := newTestRIBManager(t)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	peer := netip.MustParseAddr("192.0.2.253")
	prefix := netip.MustParsePrefix("198.18.252.0/24")
	wirePrefix := []byte{24, 198, 18, 252}
	purgeTestPeer(r, peer, 65001)
	r.bgpPeers[peer].Insert(family.IPv4Unicast, makeAttrBytes([4]byte{192, 0, 2, 253}), wirePrefix)
	if _, changed := r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil); !changed {
		t.Fatal("seeded route did not become best")
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	igpcost.Set(func(netip.Addr) igpcost.Distance {
		close(entered)
		<-release
		return igpcost.Distance{Resolved: true}
	})
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		igpcost.Set(nil)
	})
	elected := make(chan struct{})
	go func() {
		defer close(elected)
		r.reselectAIGPRoutes()
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("election did not reach candidate extraction")
	}
	withdrawn := make(chan struct{})
	go func() {
		defer close(withdrawn)
		r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peer.String(), State: rpc.SessionStateDown})
	}()
	// The election holds a read admission at the cost lookup. Failed TryRLock
	// therefore proves DOWN has queued its writer, not merely been scheduled.
	for r.peerMu.TryRLock() {
		r.peerMu.RUnlock()
		if ctx.Err() != nil {
			t.Fatal("peer DOWN did not request its write admission")
		}
		runtime.Gosched()
	}
	releaseOnce.Do(func() { close(release) })
	for _, done := range []<-chan struct{}{elected, withdrawn} {
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("election and peer DOWN did not finish")
		}
	}
	if _, present := loc.Lookup(family.IPv4Unicast, prefix); present {
		t.Fatal("pending election republished the departed peer's route")
	}
	if len(r.collectBestPaths()[family.IPv4Unicast]) != 0 {
		t.Fatal("pending election retained the departed peer as best")
	}
}

// TestLocRIBDetachRejectsRetainedTrackerCallback detaches from an earlier Loc-RIB
// callback; the tracker callback still in that dispatch snapshot must not retain
// a handle after its worker has exited. No manager lock may block that callback.
func TestLocRIBDetachRejectsRetainedTrackerCallback(t *testing.T) {
	r := newTestRIBManager(t)
	loc := locrib.NewRIB()
	unsubscribe := loc.OnChange(func(locrib.Change) { r.SetLocRIB(nil) })
	defer unsubscribe()
	r.SetLocRIB(loc)
	tracker := r.forwardTracker
	tracker.Enable()
	var balance atomic.Int64
	handle := &balanceHandle{balance: &balance, data: []byte("retained snapshot")}
	done := make(chan struct{})
	go func() {
		r.insertLocRIB(family.IPv4Unicast, netip.MustParsePrefix("198.18.254.0/24"), locrib.Path{
			Source: bgpProtocolID, NextHop: netip.MustParseAddr("192.0.2.1"), AdminDistance: 20,
		}, handle)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reentrant Loc-RIB detach deadlocked")
	}
	if r.locRIB.Load() != nil {
		t.Fatal("reentrant detach did not clear the mirror")
	}
	if handle.adds.Load() != 0 || balance.Load() != 0 || len(tracker.ch) != 0 {
		t.Fatalf("stopped tracker retained a handle: adds=%d balance=%d queued=%d",
			handle.adds.Load(), balance.Load(), len(tracker.ch))
	}
}

// TestLocRIBPublicationConcurrentMirrors exercises every mirror mutation while
// a lifecycle owner repeatedly publishes and detaches the same shared RIB, then
// proves attachment, withdrawal and detached-mirror isolation still work.
func TestLocRIBPublicationConcurrentMirrors(t *testing.T) {
	r := newTestRIBManager(t)
	loc := locrib.NewRIB()
	t.Cleanup(func() { r.SetLocRIB(nil) })
	prefix := netip.MustParsePrefix("198.18.253.0/24")
	var workers sync.WaitGroup
	workers.Go(func() {
		for range 128 {
			r.SetLocRIB(loc)
			r.SetLocRIB(nil)
		}
	})
	workers.Go(func() {
		for range 128 {
			r.insertLocRIB(family.IPv4Unicast, prefix, locrib.Path{
				Source: bgpProtocolID, NextHop: netip.MustParseAddr("192.0.2.1"), IsBGP: true, IsEBGP: true,
			}, nil)
			r.removeLocRIB(family.IPv4Unicast, prefix)
		}
	})
	workers.Wait()
	path := locrib.Path{
		Source: bgpProtocolID, NextHop: netip.MustParseAddr("192.0.2.1"), IsBGP: true, IsEBGP: true,
	}
	r.SetLocRIB(loc)
	r.insertLocRIB(family.IPv4Unicast, prefix, path, nil)
	if !loc.Inspect(family.IPv4Unicast, prefix, func(group locrib.PathGroup) {
		if len(group.Paths) != 1 || group.Paths[0].NextHop != path.NextHop || group.Paths[0].AdminDistance != 20 {
			t.Errorf("published route differs after concurrent mirror changes: %+v", group.Paths)
		}
	}) {
		t.Fatal("reattached mirror did not publish the route")
	}
	r.SetLocRIB(nil)
	r.removeLocRIB(family.IPv4Unicast, prefix)
	if _, present := loc.Lookup(family.IPv4Unicast, prefix); !present {
		t.Fatal("withdrawal changed a detached mirror")
	}
	r.SetLocRIB(loc)
	r.removeLocRIB(family.IPv4Unicast, prefix)
	if _, present := loc.Lookup(family.IPv4Unicast, prefix); present {
		t.Fatal("reattached mirror did not withdraw the route")
	}
}

// TestRIBPublicationAllowsPeerReaderDuringDown holds a synchronous Loc-RIB
// callback while DOWN requests peer state. The callback must still acquire a
// peer read admission before releasing the shard needed by DOWN's purge.
func TestRIBPublicationAllowsPeerReaderDuringDown(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	r := newTestRIBManager(t)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	peer := netip.MustParseAddr("192.0.2.252")
	purgeTestPeer(r, peer, 65001)
	wirePrefix := []byte{24, 198, 18, 251}
	r.bgpPeers[peer].Insert(family.IPv4Unicast, makeAttrBytes([4]byte{192, 0, 2, 252}), wirePrefix)
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	unsubscribe := loc.OnChange(func(change locrib.Change) {
		if change.Kind != locrib.ChangeAdd {
			return
		}
		close(entered)
		<-release
		// Bounded acquisition leaves the shard releasable if the old inverse
		// lock order regresses, rather than leaking permanently stuck workers.
		for !r.peerMu.TryRLock() {
			if ctx.Err() != nil {
				t.Error("DOWN holds peerMu while waiting for the publication shard")
				return
			}
			runtime.Gosched()
		}
		r.peerMu.RUnlock()
	})
	defer unsubscribe()
	elected := make(chan struct{})
	go func() {
		defer close(elected)
		r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil)
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("election did not publish into Loc-RIB")
	}
	withdrawn := make(chan struct{})
	go func() {
		defer close(withdrawn)
		r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peer.String(), State: rpc.SessionStateDown})
	}()
	for r.peerMu.TryRLock() {
		_, present := r.bgpPeers[peer]
		r.peerMu.RUnlock()
		if !present {
			break
		}
		if ctx.Err() != nil {
			t.Fatal("DOWN did not reach its peer-state mutation")
		}
		runtime.Gosched()
	}
	releaseOnce.Do(func() { close(release) })
	for _, done := range []<-chan struct{}{elected, withdrawn} {
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("publication and DOWN did not finish")
		}
	}
}

// TestRIBReplacementDuringPurgeKeepsPeerSlot blocks the second shard of a real
// DOWN purge, then publishes the same address's replacement into the first.
// Releasing the purge must neither withdraw that replacement nor free its slot.
func TestRIBReplacementDuringPurgeKeepsPeerSlot(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	r := newTestRIBManager(t)
	r.bestPrev.nShards = 2
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	var prefixes [2]netip.Prefix
	var wirePrefixes [2][]byte
	for i := range 256 {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{198, 19, byte(i), 0}), 24)
		shard := bestPrevShardIndex(prefix, 2)
		if !prefixes[shard].IsValid() {
			prefixes[shard] = prefix
			wirePrefixes[shard] = []byte{24, 198, 19, byte(i)}
		}
	}
	if !prefixes[0].IsValid() || !prefixes[1].IsValid() {
		t.Fatal("fixture did not find a prefix for both shards")
	}
	peer := netip.MustParseAddr("192.0.2.251")
	purgeTestPeer(r, peer, 65001)
	for _, wirePrefix := range wirePrefixes {
		r.bgpPeers[peer].Insert(family.IPv4Unicast, makeAttrBytes([4]byte{192, 0, 2, 251}), wirePrefix)
		if _, changed := r.checkBestPathChange(family.IPv4Unicast, wirePrefix, false, nil); !changed {
			t.Fatal("seeded route did not become best")
		}
	}
	idx, present := r.bestPathInterner.peerIdxOf(peer.String())
	if !present {
		t.Fatal("seeded peer has no interner slot")
	}
	var replacementWithdrawn atomic.Bool
	unsubscribe := loc.OnChange(func(change locrib.Change) {
		if change.Prefix == prefixes[0] && change.Kind == locrib.ChangeRemove {
			replacementWithdrawn.Store(true)
		}
	})
	defer unsubscribe()
	shards := r.bestPrev.familyShards(family.IPv4Unicast, false)
	shards.shards[1].mu.Lock()
	var unlockOnce sync.Once
	t.Cleanup(func() { unlockOnce.Do(shards.shards[1].mu.Unlock) })
	withdrawn := make(chan struct{})
	go func() {
		defer close(withdrawn)
		r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peer.String(), State: rpc.SessionStateDown})
	}()
	for {
		shards.shards[0].mu.RLock()
		_, _, held := shards.shards[0].store.lookup(prefixes[0], nil)
		shards.shards[0].mu.RUnlock()
		if !held {
			break
		}
		if ctx.Err() != nil {
			t.Fatal("DOWN did not purge the first shard")
		}
		runtime.Gosched()
	}
	r.peerMu.Lock()
	purgeTestPeer(r, peer, 65002)
	r.bgpPeers[peer].Insert(family.IPv4Unicast, makeAttrBytes([4]byte{192, 0, 2, 250}), wirePrefixes[0])
	r.peerMu.Unlock()
	if _, changed := r.checkBestPathChange(family.IPv4Unicast, wirePrefixes[0], false, nil); !changed {
		t.Fatal("replacement session did not publish its route")
	}
	unlockOnce.Do(shards.shards[1].mu.Unlock)
	select {
	case <-withdrawn:
	case <-ctx.Done():
		t.Fatal("old session's DOWN did not finish")
	}
	if replacementWithdrawn.Load() {
		t.Fatal("old DOWN transiently withdrew the replacement session's route")
	}
	if r.bestPathInterner.peerAt(idx) != peer.String() {
		t.Fatal("old DOWN reclaimed the replacement record's peer slot")
	}
	path, _, found := loc.LPM(family.IPv4Unicast, prefixes[0].Addr())
	if !found || path.NextHop != netip.MustParseAddr("192.0.2.250") {
		t.Fatal("old DOWN replaced or removed the replacement session's next hop")
	}
	other, ok := r.bestPathInterner.internPeer("192.0.2.249")
	if !ok {
		t.Fatal("interner rejected an unrelated peer")
	}
	defer r.bestPathInterner.releasePeer(other)
	if other == idx {
		t.Fatal("unrelated peer reused a slot still held by the replacement")
	}
	r.bestPathInterner.peersMu.RLock()
	refs := r.bestPathInterner.peerRefs[idx]
	r.bestPathInterner.peersMu.RUnlock()
	if refs != 1 {
		t.Fatalf("replacement peer has %d references, want its one best record", refs)
	}
	if _, changed := r.checkBestPathChange(family.IPv4Unicast, wirePrefixes[0], false, nil); changed {
		t.Fatal("unchanged replacement generated another best change")
	}
	r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peer.String(), State: rpc.SessionStateDown})
	if _, present := r.bestPathInterner.peerIdxOf(peer.String()); present {
		t.Fatal("unchanged election leaked a reference after the final record left")
	}
}

// TestRIBPeerSlotReferencesFollowBestRecords changes one of two winners, then
// withdraws routes through ordinary elections. Slots must survive their other
// record, and be reclaimed when their final best record is replaced or removed.
func TestRIBPeerSlotReferencesFollowBestRecords(t *testing.T) {
	r := newTestRIBManager(t)
	peerA := netip.MustParseAddr("192.0.2.247")
	peerB := netip.MustParseAddr("192.0.2.248")
	purgeTestPeer(r, peerA, 65001)
	purgeTestPeer(r, peerB, 65002)
	prefixes := [][]byte{{24, 198, 18, 247}, {24, 198, 18, 248}}
	for _, prefix := range prefixes {
		r.bgpPeers[peerA].Insert(family.IPv4Unicast, makeAttrBytes([4]byte{192, 0, 2, 247}), prefix)
		r.checkBestPathChange(family.IPv4Unicast, prefix, false, nil)
	}
	r.bgpPeers[peerB].Insert(family.IPv4Unicast,
		aigpSelectionAttrs([4]byte{192, 0, 2, 248}, nil, 200, 1), prefixes[0])
	r.checkBestPathChange(family.IPv4Unicast, prefixes[0], false, nil)
	if _, present := r.bestPathInterner.peerIdxOf(peerA.String()); !present {
		t.Fatal("replacing one best reclaimed the peer's other record")
	}
	r.bgpPeers[peerA].Remove(family.IPv4Unicast, prefixes[1])
	r.checkBestPathChange(family.IPv4Unicast, prefixes[1], false, nil)
	if _, present := r.bestPathInterner.peerIdxOf(peerA.String()); present {
		t.Fatal("ordinary withdrawal did not release the final best record")
	}
	r.bgpPeers[peerB].Remove(family.IPv4Unicast, prefixes[0])
	r.checkBestPathChange(family.IPv4Unicast, prefixes[0], false, nil)
	if _, present := r.bestPathInterner.peerIdxOf(peerB.String()); present {
		t.Fatal("replacement winner retained the old winner's slot")
	}
	r.bgpPeers[peerA].Remove(family.IPv4Unicast, prefixes[0])
	r.checkBestPathChange(family.IPv4Unicast, prefixes[0], false, nil)
	r.checkBestPathChange(family.IPv4Unicast, prefixes[0], false, nil)
	if len(r.bestPathInterner.peerIdx) != 0 {
		t.Fatal("best-record removal left an owned peer slot")
	}
}

// TestRIBCandidateSnapshotSurvivesRouteMutation pauses an election inside its
// candidate's metric lookup, then replaces or deletes that stored route without
// taking peerMu, as structured UPDATE Phase 2 does. The elected attributes must
// all come from the retained candidate, never a released handle or a new lookup.
func TestRIBCandidateSnapshotSurvivesRouteMutation(t *testing.T) {
	for _, replace := range []bool{false, true} {
		name := "delete"
		if replace {
			name = "replace"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			r := newTestRIBManager(t)
			peer := netip.MustParseAddr("192.0.2.246")
			purgeTestPeer(r, peer, 65001)
			routes := r.bgpPeers[peer]
			t.Cleanup(routes.Release)
			prefix := []byte{24, 198, 18, 246}
			routes.Insert(family.IPv4Unicast,
				aigpSelectionAttrs([4]byte{192, 0, 2, 246}, nil, 100, 1), prefix)
			entered := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			igpcost.Set(func(netip.Addr) igpcost.Distance {
				close(entered)
				<-release
				return igpcost.Distance{Resolved: true}
			})
			t.Cleanup(func() {
				releaseOnce.Do(func() { close(release) })
				igpcost.Set(nil)
			})
			elected := make(chan bestChangeEntry, 1)
			go func() {
				change, changed := r.checkBestPathChange(family.IPv4Unicast, prefix, false, nil)
				if !changed {
					t.Error("retained candidate did not produce a best change")
				}
				elected <- change
			}()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("election did not reach its candidate snapshot")
			}
			routes.Remove(family.IPv4Unicast, prefix)
			if replace {
				routes.Insert(family.IPv4Unicast,
					aigpSelectionAttrs([4]byte{192, 0, 2, 245}, nil, 200, 2), prefix)
			}
			releaseOnce.Do(func() { close(release) })
			select {
			case change := <-elected:
				if change.NextHop != netip.MustParseAddr("192.0.2.246") {
					t.Fatalf("election lost its candidate next hop: got %v", change.NextHop)
				}
				if len(change.ASPath) != 1 || change.ASPath[0] != 65001 {
					t.Fatalf("election read a released or replacement AS_PATH: %v", change.ASPath)
				}
			case <-ctx.Done():
				t.Fatal("election did not finish after route mutation")
			}
			// UPDATE Phase 3 runs an election after its storage mutation.
			// That election must reconcile the old snapshot to the new state.
			igpcost.Set(nil)
			change, changed := r.checkBestPathChange(family.IPv4Unicast, prefix, false, nil)
			if !changed {
				t.Fatal("post-mutation election did not reconcile the retained snapshot")
			}
			if replace {
				if change.NextHop != netip.MustParseAddr("192.0.2.245") || len(change.ASPath) != 2 {
					t.Fatalf("post-replacement election kept the old snapshot: %+v", change)
				}
			} else if change.Action != ribevents.BestChangeWithdraw {
				t.Fatalf("post-deletion election did not withdraw: %+v", change)
			}
		})
	}
}

// TestRIBPeerDownFencesUnrecordedElection removes the peer's last winning
// record, then blocks its next election at the existing next-hop interner lock.
// DOWN must still discover that admitted publication and fence its shard.
func TestRIBPeerDownFencesUnrecordedElection(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	r := newTestRIBManager(t)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	peer := netip.MustParseAddr("192.0.2.244")
	peerName := peer.String()
	prefix := netip.MustParsePrefix("198.18.244.0/24")
	wire := []byte{24, 198, 18, 244}
	attrs := makeAttrBytes([4]byte{192, 0, 2, 244})
	purgeTestPeer(r, peer, 65001)
	routes := r.bgpPeers[peer]
	routes.Insert(family.IPv4Unicast, attrs, wire)
	if _, changed := r.checkBestPathChange(family.IPv4Unicast, wire, false, nil); !changed {
		t.Fatal("seeded route did not become best")
	}
	routes.Remove(family.IPv4Unicast, wire)
	if _, changed := r.checkBestPathChange(family.IPv4Unicast, wire, false, nil); !changed {
		t.Fatal("last winning record was not withdrawn")
	}
	if _, present := r.bestPathInterner.peerIdxOf(peerName); present {
		t.Fatal("last withdrawal retained the peer's slot")
	}
	routes.Insert(family.IPv4Unicast, attrs, wire)

	// There is no prior record, so this blocks publication's internNextHop,
	// not the prior-record lookup before candidate extraction.
	r.bestPathInterner.nextHopsMu.Lock()
	var unlockOnce sync.Once
	unlockNextHop := func() { unlockOnce.Do(r.bestPathInterner.nextHopsMu.Unlock) }
	t.Cleanup(unlockNextHop)
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	igpcost.Set(func(netip.Addr) igpcost.Distance {
		close(entered)
		<-release
		return igpcost.Distance{Resolved: true}
	})
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		igpcost.Set(nil)
	})
	elected := make(chan struct{})
	go func() {
		defer close(elected)
		r.checkBestPathChange(family.IPv4Unicast, wire, false, nil)
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("election did not reach candidate extraction")
	}
	withdrawn := make(chan struct{})
	go func() {
		defer close(withdrawn)
		r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peerName, State: rpc.SessionStateDown})
	}()
	for r.peerMu.TryRLock() {
		r.peerMu.RUnlock()
		if ctx.Err() != nil {
			t.Fatal("DOWN did not request its write admission")
		}
		runtime.Gosched()
	}
	releaseOnce.Do(func() { close(release) })

	// With admission ownership, the scan pins the election's peer slot before
	// waiting on its shard. Without it, DOWN finishes early on a missing slot.
	// Either event proves the purge has observed the publication window.
purgeObserved:
	for {
		select {
		case <-withdrawn:
			break purgeObserved
		case <-ctx.Done():
			t.Fatal("DOWN neither pinned the pending election nor completed")
		default:
		}
		r.bestPathInterner.peersMu.RLock()
		idx, present := r.bestPathInterner.peerIdx[peerName]
		pinned := present && r.bestPathInterner.peerRefs[idx] >= 2
		r.bestPathInterner.peersMu.RUnlock()
		if pinned {
			break
		}
		runtime.Gosched()
	}
	unlockNextHop()
	for _, done := range []<-chan struct{}{elected, withdrawn} {
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("election and DOWN did not finish")
		}
	}
	if _, present := loc.Lookup(family.IPv4Unicast, prefix); present {
		t.Fatal("unrecorded election published the departed peer after DOWN")
	}
	if len(r.collectBestPaths()[family.IPv4Unicast]) != 0 {
		t.Fatal("unrecorded election retained the departed peer as best")
	}
	if _, present := r.bestPathInterner.peerIdxOf(peerName); present {
		t.Fatal("completed DOWN retained a publication peer-slot reference")
	}
}

// TestRIBFailedPublicationReleasesPeerAdmission exercises both failures after
// peer-slot reservation, so an unpublished record cannot keep the slot alive.
func TestRIBFailedPublicationReleasesPeerAdmission(t *testing.T) {
	for _, table := range []string{"next-hop", "metric"} {
		t.Run(table, func(t *testing.T) {
			r := newTestRIBManager(t)
			loc := locrib.NewRIB()
			r.SetLocRIB(loc)
			t.Cleanup(func() { r.SetLocRIB(nil) })
			for i := range internerCap {
				var ok bool
				if table == "next-hop" {
					_, ok = r.bestPathInterner.internNextHop(netip.AddrFrom4([4]byte{198, 19, byte(i >> 8), byte(i)}))
				} else {
					_, ok = r.bestPathInterner.internMetric(bestPathMetrics{MED: uint32(i + 1)})
				}
				if !ok {
					t.Fatal("interner rejected a value before its capacity")
				}
			}
			peer := netip.MustParseAddr("192.0.2.243")
			prefix := netip.MustParsePrefix("198.18.243.0/24")
			wire := []byte{24, 198, 18, 243}
			purgeTestPeer(r, peer, 65001)
			routes := r.bgpPeers[peer]
			t.Cleanup(routes.Release)
			routes.Insert(family.IPv4Unicast, makeAttrBytes([4]byte{192, 0, 2, 243}), wire)
			for range 2 {
				if _, changed := r.checkBestPathChange(family.IPv4Unicast, wire, false, nil); changed {
					t.Fatal("saturated interner allowed a publication")
				}
				if _, present := r.bestPathInterner.peerIdxOf(peer.String()); present {
					t.Fatal("failed publication retained its peer-slot admission")
				}
			}
			if _, present := loc.Lookup(family.IPv4Unicast, prefix); present {
				t.Fatal("failed publication installed a Loc-RIB route")
			}
			if len(r.collectBestPaths()[family.IPv4Unicast]) != 0 {
				t.Fatal("failed publication stored a best-path record")
			}
		})
	}
}
