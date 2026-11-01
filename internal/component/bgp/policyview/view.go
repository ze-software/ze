// Design: docs/architecture/core-design.md -- Policy Filter Chain, "The BGP route view"
// Related: readers.go -- the per-field readers over the wire bytes
// Related: register.go -- the reader registrations, one per BGP field
//
// Package policyview is the lazy BGP route view a policy chain member reads.
// It sits inside the BGP engine, over wireu.WireUpdate and
// attribute.AttributesWire, so the route-policy library
// (internal/component/policy) and ribpolicy learn no BGP type: the field names
// stay declared in the library, and the readers for the BGP ones register here,
// from the BGP side (spec-filter-wire-0-umbrella, D-7).
//
// A field decodes on its first read, from the received bytes, and is cached
// for the rest of the run. Its value borrows those bytes: it is valid only
// during the synchronous chain run that holds the view, because a received
// UPDATE points into a reusable read buffer (ai/doors/bgp-engine.md, "Received
// bytes are borrowed"). A malformed attribute answers an error and never a
// zero value, so a member cannot match on a value the peer never sent.
package policyview

import (
	"errors"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// ErrMalformed is wrapped by every reader that meets a malformed attribute or
// NLRI section. The chain applies the member's on-error to the route
// (spec-filter-wire-0-umbrella, AC-28): the value is never answered.
var ErrMalformed = errors.New("malformed on the wire")

// errZeroReader is answered for a Reader that ReaderFor did not produce.
var errZeroReader = errors.New("BUG: policyview: read through the zero Reader")

// ErrNoUpdate is answered by a read on a view that holds no UPDATE: one never
// Reset, or one Clear released after its run.
var ErrNoUpdate = errors.New("policy view holds no UPDATE")

// Session is what the view needs to know about the session the UPDATE came
// from, beyond its encoding context. The reactor fills it from the peer.
type Session struct {
	// EBGP is true when the peer is in another AS (the protocol field).
	EBGP bool
}

// View is one route's lazy view during one synchronous chain run.
//
// The reactor owns a View per chain-running goroutine and calls Reset before
// each run, so a run allocates nothing for the view itself. A View is NOT safe
// for concurrent use: one run reads it at a time. A caller MUST NOT keep a
// View, or any Value it answered, past the run: call Clear when the run ends,
// and copy a value that has to outlive it.
type View struct {
	update  *wireu.WireUpdate
	session Session
	// run numbers the Reset calls. A cached slot is current only when it
	// carries this run's number, so Reset invalidates every slot in O(1).
	// It starts at 1 on the first Reset, so a zero slot is never current.
	run    uint64
	slots  [slotsMax]slot
	nlri   shared // prefix, family and next-hop share one NLRI decode
	asPath sharedPath
	// asPathText is the AS path as text, for ASPathText (aspath_text.go).
	asPathText sharedText
	decodes    Decodes
}

// slot caches one registered reader's answer for one run.
type slot struct {
	run   uint64
	value Value
	err   error
}

// shared caches the NLRI sections for one run, read by several fields.
type shared struct {
	run     uint64
	body    []byte
	mpReach wireu.MPReachWire
	err     error
}

// sharedPath caches the reconstructed AS path for one run: as-path and
// origin-as read it, however many members ask, and it is built once.
type sharedPath struct {
	run     uint64
	path    *attribute.ASPath
	present bool
	err     error
}

// Reset points the view at update for a new chain run, dropping every value
// the previous run decoded. update MUST stay valid, and its bytes unchanged,
// until Clear or the next Reset.
func (v *View) Reset(update *wireu.WireUpdate, session Session) {
	v.update = update
	v.session = session
	v.run++
	v.decodes = Decodes{}
}

// Clear releases the UPDATE at the end of a run, so a view kept by mistake
// answers ErrNoUpdate instead of reading a buffer the reactor reused.
func (v *View) Clear() {
	v.update = nil
	v.run++
}

// Read answers the field r reads, decoding it on the first read of this run.
// An absent field answers a Value whose Present is false and a nil error; a
// malformed one answers an error wrapping ErrMalformed.
func (v *View) Read(r Reader) (Value, error) {
	if v.update == nil {
		return Value{}, ErrNoUpdate
	}
	if r.slot == 0 {
		return Value{}, errZeroReader
	}
	s := &v.slots[r.slot-1]
	if s.run == v.run {
		return s.value, s.err
	}
	value, err := readerTable.reads[r.slot-1](v)
	s.run, s.value, s.err = v.run, value, err
	return value, err
}

// SetNumber overlays a numeric field for the rest of this run: a later Read
// of r answers number, present. The chain calls it once a member's set of the
// field folds, so a later member reads what an earlier one set
// (read-after-set). Reset and Clear drop it with every cached slot.
func (v *View) SetNumber(r Reader, number uint64) error {
	if v.update == nil {
		return ErrNoUpdate
	}
	if r.slot == 0 {
		return errZeroReader
	}
	s := &v.slots[r.slot-1]
	s.run, s.value, s.err = v.run, Value{present: true, number: number, numbered: true}, nil
	return nil
}

// SetWire overlays the field r reads with wire for the rest of this run, so a
// later read answers the value an earlier chain member set (read-after-set):
// the field is present when wire is not empty. wire is borrowed, and the
// caller MUST keep it unchanged until the run ends.
func (v *View) SetWire(r Reader, wire []byte) error {
	if v.update == nil {
		return ErrNoUpdate
	}
	if r.slot == 0 {
		return errZeroReader
	}
	s := &v.slots[r.slot-1]
	s.run, s.value, s.err = v.run, Value{present: len(wire) > 0, wire: wire}, nil
	return nil
}

// SetNLRI overlays the NLRI sections with body as the body NLRI and mpReach
// as the MP_REACH_NLRI value for the rest of this run, so a later read of
// prefix, family or next-hop answers what an earlier chain member's drops or
// family strip left (read-after-set), each from the same sections. A nil
// mpReach is an UPDATE that carries none, as the reactor drops an
// MP_REACH_NLRI whose NLRI are all dropped, so its family and its next hop
// leave family and next-hop with it; the NEXT_HOP attribute stays. A field
// read earlier in the run is answered again from the overlay, and NEXT_HOP is
// not decoded a second time. Both are borrowed, and the caller MUST keep them
// unchanged until the run ends.
func (v *View) SetNLRI(body []byte, mpReach wireu.MPReachWire) error {
	if v.update == nil {
		return ErrNoUpdate
	}
	v.nlri = shared{run: v.run, body: body, mpReach: mpReach}
	if s := v.current(policy.FieldPrefix); s != nil {
		s.value, s.err = prefixValue(body, mpReach), nil
	}
	if s := v.current(policy.FieldFamily); s != nil {
		s.value, s.err = familyValue(body, mpReach), nil
	}
	if s := v.current(policy.FieldNextHop); s != nil && s.err == nil {
		s.value = nextHopValue(s.value.wire, mpReach)
	}
	return nil
}

// current answers the slot of field's reader when this run already read it,
// and nil when it did not.
func (v *View) current(field policy.Field) *slot {
	index, ok := readerTable.slots[field]
	// refuse-by-default: guards SetNLRI -- a field with no reader has no answer cached to refresh
	if !ok {
		return nil
	}
	s := &v.slots[index]
	if s.run != v.run {
		return nil
	}
	return s
}

// AddPath reports whether the session the UPDATE came from negotiated
// ADD-PATH for fam on receive, so its NLRI carry a path identifier: the
// answer the reactor's own NLRI walk takes from the same encoding context
// (nlriSectionOf). An UPDATE whose context is not registered carries none.
func (v *View) AddPath(fam family.Family) (bool, error) {
	if v.update == nil {
		return false, ErrNoUpdate
	}
	ctx := bgpctx.Registry.Get(v.update.SourceCtxID())
	return ctx != nil && ctx.AddPath(fam), nil
}

// Decodes answers what this run decoded so far.
func (v *View) Decodes() Decodes { return v.decodes }

// attrs answers the UPDATE's attributes, nil when it carries none. The
// attribute index is the WireUpdate's own, built once for the UPDATE: reading
// it decodes no attribute value.
func (v *View) attrs() (*attribute.AttributesWire, error) {
	attrs, err := v.update.Attrs()
	if err != nil {
		return nil, errors.Join(ErrMalformed, err)
	}
	return attrs, nil
}

// raw answers one attribute's value bytes, borrowed, counting the decode, and
// nil when the UPDATE does not carry it.
func (v *View) raw(code attribute.AttributeCode) ([]byte, error) {
	attrs, err := v.attrs()
	if err != nil {
		return nil, err
	}
	if attrs == nil {
		return nil, nil
	}
	raw, err := attrs.GetRaw(code)
	if err != nil {
		return nil, errors.Join(ErrMalformed, err)
	}
	if raw != nil {
		v.decodes.countAttribute(code)
	}
	return raw, nil
}

// Value is one field's value on the view. It borrows the received bytes, so it
// is valid only during the run that read it (see View).
type Value struct {
	present  bool
	wire     []byte
	mpReach  wireu.MPReachWire
	path     *attribute.ASPath
	number   uint64
	numbered bool
	ebgp     bool
	family   family.Family
}

// Present reports whether the UPDATE carries the field. An absent field is
// an answer, never a zero value standing in for one.
func (x Value) Present() bool { return x.present }

// Wire answers the field's bytes as the peer sent them, borrowed: the
// attribute value for an attribute field, the body NLRI section for prefix,
// the NEXT_HOP value for next-hop, the aggregating router's address for
// aggregator.
func (x Value) Wire() []byte { return x.wire }

// MPReach answers the MP_REACH_NLRI attribute for prefix, family and
// next-hop, nil when the UPDATE carries none.
func (x Value) MPReach() wireu.MPReachWire { return x.mpReach }

// ASPath answers the reconstructed AS path for as-path. It is shared by the
// run and MUST NOT be modified.
func (x Value) ASPath() *attribute.ASPath { return x.path }

// Number answers the field's number (origin, med, local-preference, the
// aggregator's AS, the AIGP metric, the origin AS), and false when the field
// carries none: an AIGP attribute with no metric TLV, an origin AS of NONE.
func (x Value) Number() (uint64, bool) { return x.number, x.numbered }

// EBGP answers, for protocol, whether the route came from an eBGP session.
func (x Value) EBGP() bool { return x.ebgp }

// Family answers, for family, the body NLRI's family when the UPDATE carries a
// body NLRI section; MPReach answers the multiprotocol one.
func (x Value) Family() family.Family { return x.family }

// Reader names one registered field reader. It is resolved once, when a
// chain is compiled, so a run indexes a slot instead of looking a name up.
// The zero Reader is not valid: ReaderFor is the only way to get one.
type Reader struct {
	slot  int // the reader's index plus one, so the zero Reader names none
	field policy.Field
}

// Field answers the field r reads.
func (r Reader) Field() policy.Field { return r.field }
