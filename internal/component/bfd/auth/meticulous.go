// Design: rfc/short/rfc5880.md -- Section 6.7.3 / 6.8.1 (RcvAuthSeq)
//
// Sequence-number replay protection for Keyed and Meticulous Keyed
// authentication. RFC 5880 Section 6.8.1 describes bfd.RcvAuthSeq as
// the last sequence number received from the peer. Sections 6.7.3 and
// 6.7.4 open a window above it, in the unsigned 32-bit circular space:
//
//   - Keyed variants (MD5, SHA1): bfd.RcvAuthSeq to
//     bfd.RcvAuthSeq + 3 * Detect Mult inclusive.
//   - Meticulous variants: bfd.RcvAuthSeq + 1 to
//     bfd.RcvAuthSeq + 3 * Detect Mult inclusive.
//
// A packet outside the window is discarded. Check only inspects. The
// verifier calls Advance for the first packet before the digest step,
// as Section 6.7.3 orders, and afterwards only once the digest has
// matched, so once bfd.AuthSeqKnown is 1 a packet that fails
// authentication never moves bfd.RcvAuthSeq.
package auth

import "sync/atomic"

// SeqState tracks bfd.RcvAuthSeq for one session. All methods are
// safe for concurrent use via sync/atomic; in ze the verifier runs
// from the single express-loop goroutine so the atomicity is
// defensive rather than performance-critical.
type SeqState struct {
	last atomic.Uint32
	// initialized is bfd.AuthSeqKnown: whether the first packet has
	// reached the Sequence Number step. Before it, any value is
	// acceptable and Advance records it.
	initialized atomic.Bool
}

// Check reports whether seq lies in the replay window above
// bfd.RcvAuthSeq. detectMult is the Detect Mult field of the received
// packet. Before the first accepted packet (bfd.AuthSeqKnown 0) every
// sequence passes.
//
// RFC 5880 Section 6.7.3: "For Keyed MD5, if the sequence number lies
// outside of the range of bfd.RcvAuthSeq to bfd.RcvAuthSeq+(3*Detect Mult)
// inclusive (when treated as an unsigned 32-bit circular number space), the
// received packet MUST be discarded. For Meticulous Keyed MD5, if the
// sequence number lies outside of the range of bfd.RcvAuthSeq+1 to
// bfd.RcvAuthSeq+(3*Detect Mult) inclusive (when treated as an unsigned
// 32-bit circular number space) the received packet MUST be discarded."
// Section 6.7.4 states the same for Keyed and Meticulous Keyed SHA1.
//
// Check does NOT mutate SeqState; a failing verify after a successful
// Check leaves the last-accepted sequence unchanged.
func (s *SeqState) Check(seq uint32, meticulous bool, detectMult uint8) error {
	if !s.initialized.Load() {
		return nil
	}
	// Unsigned subtraction wraps, so ahead is the distance forward from
	// bfd.RcvAuthSeq in the circular space, and a sequence behind it reads
	// as more than 2^31 ahead.
	ahead := seq - s.last.Load()
	if meticulous && ahead == 0 {
		return ErrSequenceOutsideWindow
	}
	if ahead > 3*uint32(detectMult) {
		return ErrSequenceOutsideWindow
	}
	return nil
}

// Advance records seq as bfd.RcvAuthSeq and sets bfd.AuthSeqKnown. The
// caller MUST have accepted seq through Check first: Check is what keeps
// the stored value inside the window, so Advance stores without comparing.
// The verifier calls it twice for a first packet: to seed the floor before
// the digest step and again once the digest has matched.
func (s *SeqState) Advance(seq uint32) {
	s.last.Store(seq)
	s.initialized.Store(true)
}

// Last returns the most recent sequence number accepted by Advance,
// or zero before any packet has been accepted.
func (s *SeqState) Last() uint32 { return s.last.Load() }

// Initialized reports whether Advance has been called at least once.
func (s *SeqState) Initialized() bool { return s.initialized.Load() }
