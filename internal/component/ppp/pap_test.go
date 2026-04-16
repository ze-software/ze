package ppp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"
)

// VALIDATES: ParsePAPRequest decodes Code, Identifier, Length, and
//
//	the Peer-ID / Password fields of a PAP Authenticate-Request
//	per RFC 1334 Section 2.2.
//
// PREVENTS: regression where Peer-ID / Password boundaries are read
//
//	past the Length field and the parser silently includes
//	padding bytes in the username or password.
func TestPAPParseRequest(t *testing.T) {
	cases := []struct {
		name     string
		buf      []byte
		wantID   uint8
		wantUser string
		wantPass string
	}{
		{
			name: "alice/secret",
			// Length = 4 header + 1 + 5 ("alice") + 1 + 6 ("secret") = 17
			buf: []byte{
				0x01, 0x42, 0x00, 0x11,
				0x05, 'a', 'l', 'i', 'c', 'e',
				0x06, 's', 'e', 'c', 'r', 'e', 't',
			},
			wantID:   0x42,
			wantUser: "alice",
			wantPass: "secret",
		},
		{
			name: "empty user empty pass",
			buf: []byte{
				0x01, 0x00, 0x00, 0x06,
				0x00,
				0x00,
			},
			wantID:   0x00,
			wantUser: "",
			wantPass: "",
		},
		{
			name: "trailing padding ignored",
			buf: []byte{
				0x01, 0x11, 0x00, 0x0A,
				0x01, 'u',
				0x02, 'p', 'q',
				0xFF, 0xFF, 0xFF, // padding beyond Length
			},
			wantID:   0x11,
			wantUser: "u",
			wantPass: "pq",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := ParsePAPRequest(tc.buf)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if req.Identifier != tc.wantID {
				t.Errorf("Identifier = 0x%02x, want 0x%02x", req.Identifier, tc.wantID)
			}
			if req.Username != tc.wantUser {
				t.Errorf("Username = %q, want %q", req.Username, tc.wantUser)
			}
			if req.Password != tc.wantPass {
				t.Errorf("Password = %q, want %q", req.Password, tc.wantPass)
			}
		})
	}
}

// VALIDATES: ParsePAPRequest accepts 255-octet Peer-ID and Password
//
//	(uint8 maxima) without overflow.
//
// PREVENTS: regression where a 256-byte value is silently truncated
//
//	or causes a slice-out-of-bounds panic.
func TestPAPParseRequestBoundary(t *testing.T) {
	// RFC 1334 Section 2.2: Peer-ID Length and Passwd-Length are
	// each one octet, so 255 is the maximum.
	user := bytes.Repeat([]byte{'u'}, 255)
	pass := bytes.Repeat([]byte{'p'}, 255)
	total := papHeaderLen + 1 + len(user) + 1 + len(pass) // 4 + 1 + 255 + 1 + 255 = 516
	buf := make([]byte, total)
	buf[0] = PAPAuthenticateRequest
	buf[1] = 0x7F
	binary.BigEndian.PutUint16(buf[2:4], uint16(total))
	buf[4] = 0xFF
	copy(buf[5:5+255], user)
	buf[5+255] = 0xFF
	copy(buf[5+255+1:], pass)

	req, err := ParsePAPRequest(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(req.Username) != 255 || req.Username[0] != 'u' {
		t.Errorf("Username length = %d, want 255", len(req.Username))
	}
	if len(req.Password) != 255 || req.Password[0] != 'p' {
		t.Errorf("Password length = %d, want 255", len(req.Password))
	}
}

// VALIDATES: ParsePAPRequest rejects malformed packets per RFC 1334
//
//	Section 2.1 "silently discard" guidance: buffers shorter
//	than the 4-byte header, wrong code, Length mismatches,
//	and field-length overflows must not panic or return
//	partial results.
func TestPAPParseRequestInvalid(t *testing.T) {
	cases := []struct {
		name string
		buf  []byte
		want error
	}{
		{
			name: "too short",
			buf:  []byte{0x01, 0x00, 0x00},
			want: errPAPTooShort,
		},
		{
			name: "wrong code",
			buf: []byte{
				0x02, 0x00, 0x00, 0x06,
				0x00,
				0x00,
			},
			want: errPAPWrongCode,
		},
		{
			name: "length below header",
			buf:  []byte{0x01, 0x00, 0x00, 0x03, 0x00, 0x00},
			want: errPAPLengthMismatch,
		},
		{
			name: "length exceeds buffer",
			buf:  []byte{0x01, 0x00, 0x10, 0x00, 0x00, 0x00},
			want: errPAPLengthMismatch,
		},
		{
			name: "peer-id overflows length",
			buf: []byte{
				0x01, 0x00, 0x00, 0x08,
				0xFF, 'a', 'b', 'c', // Peer-ID Length=255 but only 3 bytes follow
			},
			want: errPAPPeerIDOverflow,
		},
		{
			name: "missing passwd-length octet",
			buf: []byte{
				0x01, 0x00, 0x00, 0x07,
				0x02, 'p', 'q',
				// no Passwd-Length
			},
			want: errPAPPasswdOverflow,
		},
		{
			name: "password overflows length",
			buf: []byte{
				0x01, 0x00, 0x00, 0x09,
				0x01, 'u',
				0xFF, 'p', 'q', // Passwd-Length=255 but only 2 bytes follow
			},
			want: errPAPPasswdOverflow,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParsePAPRequest(tc.buf)
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// VALIDATES: WritePAPAck encodes Code=2, the supplied Identifier,
//
//	a correct Length, and the Msg-Length/Message fields per
//	RFC 1334 Section 2.3.
//
// PREVENTS: regression where Length is set to the message length
//
//	instead of the total packet length, or Msg-Length is
//	omitted.
func TestPAPWriteAck(t *testing.T) {
	buf := make([]byte, 32)
	n := WritePAPAck(buf, 0, 0x37, []byte("ok"))
	want := []byte{
		0x02, 0x37, 0x00, 0x07,
		0x02, 'o', 'k',
	}
	if n != len(want) {
		t.Errorf("n = %d, want %d", n, len(want))
	}
	if !bytes.Equal(buf[:n], want) {
		t.Errorf("buf = %x, want %x", buf[:n], want)
	}
}

// VALIDATES: WritePAPAck with an empty message still emits the
//
//	Msg-Length=0 octet (total length 5).
func TestPAPWriteAckEmptyMessage(t *testing.T) {
	buf := make([]byte, 16)
	n := WritePAPAck(buf, 0, 0x01, nil)
	want := []byte{0x02, 0x01, 0x00, 0x05, 0x00}
	if n != len(want) || !bytes.Equal(buf[:n], want) {
		t.Errorf("buf = %x (n=%d), want %x", buf[:n], n, want)
	}
}

// VALIDATES: WritePAPAck writes at the requested offset without
//
//	overwriting preceding buffer contents.
func TestPAPWriteAckOffset(t *testing.T) {
	buf := []byte{0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	n := WritePAPAck(buf, 2, 0x55, []byte("x"))
	if n != 6 {
		t.Errorf("n = %d, want 6", n)
	}
	if buf[0] != 0xFF || buf[1] != 0xFF {
		t.Errorf("prefix overwritten: %x", buf[:2])
	}
	want := []byte{0x02, 0x55, 0x00, 0x06, 0x01, 'x'}
	if !bytes.Equal(buf[2:2+n], want) {
		t.Errorf("buf[2:] = %x, want %x", buf[2:2+n], want)
	}
}

// VALIDATES: WritePAPNak encodes Code=3 and otherwise follows the
//
//	same format as Ack.
func TestPAPWriteNak(t *testing.T) {
	buf := make([]byte, 32)
	n := WritePAPNak(buf, 0, 0x42, []byte("bad"))
	want := []byte{
		0x03, 0x42, 0x00, 0x08,
		0x03, 'b', 'a', 'd',
	}
	if n != len(want) || !bytes.Equal(buf[:n], want) {
		t.Errorf("buf = %x (n=%d), want %x", buf[:n], n, want)
	}
}

// VALIDATES: WritePAPAck caps Msg-Length at 255 so an over-long
//
//	caller message does not corrupt the length octet.
//
// PREVENTS: regression where len(message) > 255 would silently
//
//	truncate via a Go byte() conversion that wraps around
//	instead of clamping.
func TestPAPWriteAckCapsMessageAt255(t *testing.T) {
	msg := bytes.Repeat([]byte{'a'}, 300)
	buf := make([]byte, 1024)
	n := WritePAPAck(buf, 0, 0x00, msg)
	// Expect 5-byte header + 255 message octets = 260.
	if n != 5+255 {
		t.Errorf("n = %d, want %d", n, 5+255)
	}
	if buf[4] != 0xFF {
		t.Errorf("Msg-Length = 0x%02x, want 0xFF", buf[4])
	}
}

// VALIDATES: runPAPAuthPhase parses an Authenticate-Request frame
//
//	from chanFile, emits EventAuthRequest with Method=PAP
//	and the peer's Identifier / Username / Password, waits
//	for AuthResponse(accept=true), and writes an
//	Authenticate-Ack frame back to the peer.
//
// PREVENTS: regression where the event omits PAP-specific fields,
//
//	or where the reply frame has the wrong protocol, code,
//	or Identifier echo.
func TestPAPRequestEmitsEvent(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut := newAuthTestSession(driverEnd)

	// Spawn the handler so the goroutine can read from chanFile
	// and write the reply back to the peer.
	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runPAPAuthPhase()
	}()

	// Peer writes an Authenticate-Request: Identifier=0x42,
	// Peer-ID="alice", Password="secret". Length = 4 header + 1 +
	// 5 ("alice") + 1 + 6 ("secret") = 17.
	peerWrite := make([]byte, 64)
	off := WriteFrame(peerWrite, 0, ProtoPAP, nil)
	req := []byte{
		0x01, 0x42, 0x00, 0x11,
		0x05, 'a', 'l', 'i', 'c', 'e',
		0x06, 's', 'e', 'c', 'r', 'e', 't',
	}
	copy(peerWrite[off:], req)
	off += len(req)
	if _, err := peerEnd.Write(peerWrite[:off]); err != nil {
		t.Fatalf("peer write request: %v", err)
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

	if evt.Method != AuthMethodPAP {
		t.Errorf("Method = %v, want AuthMethodPAP", evt.Method)
	}
	if evt.Identifier != 0x42 {
		t.Errorf("Identifier = 0x%02x, want 0x42", evt.Identifier)
	}
	if evt.Username != "alice" {
		t.Errorf("Username = %q, want %q", evt.Username, "alice")
	}
	if string(evt.Response) != "secret" {
		t.Errorf("Response = %q, want %q", string(evt.Response), "secret")
	}
	if len(evt.Challenge) != 0 {
		t.Errorf("Challenge = %x, want empty (PAP has no challenge)", evt.Challenge)
	}
	if evt.TunnelID != s.tunnelID || evt.SessionID != s.sessionID {
		t.Errorf("IDs = (%d,%d), want (%d,%d)",
			evt.TunnelID, evt.SessionID, s.tunnelID, s.sessionID)
	}

	// Deliver an accept decision.
	s.authRespCh <- authResponseMsg{accept: true, message: "welcome"}

	// Read the Authenticate-Ack frame on the peer side.
	proto, payload := readPeerFrame(t, peerEnd)
	if proto != ProtoPAP {
		t.Errorf("reply proto = 0x%04x, want 0x%04x", proto, ProtoPAP)
	}
	if len(payload) < papHeaderLen {
		t.Fatalf("reply payload too short: %d bytes", len(payload))
	}
	if payload[0] != PAPAuthenticateAck {
		t.Errorf("reply code = %d, want PAPAuthenticateAck", payload[0])
	}
	if payload[1] != 0x42 {
		t.Errorf("reply identifier = 0x%02x, want 0x42 (echoed from request)", payload[1])
	}

	// Drain EventAuthSuccess so the goroutine does not block.
	select {
	case ev := <-authEventsOut:
		if _, ok := ev.(EventAuthSuccess); !ok {
			t.Errorf("second auth event %T, want EventAuthSuccess", ev)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for EventAuthSuccess")
	}

	if !<-handlerDone {
		t.Errorf("runPAPAuthPhase returned false, want true on accept")
	}
}

// VALIDATES: runPAPAuthPhase, on AuthResponse(accept=false), writes
//
//	an Authenticate-Nak carrying the reject message and
//	returns false.
//
// PREVENTS: regression where the reject path sends Ack, drops the
//
//	message, or echoes the wrong identifier.
func TestPAPRejectWritesNak(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut := newAuthTestSession(driverEnd)

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runPAPAuthPhase()
	}()

	peerWrite := make([]byte, 32)
	off := WriteFrame(peerWrite, 0, ProtoPAP, nil)
	req := []byte{
		0x01, 0x77, 0x00, 0x08,
		0x01, 'u',
		0x01, 'p',
	}
	copy(peerWrite[off:], req)
	off += len(req)
	if _, err := peerEnd.Write(peerWrite[:off]); err != nil {
		t.Fatalf("peer write request: %v", err)
	}

	// Drain EventAuthRequest.
	select {
	case <-authEventsOut:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for EventAuthRequest")
	}

	s.authRespCh <- authResponseMsg{accept: false, message: "nope"}

	_, payload := readPeerFrame(t, peerEnd)
	if payload[0] != PAPAuthenticateNak {
		t.Errorf("reply code = %d, want PAPAuthenticateNak", payload[0])
	}
	if payload[1] != 0x77 {
		t.Errorf("reply identifier = 0x%02x, want 0x77", payload[1])
	}
	msgLen := int(payload[4])
	if msgLen != 4 || string(payload[5:5+msgLen]) != "nope" {
		t.Errorf("message = %q (len %d), want %q",
			string(payload[5:5+msgLen]), msgLen, "nope")
	}

	// Drain EventAuthFailure.
	select {
	case ev := <-authEventsOut:
		if _, ok := ev.(EventAuthFailure); !ok {
			t.Errorf("second auth event %T, want EventAuthFailure", ev)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for EventAuthFailure")
	}

	if <-handlerDone {
		t.Errorf("runPAPAuthPhase returned true, want false on reject")
	}
}

// VALIDATES: runPAPAuthPhase, when no AuthResponse arrives before
//
//	authTimeout fires, emits EventAuthFailure{Reason:"timeout"},
//	writes no reply frame, and returns false. Matches the
//	timeout semantics of the existing runAuthPhase
//	(session_run.go:runAuthPhase) so the session is torn down
//	via EventSessionDown + LCP Terminate-Request rather than a
//	wire-level PAP Nak.
//
// PREVENTS: regression where a stalled consumer leaves the session
//
//	parked forever, or where the handler silently discards the
//	timeout signal.
func TestPAPTimeoutEmitsFailure(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)
	s, authEventsOut := newAuthTestSession(driverEnd)
	s.authTimeout = 80 * time.Millisecond

	handlerDone := make(chan bool, 1)
	go func() {
		handlerDone <- s.runPAPAuthPhase()
	}()

	peerWrite := make([]byte, 32)
	off := WriteFrame(peerWrite, 0, ProtoPAP, nil)
	req := []byte{
		0x01, 0x55, 0x00, 0x08,
		0x01, 'u',
		0x01, 'p',
	}
	copy(peerWrite[off:], req)
	off += len(req)
	if _, err := peerEnd.Write(peerWrite[:off]); err != nil {
		t.Fatalf("peer write request: %v", err)
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
		t.Errorf("runPAPAuthPhase returned true, want false on timeout")
	}

	// The timeout path MUST NOT write a reply to chanFile. If a byte
	// shows up inside the handler's return window, assert it.
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

// VALIDATES: runPAPAuthPhase reports each malformed-wire input as a
//
//	session failure via s.fail (lifecycle EventSessionDown)
//	and returns false, without emitting EventAuthRequest.
//
// PREVENTS: regression where a wire-layer error is swallowed and the
//
//	session parks waiting for an AuthResponse that will never
//	arrive.
func TestPAPHandlerWireErrors(t *testing.T) {
	cases := []struct {
		name  string
		write func(t *testing.T, peerEnd net.Conn) // nil means close chanFile instead
	}{
		{
			name: "peer closes without sending",
			// chanFile read returns io.EOF -> s.fail called.
			write: nil,
		},
		{
			name: "frame too short for protocol field",
			write: func(t *testing.T, peerEnd net.Conn) {
				t.Helper()
				// A 1-byte write is below ParseFrame's 2-byte
				// minimum.
				if _, err := peerEnd.Write([]byte{0xFF}); err != nil {
					t.Fatalf("peer write: %v", err)
				}
			},
		},
		{
			name: "wrong protocol on chanFile",
			write: func(t *testing.T, peerEnd net.Conn) {
				t.Helper()
				// Full LCP Configure-Request instead of PAP.
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
			name: "valid PAP framing but wrong code",
			write: func(t *testing.T, peerEnd net.Conn) {
				t.Helper()
				// PAP protocol, but code=2 (Authenticate-Ack)
				// which our handler must reject as not a request.
				buf := make([]byte, 32)
				off := WriteFrame(buf, 0, ProtoPAP, nil)
				pap := []byte{0x02, 0x00, 0x00, 0x06, 0x00, 0x00}
				copy(buf[off:], pap)
				off += len(pap)
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
				handlerDone <- s.runPAPAuthPhase()
			}()

			if tc.write == nil {
				// Close peer end so the handler's Read returns EOF.
				closeConn(peerEnd)
			} else {
				tc.write(t, peerEnd)
				defer closeConn(peerEnd)
			}

			select {
			case ok := <-handlerDone:
				if ok {
					t.Errorf("runPAPAuthPhase = true, want false on %s",
						tc.name)
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("handler did not return on %s", tc.name)
			}

			// The handler MUST NOT emit EventAuthRequest on a wire
			// error: the consumer has nothing to decide about. Check
			// the channel is empty.
			select {
			case ev := <-authEventsOut:
				t.Errorf("unexpected auth event on wire error: %T", ev)
			default:
			}
		})
	}
}

// Helpers newAuthTestSession and readPeerFrame moved to
// auth_pipe_helpers_test.go so chap_test.go can reuse them. Both
// still call t.Fatal on pipe errors.
