// VALIDATES: RFC 5880 Section 6.7.3 -- for Keyed MD5 and Meticulous Keyed
// MD5 the Sequence Number field of the transmitted Authentication Section is
// bfd.XmitAuthSeq.
// PREVENTS: an MD5 transmit path that writes a constant or another counter,
// which the Keyed SHA1 fixtures of rfc5880_test.go would not see.
package session

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// RFC requirement: RFC5880-6.7.3-8 positive -- for Keyed MD5 and Meticulous
// Keyed MD5, Machine.Sign writes bfd.XmitAuthSeq (0x11223344, then the value
// AdvanceAuthSeq moves it to) into the Sequence Number field.
func TestRFC5880MD5SequenceFieldIsXmitAuthSeq(t *testing.T) {
	for _, at := range []uint8{packet.AuthTypeKeyedMD5, packet.AuthTypeMeticulousKeyedMD5} {
		m, _ := newMachine(t, newFakeClock())
		m.SetAuth(rfc5880SeqAuthPair(t, at))
		m.vars.XmitAuthSeq = 0x11223344

		buf, _ := rfc5880SignedByMachine(m)
		if got := binary.BigEndian.Uint32(buf[packet.MandatoryLen+4:]); got != 0x11223344 {
			t.Fatalf("type %d: Sequence Number field = %#x, want bfd.XmitAuthSeq 0x11223344", at, got)
		}

		m.AdvanceAuthSeq()
		buf, _ = rfc5880SignedByMachine(m)
		if got := binary.BigEndian.Uint32(buf[packet.MandatoryLen+4:]); got != m.vars.XmitAuthSeq {
			t.Fatalf("type %d: Sequence Number field = %#x after AdvanceAuthSeq, want bfd.XmitAuthSeq %#x",
				at, got, m.vars.XmitAuthSeq)
		}
	}
}
