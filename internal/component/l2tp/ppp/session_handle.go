// Design: docs/research/l2tpv2-ze-integration.md -- per-session data-plane ownership

package ppp

import (
	"context"
	"net/netip"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// SessionIdentity describes a created interface. NativeIndex belongs to its
// backend's index space; PPP treats it as opaque and never invents a unit number.
type SessionIdentity struct {
	InterfaceName string
	NativeIndex   uint32
}

// SessionCounters holds raw cumulative traffic totals and individually available
// diagnostics. An unavailable traffic reading MUST return an error. Diagnostic
// consumers MUST call CounterReading.Value and preserve Unknown, never guess zero.
type SessionCounters struct {
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
	RxErrors  CounterReading
	TxErrors  CounterReading
	RxDropped CounterReading
	TxDropped CounterReading
}

// SessionHandle owns the resources lent by FrameChannel. A successful factory
// MUST return both for the same session. The driver MUST stop IPv6 services,
// close this handle, join every frame reader, and return queued buffers before
// forgetting the session. Close is idempotent, wakes readers and serializes
// with all data-plane operations; operations after Close return an error.
// A nil handle is never a valid session. Interface values MUST NOT contain a
// typed nil. Its concrete owner MUST be comparable, so admission can recognize
// an accidental requeue of a live lease without closing the original owner.
// Production factories return pointer owners. A backend may retain its
// separate transport owner for relay use.
type SessionHandle interface {
	Identity() SessionIdentity
	SetMTU(int) error
	// Connect MUST reconcile all acknowledged address families idempotently,
	// without repeating physical attachment. The caller MUST wait for each
	// newly staged family's assignment acknowledgment before calling it.
	Connect() error
	Disconnect() error
	SetReceiveMRU(uint16) error
	SetAdminUp() error
	AddAddressP2P(local, peer netip.Prefix) error
	// RemoveAddress MUST withdraw only this owner's receipt. An absent own
	// receipt is already complete and MUST perform no native deletion. An
	// uncertain add or failed removal MUST retain its receipt for retry; the
	// caller MUST retain its cleanup owner while removal returns an error.
	RemoveAddress(local netip.Prefix) error
	AddRoute(destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) error
	RemoveRoute(destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) error
	Counters() (SessionCounters, error)
	OpenIPv6Carrier(context.Context) (IPv6Carrier, error)
	Close() error
}
