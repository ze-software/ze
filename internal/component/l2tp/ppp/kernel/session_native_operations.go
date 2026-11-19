// Design: docs/research/l2tpv2-ze-integration.md -- exact original native mutations

package kernel

import (
	"context"
	"errors"
	"io"
	"time"
)

// A short admission token serializes domain intent without keeping a resource
// lock over native ACK. Close never waits for this token and may unregister the
// original unit while a request is unresolved. The writer alone settles it.
// A successful beginNativeMutation MUST be paired with endNativeMutation.
func (s *session) beginNativeMutation() (NativeUnitOperations, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, io.ErrClosedPipe
	}
	if s.nativeMutation {
		return nil, errors.New("ppp: an original native mutation is in progress")
	}
	if nilSessionDependency(s.mutations) {
		return nil, errors.New("ppp: original native mutation authority is unavailable")
	}
	s.nativeMutation = true
	return s.mutations, nil
}

// endNativeMutation MUST follow a successful beginNativeMutation, including
// when validation, submission or acknowledgement fails.
func (s *session) endNativeMutation() {
	s.mu.Lock()
	s.nativeMutation = false
	s.mu.Unlock()
}

// Native mutations and cleanup use the same bounded lifetime as the provider.
// No receipt is deleted merely because this context expires.
const sessionNativeOperationTimeout = 2 * time.Second

func sessionNativeContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), sessionNativeOperationTimeout)
}

func (s *session) originalNativeUnit() *CreatedUnit {
	lease, genuine := s.unitLease.(*claimedCreatedUnit)
	if !genuine {
		return nil
	} // Private logical fixture; never public admission.
	return lease.unit
}

// closeNativeOperations follows physical original-file/real-seal-duplicate
// cleanup, never the reverse. Genuine retirement clears only unit-scoped local
// intents; it does not invent a remote seal grant or resend an unknown request.
func (s *session) closeNativeOperations() error {
	s.mu.Lock()
	retained := s.unitRetained
	s.mu.Unlock()
	if retained {
		// The independent remote seal still owns the actual duplicate. Local
		// settlement MUST wait until that seal releases it; waiting here would
		// manufacture a local timeout from the caller's deliberate retention.
		return nil
	}
	unit := s.originalNativeUnit()
	if unit == nil {
		return nil
	}
	ctx, cancel := sessionNativeContext()
	defer cancel()
	retired := unit.WaitNativePPPUnitRetired(ctx)
	result := unit.CloseNativeOperations(ctx)
	if retired == nil {
		s.mu.Lock()
		s.addresses = nil
		s.routes = nil
		s.mu.Unlock()
	}
	return errors.Join(retired, result)
}

// The short NCP ioctl lease ends before the native writer waits for ACK.
// A saved descriptor label never authorizes a late family withdrawal.
func (s *session) dropOriginalNativeFamily(ipv4 bool) error {
	setMode := s.ops.setIPv6Mode
	if ipv4 {
		setMode = s.ops.setIPv4Mode
	}
	if unit := s.originalNativeUnit(); unit != nil {
		return unit.WithDescriptor(func(fd int) error { return setMode(fd, NPModeDrop) })
	}
	// This lease variant exists only in explicit logical session fixtures.
	return setMode(s.unitFD, NPModeDrop)
}

// Called only after failed construction's complete physical pair cleanup has
// released pair.mu. Both server and client failures retain partial bind causes.
func closeClaimedPairNativeOperations(lease sessionUnitLease) error {
	original, genuine := lease.(*claimedCreatedUnit)
	if !genuine {
		return nil
	}
	ctx, cancel := sessionNativeContext()
	defer cancel()
	return original.unit.CloseNativeOperations(ctx)
}
