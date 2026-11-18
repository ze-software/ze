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
// established and whose transport custody remains pending. The caller MUST
// retain this writer, its exact original unit and backend. Settle receives only
// the original ACK; after genuine same-unit retirement it may instead end that
// transport custody with a retained unknown-outcome error. This never confirms
// effects, rollback or retry authority. The caller MUST independently finish
// accounting/disconnect and arrange original/seal closes before final Settle.
var ErrNativePPPPending = errors.New("original PPP native write has an unresolved outcome")

// ErrNativePPPUnavailable marks a producer-known missing/closed exact original
// authority or refusal of a required native capability. It does not classify
// ordinary validation, arbitrary errors or settled route NACKs. Consumers use
// errors.Is on this facet, never infer it from an errno or message. A missing
// bound writer is reported with this facet by the adapter that owns the bind.
var ErrNativePPPUnavailable = errors.New("original PPP native authority or required capability is unavailable")

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

// NativePPPUnitRetirementOwner is an optional capability of that SAME owner.
// Only the genuine CreatedUnit implements production authority: nil means its
// actual original file Close, every short borrow and every library-retained
// duplicate Close have completed successfully. Failure grants no retirement.
// Wait is passive and MUST NOT initiate Close. It checks completed retirement
// BEFORE selecting ctx.Done, including when ctx is already cancelled. Otherwise
// it waits only within ctx. Copied, unminted, nil and zero owners refuse proof.
// Names, indices, raw fds, absence observations and caller booleans never grant
// this capability. Primary MUST finish accounting/disconnect and arrange the
// original and native-release-seal closes independently before final Settle.
type NativePPPUnitRetirementOwner interface {
	NativePPPUnitOwner
	WaitNativePPPUnitRetired(context.Context) error
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
// this writer through uncertain send or ACK outcomes. Settle MUST never resubmit
// the original transaction. A settled negative ACK remains
// an error and preserves the native errno. A NativePPPWriteRejected marker
// confirms only a terminal matched NACK, never no effects, rollback, safe retry
// or absence. The caller MUST retain its domain effect/cleanup receipts after
// transport settlement when the operation can have partially changed state.
// Settle and Close observe the SAME owner's optional passive retirement without
// waiting for a live unit or initiating its Close. Successful original-unit
// retirement can end a poisoned or exhausted transport while preserving every
// unknown-outcome cause and closing the native socket. Close otherwise joins
// active I/O and refuses unresolved custody. Terminal error history persists on
// repeated Settle/Close. Historical unknown-outcome errors after successful
// physical socket Close MUST NOT prevent unrelated final backend cleanup.
// A failed physical socket Close retains custody and its cause. Close MUST NOT
// retry through the retired os.File or its numeric descriptor; a later
// os.ErrClosed does not prove physical release.
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
	// ClosePending reports actual retained native custody after Close. A live
	// socket needs Close even before a write. A failed physical socket Close
	// keeps this true; historical errors after successful socket Close do not.
	// Nil/zero unavailable writers retain no custody.
	ClosePending() bool
}
