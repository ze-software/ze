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
// unfeasible is the one this session held, so its withdrawal names that route.
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
// is already answered there. The first half needs the bytes, so the session
// keeps, for each route key of a body-judged family, the last well-formed
// generation it received, and a withdrawal of a malformed NLRI carries that
// generation instead. A malformed NLRI whose key holds no generation names a
// route this session never installed: removing it from the Adj-RIB-In removes
// nothing, so nothing is relayed for it.

package reactor

import (
	"bytes"
	"errors"

	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/family"
)

var errTypedNLRINoSplitter = errors.New("typed NLRI family has no splitter")

// typedNLRIRelay is the withdrawal section typedNLRIWithdrawable returns.
type typedNLRIRelay struct {
	// nlri is the section to relay. It is the input itself when changed is false.
	nlri []byte
	// changed reports that nlri differs from the input: an NLRI was replaced by
	// its held generation or dropped.
	changed bool
	// dropped counts the NLRIs that left the section: an unkeyable withdrawal,
	// and a malformed one whose key holds no generation.
	dropped int
}

// typedNLRIHold files every announced NLRI of section as the generation this
// session holds for its route key, or forgets the key when withdrawAll says the
// whole UPDATE is treated as withdrawn (RFC 7606 Section 2), because then
// message.SynthesizeWithdrawFamilies withdraws these NLRIs with their own bytes.
//
// section is a body-judged family's MP_REACH_NLRI after the typed NLRI pass, so
// every NLRI in it is one the recognizer kept and whose key forms. Runs on the
// session's receive goroutine only, which is the only reader and writer of
// typedNLRIHeld.
//
// A held generation can outlive the route in the Adj-RIB-In: a later receive
// check (the RFC 4271 Section 5.1.3 next-hop check in processValidatedMessage)
// can still withdraw the UPDATE. The only effect is that a later malformed NLRI
// of that key is withdrawn by the old bytes, towards destinations whose
// Adj-RIB-Out holds no such key, and the writer removes it there.
func (s *Session) typedNLRIHold(fam family.Family, section []byte, addPath, withdrawAll bool) error {
	split := nlrisplit.Get(fam)
	if split == nil {
		return errTypedNLRINoSplitter
	}
	_, err := split(section, addPath, func(one []byte) bool {
		key, keyErr := s.typedNLRIKey(fam, one, addPath, false)
		if keyErr != nil {
			// Unreachable through typedNLRIEdit: nlritype.Retain discards an
			// unkeyable NLRI its recognizer refused, and ValidateNLRI refuses every
			// unkeyable implemented one. Holding nothing for it is the safe answer.
			return true
		}
		if withdrawAll {
			delete(s.typedNLRIHeld, key)
			return true
		}
		s.holdTypedNLRI(key, one)
		return true
	})
	return err
}

// holdTypedNLRI records one as the held generation of key. A re-announcement of
// the same bytes, the common refresh, costs no allocation.
func (s *Session) holdTypedNLRI(key fwdPathKey, one []byte) {
	if s.typedNLRIHeld == nil {
		s.typedNLRIHeld = make(map[fwdPathKey][]byte)
	}
	held := s.typedNLRIHeld[key]
	if bytes.Equal(held, one) {
		return
	}
	// Reusing held's array is safe: every reader copies it out (append) before
	// the next receive can reach this function.
	s.typedNLRIHeld[key] = append(held[:0], one...)
}

// typedNLRIWithdrawable returns section, a withdrawal of a body-judged family,
// with every NLRI in a form a receiver decodes and that names a route this
// session held.
//
// For each NLRI: one whose key cannot form names no route and is dropped
// (AC-10a); a malformed one (recognize answers Withdraw) is replaced by the
// generation held for its key, or dropped when none is held; a well-formed one is
// kept as received. Every key named leaves typedNLRIHeld, because the route
// leaves the Adj-RIB-In.
//
// withdrawFraming says section is framed as an MP_UNREACH_NLRI rather than as
// the announced NLRIs nlritype.Retain moved out of an MP_REACH_NLRI. Returns the
// input itself, with no allocation, when nothing changed. A non-nil error means
// the section cannot be walked, which RFC 7606 Section 3(j) answers with a
// session reset.
func (s *Session) typedNLRIWithdrawable(
	fam family.Family, recognize nlritype.Recognizer, section []byte, addPath, withdrawFraming bool,
) (typedNLRIRelay, error) {
	relay := typedNLRIRelay{nlri: section}
	if len(section) == 0 {
		return relay, nil
	}
	split := nlrisplit.Get(fam)
	if withdrawFraming {
		split = nlrisplit.GetWithdraw(fam)
	}
	if split == nil {
		return relay, errTypedNLRINoSplitter
	}
	var out []byte
	consumed := 0
	_, err := split(section, addPath, func(one []byte) bool {
		emit := s.typedNLRIWithdrawal(fam, recognize, one, addPath, withdrawFraming)
		if !relay.changed && !bytes.Equal(emit, one) {
			relay.changed = true
			out = append(make([]byte, 0, len(section)), section[:consumed]...)
		}
		consumed += len(one)
		if emit == nil {
			relay.dropped++
		}
		if relay.changed {
			out = append(out, emit...)
		}
		return true
	})
	if err != nil {
		return typedNLRIRelay{nlri: section}, err
	}
	if relay.changed {
		relay.nlri = out
	}
	return relay, nil
}

// typedNLRIWithdrawal is the NLRI to relay for one withdrawn NLRI, or nil when
// nothing is relayed for it. The returned slice is one itself, or the held
// generation, which the caller copies before the next receive.
func (s *Session) typedNLRIWithdrawal(
	fam family.Family, recognize nlritype.Recognizer, one []byte, addPath, withdrawFraming bool,
) []byte {
	key, err := s.typedNLRIKey(fam, one, addPath, withdrawFraming)
	if err != nil {
		// No key forms, so no Adj-RIB-In entry can match and no withdrawal can
		// name the route (AC-10a). Relaying it would also fail at every writer
		// (adjOutWrite.key).
		return nil
	}
	held, found := s.typedNLRIHeld[key]
	delete(s.typedNLRIHeld, key)
	if recognize(one, addPath) != nlritype.Withdraw {
		return one
	}
	// draft-ietf-bess-mup-safi Section 3.1.3.1: "the mandatory fields exceed the
	// declared Length; the NLRI is malformed; MUST be treated as
	// Treat-as-withdraw." RFC 7606 Section 2: treat-as-withdraw causes the routes
	// "to be removed from the Adj-RIB-In". The route removed is the held one, so
	// its withdrawal carries the held bytes; with none held the removal is empty.
	if !found {
		return nil
	}
	return held
}

// typedNLRIKey is the held-generation key of one carved NLRI: the RFC 7911 path
// identifier when addPath frames one, and the family's canonical route key.
func (s *Session) typedNLRIKey(fam family.Family, one []byte, addPath, withdrawFraming bool) (fwdPathKey, error) {
	var key fwdPathKey
	received, raw := uint32(0), one
	if addPath {
		if len(one) < 4 {
			return key, errAdjOutProvenance
		}
		received = uint32(one[0])<<24 | uint32(one[1])<<16 | uint32(one[2])<<8 | uint32(one[3])
		raw = one[4:]
	}
	err := fwdPathKeyFor(&key, fam, received, raw, withdrawFraming, s.typedNLRIKeyScratch[:])
	return key, err
}
