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

// VALIDATES: ParseCHAPResponse decodes Code, Identifier, Length,
//
//	Value-Size, Value, and Name fields of a CHAP Response per
//	RFC 1994 Section 4.1.
//
// PREVENTS: regression where Value boundaries or the trailing Name
//
//	are read past the Length field and the parser silently
//	includes padding bytes.
func TestCHAPParseResponse(t *testing.T) {
	// Build a canonical Response: Identifier=0x42, Value-Size=16
	// (MD5 digest), Value=0x00..0x0F, Name="bob". Length = 4 + 1 +
	// 16 + 3 = 24 (0x18).
	value := make([]byte, chapMD5DigestLen)
	for i := range value {
		value[i] = byte(i)
	}
	good := []byte{0x02, 0x42, 0x00, 0x18, chapMD5DigestLen}
	good = append(good, value...)
	good = append(good, 'b', 'o', 'b')

	cases := []struct {
		name      string
		buf       []byte
		wantID    uint8
		wantValue []byte
		wantName  string
	}{
		{
			name:      "md5 digest with name",
			buf:       good,
			wantID:    0x42,
			wantValue: value,
			wantName:  "bob",
		},
		{
			name: "empty name",
			// Length = 4 + 1 + 1 = 6
			buf: []byte{
				0x02, 0x01, 0x00, 0x06,
				0x01, 0xFF,
			},
			wantID:    0x01,
			wantValue: []byte{0xFF},
			wantName:  "",
		},
		{
			name: "trailing padding ignored",
			// Length = 4 + 1 + 2 + 1 = 8; three padding bytes follow.
			buf: []byte{
				0x02, 0x11, 0x00, 0x08,
				0x02, 0xAA, 0xBB,
				'x',
				0xFF, 0xFF, 0xFF,
			},
			wantID:    0x11,
			wantValue: []byte{0xAA, 0xBB},
			wantName:  "x",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := ParseCHAPResponse(tc.buf)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Identifier != tc.wantID {
				t.Errorf("Identifier = 0x%02x, want 0x%02x", resp.Identifier, tc.wantID)
			}
			if !bytes.Equal(resp.Value, tc.wantValue) {
				t.Errorf("Value = %x, want %x", resp.Value, tc.wantValue)
			}
			if resp.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", resp.Name, tc.wantName)
			}
		})
	}
}

// VALIDATES: ParseCHAPResponse accepts a 255-octet Value (uint8
//
//	Value-Size maximum) without overflow.
//
// PREVENTS: regression where a Value-Size of 0xFF causes a
//
//	slice-out-of-bounds panic or silently truncates.
func TestCHAPParseResponseBoundaryValueSize(t *testing.T) {
	const vsz = 0xFF
	total := chapHeaderLen + 1 + vsz + 0 // no Name.
	buf := make([]byte, total)
	buf[0] = CHAPCodeResponse
	buf[1] = 0x7F
	binary.BigEndian.PutUint16(buf[2:4], uint16(total))
	buf[4] = vsz
	for i := range vsz {
		buf[5+i] = byte(i)
	}

	resp, err := ParseCHAPResponse(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Value) != vsz {
		t.Errorf("Value length = %d, want %d", len(resp.Value), vsz)
	}
	if resp.Value[0] != 0x00 || resp.Value[vsz-1] != byte(vsz-1) {
		t.Errorf("Value first/last bytes = %02x/%02x", resp.Value[0], resp.Value[vsz-1])
	}
	if resp.Name != "" {
		t.Errorf("Name = %q, want empty", resp.Name)
	}
}

// VALIDATES: ParseCHAPResponse rejects malformed packets per RFC 1994
//
//	Section 4 "silently discard" guidance: buffers shorter than
//	the 4-byte header, wrong code, Length mismatches, Value-Size
//	of zero (invalid per Section 4.1), and Value-Size overflows.
func TestCHAPParseResponseInvalid(t *testing.T) {
	cases := []struct {
		name string
		buf  []byte
		want error
	}{
		{
			name: "too short",
			buf:  []byte{0x02, 0x00, 0x00},
			want: errCHAPTooShort,
		},
		{
			name: "wrong code challenge",
			buf: []byte{
				0x01, 0x00, 0x00, 0x06,
				0x01, 0xFF,
			},
			want: errCHAPWrongCode,
		},
		{
			name: "wrong code success",
			buf:  []byte{0x03, 0x00, 0x00, 0x04},
			want: errCHAPWrongCode,
		},
		{
			name: "length below Value-Size octet",
			buf:  []byte{0x02, 0x00, 0x00, 0x04},
			want: errCHAPLengthMismatch,
		},
		{
			name: "length exceeds buffer",
			buf:  []byte{0x02, 0x00, 0x10, 0x00, 0x00},
			want: errCHAPLengthMismatch,
		},
		{
			name: "Value-Size zero",
			buf: []byte{
				0x02, 0x00, 0x00, 0x05,
				0x00,
			},
			want: errCHAPValueSizeZero,
		},
		{
			name: "Value-Size overflows Length",
			buf: []byte{
				0x02, 0x00, 0x00, 0x08,
				0xFF, 'a', 'b', 'c',
			},
			want: errCHAPValueOverflow,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCHAPResponse(tc.buf)
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// VALIDATES: WriteCHAPChallenge encodes Code=1, Identifier,
//
//	Length, Value-Size, Value bytes, and Name per RFC 1994
//	Section 4.1.
//
// PREVENTS: regression where Length excludes the Name octets,
//
//	or where Value-Size is wrong.
func TestCHAPWriteChallenge(t *testing.T) {
	buf := make([]byte, 64)
	value := bytes.Repeat([]byte{0x5A}, chapChallengeValueLen)
	n := WriteCHAPChallenge(buf, 0, 0x42, value, []byte("ze"))
	// Length = 4 + 1 + 16 + 2 = 23 (0x17).
	wantLen := chapHeaderLen + 1 + chapChallengeValueLen + 2
	if n != wantLen {
		t.Fatalf("n = %d, want %d", n, wantLen)
	}
	want := append([]byte{CHAPCodeChallenge, 0x42, 0x00, byte(wantLen), chapChallengeValueLen},
		bytes.Repeat([]byte{0x5A}, chapChallengeValueLen)...)
	want = append(want, 'z', 'e')
	if !bytes.Equal(buf[:n], want) {
		t.Errorf("buf = %x, want %x", buf[:n], want)
	}
}

// VALIDATES: WriteCHAPChallenge writes at the requested offset without
//
//	overwriting preceding buffer contents.
func TestCHAPWriteChallengeOffset(t *testing.T) {
	buf := []byte{0xAA, 0xAA, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	n := WriteCHAPChallenge(buf, 2, 0x01, []byte{0xCC}, []byte("x"))
	if buf[0] != 0xAA || buf[1] != 0xAA {
		t.Errorf("prefix overwritten: %x", buf[:2])
	}
	want := []byte{CHAPCodeChallenge, 0x01, 0x00, 0x07, 0x01, 0xCC, 'x'}
	if !bytes.Equal(buf[2:2+n], want) {
		t.Errorf("buf[2:] = %x, want %x", buf[2:2+n], want)
	}
}

// VALIDATES: WriteCHAPChallenge clamps the Name field so the total
//
//	packet fits inside a single MaxFrameLen PPP frame. The
//	written Length field MUST equal the clamped total bytes.
//
// PREVENTS: regression where an over-long Name (e.g. from a
//
//	misconfigured hostname at Phase 7) silently corrupts the
//	Length field or runs off the buffer.
func TestCHAPWriteChallengeCapsNameByFrame(t *testing.T) {
	buf := make([]byte, MaxFrameLen)
	value := bytes.Repeat([]byte{0xCD}, chapChallengeValueLen)
	hugeName := bytes.Repeat([]byte{'n'}, MaxFrameLen) // way more than fits
	// Layout space after WriteFrame (off=2) = MaxFrameLen - 2.
	n := WriteCHAPChallenge(buf, 2, 0x10, value, hugeName)
	maxName := MaxFrameLen - 2 - chapHeaderLen - 1 - chapChallengeValueLen
	wantTotal := chapHeaderLen + 1 + chapChallengeValueLen + maxName
	if n != wantTotal {
		t.Fatalf("n = %d, want %d (clamped to frame)", n, wantTotal)
	}
	gotLen := int(binary.BigEndian.Uint16(buf[2+2 : 2+4]))
	if gotLen != wantTotal {
		t.Errorf("Length field = %d, want %d", gotLen, wantTotal)
	}
	// Value bytes must remain intact before the Name region.
	for i := range chapChallengeValueLen {
		if buf[2+5+i] != 0xCD {
			t.Errorf("Value[%d] = 0x%02x, want 0xCD", i, buf[2+5+i])
		}
	}
	// Name region must be filled with the clamped bytes up to maxName.
	// Assert first and last Name bytes came from the input (both 'n').
	nameStart := 2 + 5 + chapChallengeValueLen
	if buf[nameStart] != 'n' {
		t.Errorf("Name[0] = 0x%02x, want 'n'", buf[nameStart])
	}
	if buf[nameStart+maxName-1] != 'n' {
		t.Errorf("Name[maxName-1] = 0x%02x, want 'n'", buf[nameStart+maxName-1])
	}
}

// VALIDATES: WriteCHAPSuccess encodes Code=3 and writes Message
//
//	BARE after the header -- NO Msg-Length octet. This is the
//	single most important assertion in Phase 5 because it is
//	where CHAP diverges from PAP (which DOES prefix Message
//	with a length octet). RFC 1994 Section 4.2: the Message
//	runs from byte 4 to Length.
//
// PREVENTS: regression where the encoder copies PAP's shape and
//
//	inserts a Msg-Length octet at byte 4.
func TestCHAPWriteSuccess(t *testing.T) {
	buf := make([]byte, 32)
	n := WriteCHAPSuccess(buf, 0, 0x42, []byte("auth ok"))
	// Length = 4 + 7 = 11 (0x0B). Byte 4 MUST be 'a', not 0x07.
	want := []byte{
		CHAPCodeSuccess, 0x42, 0x00, 0x0B,
		'a', 'u', 't', 'h', ' ', 'o', 'k',
	}
	if n != len(want) {
		t.Errorf("n = %d, want %d", n, len(want))
	}
	if !bytes.Equal(buf[:n], want) {
		t.Errorf("buf = %x, want %x", buf[:n], want)
	}
	if buf[4] != 'a' {
		t.Fatalf("byte 4 = 0x%02x, want 'a' (0x%02x) -- CHAP has NO Msg-Length octet",
			buf[4], 'a')
	}
}

// VALIDATES: WriteCHAPSuccess clamps the Message field so the packet
//
//	fits inside a single MaxFrameLen PPP frame. The Length
//	field MUST equal the clamped total bytes.
//
// PREVENTS: regression where an over-long Message runs off the buffer
//
//	or records a bogus Length.
func TestCHAPWriteSuccessCapsMessageByFrame(t *testing.T) {
	// Buffer one byte larger than the frame cap so we can assert that
	// the byte immediately past the clamp was NOT written (stays 0).
	buf := make([]byte, MaxFrameLen+1)
	hugeMessage := bytes.Repeat([]byte{'m'}, MaxFrameLen) // way more than fits
	// Layout space after WriteFrame (off=2) = MaxFrameLen - 2.
	n := WriteCHAPSuccess(buf, 2, 0x20, hugeMessage)
	maxMessage := MaxFrameLen - 2 - chapHeaderLen
	wantTotal := chapHeaderLen + maxMessage
	if n != wantTotal {
		t.Fatalf("n = %d, want %d (clamped to frame)", n, wantTotal)
	}
	gotLen := int(binary.BigEndian.Uint16(buf[2+2 : 2+4]))
	if gotLen != wantTotal {
		t.Errorf("Length field = %d, want %d", gotLen, wantTotal)
	}
	// First and last Message bytes must be 'm' (the clamped input).
	msgStart := 2 + chapHeaderLen
	if buf[msgStart] != 'm' {
		t.Errorf("Message[0] = 0x%02x, want 'm'", buf[msgStart])
	}
	if buf[msgStart+maxMessage-1] != 'm' {
		t.Errorf("Message[maxMessage-1] = 0x%02x, want 'm'", buf[msgStart+maxMessage-1])
	}
	// The byte immediately past the clamp must remain 0 (not written).
	if buf[msgStart+maxMessage] != 0 {
		t.Errorf("byte past clamp = 0x%02x, want 0 (encoder wrote past maxMessage)",
			buf[msgStart+maxMessage])
	}
}

// VALIDATES: WriteCHAPSuccess with an empty Message emits a 4-byte
//
//	packet with Length=4 and no trailing bytes.
func TestCHAPWriteSuccessEmptyMessage(t *testing.T) {
	buf := make([]byte, 16)
	n := WriteCHAPSuccess(buf, 0, 0x01, nil)
	want := []byte{CHAPCodeSuccess, 0x01, 0x00, 0x04}
	if n != len(want) || !bytes.Equal(buf[:n], want) {
		t.Errorf("buf = %x (n=%d), want %x", buf[:n], n, want)
	}
}

// VALIDATES: WriteCHAPFailure encodes Code=4 and otherwise follows
//
//	the same format as Success (NO Msg-Length octet).
func TestCHAPWriteFailure(t *testing.T) {
	buf := make([]byte, 32)
	n := WriteCHAPFailure(buf, 0, 0x77, []byte("bad"))
	want := []byte{
		CHAPCodeFailure, 0x77, 0x00, 0x07,
		'b', 'a', 'd',
	}
	if n != len(want) || !bytes.Equal(buf[:n], want) {
		t.Errorf("buf = %x (n=%d), want %x", buf[:n], n, want)
	}
}

// VALIDATES: runCHAPAuthPhase sends a CHAP Challenge with Code=1, a
//
//	freshly-incremented Identifier, a 16-byte Value, and the
//	LNS Name; parses the peer's Response; emits
//	EventAuthRequest with Method=CHAPMD5 and the peer's
//	Identifier / Name / Challenge-we-sent / Response-digest;
//	then on accept writes a Success frame carrying the echoed
//	Identifier.
//
// PREVENTS: regression where the event omits CHAP-specific fields,
//
//	the Identifier is not echoed into Success, or the
//	Challenge bytes reported in the event do not match the
//	wire Value.
func TestCHAPResponseEmitsEvent(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut := newAuthTestSession(driverEnd)

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runCHAPAuthPhase()
	}()

	// Read the Challenge frame the handler wrote.
	proto, payload := readPeerFrame(t, peerEnd)
	if proto != ProtoCHAP {
		t.Fatalf("challenge proto = 0x%04x, want 0x%04x", proto, ProtoCHAP)
	}
	if len(payload) < chapHeaderLen+1 {
		t.Fatalf("challenge payload too short: %d bytes", len(payload))
	}
	if payload[0] != CHAPCodeChallenge {
		t.Fatalf("challenge code = %d, want CHAPCodeChallenge", payload[0])
	}
	challengeID := payload[1]
	valueSize := int(payload[4])
	if valueSize != chapChallengeValueLen {
		t.Fatalf("Value-Size = %d, want %d", valueSize, chapChallengeValueLen)
	}
	ourValue := make([]byte, valueSize)
	copy(ourValue, payload[5:5+valueSize])
	gotName := string(payload[5+valueSize:])
	if gotName != chapLNSName {
		t.Errorf("Challenge Name = %q, want %q", gotName, chapLNSName)
	}

	// Peer writes a Response: Identifier echoed, Value = 16 bytes
	// of 0x5A (stand-in for MD5 digest), Name = "bob".
	respDigest := bytes.Repeat([]byte{0x5A}, chapMD5DigestLen)
	peerWrite := make([]byte, 64)
	off := WriteFrame(peerWrite, 0, ProtoCHAP, nil)
	resp := []byte{
		CHAPCodeResponse, challengeID, 0x00, 0x00,
		chapMD5DigestLen,
	}
	resp = append(resp, respDigest...)
	resp = append(resp, 'b', 'o', 'b')
	binary.BigEndian.PutUint16(resp[2:4], uint16(len(resp)))
	copy(peerWrite[off:], resp)
	off += len(resp)
	if _, err := peerEnd.Write(peerWrite[:off]); err != nil {
		t.Fatalf("peer write response: %v", err)
	}

	// Read the emitted EventAuthRequest.
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

	if evt.Method != AuthMethodCHAPMD5 {
		t.Errorf("Method = %v, want AuthMethodCHAPMD5", evt.Method)
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
		t.Errorf("Challenge = %x, want %x (the Value we sent on the wire)",
			evt.Challenge, ourValue)
	}
	if !bytes.Equal(evt.Response, respDigest) {
		t.Errorf("Response = %x, want %x", evt.Response, respDigest)
	}
	if evt.TunnelID != s.tunnelID || evt.SessionID != s.sessionID {
		t.Errorf("IDs = (%d,%d), want (%d,%d)",
			evt.TunnelID, evt.SessionID, s.tunnelID, s.sessionID)
	}

	// Deliver an accept decision.
	s.authRespCh <- authResponseMsg{accept: true, message: "welcome"}

	// Read the Success frame on the peer side.
	proto, payload = readPeerFrame(t, peerEnd)
	if proto != ProtoCHAP {
		t.Errorf("reply proto = 0x%04x, want 0x%04x", proto, ProtoCHAP)
	}
	if payload[0] != CHAPCodeSuccess {
		t.Errorf("reply code = %d, want CHAPCodeSuccess", payload[0])
	}
	if payload[1] != challengeID {
		t.Errorf("reply identifier = 0x%02x, want 0x%02x", payload[1], challengeID)
	}
	// CHAP Success/Failure has no Msg-Length: byte 4 is the first
	// Message byte, not a length.
	if payload[4] != 'w' {
		t.Errorf("byte 4 = 0x%02x, want 'w' (no Msg-Length octet in CHAP)",
			payload[4])
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
		t.Errorf("runCHAPAuthPhase returned false, want true on accept")
	}
}

// VALIDATES: runCHAPAuthPhase on AuthResponse(accept=false) writes a
//
//	Failure frame with Code=4 and the reject Message, and
//	returns false.
//
// PREVENTS: regression where the reject path sends Success or drops
//
//	the Message.
func TestCHAPRejectWritesFailure(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut := newAuthTestSession(driverEnd)

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runCHAPAuthPhase()
	}()

	// Drain Challenge.
	_, payload := readPeerFrame(t, peerEnd)
	challengeID := payload[1]

	// Send a Response so the handler progresses to authRespCh.
	digest := bytes.Repeat([]byte{0x11}, chapMD5DigestLen)
	peerWrite := make([]byte, 64)
	off := WriteFrame(peerWrite, 0, ProtoCHAP, nil)
	resp := []byte{
		CHAPCodeResponse, challengeID, 0x00, 0x00,
		chapMD5DigestLen,
	}
	resp = append(resp, digest...)
	resp = append(resp, 'u')
	binary.BigEndian.PutUint16(resp[2:4], uint16(len(resp)))
	copy(peerWrite[off:], resp)
	off += len(resp)
	if _, err := peerEnd.Write(peerWrite[:off]); err != nil {
		t.Fatalf("peer write response: %v", err)
	}

	// Drain EventAuthRequest.
	select {
	case <-authEventsOut:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}

	s.authRespCh <- authResponseMsg{accept: false, message: "nope"}

	_, payload = readPeerFrame(t, peerEnd)
	if payload[0] != CHAPCodeFailure {
		t.Errorf("reply code = %d, want CHAPCodeFailure", payload[0])
	}
	if payload[1] != challengeID {
		t.Errorf("reply identifier = 0x%02x, want 0x%02x", payload[1], challengeID)
	}
	// Message = payload[chapHeaderLen : Length].
	length := int(binary.BigEndian.Uint16(payload[2:4]))
	if string(payload[chapHeaderLen:length]) != "nope" {
		t.Errorf("message = %q, want %q", string(payload[chapHeaderLen:length]), "nope")
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
		t.Errorf("runCHAPAuthPhase returned true, want false on reject")
	}
}

// VALIDATES: runCHAPAuthPhase on authTimeout emits
//
//	EventAuthFailure{Reason:"timeout"}, writes no reply
//	frame, and returns false. Mirrors runAuthPhase and
//	runPAPAuthPhase so tear-down goes through LCP Terminate,
//	not a wire-level CHAP Failure on timeout.
//
// PREVENTS: regression where a stalled consumer parks the session or
//
//	a CHAP Failure frame leaks onto the wire on timeout.
func TestCHAPTimeoutEmitsFailure(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut := newAuthTestSession(driverEnd)
	s.authTimeout = 80 * time.Millisecond

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runCHAPAuthPhase()
	}()

	// Drain Challenge so the handler reaches the Read on chanFile.
	_, payload := readPeerFrame(t, peerEnd)
	challengeID := payload[1]

	// Send a Response so the handler progresses to authRespCh and
	// parks waiting for AuthResponse.
	digest := bytes.Repeat([]byte{0x22}, chapMD5DigestLen)
	peerWrite := make([]byte, 64)
	off := WriteFrame(peerWrite, 0, ProtoCHAP, nil)
	resp := []byte{
		CHAPCodeResponse, challengeID, 0x00, 0x00,
		chapMD5DigestLen,
	}
	resp = append(resp, digest...)
	resp = append(resp, 't')
	binary.BigEndian.PutUint16(resp[2:4], uint16(len(resp)))
	copy(peerWrite[off:], resp)
	off += len(resp)
	if _, err := peerEnd.Write(peerWrite[:off]); err != nil {
		t.Fatalf("peer write response: %v", err)
	}

	// Drain EventAuthRequest.
	select {
	case <-authEventsOut:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}

	// authRespCh intentionally unfed; the 80 ms authTimeout fires.
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
		t.Errorf("runCHAPAuthPhase returned true, want false on timeout")
	}

	// The timeout path MUST NOT write a reply frame.
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

// VALIDATES: runCHAPAuthPhase reports each malformed-wire input as a
//
//	session failure via s.fail and returns false, without
//	emitting EventAuthRequest. The Challenge is always written
//	first regardless (authenticator-initiated), so the peer
//	side still sees one Challenge frame even on error paths.
//
// Note: Identifier mismatch on an OTHERWISE VALID Response is not a
// wire error; Phase 9 AC-16 silent-discards such packets (RFC 1994
// §4.1) and the handler waits for a matching Response or times out.
// That positive behavior is covered by
// TestCHAPIdentifierMismatchSilentDiscard below.
func TestCHAPHandlerWireErrors(t *testing.T) {
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
				// readFrames drops frames shorter than the
				// 2-byte protocol field; close the pipe so the
				// handler observes framesIn closed and fails.
				if _, err := peerEnd.Write([]byte{0xFF}); err != nil {
					t.Fatalf("peer write: %v", err)
				}
				closeConn(peerEnd)
			},
		},
		{
			name: "LCP frame during auth wait then close",
			write: func(t *testing.T, peerEnd net.Conn, _ uint8) {
				t.Helper()
				// Phase 9: LCP frames during the CHAP wait are routed
				// back through handleFrame (so Echo/Terminate stay
				// responsive during periodic re-auth). The handler
				// then keeps waiting for a CHAP Response; closing the
				// pipe terminates the wait via framesIn close.
				buf := make([]byte, 32)
				off := WriteFrame(buf, 0, ProtoLCP, nil)
				off += WriteLCPPacket(buf, off,
					LCPConfigureRequest, 0x01, nil)
				if _, err := peerEnd.Write(buf[:off]); err != nil {
					t.Fatalf("peer write: %v", err)
				}
				closeConn(peerEnd)
			},
		},
		{
			name: "valid CHAP framing but wrong code",
			write: func(t *testing.T, peerEnd net.Conn, challengeID uint8) {
				t.Helper()
				buf := make([]byte, 32)
				off := WriteFrame(buf, 0, ProtoCHAP, nil)
				// Challenge code with echoed Identifier: valid
				// framing but not a Response.
				chap := []byte{
					CHAPCodeChallenge, challengeID, 0x00, 0x06,
					0x01, 0x00,
				}
				copy(buf[off:], chap)
				off += len(chap)
				if _, err := peerEnd.Write(buf[:off]); err != nil {
					t.Fatalf("peer write: %v", err)
				}
			},
		},
		{
			name: "mismatched-identifier response then close triggers timeout",
			write: func(t *testing.T, peerEnd net.Conn, challengeID uint8) {
				t.Helper()
				// Phase 9 AC-16: a mismatched-Identifier Response is
				// silently discarded. The handler then loops waiting
				// for a matching Response, and closing the pipe makes
				// readFrames close framesIn so the loop observes the
				// channel close (or auth-timeout, whichever fires
				// first) and returns false without emitting an event.
				mismatch := challengeID + 1
				digest := bytes.Repeat([]byte{0x33}, chapMD5DigestLen)
				buf := make([]byte, 64)
				off := WriteFrame(buf, 0, ProtoCHAP, nil)
				resp := []byte{
					CHAPCodeResponse, mismatch, 0x00, 0x00,
					chapMD5DigestLen,
				}
				resp = append(resp, digest...)
				resp = append(resp, 'p')
				binary.BigEndian.PutUint16(resp[2:4], uint16(len(resp)))
				copy(buf[off:], resp)
				off += len(resp)
				if _, err := peerEnd.Write(buf[:off]); err != nil {
					t.Fatalf("peer write: %v", err)
				}
				// Close the pipe so the subsequent framesIn read
				// terminates the handler within the subcase's 3s
				// wait (below the 30s default auth-timeout).
				closeConn(peerEnd)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peerEnd, driverEnd := net.Pipe()
			s, authEventsOut := newAuthTestSession(driverEnd)

			handlerDone := make(chan bool, 1)
			go func() {
				handlerDone <- s.runCHAPAuthPhase()
			}()

			// Drain Challenge so the handler reaches the Read.
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
					t.Errorf("runCHAPAuthPhase = true, want false on %s",
						tc.name)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("handler did not return on %s", tc.name)
			}

			// The handler MUST NOT emit EventAuthRequest on a
			// wire error: the consumer has nothing to decide.
			select {
			case ev := <-authEventsOut:
				t.Errorf("unexpected auth event on wire error: %T", ev)
			default:
			}
		})
	}
}

// VALIDATES: runCHAPAuthPhase increments s.chapIdentifier once per
//
//	Challenge, so two consecutive invocations on the same
//	session emit different Identifier values (RFC 1994
//	Section 4.1 MUST).
//
// PREVENTS: regression where the counter is reset, reused, or kept
//
//	on a per-draw stack variable -- all of which would let an
//	attacker replay a previous Response.
func TestCHAPIdentifierMonotonic(t *testing.T) {
	ids := make([]uint8, 0, 2)
	for range 2 {
		peerEnd, driverEnd := net.Pipe()
		s, _ := newAuthTestSession(driverEnd)
		// Inherit the counter from the previous run so we observe
		// the real monotonic property across calls rather than a
		// fresh 1 both times.
		if len(ids) > 0 {
			s.chapIdentifier = ids[len(ids)-1]
		}

		handlerDone := make(chan bool, 1)
		go func() {
			handlerDone <- s.runCHAPAuthPhase()
		}()

		_, payload := readPeerFrame(t, peerEnd)
		ids = append(ids, payload[1])

		// Close peerEnd to unblock the handler's Read with EOF and
		// let it exit.
		closeConn(peerEnd)
		<-handlerDone
	}
	if ids[0] == ids[1] {
		t.Errorf("Identifiers %d and %d are equal; MUST change per RFC 1994 Section 4.1",
			ids[0], ids[1])
	}
	if ids[1] != ids[0]+1 {
		t.Errorf("Identifiers not monotonic: %d -> %d, want +1 (mod 256)", ids[0], ids[1])
	}
}

// VALIDATES: s.chapIdentifier wraps from 0xFF to 0x00 on the next
//
//	Challenge. RFC 1994 Section 4.1 requires the Identifier to
//	change each Challenge; the transition across the uint8
//	boundary is still a change (0xFF != 0x00) so wrap is legal.
//
// PREVENTS: regression where the counter is clamped to 0xFF and
//
//	re-sends 0xFF forever after overflow.
func TestCHAPIdentifierWraps(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	s, _ := newAuthTestSession(driverEnd)
	s.chapIdentifier = 0xFF

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runCHAPAuthPhase()
	}()

	_, payload := readPeerFrame(t, peerEnd)
	if payload[1] != 0x00 {
		t.Errorf("Identifier after wrap = 0x%02x, want 0x00", payload[1])
	}

	closeConn(peerEnd)
	<-handlerDone
}

// VALIDATES: ParseCHAPResponse accepts a maximally-sized Name field
//
//	(Value-Size=1 leaves the largest Name room: Length -
//	chapHeaderLen - 1 - 1 bytes, where Length maxes out at
//	MaxFrameLen - 2).
//
// PREVENTS: regression where an off-by-one in the Name slice bound
//
//	drops the last byte or trips a range panic at the high end
//	of the legal Length space.
func TestCHAPParseResponseMaxName(t *testing.T) {
	length := MaxFrameLen - 2
	nameLen := length - chapHeaderLen - 1 - 1 // Value-Size=1, Value=1 byte
	buf := make([]byte, length)
	buf[0] = CHAPCodeResponse
	buf[1] = 0x55
	binary.BigEndian.PutUint16(buf[2:4], uint16(length))
	buf[4] = 0x01 // Value-Size
	buf[5] = 0xEF // Value
	for i := range nameLen {
		buf[6+i] = 'N'
	}
	resp, err := ParseCHAPResponse(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Value) != 1 || resp.Value[0] != 0xEF {
		t.Errorf("Value = %x, want [EF]", resp.Value)
	}
	if len(resp.Name) != nameLen {
		t.Errorf("Name length = %d, want %d", len(resp.Name), nameLen)
	}
	if resp.Name[0] != 'N' || resp.Name[nameLen-1] != 'N' {
		t.Errorf("Name first/last = %q/%q", resp.Name[0], resp.Name[nameLen-1])
	}
}

// VALIDATES: runCHAPAuthPhase sources its Challenge Value from
//
//	crypto/rand -- two invocations produce different Values.
//	RFC 1994 Section 2.3: "Each challenge value SHOULD be
//	unique ... SHOULD also be unpredictable."
//
// PREVENTS: regression where the Value is hard-coded, derived from
//
//	a predictable seed, or reused across Challenges.
func TestCHAPChallengeRandom(t *testing.T) {
	values := make([][]byte, 0, 2)
	for range 2 {
		peerEnd, driverEnd := net.Pipe()
		s, _ := newAuthTestSession(driverEnd)

		handlerDone := make(chan bool, 1)
		go func() {
			handlerDone <- s.runCHAPAuthPhase()
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
		t.Errorf("Challenge Values equal across invocations: %x -- MUST be unique per RFC 1994",
			values[0])
	}
}

// VALIDATES: drawCHAPChallenge panics with "BUG: ..." when the
//
//	destination slice is shorter than chapChallengeValueLen.
//	The panic prefix matches the project's panic convention for
//	programmer-error guards (rules/go-standards.md).
//
// PREVENTS: regression where a future caller passes a too-short slice
//
//	and silently gets a partial random draw or a slice panic
//	without the "BUG: ..." tag that distinguishes programmer
//	error from runtime failure.
func TestCHAPDrawChallengePanicsOnShortDst(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("drawCHAPChallenge did not panic on short dst")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("recover() type = %T, want string", r)
		}
		if !strings.HasPrefix(msg, "BUG: ") {
			t.Errorf("panic message = %q, want prefix \"BUG: \"", msg)
		}
	}()
	short := make([]byte, chapChallengeValueLen-1)
	_ = drawCHAPChallenge(short)
}
