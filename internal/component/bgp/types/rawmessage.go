// Design: docs/architecture/core-design.md — shared BGP types

package types

import (
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// RawMessage represents a BGP message sent or received.
// Contains raw wire bytes for on-demand parsing based on format config.
type RawMessage struct {
	Type          msgtype.MessageType // UPDATE, OPEN, NOTIFICATION, etc.
	RawBytes      []byte              // Original wire bytes (without marker/header)
	Timestamp     time.Time
	MessageID     uint64                    // Unique ID for all message types
	AttrsWire     *attribute.AttributesWire // Lazy attribute parsing (nil if not UPDATE or parse failed)
	WireUpdate    *wireu.WireUpdate         // UPDATE wire wrapper (nil if not UPDATE)
	Direction     rpc.MessageDirection      // DirectionSent / DirectionReceived
	ParseError    error                     // Non-nil if lazy parsing failed
	Meta          map[string]any            // Route metadata from ReceivedUpdate (sent events only)
	SourcePeerStr string                    // Source peer address for ribOut stale-scoping (sent events only)

	// ReactorForwarded is true when reactorForwardRS already forwarded this
	// UPDATE to eligible RS peers. bgp-rs checks this to skip ForwardCached.
	ReactorForwarded bool

	// FastPathSkipped lists destination peers the reactor fast path did not
	// decide for: they carry ExportFilters it cannot apply, or an egress
	// filter panicked for them. bgp-rs forwards to only these peers via
	// ForwardCached when ReactorForwarded is true. A peer the fast path
	// SUPPRESSED by policy is not listed -- that decision is final.
	FastPathSkipped []netip.AddrPort
}

// IsAsyncSafe reports whether this message's RawBytes can be safely used after
// the callback returns. Returns false for zero-copy received UPDATEs where
// RawBytes points to a buffer that may be reused.
//
// The callback return is the lifetime boundary (contract A): a false result
// means RawBytes is a Borrow into a recyclable receive buffer that a consumer
// must Own via WireUpdate.Snapshot before the boundary. In debug builds the
// buffer is poisoned at recycle, so a borrow read after the boundary is caught.
// See docs/architecture/memory/lifetime-contracts.md.
func (m *RawMessage) IsAsyncSafe() bool {
	return m.WireUpdate == nil
}
