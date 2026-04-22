// Design: docs/architecture/core-design.md — shared BGP types

package types

import (
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// RawMessage represents a BGP message sent or received.
// Contains raw wire bytes for on-demand parsing based on format config.
type RawMessage struct {
	Type       message.MessageType // UPDATE, OPEN, NOTIFICATION, etc.
	RawBytes   []byte              // Original wire bytes (without marker/header)
	Timestamp  time.Time
	MessageID  uint64                    // Unique ID for all message types
	AttrsWire  *attribute.AttributesWire // Lazy attribute parsing (nil if not UPDATE or parse failed)
	WireUpdate *wireu.WireUpdate         // UPDATE wire wrapper (nil if not UPDATE)
	Direction  rpc.MessageDirection      // DirectionSent / DirectionReceived
	ParseError error                     // Non-nil if lazy parsing failed
	Meta       map[string]any            // Route metadata from ReceivedUpdate (sent events only)

	// ReactorForwarded is true when reactorForwardRS already forwarded this
	// UPDATE to eligible RS peers. bgp-rs checks this to skip ForwardCached.
	ReactorForwarded bool

	// FastPathSkipped lists destination peers that the reactor fast path
	// skipped (e.g. because they have ExportFilters). bgp-rs forwards to
	// only these peers via ForwardCached when ReactorForwarded is true.
	FastPathSkipped []netip.AddrPort
}

// IsAsyncSafe reports whether this message's RawBytes can be safely used after
// the callback returns. Returns false for zero-copy received UPDATEs where
// RawBytes points to a buffer that may be reused.
func (m *RawMessage) IsAsyncSafe() bool {
	return m.WireUpdate == nil
}
