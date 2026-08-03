// Design: docs/architecture/wire/attributes.md — the fragment model and the header size class
// RFC: rfc/short/rfc4271.md — attribute value length cap and the Extended Length header class (Section 4.3)
// RFC: rfc/short/rfc8654.md — extended message body ceiling, which bounds the size delta
// Overview: filterapi.go — ModAccumulator, the public face of the edit set
// Related: metrics.go — the spill counter this file feeds

package filterapi

// The edit set is the vocabulary a handler uses to say how a new attribute value
// relates to the old one.
//
// Before it existed, every handler that kept most of an attribute still rebuilt
// the whole value in an intermediate buffer, because there was no way to say
// "these bytes are already on the wire, reuse them". One handler solved that by
// hand: the MP_REACH next-hop rewrite writes AFI/SAFI from the source, then the
// new next-hop, then the NLRI tail straight from the source. That is a fragment
// list, written longhand. This file gives it a name.
//
// A fragment names a SOURCE, an offset and a length. Three sources cover every
// edit the engine applies:
//
//   - the source attribute's VALUE, for bytes already on the wire;
//   - an operation's buffer, for bytes a producer already built, or its
//     GENERATOR, for bytes that exist nowhere yet and are written straight into
//     the destination at materialization time;
//   - the arena, for the rare byte a handler must synthesize.
//
// Nothing is copied while planning. The exact output length is the plan's own
// arithmetic, so a size query and the write CANNOT disagree: there is only one
// sum, maintained as fragments are added, and the writer materializes exactly
// what the plan describes.

// Inline capacities. They come from a static census of the edit producers, not
// from a traffic histogram: ORIGINATOR_ID(9), CLUSTER_LIST(10), NEXT_HOP(3),
// MP_REACH(14), COMMUNITY(8), EXTENDED_COMMUNITY(16), LARGE_COMMUNITY(32),
// OTC(35), PrefixSID(40), AS_PATH(2), AS4_PATH(17). The common case is five.
// Only these constants would change if a histogram said otherwise; the structure
// is unaffected, and a spill is counted rather than refused.
const (
	editSlotInline  = 12
	editFragInline  = 24
	editArenaInline = 128
)

// attrValueMax is the RFC 4271 Section 4.3 ceiling on one attribute value.
const attrValueMax = 65535

// fragmentSource names where a fragment's bytes live.
type fragmentSource uint8

const (
	// fragValue reads the SOURCE attribute's value bytes. Offsets are relative
	// to the start of that value, which is what every handler already thinks in.
	fragValue fragmentSource = iota
	// fragOp reads one operation: its pre-built buffer, or its generator when the
	// operation carries one. Both are named by an operation index, so they are
	// one source; only the write differs.
	fragOp
	// fragArena reads bytes the handler synthesized. Only bytes that did not
	// already exist anywhere else ever land here.
	fragArena
)

// fragment names one run of bytes by source, offset and length. Six bytes, and
// deliberately pointer-free: a slot resolves its fragments against the attribute
// section and the operation slice handed in at write time, so nothing in the
// edit set outlives the base it describes (docs/architecture/memory/lifetime-contracts.md).
type fragment struct {
	off     uint16
	n       uint16
	source  fragmentSource
	opIndex uint8 // valid when source == fragOp; index within the slot's own ops
}

// slotKind is what a planned attribute does on emission.
type slotKind uint8

const (
	// slotEmit writes a header plus the fragment list.
	slotEmit slotKind = iota
	// slotVerbatim emits the source attribute unchanged, header included. The
	// writer treats it exactly like an untouched attribute, so it coalesces into
	// the surrounding copy run instead of costing a copy of its own.
	slotVerbatim
	// slotDrop emits nothing: the attribute leaves the UPDATE.
	slotDrop
	// slotFail is a handler refusing. The route is suppressed for this
	// destination rather than forwarded missing the change the policy required
	// (ai/rules/evidence.md).
	slotFail
)

// editSlot is one touched attribute's plan. Every field is an integer: a slot
// holds no pointer into the payload, so the edit set inherits the base's buffer
// lifetime without holding it alive.
type editSlot struct {
	valOff   uint32 // source value offset, relative to the attribute SECTION
	valLen   uint32 // source value length
	fragFrom uint32
	fragTo   uint32
	outLen   uint32 // exact output VALUE length; the whole point of this type
	opFrom   uint32 // this code's operations, as a range into the accumulator's ops
	opTo     uint32
	srcOff   uint32 // whole source attribute offset in the section, for slotVerbatim
	srcLen   uint32
	code     uint8
	flags    uint8
	hdrLen   uint8 // 3, or 4 when the Extended Length flag is set
	kind     slotKind
}

// SlotID identifies one planned attribute inside an EditSet.
type SlotID int

// EditSet holds the slots, fragments and arena for ONE destination's rebuild.
//
// It lives inside the ModAccumulator so that hoisting the accumulator above the
// destination loop hoists this too. Reset re-slices the three stores to zero
// length against their inline arrays and never re-zeroes an array, so the cost
// is independent of the inline capacity — which is the whole reason the hoist
// pays for itself.
//
// NOT safe for concurrent use. One destination at a time.
type EditSet struct {
	slotsArr [editSlotInline]editSlot
	slots    []editSlot

	fragsArr [editFragInline]fragment
	frags    []fragment

	arenaArr [editArenaInline]byte
	arena    []byte

	// cur is the reusable per-attribute planner. Reusing one value is what keeps
	// planning allocation-free; the handler contract forbids retaining it.
	cur AttrPlan

	// gens resolves an operation's AttrGenerator by its one-based GenIdx. The
	// accumulator owns the store and hands it over with SetGens for the duration
	// of one rebuild; the edit set only reads it.
	gens []AttrGenerator

	// touched carries one presence bit per attribute type code, so "is this code
	// planned" is answered without scanning the slots. It is the same shape the
	// span index uses, and it replaces the 256-entry array the old grouping
	// returned by value on every destination.
	touched [4]uint64

	spilledSlots bool
	spilledFrags bool
	spilledArena bool
}

// SlotCount returns the number of planned attributes.
func (e *EditSet) SlotCount() int { return len(e.slots) }

// SlotAt returns the i-th slot in plan order.
func (e *EditSet) SlotAt(i int) SlotID { return SlotID(i) }

// Planned reports whether a type code has a slot, in constant time.
func (e *EditSet) Planned(code uint8) bool {
	return e.touched[code>>6]&(uint64(1)<<(code&63)) != 0
}

// Find returns the slot planned for a type code. The presence bitset settles
// absence before any slot is read, so the linear scan runs only for a code that
// is genuinely planned and only over the handful of slots one destination has.
func (e *EditSet) Find(code uint8) (SlotID, bool) {
	if !e.Planned(code) {
		return 0, false
	}
	for i := range e.slots {
		if e.slots[i].code == code {
			return SlotID(i), true
		}
	}
	return 0, false
}

func (e *EditSet) mark(code uint8) {
	e.touched[code>>6] |= uint64(1) << (code & 63)
}

// reset returns the edit set to its empty state in constant time.
//
// It re-slices rather than re-zeroes, exactly as ModAccumulator.Reset does, and
// for the same reason: a reset whose cost scales with the inline capacity would
// make a larger inline value a regression rather than an optimization. The
// unreachable entries keep no pointer into any payload, because a slot stores
// only integers, so nothing is held alive across the reset.
func (e *EditSet) reset() {
	e.slots = e.slotsArr[:0]
	e.frags = e.fragsArr[:0]
	e.arena = e.arenaArr[:0]
	e.touched = [4]uint64{}
	e.spilledSlots = false
	e.spilledFrags = false
	e.spilledArena = false
	// cur is the ONE part of the edit set that holds pointers: the source
	// attribute, its value and the operation slice of the attribute last
	// planned. Attr overwrites it before every handler call, so a stale value is
	// never readable -- but leaving it set keeps the PREVIOUS destination's
	// payload buffer alive for the whole of the next one, which is exactly the
	// boundary this type claims not to cross. Clearing three slice headers costs
	// the same whatever the inline capacities are, so the constant-time property
	// above is unaffected.
	e.cur = AttrPlan{}
	// The generator store belongs to the accumulator, which clears it in its own
	// Reset. Dropping the reference here keeps the edit set from holding the
	// previous destination's generators -- and through them its payload -- alive
	// across the boundary.
	e.gens = nil
}

// ensure lazily establishes the inline aliases for an accumulator that was never
// Reset. The forward rails Reset at the top of every destination, but the policy
// chains declare a fresh accumulator and plan straight away; without this they
// would heap-allocate on first append instead of using the inline arrays.
func (e *EditSet) ensure() {
	if e.slots == nil {
		e.slots = e.slotsArr[:0]
	}
	if e.frags == nil {
		e.frags = e.fragsArr[:0]
	}
	if e.arena == nil {
		e.arena = e.arenaArr[:0]
	}
}

// Spilled reports whether any of the three stores exceeded its inline capacity.
// Attribute count and community-list length are peer-influenceable, so a spill
// is visible rather than silent.
func (e *EditSet) Spilled() (slots, frags, arena bool) {
	return e.spilledSlots, e.spilledFrags, e.spilledArena
}

// SetGens hands the accumulator's generator store to the edit set for the
// duration of one rebuild. It is called after Begin, because Begin clears the
// reference along with everything else.
//
// The edit set only READS the store. It is separate from Begin so the ownership
// stays visible: the generators belong to the accumulator, which cleared and
// refilled them while the destination's operations were being recorded.
func (e *EditSet) SetGens(gens []AttrGenerator) { e.gens = gens }

// generator resolves a one-based GenIdx, returning nil for 0 (no generator) and
// for any index outside the store. An out-of-range index reads as absent rather
// than as a different generator, so a mis-recorded operation falls back to its
// Buf and cannot silently emit another attribute's bytes.
func (e *EditSet) generator(idx uint8) AttrGenerator {
	if idx == 0 || int(idx) > len(e.gens) {
		return nil
	}
	return e.gens[idx-1]
}

// Begin clears the edit set for one destination's rebuild and returns it ready
// to plan. It is separate from ModAccumulator.Reset because the operations a
// destination accumulated are read DURING the rebuild: resetting the whole
// accumulator here would discard the very ops being planned.
func (e *EditSet) Begin() {
	e.reset()
}

// AttrPlan is the builder a handler uses to describe one attribute's output.
//
// A handler never writes a byte. It appends fragments and finishes with exactly
// one of Emit, EmitExtended, Drop or Fail. The plan maintains the running value
// length as fragments are added, so the exact output size is known the moment
// the handler returns and cannot drift from what the writer later produces.
//
// MUST NOT be retained beyond the handler call: one value is reused for every
// attribute of every destination.
type AttrPlan struct {
	set    *EditSet
	src    []byte // the FULL source attribute, nil when absent from the source
	val    []byte // the source attribute's value, nil when absent
	ops    []AttrOp
	code   uint8
	nOps   int
	slot   editSlot
	closed bool
}

// Ops returns the operations accumulated for this attribute code, in the order
// their producers recorded them.
func (p *AttrPlan) Ops() []AttrOp { return p.ops }

// Source returns the FULL source attribute bytes (flags, code, length, value),
// or nil when the attribute is absent from the source UPDATE.
func (p *AttrPlan) Source() []byte { return p.src }

// Value returns the source attribute's value bytes, or nil when the attribute is
// absent. Keep offsets are relative to the start of this slice.
func (p *AttrPlan) Value() []byte { return p.val }

// Code returns the attribute type code being planned.
func (p *AttrPlan) Code() uint8 { return p.code }

// ValueLen returns the number of value bytes planned so far.
//
// A handler reads it to decide whether anything survived its operations: a
// community list whose every value was removed plans zero bytes and must Drop,
// because an attribute with an empty value is not the same thing as no
// attribute.
func (p *AttrPlan) ValueLen() int { return int(p.slot.outLen) }

// KeepAll emits the source attribute unchanged, header included.
//
// It is both the cheapest outcome and the most faithful: the bytes are copied as
// one run that coalesces with the untouched attributes around it, and the source
// header size class is preserved rather than recomputed. A handler that means
// "leave this attribute alone" must say KeepAll, not rebuild an identical value,
// because rebuilding normalizes a legal-but-unusual header and moves bytes on
// the wire.
func (p *AttrPlan) KeepAll() {
	if p.closed {
		return
	}
	if p.src == nil {
		// Nothing to keep. A handler asking to preserve an attribute that is not
		// there is a caller bug; refuse rather than emit an empty attribute.
		p.Fail()
		return
	}
	p.slot.kind = slotVerbatim
	p.closed = true
}

// Keep appends a fragment over the source attribute's VALUE bytes.
//
// off and n are relative to Value(). A fragment that runs outside the value, or
// one requested when the attribute is absent from the source, refuses the whole
// plan: a fragment is resolved against peer-controlled bytes at write time, so
// the bound is checked here, once, rather than trusted there
// (ai/rules/evidence.md).
//
// Adjacent value fragments coalesce, so a community removal that retains a run
// of consecutive values emits one fragment for the whole run.
func (p *AttrPlan) Keep(off, n int) {
	if p.closed {
		return
	}
	if n == 0 {
		return
	}
	if p.val == nil || off < 0 || n < 0 || off+n > len(p.val) {
		p.Fail()
		return
	}
	p.appendFragment(fragValue, off, n, 0)
}

// Op appends the whole of the i-th operation as a fragment. The bytes are not
// copied: the operation slice stays valid for the whole rebuild, and the writer
// resolves the fragment against it.
//
// An operation carrying a GENERATOR contributes its generator's exact length
// here and its bytes at write time, so a value that exists in no buffer -- an
// AS_PATH re-encoded to the destination's ASN width, for instance -- is sized
// now and written once, straight into the destination. Nothing is staged.
//
// The generator's GenLen MUST be stable for the whole rebuild: it is read here,
// during the size query, and again as the write is checked. A generator that
// answers two different lengths refuses the plan rather than emitting an
// attribute whose declared length does not match its contents.
func (p *AttrPlan) Op(i int) {
	if p.closed {
		return
	}
	if i < 0 || i >= len(p.ops) || i > 255 {
		p.Fail()
		return
	}
	if g := p.set.generator(p.ops[i].GenIdx); g != nil {
		n := g.GenLen()
		if n < 0 {
			p.Fail()
			return
		}
		if n == 0 {
			return
		}
		p.appendFragment(fragOp, 0, n, uint8(i)) //nolint:gosec // G115: bounded by the i > 255 check above
		return
	}
	buf := p.ops[i].Buf
	if len(buf) == 0 {
		return
	}
	p.appendFragment(fragOp, 0, len(buf), uint8(i)) //nolint:gosec // G115: bounded by the i > 255 check above
}

// New copies bytes into the arena and appends a fragment over them.
//
// It is for bytes that exist nowhere else: a length field a rewrite recomputes,
// or a value a future transcode synthesizes. Bytes that are already in the
// source or in an operation buffer must use Keep or Op, so the arena stays a
// record of what the edit genuinely added.
func (p *AttrPlan) New(b []byte) {
	if p.closed {
		return
	}
	if len(b) == 0 {
		return
	}
	e := p.set
	e.ensure()
	start := len(e.arena)
	if start+len(b) > attrValueMax {
		p.Fail()
		return
	}
	before := cap(e.arena)
	e.arena = append(e.arena, b...)
	if cap(e.arena) != before {
		e.spilledArena = true
	}
	p.appendFragment(fragArena, start, len(b), 0)
}

// NewByte copies one synthesized byte into the arena.
//
// It exists so the commonest arena use -- a recomputed length field, such as the
// next-hop length an MP_REACH rewrite changes -- needs no slice literal on a
// per-destination path (ai/rules/performance.md).
func (p *AttrPlan) NewByte(b byte) {
	if p.closed {
		return
	}
	e := p.set
	e.ensure()
	start := len(e.arena)
	if start+1 > attrValueMax {
		p.Fail()
		return
	}
	before := cap(e.arena)
	e.arena = append(e.arena, b)
	if cap(e.arena) != before {
		e.spilledArena = true
	}
	p.appendFragment(fragArena, start, 1, 0)
}

// appendFragment records one fragment and adds its length to the running total,
// coalescing with the previous fragment when the two are adjacent in the same
// source. Coalescing is what makes a run of retained community values one copy
// rather than one copy per value.
func (p *AttrPlan) appendFragment(source fragmentSource, off, n int, opIndex uint8) {
	if off < 0 || n <= 0 || off > attrValueMax || n > attrValueMax || off+n > attrValueMax {
		p.Fail()
		return
	}
	if p.slot.outLen+uint32(n) > attrValueMax { //nolint:gosec // G115: n is bounded above
		// RFC 4271 Section 4.3 caps an attribute value at 65535 octets.
		p.Fail()
		return
	}

	e := p.set
	e.ensure()

	// Coalesce with the previous fragment of this slot when it is the same
	// source and ends exactly where this one starts. An operation fragment never
	// coalesces: two operations are two distinct buffers.
	if uint32(len(e.frags)) > p.slot.fragFrom && source != fragOp {
		last := &e.frags[len(e.frags)-1]
		if last.source == source && int(last.off)+int(last.n) == off &&
			int(last.n)+n <= attrValueMax {
			last.n += uint16(n) //nolint:gosec // G115: bounded by the attrValueMax check above
			p.slot.outLen += uint32(n)
			return
		}
	}

	before := cap(e.frags)
	e.frags = append(e.frags, fragment{
		off:     uint16(off), //nolint:gosec // G115: bounded by the attrValueMax check above
		n:       uint16(n),   //nolint:gosec // G115: bounded by the attrValueMax check above
		source:  source,
		opIndex: opIndex,
	})
	if cap(e.frags) != before {
		e.spilledFrags = true
	}
	p.slot.outLen += uint32(n) //nolint:gosec // G115: bounded above
}

// Emit finishes the plan: the attribute is written with these flags and this
// code, and the header size class is decided from the FINAL value length rather
// than the source's. RFC 4271 Section 4.3: a value above 255 octets needs the
// Extended Length header, and that is a one-byte change to the total which the
// size query must predict rather than discover.
func (p *AttrPlan) Emit(flags, code byte) {
	p.emit(flags, code, false)
}

// EmitExtended finishes the plan with the Extended Length header unconditionally.
//
// It exists because the community handlers have always written a 4-byte header
// whatever the value length. That is legal (RFC 4271 Section 4.3 permits the
// Extended Length bit on any attribute) and this method preserves it exactly,
// rather than silently normalizing a byte on the wire for every community
// attribute Ze forwards.
func (p *AttrPlan) EmitExtended(flags, code byte) {
	p.emit(flags, code, true)
}

func (p *AttrPlan) emit(flags, code byte, forceExtended bool) {
	if p.closed {
		return
	}
	if p.slot.outLen > attrValueMax {
		p.Fail()
		return
	}
	p.slot.flags = flags
	p.slot.code = code
	if forceExtended || p.slot.outLen > 255 {
		p.slot.flags |= 0x10
		p.slot.hdrLen = 4
	} else {
		p.slot.flags &^= 0x10
		p.slot.hdrLen = 3
	}
	p.slot.kind = slotEmit
	p.closed = true
}

// Drop finishes the plan with the attribute removed from the UPDATE.
func (p *AttrPlan) Drop() {
	if p.closed {
		return
	}
	p.slot.kind = slotDrop
	p.closed = true
}

// Fail refuses the plan. The caller suppresses the route for this destination:
// a route missing a modification the policy required must not go out, whatever
// the reason the handler could not produce it.
func (p *AttrPlan) Fail() {
	p.slot.kind = slotFail
	p.closed = true
}

// Attr begins planning one attribute and returns the reusable planner.
//
// valOff and valLen locate the source attribute's value within the attribute
// SECTION, and srcOff and srcLen locate the whole attribute; both are recorded
// as integers so the finished slot holds no pointer into the payload. src is the
// full source attribute bytes for the handler to read, or nil when the attribute
// is absent and the plan is creating it.
//
// opFrom and opTo bound this code's operations within the accumulator's own
// operation slice, which is what lets the writer resolve an operation fragment
// without the slot storing a slice header.
func (e *EditSet) Attr(code uint8, src []byte, srcOff, srcLen, valOff, valLen int, ops []AttrOp, opFrom int) *AttrPlan {
	e.ensure()
	p := &e.cur
	*p = AttrPlan{
		set:  e,
		src:  src,
		ops:  ops,
		code: code,
		nOps: len(ops),
	}
	if src != nil && valLen >= 0 && valOff >= 0 && valOff+valLen <= len(src)+valOff {
		// val is the value window inside the full source attribute. The header
		// length is the difference between the attribute start and its value.
		hdr := valOff - srcOff
		if hdr >= 0 && hdr+valLen <= len(src) {
			p.val = src[hdr : hdr+valLen]
		}
	}
	p.slot = editSlot{
		valOff:   uint32(max(valOff, 0)),            //nolint:gosec // G115: bounded by the caller's section parse
		valLen:   uint32(max(valLen, 0)),            //nolint:gosec // G115: same
		srcOff:   uint32(max(srcOff, 0)),            //nolint:gosec // G115: same
		srcLen:   uint32(max(srcLen, 0)),            //nolint:gosec // G115: same
		fragFrom: uint32(len(e.frags)),              //nolint:gosec // G115: bounded by attrValueMax growth checks
		opFrom:   uint32(max(opFrom, 0)),            //nolint:gosec // G115: bounded by the accumulator's op count
		opTo:     uint32(max(opFrom, 0) + len(ops)), //nolint:gosec // G115: same
		code:     code,
	}
	return p
}

// Commit records the finished plan as a slot and returns its identifier. A
// handler that returned without finishing its plan is treated as a refusal:
// silence is not consent when the outcome is a route on the wire.
func (e *EditSet) Commit(p *AttrPlan) SlotID {
	if !p.closed {
		p.slot.kind = slotFail
	}
	p.slot.fragTo = uint32(len(e.frags)) //nolint:gosec // G115: bounded by the fragment growth checks
	before := cap(e.slots)
	e.slots = append(e.slots, p.slot)
	if cap(e.slots) != before {
		e.spilledSlots = true
	}
	e.mark(p.slot.code)
	return SlotID(len(e.slots) - 1)
}

// CommitFailed records a refusal for a handler that could not be run at all: a
// code with operations but no registered handler, or one whose handler panicked
// before it could plan anything.
func (e *EditSet) CommitFailed(code uint8) SlotID {
	e.ensure()
	before := cap(e.slots)
	e.slots = append(e.slots, editSlot{code: code, kind: slotFail})
	if cap(e.slots) != before {
		e.spilledSlots = true
	}
	e.mark(code)
	return SlotID(len(e.slots) - 1)
}

func (e *EditSet) slot(id SlotID) *editSlot {
	if id < 0 || int(id) >= len(e.slots) {
		panic("BUG: EditSet slot id out of range")
	}
	return &e.slots[id]
}

// SlotFailed reports whether the handler refused. The caller suppresses.
func (e *EditSet) SlotFailed(id SlotID) bool { return e.slot(id).kind == slotFail }

// SlotVerbatim reports whether the slot emits its source attribute unchanged.
// The writer coalesces such a slot into the surrounding untouched run.
func (e *EditSet) SlotVerbatim(id SlotID) bool { return e.slot(id).kind == slotVerbatim }

// SlotDropped reports whether the slot emits nothing.
func (e *EditSet) SlotDropped(id SlotID) bool { return e.slot(id).kind == slotDrop }

// SlotCode returns the attribute type code the slot emits.
func (e *EditSet) SlotCode(id SlotID) uint8 { return e.slot(id).code }

// SlotSize returns the EXACT number of bytes SlotWrite will write: the header
// size class the plan chose, plus the sum of its fragment lengths. This is the
// size query the whole design exists to make possible, and it is not an estimate
// — it is the same arithmetic the writer replays.
func (e *EditSet) SlotSize(id SlotID) int {
	s := e.slot(id)
	switch s.kind {
	case slotVerbatim:
		return int(s.srcLen)
	case slotEmit:
		return int(s.hdrLen) + int(s.outLen)
	case slotDrop, slotFail:
		return 0
	}
	return 0
}

// SlotWrite materializes the slot into buf at off and returns the new offset.
//
// section is the SOURCE attribute section the slot's offsets are relative to,
// and ops is the accumulator's operation slice. Neither is retained. The return
// is always off+SlotSize(id) on success; a short buffer returns off unchanged so
// the caller can tell the write did not happen.
func (e *EditSet) SlotWrite(id SlotID, section []byte, ops []AttrOp, buf []byte, off int) int {
	s := e.slot(id)
	size := e.SlotSize(id)
	if size == 0 {
		return off
	}
	if off < 0 || off+size > len(buf) {
		return off
	}

	if s.kind == slotVerbatim {
		end := int(s.srcOff) + int(s.srcLen)
		if end > len(section) {
			return off
		}
		copy(buf[off:], section[s.srcOff:end])
		return off + size
	}

	// Header: flags, code, then a one- or two-octet length per the size class the
	// plan already decided (RFC 4271 Section 4.3).
	buf[off] = s.flags
	buf[off+1] = s.code
	w := off + 2
	if s.hdrLen == 4 {
		buf[w] = byte(s.outLen >> 8)
		buf[w+1] = byte(s.outLen)
		w += 2
	} else {
		buf[w] = byte(s.outLen)
		w++
	}

	for i := s.fragFrom; i < s.fragTo; i++ {
		f := e.frags[i]
		var src []byte
		switch f.source {
		case fragValue:
			start := int(s.valOff) + int(f.off)
			end := start + int(f.n)
			if end > len(section) {
				return off
			}
			src = section[start:end]
		case fragOp:
			oi := int(s.opFrom) + int(f.opIndex)
			if oi >= len(ops) || oi >= int(s.opTo) {
				return off
			}
			if g := e.generator(ops[oi].GenIdx); g != nil {
				// The generator writes into the destination directly, so there is
				// no source slice to copy from. Its length was folded into the
				// slot's size during planning; a generator that now writes a
				// different number of bytes refuses the write rather than leaving
				// an attribute whose header contradicts its contents.
				if w+int(f.n) > len(buf) {
					return off
				}
				if g.GenWrite(buf, w) != int(f.n) {
					return off
				}
				w += int(f.n)
				continue
			}
			b := ops[oi].Buf
			end := int(f.off) + int(f.n)
			if end > len(b) {
				return off
			}
			src = b[f.off:end]
		case fragArena:
			end := int(f.off) + int(f.n)
			if end > len(e.arena) {
				return off
			}
			src = e.arena[f.off:end]
		}
		copy(buf[w:], src)
		w += len(src)
	}

	if w != off+size {
		// The size query and the write disagreed. That is the one invariant this
		// design exists to guarantee, so it fails loudly rather than emitting a
		// short attribute that still parses.
		return off
	}
	return w
}
