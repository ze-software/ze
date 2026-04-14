package l2tp

import (
	"bytes"
	"errors"
	"testing"
)

// TestAVPIteratorBasic validates AC-7: iterator yields the AVPs in order.
func TestAVPIteratorBasic(t *testing.T) {
	buf := make([]byte, 64)
	off := 0
	off += WriteAVPUint16(buf, off, true, AVPMessageType, uint16(MsgSCCRQ))
	// Protocol Version
	buf[off+AVPHeaderLen] = 0x01
	buf[off+AVPHeaderLen+1] = 0x00
	off += WriteAVPBytes(buf, off, true, 0, AVPProtocolVersion, ProtocolVersionValue[:])
	off += WriteAVPString(buf, off, true, AVPHostName, "lac-01")

	it := NewAVPIterator(buf[:off])
	var types []AVPType
	for {
		_, at, flags, val, ok := it.Next()
		if !ok {
			break
		}
		if flags&FlagMandatory == 0 {
			t.Fatalf("expected M=1 for %d", at)
		}
		if at == AVPHostName && string(val) != "lac-01" {
			t.Fatalf("host name: got %q", val)
		}
		types = append(types, at)
	}
	if err := it.Err(); err != nil {
		t.Fatalf("iter err: %v", err)
	}
	want := []AVPType{AVPMessageType, AVPProtocolVersion, AVPHostName}
	if len(types) != len(want) {
		t.Fatalf("count: got %v want %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("order: got %v want %v", types, want)
		}
	}
}

// TestAVPIteratorShortLength validates AC-8.
func TestAVPIteratorShortLength(t *testing.T) {
	// Length field = 4 (< 6).
	buf := []byte{0x80, 0x04, 0, 0, 0, 0}
	it := NewAVPIterator(buf)
	if _, _, _, _, ok := it.Next(); ok {
		t.Fatalf("expected iterator to reject Length < 6")
	}
	if !errors.Is(it.Err(), ErrInvalidAVPLen) {
		t.Fatalf("Err: %v", it.Err())
	}
}

// TestAVPIteratorTruncatedValue validates AC-9.
func TestAVPIteratorTruncatedValue(t *testing.T) {
	// AVP declares Length=10 but only 8 bytes present.
	buf := []byte{0x80, 0x0A, 0, 0, 0, 0, 0, 0}
	it := NewAVPIterator(buf)
	if _, _, _, _, ok := it.Next(); ok {
		t.Fatalf("expected rejection")
	}
	if !errors.Is(it.Err(), ErrInvalidAVPLen) {
		t.Fatalf("Err: %v", it.Err())
	}
}

// TestAVPIteratorEmptyValueExhaustion validates boundary: a payload that
// is exactly one 6-byte header-only AVP yields one iteration and then
// Remaining()==0.
// VALIDATES: iterator offset advances exactly length bytes; value view of
// zero-length AVP is an empty (non-nil) slice.
// PREVENTS: off-by-one that would re-yield the same AVP or trip on the
// exact-exhaustion boundary.
func TestAVPIteratorEmptyValueExhaustion(t *testing.T) {
	// M=1, Length=6, vendor=0, attrType=39 (Sequencing Required — no value).
	buf := []byte{0x80, 0x06, 0x00, 0x00, 0x00, 0x27}
	it := NewAVPIterator(buf)
	_, at, _, val, ok := it.Next()
	if !ok {
		t.Fatalf("first Next: err=%v", it.Err())
	}
	if at != AVPSequencingRequired {
		t.Fatalf("attr: got %d want %d", at, AVPSequencingRequired)
	}
	if len(val) != 0 {
		t.Fatalf("value len: got %d want 0", len(val))
	}
	if it.Remaining() != 0 {
		t.Fatalf("Remaining after consume: got %d want 0", it.Remaining())
	}
	if _, _, _, _, ok := it.Next(); ok {
		t.Fatalf("second Next should return ok=false (exhausted)")
	}
	if err := it.Err(); err != nil {
		t.Fatalf("clean exhaustion should not set Err, got %v", err)
	}
}

// TestAVPIteratorReservedBits validates AC-10: reserved bit exposed in flags.
func TestAVPIteratorReservedBits(t *testing.T) {
	// Reserved bit 2 set: 0x2008 (len=8, reserved bit high in first nibble).
	// Actually bits 2-5 of the first octet are represented in the top word as
	// 0x3C00 mask. Set one reserved bit: 0x2000 (bit 2 of the first byte).
	buf := []byte{0x20, 0x08, 0, 0, 0, 0, 0, 0}
	it := NewAVPIterator(buf)
	_, _, flags, _, ok := it.Next()
	if !ok {
		t.Fatalf("expected success, got err=%v", it.Err())
	}
	if flags&FlagReserved == 0 {
		t.Fatalf("FlagReserved not set: %v", flags)
	}
}

// TestWriteAVPUint16 validates AC-11.
func TestWriteAVPUint16(t *testing.T) {
	buf := make([]byte, 8)
	n := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(MsgSCCRP))
	if n != 8 {
		t.Fatalf("n=%d want 8", n)
	}
	// M=1 (0x80) | length 8 (0x008) => byte0=0x80, byte1=0x08
	want := []byte{0x80, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02}
	if !bytes.Equal(buf, want) {
		t.Fatalf("bytes:\n got  %x\n want %x", buf, want)
	}
}

// TestWriteAVPEmptyString validates AC-12.
func TestWriteAVPEmptyString(t *testing.T) {
	buf := make([]byte, 6)
	n := WriteAVPString(buf, 0, true, AVPHostName, "")
	if n != 6 {
		t.Fatalf("n=%d want 6", n)
	}
	want := []byte{0x80, 0x06, 0x00, 0x00, 0x00, 0x07}
	if !bytes.Equal(buf, want) {
		t.Fatalf("bytes:\n got  %x\n want %x", buf, want)
	}
}

// TestWriteAVPMaxBytes validates AC-13: 1023-byte max AVP.
func TestWriteAVPMaxBytes(t *testing.T) {
	value := make([]byte, 1017) // 1017 + 6 = 1023
	for i := range value {
		value[i] = byte(i)
	}
	buf := make([]byte, 2048)
	n := WriteAVPBytes(buf, 0, true, 0, AVPVendorName, value)
	if n != 1023 {
		t.Fatalf("n=%d want 1023", n)
	}
	// Length low 10 bits must equal 1023.
	word := uint16(buf[0])<<8 | uint16(buf[1])
	if word&0x03FF != 1023 {
		t.Fatalf("length field: %d want 1023", word&0x03FF)
	}
	// Round-trip through iterator.
	it := NewAVPIterator(buf[:n])
	_, at, _, v, ok := it.Next()
	if !ok {
		t.Fatalf("iter err: %v", it.Err())
	}
	if at != AVPVendorName || !bytes.Equal(v, value) {
		t.Fatalf("round-trip mismatch (%d bytes)", len(v))
	}
}

// TestAVPCatalogRoundTrip validates AC-15: every catalog AVP round-trips.
func TestAVPCatalogRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		enc  func(buf []byte, off int) int
		want struct {
			attr AVPType
			size int
		}
	}{
		{"MessageType u16", func(b []byte, o int) int { return WriteAVPUint16(b, o, true, AVPMessageType, 1) }, struct {
			attr AVPType
			size int
		}{AVPMessageType, 8}},
		{"ProtocolVersion bytes", func(b []byte, o int) int {
			return WriteAVPBytes(b, o, true, 0, AVPProtocolVersion, ProtocolVersionValue[:])
		}, struct {
			attr AVPType
			size int
		}{AVPProtocolVersion, 8}},
		{"FramingCap u32", func(b []byte, o int) int { return WriteAVPUint32(b, o, true, AVPFramingCapabilities, 0x03) }, struct {
			attr AVPType
			size int
		}{AVPFramingCapabilities, 10}},
		{"TieBreaker u64", func(b []byte, o int) int { return WriteAVPUint64(b, o, false, AVPTieBreaker, 0xdeadbeefcafef00d) }, struct {
			attr AVPType
			size int
		}{AVPTieBreaker, 14}},
		{"HostName string", func(b []byte, o int) int { return WriteAVPString(b, o, true, AVPHostName, "h") }, struct {
			attr AVPType
			size int
		}{AVPHostName, 7}},
		{"AssignedTID u16", func(b []byte, o int) int { return WriteAVPUint16(b, o, true, AVPAssignedTunnelID, 0x1234) }, struct {
			attr AVPType
			size int
		}{AVPAssignedTunnelID, 8}},
		{"SequencingRequired empty", func(b []byte, o int) int { return WriteAVPEmpty(b, o, true, 0, AVPSequencingRequired) }, struct {
			attr AVPType
			size int
		}{AVPSequencingRequired, 6}},
		{"ProxyAuthenID", func(b []byte, o int) int {
			return WriteAVPProxyAuthenID(b, o, false, ProxyAuthenIDValue{ChapID: 7})
		}, struct {
			attr AVPType
			size int
		}{AVPProxyAuthenID, 8}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, 128)
			n := tc.enc(buf, 0)
			if n != tc.want.size {
				t.Fatalf("n=%d want %d", n, tc.want.size)
			}
			it := NewAVPIterator(buf[:n])
			_, at, _, _, ok := it.Next()
			if !ok {
				t.Fatalf("iter err: %v", it.Err())
			}
			if at != tc.want.attr {
				t.Fatalf("attr: %d want %d", at, tc.want.attr)
			}
		})
	}
}

// TestResultCodeRoundTrip validates the compound Result Code AVP shape.
func TestResultCodeRoundTrip(t *testing.T) {
	cases := []ResultCodeValue{
		{Result: 1}, // result only
		{Result: 2, Error: 4, ErrorPresent: true},                                        // result + error
		{Result: 2, Error: 4, ErrorPresent: true, Message: "nope", MessagePresent: true}, // full
	}
	for _, tc := range cases {
		buf := make([]byte, 64)
		n := WriteAVPResultCode(buf, 0, true, tc)
		it := NewAVPIterator(buf[:n])
		_, at, _, v, ok := it.Next()
		if !ok || at != AVPResultCode {
			t.Fatalf("iter err: %v at=%d", it.Err(), at)
		}
		got, err := ReadResultCode(v)
		if err != nil {
			t.Fatalf("ReadResultCode: %v", err)
		}
		if got != tc {
			t.Fatalf("got %+v want %+v", got, tc)
		}
	}
}

// TestQ931CauseRoundTrip validates the Q.931 compound AVP.
func TestQ931CauseRoundTrip(t *testing.T) {
	v := Q931CauseValue{Cause: 17, Msg: 0x88, Advisory: "busy", AdvisoryPresent: true}
	buf := make([]byte, 32)
	n := WriteAVPQ931Cause(buf, 0, true, v)
	it := NewAVPIterator(buf[:n])
	_, at, _, val, ok := it.Next()
	if !ok || at != AVPQ931CauseCode {
		t.Fatalf("iter err: %v at=%d", it.Err(), at)
	}
	got, err := ReadQ931Cause(val)
	if err != nil {
		t.Fatalf("ReadQ931Cause: %v", err)
	}
	if got != v {
		t.Fatalf("got %+v want %+v", got, v)
	}
}

// TestCallErrorsRoundTrip validates the fixed 26-byte layout.
func TestCallErrorsRoundTrip(t *testing.T) {
	v := CallErrorsValue{
		CRCErrors: 1, FramingErrors: 2, HardwareOverruns: 3,
		BufferOverruns: 4, TimeoutErrors: 5, AlignmentErrors: 6,
	}
	buf := make([]byte, 40)
	n := WriteAVPCallErrors(buf, 0, true, v)
	if n != 32 { // 6 header + 26 value
		t.Fatalf("n=%d want 32", n)
	}
	it := NewAVPIterator(buf[:n])
	_, at, _, val, ok := it.Next()
	if !ok || at != AVPCallErrors {
		t.Fatalf("at=%d ok=%v err=%v", at, ok, it.Err())
	}
	got, err := ReadCallErrors(val)
	if err != nil {
		t.Fatalf("ReadCallErrors: %v", err)
	}
	if got != v {
		t.Fatalf("got %+v want %+v", got, v)
	}
}

// TestACCMRoundTrip validates the fixed 10-byte layout.
func TestACCMRoundTrip(t *testing.T) {
	v := ACCMValue{SendACCM: 0xAABBCCDD, RecvACCM: 0x11223344}
	buf := make([]byte, 32)
	n := WriteAVPACCM(buf, 0, true, v)
	if n != 16 {
		t.Fatalf("n=%d want 16", n)
	}
	it := NewAVPIterator(buf[:n])
	_, at, _, val, ok := it.Next()
	if !ok || at != AVPACCM {
		t.Fatalf("at=%d ok=%v err=%v", at, ok, it.Err())
	}
	got, err := ReadACCM(val)
	if err != nil {
		t.Fatalf("ReadACCM: %v", err)
	}
	if got != v {
		t.Fatalf("got %+v want %+v", got, v)
	}
}

// TestProtocolVersionValue validates the constant's wire bytes.
func TestProtocolVersionValue(t *testing.T) {
	if ProtocolVersionValue != [2]byte{0x01, 0x00} {
		t.Fatalf("wrong bytes: %x", ProtocolVersionValue)
	}
}
