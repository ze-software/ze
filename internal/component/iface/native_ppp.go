// Design: docs/features/interfaces.md -- original PPP file guarded native writers
// Related: docs/architecture/appliance/kernel-profiles.md -- PPP original-resource ABI

package iface

import (
	"context"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// ErrNativePPPPending reports an original native write whose outcome is not
// established. The caller MUST retain this writer, its exact original unit and
// its original backend; it MUST use Settle before another mutation or Close.
var ErrNativePPPPending = errors.New("original PPP native write has an unresolved outcome")

// NativePPPUnitOwner borrows the same retained original creator file. The
// actual CreatedUnit adapter supplies it; labels or another attachment cannot.
// WithDescriptor MUST refuse nil, typed nil and closed owners. Its operation
// MUST NOT close, duplicate, retain or publish the descriptor. The provider
// invokes only the short ioctl or one synchronous send under this borrow;
// it MUST release the borrow before waiting for a native acknowledgment.
// Descriptor zero is valid. Safe for concurrent use and original owner closure.
type NativePPPUnitOwner interface {
	WithDescriptor(func(int) error) error
}

// NativePPPBackend is an optional capability of one retained backend instance.
// BindNativePPPUnit MUST run through the caller's existing joined original
// namespace helper; it captures a dedicated socket in that namespace and a
// positive creator-only ioctl51 receipt for the exact captured index and unit.
// A returned partial writer MUST remain owned until Close succeeds. A failed
// bind permits no mutation, including on a returned partial writer.
type NativePPPBackend interface {
	BindNativePPPUnit(NativePPPUnitOwner, int, int32) (NativePPPWriter, error)
}

// NativePPPWriter borrows an actual original creator file and its original
// backend socket; it never owns or closes that unit file. Its zero/nil value is
// unavailable. All methods are safe for concurrent use. Requests are bounded;
// ACK waiting does not hold the unit descriptor borrow. The caller MUST retain
// this writer through uncertain send or ACK outcomes. Settle MUST only receive
// the original transaction, never resubmit it. A settled negative ACK remains
// an error and preserves the native errno. A NativePPPWriteRejected marker
// confirms only a terminal matched NACK, never no effects, rollback, safe retry
// or absence. The caller MUST retain its domain effect/cleanup receipts after
// transport settlement when the operation can have partially changed state.
// Close MUST join active I/O and
// refuse unresolved outcomes; a caller MUST retry Close only after settlement.
type NativePPPWriter interface {
	SetMTU(context.Context, int) error
	SetAdminUp(context.Context) error
	SetAdminDown(context.Context) error
	SetAlias(context.Context, string) error
	AddAddressP2P(context.Context, netip.Prefix, netip.Prefix) error
	RemoveAddressP2P(context.Context, netip.Prefix, netip.Prefix) error
	AddRoute(context.Context, netip.Prefix, netip.Addr, int, rtproto.Proto) error
	RemoveRoute(context.Context, netip.Prefix, netip.Addr, int, rtproto.Proto) error
	Settle(context.Context) error
	Close() error
}
