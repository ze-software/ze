// Design: plan/spec-diag-0-umbrella.md -- BGP message capture ring
// Related: reactor_notify.go -- notifyMessageReceiver appends here

package reactor

import (
	"net/netip"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
)

const bgpCaptureRingCapacity = 256

// bgpCaptureRecord stores numeric fields only. String formatting at snapshot time.
type bgpCaptureRecord struct {
	timestamp time.Time
	direction uint8 // 0=in, 1=out
	peerAddr  netip.Addr
	msgType   message.MessageType
	byteCount uint16
	errorCode uint8
	errorSub  uint8
}

// BGPCaptureEntry is the formatted view returned by Snapshot.
type BGPCaptureEntry struct {
	Timestamp string `json:"timestamp"`
	Direction string `json:"direction"`
	PeerAddr  string `json:"peer-addr"`
	MsgType   string `json:"msg-type"`
	ByteCount int    `json:"byte-count"`
	ErrorCode int    `json:"error-code,omitempty"`
	ErrorSub  int    `json:"error-sub,omitempty"`
}

func (r *bgpCaptureRecord) format() BGPCaptureEntry {
	dir := "in"
	if r.direction == 1 {
		dir = "out"
	}
	e := BGPCaptureEntry{
		Timestamp: r.timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
		Direction: dir,
		PeerAddr:  r.peerAddr.String(),
		MsgType:   r.msgType.String(),
		ByteCount: int(r.byteCount),
	}
	if r.errorCode > 0 {
		e.ErrorCode = int(r.errorCode)
		e.ErrorSub = int(r.errorSub)
	}
	return e
}

// BGPCaptureRing is a fixed-size circular buffer of BGP message records.
// Safe for concurrent use. Append is zero-alloc.
type BGPCaptureRing struct {
	mu      sync.Mutex
	records []bgpCaptureRecord
	head    int
	count   int
}

// NewBGPCaptureRing creates a capture ring.
func NewBGPCaptureRing() *BGPCaptureRing {
	return &BGPCaptureRing{records: make([]bgpCaptureRecord, bgpCaptureRingCapacity)}
}

// Append records a BGP message. dirOut true = sent, false = received.
func (r *BGPCaptureRing) Append(dirOut bool, peer netip.Addr, msgType message.MessageType, byteCount int, errorCode, errorSub uint8) {
	var d uint8
	if dirOut {
		d = 1
	}
	now := time.Now()
	r.mu.Lock()
	r.records[r.head] = bgpCaptureRecord{
		timestamp: now,
		direction: d,
		peerAddr:  peer,
		msgType:   msgType,
		byteCount: clampU16(byteCount),
		errorCode: errorCode,
		errorSub:  errorSub,
	}
	r.head = (r.head + 1) % len(r.records)
	if r.count < len(r.records) {
		r.count++
	}
	r.mu.Unlock()
}

// Snapshot returns up to limit formatted records, newest first.
// peer filters by address (zero = no filter).
func (r *BGPCaptureRing) Snapshot(limit int, peer netip.Addr) []BGPCaptureEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == 0 {
		return []BGPCaptureEntry{}
	}
	out := make([]BGPCaptureEntry, 0, r.count)
	for i := range r.count {
		idx := (r.head - 1 - i + len(r.records)) % len(r.records)
		rec := &r.records[idx]
		if peer.IsValid() && rec.peerAddr != peer {
			continue
		}
		out = append(out, rec.format())
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func clampU16(n int) uint16 {
	if n > 65535 {
		return 65535
	}
	return uint16(n)
}
