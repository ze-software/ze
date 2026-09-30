// VALIDATES: two RFC 5880 Sections 6.7.3 and 6.7.4 transmit rules from the
// receiving side. For the Meticulous types, the packet a transmitter emits at
// the 32-bit boundary when bfd.XmitAuthSeq does not wrap is refused. For the
// four keyed types, a section whose Sequence Number is not bfd.XmitAuthSeq is
// refused.
// PREVENTS: a peer relationship that survives a transmitter which stops at
// 0xFFFFFFFF, which is what a saturating increment produces, or one that
// signs with a value other than its own bfd.XmitAuthSeq.
//
// Method: the transmitting Machine signs at 0xFFFFFFFF, and the peer accepts
// that packet. The transmitter then signs a second packet with
// bfd.XmitAuthSeq still at 0xFFFFFFFF, which is the value a non-circular
// increment leaves there, and the peer must refuse it. The circular successor,
// 0, is accepted by a peer in the same state, so the refusal is the repeat.
// The Sequence Number cases sign with the production signer at a value other
// than bfd.XmitAuthSeq, then show the Machine.Sign packet is accepted.
package session

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/auth"
	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// RFC requirement: RFC5880-6.7.3-4 negative -- for Meticulous Keyed MD5, a
// peer that accepted Sequence Number 0xFFFFFFFF signed by Machine.Sign
// discards with auth.ErrSequenceOutsideWindow the next packet when it again
// carries 0xFFFFFFFF, the value a non-circular bfd.XmitAuthSeq keeps, and
// accepts 0, the circular successor, in the same state.
// RFC requirement: RFC5880-6.7.4-4 negative -- the same repeat of 0xFFFFFFFF
// is discarded, and 0 accepted, for Meticulous Keyed SHA1.
func TestRFC5880MeticulousNonCircularRepeatAtWrapDiscarded(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeMeticulousKeyedMD5, packet.AuthTypeMeticulousKeyedSHA1} {
		m, _ := newMachine(t, newFakeClock())
		pair := rfc5880SeqAuthPair(t, at)
		m.SetAuth(pair)
		m.vars.XmitAuthSeq = 0xFFFFFFFF

		var peer auth.SeqState
		first, c := rfc5880SignedByMachine(m)
		if err := pair.Verifier.Verify(first, c, &peer); err != nil {
			t.Fatalf("type %d: peer discarded 0xFFFFFFFF: %v", at, err)
		}

		repeat, c := rfc5880SignedByMachine(m)
		if got := binary.BigEndian.Uint32(repeat[packet.MandatoryLen+4:]); got != 0xFFFFFFFF {
			t.Fatalf("type %d: precondition: repeated Sequence Number %#x, want 0xFFFFFFFF", at, got)
		}
		if err := pair.Verifier.Verify(repeat, c, &peer); !errors.Is(err, auth.ErrSequenceOutsideWindow) {
			t.Fatalf("type %d: 0xFFFFFFFF after 0xFFFFFFFF: got %v, want auth.ErrSequenceOutsideWindow", at, err)
		}

		m.AdvanceAuthSeq()
		wrapped, c := rfc5880SignedByMachine(m)
		if got := binary.BigEndian.Uint32(wrapped[packet.MandatoryLen+4:]); got != 0 {
			t.Fatalf("type %d: circular successor %#x, want 0", at, got)
		}
		if err := pair.Verifier.Verify(wrapped, c, &peer); err != nil {
			t.Fatalf("type %d: peer discarded the circular successor 0: %v", at, err)
		}
	}
}

// RFC requirement: RFC5880-6.7.3-9 negative -- for Keyed MD5 (and Keyed
// SHA1, which Section 6.7.4 gives the same window), after the peer accepted a
// packet at 100 so bfd.RcvAuthSeq is 100, a section signed with 1101 lies
// beyond RcvAuthSeq+(3*Detect Mult) and the peer discards it with
// auth.ErrSequenceOutsideWindow; the packet at 101 is then accepted.
// RFC requirement: RFC5880-6.7.3-10 negative -- for Meticulous Keyed MD5
// (and Meticulous Keyed SHA1), after bfd.RcvAuthSeq became 100, both 1101
// (beyond the window) and 100 (below RcvAuthSeq+1) are discarded with
// auth.ErrSequenceOutsideWindow; the packet at 101 is then accepted.
func TestRFC5880SequenceFieldOtherThanXmitAuthSeqDiscarded(t *testing.T) {
	keyed := []uint8{
		packet.AuthTypeKeyedMD5, packet.AuthTypeMeticulousKeyedMD5,
		packet.AuthTypeKeyedSHA1, packet.AuthTypeMeticulousKeyedSHA1,
	}
	for _, at := range keyed {
		m, _ := newMachine(t, newFakeClock())
		pair := rfc5880SeqAuthPair(t, at)
		m.SetAuth(pair)
		m.vars.XmitAuthSeq = 100

		var peer auth.SeqState
		first, c := rfc5880SignedByMachine(m)
		if err := pair.Verifier.Verify(first, c, &peer); err != nil {
			t.Fatalf("type %d: peer discarded bfd.XmitAuthSeq 100: %v", at, err)
		}
		m.AdvanceAuthSeq()

		others := []uint32{m.vars.XmitAuthSeq + 1000}
		meticulous := at == packet.AuthTypeMeticulousKeyedMD5 || at == packet.AuthTypeMeticulousKeyedSHA1
		if meticulous {
			others = append(others, m.vars.XmitAuthSeq-1)
		}
		for _, other := range others {
			c := m.Build()
			buf := make([]byte, c.Length)
			c.WriteTo(buf, 0)
			pair.Signer.Sign(buf, packet.MandatoryLen, other)
			if err := pair.Verifier.Verify(buf, c, &peer); !errors.Is(err, auth.ErrSequenceOutsideWindow) {
				t.Fatalf("type %d: Sequence Number %d with bfd.XmitAuthSeq %d: got %v, want auth.ErrSequenceOutsideWindow",
					at, other, m.vars.XmitAuthSeq, err)
			}
		}

		compliant, c := rfc5880SignedByMachine(m)
		if err := pair.Verifier.Verify(compliant, c, &peer); err != nil {
			t.Fatalf("type %d: peer discarded bfd.XmitAuthSeq 101: %v", at, err)
		}
	}
}
