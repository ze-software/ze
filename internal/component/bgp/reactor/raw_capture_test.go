// Goal: prove the raw ring holds what a pcap export needs -- whole messages
// with their reconstructed 19-byte header, both ends of the session, and the
// true length of a message the slot could not hold whole.
// Method: append through the ring's own entry point and read the octets back,
// because the export reads exactly these octets.

package reactor

import (
	"bytes"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/test/sim"
)

var rawTestStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// The two ends of the session the tests capture. They carry different
// addresses, so no assertion about direction passes by confusing them.
var (
	rawTestPeer  = netip.MustParseAddr("192.0.2.1")
	rawTestLocal = netip.MustParseAddr("192.0.2.2")
)

func TestBGPRawCaptureRingAppend(t *testing.T) {
	c := sim.NewFakeClock(rawTestStart)
	r := newBGPRawCaptureRing(c)
	r.Append(0, rawTestPeer, rawTestLocal, msgtype.TypeKEEPALIVE, nil)
	r.Append(1, rawTestPeer, rawTestLocal, msgtype.TypeUPDATE, []byte{4, 5, 6, 7})

	snap := r.Snapshot(0)
	if len(snap) != 2 {
		t.Fatalf("count = %d, want 2", len(snap))
	}
	if want := message.HeaderLen + 4; len(snap[0].Data) != want {
		t.Errorf("newest len = %d, want %d: the header travels with the body", len(snap[0].Data), want)
	}
	if snap[0].Direction != 1 {
		t.Errorf("newest direction = %d, want 1", snap[0].Direction)
	}
}

// TestRingCarriesFramingFields is the framing contract: every field the pcap
// export reads off an entry is present and correct, and the 19-byte header the
// tap never received is reconstructed byte for byte.
func TestRingCarriesFramingFields(t *testing.T) {
	c := sim.NewFakeClock(rawTestStart)
	r := newBGPRawCaptureRing(c)
	body := []byte{0x00, 0x00, 0x00, 0x00}
	r.Append(0, rawTestPeer, rawTestLocal, msgtype.TypeUPDATE, body)

	snap := r.Snapshot(0)
	if len(snap) != 1 {
		t.Fatalf("count = %d, want 1", len(snap))
	}
	entry := &snap[0]

	if entry.PeerAddr != rawTestPeer {
		t.Errorf("peer address = %s, want %s", entry.PeerAddr, rawTestPeer)
	}
	if entry.LocalAddr != rawTestLocal {
		t.Errorf("local address = %s, want %s", entry.LocalAddr, rawTestLocal)
	}
	if want := message.HeaderLen + len(body); entry.OriginalLen != want {
		t.Errorf("original length = %d, want %d", entry.OriginalLen, want)
	}

	// The reconstruction is what the whole framing rests on, so it is compared
	// against the bytes a header carries on the wire rather than to itself.
	var wire [message.HeaderLen]byte
	message.Header{Length: uint16(message.HeaderLen + len(body)), Type: msgtype.TypeUPDATE}.WriteTo(wire[:], 0)
	if !bytes.Equal(entry.Data[:message.HeaderLen], wire[:]) {
		t.Errorf("header = % x, want % x", entry.Data[:message.HeaderLen], wire[:])
	}
	if !bytes.Equal(entry.Data[message.HeaderLen:], body) {
		t.Errorf("body = % x, want % x", entry.Data[message.HeaderLen:], body)
	}
	// RFC 4271 Section 4.1: the marker is 16 octets of all ones, which is what
	// lets a reader find a message boundary in a reassembled stream.
	if !bytes.Equal(entry.Data[:message.MarkerLen], bytes.Repeat([]byte{0xFF}, message.MarkerLen)) {
		t.Errorf("marker = % x, want 16 octets of 0xFF", entry.Data[:message.MarkerLen])
	}
}

func TestBGPRawCaptureRingOverflow(t *testing.T) {
	c := sim.NewFakeClock(rawTestStart)
	r := newBGPRawCaptureRing(c)
	for i := range bgpRawSlotCount + 10 {
		r.Append(0, rawTestPeer, rawTestLocal, msgtype.TypeKEEPALIVE, []byte{byte(i)}) //nolint:gosec // a loop index below 300
	}
	snap := r.Snapshot(0)
	if len(snap) != bgpRawSlotCount {
		t.Fatalf("count = %d, want %d", len(snap), bgpRawSlotCount)
	}
}

func TestBGPRawCaptureRingLimit(t *testing.T) {
	c := sim.NewFakeClock(rawTestStart)
	r := newBGPRawCaptureRing(c)
	for range 10 {
		r.Append(0, rawTestPeer, rawTestLocal, msgtype.TypeKEEPALIVE, []byte{1})
	}
	snap := r.Snapshot(3)
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
}

func TestBGPRawCaptureRingEmpty(t *testing.T) {
	c := sim.NewFakeClock(rawTestStart)
	r := newBGPRawCaptureRing(c)
	snap := r.Snapshot(0)
	if snap == nil {
		t.Fatal("snapshot should be non-nil empty slice")
	}
	if len(snap) != 0 {
		t.Fatalf("count = %d, want 0", len(snap))
	}
}

// TestBGPRawCaptureRingTruncation covers the message longer than a slot: the
// entry holds what fits and states what was on the wire, so the pcap record
// declares the truncation instead of describing a shorter message.
func TestBGPRawCaptureRingTruncation(t *testing.T) {
	c := sim.NewFakeClock(rawTestStart)
	r := newBGPRawCaptureRing(c)
	body := make([]byte, bgpRawSlotSize+100)
	r.Append(0, rawTestPeer, rawTestLocal, msgtype.TypeUPDATE, body)

	snap := r.Snapshot(0)
	if len(snap[0].Data) != bgpRawSlotSize {
		t.Errorf("data len = %d, want %d", len(snap[0].Data), bgpRawSlotSize)
	}
	if want := message.HeaderLen + len(body); snap[0].OriginalLen != want {
		t.Errorf("original length = %d, want %d", snap[0].OriginalLen, want)
	}
	// The header still states the on-wire length, which is what a reader needs
	// to know the record is short.
	header, err := message.ParseHeader(snap[0].Data)
	if err != nil {
		t.Fatalf("the truncated entry carries no parsable header: %v", err)
	}
	if want := message.HeaderLen + len(body); int(header.Length) != want {
		t.Errorf("header length field = %d, want %d", header.Length, want)
	}
}

// TestBGPRawCaptureRingRefusesUnwireableMessage proves a body no BGP header can
// describe is not captured with a header that lies about it. RFC 8654 Section 4
// caps a message at 65535 octets, so a longer one never crossed the wire.
func TestBGPRawCaptureRingRefusesUnwireableMessage(t *testing.T) {
	c := sim.NewFakeClock(rawTestStart)
	r := newBGPRawCaptureRing(c)
	r.Append(0, rawTestPeer, rawTestLocal, msgtype.TypeUPDATE, make([]byte, message.ExtMsgLen))

	if snap := r.Snapshot(0); len(snap) != 0 {
		t.Errorf("captured %d entries for a body past the wire maximum, want none", len(snap))
	}
}
