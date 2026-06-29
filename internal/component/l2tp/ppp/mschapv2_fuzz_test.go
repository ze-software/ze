package ppp

import (
	"bytes"
	"testing"
)

// FuzzParseMSCHAPv2Response exercises ParseMSCHAPv2Response against
// arbitrary input to catch panics and out-of-bounds reads. Required by
// .claude/rules/tdd.md "Fuzz (MANDATORY for wire format)".
func FuzzParseMSCHAPv2Response(f *testing.F) {
	// Build a valid minimal Response: Identifier=0, Peer-Challenge=0x01
	// x16, Reserved zero, NT-Response=0x02 x24, Flags=0, empty Name.
	peerChallenge := bytes.Repeat([]byte{0x01}, mschapv2PeerChallengeLen)
	ntResponse := bytes.Repeat([]byte{0x02}, mschapv2NTResponseLen)
	valid := buildMSCHAPv2ResponsePayload(0, peerChallenge, ntResponse, "")
	validWithName := buildMSCHAPv2ResponsePayload(0x42, peerChallenge, ntResponse, "alice")

	// A buffer where Reserved has a stray bit set.
	reservedNonZero := bytes.Clone(valid)
	reservedNonZero[21] = 0x80

	// A buffer where Flags has a stray bit set.
	flagsNonZero := bytes.Clone(valid)
	flagsNonZero[4+1+mschapv2PeerChallengeLen+mschapv2ReservedLen+mschapv2NTResponseLen] = 0x01

	// Value-Size variants.
	vs48 := bytes.Clone(valid)
	vs48[4] = 48
	vs50 := bytes.Clone(valid)
	vs50[4] = 50
	vs255 := bytes.Clone(valid)
	vs255[4] = 0xFF
	vs0 := bytes.Clone(valid)
	vs0[4] = 0

	seeds := [][]byte{
		nil,
		valid,
		validWithName,
		// Below minimum length.
		{MSCHAPv2CodeResponse, 0x00, 0x00, 0x04},
		// Length far beyond buffer.
		{MSCHAPv2CodeResponse, 0x00, 0xFF, 0xFF, byte(mschapv2ResponseValueLen)},
		// Wrong code.
		{MSCHAPv2CodeChallenge, 0x00, 0x00, byte(mschapv2HeaderLen + 1 + mschapv2ResponseValueLen),
			byte(mschapv2ResponseValueLen)},
		vs0,
		vs48,
		vs50,
		vs255,
		reservedNonZero,
		flagsNonZero,
		// Max-frame-sized buffer of zeros.
		make([]byte, MaxFrameLen-2),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		resp, err := ParseMSCHAPv2Response(data)
		if err != nil {
			return
		}
		// Invariant 1: declared Length reachable and covers the parsed
		// body exactly.
		if len(data) < mschapv2HeaderLen {
			t.Errorf("parse succeeded on buf too short for header: len=%d", len(data))
			return
		}
		length := int(data[2])<<8 | int(data[3])
		minLength := mschapv2HeaderLen + 1 + mschapv2ResponseValueLen
		if length < minLength {
			t.Errorf("parse succeeded with Length %d below minimum %d",
				length, minLength)
			return
		}
		// On a successful parse: body covers header + VS(1) + Value(49)
		// + Name.
		bodyLen := 1 + mschapv2ResponseValueLen + len(resp.Name)
		if mschapv2HeaderLen+bodyLen != length {
			t.Errorf("parsed fields (%d body bytes) do not cover Length %d (diff %d)",
				bodyLen, length, length-mschapv2HeaderLen-bodyLen)
			return
		}
		// Invariant 2: Value-Size must have been 49.
		if int(data[mschapv2HeaderLen]) != mschapv2ResponseValueLen {
			t.Errorf("accepted Value-Size %d, want %d",
				data[mschapv2HeaderLen], mschapv2ResponseValueLen)
			return
		}
		// Invariant 2a: Identifier must be copied verbatim from wire
		// byte 1 (guards against refactors that swap the field with a
		// constant or with Code).
		if resp.Identifier != data[1] {
			t.Errorf("Identifier = 0x%02x, wire byte 1 = 0x%02x",
				resp.Identifier, data[1])
			return
		}
		// Invariant 3: Reserved bytes were all zero on the wire.
		reservedOff := mschapv2HeaderLen + 1 + mschapv2PeerChallengeLen
		for i := range mschapv2ReservedLen {
			if data[reservedOff+i] != 0 {
				t.Errorf("accepted non-zero Reserved[%d] = 0x%02x",
					i, data[reservedOff+i])
				return
			}
		}
		// Invariant 4: Flags byte was zero on the wire.
		flagsOff := reservedOff + mschapv2ReservedLen + mschapv2NTResponseLen
		if data[flagsOff] != 0 {
			t.Errorf("accepted non-zero Flags = 0x%02x", data[flagsOff])
			return
		}
		// Invariant 5: parsed Peer-Challenge matches wire bytes.
		peerChallengeOff := mschapv2HeaderLen + 1
		for i, b := range resp.PeerChallenge {
			if data[peerChallengeOff+i] != b {
				t.Errorf("PeerChallenge[%d] = %02x, wire = %02x",
					i, b, data[peerChallengeOff+i])
				return
			}
		}
		// Invariant 6: parsed NT-Response matches wire bytes.
		ntResponseOff := reservedOff + mschapv2ReservedLen
		for i, b := range resp.NTResponse {
			if data[ntResponseOff+i] != b {
				t.Errorf("NTResponse[%d] = %02x, wire = %02x",
					i, b, data[ntResponseOff+i])
				return
			}
		}
		// Invariant 7: parsed Name matches wire bytes.
		nameOff := flagsOff + mschapv2FlagsLen
		nameEnd := nameOff + len(resp.Name)
		if nameEnd > length {
			t.Errorf("Name end %d exceeds Length %d", nameEnd, length)
			return
		}
		if string(data[nameOff:nameEnd]) != resp.Name {
			t.Errorf("Name %q does not match input bytes %q",
				resp.Name, string(data[nameOff:nameEnd]))
		}
	})
}
