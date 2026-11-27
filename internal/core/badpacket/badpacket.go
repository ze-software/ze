// Design: docs/architecture/diagnostics/production-diagnostics.md -- always-on bounded bad wire packet ring

// Package badpacket holds a fixed-size in-memory circular ring of the most
// recent offending wire frames recorded on cold malformed-message and panic
// recovery paths. Memory is strictly bounded (16 records x 4096 payload bytes,
// 64 KiB maximum payload storage), with zero allocations on happy paths.
package badpacket

import (
	"encoding/hex"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// MaxRecords is the fixed circular capacity of the bad-packet ring.
	MaxRecords = 16
	// MaxPayloadBytes is the maximum number of raw wire bytes retained per record.
	MaxPayloadBytes = 4096
)

// PacketRecord is one captured offending wire frame stored in the ring.
type PacketRecord struct {
	ID          string    `json:"id"`
	CapturedAt  time.Time `json:"captured-at"`
	Protocol    string    `json:"protocol"`
	Peer        string    `json:"peer,omitempty"`
	MessageType string    `json:"message-type,omitempty"`
	Action      string    `json:"action"`
	Reason      string    `json:"reason"`
	Length      int       `json:"length"`
	Truncated   bool      `json:"truncated,omitempty"`
	Hex         string    `json:"hex"`
}

type ring struct {
	mu      sync.Mutex
	entries [MaxRecords]PacketRecord
	head    int
	length  int
	count   atomic.Uint64
}

var defaultRing ring

// CopyBounded returns a freshly allocated copy of at most MaxPayloadBytes from
// raw, or nil when raw is empty. Callers on panic-unwind paths use it to detach
// bytes before releasing a pooled read buffer.
func CopyBounded(raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	n := min(len(raw), MaxPayloadBytes)
	out := make([]byte, n)
	copy(out, raw[:n])
	return out
}

// Record stores an offending wire frame in the bounded circular ring and
// returns its monotonic identifier ("pkt-<N>"). Payloads larger than
// MaxPayloadBytes are truncated while Length preserves the original byte count.
// Safe for concurrent use.
func Record(protocol, peer, messageType, action, reason string, raw []byte) string {
	origLen := len(raw)
	truncated := origLen > MaxPayloadBytes
	keep := min(origLen, MaxPayloadBytes)
	hexPayload := hex.EncodeToString(raw[:keep])
	now := time.Now().UTC()

	defaultRing.mu.Lock()
	seq := defaultRing.count.Add(1)
	id := textbuf.StrUint("pkt-", seq)
	defaultRing.entries[defaultRing.head] = PacketRecord{
		ID:          id,
		CapturedAt:  now,
		Protocol:    protocol,
		Peer:        peer,
		MessageType: messageType,
		Action:      action,
		Reason:      reason,
		Length:      origLen,
		Truncated:   truncated,
		Hex:         hexPayload,
	}
	defaultRing.head = (defaultRing.head + 1) % MaxRecords
	if defaultRing.length < MaxRecords {
		defaultRing.length++
	}
	defaultRing.mu.Unlock()

	return id
}

// Snapshot returns recorded bad packets ordered newest-first as a non-nil slice.
// When protocol is non-empty, only records matching protocol case-insensitively
// are returned. When limit > 0, at most limit records are returned.
// Safe for concurrent use.
func Snapshot(limit int, protocol string) []PacketRecord {
	defaultRing.mu.Lock()
	defer defaultRing.mu.Unlock()

	capHint := defaultRing.length
	if limit > 0 {
		capHint = min(capHint, limit)
	}
	out := make([]PacketRecord, 0, capHint)
	for i := range defaultRing.length {
		idx := (defaultRing.head - 1 - i + MaxRecords) % MaxRecords
		rec := defaultRing.entries[idx]
		if protocol != "" {
			if !strings.EqualFold(rec.Protocol, protocol) {
				continue
			}
		}
		out = append(out, rec)
		if limit > 0 {
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

// Count returns the total number of offending frames recorded since process
// start. Safe for concurrent use.
func Count() uint64 {
	return defaultRing.count.Load()
}

func (r *ring) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = [MaxRecords]PacketRecord{}
	r.head = 0
	r.length = 0
	r.count.Store(0)
}
