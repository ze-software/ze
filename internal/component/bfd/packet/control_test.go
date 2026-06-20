package packet

import (
	"bytes"
	"errors"
	"testing"
)

// VALIDATES: round-trip encoding for representative BFD Control packets.
// PREVENTS: regression where a flag, discriminator, or interval field is
// truncated or shifted in WriteTo/ParseControl.
func TestRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		c    Control
	}{
		{
			name: "down-no-flags",
			c: Control{
				Version:                   1,
				Diag:                      DiagNone,
				State:                     StateDown,
				DetectMult:                3,
				Length:                    MandatoryLen,
				MyDiscriminator:           0x12345678,
				YourDiscriminator:         0,
				DesiredMinTxInterval:      1_000_000,
				RequiredMinRxInterval:     1_000_000,
				RequiredMinEchoRxInterval: 0,
			},
		},
		{
			name: "init-poll",
			c: Control{
				Version:                   1,
				Diag:                      DiagNeighborSignaledDown,
				State:                     StateInit,
				Poll:                      true,
				DetectMult:                3,
				Length:                    MandatoryLen,
				MyDiscriminator:           0xCAFEBABE,
				YourDiscriminator:         0xDEADBEEF,
				DesiredMinTxInterval:      50_000,
				RequiredMinRxInterval:     50_000,
				RequiredMinEchoRxInterval: 0,
			},
		},
		{
			name: "up-final-cpi-demand",
			c: Control{
				Version:                   1,
				Diag:                      DiagNone,
				State:                     StateUp,
				Final:                     true,
				CPI:                       true,
				Demand:                    true,
				DetectMult:                5,
				Length:                    MandatoryLen,
				MyDiscriminator:           0x00000001,
				YourDiscriminator:         0xFFFFFFFE,
				DesiredMinTxInterval:      300_000,
				RequiredMinRxInterval:     300_000,
				RequiredMinEchoRxInterval: 100_000,
			},
		},
		{
			name: "admin-down-max-mult",
			c: Control{
				Version:                   1,
				Diag:                      DiagAdminDown,
				State:                     StateAdminDown,
				DetectMult:                255,
				Length:                    MandatoryLen,
				MyDiscriminator:           0x80000000,
				YourDiscriminator:         0x00000001,
				DesiredMinTxInterval:      1_000_000,
				RequiredMinRxInterval:     1_000_000,
				RequiredMinEchoRxInterval: 0,
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			pb := Acquire()
			defer Release(pb)
			buf := pb.Data()
			n := tt.c.WriteTo(buf, 0)
			if n != MandatoryLen {
				t.Fatalf("WriteTo returned %d, want %d", n, MandatoryLen)
			}
			got, consumed, err := ParseControl(buf[:MandatoryLen])
			if err != nil {
				t.Fatalf("ParseControl: %v", err)
			}
			if consumed != MandatoryLen {
				t.Fatalf("ParseControl consumed %d, want %d", consumed, MandatoryLen)
			}
			if got != tt.c {
				t.Fatalf("round trip mismatch:\ngot:  %+v\nwant: %+v", got, tt.c)
			}
		})
	}
}

// VALIDATES: WriteTo writes into the buffer at the requested offset and
// leaves earlier bytes untouched.
// PREVENTS: regression where the offset argument is ignored or misapplied.
func TestWriteToOffset(t *testing.T) {
	const prefix = 8
	pb := Acquire()
	defer Release(pb)
	buf := pb.Data()
	for i := range prefix {
		buf[i] = 0xAA
	}
	c := Control{
		Version:         1,
		State:           StateUp,
		DetectMult:      3,
		Length:          MandatoryLen,
		MyDiscriminator: 1,
	}
	n := c.WriteTo(buf, prefix)
	if n != MandatoryLen {
		t.Fatalf("WriteTo returned %d, want %d", n, MandatoryLen)
	}
	for i := range prefix {
		if buf[i] != 0xAA {
			t.Fatalf("byte %d clobbered: %#x", i, buf[i])
		}
	}
	got, _, err := ParseControl(buf[prefix : prefix+MandatoryLen])
	if err != nil {
		t.Fatalf("ParseControl: %v", err)
	}
	if got.MyDiscriminator != 1 || got.State != StateUp {
		t.Fatalf("decoded wrong: %+v", got)
	}
}

// VALIDATES: ParseControl rejects every structural error in RFC 5880
// Section 6.8.6 reception order.
// PREVENTS: regression where a malformed packet is silently accepted.
func TestParseControlRejection(t *testing.T) {
	good := Control{
		Version:                   1,
		State:                     StateUp,
		DetectMult:                3,
		Length:                    MandatoryLen,
		MyDiscriminator:           1,
		DesiredMinTxInterval:      1_000_000,
		RequiredMinRxInterval:     1_000_000,
		RequiredMinEchoRxInterval: 0,
	}
	mk := func(mut func(b []byte)) []byte {
		b := make([]byte, MandatoryLen)
		good.WriteTo(b, 0)
		mut(b)
		return b
	}

	cases := []struct {
		name string
		data []byte
		err  error
	}{
		{"short-packet", make([]byte, MandatoryLen-1), ErrShortPacket},
		{"bad-version", mk(func(b []byte) { b[0] = 2 << 5 }), ErrBadVersion},
		{"length-too-small", mk(func(b []byte) { b[3] = MandatoryLen - 1 }), ErrLengthTooSmall},
		{"length-overflow", mk(func(b []byte) { b[3] = MandatoryLen + 4 }), ErrLengthOverBuffer},
		{"zero-detect-mult", mk(func(b []byte) { b[2] = 0 }), ErrZeroDetectMult},
		{"multipoint-set", mk(func(b []byte) { b[1] |= FlagMultipoint }), ErrMultipointSet},
		{"zero-my-disc", mk(func(b []byte) {
			b[4], b[5], b[6], b[7] = 0, 0, 0, 0
		}), ErrZeroMyDisc},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseControl(tt.data)
			if !errors.Is(err, tt.err) {
				t.Fatalf("got err %v, want %v", err, tt.err)
			}
		})
	}
}

// VALIDATES: ParseControl rejects an A=1 packet whose Length field reflects
// only the mandatory section (24); the minimum is 26 because the auth
// section header itself is two bytes.
// PREVENTS: regression where the auth-bit minimum-length check is dropped.
func TestParseControlAuthMinimumLength(t *testing.T) {
	c := Control{
		Version:                   1,
		State:                     StateDown,
		Auth:                      true,
		DetectMult:                3,
		Length:                    MandatoryLen, // too small with A=1
		MyDiscriminator:           1,
		DesiredMinTxInterval:      1_000_000,
		RequiredMinRxInterval:     1_000_000,
		RequiredMinEchoRxInterval: 0,
	}
	buf := make([]byte, MandatoryLen)
	c.WriteTo(buf, 0)
	_, _, err := ParseControl(buf)
	if !errors.Is(err, ErrLengthTooSmall) {
		t.Fatalf("got err %v, want ErrLengthTooSmall", err)
	}
}

// VALIDATES: ParseControl accepts a valid auth-bearing packet whose Length
// covers a 24-byte auth body (Keyed MD5 layout) plus the mandatory section.
// PREVENTS: regression where ParseControl rejects legitimate authenticated
// packets.
func TestParseControlAuthHappyPath(t *testing.T) {
	const total = MandatoryLen + AuthLenKeyedMD5
	c := Control{
		Version:                   1,
		State:                     StateUp,
		Auth:                      true,
		DetectMult:                3,
		Length:                    total,
		MyDiscriminator:           42,
		YourDiscriminator:         24,
		DesiredMinTxInterval:      300_000,
		RequiredMinRxInterval:     300_000,
		RequiredMinEchoRxInterval: 0,
	}
	buf := make([]byte, total)
	c.WriteTo(buf, 0)
	// Append a fake Keyed MD5 auth section: type 2, len 24, then zeros.
	buf[MandatoryLen] = AuthTypeKeyedMD5
	buf[MandatoryLen+1] = AuthLenKeyedMD5

	got, _, err := ParseControl(buf)
	if err != nil {
		t.Fatalf("ParseControl: %v", err)
	}
	if !got.Auth {
		t.Fatal("expected Auth flag to be set")
	}
	if got.Length != total {
		t.Fatalf("length: got %d want %d", got.Length, total)
	}

	h, err := ParseAuth(buf[MandatoryLen:])
	if err != nil {
		t.Fatalf("ParseAuth: %v", err)
	}
	if h.Type != AuthTypeKeyedMD5 {
		t.Fatalf("auth type: got %d want %d", h.Type, AuthTypeKeyedMD5)
	}
	if h.Len != AuthLenKeyedMD5 {
		t.Fatalf("auth len: got %d want %d", h.Len, AuthLenKeyedMD5)
	}
	if len(h.Body) != AuthLenKeyedMD5-AuthHeaderLen {
		t.Fatalf("auth body length: got %d want %d", len(h.Body), AuthLenKeyedMD5-AuthHeaderLen)
	}
}

// VALIDATES: ParseAuth rejects authentication sections with invalid lengths.
// PREVENTS: an attacker shaping a too-large Auth Len from causing a buffer
// over-read in a future authenticator.
func TestParseAuthRejection(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		err  error
	}{
		{"empty", nil, ErrAuthShort},
		{"one-byte", []byte{0x02}, ErrAuthShort},
		{"len-zero", []byte{AuthTypeKeyedMD5, 0x00}, ErrAuthLenInvalid},
		{"len-one", []byte{AuthTypeKeyedMD5, 0x01}, ErrAuthLenInvalid},
		{"len-overflows", []byte{AuthTypeKeyedMD5, 0xFF, 0x00}, ErrAuthLenOverflow},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAuth(tt.data)
			if !errors.Is(err, tt.err) {
				t.Fatalf("got err %v, want %v", err, tt.err)
			}
		})
	}
}

// VALIDATES: every diagnostic and state value renders to a non-empty string
// without panicking.
// PREVENTS: future code that adds a Diag/State without a name silently
// printing as "" or panicking.
func TestStringers(t *testing.T) {
	for d := Diag(0); d <= Diag(31); d++ {
		s := d.String()
		if s == "" {
			t.Errorf("Diag(%d).String() empty", d)
		}
	}
	for s := State(0); s <= State(3); s++ {
		if s.String() == "" || s.String() == "invalid" {
			t.Errorf("State(%d).String() = %q", s, s.String())
		}
	}
}

// VALIDATES: ParseControl produces byte-for-byte stable wire output for a
// known reference packet (manually laid out from RFC 5880 Section 4.1).
// PREVENTS: silent re-ordering of fields between major refactors.
func TestKnownWire(t *testing.T) {
	want := []byte{
		// byte 0: ver=1, diag=1
		(1 << 5) | 1,
		// byte 1: state=Up (3), poll=1, final=0, cpi=0, auth=0, demand=0, mp=0
		(3 << 6) | FlagPoll,
		// byte 2: detect mult
		3,
		// byte 3: length
		MandatoryLen,
		// my discriminator: 0x01020304
		0x01, 0x02, 0x03, 0x04,
		// your discriminator: 0x05060708
		0x05, 0x06, 0x07, 0x08,
		// desired min tx interval: 1000000us = 0x000F4240
		0x00, 0x0F, 0x42, 0x40,
		// required min rx interval: 1000000us = 0x000F4240
		0x00, 0x0F, 0x42, 0x40,
		// required min echo rx interval: 0
		0x00, 0x00, 0x00, 0x00,
	}
	c := Control{
		Version:                   1,
		Diag:                      DiagControlDetectExpired,
		State:                     StateUp,
		Poll:                      true,
		DetectMult:                3,
		Length:                    MandatoryLen,
		MyDiscriminator:           0x01020304,
		YourDiscriminator:         0x05060708,
		DesiredMinTxInterval:      1_000_000,
		RequiredMinRxInterval:     1_000_000,
		RequiredMinEchoRxInterval: 0,
	}
	got := make([]byte, MandatoryLen)
	c.WriteTo(got, 0)
	if !bytes.Equal(got, want) {
		t.Fatalf("wire mismatch:\ngot:  % x\nwant: % x", got, want)
	}
}
