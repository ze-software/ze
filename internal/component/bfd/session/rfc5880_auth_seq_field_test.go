// VALIDATES: RFC 5880 Sections 6.7.3 and 6.7.4 -- the transmitted Sequence
// Number field is bfd.XmitAuthSeq for every keyed Auth Type, whatever value
// the variable holds, including across the 32-bit wrap.
// PREVENTS: a transmitter that writes something other than bfd.XmitAuthSeq
// once the variable leaves the small values a fixture usually picks: a signed
// or saturating counter above 0x7FFFFFFF, a truncated field, or a wrap that
// does not follow the variable.
package session

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// RFC requirement: RFC5880-6.7.3-8 negative -- "The Sequence Number field
// MUST be set to bfd.XmitAuthSeq." No receiver can refuse a violation, because
// it cannot know the sender's variable, so the inputs are forced toward one
// instead: for each of the four keyed Auth Types, bfd.XmitAuthSeq is set to
// 0, 1, 0x7FFFFFFF, 0x80000000, 0xFFFFFFFE and 0xFFFFFFFF, and then advanced
// across the 32-bit wrap to 0 and 1. After every Sign the field equals the
// variable's value at that moment.
func TestRFC5880SequenceFieldIsXmitAuthSeqAcrossTheCounterRange(t *testing.T) {
	keyed := []uint8{
		packet.AuthTypeKeyedMD5, packet.AuthTypeMeticulousKeyedMD5,
		packet.AuthTypeKeyedSHA1, packet.AuthTypeMeticulousKeyedSHA1,
	}
	forced := []uint32{0, 1, 0x7FFFFFFF, 0x80000000, 0xFFFFFFFE, 0xFFFFFFFF}
	for _, at := range keyed {
		m, _ := newMachine(t, newFakeClock())
		m.SetAuth(rfc5880SeqAuthPair(t, at))

		for _, value := range forced {
			m.vars.XmitAuthSeq = value
			buf, _ := rfc5880SignedByMachine(m)
			if got := binary.BigEndian.Uint32(buf[packet.MandatoryLen+4:]); got != value {
				t.Fatalf("type %d: Sequence Number field = %#x, want bfd.XmitAuthSeq %#x", at, got, value)
			}
		}

		// The last forced value is 0xFFFFFFFF: two advances wrap the
		// variable to 0 and then 1, and the field follows it.
		for range 2 {
			m.AdvanceAuthSeq()
			buf, _ := rfc5880SignedByMachine(m)
			if got := binary.BigEndian.Uint32(buf[packet.MandatoryLen+4:]); got != m.vars.XmitAuthSeq {
				t.Fatalf("type %d: Sequence Number field = %#x after the wrap, want bfd.XmitAuthSeq %#x",
					at, got, m.vars.XmitAuthSeq)
			}
		}
		if m.vars.XmitAuthSeq != 1 {
			t.Fatalf("type %d: bfd.XmitAuthSeq = %#x after two advances from 0xFFFFFFFF, want 1", at, m.vars.XmitAuthSeq)
		}
	}
}
