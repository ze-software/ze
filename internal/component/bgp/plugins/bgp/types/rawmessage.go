// Design: docs/architecture/core-design.md — shared BGP types

package types

import (
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/wireu"
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
	Direction  string                    // "sent" or "received"
	ParseError error                     // Non-nil if lazy parsing failed
}

// IsAsyncSafe reports whether this message's RawBytes can be safely used after
// the callback returns. Returns false for zero-copy received UPDATEs where
// RawBytes points to a buffer that may be reused.
func (m *RawMessage) IsAsyncSafe() bool {
	return m.WireUpdate == nil
}
