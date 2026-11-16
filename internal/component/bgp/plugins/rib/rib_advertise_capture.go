// Design: docs/architecture/plugin/rib-storage-design.md -- retained selected output
// Related: rib_bestchange.go -- the retained winner that owns each capture
// Related: rib_recovery.go -- owned native route extraction, without re-election
package rib

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// advertiseState keeps only current learned selections. Its zero value is
// usable; current is allocated on the first winner. It owns no goroutine or
// output authority. A revision is global to this manager, so removing an unsent
// key needs no historical counter or tombstone. Safe for concurrent use.
//
// Election locks precede mu. Nothing holding mu may acquire a selection shard,
// flowSpecMu or peerMu, or perform an engine call.
type advertiseState struct {
	mu       sync.Mutex
	current  map[advertiseKey]advertiseRoute
	revision uint64
	failure  error
	// Reporting is coalesced separately from capture. A failed engine request
	// stays owed, with at most one retry per report-bus window.
	reporting    bool
	reportNeeded atomic.Bool
	reportAt     time.Time
}

// advertiseKey spans source paths: path zero and every other ADD-PATH path
// compete for this same native route. Prefix is valid for CIDR families only.
// native is the registered receive-storage identity for every other family.
type advertiseKey struct {
	family family.Family
	prefix netip.Prefix
	native string
}

// advertiseRoute owns immutable wire bytes, independent of received pool
// handles. Snapshots share those bytes; replacing a selection replaces the
// whole value, never its slices. Zero revision is not a completed capture.
//
// route carries its original source receipt and policy epoch from retained
// storage. A snapshot alone never authorizes output and MUST NOT acquire
// provenance from current peer metadata.
type advertiseRoute struct {
	key         advertiseKey
	keyNLRI     []byte
	revision    uint64
	route       ribevents.RecoveryRoute
	staleLevel  uint8
	fingerprint uint64
	attrLen     uint32
	labels      []uint32
	wireRoute   string
}

// advertiseSnapshot returns the current immutable selections, never withdrawn
// history. Callers MUST NOT mutate any returned slice. A failed capture refuses
// the whole snapshot: returning an old table as current would permit replay of
// a route whose new revision could not be recorded. Safe for concurrent use.
func (r *RIBManager) advertiseSnapshot() ([]advertiseRoute, error) {
	r.advertise.mu.Lock()
	defer r.advertise.mu.Unlock()
	if r.advertise.failure != nil {
		return nil, r.advertise.failure
	}
	current := make([]advertiseRoute, 0, len(r.advertise.current))
	for _, route := range r.advertise.current {
		current = append(current, route)
	}
	return current, nil
}

// prepareAdvertise copies only a changed winner while its candidate remains
// retained. The caller MUST hold its selection shard (flowSpecMu for FlowSpec)
// and MUST keep the candidate until this returns. labels MUST be the winner's
// already resolved label values; only a changed capture owns a copy. Commit
// only after the same election records its winner. Nil means the current
// capture already describes this route; a new received message ID alone is not
// a selected-route change.
func (r *RIBManager) prepareAdvertise(fam family.Family, prefix netip.Prefix, native []byte, best *Candidate, labels []uint32) *advertiseRoute {
	key := advertiseKey{family: fam, prefix: prefix}
	if !prefix.IsValid() {
		key.native = string(native)
	}
	stale := storage.AdvertisedStaleLevel(best.entry.StaleLevel, best.entry.ReceivedLLGRStale)
	r.advertise.mu.Lock()
	current, found := r.advertise.current[key]
	failed := r.advertise.failure != nil
	r.advertise.mu.Unlock()
	if failed {
		return nil
	}
	if found {
		if current.matches(best, stale, labels) {
			return nil
		}
	}
	owned, err := recoveryCandidateWithLabels(fam, prefix, best, labels)
	if err != nil {
		r.failAdvertise(fmt.Errorf("bgp-rib: cannot capture selected %s route: %w", fam, err))
		return nil
	}
	keyNLRI := owned.NLRI
	if owned.AddPath {
		// Reconstruction framed the exact source identifier, including zero.
		keyNLRI = keyNLRI[4:]
	}
	return &advertiseRoute{key: key, keyNLRI: keyNLRI, route: owned,
		staleLevel: stale, fingerprint: best.entry.AttrFingerprint,
		attrLen: best.entry.AttrLen, labels: slices.Clone(labels), wireRoute: best.Route}
}

// matches uses the retained route's semantic and admission identity. Message
// IDs can advance on an identical receipt. A changed source receipt, epoch or
// stale treatment gets its own immutable snapshot even when bestPrev stays equal.
func (route *advertiseRoute) matches(best *Candidate, stale uint8, labels []uint32) bool {
	if route.route.Source != best.PeerIP {
		return false
	}
	if route.route.PathID != best.PathID {
		return false
	}
	if route.route.AddPath != best.AddPath {
		return false
	}
	if route.route.SourcePolicyEpoch != best.entry.SourcePolicyEpoch {
		return false
	}
	if route.route.SourceReceipt != best.entry.SourceReceipt {
		return false
	}
	if route.staleLevel != stale {
		return false
	}
	if !sameAttributeSet(route.fingerprint, best.entry.AttrFingerprint) {
		return false
	}
	if route.attrLen != best.entry.AttrLen {
		return false
	}
	if !slices.Equal(route.labels, labels) {
		return false
	}
	return route.wireRoute == best.Route
}

// commitAdvertise publishes a prepared winner under the same selection lock
// that prepared it. The single revision clock also covers removals, so a route
// appearing after an absence can never reuse its old revision.
func (r *RIBManager) commitAdvertise(route *advertiseRoute) {
	if route == nil {
		return
	}
	r.advertise.mu.Lock()
	defer r.advertise.mu.Unlock()
	if r.advertise.failure != nil {
		return
	}
	if r.advertise.revision == math.MaxUint64 {
		r.advertise.failure = errors.New("bgp-rib: selected revision exhausted; restart the deciding process before replaying selected routes")
		r.advertise.reportNeeded.Store(true)
		return
	}
	r.advertise.revision++
	route.revision = r.advertise.revision
	if r.advertise.current == nil {
		r.advertise.current = make(map[advertiseKey]advertiseRoute)
	}
	r.advertise.current[route.key] = *route
}

// removeAdvertise drops an absent selection under its existing selection lock.
// Until a destination admits output there is no retirement obligation, so the
// capture contains no absence records. The dissemination owner will keep its
// actual acknowledged/in-flight obligations separately when it is wired.
func (r *RIBManager) removeAdvertise(fam family.Family, prefix netip.Prefix, native []byte) {
	key := advertiseKey{family: fam, prefix: prefix}
	if !prefix.IsValid() {
		key.native = string(native)
	}
	r.advertise.mu.Lock()
	defer r.advertise.mu.Unlock()
	if _, held := r.advertise.current[key]; !held {
		return
	}
	delete(r.advertise.current, key)
	if r.advertise.failure != nil {
		return
	}
	if r.advertise.revision == math.MaxUint64 {
		r.advertise.failure = errors.New("bgp-rib: selected revision exhausted; restart the deciding process before replaying selected routes")
		r.advertise.reportNeeded.Store(true)
		return
	}
	r.advertise.revision++
}

// failAdvertise makes a missing retained value an explicit failed snapshot,
// never a successful table lacking the route. No engine call runs under mu.
func (r *RIBManager) failAdvertise(err error) {
	r.advertise.mu.Lock()
	defer r.advertise.mu.Unlock()
	if r.advertise.failure != nil {
		return
	}
	r.advertise.failure = err
	r.advertise.reportNeeded.Store(true)
}

// prepareFlowSpecAdvertise captures the winner the authorization pass already
// chose. It supplies that retained candidate's own framing; it runs no election
// and performs no later lookup. Caller MUST hold flowSpecMu and its entry ref.
func (r *RIBManager) prepareFlowSpecAdvertise(flow *flowSpecRoute) *advertiseRoute {
	best := *flow.candidate
	best.entry = flow.entry
	best.Route = flow.key.NLRI
	best.PathID = flow.key.PathID
	best.AddPath = flow.addPath
	return r.prepareAdvertise(flow.key.Family, netip.Prefix{}, []byte(flow.key.NLRI), &best, nil)
}

// reelectAdvertiseStale refreshes affected native keys after a GR/LLGR mark.
// The caller MUST release peerMu before returning through this deferred call.
// A second candidate election belongs to checkBestPathChange, never capture.
func (r *RIBManager) reelectAdvertiseStale(peer netip.Addr, selected family.Family) {
	r.peerMu.RLock()
	routes := r.bgpPeers[peer]
	if routes == nil {
		r.peerMu.RUnlock()
		return
	}
	type retainedKey struct {
		family  family.Family
		nlri    []byte
		addPath bool
	}
	var keys []retainedKey
	for _, fam := range routes.Families() {
		if selected != (family.Family{}) {
			if fam != selected {
				continue
			}
		}
		if ribevents.IsFlowSpec(fam) {
			continue
		}
		addPath := routes.IsAddPath(fam)
		routes.IterateFamily(fam, func(raw []byte, _ storage.RouteEntry) bool {
			keys = append(keys, retainedKey{family: fam, nlri: bytes.Clone(raw), addPath: addPath})
			return true
		})
	}
	r.peerMu.RUnlock()
	for _, key := range keys {
		if change, changed := r.checkBestPathChange(key.family, key.nlri, key.addPath, nil); changed {
			publishBestChanges([]bestChangeEntry{change}, key.family)
		}
	}
}
