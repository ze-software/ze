// Design: docs/architecture/core-design.md — one exactly-sized one-pass writer for every announce rail
// RFC: rfc/short/rfc4271.md — attribute header size class (Section 4.3) and ascending emission order (Section 5)
// RFC: rfc/short/rfc6793.md — two- versus four-octet AS_PATH encoding toward the destination
// Overview: forward_build.go — attrEmitter and the edit-set contract this file drives
// Related: reactor_api_batch.go — the established-peer announce rail
// Related: peer_rib_routes.go — the initial-sync drain announce rail

package reactor

import (
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// An announce is an edit set over a base attribute block.
//
// The two announce rails and the forward path used to hold three separate
// writers, each with its own idea of where an attribute goes: attribute.Builder
// emitted a fixed order coded into a function body, the batch rail merge-inserted
// into a byte block with findAttrInsertPosition, and the queued rail interleaved
// range passes around an attrWriter. Which rail ran was decided by
// Peer.shouldQueue -- by scheduling -- so one route could reach the wire as two
// different byte strings.
//
// This file removes the choice. An announce names a BASE (the caller's verbatim
// attribute block, or nothing at all) plus the attributes the rail contributes,
// and hands both to attrEmitter: the same plan-size-write walk buildModifiedPayload
// runs for a forwarded UPDATE. Ordering, header size class and the exact size are
// therefore properties of one writer rather than an agreement between call sites.
//
// The contributed attributes reach the writer through the edit-set vocabulary
// (internal/component/bgp/filterapi/editset.go). Each one is encoded ONCE into a
// scratch region and named as a fragment over it, so the writer copies each value
// straight into the output. That is one more copy per contributed attribute than
// writing it into the output directly, and it is what buys a single writer: the
// forward path pays exactly the same copy for an attribute a filter rebuilt.

// announceInlinePlans is the number of contributed attributes a plan holds without
// a heap allocation. Seven cover the rails' own injections (ORIGIN, AS_PATH,
// NEXT_HOP, MED, LOCAL_PREF, MP_REACH_NLRI, AS4_PATH); the rest absorb the stored
// optional attributes the initial-sync drain replays.
const announceInlinePlans = 16

// announceScratchSize is the scratch region a plan starts with. It matches the
// build-buffer size the announce rails already use, so the common announce never
// grows it.
const announceScratchSize = 4096

// announcePlanPool holds the plans themselves, scratch region included.
//
// A plan is about two kilobytes -- the inline slot, fragment and arena stores the
// edit set carries, plus this file's own entry array and scratch. Declaring one per
// route put that on the heap on every announce, which is the cost the exactly-sized
// rebuild exists to avoid, so a plan is taken and returned exactly as a build
// buffer is. getAnnouncePlan begins it; putAnnouncePlan releases it.
var announcePlanPool = sync.Pool{
	New: func() any { return &announceAttrs{} },
}

// getAnnouncePlan returns a plan ready to collect contributions. The caller MUST
// call putAnnouncePlan when the announce is built or refused.
func getAnnouncePlan() *announceAttrs {
	p, _ := announcePlanPool.Get().(*announceAttrs)
	p.begin()
	return p
}

// putAnnouncePlan returns a plan to the pool.
func putAnnouncePlan(p *announceAttrs) {
	p.release()
	announcePlanPool.Put(p)
}

// announceDstCtxASN4 and announceDstCtxOld are the two destination encoding
// contexts an announce can need (RFC 6793: four- versus two-octet AS numbers).
//
// bgpctx.EncodingContextForASN4 builds a fresh context and FNV-hashes it on every
// call -- six allocations -- and its answer is a pure function of one bool. An
// EncodingContext is immutable after construction (it has no mutating method and
// assigns no field outside its constructor) and the registry already shares one
// pointer across goroutines, so two package-level values serve every announce.
var (
	announceDstCtxASN4 = bgpctx.EncodingContextForASN4(true)
	announceDstCtxOld  = bgpctx.EncodingContextForASN4(false)
)

// announceDstCtx returns the shared destination context for a peer's ASN width.
func announceDstCtx(asn4 bool) *bgpctx.EncodingContext {
	if asn4 {
		return announceDstCtxASN4
	}
	return announceDstCtxOld
}

// announcePlanEntry is one contributed attribute: its type code, the flags byte the
// attribute type declares, and the window in the scratch region holding its
// encoded VALUE.
type announcePlanEntry struct {
	off    int
	length int
	code   uint8
	flags  byte
	ext    bool // the attribute declares the Extended Length header whatever its size
	remove bool // the attribute leaves the UPDATE: the base carries it and the rail must not emit it
}

// announceAttrs collects the attributes one announce contributes and materializes
// them over a base block.
//
// NOT safe for concurrent use. One announce at a time; call release when done.
type announceAttrs struct {
	mods    filterapi.ModAccumulator
	scratch []byte
	used    int

	planArr [announceInlinePlans]announcePlanEntry
	plans   []announcePlanEntry

	// Reusable attribute values. A rail contributes an attribute through an
	// interface, so a composite literal at the call site escapes and costs one
	// allocation per route. These fields live in the pooled plan, so taking their
	// address allocates nothing: the plan is already on the heap and is reused.
	nh      attribute.NextHop
	asPath  attribute.ASPath
	as4Path attribute.AS4Path
	segArr  [1]attribute.ASPathSegment
	asnArr  [2]uint32

	present [4]uint64 // one bit per contributed type code, so a duplicate is caught
	failed  bool
	reason  string
	// cause is the refusal's machine-readable half, for a caller whose operator
	// action differs by reason. Nil when the reason has no cause a caller acts on
	// differently from "this announce was not encodable".
	cause error
}

// begin readies the plan for one announce.
func (p *announceAttrs) begin() {
	if p.scratch == nil {
		p.scratch = make([]byte, announceScratchSize) // once per pooled plan
	}
	p.used = 0
	p.plans = p.planArr[:0]
	p.present = [4]uint64{}
	p.failed = false
	p.reason = ""
	p.cause = nil
	p.mods.Reset()
}

// release drops what the next announce must not inherit. The scratch region and
// the accumulator stay: reusing them is the point of pooling the plan.
func (p *announceAttrs) release() {
	p.plans = nil
	p.asPath.Segments = nil
	p.as4Path.Segments = nil
	p.mods.Reset()
	if len(p.scratch) > announceScratchSize {
		// An announce that outgrew the standard region does not hand an outsized
		// buffer to the next one, exactly as acquireModBuf declines to pool an
		// oversized rebuild buffer.
		p.scratch = nil
	}
}

// asPathFor returns the plan's reusable AS_PATH carrying asns as one AS_SEQUENCE.
//
// RFC 4271 Section 4.3: a segment carries at most 255 AS numbers, and the announce
// rails synthesize at most two, so one segment always suffices. The segment and ASN
// arrays live in the pooled plan, so the whole shape costs no allocation.
func (p *announceAttrs) asPathFor(asns []uint32) *attribute.ASPath {
	n := copy(p.asnArr[:], asns)
	p.segArr[0] = attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: p.asnArr[:n]}
	p.asPath.Segments = p.segArr[:1]
	if n == 0 {
		p.asPath.Segments = nil
	}
	return &p.asPath
}

// as4PathFor returns the plan's reusable AS4_PATH over the same segments.
func (p *announceAttrs) as4PathFor(segments []attribute.ASPathSegment) *attribute.AS4Path {
	p.as4Path.Segments = segments
	return &p.as4Path
}

// nextHopFor returns the plan's reusable NEXT_HOP.
func (p *announceAttrs) nextHopFor(addr netip.Addr) *attribute.NextHop {
	p.nh.Addr = addr
	return &p.nh
}

// fail records the first reason the plan cannot be materialized, and the cause a
// caller acts on. Every later call is a no-op, so a caller checks once at the end
// rather than at every add (ai/rules/evidence.md), and first-reason-wins holds for
// the cause because it is recorded here rather than beside the call.
//
// cause is nil for a reason whose operator action is the rail's default. It is
// non-nil where the action DIFFERS, which is what refusalCause exists to carry.
func (p *announceAttrs) fail(reason string, cause error) {
	if !p.failed {
		p.failed = true
		p.reason = reason
		p.cause = cause
	}
}

// refusalCause returns the cause of the refusal emit reported, or nil when the
// plan did not fail or its reason carries no distinct operator action.
//
// It exists because a rail cannot tell WHY emit refused, and the two answers need
// different words: "the announce did not fit" asks for fewer prefixes, and an
// unencodable next hop asks for a next hop. Without it both rails reported every
// refusal as oversize, so the one refusal the add backstop can raise on its own --
// an attribute with no wire form -- told the operator to send fewer prefixes on a
// batch whose size was never the problem (ai/rules/cli.md).
func (p *announceAttrs) refusalCause() error { return p.cause }

// reserve makes room for n more scratch bytes, growing past the pooled region when
// an announce needs it.
func (p *announceAttrs) reserve(n int) bool {
	if n < 0 {
		return false
	}
	if p.used+n <= len(p.scratch) {
		return true
	}
	if p.used+n > maxUpdateBody {
		return false
	}
	grown := make([]byte, max(p.used+n, 2*len(p.scratch))) // outgrew the pooled region
	copy(grown, p.scratch[:p.used])
	p.scratch = grown
	return true
}

// planned reports whether a type code has already been contributed.
func (p *announceAttrs) planned(code uint8) bool {
	return p.present[code>>6]&(uint64(1)<<(code&63)) != 0
}

// announceNextHopValidator is what the plan asks an attribute that can be
// well-formed as a Go value and still have no wire form. attribute.MPReachNLRI is
// the case: RFC 4760 Section 3 counts its Next Hop field with a length octet, and
// an unresolved address fills none of it.
//
// Declared here rather than in the attribute package because this is the consumer
// that needs the answer, and asking through an interface keeps add from spelling a
// type code.
type announceNextHopValidator interface{ ValidateNextHops() error }

// add contributes one attribute, encoded under dstCtx.
//
// dstCtx decides the AS_PATH and AGGREGATOR ASN width (RFC 6793 Section 4.1). It
// may be nil, which means the four-octet encoding. The value length comes from the
// same ValueLenWithContext the write uses, and a write that returns a different
// count fails the plan: a size query and a write that disagree are the one thing
// this whole shape exists to make impossible.
func (p *announceAttrs) add(attr attribute.Attribute, dstCtx *bgpctx.EncodingContext) {
	if p.failed {
		return
	}
	code := uint8(attr.Code())
	if p.planned(code) {
		// Two contributions for one type code would emit the attribute twice, which
		// RFC 4271 Section 4.3 makes a Malformed Attribute List and RFC 7606
		// Section 3(g) makes treat-as-withdraw at the receiver.
		p.fail("duplicate attribute type code", nil)
		return
	}
	// An attribute with no wire form is refused HERE, and this is the place because
	// add is the single point both announce rails reach the wire through: they go
	// WriteToWithContext -> WriteTo, and neither ever calls CheckedWriteTo, which
	// takes no EncodingContext and so cannot be their entry point.
	//
	// It is not redundant with the length check below. Until 2026-08-08 an
	// unresolved MP_REACH next hop was caught only by accident, because Len()
	// counted sixteen octets the write never emitted; deriving both from
	// netip.Addr.AsSlice made them agree at ZERO and silenced that accident, leaving
	// buildRIBRouteUpdate (peer_rib_routes.go) emitting a Length of Next Hop Network
	// Address octet of 0x00. validateMPReachNextHop (message/rfc7606.go) makes
	// exactly that octet an RFC7606ActionSessionReset, so the trade was a dropped
	// route for a reset session (RFC 7606 Section 7.11).
	//
	// It costs the hot path no allocation: the assertion is interface-to-interface
	// over a value that is already an interface, and the check itself is a loop of
	// netip.Addr.IsValid. Measured over BenchmarkAnnounceRails/batch-mp, 3 allocs/op
	// and 304 B/op with the guard and with it disabled.
	//
	// batch-mp is the sub-benchmark that names the family whose next hop travels
	// INSIDE the attribute, and it is the only one of the three that measures this
	// branch: the batch and queued sub-benchmarks are IPv4 unicast, so their next hop
	// is a NEXT_HOP attribute, this assertion fails, and ValidateNextHops never runs.
	// An earlier version of this comment cited the whole benchmark for the number,
	// which measured the failed assertion alone (ai/rules/evidence.md).
	if v, ok := attr.(announceNextHopValidator); ok {
		if err := v.ValidateNextHops(); err != nil {
			// The cause travels with the refusal. It is the one reason this function
			// raises that a rail must report differently from every other: the announce
			// is the right size and the next hop is what must change, so a rail that
			// assumed oversize told the operator to send fewer prefixes (refusalCause).
			p.fail("attribute next hop has no wire form", err)
			return
		}
	}
	valueLen := attribute.ValueLenWithContext(attr, dstCtx)
	if valueLen < 0 {
		p.fail("attribute value length is not computable", nil)
		return
	}
	if !p.reserve(valueLen) {
		p.fail("attribute values exceed the announce scratch region", nil)
		return
	}
	if n := attr.WriteToWithContext(p.scratch, p.used, nil, dstCtx); n != valueLen {
		p.fail("attribute size query disagreed with its own write", nil)
		return
	}
	flags := attr.Flags()
	p.plans = append(p.plans, announcePlanEntry{
		off:    p.used,
		length: valueLen,
		code:   code,
		flags:  byte(flags),
		ext:    flags.IsExtLength(),
	})
	p.present[code>>6] |= uint64(1) << (code & 63)
	p.used += valueLen
	if len(p.plans) > announceInlinePlans {
		// The inline capacity is a static census of the rails' injections plus the
		// replayed optional attributes; a spill is correct and never refused, but it
		// allocated, so it is counted rather than assumed
		// (filterapi.RecordEditSpill covers the edit set's own three stores).
		filterapi.RecordEditSpill(filterapi.EditStoreSlots)
	}
}

// drop removes an attribute the BASE carries from the announce.
//
// It is the third thing a rail can say about a type code, beside contributing a
// value and leaving the base alone, and it exists because a mandatory-attribute
// rule can be a prohibition as well as an obligation: RFC 4271 Section 5.1.5
// forbids LOCAL_PREF toward an external peer, and the caller's block is copied
// through, so "say nothing" emits it.
//
// The base is not rewritten. The slot's kind makes the writer emit nothing for
// that code, exactly as a filter handler's Drop does on the forward path, so the
// removal costs a skipped copy rather than a memmove.
func (p *announceAttrs) drop(code uint8) {
	if p.failed {
		return
	}
	if p.planned(code) {
		p.fail("duplicate attribute type code", nil)
		return
	}
	p.plans = append(p.plans, announcePlanEntry{code: code, remove: true})
	p.present[code>>6] |= uint64(1) << (code & 63)
}

// emit materializes the announce into dst and returns the bytes written.
//
// base is the caller's verbatim attribute block, or nil for an announce built
// entirely from contributed attributes -- the edit set over an EMPTY base. dst is
// the attribute REGION, not the whole buffer: the initial-sync rail parks the NLRI
// at the tail of the same slot, so the region bound is an argument rather than a
// buffer length. Passing a longer slice would let the attributes overwrite the
// prefix being announced.
//
// Returns ok=false, having written nothing, when the announce cannot be encoded.
// Every reason is decided before the write, so a partially written region is not a
// state this function can produce.
func (p *announceAttrs) emit(base, dst []byte) (int, bool) {
	if p.failed {
		routesLogger().Warn("announce rejected: attribute plan refused",
			"reason", p.reason, "attr-count", len(p.plans),
			"action", "route not sent to this peer")
		return 0, false
	}

	// Ascending type-code order (RFC 4271 Section 5). attrEmitter merge-inserts a
	// contributed attribute at the first base position whose code sorts after it,
	// and reads the contributions in plan order, so the plan is sorted first. The
	// forward path reaches the same order through GroupedOps; this is the same sort
	// over a handful of entries, in place and allocation-free.
	p.sortByCode()

	p.mods.Reset()
	for i := range p.plans {
		e := &p.plans[i]
		p.mods.Op(e.code, filterapi.AttrModSet, p.scratch[e.off:e.off+e.length])
	}

	// Index the base once, with the verdicts RFC 4271 Section 4.3 requires: a header
	// that does not parse, a duplicate type code, or an attribute running past the
	// end is malformed. The batch rail used to walk a caller-supplied block with
	// none of those checks and merge-insert into it regardless.
	spans, err := attribute.BuildSpanIndex(base)
	if err != nil {
		routesLogger().Warn("announce rejected: base attribute block does not index",
			"base-bytes", len(base), "error", err,
			"action", "route not sent to this peer")
		return 0, false
	}

	edit := p.mods.EditSet()
	edit.Begin()
	ops := p.mods.Ops()

	for i := range p.plans {
		e := &p.plans[i]
		var src []byte
		srcOff, srcLen, valOff, valLen := 0, 0, 0, 0
		if span, ok := spans.Find(attribute.AttributeCode(e.code)); ok {
			// The base already carries this code, so the contribution REPLACES it in
			// the base's own position. That is what makes an authoritative NEXT_HOP or
			// MP_REACH_NLRI a replacement rather than a second copy (RFC 7606
			// Section 3(g): a duplicate is treat-as-withdraw).
			valOff = int(span.Offset)
			valLen = int(span.Length)
			srcOff = valOff - int(span.HdrLen)
			srcLen = int(span.HdrLen) + valLen
			src = base[srcOff : srcOff+srcLen]
		}
		ap := edit.Attr(e.code, src, srcOff, srcLen, valOff, valLen, ops[i:i+1], i)
		switch {
		case e.remove:
			ap.Drop()
		case e.ext:
			ap.Op(0)
			ap.EmitExtended(e.flags, e.code)
		default:
			ap.Op(0)
			ap.Emit(e.flags, e.code)
		}
		if id := edit.Commit(ap); edit.SlotFailed(id) {
			routesLogger().Warn("announce rejected: attribute plan refused at commit",
				"code", e.code, "action", "route not sent to this peer")
			return 0, false
		}
	}

	emitter := attrEmitter{edit: edit, spans: &spans, section: base, ops: ops}

	// SIZE, then WRITE, by the same walk. The count is exact rather than an upper
	// bound, which is what lets the region bound below be a refusal instead of a
	// silently clamped copy.
	size, fail := emitter.run(nil, 0)
	if fail.failed() {
		return 0, false
	}
	if size > len(dst) {
		return 0, false
	}
	written, fail := emitter.run(dst, 0)
	if fail.failed() {
		return 0, false
	}
	if written != size {
		routesLogger().Warn("announce rejected: attribute size query disagreed with the write",
			"sized", size, "written", written, "action", "route not sent to this peer")
		return 0, false
	}
	return written, true
}

// sortByCode orders the contributions by ascending attribute type code.
//
// Insertion sort rather than sort.Slice, for the reason GroupedOps gives: the
// entry count is a handful and sort.Slice would allocate a reflect-based swapper.
func (p *announceAttrs) sortByCode() {
	for i := 1; i < len(p.plans); i++ {
		e := p.plans[i]
		j := i - 1
		for j >= 0 && p.plans[j].code > e.code {
			p.plans[j+1] = p.plans[j]
			j--
		}
		p.plans[j+1] = e
	}
}
