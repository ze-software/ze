package perf

import (
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

func TestBuildOpen(t *testing.T) {
	t.Parallel()

	cfg := SessionConfig{
		ASN:      65001,
		RouterID: netip.MustParseAddr("1.1.1.1"),
		HoldTime: 90,
		Family:   "ipv4/unicast",
	}

	data := BuildOpen(cfg)

	// Minimum OPEN is 29 bytes (header 19 + body 10).
	// With capabilities it must be larger.
	if len(data) <= 29 {
		t.Fatalf("OPEN too short: got %d bytes, want >29", len(data))
	}

	// First 16 bytes must be 0xFF marker.
	for i := range 16 {
		if data[i] != 0xFF {
			t.Fatalf("marker byte %d: got 0x%02x, want 0xFF", i, data[i])
		}
	}

	// Type byte at offset 18 must be 1 (OPEN).
	if data[18] != 1 {
		t.Fatalf("message type: got %d, want 1 (OPEN)", data[18])
	}
}

func TestBuildOpenIPv6(t *testing.T) {
	t.Parallel()

	cfg := SessionConfig{
		ASN:      65002,
		RouterID: netip.MustParseAddr("2.2.2.2"),
		HoldTime: 90,
		Family:   "ipv6/unicast",
	}

	data := BuildOpen(cfg)

	// Must contain a multiprotocol capability with AFI=2 (IPv6).
	// Walk optional parameters to find it.
	// OPEN body starts at offset 19.
	// Body layout: Version(1) + MyAS(2) + HoldTime(2) + BGPID(4) + OptLen(1) + OptParams
	optParamsStart := 19 + 10 // after header + fixed body fields

	// OptLen is at offset 19+9 (the byte before optional params start).
	// But if extended, it changes. For our test the params are small.
	optLen := int(data[optParamsStart-1])
	if optLen == 0 {
		t.Fatal("no optional parameters in OPEN")
	}

	// Walk optional parameters looking for capability param (type=2)
	// containing multiprotocol capability (code=1) with AFI=2.
	found := false
	pos := optParamsStart
	end := optParamsStart + optLen

	for pos < end {
		paramType := data[pos]
		paramLen := int(data[pos+1])
		pos += 2

		if paramType == 2 && paramLen >= 4 {
			// Capability parameter. Walk capability TLVs inside.
			capCode := data[pos]
			if capCode == 1 { // Multiprotocol capability
				// Value: AFI(2) + Reserved(1) + SAFI(1)
				afi := uint16(data[pos+2])<<8 | uint16(data[pos+3])
				if afi == 2 { // AFI IPv6
					found = true
					break
				}
			}
		}

		pos += paramLen
	}

	if !found {
		t.Fatal("OPEN does not contain multiprotocol capability with AFI=2 (IPv6)")
	}
}

func TestBuildKeepalive(t *testing.T) {
	t.Parallel()

	data := buildKeepalive()

	if len(data) != 19 {
		t.Fatalf("KEEPALIVE length: got %d, want 19", len(data))
	}

	// Marker check.
	for i := range 16 {
		if data[i] != 0xFF {
			t.Fatalf("marker byte %d: got 0x%02x, want 0xFF", i, data[i])
		}
	}

	// Type byte at offset 18 must be 4 (KEEPALIVE).
	if data[18] != 4 {
		t.Fatalf("message type: got %d, want 4 (KEEPALIVE)", data[18])
	}
}

// VALIDATES: "readMessageSlab reads into slab, falls back to heap when exhausted."
// PREVENTS: Slab overflow, incorrect fallback, or data corruption.
func TestReadMessageSlab(t *testing.T) {
	t.Parallel()

	// Build a valid KEEPALIVE (19 bytes) as test data.
	ka := buildKeepalive()

	tests := []struct {
		name     string
		slabSize int
		wantSlab bool // true if message should be in slab, false for heap
	}{
		{"fits in slab", 100, true},
		{"slab exhausted", 5, false},    // too small for 19-byte message
		{"slab exactly fits", 19, true}, // exactly one message
		{"nil slab", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := newBytesReader(ka)
			hdr := make([]byte, 19)

			var slab []byte
			if tt.slabSize > 0 {
				slab = make([]byte, tt.slabSize)
			}

			msgType, msg, newOff, err := readMessageSlab(newScriptedConn(r), r, hdr, slab, 0, time.Second)
			if err != nil {
				t.Fatalf("readMessageSlab: %v", err)
			}

			if msgType != 4 { // KEEPALIVE
				t.Errorf("type: got %d, want 4", msgType)
			}

			if len(msg) != 19 {
				t.Errorf("msg length: got %d, want 19", len(msg))
			}

			if tt.wantSlab {
				// Message should be a sub-slice of the slab.
				if newOff != 19 {
					t.Errorf("slab offset: got %d, want 19", newOff)
				}
			} else {
				// Heap fallback: slab offset unchanged.
				if newOff != 0 {
					t.Errorf("slab offset: got %d, want 0 (heap fallback)", newOff)
				}
			}
		})
	}
}

// VALIDATES: "readMessageSlab rejects invalid message length."
// PREVENTS: Accepting messages with length < HeaderLen.
func TestReadMessageSlabInvalidLength(t *testing.T) {
	t.Parallel()

	// Craft a header with length = 10 (< HeaderLen=19).
	var badHdr [19]byte
	for i := range 16 {
		badHdr[i] = 0xFF
	}

	badHdr[16] = 0
	badHdr[17] = 10 // length = 10, invalid
	badHdr[18] = 4  // KEEPALIVE type

	r := newBytesReader(badHdr[:])
	hdr := make([]byte, 19)
	slab := make([]byte, 100)

	_, _, _, err := readMessageSlab(newScriptedConn(r), r, hdr, slab, 0, time.Second)
	if err == nil {
		t.Fatal("expected error for invalid message length")
	}
}

// VALIDATES: "ReadMessageBuf rejects header buffer shorter than HeaderLen."
// PREVENTS: Panic on short header buffer.
func TestReadMessageBufShortHdr(t *testing.T) {
	t.Parallel()

	r := newBytesReader(buildKeepalive())
	shortHdr := make([]byte, 5) // too small

	_, _, err := readMessageBuf(r, shortHdr)
	if err == nil {
		t.Fatal("expected error for short header buffer")
	}
}

// newBytesReader creates an io.Reader from a byte slice.
func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{data: data}
}

type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}

	n := copy(p, r.data[r.pos:])
	r.pos += n

	return n, nil
}

// scriptedConn is a net.Conn whose reads come from src and whose read deadline
// buys exactly one read. A second read under the same deadline fails with a
// timeout, which is what a socket does once its deadline has passed: the poll
// wait for the header consumes the deadline, and the body read that follows
// finds it expired even when the bytes are already in the socket buffer.
//
// Only SetReadDeadline and Read are implemented. Any other net.Conn method
// panics on the nil embedded interface, which is the intent: a caller of these
// readers uses no other method.
type scriptedConn struct {
	net.Conn

	src   io.Reader
	armed bool
}

func newScriptedConn(src io.Reader) *scriptedConn {
	return &scriptedConn{src: src}
}

func (c *scriptedConn) SetReadDeadline(time.Time) error {
	c.armed = true

	return nil
}

func (c *scriptedConn) Read(p []byte) (int, error) {
	if !c.armed {
		return 0, expiredDeadline{}
	}

	c.armed = false

	return c.src.Read(p)
}

// idleReader never delivers a byte. It stands for a connection that is quiet
// for the whole poll, which is how a receive loop checks for cancellation.
type idleReader struct{}

func (idleReader) Read([]byte) (int, error) { return 0, expiredDeadline{} }

// expiredDeadline is the timeout a socket reports once its read deadline has
// passed.
type expiredDeadline struct{}

func (expiredDeadline) Error() string { return "i/o timeout" }
func (expiredDeadline) Timeout() bool { return true }
func (expiredDeadline) Temporary() bool {
	return true
}

// VALIDATES: "A poll deadline that expires after the header is read still reads
// the message whole, so the next read is on a message boundary."
// PREVENTS: The reader framing its next read on a message body, where a body
// byte pair reads as a Length field and every later message is garbage.
//
// The 100ms poll in a receive loop and the 100ms warmup before the first UPDATE
// have the same period, so a message arriving just as the poll expires is not a
// rare accident. It cost internal/perf one iteration in ten, and each such
// iteration counted no route at all for a DUT that had forwarded every one.
func TestReadMessagePolledReadsTheBodyAfterThePollExpires(t *testing.T) {
	t.Parallel()

	// Two NOTIFICATIONs: 19 header bytes and a 2-byte body each. The body read
	// is a second read, and it lands after the poll deadline is spent.
	wire := append(BuildCeaseNotification(), BuildCeaseNotification()...)
	conn := newScriptedConn(newBytesReader(wire))
	hdr := make([]byte, message.HeaderLen)

	for i := range 2 {
		msgType, msg, err := readMessagePolled(conn, conn, hdr, time.Second)
		if err != nil {
			t.Fatalf("message %d: readMessagePolled: %v", i, err)
		}

		if msgType != msgtype.TypeNOTIFICATION {
			t.Errorf("message %d: type = %d, want %d", i, msgType, msgtype.TypeNOTIFICATION)
		}

		if len(msg) != len(wire)/2 {
			t.Errorf("message %d: length = %d, want %d", i, len(msg), len(wire)/2)
		}
	}
}

// VALIDATES: "A poll that expires with no byte consumed is reported as a
// timeout, which a receive loop MAY retry."
// PREVENTS: A receive loop reading a quiet DUT as a stream failure.
func TestReadMessagePolledReportsAnIdlePollAsATimeout(t *testing.T) {
	t.Parallel()

	conn := newScriptedConn(idleReader{})
	hdr := make([]byte, message.HeaderLen)

	_, _, err := readMessagePolled(conn, conn, hdr, time.Second)
	if err == nil {
		t.Fatal("expected a timeout error")
	}

	if !isTimeout(err) {
		t.Errorf("err = %v, want a timeout", err)
	}
}

func TestBuildCeaseNotification(t *testing.T) {
	t.Parallel()

	data := BuildCeaseNotification()

	// Minimum NOTIFICATION is header(19) + error code(1) + subcode(1) = 21.
	if len(data) < 21 {
		t.Fatalf("NOTIFICATION too short: got %d bytes, want >=21", len(data))
	}

	// Type byte at offset 18 must be 3 (NOTIFICATION).
	if data[18] != 3 {
		t.Fatalf("message type: got %d, want 3 (NOTIFICATION)", data[18])
	}

	// Error code at offset 19 must be 6 (Cease).
	if data[19] != 6 {
		t.Fatalf("error code: got %d, want 6 (Cease)", data[19])
	}

	// Error subcode at offset 20 must be 2 (Administrative Shutdown).
	if data[20] != 2 {
		t.Fatalf("error subcode: got %d, want 2 (Administrative Shutdown)", data[20])
	}
}
