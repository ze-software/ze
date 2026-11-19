package l2tp

import (
	"bytes"
	"io"
	"log/slog"
	"net/netip"
	"testing"
)

// FuzzRevealHiddenAVPs feeds arbitrary control-message bodies to the reveal
// that runs before every parser, with and without a secret. It never panics,
// never grows the body (a revealed AVP is shorter than its hidden form), and
// returns a body with no hidden AVP unchanged.
func FuzzRevealHiddenAVPs(f *testing.F) {
	var seed [64]byte
	off := WriteAVPUint16(seed[:], 0, true, AVPMessageType, uint16(MsgICRQ))
	off += WriteAVPBytes(seed[:], off, true, 0, AVPRandomVector, []byte("0123456789abcdef"))
	ct := HiddenEncrypt(seed[off+AVPHeaderLen:], AVPCallingNumber, []byte("s"), []byte("0123456789abcdef"), []byte("5550100"))
	WriteAVPHeader(seed[:], off, FlagMandatory|FlagHidden, 0, AVPCallingNumber, AVPHeaderLen+len(ct))
	off += AVPHeaderLen + len(ct)
	f.Add(seed[:off], "s")
	f.Add(seed[:off], "")
	f.Add([]byte{0xC0, 0x06, 0, 0, 0, 22}, "s")
	f.Add([]byte{0xC0, 0x08, 0, 0, 0, 22, 0xFF, 0xFF}, "s")
	f.Add([]byte{0xC0, 0x05, 0, 0}, "s")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	f.Fuzz(func(t *testing.T, body []byte, secret string) {
		in := bytes.Clone(body)
		out, left := revealHiddenAVPs(body, secret)
		left.log(logger, netip.MustParseAddrPort("192.0.2.1:1701"))
		if len(out) > len(body) {
			t.Fatalf("revealed body grew from %d to %d octets", len(body), len(out))
		}
		if !bytes.Equal(body, in) {
			t.Fatal("revealHiddenAVPs wrote the received body")
		}
		if !carriesHiddenAVP(body) && !bytes.Equal(out, body) {
			t.Fatal("a body with no hidden AVP came back changed")
		}
	})
}

// FuzzHiddenDecrypt ensures HiddenDecrypt never panics or reads out of bounds
// on arbitrary ciphertext input. Correctness of the recovered plaintext is
// covered by TestHidden* round-trip tests.
func FuzzHiddenDecrypt(f *testing.F) {
	seed := [][]byte{
		{},
		{0, 0},
		{0, 0x0A, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		make([]byte, 16),
		make([]byte, 40),
		// accel-ppp-ng a8ca0f3 (T8439): hidden value of one octet, and
		// ciphertexts whose decrypted Original Length is 0xFFFF or 1.
		{0x00},
		{0xFF, 0xFF, 1, 2},
		{0x00, 0x01},
	}
	for _, s := range seed {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, ct []byte) {
		dst := make([]byte, len(ct))
		// Either error or successful extraction, never a panic.
		if _, err := HiddenDecrypt(dst, AVPHostName, []byte("s"), []byte("rv"), ct); err != nil {
			return // error outcome is acceptable; no-panic is the invariant.
		}
	})
}
