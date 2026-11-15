// Design: docs/architecture/bgp/egress-attribute-rules.md -- source retirement after marked subsets.
// Related: filter_atomic_aggregate.go -- canonical native-key admission and scratch.
// RFC 4271 Section 9.1.3 -- see rfc/short/rfc4271.md.
package reactor

import (
	"bytes"
	"errors"
	"hash/maphash"
	"sync"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// atomicAggregatePolicyRemovals returns owned withdrawal bodies for original
// marked identities absent from the admitted output's announcements AND
// withdrawals. The caller MUST have admitted output against the same immutable
// original input and MUST dispatch these bodies with that source's ownership.
// Each original identity is retired once; a kept identity is never retired.
// Both bodies MUST retain ctxID's native framing until this call returns.
//
// RFC 4271 Section 9.1.3: "If a route in Loc-RIB is excluded from a particular
// Adj-RIB-Out, the previously advertised route in that Adj-RIB-Out MUST be
// withdrawn from service by means of an UPDATE message (see 9.2)."
func atomicAggregatePolicyRemovals(input, output []byte, ctxID bgpctx.ContextID) ([][]byte, error) {
	if len(input) > maxUpdateBody {
		return nil, errAtomicPolicySize
	}
	// RFC 4271 Section 4.3.
	sections, err := wire.ParseUpdateSections(input)
	if err != nil {
		return nil, err
	}
	// RFC 4271 Section 5.1.6.
	marked, err := atomicPolicyMarker(sections.Attrs(input))
	if err != nil {
		return nil, err
	}
	if !marked {
		return nil, nil
	}
	if bytes.Equal(input, output) {
		return nil, nil
	}
	ctx := bgpctx.Registry.Get(ctxID)
	if ctx == nil {
		return nil, errAtomicPolicyContext
	}
	if len(output) > maxUpdateBody {
		return nil, errAtomicPolicySize
	}
	scratch, ok := atomicSubsetPool.Get().(*atomicSubsetScratch)
	if !ok {
		panic("BUG: atomic subset scratch pool holds a foreign value")
	}
	defer scratch.release()
	scratch.keys = acquireAtomicPolicyScratch()
	scratch.keys.ctx = ctx
	// RFC 4271 Section 4.3 and RFC 4760 Sections 3 and 4.
	if err := fwdBodySections(input, scratch.countSection); err != nil {
		return nil, err
	}
	if err := scratch.keys.reserve(); err != nil {
		return nil, err
	}
	if err := fwdBodySections(input, scratch.sourceSection); err != nil {
		return nil, err
	}
	if err := fwdBodySections(output, scratch.coverSection); err != nil {
		return nil, err
	}
	missing := false
	for i := range scratch.keys.arena.records {
		if !scratch.keys.arena.records[i].covered {
			missing = true
			break
		}
	}
	if !missing {
		return nil, nil
	}
	scratch.values = acquireValueScratch()
	// A registered conversion never grows the native field: an RFC 8277
	// label stack becomes one Compatibility field; other framing is copied.
	// The sum of all selected fields is bounded by the original body length.
	scratch.buf = scratch.values.carveBytes(len(input))
	if err := fwdBodySections(input, scratch.emitSection); err != nil {
		return nil, err
	}
	return scratch.bodies()
}

var (
	errAtomicSubsetSource = errors.New("atomic subset: original route identity is unavailable")
	errAtomicSubsetLayout = errors.New("atomic subset: original MP families cannot be grouped")
	errAtomicSubsetWrite  = errors.New("atomic subset: source withdrawal cannot be encoded")
)

type atomicSubsetGroup struct {
	family  family.Family
	afiSAFI [3]byte
	start   int
	length  int
}

// atomicSubsetScratch is one synchronous difference operation. Pool.New binds
// its callbacks once. The zero value is not usable, and a borrower MUST call
// release exactly once. It MUST retain no input, output or value-arena aliases
// after release; release clears those aliases before returning either arena.
// Not safe for concurrent use; different borrowers use separate scratch.
type atomicSubsetScratch struct {
	keys          *atomicPolicyScratch
	values        *valueScratch
	buf           []byte
	native        []byte
	family        family.Family
	withdraw      bool
	multiprotocol bool
	addPath       bool
	err           error
	offset        int
	legacy        int
	groups        [2]atomicSubsetGroup
	groupCount    int
	countSection  func([]byte, family.Family, bool, bool) error
	sourceSection func([]byte, family.Family, bool, bool) error
	coverSection  func([]byte, family.Family, bool, bool) error
	emitSection   func([]byte, family.Family, bool, bool) error
	sourceEntry   func([]byte) bool
	coverEntry    func([]byte) bool
	emitEntry     func([]byte) bool
}

var atomicSubsetPool = sync.Pool{New: func() any {
	scratch := new(atomicSubsetScratch)
	scratch.countSection = scratch.countSource
	scratch.sourceSection = scratch.sourceRoutes
	scratch.coverSection = scratch.coverRoutes
	scratch.emitSection = scratch.emitRoutes
	scratch.sourceEntry = scratch.sourceRoute
	scratch.coverEntry = scratch.coverRoute
	scratch.emitEntry = scratch.emitRoute
	return scratch
}}

func (s *atomicSubsetScratch) release() {
	s.buf, s.native = nil, nil
	if s.values != nil {
		releaseValueScratch(s.values)
		s.values = nil
	}
	if s.keys != nil {
		s.keys.release()
		s.keys = nil
	}
	s.family = family.Family{}
	s.withdraw, s.multiprotocol, s.addPath = false, false, false
	s.err = nil
	s.offset, s.legacy, s.groupCount = 0, 0, 0
	s.groups = [2]atomicSubsetGroup{}
	atomicSubsetPool.Put(s)
}

func (s *atomicSubsetScratch) countSource(raw []byte, fam family.Family, withdraw, _ bool) error {
	return s.keys.countRoutes(raw, fam, withdraw)
}

func (s *atomicSubsetScratch) sourceRoutes(raw []byte, fam family.Family, withdraw, mp bool) error {
	return s.walk(raw, fam, withdraw, mp, s.sourceEntry)
}

func (s *atomicSubsetScratch) coverRoutes(raw []byte, fam family.Family, withdraw, mp bool) error {
	return s.walk(raw, fam, withdraw, mp, s.coverEntry)
}

func (s *atomicSubsetScratch) emitRoutes(raw []byte, fam family.Family, withdraw, mp bool) error {
	return s.walk(raw, fam, withdraw, mp, s.emitEntry)
}

func (s *atomicSubsetScratch) walk(raw []byte, fam family.Family, withdraw, mp bool, visit func([]byte) bool) error {
	split := nlrisplit.Get(fam)
	if withdraw {
		split = nlrisplit.GetWithdraw(fam)
	}
	if split == nil {
		return nlrisplit.ErrUnsupported
	}
	s.family, s.withdraw, s.multiprotocol = fam, withdraw, mp
	s.addPath = s.keys.ctx.AddPath(fam)
	s.err = nil
	if _, err := split(raw, s.addPath, visit); err != nil {
		return err
	}
	return s.err
}

// keyEntry keeps path identity separate from native route identity.
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
// message, the NLRI encoding MUST be extended by prepending the Path Identifier
// field, which is of four octets."
func (s *atomicSubsetScratch) keyEntry(raw []byte) (atomicPolicyBucket, []byte, error) {
	// RFC 7911 Section 3.
	id, native, err := nlri.SplitPathID(raw, s.addPath)
	if err != nil {
		return atomicPolicyBucket{}, nil, err
	}
	s.native = native
	key, err := nlrisplit.GetPrefixKey(s.family)(native, s.keys.key[:], s.withdraw)
	if err != nil {
		return atomicPolicyBucket{}, nil, err
	}
	bucket := atomicPolicyBucket{family: s.family, pathID: id, addPath: s.addPath,
		hash: maphash.Bytes(s.keys.arena.seed, key)}
	return bucket, key, nil
}

func (s *atomicSubsetScratch) sourceRoute(raw []byte) bool {
	bucket, key, err := s.keyEntry(raw)
	if err != nil {
		s.err = err
		return false
	}
	index, err := s.keys.find(bucket, key)
	if err != nil {
		s.err = err
		return false
	}
	if index == 0 {
		s.keys.insert(bucket, s.native)
		s.keys.arena.records[len(s.keys.arena.records)-1].withdraw = s.withdraw
	}
	return true
}

func (s *atomicSubsetScratch) coverRoute(raw []byte) bool {
	bucket, key, err := s.keyEntry(raw)
	if err != nil {
		s.err = err
		return false
	}
	index, err := s.keys.find(bucket, key)
	if err != nil {
		s.err = err
		return false
	}
	if index != 0 {
		s.keys.arena.records[index-1].covered = true
	}
	return true
}

func (s *atomicSubsetScratch) emitRoute(raw []byte) bool {
	bucket, key, err := s.keyEntry(raw)
	if err != nil {
		s.err = err
		return false
	}
	index, err := s.keys.find(bucket, key)
	if err != nil {
		s.err = err
		return false
	}
	if index == 0 {
		s.err = errAtomicSubsetSource
		return false
	}
	record := &s.keys.arena.records[index-1]
	if record.covered {
		return true
	}
	record.covered = true // Also deduplicates original identities during emission.
	n, err := s.writeRoute(raw)
	if err != nil {
		s.err = err
		return false
	}
	if s.multiprotocol {
		s.err = s.groupRoute(n)
	} else {
		s.legacy += n
	}
	s.offset += n
	return s.err == nil
}

// writeRoute converts each source field through its registered writer.
// RFC 8277 Section 2.4: "Upon transmission, the Compatibility field SHOULD be
// set to 0x800000."
func (s *atomicSubsetScratch) writeRoute(raw []byte) (int, error) {
	writer := nlrisplit.GetWithdrawalWriter(s.family)
	if writer != nil {
		// RFC 8277 Section 2.4.
		return writer(s.buf[s.offset:], raw, s.addPath, s.withdraw)
	}
	if len(raw) > len(s.buf)-s.offset {
		return 0, errAtomicSubsetWrite
	}
	return copy(s.buf[s.offset:], raw), nil
}

func (s *atomicSubsetScratch) groupRoute(n int) error {
	for i := range s.groupCount {
		group := &s.groups[i]
		if group.family == s.family {
			if group.start+group.length != s.offset {
				return errAtomicSubsetLayout
			}
			group.length += n
			return nil
		}
	}
	if s.groupCount == len(s.groups) {
		return errAtomicSubsetLayout
	}
	group := &s.groups[s.groupCount]
	group.family, group.start, group.length = s.family, s.offset, n
	group.afiSAFI = [3]byte{byte(s.family.AFI >> 8), byte(s.family.AFI), byte(s.family.SAFI)}
	s.groupCount++
	return nil
}

// bodies makes the existing withdrawal builder own its output before the
// value arena is released. Two MP families require two bodies, never repeated
// MP_UNREACH attributes in one body.
// RFC 4760 Section 4: "An UPDATE message that contains the MP_UNREACH_NLRI is
// not required to carry any other path attributes."
func (s *atomicSubsetScratch) bodies() ([][]byte, error) {
	out := make([][]byte, 0, 2)
	var mp withdrawalMPHalves
	if s.groupCount > 0 {
		group := &s.groups[0]
		mp = withdrawalMPHalves{afiSAFI: group.afiSAFI[:], unreach: s.buf[group.start : group.start+group.length]}
	}
	first, err := s.body(s.buf[:s.legacy], mp)
	if err != nil {
		return nil, err
	}
	out = append(out, first)
	if s.groupCount == 2 {
		group := &s.groups[1]
		mp = withdrawalMPHalves{afiSAFI: group.afiSAFI[:], unreach: s.buf[group.start : group.start+group.length]}
		second, err := s.body(nil, mp)
		if err != nil {
			return nil, err
		}
		out = append(out, second)
	}
	return out, nil
}

// body is the deliberate ownership copy of a synthesized withdrawal.
// RFC 4760 Section 4: "An UPDATE message that contains the MP_UNREACH_NLRI is
// not required to carry any other path attributes."
func (s *atomicSubsetScratch) body(legacy []byte, mp withdrawalMPHalves) ([]byte, error) {
	bound := 4 + len(legacy)
	if mp.afiSAFI != nil {
		bound += 4 + 3 + len(mp.unreach)
	}
	buf := make([]byte, bound)
	// RFC 4271 Section 4.3 and RFC 4760 Section 4.
	n := writeWithdrawal(buf, legacy, nil, mp, s.keys.ctx)
	if n == 0 {
		return nil, errAtomicSubsetWrite
	}
	return buf[:n:n], nil
}
