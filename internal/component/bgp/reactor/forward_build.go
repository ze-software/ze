// Design: docs/architecture/core-design.md — exactly-sized one-pass rebuild for egress attribute modification
// Design: docs/architecture/buffer-architecture.md -- zero-copy, copy-on-modify (Outgoing Peer Pool is the copy point)
// RFC: rfc/short/rfc4271.md — UPDATE body layout, Total Path Attribute Length, attribute ordering (Section 5)
// RFC: rfc/short/rfc8654.md — extended message body ceiling
// RFC: rfc/short/rfc9494.md — announce-to-withdrawal conversion for non-LLGR EBGP peers
// Overview: reactor_api_forward.go — UPDATE forwarding dispatch
// Detail: forward_modify_failure.go — why a modification could not be applied
// Related: reactor_notify.go — panic recovery helpers

package reactor

import (
	"encoding/binary"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// maxUpdateBody is the largest UPDATE body Ze will build.
//
// RFC 8654 raises the message ceiling to 65535 octets, of which 19 are the fixed
// header, so a body cannot exceed 65516. An edit whose exact size lands above
// this cannot be sent to any peer under any negotiated size, so it is refused
// here rather than handed to a session that would refuse it with less context.
const maxUpdateBody = 65516

// modBufPool provides reusable buffers for the progressive build.
// Standard UPDATE max body is 4096 - 19 = 4077 bytes.
// Extended messages can reach 65535 - 19 bytes, but these are rare
// and the pool buffer will be replaced with a larger allocation if needed.
var modBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 4096)
		return &buf
	},
}

// spanIndexPool supplies the reusable attribute index each rebuild needs.
//
// A pool rather than a field on the accumulator, because buildModifiedPayload is
// reached from five call sites with different owners and only some of them hoist
// an accumulator above a destination loop. A pool gives every one of them the
// same allocation-free behavior without changing a signature that a reserved
// file also calls.
var spanIndexPool = sync.Pool{
	New: func() any { return new(attribute.SpanIndex) },
}

// advertiseGate answers, at most once per rebuild, whether the body this rebuild
// WRITES carries reachable NLRI. planAttr reads it to decide whether an
// attribute that is absent from the source may be CREATED.
//
// RFC 4271 Section 4.3 describes the shape it protects: "An UPDATE message might
// advertise only routes that are to be withdrawn from service, in which case the
// message will not include path attributes or Network Layer Reachability
// Information." Section 6.3 makes an incomplete well-known set a wire error: "If
// any of the well-known mandatory attributes are not present, then the Error
// Subcode MUST be set to Missing Well-known Attribute." An egress rule that adds
// one attribute to an UPDATE advertising nothing produces exactly that set, and
// FRR 10.3.1 answers it with "Missing well-known attribute <TYPE>" followed by
// "rcvd UPDATE with errors in attr(s)!! Withdrawing route", so the withdrawal
// never takes effect at the peer. It names whichever well-known mandatory
// attribute it misses first: measured AS_PATH on 2026-08-04, for a withdrawal
// carrying a lone NEXT_HOP (test/interop/scenarios/bgp-relay-withdraw-nexthop-self-frr).
//
// The verdict is lazy because it is needed only when a plan would create
// something: a rebuild that merely rewrites attributes already on the wire never
// asks. It is also answered from values buildModifiedPayload already computed,
// so it costs no walk of its own: attrEnd comes from the header parse, and the
// MP_REACH question from the span index that the same function just built.
//
// wireu.PayloadAdvertisesNLRI is the definition of this question, and
// TestAdvertiseGateAgreesWithPayloadAdvertisesNLRI pins the two together over
// every shape rather than leaving the agreement to a comment. The predicate
// itself stays the one definition for callers that hold only a payload
// (ASPathEdit.Record, the RFC 9234 OTC gate).
type advertiseGate struct {
	payload      []byte
	attrEnd      int
	nlriOverride []byte
	spans        *attribute.SpanIndex
	known        bool
	value        bool
}

// advertises reports whether the body this rebuild WRITES carries reachable
// NLRI.
//
// The legacy NLRI section it asks about is the one being written, which is the
// override when the caller supplies one. A filter's `nlri ipv4/unicast add`
// block is never proven a subset of the source (extractLegacyNLRIOverride,
// filter_delta.go), so BOTH directions matter: an override can empty a body that
// advertised, and it can fill one that did not.
func (g *advertiseGate) advertises() bool {
	if g.known {
		return g.value
	}
	g.known = true

	if g.nlriOverride != nil {
		// The rebuild REPLACES the legacy NLRI section, so the source's own
		// prefixes say nothing about the body being written.
		if len(g.nlriOverride) > 0 {
			g.value = true
			return g.value
		}
		// Every legacy prefix dropped. Only MP_REACH_NLRI can still advertise.
		_, g.value = g.spans.Find(attribute.AttrMPReachNLRI)
		return g.value
	}

	// RFC 4271 Section 4.3: the NLRI field is whatever trails the attributes.
	if len(g.payload) > g.attrEnd {
		g.value = true
		return g.value
	}
	// No native NLRI, but an advertisement can still ride in MP_REACH_NLRI
	// (RFC 4760 Section 3).
	_, g.value = g.spans.Find(attribute.AttrMPReachNLRI)
	return g.value
}

// buildModifiedPayload applies attribute modifications to a source UPDATE
// payload with a single exactly-sized merge walk into a pooled buffer.
//
// The source payload has the standard UPDATE structure:
//
//	withdrawn_len(2) + withdrawn + attr_len(2) + attrs + nlri
//
// Three things happen, in order.
//
//  1. PLAN. The attribute section is indexed once (attribute.BuildSpanIndex),
//     the operations are grouped by code, and each touched code's registered
//     handler describes its output as a fragment list. No byte is written and no
//     intermediate value is built: a handler that keeps most of an attribute
//     says so with fragments over the source, so the MP_REACH NLRI tail and a
//     retained run of community values are named rather than copied.
//  2. SIZE. The plan is walked in emission order and its bytes are counted.
//     Because the count and the write replay the same walk, the size is exact
//     rather than an upper bound. That removes the old len(payload)+256 slack
//     and, with it, the branch that abandoned every modification on overflow and
//     forwarded the route unchanged (ai/rules/evidence.md).
//  3. WRITE. A buffer of exactly that size is acquired and the same walk writes
//     it. Adjacent untouched attributes coalesce into one copy, and a new
//     attribute is merge-inserted at its ascending type-code position rather
//     than appended after every source attribute.
//
// RFC 4271 Section 5 orders path attributes by ascending type code on emission.
// Merge-insert is what makes the forward path agree with the announce rails,
// which were already ascending. An UPDATE that gains NO new attribute keeps base
// order unconditionally, so a pure forward stays byte-identical and the
// zero-copy identity survives.
//
// Copy-on-modify: when pp is non-nil and has a free buffer large enough for
// the payload, the modified data is written directly into the per-peer pool
// buffer. The caller stores the returned peerBufIdx in fwdItem so releaseItem
// returns it after the worker writes to TCP. When no per-peer buffer is
// available, falls back to sync.Pool + a result copy.
//
// nlriOverride: when non-nil, the function replaces the legacy NLRI section
// (payload[attrEnd:]) with the provided bytes. A zero-length (but non-nil)
// slice means "drop every legacy NLRI prefix"; callers use this for the
// per-prefix filter modify path when every prefix in the UPDATE was denied
// but attributes remained intact. A nil slice preserves the original NLRI
// copy. nlriOverride affects ONLY the legacy IPv4 NLRI section; MP_REACH /
// MP_UNREACH rewriting is out of scope for this function (filter plugins
// that need per-NLRI decisions on non-CIDR families must declare raw=true
// and return a full payload rewrite themselves).
//
// Returns (modified payload, peerBufIdx, failure). peerBufIdx > 0 means the
// returned slice is backed by pp and MUST be returned via pp.Return(peerBufIdx).
// peerBufIdx == 0 means the slice is independently allocated (safe to retain).
//
// A nil payload has TWO meanings and the caller MUST tell them apart by the
// third value (ai/rules/evidence.md):
//
//   - (nil, 0, modifyFailureNone): no modifications were needed. Forward the
//     route as it stands.
//   - (nil, 0, anything else): the modifications were needed and could NOT be
//     applied. The caller MUST suppress the route for this destination.
//     Forwarding it would send a route the policy was supposed to change.
//
// Every failure path RETURNS, so the compiler forces a reason to be named. The
// keep-building failures the previous shape needed -- a missing handler, a
// faulting handler, a truncated section -- are now decided during the plan,
// before any buffer exists, so there is no half-built payload to reason about
// and no `unapplied` flag a future edit could forget to set.
func buildModifiedPayload(
	payload []byte,
	mods *filterapi.ModAccumulator,
	handlers map[uint8]filterapi.AttrModHandler,
	pp *peerPool,
	nlriOverride []byte,
) ([]byte, int, modifyFailure) {
	// The ModAccumulator can also carry per-peer NLRI rewrites. An explicit
	// nlriOverride argument (the legacy per-prefix modify path) takes precedence;
	// otherwise the accumulator's announce-NLRI rewrite applies. The withdrawn
	// rewrite has no legacy argument, so it comes only from the accumulator.
	if nlriOverride == nil {
		nlriOverride = mods.NLRIRewrite()
	}
	withdrawnOverride := mods.WithdrawnRewrite()
	if mods.Len() == 0 && nlriOverride == nil && withdrawnOverride == nil {
		// A legitimate nil: there was nothing to apply. The advertise gate below
		// produces the same verdict for a second reason; both mean "forward the
		// route as it stands", and neither means a modification was lost.
		return nil, 0, modifyFailureNone
	}

	// Parse source payload structure. From here on every nil return is a
	// FAILURE: modifications were required and could not be applied.
	if len(payload) < 4 {
		fwdLogger().Warn("malformed payload in buildModifiedPayload, suppressing route", "payloadLen", len(payload))
		return nil, 0, modifyFailureMalformed
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrOffset := 2 + withdrawnLen
	if len(payload) < attrOffset+2 {
		fwdLogger().Warn("malformed payload in buildModifiedPayload, suppressing route", "payloadLen", len(payload))
		return nil, 0, modifyFailureMalformed
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrOffset : attrOffset+2]))
	attrStart := attrOffset + 2
	attrEnd := attrStart + attrLen
	if len(payload) < attrEnd {
		fwdLogger().Warn("malformed payload in buildModifiedPayload, suppressing route", "payloadLen", len(payload))
		return nil, 0, modifyFailureMalformed
	}

	section := payload[attrStart:attrEnd]

	// Index the attribute section once. This is the same builder and the same
	// verdicts the receive path publishes on the immutable base (RFC 4271
	// Section 4.3: a header that does not parse, a duplicate type code, and an
	// attribute running past the section end are each malformed).
	//
	// Failing closed here is a deliberate tightening. The old walk tolerated a
	// duplicate type code and applied the operations to BOTH copies, which emits
	// an UPDATE a conforming peer rejects, and it reported truncation only after
	// having already copied part of the section.
	// The index is REUSED rather than produced by value. Returning it by value and
	// taking its address forces it to the heap once per destination as soon as
	// anything in the plan makes an indirect call the escape analysis cannot see
	// through -- which is exactly what a value generator is. Reusing a pooled
	// index keeps the rebuild allocation-free whatever the plan contains, and it
	// is what TestModifyPathZeroAlloc measures.
	spans, _ := spanIndexPool.Get().(*attribute.SpanIndex)
	defer spanIndexPool.Put(spans)
	err := spans.Rebuild(section)
	if err != nil {
		fwdLogger().Warn("attribute section does not index, suppressing route",
			"payloadLen", len(payload), "attrLen", attrLen, "error", err)
		return nil, 0, modifyFailureTruncated
	}

	// Group the operations by code: one in-place stable sort, no allocation, and
	// no 256-entry array returned by value.
	ops := mods.GroupedOps()
	edit := mods.EditSet()
	edit.Begin()
	// Begin cleared the reference, so the generators recorded while this
	// destination's operations were accumulated are handed back now.
	edit.SetGens(mods.Gens())

	// The one gate that stops any handler creating an attribute on a body that
	// advertises nothing. It is read from planAttr, the single place a handler
	// runs, so a producer recording the operation cannot forget it and a plugin
	// handler in another package inherits it.
	gate := advertiseGate{payload: payload, attrEnd: attrEnd, nlriOverride: nlriOverride, spans: spans}

	// PLAN. One slot per touched code, in ascending code order because the
	// operations are now sorted that way.
	planned := false
	for from := 0; from < len(ops); {
		code := ops[from].Code
		to := from + 1
		for to < len(ops) && ops[to].Code == code {
			to++
		}
		did, fail := planAttr(edit, spans, section, ops[from:to], from, code, handlers, &gate)
		if fail.failed() {
			return nil, 0, fail
		}
		planned = planned || did
		from = to
	}

	// Every operation was refused by the gate above, so the rebuild would copy
	// the source byte for byte. This is the SECOND producer of the nil that means
	// "nothing to apply" -- the first is the empty accumulator above -- and it
	// keeps a relayed withdrawal on the zero-copy forward path instead of
	// charging it a per-destination copy for a change that did not happen.
	// Unreachable when any code planned, because planAttr either commits a slot
	// or returns a failure.
	if !planned && nlriOverride == nil && withdrawnOverride == nil {
		return nil, 0, modifyFailureNone
	}

	// A store that outgrew its inline capacity allocated. That is correct and is
	// never refused, but the capacities come from a static census rather than a
	// traffic histogram, so the rate is counted instead of assumed.
	if slotSpill, fragSpill, arenaSpill := edit.Spilled(); slotSpill || fragSpill || arenaSpill {
		if slotSpill {
			filterapi.RecordEditSpill(filterapi.EditStoreSlots)
		}
		if fragSpill {
			filterapi.RecordEditSpill(filterapi.EditStoreFragments)
		}
		if arenaSpill {
			filterapi.RecordEditSpill(filterapi.EditStoreArena)
		}
	}

	// SIZE. The same walk that will write, counting instead.
	emitter := attrEmitter{edit: edit, spans: spans, section: section, ops: ops}
	attrBytes, fail := emitter.run(nil, 0)
	if fail.failed() {
		return nil, 0, fail
	}

	// RFC 4271 Section 4.3: Total Path Attribute Length is a 2-octet field.
	if attrBytes > 65535 {
		fwdLogger().Warn("modified attribute section does not fit the 2-octet length field, suppressing route",
			"newAttrLen", attrBytes, "max", 65535, "attrCount", spans.Len())
		return nil, 0, modifyFailureAttrLenRange
	}

	// The withdrawn section: the accumulator's rewrite, or the original.
	wdBytes := 2 + withdrawnLen
	if withdrawnOverride != nil {
		if len(withdrawnOverride) > 65535 {
			fwdLogger().Warn("withdrawn rewrite exceeds the 2-octet length field, suppressing route",
				"len", len(withdrawnOverride), "max", 65535)
			return nil, 0, modifyFailureWithdrawnSize
		}
		wdBytes = 2 + len(withdrawnOverride)
	}

	// The NLRI section: the override, or the original tail.
	nlriBytes := len(payload) - attrEnd
	if nlriOverride != nil {
		nlriBytes = len(nlriOverride)
	}

	// The EXACT output size. Nothing below this line estimates.
	needSize := wdBytes + 2 + attrBytes + nlriBytes
	if needSize > maxUpdateBody {
		// RFC 8654 bounds the body at 65516 octets, so an edit above it cannot
		// be sent to any peer under any negotiated size. The route is suppressed
		// for this destination and says so, rather than going out unmodified
		// carrying exactly what the policy exists to strip.
		fwdLogger().Warn("modified UPDATE body exceeds the message ceiling, suppressing route",
			"bodyLen", needSize, "max", maxUpdateBody, "attrCount", spans.Len())
		return nil, 0, modifyFailureOverflow
	}

	buf, peerBufIdx, poolBuf := acquireModBuf(pp, needSize)

	// Ensure buffers are returned on panic and on every failure below.
	defer func() {
		if peerBufIdx > 0 && pp != nil {
			pp.Return(peerBufIdx)
			peerBufIdx = 0
		}
		if poolBuf != nil {
			modBufPool.Put(poolBuf)
			poolBuf = nil
		}
	}()

	off := 0

	// Step 1: the withdrawn section. When withdrawnOverride is non-nil the egress
	// filter has rewritten the withdrawn NLRI (per-peer prefix translation on
	// withdrawal, keeping adj-rib-out consistent): write a fresh 2-byte length
	// plus the override bytes. A zero-length (non-nil) override drops every
	// withdrawn prefix. A nil override copies the original withdrawn section.
	if withdrawnOverride != nil {
		binary.BigEndian.PutUint16(buf[off:], uint16(len(withdrawnOverride))) //nolint:gosec // G115: bounded by the check above
		off += 2
		off += copy(buf[off:], withdrawnOverride)
	} else {
		off += copy(buf[off:], payload[:wdBytes])
	}

	// Step 2: the attribute section length, known exactly before it is written.
	// There is no backfill: the size query already produced this number, and the
	// write below is checked against it.
	binary.BigEndian.PutUint16(buf[off:], uint16(attrBytes)) //nolint:gosec // G115: bounded by the 65535 check above
	off += 2

	// Step 3: the attributes, by the same walk that sized them.
	written, fail := emitter.run(buf, off)
	if fail.failed() {
		return nil, 0, fail
	}
	if written != attrBytes {
		// The size query and the write disagreed. That is the invariant this
		// whole design exists to guarantee, so it suppresses rather than emitting
		// a section whose declared length does not match its contents.
		fwdLogger().Warn("attribute size query disagreed with the write, suppressing route",
			"sized", attrBytes, "written", written)
		return nil, 0, modifyFailureOverflow
	}
	off += written

	// Step 4: the NLRI section. When nlriOverride is non-nil the filter chain
	// has rewritten the legacy IPv4 NLRI (per-prefix modify path); an override
	// of length zero explicitly drops every legacy NLRI prefix.
	if nlriOverride != nil {
		off += copy(buf[off:], nlriOverride)
	} else {
		off += copy(buf[off:], payload[attrEnd:])
	}

	if off != needSize {
		fwdLogger().Warn("rebuilt body size disagreed with the size query, suppressing route",
			"sized", needSize, "written", off)
		return nil, 0, modifyFailureOverflow
	}

	// Per-peer buffer path: return the slice directly. The caller stores
	// peerBufIdx in fwdItem so releaseItem returns it after TCP write.
	// No copy needed -- the buffer IS the result.
	if peerBufIdx > 0 {
		if poolBuf != nil {
			modBufPool.Put(poolBuf)
			poolBuf = nil
		}
		bufIdx := peerBufIdx
		peerBufIdx = 0 // prevent defer from double-returning
		// Every reason a modification could not be applied returned above, before
		// a buffer was acquired, so reaching here means the plan landed in full.
		return buf[:off], bufIdx, modifyFailureNone
	}

	// Sync.Pool fallback: copy result so pool buffer can be returned.
	result := make([]byte, off) // pool-fallback
	copy(result, buf[:off])

	return result, 0, modifyFailureNone
}

// planAttr runs one code's handler under panic recovery and commits its slot.
//
// It is the only place a handler is called, so every reason a handler's
// operations might not land is decided here, before any buffer exists. A
// half-built payload is therefore not a state this function can produce.
//
// It reports whether it planned anything. False means the gate refused the code
// outright, which is not a failure: the operations asked to CREATE an attribute
// on a body that advertises nothing (see advertiseGate).
func planAttr(
	edit *filterapi.EditSet,
	spans *attribute.SpanIndex,
	section []byte,
	codeOps []filterapi.AttrOp,
	opFrom int,
	code uint8,
	handlers map[uint8]filterapi.AttrModHandler,
	gate *advertiseGate,
) (bool, modifyFailure) {
	var src []byte
	srcOff, srcLen, valOff, valLen := 0, 0, 0, 0
	if span, ok := spans.Find(attribute.AttributeCode(code)); ok {
		valOff = int(span.Offset)
		valLen = int(span.Length)
		srcOff = valOff - int(span.HdrLen)
		srcLen = int(span.HdrLen) + valLen
		src = section[srcOff : srcOff+srcLen]
	}

	// RFC 4271 Section 4.3 and Section 6.3: no rule may CREATE a path attribute
	// on an UPDATE that advertises no reachable NLRI -- a withdrawal, or an RFC
	// 4724 Section 2 End-of-RIB marker, both of which stop being what they are
	// the moment one attribute is stamped on them. Rewriting an attribute the
	// source already carried stays allowed: presence is what the receiver's
	// well-known-mandatory check reads, and re-encoding an AS_PATH that rode
	// along at the destination's AS width is still owed (RFC 6793 Section 4.2.2).
	//
	// This is decided BEFORE the handler lookup so an unregistered code cannot
	// fail the route closed over an attribute that must not exist anyway.
	if src == nil && !gate.advertises() {
		return false, modifyFailureNone
	}

	handler := handlers[code]
	if handler == nil {
		// The operations for this code are NOT applied. Copying the source
		// through and reporting success forwards the route carrying the very
		// attribute the policy meant to change, and for RFC 9234 OTC it emits a
		// route missing an attribute a MUST requires.
		fwdLogger().Warn("no attr mod handler registered, suppressing route", "code", code)
		edit.CommitFailed(code)
		return false, modifyFailureNoHandler
	}

	p := edit.Attr(code, src, srcOff, srcLen, valOff, valLen, codeOps, opFrom)
	if panicked := safeAttrModHandler(handler, code, p); panicked {
		edit.CommitFailed(code)
		return false, modifyFailureHandlerFault
	}
	id := edit.Commit(p)
	if edit.SlotFailed(id) {
		fwdLogger().Warn("attr mod handler refused the modification, suppressing route", "code", code)
		return false, modifyFailureHandlerFault
	}
	return true, modifyFailureNone
}

// attrEmitter walks the output attribute order exactly once per call.
//
// It is called twice per rebuild: with a nil buffer to SIZE, and with the
// acquired buffer to WRITE. Both calls execute the same statements in the same
// order, which is what makes the size exact rather than an upper bound. An edit
// that changes the order changes it for both, so the two cannot drift apart.
type attrEmitter struct {
	edit    *filterapi.EditSet
	spans   *attribute.SpanIndex
	section []byte
	ops     []filterapi.AttrOp

	buf []byte // nil on the sizing pass
	off int    // absolute write offset; ignored on the sizing pass
	n   int    // bytes emitted so far

	runOff int // pending verbatim run: offset into section
	runLen int
}

// keep extends the pending verbatim run, or starts a new one when this
// attribute does not continue the last. Coalescing is why a stretch of
// untouched attributes costs one copy rather than one copy each, and it is free
// because the span index already knows where each attribute begins and ends.
func (e *attrEmitter) keep(off, length int) {
	if e.runLen > 0 && e.runOff+e.runLen == off {
		e.runLen += length
		return
	}
	e.flush()
	e.runOff = off
	e.runLen = length
}

// flush emits the pending verbatim run.
func (e *attrEmitter) flush() {
	if e.runLen == 0 {
		return
	}
	if e.buf != nil {
		copy(e.buf[e.off+e.n:], e.section[e.runOff:e.runOff+e.runLen])
	}
	e.n += e.runLen
	e.runLen = 0
}

// emitSlot emits one planned attribute.
func (e *attrEmitter) emitSlot(id filterapi.SlotID) modifyFailure {
	size := e.edit.SlotSize(id)
	if size == 0 {
		return modifyFailureNone
	}
	if e.buf == nil {
		e.n += size
		return modifyFailureNone
	}
	at := e.off + e.n
	got := e.edit.SlotWrite(id, e.section, e.ops, e.buf, at)
	if got != at+size {
		fwdLogger().Warn("attr mod slot did not write its planned size, suppressing route",
			"code", e.edit.SlotCode(id), "planned", size, "written", got-at)
		return modifyFailureOverflow
	}
	e.n += size
	return modifyFailureNone
}

// nextNew returns the next planned attribute that has no source span, which is
// an attribute being ADDED and therefore one waiting for its insertion point.
// The slots are already in ascending code order, because the operations were
// sorted by code before planning.
func (e *attrEmitter) nextNew(from int) (filterapi.SlotID, uint8, int, bool) {
	for i := from; i < e.edit.SlotCount(); i++ {
		id := e.edit.SlotAt(i)
		code := e.edit.SlotCode(id)
		if _, present := e.spans.Find(attribute.AttributeCode(code)); present {
			continue
		}
		return id, code, i, true
	}
	return 0, 0, e.edit.SlotCount(), false
}

// run walks the emission order and returns the number of bytes emitted.
//
// Base attributes keep their wire order, and a planned attribute that HAS a
// source is emitted in that source's position, so an edit which only changes
// values never reorders anything and a pure forward stays byte-identical.
//
// A planned attribute with NO source is merge-inserted at the first position
// whose code sorts after it. RFC 4271 Section 5 describes attributes as ordered
// by ascending type code on emission, which is what both announce rails already
// emit; appending after every source attribute, as the previous build did, let
// one route reach the wire in two different byte orders depending on which path
// built it.
func (e *attrEmitter) run(buf []byte, off int) (int, modifyFailure) {
	e.buf = buf
	e.off = off
	e.n = 0
	e.runLen = 0
	e.runOff = 0

	newAt := 0
	for i := range e.spans.Len() {
		span := e.spans.At(i)
		baseCode := uint8(span.Code)

		// Merge-insert every pending new attribute that sorts before this one.
		for {
			id, code, at, ok := e.nextNew(newAt)
			if !ok || code >= baseCode {
				break
			}
			e.flush()
			if fail := e.emitSlot(id); fail.failed() {
				return 0, fail
			}
			newAt = at + 1
		}

		attrOff := int(span.Offset) - int(span.HdrLen)
		attrLen := int(span.HdrLen) + int(span.Length)

		id, planned := e.edit.Find(baseCode)
		if !planned || e.edit.SlotVerbatim(id) {
			// Untouched, or a handler that asked for the source unchanged. Both
			// coalesce into the same copy run: "leave this attribute alone" and
			// "nothing asked to change it" are the same bytes.
			e.keep(attrOff, attrLen)
			continue
		}
		e.flush()
		if fail := e.emitSlot(id); fail.failed() {
			return 0, fail
		}
	}

	e.flush()

	// Anything still pending sorts after every base attribute.
	for {
		id, _, at, ok := e.nextNew(newAt)
		if !ok {
			break
		}
		if fail := e.emitSlot(id); fail.failed() {
			return 0, fail
		}
		newAt = at + 1
	}

	return e.n, modifyFailureNone
}

// buildWithdrawalPayload converts an announce UPDATE payload to a withdrawal.
// RFC 9494: EBGP non-LLGR peers must receive a withdrawal for stale routes.
//
// For IPv4 unicast (legacy NLRI in payload tail):
//
//	Move NLRI bytes to Withdrawn Routes, set attr_len=0.
//	Result: withdrawn_len(2) + nlri_bytes + attr_len(2)=0
//
// For other families (MP_REACH_NLRI attr 14):
//
//	Extract AFI/SAFI + NLRI from MP_REACH, build MP_UNREACH_NLRI (attr 15).
//	Result: withdrawn_len(2)=0 + attr_len(2) + mp_unreach_attr
//
// Copy-on-modify: when pp is non-nil and has a free buffer, the result is
// written directly into the per-peer pool buffer. The caller stores the
// returned index in fwdItem so releaseItem returns it after the worker
// writes to TCP. When no per-peer buffer is available, falls back to
// sync.Pool + a result copy, matching buildModifiedPayload's shape.
//
// Returns (nil, 0) if the payload cannot be converted (malformed or
// unsupported).
func buildWithdrawalPayload(payload []byte, pp *peerPool) ([]byte, int) {
	if len(payload) < 4 {
		return nil, 0
	}
	withdrawnLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrOffset := 2 + withdrawnLen
	if len(payload) < attrOffset+2 {
		return nil, 0
	}
	attrLen := int(binary.BigEndian.Uint16(payload[attrOffset : attrOffset+2]))
	attrStart := attrOffset + 2
	attrEnd := attrStart + attrLen
	if len(payload) < attrEnd {
		return nil, 0
	}

	// Acquire a buffer sized to the worst-case withdrawal (<= len(payload)
	// since a withdrawal is strictly shorter than the source announce).
	needSize := len(payload) + 4 // slack for headers
	buf, peerBufIdx, poolBuf := acquireModBuf(pp, needSize)
	defer func() {
		if poolBuf != nil {
			modBufPool.Put(poolBuf)
		}
	}()

	nlriBytes := payload[attrEnd:]
	var n int
	if len(nlriBytes) > 0 {
		// IPv4 unicast: move NLRI to withdrawn routes, no attributes.
		n = writeIPv4Withdrawal(buf, nlriBytes)
	} else {
		// No legacy NLRI: look for MP_REACH_NLRI (attr code 14) to convert.
		n = writeMPUnreachFromReach(buf, payload[attrStart:attrEnd])
	}

	if n == 0 {
		if peerBufIdx > 0 && pp != nil {
			pp.Return(peerBufIdx)
		}
		return nil, 0
	}

	if peerBufIdx > 0 {
		return buf[:n], peerBufIdx
	}
	// Sync.Pool fallback: copy result so pool buffer can be returned.
	result := make([]byte, n) // pool-fallback
	copy(result, buf[:n])
	return result, 0
}

// acquireModBuf returns a buffer sized for modification output. Prefers
// the per-peer pool (zero-copy on hit); falls back to modBufPool for
// small payloads; falls back to a fresh make only for oversized payloads
// when both pools are unavailable. Returns (buf, peerBufIdx, poolBufPtr).
// peerBufIdx > 0 means pp owns the buffer. poolBufPtr != nil means the
// caller MUST return it to modBufPool after use.
func acquireModBuf(pp *peerPool, needSize int) ([]byte, int, *[]byte) {
	if pp != nil {
		b, idx := pp.Get()
		if idx > 0 && len(b) >= needSize {
			return b, idx, nil
		} else if idx > 0 {
			pp.Return(idx)
		}
	}
	if needSize <= 4096 {
		if poolBuf, ok := modBufPool.Get().(*[]byte); ok {
			return *poolBuf, 0, poolBuf
		}
		return make([]byte, 4096), 0, nil
	}
	return make([]byte, needSize), 0, nil
}

// writeIPv4Withdrawal writes an IPv4 withdrawal (withdrawn_len + NLRI +
// attr_len=0) into buf and returns the bytes written. Returns 0 if buf
// is too small or nlri exceeds the uint16 withdrawn_len ceiling.
func writeIPv4Withdrawal(buf, nlri []byte) int {
	need := 2 + len(nlri) + 2
	if len(buf) < need || len(nlri) > 65535 {
		return 0
	}
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(nlri)))
	copy(buf[2:], nlri)
	buf[2+len(nlri)] = 0
	buf[2+len(nlri)+1] = 0
	return need
}

// writeMPUnreachFromReach extracts AFI/SAFI + NLRI from MP_REACH_NLRI
// (attr 14) and writes an UPDATE body with MP_UNREACH_NLRI (attr 15)
// directly into buf. Returns bytes written, or 0 if no MP_REACH was
// found / the payload is malformed / buf is too small.
//
// MP_REACH_NLRI value: AFI(2) + SAFI(1) + NH_Len(1) + NH(var) + Reserved(1) + NLRI(var).
// MP_UNREACH_NLRI value: AFI(2) + SAFI(1) + NLRI(var).
func writeMPUnreachFromReach(buf, attrs []byte) int {
	off := 0
	for off < len(attrs) {
		if off+2 > len(attrs) {
			return 0
		}
		flags := attrs[off]
		code := attrs[off+1]
		var hdrLen int
		var aLen uint16
		if flags&0x10 != 0 { // Extended length.
			if off+4 > len(attrs) {
				return 0
			}
			aLen = binary.BigEndian.Uint16(attrs[off+2 : off+4])
			hdrLen = 4
		} else {
			if off+3 > len(attrs) {
				return 0
			}
			aLen = uint16(attrs[off+2])
			hdrLen = 3
		}
		valStart := off + hdrLen
		valEnd := valStart + int(aLen)
		if valEnd > len(attrs) {
			return 0
		}

		if code != 14 { // not MP_REACH_NLRI
			off = valEnd
			continue
		}

		val := attrs[valStart:valEnd]
		if len(val) < 4 { // AFI(2) + SAFI(1) + NH_Len(1) minimum
			return 0
		}
		nhLen := int(val[3])
		nlriStart := 4 + nhLen + 1 // skip NH + reserved byte
		if nlriStart > len(val) {
			return 0
		}
		nlriData := val[nlriStart:]

		// Compute size of MP_UNREACH attribute value (AFI+SAFI+NLRI).
		unreachValLen := 3 + len(nlriData)
		if unreachValLen > 65535 {
			return 0
		}

		// Attribute header size: 3 (short) or 4 (extended) bytes.
		var attrHdrLen int
		var attrFlags byte
		if unreachValLen > 255 {
			attrFlags = 0x90 // Optional, Transitive, Extended Length.
			attrHdrLen = 4
		} else {
			attrFlags = 0x80 // Optional, Transitive.
			attrHdrLen = 3
		}
		attrTotalLen := attrHdrLen + unreachValLen

		// Total wire body: withdrawn_len(2) + attr_len(2) + attr.
		need := 4 + attrTotalLen
		if len(buf) < need {
			return 0
		}

		// withdrawn_len = 0.
		buf[0] = 0
		buf[1] = 0
		// attr_len covers only the attribute (header + value).
		binary.BigEndian.PutUint16(buf[2:4], uint16(attrTotalLen)) //nolint:gosec // G115: bounded by uint16 check
		// MP_UNREACH header.
		w := 4
		buf[w] = attrFlags
		buf[w+1] = 15 // MP_UNREACH_NLRI
		if attrFlags == 0x90 {
			binary.BigEndian.PutUint16(buf[w+2:w+4], uint16(unreachValLen)) //nolint:gosec // G115: bounded above
			w += 4
		} else {
			buf[w+2] = byte(unreachValLen)
			w += 3
		}
		// MP_UNREACH value: AFI(2) + SAFI(1) + NLRI.
		copy(buf[w:w+2], val[0:2])
		buf[w+2] = val[2]
		copy(buf[w+3:], nlriData)
		return need
	}

	return 0 // No MP_REACH_NLRI found.
}

// safeAttrModHandler runs an AttrModHandler with panic recovery.
//
// A handler now PLANS rather than writes, so a panic can no longer leave half an
// attribute in an output buffer: at this point there is no output buffer. The
// recovery marks the plan refused and the caller suppresses the route, rather
// than forwarding it carrying the attribute the handler was meant to change.
func safeAttrModHandler(handler filterapi.AttrModHandler, code uint8, p *filterapi.AttrPlan) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			fwdLogger().Error("attr mod handler panic, suppressing route",
				"code", code, "panic", r)
			panicked = true
			p.Fail()
		}
	}()
	handler(p)
	return false
}
