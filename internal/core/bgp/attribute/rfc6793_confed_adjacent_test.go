// VALIDATES: AS_PATH/AS4_PATH reconciliation prepends a confederation segment
// adjacent to a prepended segment and drops one that is not (RFC 6793
// Section 4.2.3).
// PREVENTS: losing a confederation segment that must be kept, or keeping one
// the rule leaves out.

package attribute

import (
	"bytes"
	"testing"
)

// TestRFC6793ConfedSegmentAdjacentToPrependedIsPrepended drives the second
// clause of the RFC 6793 Section 4.2.3 confederation rule: a confederation
// segment that does not lead the path but sits right after a prepended segment.
// Method: AS_PATH [AS_SEQUENCE 64500] [AS_CONFED_SEQUENCE 65001]
// [AS_SEQUENCE 23456 23456] beside an AS4_PATH of two hops. The AS_PATH counts
// three AS numbers and the AS4_PATH two, so one leading AS number (64500) is
// prepended, and the confederation segment adjacent to it must follow it into
// the reconstructed path. A second input puts a two-hop segment between the
// prepended hop and the confederation segment, so the confederation segment is
// adjacent only to a segment that is not prepended, and must be dropped.
//
// RFC requirement: RFC6793-4.2.3-10 positive -- an AS_CONFED_SEQUENCE that is not the leading segment but is adjacent to the prepended AS_SEQUENCE 64500 is prepended with it.
// RFC requirement: RFC6793-4.2.3-10 negative -- the same AS_CONFED_SEQUENCE placed after an AS_SEQUENCE that is not prepended is not prepended.
func TestRFC6793ConfedSegmentAdjacentToPrependedIsPrepended(t *testing.T) {
	adjacent := []byte{
		0x40, 0x02, 14,
		0x02, 0x01, 0xFB, 0xF4, // AS_SEQUENCE 64500
		0x03, 0x01, 0xFD, 0xE9, // AS_CONFED_SEQUENCE 65001
		0x02, 0x02, 0x5B, 0xA0, 0x5B, 0xA0, // AS_SEQUENCE AS_TRANS AS_TRANS
	}
	got := reconcileSection(t, concatAttrs(wireOriginIGP, adjacent, wireAS4PathTwoHops), false)
	want := []byte{
		0x02, 0x01, 0x00, 0x00, 0xFB, 0xF4,
		0x03, 0x01, 0x00, 0x00, 0xFD, 0xE9,
		0x02, 0x02, 0x00, 0x03, 0x0B, 0x64, 0x00, 0x03, 0x0B, 0x65,
	}
	if !bytes.Equal(got.ASPath, want) {
		t.Fatalf("adjacent confederation segment: AS path %x, want %x", got.ASPath, want)
	}

	separated := []byte{
		0x40, 0x02, 18,
		0x02, 0x01, 0xFB, 0xF4, // AS_SEQUENCE 64500 (prepended)
		0x02, 0x02, 0x5B, 0xA0, 0x5B, 0xA0, // AS_SEQUENCE AS_TRANS AS_TRANS (not prepended)
		0x03, 0x01, 0xFD, 0xE9, // AS_CONFED_SEQUENCE 65001
		0x02, 0x01, 0x5B, 0xA0, // AS_SEQUENCE AS_TRANS
	}
	threeHops := []byte{
		0xC0, 0x11, 14,
		0x02, 0x03, 0x00, 0x03, 0x0B, 0x64, 0x00, 0x03, 0x0B, 0x65, 0x00, 0x03, 0x0B, 0x66,
	}
	got = reconcileSection(t, concatAttrs(wireOriginIGP, separated, threeHops), false)
	want = []byte{
		0x02, 0x01, 0x00, 0x00, 0xFB, 0xF4,
		0x02, 0x03, 0x00, 0x03, 0x0B, 0x64, 0x00, 0x03, 0x0B, 0x65, 0x00, 0x03, 0x0B, 0x66,
	}
	if !bytes.Equal(got.ASPath, want) {
		t.Fatalf("separated confederation segment: AS path %x, want %x", got.ASPath, want)
	}
}
