// Design: rfc/short/rfc5880.md -- Section 6.7.3 / 6.8.1 (RcvAuthSeq)
//
// Sequence-number replay protection for Keyed and Meticulous Keyed
// authentication. RFC 5880 Section 6.8.1 describes bfd.RcvAuthSeq as
// the last sequence number accepted from the peer. Section 6.7.3
// splits the comparison rule between:
//
//   - Keyed variants (MD5, SHA1): a received sequence number MUST be
//     in the range bfd.RcvAuthSeq .. bfd.RcvAuthSeq + 3 * bfd.DetectMult
//     to pass replay. In practice ze relaxes this to ">= RcvAuthSeq"
//     so late packets under bursty scheduling do not cause false
//     negatives.
//   - Meticulous variants: every packet MUST carry a strictly-greater
//     sequence number than the last accepted one; equal or smaller
//     is a replay and the packet is dropped.
//
// Forward progress on SeqState is the caller's responsibility via
// Advance; Check only inspects without mutating so a verifier can
// undo the comparison if it later fails (e.g., digest mismatch).
package auth

import "sync/atomic"

// SeqState tracks bfd.RcvAuthSeq for one session. All methods are
// safe for concurrent use via sync/atomic; in ze the verifier runs
// from the single express-loop goroutine so the atomicity is
// defensive rather than performance-critical.
type SeqState struct {
	last atomic.Uint32
	// initialized tracks whether the first authenticated packet
	// has been accepted. Before the first packet, any value is
	// acceptable and Advance records it.
	initialized atomic.Bool
}

// Check reports whether seq passes the replay-protection check for
// the current state. Meticulous variants require strictly greater;
// non-meticulous require greater-or-equal.
//
// Check does NOT mutate SeqState; a failing verify after a successful
// Check leaves the last-accepted sequence unchanged.
func (s *SeqState) Check(seq uint32, meticulous bool) error {
	if !s.initialized.Load() {
		return nil
	}
	last := s.last.Load()
	if meticulous {
		if seq <= last {
			return ErrSequenceRegress
		}
		return nil
	}
	if seq < last {
		return ErrSequenceRegress
	}
	return nil
}

// Advance records seq as the most recent accepted sequence number.
// For meticulous variants this also enforces strict monotonic; for
// non-meticulous it accepts equal values.
func (s *SeqState) Advance(seq uint32, meticulous bool) {
	if !s.initialized.Load() {
		s.last.Store(seq)
		s.initialized.Store(true)
		return
	}
	cur := s.last.Load()
	if meticulous {
		if seq > cur {
			s.last.Store(seq)
		}
		return
	}
	if seq >= cur {
		s.last.Store(seq)
	}
}

// Last returns the most recent sequence number accepted by Advance,
// or zero before any packet has been accepted.
func (s *SeqState) Last() uint32 { return s.last.Load() }

// Initialized reports whether Advance has been called at least once.
func (s *SeqState) Initialized() bool { return s.initialized.Load() }
