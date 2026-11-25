// Design: docs/architecture/plugin/rib-storage-design.md -- RIB process ownership.
// Related: rib_source_lease.go -- local candidate references and engine authority.
package rib

import (
	"context"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/core/opererr"
)

var errRIBStopped = opererr.New("BGP RIB has stopped",
	"Its route, timer and reader ownership has ended, so it cannot accept another operation.",
	"ze request restart")

// startRIBWorkers starts this manager's readers once startup has completed.
// peerMu serializes worker admission with shutdown's Wait; no reader or engine
// operation runs under that lock. SDK Run MUST drain startup before shutdown.
func (r *RIBManager) startRIBWorkers(parent context.Context) error {
	r.peerMu.Lock()
	defer r.peerMu.Unlock()
	if r.stopping {
		return errRIBStopped
	}
	if r.backgroundCancel != nil {
		return opererr.New("BGP RIB readers have already started",
			"One plugin lifetime owns one set of background readers.", "ze request restart")
	}
	ctx, cancel := context.WithCancel(parent)
	r.backgroundCtx = ctx
	r.backgroundCancel = cancel
	r.backgroundWG.Go(func() { runCompaction(ctx, pool.AllPools()) })
	r.backgroundWG.Go(func() { r.runAIGPSelection(ctx) })
	if metricsPtr.Load() != nil {
		r.backgroundWG.Go(func() { r.runMetricsLoop(ctx) })
	}
	return nil
}

// cancelPeerReplayLocked aborts any in-flight peer-up replay for peerOwner.
// MUST hold peerMu.
func (r *RIBManager) cancelPeerReplayLocked(peerOwner uint64) {
	if h, ok := r.peerReplay[peerOwner]; ok {
		h.cancel()
		delete(r.peerReplay, peerOwner)
	}
}

// launchPeerUpReplay runs peer-up Adj-RIB-Out and aggregate replay.
//
// In production (a live SDK plugin without synchronous test hooks), replay MUST
// run off Process.deliveryLoop: on DirectBridge, each replayed update-route RPC
// writes to the peer and notifies onMessageSent -> Process(bgp-rib).Deliver back
// onto bgp-rib's own bounded eventChan (64 slots). Running replay inline on
// deliveryLoop self-deadlocks after 64 UPDATEs and stalls deliverPeerState on
// the peer's read loop.
func (r *RIBManager) launchPeerUpReplay(peerOwner uint64, peerAddr string, cameUp bool, replayGroups []replayGroup, initialReplay uint64, replayAggregates bool) {
	if !cameUp && !replayAggregates {
		return
	}
	runSync := func() {
		if cameUp {
			r.replayRoutesWithCursor(peerAddr, replayGroups, initialReplay)
		}
		if replayAggregates {
			r.replayAggregatesToPeer(peerAddr)
		}
	}
	if r.plugin == nil {
		runSync()
		return
	}
	if r.updateHook != nil {
		runSync()
		return
	}
	if r.dispatchHook != nil {
		runSync()
		return
	}
	r.peerMu.Lock()
	if r.stopping {
		r.peerMu.Unlock()
		return
	}
	if r.backgroundCtx == nil {
		// Unit tests that construct RIBManager without startRIBWorkers expect
		// synchronous replay and do not call shutdownReceivedSources to join
		// backgroundWG.
		r.peerMu.Unlock()
		if cameUp {
			r.replayRoutesWithCursor(peerAddr, replayGroups, initialReplay)
		}
		if replayAggregates {
			r.replayAggregatesToPeer(peerAddr)
		}
		return
	}
	r.cancelPeerReplayLocked(peerOwner)
	peerCtx, peerCancel := context.WithCancel(r.backgroundCtx)
	r.peerReplayGen++
	gen := r.peerReplayGen
	if r.peerReplay == nil {
		r.peerReplay = make(map[uint64]peerReplayHandle)
	}
	r.peerReplay[peerOwner] = peerReplayHandle{gen: gen, cancel: peerCancel}
	r.backgroundWG.Go(func() {
		defer func() {
			peerCancel()
			r.peerMu.Lock()
			if cur, ok := r.peerReplay[peerOwner]; ok && cur.gen == gen {
				delete(r.peerReplay, peerOwner)
			}
			r.peerMu.Unlock()
		}()
		if cameUp {
			r.replayRoutesWithCursorContext(peerCtx, peerAddr, replayGroups, initialReplay)
		}
		if replayAggregates && peerCtx.Err() == nil {
			r.replayAggregatesToPeer(peerAddr)
		}
	})
	r.peerMu.Unlock()
}

// startGRExpiryLocked counts scheduled timers, including a replaced callback
// that already started. MUST hold peerMu and reject new work after stopping.
// A successful Stop and the callback are mutually exclusive discharge owners.
func (r *RIBManager) startGRExpiryLocked(state *peerGRState, delay time.Duration, expire func()) {
	if r.stopping {
		panic("BUG: GR timer admitted after RIB shutdown")
	}
	r.grTimers.Add(1)
	state.expiryOwned = true
	state.expiryTimer = time.AfterFunc(delay, func() {
		defer r.grTimers.Done()
		expire()
	})
}

// stopGRExpiryLocked cancels only this state's timer ownership. A callback
// already running retains its count until its complete publication returns.
// MUST hold peerMu; joining happens after releasing that lock.
func (r *RIBManager) stopGRExpiryLocked(state *peerGRState) {
	if state == nil || state.expiryTimer == nil {
		return
	}
	if state.expiryTimer.Stop() && state.expiryOwned {
		state.expiryOwned = false
		r.grTimers.Done()
	}
}

// shutdownReceivedSources MUST follow SDK Run's delivery/callback drain. It
// ends all local readers before releasing their references. Engine generation
// cleanup owns terminal source revocation and report clearing; an ended SDK
// connection MUST NOT be used for new lease-release or warning-clear requests.
func (r *RIBManager) shutdownReceivedSources() {
	r.shutdownOnce.Do(func() {
		r.peerMu.Lock()
		r.stopping = true
		cancel := r.backgroundCancel
		for owner := range r.peerReplay {
			r.cancelPeerReplayLocked(owner)
		}
		for _, state := range r.grState {
			r.stopGRExpiryLocked(state)
		}
		r.peerMu.Unlock()
		if cancel != nil {
			cancel()
		}
		r.sourceLeases.stop()
		r.selectionDefer.stop()
		r.backgroundWG.Wait()
		r.grTimers.Wait()
		r.stopSelectedDispatch()
		r.stopRecoveryRemovals()
		r.SetLocRIB(nil)

		r.peerMu.Lock()
		for _, routes := range r.bgpPeers {
			routes.Release()
		}
		for _, peers := range r.ribInPool {
			for _, routes := range peers {
				routes.Release()
			}
			clear(peers)
		}
		for _, transaction := range r.sourcePolicy {
			transaction.releaseSent()
		}
		for _, families := range r.ribOut {
			for _, routes := range families {
				for _, entry := range routes {
					entry.release()
				}
			}
		}
		clear(r.bgpPeers)
		clear(r.ribInPool)
		clear(r.ribOut)
		clear(r.sourcePolicy)
		clear(r.peerUp)
		clear(r.peerMeta)
		clear(r.peerNames)
		clear(r.sourceApplied)
		clear(r.retainedPeers)
		clear(r.grState)
		r.peerMu.Unlock()

		r.advertise.mu.Lock()
		clear(r.advertise.current)
		r.advertise.mu.Unlock()
		r.sourceLeases.releaseLocal()
	})
}

// releaseLocal follows the SDK drain and local worker joins. Its references
// MUST already have been returned by every store and transaction owner. The
// engine, not this ended connection, owns terminal authority/report cleanup.
func (s *ribSourceLeases) releaseLocal() {
	if s == nil {
		return
	}
	s.reportMu.Lock()
	defer s.reportMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, lease := range s.leases {
		if lease.ref.References() != 0 {
			panic("BUG: RIB shutdown retained an original source reference")
		}
	}
	clear(s.leases)
	clear(s.admission)
	clear(s.reported)
	clear(s.reportAfter)
}
