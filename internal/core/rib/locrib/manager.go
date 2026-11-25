// Design: docs/architecture/rib/unified-locrib.md -- the sharded Loc-RIB manager
// Design: docs/architecture/rib/forward-handle.md -- InsertForward threads a ForwardHandle to Change subscribers
// Related: candidate.go -- Path value type
// Related: entry.go -- PathGroup, selectBest
// Related: forward_handle.go -- ForwardHandle interface
// Related: shard.go -- familyShards owns the per-prefix shards under RIB
// Related: change.go -- subscriberList replicated per shard

package locrib

import (
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// RIB is the unified Loc-RIB. It holds one familyShards per address family;
// each familyShards splits the prefix space across N shards (default
// GOMAXPROCS, ze.rib.shards override). Each stored PathGroup arbitrates
// across every protocol (BGP, OSPF, static, kernel, connected) that
// advertised the prefix.
//
// Concurrency: writers contend only on the single shard owning the prefix.
// The outer famMu RWMutex protects only the family.Family -> *familyShards
// map; family creation is rare (O(few) per process), so adding a family
// briefly takes the write lock. subsMu serializes OnChange subscriber
// registration with family creation so a new family inherits a consistent
// subscriber snapshot.
type RIB struct {
	nShards int

	famMu    sync.RWMutex
	families map[family.Family]*familyShards

	subsMu   sync.Mutex
	subsList []subEntry

	nextSub  atomic.Uint64
	revision atomic.Uint64
}

// NewRIB creates an empty Loc-RIB. Families are created lazily on first
// Insert. Shard count comes from ze.rib.shards (clamped [1,64], default
// GOMAXPROCS).
func NewRIB() *RIB {
	return &RIB{
		nShards:  shardCount(),
		families: make(map[family.Family]*familyShards),
	}
}

// Revision changes whenever the selected routing view changes. Consumers read
// it before a scan: a concurrent mutation then remains visible on the next scan.
func (r *RIB) Revision() uint64 { return r.revision.Load() }

// OnChange registers fn to receive a Change every time the best path for a
// prefix is added, updated, removed, or keeps the same best with changed ECMP
// membership or forwarding metadata. Handlers run synchronously under the owning
// shard's write lock, so
// fn MUST NOT re-enter Insert/Remove on the same RIB and should defer any heavy
// work to a goroutine. Returns a
// function that, when called, removes fn from every shard; further changes
// after unsubscribe do not invoke fn.
//
// OnChange replicates the registration into every existing shard's
// subscriber list and into the RIB's subscriber template, so a family
// created after registration also delivers to fn.
func (r *RIB) OnChange(fn ChangeHandler) func() {
	if fn == nil {
		return func() {}
	}
	id := r.nextSub.Add(1)
	entry := subEntry{id: id, fn: fn}

	r.subsMu.Lock()
	r.subsList = append(append([]subEntry(nil), r.subsList...), entry)
	r.famMu.RLock()
	for _, fs := range r.families {
		for i := range fs.shards {
			fs.shards[i].subs.appendEntry(entry)
		}
	}
	r.famMu.RUnlock()
	r.subsMu.Unlock()

	return func() {
		r.subsMu.Lock()
		next := make([]subEntry, 0, len(r.subsList))
		for _, e := range r.subsList {
			if e.id == id {
				continue
			}
			next = append(next, e)
		}
		r.subsList = next
		r.famMu.RLock()
		for _, fs := range r.families {
			for i := range fs.shards {
				fs.shards[i].subs.removeID(id)
			}
		}
		r.famMu.RUnlock()
		r.subsMu.Unlock()
	}
}

// familyShardsFor returns the familyShards for fam, creating it on demand.
// Lock-free fast path on the common (already-present) case; family creation
// briefly takes famMu.Lock + subsMu.Lock to seed shard subscriber lists from
// the current template.
func (r *RIB) familyShardsFor(fam family.Family) *familyShards {
	r.famMu.RLock()
	fs, ok := r.families[fam]
	r.famMu.RUnlock()
	if ok {
		return fs
	}
	r.subsMu.Lock()
	r.famMu.Lock()
	fs, ok = r.families[fam]
	if !ok {
		fs = newFamilyShards(fam, r.nShards, r.subsList)
		r.families[fam] = fs
	}
	r.famMu.Unlock()
	r.subsMu.Unlock()
	return fs
}

// Insert upserts p into (fam, prefix). Returns (best, changed) where best
// is the newly-selected best Path after the insert, and changed reports
// whether the best differs from the pre-insert best. When the prefix is new
// or had no valid best, changed is true whenever the inserted Path is valid
// and an import chain admitted it.
//
// p passes the import gate first (admit, policy.go): a path a chain rejects is
// stored hidden, and when that leaves the prefix with no selectable best the
// insert dispatches ChangeRemove and returns (zero, true). p carries the
// producer's own values: the gate keeps them and applies set actions to a copy.
//
// Insert dispatches Change events without a ForwardHandle. Producers that
// have a wire buffer to share with subscribers call InsertForward instead.
func (r *RIB) Insert(fam family.Family, prefix netip.Prefix, p Path) (Path, bool) {
	return r.insert(fam, prefix, p, nil)
}

// InsertForward is Insert with an attached ForwardHandle. The handle is
// placed on ChangeAdd / ChangeUpdate events dispatched by this insert, so
// subscribers can forward the producer's wire buffer without rebuilding
// from Best.
//
// The caller MUST hold a reference to the handle's backing buffer for the
// duration of this call. Subscribers that retain the buffer past dispatch
// MUST AddRef before returning from the handler.
//
// No handle is propagated on ChangeRemove (Remove means no valid best; the
// subscriber must produce a withdrawal without a source buffer).
//
// Nil contract. To pass "no handle," use untyped nil (or call Insert
// instead). A typed-nil concrete handle packed into the interface
// (`(*myHandle)(nil)`) is stored as-is; subscribers doing the standard
// `if c.Forward != nil { c.Forward.AddRef() }` guard will see the
// interface as non-nil and panic on method dispatch. See ForwardHandle.
func (r *RIB) InsertForward(fam family.Family, prefix netip.Prefix, p Path, forward ForwardHandle) (Path, bool) {
	return r.insert(fam, prefix, p, forward)
}

func (r *RIB) insert(fam family.Family, prefix netip.Prefix, p Path, forward ForwardHandle) (Path, bool) {
	if !p.Valid() || !prefix.IsValid() {
		return Path{}, false
	}
	fs := r.familyShardsFor(fam)
	sh := fs.shardFor(prefix)
	family := fam.String()
	shardIdx := shardIndex(prefix, len(fs.shards))

	// The import gate runs before the shard lock, so a chain's cost never
	// lengthens the time a shard is held. With no chain configured the gate is
	// the snapshot load and admit's nil check.
	snap := ribpolicy.Current()
	stored := admit(snap, fam, prefix, p)

	sh.mu.Lock()
	// A snapshot published between the load above and the lock may already
	// have had its Reselect pass over this shard, which could not see the path
	// not yet stored: judge it again under the new snapshot, or it would keep
	// the old verdict until the next publication.
	if latest := ribpolicy.Current(); latest != snap {
		stored = admit(latest, fam, prefix, p)
	}
	// upsert MUST receive this same resolved value: a second distance read
	// could differ from the one used to judge whether membership changed.
	stored.AdminDistance = resolvedDistance(stored)
	var prevBest Path
	var hadBest bool
	var newBest Path
	var newHad bool
	var created bool
	// A changed effective path requires a before/after member comparison.
	// An unchanged upsert needs no temporary forwarding or observation slices,
	// but MUST still store producer originals and account for the offer below.
	var ecmp []NextHop
	var prevMembers, members []ForwardingMember
	var sameEffective bool

	// Lookup then Insert, the two trie walks store.Modify makes, without its
	// callback: Modify hands the callback a pointer to its local copy, which
	// moves that copy to the heap on every call, and this is the BGP full-table
	// hot path (AC-16 of the route policy spec pins zero allocations).
	g, found := sh.store.Lookup(prefix)
	if found {
		prevBest, hadBest = g.best()
		for i := range g.Paths {
			if g.Paths[i].key() != stored.key() {
				continue
			}
			sameEffective = g.Paths[i].Equal(stored) && slices.Equal(g.Paths[i].ECMP, stored.ECMP)
			break
		}
	} else {
		g = PathGroup{Best: -1}
		created = true
	}
	compareMembers := hadBest && !sameEffective
	if compareMembers {
		prevMembers = forwardingMembers(&g, &prevBest)
	}
	g.upsert(stored)
	sh.store.Insert(prefix, g)
	newBest, newHad = g.best()
	if compareMembers {
		members = forwardingMembers(&g, &newBest)
	}
	depth := sh.store.Len()

	// The path is valid (checked at entry) and upsert placed it into the group,
	// but an import chain may have hidden it, so the group can still have no
	// best. Six outcomes: no best before or after (a hidden path stored, no
	// dispatch), a best before and none after (Remove: every remaining path is
	// hidden), a first best (Add), best identity changed (Update), ECMP
	// membership or metadata changed while best stayed stable (Update without Forward),
	// or no dispatch. changed keeps its long-standing API meaning: true only
	// when the selected best Path changed.
	var (
		retBest Path
		changed bool
	)
	bestChanged := hadBest && !prevBest.Equal(newBest)
	ecmpChanged := compareMembers && !equalForwardingMemberSets(prevMembers, members)
	selectedChanged := hadBest != newHad || bestChanged || ecmpChanged
	if selectedChanged {
		r.revision.Add(1)
		if newHad {
			ecmp = siblingNextHops(&g, newBest)
		}
	}
	switch {
	case !hadBest && !newHad:
		// Only hidden paths: nothing was installed and nothing is.
	case !newHad:
		sh.subs.dispatch(&Change{Family: fam, Prefix: prefix, Kind: ChangeRemove})
		changed = true
	case !hadBest:
		sh.subs.dispatch(&Change{Family: fam, Prefix: prefix, Kind: ChangeAdd, Best: newBest, Forward: forward, ECMP: ecmp})
		retBest, changed = newBest, true
	case bestChanged:
		sh.subs.dispatch(&Change{Family: fam, Prefix: prefix, Kind: ChangeUpdate, Best: newBest, Forward: forward, ECMP: ecmp})
		retBest, changed = newBest, true
	case ecmpChanged:
		sh.subs.dispatch(&Change{Family: fam, Prefix: prefix, Kind: ChangeUpdate, Best: newBest, ECMP: ecmp})
		retBest = newBest
	default:
		retBest = newBest
	}
	sh.mu.Unlock()
	recordInsert(family, shardIdx)
	// Depth only changes when a new prefix entered the store, whether or not
	// its path is hidden. Update and no-op Inserts leave the prefix set size
	// unchanged.
	if created {
		updateDepth(family, shardIdx, depth)
	}
	return retBest, changed
}

// siblingNextHops returns the intra-source equal-cost sibling next-hops of best
// within g: the valid next-hops of Paths that share best's Source and tie it on
// AdminDistance and Metric, EXCLUDING best.NextHop, deduped. It is the data
// carried on Change.ECMP so a consumer can build an ECMP group without
// re-reading the PathGroup. Returns nil for single-path groups (the common
// case) and for any group whose best is not itself a multipath member.
//
// The filter mirrors what sysrib formerly recomputed by re-looking-up the
// PathGroup: same Source as best, not hidden, same AdminDistance, same Metric,
// valid next-hop, different from best.NextHop. Runs under the shard lock with g in
// hand; allocates nothing for single-path groups.
func siblingNextHops(g *PathGroup, best Path) []NextHop {
	if !best.Valid() {
		return nil
	}
	if len(best.ECMP) > 0 {
		return best.ECMP
	}
	return memberNextHops(siblingMembers(g, best))
}

// siblingMembers supplies the intra-source membership for protocol observers.
// Explicit groups on one Path share its metadata; separately inserted paths
// contribute their own metadata. No slice is allocated for a single path.
func siblingMembers(g *PathGroup, best Path) []ECMPMember {
	if !best.Valid() {
		return nil
	}
	// A source that carries its own equal-cost set on the best Path (BGP
	// multipath, which arbitrates one winner across peers rather than inserting
	// one Path per next-hop) supplies the ECMP next-hops directly. Intra-source
	// producers (IS-IS/OSPF) leave Best.ECMP nil and are computed from the group.
	if len(best.ECMP) > 0 {
		members := make([]ECMPMember, len(best.ECMP))
		for i := range best.ECMP {
			members[i] = ECMPMember{NextHop: best.ECMP[i], Labels: best.Labels, SRv6SID: best.SRv6SID}
		}
		return members
	}
	if g == nil || len(g.Paths) <= 1 {
		return nil
	}
	var out []ECMPMember
	for i := range g.Paths {
		p := g.Paths[i]
		if p.Source != best.Source {
			continue
		}
		if !equalCostMember(&p, &best) {
			continue
		}
		nh := NextHop{Addr: p.NextHop, Interface: p.Interface, OnLink: p.OnLink, Weight: p.Weight}
		if !containsMemberTarget(out, nh) {
			out = append(out, ECMPMember{NextHop: nh, Labels: p.Labels, SRv6SID: p.SRv6SID})
		}
	}
	return out
}

func containsMemberTarget(members []ECMPMember, target NextHop) bool {
	for i := range members {
		if members[i].NextHop == target {
			return true
		}
	}
	return false
}

func memberNextHops(members []ECMPMember) []NextHop {
	if len(members) == 0 {
		return nil
	}
	hops := make([]NextHop, len(members))
	for i := range members {
		hops[i] = members[i].NextHop
	}
	return hops
}

// ECMPMembers returns the same selected membership as ECMPNextHops, with the
// forwarding metadata owned by each member. Labels remain borrowed from the
// PathGroup, so callers copy them before leaving its authority read lock.
func (g PathGroup) ECMPMembers(best Path) []ECMPMember {
	return siblingMembers(&g, best)
}

// ECMPNextHops returns the equal-cost sibling next-hops for best in this
// PathGroup. It is the snapshot/replay counterpart to Change.ECMP, for consumers
// that already hold a copied PathGroup and need the same ECMP membership data
// without re-querying the RIB.
func (g PathGroup) ECMPNextHops(best Path) []NextHop {
	return siblingNextHops(&g, best)
}

// removeBatchChunkSize bounds how many prefixes RemoveBatch processes per shard
// write-lock hold so large withdrawals (such as a 1M-route plugin disconnect)
// do not starve concurrent BGP or sysrib lookups.
const removeBatchChunkSize = 1024

// RemoveKey identifies one path to withdraw in RIB.RemoveBatch: the same
// (Family, Prefix, Source, Instance) tuple that RIB.Remove takes.
type RemoveKey struct {
	Family   family.Family
	Prefix   netip.Prefix
	Source   redistevents.ProtocolID
	Instance uint32
}

type removeOutcome struct {
	prevBest    Path
	newBest     Path
	prevMembers []ForwardingMember
	members     []ForwardingMember
	ecmp        []NextHop
	hadBest     bool
	newHad      bool
}

type keyPlacement struct {
	batchIdx int // 1-based index into batches; 0 means skipped
	shardIdx int
}

type familyBatch struct {
	fam     family.Family
	fs      *familyShards
	counts  [maxShards]int
	starts  [maxShards]int
	cursors [maxShards]int
	total   int
	ordered []RemoveKey
}

// Remove deletes the Path matching (source, instance) at (fam, prefix).
// Returns (best, changed) after the removal: best is the remaining best
// Path (zero-value if none), changed reports whether the best differs from
// before. When the last Path for a prefix is removed the prefix is deleted
// from its shard.
func (r *RIB) Remove(fam family.Family, prefix netip.Prefix, source redistevents.ProtocolID, instance uint32) (Path, bool) {
	if !prefix.IsValid() {
		return Path{}, false
	}
	r.famMu.RLock()
	fs, ok := r.families[fam]
	r.famMu.RUnlock()
	if !ok {
		return Path{}, false
	}
	shardIdx := shardIndex(prefix, len(fs.shards))
	sh := &fs.shards[shardIdx]

	sh.mu.Lock()
	var out removeOutcome
	var didRemove bool
	var entryEmpty bool
	key := pathKey{source: source, instance: instance}

	sh.store.ModifyOrDelete(prefix, func(g *PathGroup) bool {
		out.prevBest, out.hadBest = g.best()
		// Capture selected members before removal only when another path can
		// survive. Removing the sole path leaves no best, so members are unused.
		if out.hadBest && len(g.Paths) > 1 {
			out.prevMembers = forwardingMembers(g, &out.prevBest)
		}
		didRemove = g.remove(key)
		if !didRemove {
			return true
		}
		if len(g.Paths) == 0 {
			entryEmpty = true
			return false
		}
		out.newBest, out.newHad = g.best()
		if out.newHad {
			out.members = forwardingMembers(g, &out.newBest)
			out.ecmp = siblingNextHops(g, out.newBest)
		}
		return true
	})

	if !didRemove {
		sh.mu.Unlock()
		return out.prevBest, false
	}

	depth := 0
	if entryEmpty {
		depth = sh.store.Len()
	}
	best, changed := r.dispatchRemoved(sh, fam, prefix, &out)
	sh.mu.Unlock()
	if metricsEnabled() {
		famLabel := fam.String()
		recordRemove(famLabel, shardIdx)
		// Depth changed only when the prefix itself was deleted from the
		// store (entryEmpty == true). entryEmpty == false && !out.newHad means
		// paths remain but none is valid -- prefix count unchanged.
		if entryEmpty {
			updateDepth(famLabel, shardIdx, depth)
		}
	}
	return best, changed
}

// RemoveBatch withdraws every key in keys from the RIB and returns the number
// of paths actually removed. Keys are partitioned by (Family, shard) so each
// shard write lock is acquired per chunk of at most removeBatchChunkSize
// prefixes rather than once per route, and shard metrics are updated once per
// chunk.
//
// Semantics match calling Remove serially for each key: best-path fallback,
// ECMP updates, revision increments, and synchronous OnChange notifications
// under the owning shard's write lock are identical, with keys for the same
// prefix processed in their input order.
func (r *RIB) RemoveBatch(keys []RemoveKey) int {
	if len(keys) == 0 {
		return 0
	}
	batches := r.partitionRemoveKeys(keys)
	totalRemoved := 0
	for i := range batches {
		fb := &batches[i]
		famLabel := ""
		for shardIdx := range fb.fs.shards {
			start := fb.starts[shardIdx]
			end := start + fb.counts[shardIdx]
			if start == end {
				continue
			}
			sh := &fb.fs.shards[shardIdx]
			for off := start; off < end; off += removeBatchChunkSize {
				chunk := fb.ordered[off:min(off+removeBatchChunkSize, end)]
				chunkRemoved, chunkDeleted, depth := r.removeShardChunk(sh, fb.fam, chunk)
				totalRemoved += chunkRemoved
				if chunkRemoved > 0 && metricsEnabled() {
					if famLabel == "" {
						famLabel = fb.fam.String()
					}
					recordRemoveBatch(famLabel, shardIdx, chunkRemoved, depth, chunkDeleted > 0)
				}
			}
		}
	}
	return totalRemoved
}

// partitionRemoveKeys groups keys stably by (Family, shardIdx) into one
// contiguous slice per family, skipping invalid prefixes and absent families.
func (r *RIB) partitionRemoveKeys(keys []RemoveKey) []familyBatch {
	placements := make([]keyPlacement, len(keys))
	var batches []familyBatch
	var famToBatch map[family.Family]int
	var lastFam family.Family
	var lastBatchID int
	hasLast := false

	for i := range keys {
		pfx := keys[i].Prefix
		if !pfx.IsValid() {
			continue
		}
		fam := keys[i].Family
		var batchID int
		if hasLast && fam == lastFam {
			batchID = lastBatchID
		} else {
			var cached bool
			if famToBatch != nil {
				batchID, cached = famToBatch[fam]
			}
			if !cached {
				r.famMu.RLock()
				fs, ok := r.families[fam]
				r.famMu.RUnlock()
				if ok {
					batches = append(batches, familyBatch{fam: fam, fs: fs})
					batchID = len(batches)
				}
				if hasLast {
					if famToBatch == nil {
						famToBatch = make(map[family.Family]int, 4)
						famToBatch[lastFam] = lastBatchID
					}
					famToBatch[fam] = batchID
				}
			}
			lastFam = fam
			lastBatchID = batchID
			hasLast = true
		}
		if batchID == 0 {
			continue
		}
		fb := &batches[batchID-1]
		sIdx := shardIndex(pfx, len(fb.fs.shards))
		placements[i] = keyPlacement{batchIdx: batchID, shardIdx: sIdx}
		fb.counts[sIdx]++
		fb.total++
	}
	if len(batches) == 0 {
		return nil
	}

	for i := range batches {
		fb := &batches[i]
		fb.ordered = make([]RemoveKey, fb.total)
		offset := 0
		for s := range len(fb.fs.shards) {
			fb.starts[s] = offset
			fb.cursors[s] = offset
			offset += fb.counts[s]
		}
	}
	for i := range keys {
		pl := placements[i]
		if pl.batchIdx == 0 {
			continue
		}
		fb := &batches[pl.batchIdx-1]
		pos := fb.cursors[pl.shardIdx]
		fb.ordered[pos] = keys[i]
		fb.cursors[pl.shardIdx] = pos + 1
	}
	return batches
}

// removeShardChunk removes one bounded chunk of keys from sh under a single
// sh.mu write-lock hold.
func (r *RIB) removeShardChunk(sh *shard, fam family.Family, keys []RemoveKey) (removed, deleted, depth int) {
	sh.mu.Lock()
	var (
		key        pathKey
		out        removeOutcome
		didRemove  bool
		entryEmpty bool
	)
	mutate := func(g *PathGroup) bool {
		out.prevBest, out.hadBest = g.best()
		if out.hadBest && len(g.Paths) > 1 {
			out.prevMembers = forwardingMembers(g, &out.prevBest)
		}
		didRemove = g.remove(key)
		if !didRemove {
			return true
		}
		if len(g.Paths) == 0 {
			entryEmpty = true
			return false
		}
		out.newBest, out.newHad = g.best()
		if out.newHad {
			out.members = forwardingMembers(g, &out.newBest)
			out.ecmp = siblingNextHops(g, out.newBest)
		}
		return true
	}
	for i := range keys {
		k := keys[i]
		key = pathKey{source: k.Source, instance: k.Instance}
		out = removeOutcome{}
		didRemove = false
		entryEmpty = false

		sh.store.ModifyOrDelete(k.Prefix, mutate)
		if !didRemove {
			continue
		}
		removed++
		if entryEmpty {
			deleted++
		}
		r.dispatchRemoved(sh, fam, k.Prefix, &out)
	}
	if deleted > 0 {
		depth = sh.store.Len()
	}
	sh.mu.Unlock()
	return removed, deleted, depth
}

// dispatchRemoved updates the revision counter and notifies subscribers for one
// removed path. Caller MUST hold sh.mu.
func (r *RIB) dispatchRemoved(sh *shard, fam family.Family, prefix netip.Prefix, out *removeOutcome) (Path, bool) {
	if !out.newHad {
		if out.hadBest {
			r.revision.Add(1)
			sh.subs.dispatch(&Change{Family: fam, Prefix: prefix, Kind: ChangeRemove})
		}
		return Path{}, out.hadBest
	}
	changed := !out.prevBest.Equal(out.newBest)
	ecmpChanged := !equalForwardingMemberSets(out.prevMembers, out.members)
	if changed || ecmpChanged {
		r.revision.Add(1)
		sh.subs.dispatch(&Change{Family: fam, Prefix: prefix, Kind: ChangeUpdate, Best: out.newBest, ECMP: out.ecmp})
	}
	return out.newBest, changed
}

// Lookup returns a copy of the PathGroup for (fam, prefix). Returns
// (zero, false) when the prefix has no entry.
func (r *RIB) Lookup(fam family.Family, prefix netip.Prefix) (PathGroup, bool) {
	if !prefix.IsValid() {
		return PathGroup{}, false
	}
	r.famMu.RLock()
	fs, ok := r.families[fam]
	r.famMu.RUnlock()
	if !ok {
		return PathGroup{}, false
	}
	sh := fs.shardFor(prefix)
	sh.mu.RLock()
	g, found := sh.store.Lookup(prefix)
	sh.mu.RUnlock()
	recordLookup(fam.String(), shardIndex(prefix, len(fs.shards)))
	return g, found
}

// Inspect runs fn under the shard read lock with the live PathGroup for (fam,
// prefix). It returns true when the prefix exists (fn was called), false
// otherwise. Because the lock is held for the duration of fn, fn may safely
// range g.Paths; it MUST NOT retain g or g.Paths past the call, mutate the RIB,
// or block. Use this instead of Lookup when inspecting Paths concurrently with
// writers: Lookup returns a shallow PathGroup copy whose Paths slice shares the
// stored backing array, so ranging it off-lock races an in-place upsert.
func (r *RIB) Inspect(fam family.Family, prefix netip.Prefix, fn func(PathGroup)) bool {
	if !prefix.IsValid() || fn == nil {
		return false
	}
	r.famMu.RLock()
	fs, ok := r.families[fam]
	r.famMu.RUnlock()
	if !ok {
		return false
	}
	sh := fs.shardFor(prefix)
	// fn is caller-supplied: hold the read lock for its duration but release it even if
	// fn panics, so a panicking inspector cannot wedge the shard for every later reader.
	found := func() bool {
		sh.mu.RLock()
		defer sh.mu.RUnlock()
		g, ok := sh.store.Lookup(prefix)
		if ok {
			fn(g)
		}
		return ok
	}()
	recordLookup(fam.String(), shardIndex(prefix, len(fs.shards)))
	return found
}

// Best returns the currently selected best Path for (fam, prefix), selected
// under the shard read lock rather than from Lookup's borrowed Paths slice.
// Returns (zero, false) when the prefix has no entry or no valid best.
func (r *RIB) Best(fam family.Family, prefix netip.Prefix) (Path, bool) {
	var best Path
	var found bool
	r.Inspect(fam, prefix, func(g PathGroup) {
		best, found = g.best()
	})
	return best, found
}

// LPM performs a longest-prefix-match lookup for addr within the given
// family. Queries all shards and returns the best Path from the PathGroup
// stored under the most specific selectable covering prefix. Hidden-only
// groups do not mask selectable ancestors. Returns (zero, invalid, false)
// when no selectable prefix in the family covers addr.
func (r *RIB) LPM(fam family.Family, addr netip.Addr) (Path, netip.Prefix, bool) {
	if !addr.IsValid() {
		return Path{}, netip.Prefix{}, false
	}
	r.famMu.RLock()
	fs, ok := r.families[fam]
	r.famMu.RUnlock()
	if !ok {
		return Path{}, netip.Prefix{}, false
	}

	var bestPath Path
	var bestPfx netip.Prefix
	found := false

	for i := range fs.shards {
		sh := &fs.shards[i]
		sh.mu.RLock()
		g, pfx, ok := sh.store.LookupLPM(addr)
		if ok {
			p, have := g.best()
			if !have {
				// Hidden candidates stay in the store for inspection, so its
				// longest match can obscure a selectable ancestor in this shard.
				// Only that case walks parents: at most 32 or 128 exact lookups.
				for bits := pfx.Bits() - 1; bits >= 0; bits-- {
					cover := netip.PrefixFrom(addr, bits).Masked()
					ancestor, exists := sh.store.Lookup(cover)
					if !exists {
						continue
					}
					p, have = ancestor.best()
					if have {
						pfx = cover
						break
					}
				}
			}
			if have {
				if !found || pfx.Bits() > bestPfx.Bits() {
					bestPath = p
					bestPfx = pfx
					found = true
				}
			}
		}
		sh.mu.RUnlock()
	}
	if !found {
		return Path{}, netip.Prefix{}, false
	}
	return bestPath, bestPfx, true
}

// Families returns the set of address families that currently hold at
// least one prefix. Order is unspecified.
func (r *RIB) Families() []family.Family {
	r.famMu.RLock()
	defer r.famMu.RUnlock()
	out := make([]family.Family, 0, len(r.families))
	for fam := range r.families {
		out = append(out, fam)
	}
	return out
}

// Iterate visits every prefix in fam. A callback return of false stops
// iteration for that family. Order is unspecified across shards; callers
// that need sorted output must sort at the call site.
//
// The PathGroup passed to fn is a copy; callers must not retain pointers
// into its Paths slice beyond the callback.
func (r *RIB) Iterate(fam family.Family, fn func(prefix netip.Prefix, g PathGroup) bool) {
	r.famMu.RLock()
	fs, ok := r.families[fam]
	r.famMu.RUnlock()
	if !ok {
		return
	}
	for i := range fs.shards {
		sh := &fs.shards[i]
		sh.mu.RLock()
		stop := false
		sh.store.Iterate(func(p netip.Prefix, g PathGroup) bool {
			if !fn(p, g) {
				stop = true
				return false
			}
			return true
		})
		sh.mu.RUnlock()
		if stop {
			return
		}
	}
}

// Len returns the number of prefixes stored for fam across all shards.
func (r *RIB) Len(fam family.Family) int {
	r.famMu.RLock()
	fs, ok := r.families[fam]
	r.famMu.RUnlock()
	// refuse-by-default: guards familyShardsFor -- families are allocated lazily on first insert, so an absent family holds zero prefixes
	if !ok {
		return 0
	}
	return fs.Len()
}
