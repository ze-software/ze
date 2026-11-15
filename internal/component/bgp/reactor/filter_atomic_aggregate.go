// Design: docs/architecture/bgp/egress-attribute-rules.md -- restricted marked-route policy rewrites.
// Related: forward_provenance.go -- registered native NLRI framing.
// RFC 4271 Section 5.1.6 -- see rfc/short/rfc4271.md.
package reactor

import (
	"bytes"
	"errors"
	"hash/maphash"
	"sync"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// atomicAggregatePolicyAllowed applies the release restriction: a marked input
// may retain or remove native route identities, but cannot readdress one or
// remove its marker while still announcing. It intentionally refuses even a
// less-specific replacement; native key equality needs no containment model.
// input MUST be the original chain input, including across final-judge passes.
// Both bodies MUST use ctxID's framing and remain immutable until return. The
// caller MUST turn a refusal or error into its original-route withdrawal.
// Nothing borrowed by this synchronous check survives its return.
//
// RFC 4271 Section 5.1.6: "A BGP speaker that receives a route with the
// ATOMIC_AGGREGATE attribute MUST NOT make any NLRI of that route more specific
// (as defined in 9.1.4) when advertising this route to other BGP speakers."
func atomicAggregatePolicyAllowed(input, output []byte, ctxID bgpctx.ContextID) (bool, error) {
	// RFC 8654 Section 6: the message ceiling also bounds scratch membership.
	if len(input) > maxUpdateBody {
		return false, errAtomicPolicySize
	}
	// RFC 4271 Section 4.3.
	sections, err := wire.ParseUpdateSections(input)
	if err != nil {
		return false, err
	}
	// RFC 4271 Section 5.1.6.
	marked, err := atomicPolicyMarker(sections.Attrs(input))
	if err != nil {
		return false, err
	}
	if !marked {
		return true, nil
	}
	ctx := bgpctx.Registry.Get(ctxID)
	if ctx == nil {
		return false, errAtomicPolicyContext
	}
	if len(output) > maxUpdateBody {
		return false, errAtomicPolicySize
	}
	// The common raw response preserves the complete body. Receipt already
	// validated that input, and no native identity can have changed here.
	if bytes.Equal(input, output) {
		return true, nil
	}
	// RFC 4271 Section 4.3.
	result, err := wire.ParseUpdateSections(output)
	if err != nil {
		return false, err
	}
	scratch := acquireAtomicPolicyScratch()
	scratch.ctx = ctx
	defer scratch.release()
	// RFC 4271 Section 4.3 and RFC 4760 Section 3.
	if err := fwdBodySections(output, scratch.countSection); err != nil {
		return false, err
	}
	if scratch.count == 0 {
		return true, nil // Withdrawals and End-of-RIB advertise no destination.
	}
	// RFC 4271 Section 5.1.6.
	retained, err := atomicPolicyMarker(result.Attrs(output))
	if err != nil {
		return false, err
	}
	if !retained {
		return false, nil
	}
	scratch.count = 0
	// RFC 4271 Section 4.3 and RFC 4760 Section 3.
	if err := fwdBodySections(input, scratch.countSection); err != nil {
		return false, err
	}
	if err := scratch.reserve(); err != nil {
		return false, err
	}
	// RFC 4271 Section 4.3 and RFC 4760 Section 3.
	if err := fwdBodySections(input, scratch.recordSection); err != nil {
		return false, err
	}
	// RFC 4271 Section 5.1.6: compare every effective native announcement.
	if err := fwdBodySections(output, scratch.checkSection); err != nil {
		if errors.Is(err, errAtomicPolicyNewNLRI) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

var (
	errAtomicPolicyContext = errors.New("atomic aggregate policy: NLRI encoding context is unavailable")
	errAtomicPolicySize    = errors.New("atomic aggregate policy: UPDATE exceeds the extended message ceiling")
	errAtomicPolicyNewNLRI = errors.New("atomic aggregate policy: output announces a new native route identity")
	errAtomicPolicyCount   = errors.New("atomic aggregate policy: native route count exceeds validated framing")
)

// atomicPolicyMarker uses the shared attribute index so a duplicate or
// truncated marker cannot count as preservation of a well-known attribute.
//
// RFC 4271 Section 5.1.6: "ATOMIC_AGGREGATE is a well-known discretionary attribute."
func atomicPolicyMarker(attrs []byte) (bool, error) {
	index, ok := spanIndexPool.Get().(*attribute.SpanIndex)
	if !ok {
		panic("BUG: atomic aggregate policy attribute pool holds a foreign value")
	}
	defer spanIndexPool.Put(index)
	// RFC 4271 Section 4.3.
	if err := index.Rebuild(attrs); err != nil {
		return false, err
	}
	span, present := index.Find(attribute.AttrAtomicAggregate)
	if !present {
		return false, nil
	}
	if span.Length != 0 {
		return false, nil
	}
	// The unused low flag bits and Extended Length framing do not change the
	// attribute's identity; Optional, Transitive and Partial do.
	flags := attrs[int(span.Offset)-int(span.HdrLen)]
	return flags&0xe0 == 0x40, nil
}

// atomicPolicyBucket keeps the received path identity separate from its native
// route key. A hash chooses a chain; full canonical key equality decides it.
type atomicPolicyBucket struct {
	family  family.Family
	pathID  uint32
	addPath bool
	hash    uint64
}

type atomicPolicyRecord struct {
	raw      []byte
	next     int
	withdraw bool
	covered  bool
}

// atomicPolicyScratch owns synchronous membership scratch. The zero value is
// not usable: Pool.New installs the seed, callbacks and initial capacity.
// A borrower MUST call release once; release MUST clear every borrowed body
// reference and context before returning it. Safe for one borrower at a time.
// The counted source framing bounds records by maxUpdateBody. Membership uses
// a separate size-class arena: tiny calls do not reserve a maximum-sized map,
// and legal large calls return their arena to its own reusable class.
type atomicPolicyScratch struct {
	arena         *atomicPolicyArena
	ctx           *bgpctx.EncodingContext
	family        family.Family
	count         int
	key           [nlrisplit.PrefixKeyScratchSize]byte
	comparison    [nlrisplit.PrefixKeyScratchSize]byte
	addPath       bool
	err           error
	countSection  func([]byte, family.Family, bool, bool) error
	recordSection func([]byte, family.Family, bool, bool) error
	checkSection  func([]byte, family.Family, bool, bool) error
	recordEntry   func([]byte) bool
	checkEntry    func([]byte) bool
}

var atomicPolicyPool = sync.Pool{New: func() any {
	scratch := new(atomicPolicyScratch)
	// Binding callbacks once prevents an indirect registered splitter from
	// making a new closure and canonical-key buffer escape on every call.
	scratch.countSection = scratch.countAnnouncements
	scratch.recordSection = scratch.recordAnnouncements
	scratch.checkSection = scratch.checkAnnouncements
	scratch.recordEntry = scratch.recordNativeEntry
	scratch.checkEntry = scratch.checkNativeEntry
	return scratch
}}

func acquireAtomicPolicyScratch() *atomicPolicyScratch {
	scratch, ok := atomicPolicyPool.Get().(*atomicPolicyScratch)
	if !ok {
		panic("BUG: atomic aggregate policy scratch pool holds a foreign value")
	}
	return scratch
}

// atomicPolicyArena contains membership storage for one fixed size class. Only
// its pool constructs it; the zero value is not usable. The borrower MUST clear
// records and buckets before returning it to the pool named by class, and MUST
// not read a borrowed raw NLRI after returning the arena.
type atomicPolicyArena struct {
	buckets map[atomicPolicyBucket]int
	records []atomicPolicyRecord
	seed    maphash.Seed
	class   int
}

var atomicPolicyArenaPools = newAtomicPolicyArenaPools()

// newAtomicPolicyArenaPools covers the message-derived bound with powers of
// two from 64 records. Pool.New is the sole storage allocation point; even the
// largest legal input reuses its class on the next call. Each fixed capacity
// bounds one borrow, while normal sync.Pool eviction handles memory pressure.
func newAtomicPolicyArenaPools() []*sync.Pool {
	var pools []*sync.Pool
	for capacity := 64; ; capacity *= 2 {
		class := len(pools)
		pools = append(pools, &sync.Pool{New: func() any {
			return &atomicPolicyArena{
				buckets: make(map[atomicPolicyBucket]int, capacity),
				records: make([]atomicPolicyRecord, 0, capacity),
				seed:    maphash.MakeSeed(),
				class:   class,
			}
		}})
		if capacity >= maxUpdateBody {
			return pools
		}
	}
}

func (s *atomicPolicyScratch) reserve() error {
	if s.count < 0 {
		return errAtomicPolicyCount
	}
	if s.count > maxUpdateBody {
		return errAtomicPolicyCount
	}
	class := 0
	for capacity := 64; capacity < s.count; capacity *= 2 {
		class++
	}
	arena, ok := atomicPolicyArenaPools[class].Get().(*atomicPolicyArena)
	if !ok {
		panic("BUG: atomic aggregate policy arena pool holds a foreign value")
	}
	s.arena = arena
	return nil
}

func (s *atomicPolicyScratch) release() {
	if s.arena != nil {
		clear(s.arena.records)
		s.arena.records = s.arena.records[:0]
		clear(s.arena.buckets)
		atomicPolicyArenaPools[s.arena.class].Put(s.arena)
		s.arena = nil
	}
	s.ctx = nil
	s.family = family.Family{}
	s.count = 0
	s.addPath = false
	s.err = nil
	atomicPolicyPool.Put(s)
}

func (s *atomicPolicyScratch) countAnnouncements(raw []byte, fam family.Family, withdraw, _ bool) error {
	if withdraw {
		return nil
	}
	return s.countRoutes(raw, fam, false)
}

func (s *atomicPolicyScratch) countRoutes(raw []byte, fam family.Family, withdraw bool) error {
	split := nlrisplit.Get(fam)
	if withdraw {
		split = nlrisplit.GetWithdraw(fam)
	}
	if split == nil {
		return nlrisplit.ErrUnsupported
	}
	count, err := split(raw, s.ctx.AddPath(fam), nil)
	if err != nil {
		return err
	}
	if count < 0 {
		return errAtomicPolicyCount
	}
	if count > len(raw) {
		return errAtomicPolicyCount
	}
	if s.count > maxUpdateBody-count {
		return errAtomicPolicyCount
	}
	s.count += count
	return nil
}

func (s *atomicPolicyScratch) recordAnnouncements(raw []byte, fam family.Family, withdraw, _ bool) error {
	if withdraw {
		return nil
	}
	return s.walk(raw, fam, s.recordEntry)
}

func (s *atomicPolicyScratch) checkAnnouncements(raw []byte, fam family.Family, withdraw, _ bool) error {
	if withdraw {
		return nil
	}
	return s.walk(raw, fam, s.checkEntry)
}

func (s *atomicPolicyScratch) walk(raw []byte, fam family.Family, visit func([]byte) bool) error {
	split := nlrisplit.Get(fam)
	if split == nil {
		return nlrisplit.ErrUnsupported
	}
	s.family = fam
	s.addPath = s.ctx.AddPath(fam)
	s.err = nil
	if _, err := split(raw, s.addPath, visit); err != nil {
		return err
	}
	return s.err
}

// recordNativeEntry keeps Path Identifier zero distinct from no ADD-PATH.
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
// message, the NLRI encoding MUST be extended by prepending the Path Identifier
// field, which is of four octets."
func (s *atomicPolicyScratch) recordNativeEntry(raw []byte) bool {
	// RFC 7911 Section 3.
	pathID, native, err := nlri.SplitPathID(raw, s.addPath)
	if err != nil {
		s.err = err
		return false
	}
	s.err = s.recordAnnouncement(native, pathID, s.addPath)
	return s.err == nil
}

// checkNativeEntry reads the same negotiated framing as recordNativeEntry.
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
// message, the NLRI encoding MUST be extended by prepending the Path Identifier
// field, which is of four octets."
func (s *atomicPolicyScratch) checkNativeEntry(raw []byte) bool {
	// RFC 7911 Section 3.
	pathID, native, err := nlri.SplitPathID(raw, s.addPath)
	if err != nil {
		s.err = err
		return false
	}
	s.err = s.checkAnnouncement(native, pathID, s.addPath)
	return s.err == nil
}

func (s *atomicPolicyScratch) recordAnnouncement(raw []byte, pathID uint32, addPath bool) error {
	key, err := nlrisplit.GetPrefixKey(s.family)(raw, s.key[:], false)
	if err != nil {
		return err
	}
	bucket := atomicPolicyBucket{family: s.family, pathID: pathID, addPath: addPath,
		hash: maphash.Bytes(s.arena.seed, key)}
	s.insert(bucket, raw)
	return nil
}

func (s *atomicPolicyScratch) checkAnnouncement(raw []byte, pathID uint32, addPath bool) error {
	key, err := nlrisplit.GetPrefixKey(s.family)(raw, s.key[:], false)
	if err != nil {
		return err
	}
	bucket := atomicPolicyBucket{family: s.family, pathID: pathID, addPath: addPath,
		hash: maphash.Bytes(s.arena.seed, key)}
	present, err := s.contains(bucket, key)
	if err != nil {
		return err
	}
	if !present {
		return errAtomicPolicyNewNLRI
	}
	return nil
}

func (s *atomicPolicyScratch) insert(bucket atomicPolicyBucket, raw []byte) {
	s.arena.records = append(s.arena.records, atomicPolicyRecord{raw: raw, next: s.arena.buckets[bucket]})
	s.arena.buckets[bucket] = len(s.arena.records)
}

// contains is collision-safe even when two different native keys have the same
// hash. Its chain is bounded by the source NLRI count; the per-borrower random
// seed prevents a peer from selecting a predictable quadratic collision set.
func (s *atomicPolicyScratch) contains(bucket atomicPolicyBucket, key []byte) (bool, error) {
	index, err := s.find(bucket, key)
	return index != 0, err
}

// find returns a one-based matching record index, or zero when absent. Native
// withdrawal framing can differ from announcement framing, so each stored raw
// record retains the operation its canonical key must be read under.
func (s *atomicPolicyScratch) find(bucket atomicPolicyBucket, key []byte) (int, error) {
	for next := s.arena.buckets[bucket]; next != 0; {
		record := &s.arena.records[next-1]
		candidate, err := nlrisplit.GetPrefixKey(bucket.family)(record.raw, s.comparison[:], record.withdraw)
		if err != nil {
			return 0, err
		}
		if bytes.Equal(candidate, key) {
			return next, nil
		}
		next = record.next
	}
	return 0, nil
}
