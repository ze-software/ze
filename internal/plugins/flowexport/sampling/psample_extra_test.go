// VALIDATES: parsePsampleMessage skips attribute types it does not recognize,
// leaves unset fields at zero when a message is partial, and surfaces a decoder
// error for a structurally malformed attribute buffer.
// PREVENTS: an unknown/future psample attribute derailing the parse, a partial
// sample being reported with stale field values, or a truncated genetlink
// message being accepted as a valid sample.

package sampling

import "testing"

func TestParsePsampleMessageUnknownAttr(t *testing.T) {
	var buf []byte
	// An attribute type outside the handled set (1,3,6,7) must be ignored.
	buf = appendNLA(buf, 99, uint32ToBytes(0xDEADBEEF))
	buf = appendNLA(buf, psampleAttrIIfIndex, uint32ToBytes(7))

	pkt, err := parsePsampleMessage(buf)
	if err != nil {
		t.Fatalf("parsePsampleMessage: %v", err)
	}
	if pkt.IfIndex != 7 {
		t.Errorf("IfIndex = %d, want 7 (unknown attr must not disturb parsing)", pkt.IfIndex)
	}
	if pkt.Rate != 0 || pkt.OrigSize != 0 || pkt.Header != nil {
		t.Errorf("unexpected non-zero fields from unknown attr: %+v", pkt)
	}
}

func TestParsePsampleMessagePartial(t *testing.T) {
	// Only the sample rate is present; every other field stays zero/nil.
	buf := appendNLA(nil, psampleAttrSampleRate, uint32ToBytes(1024))

	pkt, err := parsePsampleMessage(buf)
	if err != nil {
		t.Fatalf("parsePsampleMessage: %v", err)
	}
	if pkt.Rate != 1024 {
		t.Errorf("Rate = %d, want 1024", pkt.Rate)
	}
	if pkt.IfIndex != 0 || pkt.OrigSize != 0 || pkt.Header != nil {
		t.Errorf("partial message left non-zero fields: %+v", pkt)
	}
}

func TestParsePsampleMessageMalformed(t *testing.T) {
	// An NLA header claiming a length (0xffff) far larger than the remaining
	// bytes must produce a decode error, not a silently-empty sample.
	bad := []byte{0xff, 0xff, 0x01, 0x00, 0xde, 0xad, 0xbe, 0xef}
	if _, err := parsePsampleMessage(bad); err == nil {
		t.Error("expected a decode error for a malformed attribute buffer")
	}
}
