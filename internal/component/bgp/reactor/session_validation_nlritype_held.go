// Design: docs/architecture/core-design.md -- RFC 7606 UPDATE validation
// RFC: rfc/short/rfc7606.md -- Section 2 treat-as-withdraw; rfc/short/draft-ietf-bess-mup-safi.md -- Section 3.1.3.1
// Overview: session_validation_nlritype.go -- typedNLRIEdit, which calls these
// Related: forward_path_id.go -- fwdPathKey, the route key a held generation is filed under
//
// A family whose recognizer judges the NLRI body (nlritype.RegisterBodyRecognizer)
// answers a malformed announced route with treat-as-withdraw. RFC 7606 Section 2
// defines that as the route being "treated as though all contained routes had
// been withdrawn just as if they had been listed in the WITHDRAWN ROUTES field (or
// in the MP_UNREACH_NLRI attribute if appropriate) of the UPDATE message, thus
// causing them to be removed from the Adj-RIB-In according to the procedures of
// [RFC4271]."
//
// The obligation is the Adj-RIB-In removal. What reaches another peer is the
// RFC 4271 Section 9.2 consequence of it: "All newly installed routes and all
// newly unfeasible routes for which there is no replacement route SHALL be
// advertised to its peers by means of an UPDATE message." The route that became
// unfeasible is the one held for the peer, so its withdrawal names that route.
// The malformed bytes cannot: a receiver decodes every field of a withdrawn
// NLRI, Ze's own recognizer refused those bytes, and GoBGP 3.31.0 panics on them.
//
// FRR and BIRD build the downstream withdrawal the same way, from the route they
// hold and only when they advertised it. FRR's bgp_adj_out_unset_subgroup
// (bgpd/bgp_updgrp_adv.c) queues a withdrawal only "if (adj->attr && withdraw)",
// and subgroup_withdraw_packet (bgpd/bgp_updgrp_packet.c) encodes it from
// bgp_dest_get_prefix(dest), the stored route. BIRD's bgp_rte_update
// (proto/bgp/packets.c) turns the invalid route into rte_update3(..., n, NULL, ...),
// a withdrawal of the net, and rt_notify_basic (nest/rt-table.c) drops it unless
// "bmap_test(&c->export_map, old->id)" says the route was exported.
//
// Ze's forward rails relay the received withdrawal and ask each destination's
// Adj-RIB-Out whether it holds the key (adjOutWrite.allow), so the second half
// is answered there. The first half needs the bytes, so the peer keeps, for each
// route key of a body-judged family, the last well-formed generation it received,
// and a withdrawal of a malformed NLRI carries that generation instead. A
// malformed NLRI whose key holds no generation names a route the peer never
// installed: removing it from the Adj-RIB-In removes nothing, so nothing is
// relayed for it.
//
// The hold lives as long as the routes it mirrors. It belongs to the Peer, so a
// route RFC 4724 Section 4.2 retains across a restart ("MUST retain the routes
// received from the peer for all the address families that were previously
// received in the Graceful Restart Capability and MUST mark them as stale routes")
// is still withdrawable by a malformed NLRI on the new session. Each session
// starts a new generation. A session that negotiates no Graceful Restart drops
// every older generation at once, as Section 4.2 has the stale routes removed
// ("If the Graceful Restart Capability ... is not received in the re-established
// session, the receiving speaker MUST immediately remove all the stale routes"),
// and an End-of-RIB drops the older generations of its family, as the stale
// routes are ("Once the End-of-RIB marker for an address family is received
// from the peer, it MUST immediately remove any routes from the peer that are
// still marked as stale for that address family").

package reactor

import (
	"bytes"
	"errors"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/family"
)

var errTypedNLRINoSplitter = errors.New("typed NLRI family has no splitter")

// typedNLRIHeldRoutes is a peer's last well-formed generation of each route key
// of the body-judged families. Bounded by the routes the peer holds in those
// families, plus, until the next End-of-RIB or a session without Graceful
// Restart, those it held on the previous session. Safe for concurrent use: two
// sessions of one peer can overlap during connection collision.
type typedNLRIHeldRoutes struct {
	mu         sync.Mutex
	routes     map[fwdPathKey]typedNLRIHeldRoute
	generation uint64
}

// typedNLRIHeldRoute is one held generation, the session generation that last
// announced it, and the UPDATE of that session that did (Session.typedNLRIUpdate).
type typedNLRIHeldRoute struct {
	nlri       []byte
	generation uint64
	update     uint64
}

// beginSession starts a new generation for a newly established session. When
// the session negotiated no Graceful Restart, every route of an older session is
// gone from the Adj-RIB-In, so its held generation is dropped too.
func (h *typedNLRIHeldRoutes) beginSession(gracefulRestart bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.generation++
	if gracefulRestart {
		return
	}
	clear(h.routes)
}

// endOfRIB drops the held generations of fam that the current session has not
// announced again: RFC 4724 Section 4.2 removes those stale routes at this marker.
func (h *typedNLRIHeldRoutes) endOfRIB(fam family.Family) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for key, held := range h.routes {
		if key.family != fam {
			continue
		}
		if held.generation == h.generation {
			continue
		}
		delete(h.routes, key)
	}
}

// typedNLRIRelay is the withdrawal section typedNLRIWithdrawable returns.
type typedNLRIRelay struct {
	// nlri holds the NLRIs relayed as received. It is the input itself when
	// changed is false.
	nlri []byte
	// replaced holds the held generations that stand in for malformed NLRIs.
	// They can be longer than what they replace, so the caller carries them in
	// withdrawals sized to the message limit (typedNLRIWithdrawBodies), never in
	// the section that held the malformed bytes.
	replaced []byte
	// changed reports that nlri differs from the input: an NLRI was replaced or
	// dropped.
	changed bool
	// dropped counts the NLRIs that left the section with nothing relayed for
	// them: an unkeyable withdrawal, and a malformed one whose key holds no
	// generation.
	dropped int
}

// typedNLRIWalk is the state of one walk over a section, kept on the Session
// with its two visitors bound once, so a walk on the receive path allocates
// nothing: a closure handed to the family's splitter would escape per call.
// Used only on the session's receive goroutine.
type typedNLRIWalk struct {
	session        *Session
	held           *typedNLRIHeldRoutes
	fam            family.Family
	recognize      nlritype.Recognizer
	addPath        bool
	withdrawAll    bool
	withdrawFrame  bool
	section        []byte
	consumed       int
	relay          typedNLRIRelay
	holdVisitor    func([]byte) bool
	relayVisitor   func([]byte) bool
	scratchHeldKey fwdPathKey
}

// typedNLRIWalkState returns the session's walk state, binding its visitors on
// first use.
func (s *Session) typedNLRIWalkState() *typedNLRIWalk {
	if w := s.typedNLRIWalk; w != nil {
		return w
	}
	w := &typedNLRIWalk{session: s}
	w.holdVisitor = w.hold
	w.relayVisitor = w.withdrawal
	s.typedNLRIWalk = w
	return w
}

// heldTypedNLRI returns the hold this session files into: its Peer's,
// set by Peer.newSession, or a session-private one for a Session built without
// a Peer.
func (s *Session) heldTypedNLRI() *typedNLRIHeldRoutes {
	if s.typedNLRIHeld == nil {
		s.typedNLRIHeld = &typedNLRIHeldRoutes{}
	}
	return s.typedNLRIHeld
}

// typedNLRIHold files every announced NLRI of section as the generation held
// for its route key, or forgets the key when withdrawAll says the whole UPDATE is
// treated as withdrawn (RFC 7606 Section 2), because then
// message.SynthesizeWithdrawFamilies withdraws these NLRIs with their own bytes.
//
// section is a body-judged family's MP_REACH_NLRI after the typed NLRI pass, so
// every NLRI in it is one the recognizer kept and whose key forms.
//
// A held generation can outlive the route in the Adj-RIB-In: a later receive
// check (the RFC 4271 Section 5.1.3 next-hop check in processValidatedMessage)
// can still withdraw the UPDATE. The only effect is that a later malformed NLRI
// of that key is withdrawn by the old bytes, towards destinations whose
// Adj-RIB-Out holds no such key, and the writer removes it there.
func (s *Session) typedNLRIHold(fam family.Family, section []byte, addPath, withdrawAll bool) error {
	if len(section) == 0 {
		return nil
	}
	split := nlrisplit.Get(fam)
	if split == nil {
		return errTypedNLRINoSplitter
	}
	w := s.typedNLRIWalkState()
	w.held, w.fam, w.addPath, w.withdrawAll, w.withdrawFrame = s.heldTypedNLRI(), fam, addPath, withdrawAll, false
	w.held.mu.Lock()
	_, err := split(section, addPath, w.holdVisitor)
	w.held.mu.Unlock()
	w.held = nil
	return err
}

// hold is the splitter visitor of typedNLRIHold. Called with held.mu held.
func (w *typedNLRIWalk) hold(one []byte) bool {
	if !w.session.typedNLRIKey(&w.scratchHeldKey, w.fam, one, w.addPath, false) {
		// Unreachable through typedNLRIEdit: nlritype.Retain discards an
		// unkeyable NLRI its recognizer refused, and ValidateNLRI refuses every
		// unkeyable implemented one. Holding nothing for it is the safe answer.
		return true
	}
	key := w.scratchHeldKey
	if w.withdrawAll {
		delete(w.held.routes, key)
		return true
	}
	if w.held.routes == nil {
		w.held.routes = make(map[fwdPathKey]typedNLRIHeldRoute)
	}
	held := w.held.routes[key]
	held.generation = w.held.generation
	held.update = w.session.typedNLRIUpdate
	if !bytes.Equal(held.nlri, one) {
		// Reusing held's array is safe: every reader copies it out (append)
		// before it is deleted or written again, under held.mu.
		held.nlri = append(held.nlri[:0], one...)
	}
	w.held.routes[key] = held
	return true
}

// typedNLRIWithdrawable splits section, a withdrawal of a body-judged family,
// into the NLRIs relayed as received and the held generations that replace the
// malformed ones.
//
// For each NLRI: one whose key cannot form names no route and is dropped
// (AC-10a); a malformed one (recognize answers Withdraw) is replaced by the
// generation held for its key, or dropped when none is held; a well-formed one is
// kept as received. Every key named leaves the hold, because the route leaves
// the Adj-RIB-In.
//
// withdrawFraming says section is framed as an MP_UNREACH_NLRI rather than as
// the announced NLRIs nlritype.Retain moved out of an MP_REACH_NLRI. The common
// "nothing changed" answer returns the input itself and allocates nothing. A
// non-nil error means the section cannot be walked, which RFC 7606 Section 3(j)
// answers with a session reset.
func (s *Session) typedNLRIWithdrawable(
	fam family.Family, recognize nlritype.Recognizer, section []byte, addPath, withdrawFraming bool,
) (typedNLRIRelay, error) {
	if len(section) == 0 {
		return typedNLRIRelay{nlri: section}, nil
	}
	split := nlrisplit.Get(fam)
	if withdrawFraming {
		split = nlrisplit.GetWithdraw(fam)
	}
	if split == nil {
		return typedNLRIRelay{nlri: section}, errTypedNLRINoSplitter
	}
	w := s.typedNLRIWalkState()
	w.held, w.fam, w.recognize, w.addPath, w.withdrawFrame = s.heldTypedNLRI(), fam, recognize, addPath, withdrawFraming
	w.section, w.consumed, w.relay = section, 0, typedNLRIRelay{nlri: section}
	w.held.mu.Lock()
	_, err := split(section, addPath, w.relayVisitor)
	w.held.mu.Unlock()
	relay := w.relay
	w.held, w.recognize, w.section, w.relay = nil, nil, nil, typedNLRIRelay{}
	if err != nil {
		return typedNLRIRelay{nlri: section}, err
	}
	return relay, nil
}

// withdrawal is the splitter visitor of typedNLRIWithdrawable. Called with
// held.mu held.
func (w *typedNLRIWalk) withdrawal(one []byte) bool {
	start := w.consumed
	w.consumed += len(one)
	keyed := w.session.typedNLRIKey(&w.scratchHeldKey, w.fam, one, w.addPath, w.withdrawFrame)
	if !keyed {
		// No key forms, so no Adj-RIB-In entry can match and no withdrawal can
		// name the route (AC-10a). Relaying it would also fail at every writer
		// (adjOutWrite.key).
		w.leave(start)
		w.relay.dropped++
		return true
	}
	held, found := w.held.routes[w.scratchHeldKey]
	// RFC 4271 Section 4.3: "A BGP speaker SHOULD treat an UPDATE message of this
	// form as though the WITHDRAWN ROUTES do not contain the address prefix." A key
	// this UPDATE announces keeps its route and its hold; a malformed withdrawal of
	// it names nothing to relay.
	announced := found && held.generation == w.held.generation && held.update == w.session.typedNLRIUpdate
	if announced {
		if w.recognize(one, w.addPath) == nlritype.Withdraw {
			w.leave(start)
			w.relay.dropped++
			return true
		}
		if w.relay.changed {
			w.relay.nlri = append(w.relay.nlri, one...)
		}
		return true
	}
	delete(w.held.routes, w.scratchHeldKey)
	if w.recognize(one, w.addPath) != nlritype.Withdraw {
		if w.relay.changed {
			w.relay.nlri = append(w.relay.nlri, one...)
		}
		return true
	}
	// draft-ietf-bess-mup-safi Section 3.1.3.1: "the mandatory fields exceed the
	// declared Length; the NLRI is malformed; MUST be treated as
	// Treat-as-withdraw." RFC 7606 Section 2: treat-as-withdraw causes the routes
	// "to be removed from the Adj-RIB-In". The route removed is the held one, so
	// its withdrawal carries the held bytes; with none held the removal is empty.
	w.leave(start)
	if !found {
		w.relay.dropped++
		return true
	}
	w.relay.replaced = append(w.relay.replaced, held.nlri...)
	return true
}

// leave records that the NLRI starting at start is not relayed as received:
// the first time, the NLRIs before it are copied out of the section.
func (w *typedNLRIWalk) leave(start int) {
	if w.relay.changed {
		return
	}
	w.relay.changed = true
	w.relay.nlri = append(make([]byte, 0, len(w.section)), w.section[:start]...)
}

// typedNLRIKey writes into out the held-generation key of one carved NLRI: the
// RFC 7911 path identifier when addPath frames one, and the family's canonical
// route key. It reports false when no key forms.
func (s *Session) typedNLRIKey(out *fwdPathKey, fam family.Family, one []byte, addPath, withdrawFraming bool) bool {
	received, raw := uint32(0), one
	if addPath {
		if len(one) < 4 {
			return false
		}
		received = uint32(one[0])<<24 | uint32(one[1])<<16 | uint32(one[2])<<8 | uint32(one[3])
		raw = one[4:]
	}
	return fwdPathKeyFor(out, fam, received, raw, withdrawFraming, s.typedNLRIKeyScratch[:]) == nil
}

// typedNLRIWithdrawBodies returns withdraw-only UPDATE bodies carrying nlri, the
// held generations of one family, each no larger than the session's UPDATE body
// limit. A held generation can be longer than the malformed NLRI it replaces, so
// the replacements never go back into the received UPDATE, which already sits at
// most at that limit.
//
// RFC 8654 Section 4: "A BGP speaker MUST NOT send a BGP Extended Message to a
// peer that did not advertise the BGP Extended Message Capability"; the bodies
// are sized to what this session itself accepts (extendedMessage), the limit
// the received UPDATE met, and the forward rails split again for a destination
// with a smaller one.
func (s *Session) typedNLRIWithdrawBodies(bodies [][]byte, afi uint16, safi uint8, nlri []byte, addPath bool) ([][]byte, error) {
	if len(nlri) == 0 {
		return bodies, nil
	}
	fam := family.Family{AFI: family.AFI(afi), SAFI: family.SAFI(safi)}
	split := nlrisplit.GetWithdraw(fam)
	if split == nil {
		return bodies, errTypedNLRINoSplitter
	}
	// Withdrawn Routes Length (2), Total Path Attribute Length (2), an Extended
	// Length MP_UNREACH_NLRI header (4), and its AFI and SAFI (3).
	const overhead = 2 + 2 + 4 + 3
	budget := int(message.MaxMessageLength(msgtype.TypeUPDATE, s.extendedMessage)) - message.HeaderLen - overhead
	start, end := 0, 0
	_, err := split(nlri, addPath, func(one []byte) bool {
		if end-start+len(one) > budget && end > start {
			bodies = append(bodies, message.BuildMPUnreachBody(afi, safi, nlri[start:end]))
			start = end
		}
		end += len(one)
		return true
	})
	if err != nil {
		return bodies, err
	}
	if end > start {
		bodies = append(bodies, message.BuildMPUnreachBody(afi, safi, nlri[start:end]))
	}
	return bodies, nil
}
