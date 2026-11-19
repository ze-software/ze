// Design: docs/research/l2tpv2-ze-integration.md -- original-unit mutation binding

package kernel

import (
	"context"
	"errors"
	"math"
	"net/netip"
	"os"
	"reflect"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// NativeUnitOperations is the per-original-unit mutation facet. The original
// backend remains the independent creation/counter owner. The caller MUST bind
// through BindNativeOperations and retain the returned partial facet on error.
// All methods are bounded by their context. Settle MUST receive only the exact
// original request; it MUST NOT resubmit an uncertain native mutation.
type NativeUnitOperations interface {
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
	ClosePending() bool
}

// NativeUnitBinder MUST call the captured backend's native binding producer
// using the supplied SAME owner and immutable creation labels. It runs only in
// that unit's joined original namespace. It MUST return any partial writer on
// error; it MUST NOT replace the creator/counter backend or reopen by name.
type NativeUnitBinder func(*CreatedUnit, int, int32) (NativeUnitOperations, error)

type nativeUnitOperationsOwnership interface{ nativeUnitOperationsOwnership() }
type nativeUnitOperationsOwned struct{ operations NativeUnitOperations }
type nativeUnitOperationsClosed struct{ cause error }

func (nativeUnitOperationsOwned) nativeUnitOperationsOwnership()  {}
func (nativeUnitOperationsClosed) nativeUnitOperationsOwnership() {}

// One completed bind owns at most one native socket. A pending completion is
// explicit; its nil ownership does not report absence while the producer runs.
type nativeUnitOperationsBinding struct {
	done      chan struct{}
	ownership nativeUnitOperationsOwnership
	cause     error
}

// BindNativeOperations captures the separate mutation capability once. It MUST
// precede SessionConfig transfer; CloseNativeOperations MUST follow actual unit
// closure and release of any same-unit native-release duplicate. No owner lock
// or descriptor borrow is held across the producer or its acknowledgement.
// Safe for concurrent unit closure; a late result remains owned but is refused.
func (unit *CreatedUnit) BindNativeOperations(ctx context.Context, backend InterfaceOperations, bind NativeUnitBinder) (NativeUnitOperations, error) {
	if ctx == nil {
		return nil, errors.New("ppp: native binding has no context")
	}
	if bind == nil {
		return nil, errors.New("ppp: native binding has no original producer")
	}
	if unit == nil {
		return nil, errors.New("ppp: native binding has no original unit")
	}
	if nilSessionDependency(backend) {
		return nil, errors.New("ppp: native binding has no original backend")
	}
	if !reflect.TypeOf(backend).Comparable() {
		return nil, errors.New("ppp: native binding backend has no stable identity")
	}
	unit.mu.Lock()
	if unit.self != unit {
		unit.mu.Unlock()
		return nil, errors.New("ppp: native binding refuses a copied unit")
	}
	if unit.closed {
		unit.mu.Unlock()
		return nil, os.ErrClosed
	}
	if !unit.ready {
		unit.mu.Unlock()
		return nil, errors.New("ppp: native binding requires confirmed creation")
	}
	if backend != unit.backend {
		unit.mu.Unlock()
		return nil, errors.New("ppp: native binding changed the original backend")
	}
	if unit.nativeOperations != nil {
		unit.mu.Unlock()
		return nil, errors.New("ppp: original unit native binding was already attempted")
	}
	if unit.sessionClaim != nil {
		unit.mu.Unlock()
		return nil, errors.New("ppp: native binding must precede session transfer")
	}
	index, number := unit.identity.NativeIndex, unit.number
	if index == 0 {
		unit.mu.Unlock()
		return nil, errors.New("ppp: native binding has no captured index")
	}
	if index > math.MaxInt32 {
		unit.mu.Unlock()
		return nil, errors.New("ppp: captured index exceeds native width")
	}
	if number < 0 {
		unit.mu.Unlock()
		return nil, errors.New("ppp: native binding has no captured unit number")
	}
	if int64(number) > math.MaxInt32 {
		unit.mu.Unlock()
		return nil, errors.New("ppp: captured unit exceeds native width")
	}
	binding := &nativeUnitOperationsBinding{done: make(chan struct{})}
	unit.nativeOperations = binding
	unit.mu.Unlock()
	var operations NativeUnitOperations
	err := unit.WithOriginalNamespace(ctx, func() error {
		var err error
		operations, err = bind(unit, int(index), int32(number))
		return err
	})
	if nilSessionDependency(operations) {
		operations = nil
		if err == nil {
			err = errors.New("ppp: native binding returned no owned writer")
		}
	}
	unit.mu.Lock()
	if unit.closed {
		err = errors.Join(err, os.ErrClosed)
	}
	if operations != nil {
		binding.ownership = nativeUnitOperationsOwned{operations}
	} else {
		binding.ownership = nativeUnitOperationsClosed{cause: err}
	}
	binding.cause = err
	close(binding.done)
	unit.mu.Unlock()
	return operations, err
}

func (unit *CreatedUnit) nativeOperationsLocked() (NativeUnitOperations, error) {
	binding := unit.nativeOperations
	if binding == nil {
		return nil, errors.New("ppp: original unit has no native mutation binding")
	}
	select {
	case <-binding.done:
	default:
		return nil, errors.New("ppp: original native mutation binding is pending")
	}
	if binding.cause != nil {
		return nil, binding.cause
	}
	switch ownership := binding.ownership.(type) {
	case nativeUnitOperationsOwned:
		return ownership.operations, nil
	case nativeUnitOperationsClosed:
		return nil, errors.Join(os.ErrClosed, ownership.cause)
	}
	return nil, errors.New("ppp: original native binding has no completed ownership")
}

// NativeOperationsPending reports exact retained local ownership, not native
// absence. A live bind or failed native socket close keeps this result true.
// The caller MUST retain the unit/backend and retry CloseNativeOperations.
func (unit *CreatedUnit) NativeOperationsPending() bool {
	if unit == nil {
		return false
	}
	unit.mu.Lock()
	defer unit.mu.Unlock()
	if unit.self != unit {
		return true
	}
	if unit.nativeOperations == nil {
		return false
	}
	select {
	case <-unit.nativeOperations.done:
	default:
		return true
	}
	_, owned := unit.nativeOperations.ownership.(nativeUnitOperationsOwned)
	return owned
}

// CloseNativeOperations MUST follow caller-ordered original-file closure and
// the same native-release duplicate close. It never initiates either one. A
// bounded Settle failure cannot suppress Close of a terminal original socket.
// Safe for repeated/concurrent use; successful Close consumes only this facet.
func (unit *CreatedUnit) CloseNativeOperations(ctx context.Context) error {
	if unit == nil {
		return errors.New("ppp: native cleanup has no original unit")
	}
	if ctx == nil {
		return errors.New("ppp: native cleanup has no context")
	}
	unit.mu.Lock()
	if unit.self != unit {
		unit.mu.Unlock()
		return errors.New("ppp: native cleanup refuses a copied unit")
	}
	binding := unit.nativeOperations
	unit.mu.Unlock()
	if binding == nil {
		return nil
	} // No binding producer ever ran for this owner.
	select {
	case <-binding.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	unit.mu.Lock()
	ownership := binding.ownership
	unit.mu.Unlock()
	switch owned := ownership.(type) {
	case nativeUnitOperationsClosed:
		return owned.cause
	case nativeUnitOperationsOwned:
		settleErr := owned.operations.Settle(ctx)
		closeErr := owned.operations.Close()
		pending := owned.operations.ClosePending()
		result := errors.Join(binding.cause, closeErr)
		// Pending is current custody, not permanent history. Once Close reports
		// terminal socket retirement, a transient Settle marker is superseded by
		// the producer's final Close result, which retains genuine prior causes.
		if pending {
			result = errors.Join(result, settleErr)
		}
		if !pending {
			unit.mu.Lock()
			if previous, complete := binding.ownership.(nativeUnitOperationsClosed); complete {
				result = errors.Join(previous.cause, result)
			}
			binding.ownership = nativeUnitOperationsClosed{cause: result}
			unit.mu.Unlock()
		}
		return result
	}
	return errors.New("ppp: native cleanup has no completed ownership")
}
