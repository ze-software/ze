package ppp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

// buildMSCHAPv2ResponsePayload constructs a valid MS-CHAPv2 Response
// payload (code 2) with the given Identifier, 16-byte Peer-Challenge,
// 24-byte NT-Response, and Name. Reserved and Flags are zero. Length
// is backfilled from the total size. Returned slice is the payload
// AFTER the PPP protocol header has been stripped (as ParseFrame
// produces).
func buildMSCHAPv2ResponsePayload(identifier uint8, peerChallenge, ntResponse []byte, name string) []byte {
	total := mschapv2HeaderLen + 1 + mschapv2ResponseValueLen + len(name)
	buf := make([]byte, total)
	buf[0] = MSCHAPv2CodeResponse
	buf[1] = identifier
	binary.BigEndian.PutUint16(buf[2:4], uint16(total))
	buf[4] = byte(mschapv2ResponseValueLen)
	copy(buf[5:5+mschapv2PeerChallengeLen], peerChallenge)
	// Reserved is already zero from make.
	ntOff := 5 + mschapv2PeerChallengeLen + mschapv2ReservedLen
	copy(buf[ntOff:ntOff+mschapv2NTResponseLen], ntResponse)
	// Flags is already zero.
	nameOff := ntOff + mschapv2NTResponseLen + mschapv2FlagsLen
	copy(buf[nameOff:], name)
	return buf
}

// VALIDATES: ParseMSCHAPv2Response decodes Code, Identifier, Length,
//
//	Value-Size, Peer-Challenge, Reserved (validated zero and
//	dropped), NT-Response, Flags (validated zero and dropped),
//	and Name per RFC 2759 Section 4.
//
// PREVENTS: regression where the 49-byte Response Value is miscounted
//
//	(e.g. Reserved folded into NT-Response) or the Name slice
//	runs past Length.
func TestMSCHAPv2ParseResponse(t *testing.T) {
	peerChallenge := bytes.Repeat([]byte{0x5A}, mschapv2PeerChallengeLen)
	ntResponse := make([]byte, mschapv2NTResponseLen)
	for i := range ntResponse {
		ntResponse[i] = byte(i + 1)
	}
	cases := []struct {
		name              string
		buf               []byte
		wantID            uint8
		wantPeerChallenge []byte
		wantNTResponse    []byte
		wantName          string
	}{
		{
			name:              "full response with name",
			buf:               buildMSCHAPv2ResponsePayload(0x42, peerChallenge, ntResponse, "bob"),
			wantID:            0x42,
			wantPeerChallenge: peerChallenge,
			wantNTResponse:    ntResponse,
			wantName:          "bob",
		},
		{
			name:              "empty name",
			buf:               buildMSCHAPv2ResponsePayload(0x01, peerChallenge, ntResponse, ""),
			wantID:            0x01,
			wantPeerChallenge: peerChallenge,
			wantNTResponse:    ntResponse,
			wantName:          "",
		},
		{
			name:              "domain-qualified name",
			buf:               buildMSCHAPv2ResponsePayload(0xFE, peerChallenge, ntResponse, `BIGCO\alice`),
			wantID:            0xFE,
			wantPeerChallenge: peerChallenge,
			wantNTResponse:    ntResponse,
			wantName:          `BIGCO\alice`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := ParseMSCHAPv2Response(tc.buf)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Identifier != tc.wantID {
				t.Errorf("Identifier = 0x%02x, want 0x%02x", resp.Identifier, tc.wantID)
			}
			if !bytes.Equal(resp.PeerChallenge, tc.wantPeerChallenge) {
				t.Errorf("PeerChallenge = %x, want %x", resp.PeerChallenge, tc.wantPeerChallenge)
			}
			if !bytes.Equal(resp.NTResponse, tc.wantNTResponse) {
				t.Errorf("NTResponse = %x, want %x", resp.NTResponse, tc.wantNTResponse)
			}
			if resp.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", resp.Name, tc.wantName)
			}
		})
	}
}

// VALIDATES: ParseMSCHAPv2Response rejects malformed packets per RFC 2759
//
//	Section 4: buffers shorter than the 4-byte header, wrong
//	code, Length mismatches, Value-Size != 49, and non-zero
//	Reserved / Flags octets.
func TestMSCHAPv2ParseResponseInvalid(t *testing.T) {
	validPC := bytes.Repeat([]byte{0x11}, mschapv2PeerChallengeLen)
	validNT := bytes.Repeat([]byte{0x22}, mschapv2NTResponseLen)
	valid := buildMSCHAPv2ResponsePayload(0x01, validPC, validNT, "u")

	cases := []struct {
		name string
		mut  func([]byte) []byte
		want error
	}{
		{
			name: "too short",
			mut:  func(_ []byte) []byte { return []byte{MSCHAPv2CodeResponse, 0x00, 0x00} },
			want: errMSCHAPv2TooShort,
		},
		{
			name: "wrong code challenge",
			mut: func(b []byte) []byte {
				out := bytes.Clone(b)
				out[0] = MSCHAPv2CodeChallenge
				return out
			},
			want: errMSCHAPv2WrongCode,
		},
		{
			name: "wrong code success",
			mut: func(b []byte) []byte {
				out := bytes.Clone(b)
				out[0] = MSCHAPv2CodeSuccess
				return out
			},
			want: errMSCHAPv2WrongCode,
		},
		{
			name: "length below minimum response body",
			mut: func(_ []byte) []byte {
				// 4-byte header claims Length=5 (no room for
				// Value-Size + 49-byte Value).
				return []byte{MSCHAPv2CodeResponse, 0x00, 0x00, 0x05, 0x00}
			},
			want: errMSCHAPv2LengthMismatch,
		},
		{
			name: "length exceeds buffer",
			mut: func(b []byte) []byte {
				out := bytes.Clone(b)
				binary.BigEndian.PutUint16(out[2:4], uint16(len(out)+10))
				return out
			},
			want: errMSCHAPv2LengthMismatch,
		},
		{
			name: "Value-Size zero",
			mut: func(b []byte) []byte {
				out := bytes.Clone(b)
				out[4] = 0
				return out
			},
			want: errMSCHAPv2ValueSizeWrong,
		},
		{
			name: "Value-Size 48 (one below)",
			mut: func(b []byte) []byte {
				out := bytes.Clone(b)
				out[4] = 48
				return out
			},
			want: errMSCHAPv2ValueSizeWrong,
		},
		{
			name: "Value-Size 50 (one above)",
			mut: func(b []byte) []byte {
				out := bytes.Clone(b)
				out[4] = 50
				return out
			},
			want: errMSCHAPv2ValueSizeWrong,
		},
		{
			name: "Value-Size 255",
			mut: func(b []byte) []byte {
				out := bytes.Clone(b)
				out[4] = 0xFF
				return out
			},
			want: errMSCHAPv2ValueSizeWrong,
		},
		{
			name: "Reserved non-zero (first octet)",
			mut: func(b []byte) []byte {
				out := bytes.Clone(b)
				// Reserved starts at body offset 1+16 = 17,
				// wire offset 4+17 = 21.
				out[21] = 0x01
				return out
			},
			want: errMSCHAPv2ReservedNonZero,
		},
		{
			name: "Reserved non-zero (last octet)",
			mut: func(b []byte) []byte {
				out := bytes.Clone(b)
				// Reserved last octet at body 1+16+7 = 24,
				// wire offset 4+24 = 28.
				out[28] = 0x80
				return out
			},
			want: errMSCHAPv2ReservedNonZero,
		},
		{
			name: "Flags non-zero",
			mut: func(b []byte) []byte {
				out := bytes.Clone(b)
				// Flags at body offset 1+16+8+24 = 49,
				// wire offset 4+49 = 53.
				out[53] = 0x01
				return out
			},
			want: errMSCHAPv2FlagsNonZero,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := tc.mut(valid)
			_, err := ParseMSCHAPv2Response(buf)
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// VALIDATES: ParseMSCHAPv2Response accepts Value-Size == 49 and rejects
//
//	48 / 50. RFC 2759 Section 4 fixes the Response Value at
//	exactly 49 octets.
//
// PREVENTS: regression where an off-by-one bounds check accepts 48 or
//
//	50.
func TestMSCHAPv2ParseResponseBoundary49(t *testing.T) {
	pc := bytes.Repeat([]byte{0x33}, mschapv2PeerChallengeLen)
	nt := bytes.Repeat([]byte{0x44}, mschapv2NTResponseLen)
	good := buildMSCHAPv2ResponsePayload(0x20, pc, nt, "")
	if _, err := ParseMSCHAPv2Response(good); err != nil {
		t.Fatalf("Value-Size=49 rejected: %v", err)
	}

	low := bytes.Clone(good)
	low[4] = 48
	if _, err := ParseMSCHAPv2Response(low); !errors.Is(err, errMSCHAPv2ValueSizeWrong) {
		t.Errorf("Value-Size=48 err = %v, want errMSCHAPv2ValueSizeWrong", err)
	}

	high := bytes.Clone(good)
	high[4] = 50
	if _, err := ParseMSCHAPv2Response(high); !errors.Is(err, errMSCHAPv2ValueSizeWrong) {
		t.Errorf("Value-Size=50 err = %v, want errMSCHAPv2ValueSizeWrong", err)
	}
}

// VALIDATES: ParseMSCHAPv2Response accepts the maximum legal Name
//
//	(Length = MaxFrameLen - 2; Name runs from byte 54 to Length).
//
// PREVENTS: regression where an off-by-one in the Name slice bound
//
//	drops the last byte at the high end of the legal Length
//	space.
func TestMSCHAPv2ParseResponseMaxName(t *testing.T) {
	length := MaxFrameLen - 2
	// Layout: header (4) + VS (1) + Value (49) + Name.
	headerLen := mschapv2HeaderLen + 1 + mschapv2ResponseValueLen
	nameLen := length - headerLen
	buf := make([]byte, length)
	buf[0] = MSCHAPv2CodeResponse
	buf[1] = 0x55
	binary.BigEndian.PutUint16(buf[2:4], uint16(length))
	buf[4] = byte(mschapv2ResponseValueLen)
	// Reserved / Flags remain zero. Fill Peer-Challenge + NT-Response
	// with 0xEE so we can assert the parser copied the right region.
	for i := range mschapv2PeerChallengeLen {
		buf[5+i] = 0xEE
	}
	ntOff := 5 + mschapv2PeerChallengeLen + mschapv2ReservedLen
	for i := range mschapv2NTResponseLen {
		buf[ntOff+i] = 0xEE
	}
	for i := range nameLen {
		buf[headerLen+i] = 'N'
	}
	resp, err := ParseMSCHAPv2Response(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Name) != nameLen {
		t.Errorf("Name length = %d, want %d", len(resp.Name), nameLen)
	}
	if resp.Name != "" && (resp.Name[0] != 'N' || resp.Name[nameLen-1] != 'N') {
		t.Errorf("Name first/last = %q/%q", resp.Name[0], resp.Name[nameLen-1])
	}
}

// VALIDATES: WriteMSCHAPv2Challenge encodes Code=1, Identifier, Length,
//
//	Value-Size=16, Value bytes, and Name per RFC 2759 Section 4.
//
// PREVENTS: regression where Value-Size differs from 16 or Length
//
//	excludes the Name octets.
func TestMSCHAPv2WriteChallenge(t *testing.T) {
	buf := make([]byte, 64)
	value := bytes.Repeat([]byte{0x5A}, mschapv2ChallengeValueLen)
	n := WriteMSCHAPv2Challenge(buf, 0, 0x42, value, []byte("ze"))
	wantLen := mschapv2HeaderLen + 1 + mschapv2ChallengeValueLen + 2
	if n != wantLen {
		t.Fatalf("n = %d, want %d", n, wantLen)
	}
	want := append([]byte{MSCHAPv2CodeChallenge, 0x42, 0x00, byte(wantLen),
		byte(mschapv2ChallengeValueLen)},
		bytes.Repeat([]byte{0x5A}, mschapv2ChallengeValueLen)...)
	want = append(want, 'z', 'e')
	if !bytes.Equal(buf[:n], want) {
		t.Errorf("buf = %x, want %x", buf[:n], want)
	}
}

// VALIDATES: WriteMSCHAPv2Challenge writes at the requested offset
//
//	without overwriting preceding buffer contents.
func TestMSCHAPv2WriteChallengeOffset(t *testing.T) {
	buf := make([]byte, 64)
	buf[0] = 0xAA
	buf[1] = 0xAA
	value := bytes.Repeat([]byte{0xCC}, mschapv2ChallengeValueLen)
	n := WriteMSCHAPv2Challenge(buf, 2, 0x01, value, []byte("x"))
	if buf[0] != 0xAA || buf[1] != 0xAA {
		t.Errorf("prefix overwritten: %x", buf[:2])
	}
	wantLen := mschapv2HeaderLen + 1 + mschapv2ChallengeValueLen + 1
	if n != wantLen {
		t.Fatalf("n = %d, want %d", n, wantLen)
	}
	if buf[2] != MSCHAPv2CodeChallenge {
		t.Errorf("code = %d, want MSCHAPv2CodeChallenge", buf[2])
	}
	if buf[6] != byte(mschapv2ChallengeValueLen) {
		t.Errorf("Value-Size = %d, want %d", buf[6], mschapv2ChallengeValueLen)
	}
}

// VALIDATES: WriteMSCHAPv2Challenge clamps the Name field so the total
//
//	packet fits inside a single MaxFrameLen PPP frame. The
//	written Length field MUST equal the clamped total bytes.
func TestMSCHAPv2WriteChallengeCapsNameByFrame(t *testing.T) {
	buf := make([]byte, MaxFrameLen)
	value := bytes.Repeat([]byte{0xCD}, mschapv2ChallengeValueLen)
	hugeName := bytes.Repeat([]byte{'n'}, MaxFrameLen)
	n := WriteMSCHAPv2Challenge(buf, 2, 0x10, value, hugeName)
	maxName := MaxFrameLen - 2 - mschapv2HeaderLen - 1 - mschapv2ChallengeValueLen
	wantTotal := mschapv2HeaderLen + 1 + mschapv2ChallengeValueLen + maxName
	if n != wantTotal {
		t.Fatalf("n = %d, want %d (clamped to frame)", n, wantTotal)
	}
	gotLen := int(binary.BigEndian.Uint16(buf[2+2 : 2+4]))
	if gotLen != wantTotal {
		t.Errorf("Length field = %d, want %d", gotLen, wantTotal)
	}
	// Value bytes must remain intact before the Name region.
	for i := range mschapv2ChallengeValueLen {
		if buf[2+5+i] != 0xCD {
			t.Errorf("Value[%d] = 0x%02x, want 0xCD", i, buf[2+5+i])
		}
	}
	nameStart := 2 + 5 + mschapv2ChallengeValueLen
	if buf[nameStart] != 'n' {
		t.Errorf("Name[0] = 0x%02x, want 'n'", buf[nameStart])
	}
	if buf[nameStart+maxName-1] != 'n' {
		t.Errorf("Name[maxName-1] = 0x%02x, want 'n'", buf[nameStart+maxName-1])
	}
}

// VALIDATES: WriteMSCHAPv2Success encodes Code=3 and writes a Message
//
//	of the form "S=<40-hex-upper> M=<msg>" BARE after the header
//	-- NO Msg-Length octet. Hex digits A-F are uppercase per
//	RFC 2759 Section 5.
//
// PREVENTS: regression where the encoder emits lowercase hex, inserts
//
//	a Msg-Length octet, or drops the " M=" separator.
func TestMSCHAPv2WriteSuccess(t *testing.T) {
	buf := make([]byte, 128)
	blob := bytes.Repeat([]byte{0xAB}, mschapv2AuthenticatorResponseLen)
	message := []byte("welcome")
	n := WriteMSCHAPv2Success(buf, 0, 0x42, blob, message)

	// "S=" + 40 uppercase hex + " M=" + "welcome"
	// Hex of 0xAB -> 'A' 'B'; 20 bytes => 40 chars of "AB"x20.
	hexPart := strings.Repeat("AB", mschapv2AuthenticatorResponseLen)
	wantMsg := "S=" + hexPart + " M=welcome"
	wantTotal := mschapv2HeaderLen + len(wantMsg)
	if n != wantTotal {
		t.Fatalf("n = %d, want %d", n, wantTotal)
	}
	if buf[0] != MSCHAPv2CodeSuccess {
		t.Errorf("code = %d, want MSCHAPv2CodeSuccess", buf[0])
	}
	if buf[1] != 0x42 {
		t.Errorf("identifier = 0x%02x, want 0x42", buf[1])
	}
	gotLen := int(binary.BigEndian.Uint16(buf[2:4]))
	if gotLen != wantTotal {
		t.Errorf("Length = %d, want %d", gotLen, wantTotal)
	}
	if buf[4] != 'S' {
		t.Fatalf("byte 4 = 0x%02x, want 'S' (no Msg-Length octet in MS-CHAPv2)",
			buf[4])
	}
	if buf[5] != '=' {
		t.Fatalf("byte 5 = 0x%02x, want '='", buf[5])
	}
	if string(buf[mschapv2HeaderLen:n]) != wantMsg {
		t.Errorf("Message = %q, want %q",
			string(buf[mschapv2HeaderLen:n]), wantMsg)
	}
	// Assert hex digits A-F are uppercase.
	for i := 6; i < 6+40; i++ {
		c := buf[i]
		if c >= 'a' && c <= 'f' {
			t.Errorf("hex char at byte %d = %q, MUST be uppercase", i, c)
		}
	}
}

// VALIDATES: WriteMSCHAPv2Success with an empty Message emits "S=<hex>
//
//	M=" -- the preface is always present.
func TestMSCHAPv2WriteSuccessEmptyMessage(t *testing.T) {
	buf := make([]byte, 128)
	blob := make([]byte, mschapv2AuthenticatorResponseLen)
	for i := range blob {
		blob[i] = 0x00
	}
	n := WriteMSCHAPv2Success(buf, 0, 0x01, blob, nil)
	hexPart := strings.Repeat("00", mschapv2AuthenticatorResponseLen)
	wantMsg := "S=" + hexPart + " M="
	wantTotal := mschapv2HeaderLen + len(wantMsg)
	if n != wantTotal {
		t.Fatalf("n = %d, want %d", n, wantTotal)
	}
	if string(buf[mschapv2HeaderLen:n]) != wantMsg {
		t.Errorf("Message = %q, want %q",
			string(buf[mschapv2HeaderLen:n]), wantMsg)
	}
}

// VALIDATES: WriteMSCHAPv2Success panics with "BUG: ..." when
//
//	authResponseBlob is not exactly 20 bytes -- caller contract
//	violation surfaced as a programmer-error guard per
//	rules/go-standards.md.
func TestMSCHAPv2WriteSuccessPanicsOnWrongBlobLen(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("WriteMSCHAPv2Success did not panic on wrong blob len")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("recover() type = %T, want string", r)
		}
		if !strings.HasPrefix(msg, "BUG: ") {
			t.Errorf("panic message = %q, want prefix \"BUG: \"", msg)
		}
	}()
	buf := make([]byte, 128)
	blob := make([]byte, mschapv2AuthenticatorResponseLen-1)
	_ = WriteMSCHAPv2Success(buf, 0, 0x01, blob, nil)
}

// VALIDATES: WriteMSCHAPv2Success clamps the Message field so the
//
//	packet fits inside a single MaxFrameLen PPP frame.
//
// PREVENTS: regression where an over-long Message runs off the buffer
//
//	or records a bogus Length.
func TestMSCHAPv2WriteSuccessCapsMessageByFrame(t *testing.T) {
	buf := make([]byte, MaxFrameLen+1)
	blob := bytes.Repeat([]byte{0x55}, mschapv2AuthenticatorResponseLen)
	hugeMessage := bytes.Repeat([]byte{'m'}, MaxFrameLen)
	n := WriteMSCHAPv2Success(buf, 2, 0x20, blob, hugeMessage)
	maxMessage := MaxFrameLen - 2 - mschapv2HeaderLen
	wantTotal := mschapv2HeaderLen + maxMessage
	if n != wantTotal {
		t.Fatalf("n = %d, want %d (clamped to frame)", n, wantTotal)
	}
	gotLen := int(binary.BigEndian.Uint16(buf[2+2 : 2+4]))
	if gotLen != wantTotal {
		t.Errorf("Length field = %d, want %d", gotLen, wantTotal)
	}
	// Byte immediately past the clamp must remain 0 (not written).
	pastClamp := 2 + mschapv2HeaderLen + maxMessage
	if buf[pastClamp] != 0 {
		t.Errorf("byte past clamp = 0x%02x, want 0 (encoder wrote past maxMessage)",
			buf[pastClamp])
	}
}

// VALIDATES: WriteMSCHAPv2Failure encodes Code=4 and writes the Message
//
//	bytes VERBATIM after the header (no prefix). The auth
//	handler is responsible for assembling the structured
//	"E=... R=... C=... V=... M=..." string per RFC 2759
//	Section 5.
//
// PREVENTS: regression where the Failure encoder injects its own
//
//	"E=" prefix or drops bytes.
func TestMSCHAPv2WriteFailure(t *testing.T) {
	buf := make([]byte, 128)
	msg := []byte("E=691 R=0 V=3 M=invalid credentials")
	n := WriteMSCHAPv2Failure(buf, 0, 0x77, msg)
	wantTotal := mschapv2HeaderLen + len(msg)
	if n != wantTotal {
		t.Fatalf("n = %d, want %d", n, wantTotal)
	}
	if buf[0] != MSCHAPv2CodeFailure {
		t.Errorf("code = %d, want MSCHAPv2CodeFailure", buf[0])
	}
	if buf[1] != 0x77 {
		t.Errorf("identifier = 0x%02x, want 0x77", buf[1])
	}
	gotLen := int(binary.BigEndian.Uint16(buf[2:4]))
	if gotLen != wantTotal {
		t.Errorf("Length = %d, want %d", gotLen, wantTotal)
	}
	if !bytes.Equal(buf[mschapv2HeaderLen:n], msg) {
		t.Errorf("Message = %q, want %q (verbatim)",
			string(buf[mschapv2HeaderLen:n]), string(msg))
	}
}

// VALIDATES: runMSCHAPv2AuthPhase sends a Challenge with Code=1, a
//
//	freshly-incremented Identifier, a 16-byte Authenticator
//	Challenge, and the LNS Name; parses the peer's 49-byte
//	Response; emits EventAuthRequest with Method=MSCHAPv2, the
//	peer's Identifier / Name, our Authenticator Challenge (16
//	bytes) and Peer-Challenge||NT-Response (40 bytes); then on
//	accept writes a Success frame carrying the formatted
//	"S=<hex> M=<msg>" Message.
//
// PREVENTS: regression where the event layout swaps Challenge with
//
//	Peer-Challenge or includes Reserved / Flags bytes in
//	Response.
func TestMSCHAPv2ResponseEmitsEvent(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut := newAuthTestSession(driverEnd)

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runMSCHAPv2AuthPhase()
	}()

	proto, payload := readPeerFrame(t, peerEnd)
	if proto != ProtoCHAP {
		t.Fatalf("challenge proto = 0x%04x, want 0x%04x", proto, ProtoCHAP)
	}
	if payload[0] != MSCHAPv2CodeChallenge {
		t.Fatalf("challenge code = %d, want MSCHAPv2CodeChallenge", payload[0])
	}
	challengeID := payload[1]
	valueSize := int(payload[4])
	if valueSize != mschapv2ChallengeValueLen {
		t.Fatalf("Value-Size = %d, want %d", valueSize, mschapv2ChallengeValueLen)
	}
	ourValue := make([]byte, valueSize)
	copy(ourValue, payload[5:5+valueSize])
	gotName := string(payload[5+valueSize:])
	if gotName != mschapv2LNSName {
		t.Errorf("Challenge Name = %q, want %q", gotName, mschapv2LNSName)
	}

	// Build a valid Response: Identifier echoed, known Peer-Challenge
	// and NT-Response, Reserved zero, Flags zero, Name = "bob".
	peerChallenge := bytes.Repeat([]byte{0xAA}, mschapv2PeerChallengeLen)
	ntResponse := bytes.Repeat([]byte{0xBB}, mschapv2NTResponseLen)
	respPayload := buildMSCHAPv2ResponsePayload(challengeID, peerChallenge, ntResponse, "bob")
	peerWrite := make([]byte, MaxFrameLen)
	off := WriteFrame(peerWrite, 0, ProtoCHAP, nil)
	copy(peerWrite[off:], respPayload)
	off += len(respPayload)
	if _, err := peerEnd.Write(peerWrite[:off]); err != nil {
		t.Fatalf("peer write response: %v", err)
	}

	var evt EventAuthRequest
	select {
	case raw := <-authEventsOut:
		got, ok := raw.(EventAuthRequest)
		if !ok {
			t.Fatalf("first auth event %T, want EventAuthRequest", raw)
		}
		evt = got
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}

	if evt.Method != AuthMethodMSCHAPv2 {
		t.Errorf("Method = %v, want AuthMethodMSCHAPv2", evt.Method)
	}
	if evt.Identifier != challengeID {
		t.Errorf("Identifier = 0x%02x, want 0x%02x", evt.Identifier, challengeID)
	}
	if evt.Username != "bob" {
		t.Errorf("Username = %q, want %q", evt.Username, "bob")
	}
	if evt.PeerName != "bob" {
		t.Errorf("PeerName = %q, want %q", evt.PeerName, "bob")
	}
	if !bytes.Equal(evt.Challenge, ourValue) {
		t.Errorf("Challenge = %x, want %x (our 16B Authenticator Challenge)",
			evt.Challenge, ourValue)
	}
	wantResp := append(bytes.Clone(peerChallenge), ntResponse...)
	if !bytes.Equal(evt.Response, wantResp) {
		t.Errorf("Response = %x, want %x (peer-challenge || nt-response, 40B)",
			evt.Response, wantResp)
	}
	if len(evt.Response) != mschapv2PeerChallengeLen+mschapv2NTResponseLen {
		t.Errorf("Response length = %d, want %d", len(evt.Response),
			mschapv2PeerChallengeLen+mschapv2NTResponseLen)
	}
	if evt.TunnelID != s.tunnelID || evt.SessionID != s.sessionID {
		t.Errorf("IDs = (%d,%d), want (%d,%d)",
			evt.TunnelID, evt.SessionID, s.tunnelID, s.sessionID)
	}

	// Deliver an accept decision with a known 20-byte blob.
	blob := bytes.Repeat([]byte{0xCD}, mschapv2AuthenticatorResponseLen)
	s.authRespCh <- authResponseMsg{
		accept:           true,
		message:          "ok",
		authResponseBlob: blob,
	}

	proto, payload = readPeerFrame(t, peerEnd)
	if proto != ProtoCHAP {
		t.Errorf("reply proto = 0x%04x, want 0x%04x", proto, ProtoCHAP)
	}
	if payload[0] != MSCHAPv2CodeSuccess {
		t.Errorf("reply code = %d, want MSCHAPv2CodeSuccess", payload[0])
	}
	if payload[1] != challengeID {
		t.Errorf("reply identifier = 0x%02x, want 0x%02x", payload[1], challengeID)
	}
	// MS-CHAPv2 Success/Failure has no Msg-Length: byte 4 starts the
	// formatted Message "S=<hex> M=<msg>".
	if payload[4] != 'S' {
		t.Errorf("byte 4 = 0x%02x, want 'S' (no Msg-Length octet)", payload[4])
	}
	length := int(binary.BigEndian.Uint16(payload[2:4]))
	hexPart := strings.Repeat("CD", mschapv2AuthenticatorResponseLen)
	wantMsg := "S=" + hexPart + " M=ok"
	if string(payload[mschapv2HeaderLen:length]) != wantMsg {
		t.Errorf("Success Message = %q, want %q",
			string(payload[mschapv2HeaderLen:length]), wantMsg)
	}

	select {
	case ev := <-authEventsOut:
		if _, ok := ev.(EventAuthSuccess); !ok {
			t.Errorf("second auth event %T, want EventAuthSuccess", ev)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for EventAuthSuccess")
	}

	if !<-handlerDone {
		t.Errorf("runMSCHAPv2AuthPhase returned false, want true on accept")
	}
}

// VALIDATES: runMSCHAPv2AuthPhase on AuthResponse(accept=false) writes
//
//	a Failure frame with Code=4 and the reject Message bytes
//	verbatim, and returns false.
func TestMSCHAPv2RejectWritesFailure(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut := newAuthTestSession(driverEnd)

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runMSCHAPv2AuthPhase()
	}()

	_, payload := readPeerFrame(t, peerEnd)
	challengeID := payload[1]

	pc := bytes.Repeat([]byte{0x11}, mschapv2PeerChallengeLen)
	nt := bytes.Repeat([]byte{0x22}, mschapv2NTResponseLen)
	respPayload := buildMSCHAPv2ResponsePayload(challengeID, pc, nt, "u")
	peerWrite := make([]byte, MaxFrameLen)
	off := WriteFrame(peerWrite, 0, ProtoCHAP, nil)
	copy(peerWrite[off:], respPayload)
	off += len(respPayload)
	if _, err := peerEnd.Write(peerWrite[:off]); err != nil {
		t.Fatalf("peer write response: %v", err)
	}

	select {
	case <-authEventsOut:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}

	reject := "E=691 R=0 C=00000000000000000000000000000000 V=3 M=nope"
	s.authRespCh <- authResponseMsg{accept: false, message: reject}

	_, payload = readPeerFrame(t, peerEnd)
	if payload[0] != MSCHAPv2CodeFailure {
		t.Errorf("reply code = %d, want MSCHAPv2CodeFailure", payload[0])
	}
	if payload[1] != challengeID {
		t.Errorf("reply identifier = 0x%02x, want 0x%02x", payload[1], challengeID)
	}
	length := int(binary.BigEndian.Uint16(payload[2:4]))
	if string(payload[mschapv2HeaderLen:length]) != reject {
		t.Errorf("message = %q, want %q",
			string(payload[mschapv2HeaderLen:length]), reject)
	}

	select {
	case ev := <-authEventsOut:
		if _, ok := ev.(EventAuthFailure); !ok {
			t.Errorf("second auth event %T, want EventAuthFailure", ev)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for EventAuthFailure")
	}

	if <-handlerDone {
		t.Errorf("runMSCHAPv2AuthPhase returned true, want false on reject")
	}
}

// VALIDATES: runMSCHAPv2AuthPhase fails the session cleanly (does NOT
//
//	panic) when the auth handler returns an authResponseBlob of
//	any length other than mschapv2AuthenticatorResponseLen. The
//	PPP per-session goroutine is not recovered at any higher
//	level; a panic here would crash the whole ze daemon for one
//	misbehaving plugin. Boundary cases enumerated per rules/tdd.md:
//	last-valid (20) is covered by TestMSCHAPv2ResponseEmitsEvent;
//	this test covers first-invalid-below (19), first-invalid-above
//	(21), empty (nil -> 0 bytes), and far-above (255).
//
// PREVENTS: regression where a misconfigured l2tp-auth plugin (Phase
//
//  8. takes the entire daemon down, OR an off-by-one in the
//     length comparison silently accepts 19- or 21-byte blobs.
func TestMSCHAPv2HandlerWrongBlobLenFailsClean(t *testing.T) {
	cases := []struct {
		name string
		blob []byte
	}{
		{name: "nil blob (0 bytes)", blob: nil},
		{name: "one below valid (19 bytes)",
			blob: bytes.Repeat([]byte{0xCD}, mschapv2AuthenticatorResponseLen-1)},
		{name: "one above valid (21 bytes)",
			blob: bytes.Repeat([]byte{0xCD}, mschapv2AuthenticatorResponseLen+1)},
		{name: "far above valid (255 bytes)", blob: bytes.Repeat([]byte{0xCD}, 0xFF)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peerEnd, driverEnd := net.Pipe()
			defer closeConn(peerEnd)
			s, authEventsOut := newAuthTestSession(driverEnd)

			type runResult struct {
				ok    bool
				panic any
			}
			result := make(chan runResult, 1)
			go func() {
				var p any
				var ok bool
				defer func() {
					p = recover()
					result <- runResult{ok: ok, panic: p}
				}()
				ok = s.runMSCHAPv2AuthPhase()
			}()

			_, payload := readPeerFrame(t, peerEnd)
			challengeID := payload[1]

			pc := bytes.Repeat([]byte{0x11}, mschapv2PeerChallengeLen)
			nt := bytes.Repeat([]byte{0x22}, mschapv2NTResponseLen)
			respPayload := buildMSCHAPv2ResponsePayload(challengeID, pc, nt, "u")
			peerWrite := make([]byte, MaxFrameLen)
			off := WriteFrame(peerWrite, 0, ProtoCHAP, nil)
			copy(peerWrite[off:], respPayload)
			off += len(respPayload)
			if _, err := peerEnd.Write(peerWrite[:off]); err != nil {
				t.Fatalf("peer write: %v", err)
			}

			select {
			case <-authEventsOut:
			case <-time.After(2 * time.Second):
				t.Fatal("timed out waiting for EventAuthRequest")
			}

			s.authRespCh <- authResponseMsg{
				accept:           true,
				message:          "ok",
				authResponseBlob: tc.blob,
			}

			// Single select distinguishes panic from timeout so a
			// regression that re-introduces the panic surfaces with
			// the panic value in the test output rather than as an
			// ambiguous timeout.
			select {
			case r := <-result:
				if r.panic != nil {
					t.Fatalf("handler panicked (would crash daemon): %v", r.panic)
				}
				if r.ok {
					t.Errorf("handler returned true, want false on blob len %d",
						len(tc.blob))
				}
			case <-time.After(2 * time.Second):
				t.Fatal("handler did not return and did not panic")
			}

			select {
			case ev := <-authEventsOut:
				fail, ok := ev.(EventAuthFailure)
				if !ok {
					t.Fatalf("second auth event %T, want EventAuthFailure", ev)
				}
				if !strings.Contains(fail.Reason, "authResponseBlob") {
					t.Errorf("Reason = %q, want mention of authResponseBlob",
						fail.Reason)
				}
			case <-time.After(1 * time.Second):
				t.Fatal("timed out waiting for EventAuthFailure")
			}
		})
	}
}

// VALIDATES: runMSCHAPv2AuthPhase on authTimeout emits
//
//	EventAuthFailure{Reason:"timeout"}, writes no reply frame,
//	and returns false. Same contract as runCHAPAuthPhase.
func TestMSCHAPv2TimeoutEmitsFailure(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut := newAuthTestSession(driverEnd)
	s.authTimeout = 80 * time.Millisecond

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runMSCHAPv2AuthPhase()
	}()

	_, payload := readPeerFrame(t, peerEnd)
	challengeID := payload[1]

	pc := bytes.Repeat([]byte{0x33}, mschapv2PeerChallengeLen)
	nt := bytes.Repeat([]byte{0x44}, mschapv2NTResponseLen)
	respPayload := buildMSCHAPv2ResponsePayload(challengeID, pc, nt, "t")
	peerWrite := make([]byte, MaxFrameLen)
	off := WriteFrame(peerWrite, 0, ProtoCHAP, nil)
	copy(peerWrite[off:], respPayload)
	off += len(respPayload)
	if _, err := peerEnd.Write(peerWrite[:off]); err != nil {
		t.Fatalf("peer write response: %v", err)
	}

	select {
	case <-authEventsOut:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}

	select {
	case ev := <-authEventsOut:
		fail, ok := ev.(EventAuthFailure)
		if !ok {
			t.Fatalf("second auth event %T, want EventAuthFailure", ev)
		}
		if fail.Reason != "timeout" {
			t.Errorf("Reason = %q, want \"timeout\"", fail.Reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthFailure{timeout}")
	}

	if <-handlerDone {
		t.Errorf("runMSCHAPv2AuthPhase returned true, want false on timeout")
	}

	pollRead := make(chan error, 1)
	go func() {
		buf := make([]byte, 32)
		_ = peerEnd.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
		_, err := peerEnd.Read(buf)
		pollRead <- err
	}()
	select {
	case err := <-pollRead:
		if err == nil {
			t.Errorf("unexpected wire write on timeout path")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("poll read never returned")
	}
}

// VALIDATES: runMSCHAPv2AuthPhase reports each malformed-wire input as
//
//	a session failure via s.fail and returns false, without
//	emitting EventAuthRequest. The Challenge is always written
//	first (authenticator-initiated).
func TestMSCHAPv2HandlerWireErrors(t *testing.T) {
	cases := []struct {
		name  string
		write func(t *testing.T, peerEnd net.Conn, challengeID uint8)
	}{
		{
			name:  "peer closes without sending response",
			write: nil,
		},
		{
			name: "frame too short for protocol field",
			write: func(t *testing.T, peerEnd net.Conn, _ uint8) {
				t.Helper()
				if _, err := peerEnd.Write([]byte{0xFF}); err != nil {
					t.Fatalf("peer write: %v", err)
				}
			},
		},
		{
			name: "wrong protocol on chanFile",
			write: func(t *testing.T, peerEnd net.Conn, _ uint8) {
				t.Helper()
				buf := make([]byte, 32)
				off := WriteFrame(buf, 0, ProtoLCP, nil)
				off += WriteLCPPacket(buf, off,
					LCPConfigureRequest, 0x01, nil)
				if _, err := peerEnd.Write(buf[:off]); err != nil {
					t.Fatalf("peer write: %v", err)
				}
			},
		},
		{
			name: "valid framing but wrong code (Challenge as Response)",
			write: func(t *testing.T, peerEnd net.Conn, challengeID uint8) {
				t.Helper()
				buf := make([]byte, 64)
				off := WriteFrame(buf, 0, ProtoCHAP, nil)
				value := bytes.Repeat([]byte{0x01}, mschapv2ChallengeValueLen)
				off += WriteMSCHAPv2Challenge(buf, off, challengeID,
					value, []byte("z"))
				if _, err := peerEnd.Write(buf[:off]); err != nil {
					t.Fatalf("peer write: %v", err)
				}
			},
		},
		{
			name: "identifier mismatch on response",
			write: func(t *testing.T, peerEnd net.Conn, challengeID uint8) {
				t.Helper()
				mismatch := challengeID + 1
				pc := bytes.Repeat([]byte{0x55}, mschapv2PeerChallengeLen)
				nt := bytes.Repeat([]byte{0x66}, mschapv2NTResponseLen)
				respPayload := buildMSCHAPv2ResponsePayload(mismatch, pc, nt, "p")
				buf := make([]byte, MaxFrameLen)
				off := WriteFrame(buf, 0, ProtoCHAP, nil)
				copy(buf[off:], respPayload)
				off += len(respPayload)
				if _, err := peerEnd.Write(buf[:off]); err != nil {
					t.Fatalf("peer write: %v", err)
				}
			},
		},
		{
			name: "Value-Size 48 (one below)",
			write: func(t *testing.T, peerEnd net.Conn, challengeID uint8) {
				t.Helper()
				pc := bytes.Repeat([]byte{0x77}, mschapv2PeerChallengeLen)
				nt := bytes.Repeat([]byte{0x88}, mschapv2NTResponseLen)
				respPayload := buildMSCHAPv2ResponsePayload(challengeID, pc, nt, "p")
				respPayload[4] = 48 // corrupt Value-Size
				buf := make([]byte, MaxFrameLen)
				off := WriteFrame(buf, 0, ProtoCHAP, nil)
				copy(buf[off:], respPayload)
				off += len(respPayload)
				if _, err := peerEnd.Write(buf[:off]); err != nil {
					t.Fatalf("peer write: %v", err)
				}
			},
		},
		{
			name: "reserved non-zero",
			write: func(t *testing.T, peerEnd net.Conn, challengeID uint8) {
				t.Helper()
				pc := bytes.Repeat([]byte{0x99}, mschapv2PeerChallengeLen)
				nt := bytes.Repeat([]byte{0xAA}, mschapv2NTResponseLen)
				respPayload := buildMSCHAPv2ResponsePayload(challengeID, pc, nt, "p")
				// Reserved at wire offset 4+1+16 = 21.
				respPayload[21] = 0x01
				buf := make([]byte, MaxFrameLen)
				off := WriteFrame(buf, 0, ProtoCHAP, nil)
				copy(buf[off:], respPayload)
				off += len(respPayload)
				if _, err := peerEnd.Write(buf[:off]); err != nil {
					t.Fatalf("peer write: %v", err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peerEnd, driverEnd := net.Pipe()
			s, authEventsOut := newAuthTestSession(driverEnd)

			handlerDone := make(chan bool, 1)
			go func() {
				handlerDone <- s.runMSCHAPv2AuthPhase()
			}()

			_, payload := readPeerFrame(t, peerEnd)
			challengeID := payload[1]

			if tc.write == nil {
				closeConn(peerEnd)
			} else {
				tc.write(t, peerEnd, challengeID)
				defer closeConn(peerEnd)
			}

			select {
			case ok := <-handlerDone:
				if ok {
					t.Errorf("runMSCHAPv2AuthPhase = true, want false on %s",
						tc.name)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("handler did not return on %s", tc.name)
			}

			// The handler MUST NOT emit EventAuthRequest on a wire
			// error -- the consumer has nothing to decide.
			select {
			case ev := <-authEventsOut:
				t.Errorf("unexpected auth event on wire error: %T", ev)
			default:
			}
		})
	}
}

// VALIDATES: runMSCHAPv2AuthPhase rejects a Response whose Reserved
//
//	octets are non-zero (AC-17) and does NOT emit
//	EventAuthRequest. RFC 2759 Section 4: Reserved MUST be zero;
//	a protocol-violating Response aborts the session before the
//	credential round-trip.
//
// PREVENTS: regression where a peer smuggles bits in Reserved and ze
//
//	forwards them to the auth handler.
func TestMSCHAPv2RejectsNonZeroReserved(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut := newAuthTestSession(driverEnd)

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runMSCHAPv2AuthPhase()
	}()

	_, payload := readPeerFrame(t, peerEnd)
	challengeID := payload[1]

	pc := bytes.Repeat([]byte{0x11}, mschapv2PeerChallengeLen)
	nt := bytes.Repeat([]byte{0x22}, mschapv2NTResponseLen)
	respPayload := buildMSCHAPv2ResponsePayload(challengeID, pc, nt, "bad")
	// Corrupt the middle byte of Reserved at body offset 1+16+3 = 20,
	// wire offset 4+20 = 24.
	respPayload[24] = 0x01
	buf := make([]byte, MaxFrameLen)
	off := WriteFrame(buf, 0, ProtoCHAP, nil)
	copy(buf[off:], respPayload)
	off += len(respPayload)
	if _, err := peerEnd.Write(buf[:off]); err != nil {
		t.Fatalf("peer write: %v", err)
	}

	select {
	case ok := <-handlerDone:
		if ok {
			t.Errorf("handler returned true, want false on reserved non-zero")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return")
	}

	select {
	case ev := <-authEventsOut:
		t.Errorf("unexpected auth event (AC-17 says NO EventAuthRequest): %T", ev)
	default:
	}
}

// VALIDATES: runMSCHAPv2AuthPhase increments s.chapIdentifier once per
//
//	Challenge (RFC 2759 Section 4 inherits RFC 1994 Section 4.1
//	"MUST change each Challenge").
func TestMSCHAPv2IdentifierMonotonic(t *testing.T) {
	ids := make([]uint8, 0, 2)
	for range 2 {
		peerEnd, driverEnd := net.Pipe()
		s, _ := newAuthTestSession(driverEnd)
		if len(ids) > 0 {
			s.chapIdentifier = ids[len(ids)-1]
		}

		handlerDone := make(chan bool, 1)
		go func() {
			handlerDone <- s.runMSCHAPv2AuthPhase()
		}()

		_, payload := readPeerFrame(t, peerEnd)
		ids = append(ids, payload[1])

		closeConn(peerEnd)
		<-handlerDone
	}
	if ids[0] == ids[1] {
		t.Errorf("Identifiers %d and %d are equal; MUST change per RFC 2759 Section 4",
			ids[0], ids[1])
	}
	if ids[1] != ids[0]+1 {
		t.Errorf("Identifiers not monotonic: %d -> %d, want +1 (mod 256)", ids[0], ids[1])
	}
}

// VALIDATES: s.chapIdentifier wraps from 0xFF to 0x00 on the next
//
//	Challenge. The uint8 boundary is still a change (0xFF !=
//	0x00) so wrap is legal.
func TestMSCHAPv2IdentifierWraps(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	s, _ := newAuthTestSession(driverEnd)
	s.chapIdentifier = 0xFF

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runMSCHAPv2AuthPhase()
	}()

	_, payload := readPeerFrame(t, peerEnd)
	if payload[1] != 0x00 {
		t.Errorf("Identifier after wrap = 0x%02x, want 0x00", payload[1])
	}

	closeConn(peerEnd)
	<-handlerDone
}

// VALIDATES: s.chapIdentifier is SHARED between CHAP-MD5 and MS-CHAPv2
//
//	handlers. Running one CHAP-MD5 Challenge followed by one
//	MS-CHAPv2 Challenge on the same session produces
//	consecutive Identifiers (N and N+1). LCP negotiates exactly
//	one Auth-Protocol method per session, so a single
//	per-session counter is correct for both methods and guards
//	against a future refactor that accidentally duplicates the
//	counter.
//
// PREVENTS: regression where adding MS-CHAPv2 introduces a separate
//
//	counter and two Challenges on different methods reuse the
//	same Identifier (violating RFC 1994/2759 "MUST change").
func TestCHAPAndMSCHAPv2ShareIdentifier(t *testing.T) {
	// Run CHAP-MD5 Challenge.
	peerEnd1, driverEnd1 := net.Pipe()
	s, _ := newAuthTestSession(driverEnd1)
	s.chapIdentifier = 0x20

	done1 := make(chan bool, 1)
	go func() {
		done1 <- s.runCHAPAuthPhase()
	}()
	_, payload1 := readPeerFrame(t, peerEnd1)
	chapID := payload1[1]
	closeConn(peerEnd1)
	<-done1

	// Re-wire the session to a new pipe for the MS-CHAPv2 Challenge;
	// carry the chapIdentifier counter across. newAuthTestSession
	// resets fields it initializes, so we reuse the existing session
	// struct and only swap the chanFile.
	//
	// This test relies on pppSession carrying NO cross-method state
	// other than chapIdentifier: if a future field gains cross-phase
	// semantics (pending event buffer, in-flight request tracker,
	// auth-request retry counter), this test will still pass while the
	// production invariant silently breaks. When adding such a field,
	// update this test to assert the field is reset between phases OR
	// split the test into two independent sessions that inject the
	// starting counter explicitly.
	peerEnd2, driverEnd2 := net.Pipe()
	s.chanFile = driverEnd2
	// Pin the invariant we actually care about at the time of writing:
	// the only post-CHAP state we inherit is chapIdentifier. Any
	// listed field left non-zero after runCHAPAuthPhase signals a
	// cross-phase leak we need to account for.
	if s.authTimeout != 2*time.Second {
		t.Fatalf("authTimeout drifted during CHAP phase: %v", s.authTimeout)
	}
	if len(s.authRespCh) != 0 {
		t.Fatalf("authRespCh has %d pending messages after CHAP phase", len(s.authRespCh))
	}

	done2 := make(chan bool, 1)
	go func() {
		done2 <- s.runMSCHAPv2AuthPhase()
	}()
	_, payload2 := readPeerFrame(t, peerEnd2)
	mschapID := payload2[1]
	closeConn(peerEnd2)
	<-done2

	if chapID == mschapID {
		t.Errorf("CHAP-MD5 Identifier 0x%02x == MS-CHAPv2 Identifier 0x%02x; "+
			"counter NOT shared (each method has its own)", chapID, mschapID)
	}
	if mschapID != chapID+1 {
		t.Errorf("Identifiers 0x%02x -> 0x%02x, want +1 (shared counter)",
			chapID, mschapID)
	}
}

// VALIDATES: runMSCHAPv2AuthPhase sources its Authenticator Challenge
//
//	from crypto/rand -- two invocations produce different
//	Values. RFC 2759 Section 4 requires a cryptographic random
//	source.
func TestMSCHAPv2ChallengeRandom(t *testing.T) {
	values := make([][]byte, 0, 2)
	for range 2 {
		peerEnd, driverEnd := net.Pipe()
		s, _ := newAuthTestSession(driverEnd)

		handlerDone := make(chan bool, 1)
		go func() {
			handlerDone <- s.runMSCHAPv2AuthPhase()
		}()

		_, payload := readPeerFrame(t, peerEnd)
		valueSize := int(payload[4])
		v := make([]byte, valueSize)
		copy(v, payload[5:5+valueSize])
		values = append(values, v)

		closeConn(peerEnd)
		<-handlerDone
	}
	if bytes.Equal(values[0], values[1]) {
		t.Errorf("Challenge Values equal across invocations: %x -- MUST be unique",
			values[0])
	}
}

// VALIDATES: drawMSCHAPv2Challenge panics with "BUG: ..." when the
//
//	destination slice is shorter than mschapv2ChallengeValueLen.
func TestMSCHAPv2DrawChallengePanicsOnShortDst(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("drawMSCHAPv2Challenge did not panic on short dst")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("recover() type = %T, want string", r)
		}
		if !strings.HasPrefix(msg, "BUG: ") {
			t.Errorf("panic message = %q, want prefix \"BUG: \"", msg)
		}
	}()
	short := make([]byte, mschapv2ChallengeValueLen-1)
	_ = drawMSCHAPv2Challenge(short)
}
