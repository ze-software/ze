// Design: plan/spec-diag-0-umbrella.md -- L2TP control packet capture ring
// Related: reactor.go -- append on inbound/outbound control messages

package l2tp

import (
	"fmt"
	"net/netip"
	"sync"
	"time"
)

const captureRingCapacity = 256

// captureRecord stores numeric fields only; string formatting
// happens at snapshot time so Append() is zero-alloc.
type captureRecord struct {
	timestamp  time.Time
	direction  uint8 // 0=in, 1=out
	tunnelID   uint16
	sessionID  uint16
	msgType    MessageType
	peerAddr   netip.AddrPort
	byteCount  uint16
	resultCode uint16
}

// CaptureEntry is the formatted view returned by Snapshot.
type CaptureEntry struct {
	Timestamp  string `json:"timestamp"`
	Direction  string `json:"direction"`
	TunnelID   int    `json:"tunnel-id"`
	SessionID  int    `json:"session-id"`
	MsgType    string `json:"msg-type"`
	PeerAddr   string `json:"peer-addr"`
	ByteCount  int    `json:"byte-count"`
	ResultCode int    `json:"result-code,omitempty"`
}

func (r *captureRecord) format() CaptureEntry {
	dir := "in"
	if r.direction == 1 {
		dir = "out"
	}
	e := CaptureEntry{
		Timestamp: r.timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
		Direction: dir,
		TunnelID:  int(r.tunnelID),
		SessionID: int(r.sessionID),
		MsgType:   r.msgType.String(),
		PeerAddr:  r.peerAddr.String(),
		ByteCount: int(r.byteCount),
	}
	if r.resultCode > 0 {
		e.ResultCode = int(r.resultCode)
	}
	return e
}

// CaptureRing is a fixed-size circular buffer of L2TP control message records.
// Safe for concurrent use. Append is zero-alloc (stores value types only).
// Nil-safe: all methods are no-ops on nil receiver.
type CaptureRing struct {
	mu      sync.Mutex
	records []captureRecord
	head    int
	count   int
}

// NewCaptureRing creates a capture ring with the default capacity.
func NewCaptureRing() *CaptureRing {
	return &CaptureRing{records: make([]captureRecord, captureRingCapacity)}
}

// AppendInbound records an inbound control message.
func (r *CaptureRing) AppendInbound(tunnelID, sessionID uint16, msgType MessageType, peer netip.AddrPort, byteCount int, resultCode uint16) {
	r.mu.Lock()
	r.records[r.head] = captureRecord{
		timestamp:  time.Now(),
		direction:  0,
		tunnelID:   tunnelID,
		sessionID:  sessionID,
		msgType:    msgType,
		peerAddr:   peer,
		byteCount:  clampUint16(byteCount),
		resultCode: resultCode,
	}
	r.head = (r.head + 1) % len(r.records)
	if r.count < len(r.records) {
		r.count++
	}
	r.mu.Unlock()
}

// AppendOutbound records an outbound control message.
func (r *CaptureRing) AppendOutbound(tunnelID, sessionID uint16, msgType MessageType, peer netip.AddrPort, byteCount int) {
	r.mu.Lock()
	r.records[r.head] = captureRecord{
		timestamp: time.Now(),
		direction: 1,
		tunnelID:  tunnelID,
		sessionID: sessionID,
		msgType:   msgType,
		peerAddr:  peer,
		byteCount: clampUint16(byteCount),
	}
	r.head = (r.head + 1) % len(r.records)
	if r.count < len(r.records) {
		r.count++
	}
	r.mu.Unlock()
}

// Snapshot returns up to limit formatted records, newest first.
// limit <= 0 returns all. tunnelID > 0 filters by tunnel.
// peer filters by peer address (empty = no filter).
func (r *CaptureRing) Snapshot(limit int, tunnelID uint16, peer string) []CaptureEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == 0 {
		return []CaptureEntry{}
	}
	out := make([]CaptureEntry, 0, r.count)
	for i := range r.count {
		idx := (r.head - 1 - i + len(r.records)) % len(r.records)
		rec := &r.records[idx]
		if tunnelID > 0 && rec.tunnelID != tunnelID {
			continue
		}
		if peer != "" && rec.peerAddr.Addr().String() != peer {
			continue
		}
		out = append(out, rec.format())
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// extractMsgType reads the Message Type AVP from the first 8 bytes of
// an L2TP control payload. Returns 0 (ZLB) when the payload is too short.
func extractMsgType(payload []byte) MessageType {
	if len(payload) < 8 {
		return 0
	}
	return MessageType(uint16(payload[6])<<8 | uint16(payload[7]))
}

func clampUint16(n int) uint16 {
	if n > 65535 {
		return 65535
	}
	return uint16(n)
}

func (m MessageType) String() string {
	switch m {
	case MsgSCCRQ:
		return "SCCRQ"
	case MsgSCCRP:
		return "SCCRP"
	case MsgSCCCN:
		return "SCCCN"
	case MsgStopCCN:
		return "StopCCN"
	case MsgHello:
		return "HELLO"
	case MsgOCRQ:
		return "OCRQ"
	case MsgOCRP:
		return "OCRP"
	case MsgOCCN:
		return "OCCN"
	case MsgICRQ:
		return "ICRQ"
	case MsgICRP:
		return "ICRP"
	case MsgICCN:
		return "ICCN"
	case MsgCDN:
		return "CDN"
	case MsgWEN:
		return "WEN"
	case MsgSLI:
		return "SLI"
	case 0:
		return "ZLB"
	default:
		return fmt.Sprintf("MSG-%d", uint16(m))
	}
}
