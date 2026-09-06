// Design: docs/architecture/diagnostics/packet-capture.md -- opt-in raw byte capture for pcap export

package reactor

import (
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/clock"
)

const (
	bgpRawSlotSize  = 4096
	bgpRawSlotCount = 256
)

type bgpRawSlot struct {
	timestamp time.Time
	peerAddr  netip.Addr
	localAddr netip.Addr
	direction uint8
	// length is the bytes held in data, which is shorter than original when
	// the message did not fit the slot.
	length int
	// original is the whole message's length on the wire, header included.
	original int
	data     [bgpRawSlotSize]byte
}

// BGPRawCaptureEntry is the exported view of one raw packet. Data holds a whole
// BGP message, its 19-byte header included, and OriginalLen says how long that
// message was on the wire when the slot truncated it.
type BGPRawCaptureEntry struct {
	Timestamp   time.Time
	PeerAddr    netip.Addr
	LocalAddr   netip.Addr
	Direction   uint8
	Data        []byte
	OriginalLen int
}

// BGPRawCaptureRing stores whole raw BGP messages when activated.
// Append copies into fixed-size array slots. Messages exceeding
// bgpRawSlotSize are truncated (extended messages up to 65535 bytes
// lose the tail, but headers and most attributes are preserved) and the
// entry's OriginalLen carries the length the message had on the wire.
// Safe for concurrent use.
type BGPRawCaptureRing struct {
	mu    sync.Mutex
	clock clock.Clock
	slots []bgpRawSlot
	head  int
	count int
}

// newBGPRawCaptureRing allocates a raw capture ring.
func newBGPRawCaptureRing(c clock.Clock) *BGPRawCaptureRing {
	return &BGPRawCaptureRing{clock: c, slots: make([]bgpRawSlot, bgpRawSlotCount)}
}

// Append copies one whole BGP message into the ring, truncating to slot size.
//
// The tap receives a header-stripped body with the type as a separate argument,
// so the 19-byte header is rebuilt here from data already in scope: RFC 4271
// Section 4.1 fixes the marker at all ones, the Length field counts the header,
// and the Type is the argument. This is the single reconstruction point, and a
// second tap that needs a whole message calls it rather than repeating it.
//
// peerAddr and localAddr are the two ends of the session, which the pcap export
// frames each message between.
func (r *BGPRawCaptureRing) Append(direction uint8, peerAddr, localAddr netip.Addr, msgType msgtype.MessageType, body []byte) {
	original := message.HeaderLen + len(body)
	// RFC 8654 Section 4 caps a BGP message at 65535 octets, which is what the
	// header's 2-octet Length field can state. A longer body never crossed the
	// wire, so no honest header describes it and it is not captured.
	if original > message.ExtMsgLen {
		return
	}

	now := r.clock.Now()
	captured := min(original, bgpRawSlotSize)

	r.mu.Lock()
	s := &r.slots[r.head]
	s.timestamp = now
	s.peerAddr = peerAddr
	s.localAddr = localAddr
	s.direction = direction
	s.length = captured
	s.original = original

	message.Header{Length: uint16(original), Type: msgType}.WriteTo(s.data[:], 0) //nolint:gosec // bounded by ExtMsgLen above
	copy(s.data[message.HeaderLen:captured], body)

	r.head = (r.head + 1) % len(r.slots)
	if r.count < len(r.slots) {
		r.count++
	}
	r.mu.Unlock()
}

// Snapshot returns raw entries newest-first. limit <= 0 returns all.
func (r *BGPRawCaptureRing) Snapshot(limit int) []BGPRawCaptureEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == 0 {
		return []BGPRawCaptureEntry{}
	}
	out := make([]BGPRawCaptureEntry, 0, r.count)
	for i := range r.count {
		idx := (r.head - 1 - i + len(r.slots)) % len(r.slots)
		s := &r.slots[idx]
		buf := make([]byte, s.length)
		copy(buf, s.data[:s.length])
		out = append(out, BGPRawCaptureEntry{
			Timestamp:   s.timestamp,
			PeerAddr:    s.peerAddr,
			LocalAddr:   s.localAddr,
			Direction:   s.direction,
			Data:        buf,
			OriginalLen: s.original,
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}
