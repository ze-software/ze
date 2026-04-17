package ppp

import (
	"errors"
	"testing"
	"time"
)

// VALIDATES: ParseIPv6CPOptions decodes the Interface-Identifier option
//
//	(type 1, length 10, 8-byte value).
func TestIPv6CPParseOptions(t *testing.T) {
	buf := []byte{1, 10, 0x02, 0x00, 0x5E, 0xFF, 0xFE, 0x00, 0x12, 0x34}
	opts, err := ParseIPv6CPOptions(buf)
	if err != nil {
		t.Fatalf("ParseIPv6CPOptions: %v", err)
	}
	if !opts.HasInterfaceID {
		t.Fatalf("HasInterfaceID false, want true")
	}
	want := [8]byte{0x02, 0x00, 0x5E, 0xFF, 0xFE, 0x00, 0x12, 0x34}
	if opts.InterfaceID != want {
		t.Errorf("InterfaceID = %x, want %x", opts.InterfaceID, want)
	}
}

// VALIDATES: WriteIPv6CPOptions + ParseIPv6CPOptions round-trip
//
//	preserves the 8-byte identifier.
func TestIPv6CPRoundtrip(t *testing.T) {
	src := IPv6CPOptions{
		InterfaceID:    [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		HasInterfaceID: true,
	}
	buf := make([]byte, 16)
	n := WriteIPv6CPOptions(buf, 0, src)
	if n != 10 {
		t.Fatalf("wrote %d bytes, want 10", n)
	}
	got, err := ParseIPv6CPOptions(buf[:n])
	if err != nil {
		t.Fatalf("ParseIPv6CPOptions: %v", err)
	}
	if got != src {
		t.Errorf("roundtrip mismatch: got %x want %x", got.InterfaceID, src.InterfaceID)
	}
}

// VALIDATES: ParseIPv6CPOptions rejects wrong option length, truncated
//
//	buffer, header-under-minimum.
func TestIPv6CPParseRejects(t *testing.T) {
	cases := []struct {
		name    string
		buf     []byte
		wantErr error
	}{
		{"too short", []byte{1}, errOptionTooShort},
		{"len below header", []byte{1, 1}, errOptionLengthMismatch},
		{"len 9 (short)", []byte{1, 9, 1, 2, 3, 4, 5, 6, 7}, errIPv6CPBadOptionLen},
		{"len 11 (long)", []byte{1, 11, 1, 2, 3, 4, 5, 6, 7, 8, 9}, errIPv6CPBadOptionLen},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseIPv6CPOptions(tc.buf)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// VALIDATES: isValidIPv6CPInterfaceID rejects all-zero and all-ones per
//
//	RFC 5072 §3.2 (all-zero forbidden; all-ones is meaningless).
func TestIPv6CPInterfaceIDValidity(t *testing.T) {
	var zero, ones [8]byte
	for i := range ones {
		ones[i] = 0xFF
	}
	good := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	if isValidIPv6CPInterfaceID(zero) {
		t.Error("all-zero accepted as valid")
	}
	if isValidIPv6CPInterfaceID(ones) {
		t.Error("all-ones accepted as valid")
	}
	if !isValidIPv6CPInterfaceID(good) {
		t.Error("good id rejected")
	}
}

// VALIDATES: generateIPv6CPInterfaceID produces a non-zero, non-all-ones
//
//	identifier across repeated draws (statistical confidence on an
//	untrusted RNG).
func TestGenerateIPv6CPInterfaceID(t *testing.T) {
	for i := range 32 {
		id, err := generateIPv6CPInterfaceID()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if !isValidIPv6CPInterfaceID(id) {
			t.Errorf("draw %d produced invalid id %x", i, id)
		}
	}
}

// VALIDATES: AC-9 -- IPv6CP's first CONFREQ proposes a locally-generated
//
//	8-byte Interface-Identifier.
//
// PREVENTS: ze shipping with an all-zero or fixed identifier.
func TestIPv6CPProposesInterfaceID(t *testing.T) {
	td := newNCPTestDriverCfg(t, StartSession{DisableIPCP: true})
	defer td.cleanup()

	pkt := td.readPeerNCPPacket(t, ProtoIPv6CP)
	if pkt.Code != LCPConfigureRequest {
		t.Fatalf("first IPv6CP code = %d, want CR", pkt.Code)
	}
	opts, err := ParseIPv6CPOptions(pkt.Data)
	if err != nil {
		t.Fatalf("ParseIPv6CPOptions: %v", err)
	}
	if !opts.HasInterfaceID {
		t.Fatalf("initial CR missing Interface-Identifier")
	}
	if !isValidIPv6CPInterfaceID(opts.InterfaceID) {
		t.Errorf("proposed identifier %x invalid (zero or all-ones)", opts.InterfaceID)
	}
	// Keep time referenced so the linter does not flag it unused on
	// platforms where every other time usage goes through the helper.
	_ = time.Now
}
