package packet

import "testing"

// VALIDATES: WriteEcho and ParseEcho round-trip all fields.
// PREVENTS: endian drift in the 16-byte envelope.
func TestEchoRoundTrip(t *testing.T) {
	want := Echo{
		LocalDiscriminator: 0xCAFEBABE,
		Sequence:           0x01020304,
		TimestampMs:        0xDEADBEEF,
	}
	buf := make([]byte, EchoLen)
	if n := WriteEcho(buf, 0, want); n != EchoLen {
		t.Fatalf("WriteEcho n=%d, want %d", n, EchoLen)
	}
	got, err := ParseEcho(buf)
	if err != nil {
		t.Fatalf("ParseEcho: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// VALIDATES: ParseEcho rejects short buffers.
// PREVENTS: slice out-of-range on adversarial input.
func TestEchoShort(t *testing.T) {
	if _, err := ParseEcho(make([]byte, 10)); err == nil {
		t.Fatal("ParseEcho short returned nil")
	}
}

// VALIDATES: ParseEcho rejects wrong-magic packets.
// PREVENTS: the engine mistaking random UDP 3785 traffic for an
// echo reflection.
func TestEchoBadMagic(t *testing.T) {
	buf := make([]byte, EchoLen)
	copy(buf[:4], "XEEC")
	if _, err := ParseEcho(buf); err == nil {
		t.Fatal("ParseEcho bad magic returned nil")
	}
}
