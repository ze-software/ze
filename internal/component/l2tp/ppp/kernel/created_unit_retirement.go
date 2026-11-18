// Design: docs/research/l2tpv2-ze-integration.md -- exact original unit retirement
// Related: session_unit_retention.go -- the library's one retained duplicate

package kernel

import (
	"context"
	"errors"
	"os"
)

// nativePPPUnitRetirement is embedded in its SAME CreatedUnit and protected by
// unit.mu. It has at most one library-managed retained duplicate. The two result
// pointers distinguish pending Close from completed Close and terminal custody.
// There is no public receipt constructor and no name/index observation here.
type nativePPPUnitRetirement struct {
	done      chan struct{}
	borrows   int
	retained  *nativePPPUnitHold
	fileClose *nativePPPUnitCloseResult
	retired   *nativePPPUnitCloseResult
}

type nativePPPUnitCloseResult struct{ cause error }

// WaitNativePPPUnitRetired joins the SAME original file's completed Close,
// every short public borrow and the library's retained original duplicate.
// Nil confirms successful completion of all of those native closes. A failed
// close remains an error and supplies no retirement proof. A namespace-file
// cleanup error is reported by Close separately; it cannot resurrect the unit.
//
// This method NEVER initiates Close. The caller MUST first arrange final
// accounting, PPP disconnect, reader interruption and final exact-owner Close.
// Close and the native-release seal MUST remain able to run while this method
// waits. No unit/pair mutex or native descriptor lease is held while waiting.
// Safe for concurrent calls; zero, nil and copied owners grant no authority.
func (unit *CreatedUnit) WaitNativePPPUnitRetired(ctx context.Context) error {
	if unit == nil {
		return errors.New("ppp: no original unit retirement owner")
	}
	if unit.self != unit {
		return errors.New("ppp: copied or unminted unit cannot confirm retirement")
	}
	if ctx == nil {
		return errors.New("ppp: original unit retirement requires a wait context")
	}
	unit.mu.Lock()
	if unit.file == nil {
		unit.mu.Unlock()
		return errors.New("ppp: original unit has no creation file")
	}
	if unit.retirement.retired != nil {
		cause := unit.retirement.retired.cause
		unit.mu.Unlock()
		return cause
	}
	if unit.retirement.done == nil {
		unit.retirement.done = make(chan struct{})
	}
	done := unit.retirement.done
	unit.mu.Unlock()
	select {
	case <-done:
		unit.mu.Lock()
		cause := unit.retirement.retired.cause
		unit.mu.Unlock()
		return cause
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Only short descriptor or zero-time liveness operations use this helper.
// The one internal retention operation may create its tracked duplicate here;
// public WithDescriptor callers MUST NOT duplicate or retain a descriptor.
// Native counter requests MUST run outside it, so Close can retire an original
// while a read-only ACK/reply is delayed. This also joins a RawConn operation on
// platforms/file modes where os.File.Close itself returns before final decref.
func (unit *CreatedUnit) withOriginalUnitFile(operation func(*os.File) error) error {
	unit.mu.Lock()
	if unit.closed {
		unit.mu.Unlock()
		return os.ErrClosed
	}
	if unit.file == nil {
		unit.mu.Unlock()
		return os.ErrClosed
	}
	file := unit.file
	unit.retirement.borrows++
	unit.mu.Unlock()
	defer unit.finishNativeUnitBorrow()
	return operation(file)
}

func (unit *CreatedUnit) finishNativeUnitBorrow() {
	unit.mu.Lock()
	defer unit.mu.Unlock()
	unit.retirement.borrows--
	unit.publishNativeUnitRetirementLocked()
}

func (unit *CreatedUnit) withOriginalUnitDescriptor(operation func(int) error) error {
	return unit.withOriginalUnitFile(func(file *os.File) error {
		connection, err := file.SyscallConn()
		if err != nil {
			return err
		}
		var operationErr error
		if err := connection.Control(func(fd uintptr) { operationErr = operation(int(fd)) }); err != nil {
			return err
		}
		return operationErr
	})
}

func (unit *CreatedUnit) pollNativeUnitFile(file *os.File, poll func(*os.File) error) error {
	return unit.withOriginalUnitFile(func(original *os.File) error {
		if original != file {
			return errors.New("ppp: original unit file changed before liveness check")
		}
		return poll(original)
	})
}

// MUST be called exactly after the actual original os.File.Close returns,
// before closing its separate namespace file. It never substitutes for Close.
func (unit *CreatedUnit) finishNativeUnitFileClose(cause error) {
	unit.mu.Lock()
	defer unit.mu.Unlock()
	unit.retirement.fileClose = &nativePPPUnitCloseResult{cause: cause}
	unit.publishNativeUnitRetirementLocked()
}

// unit.mu is held. No second native operation is initiated while publishing.
func (unit *CreatedUnit) publishNativeUnitRetirementLocked() {
	if unit.retirement.retired != nil {
		return
	}
	if unit.retirement.fileClose == nil {
		return
	}
	if unit.retirement.borrows != 0 {
		return
	}
	cause := unit.retirement.fileClose.cause
	if hold := unit.retirement.retained; hold != nil {
		if hold.outcome == nil {
			return
		}
		cause = errors.Join(cause, hold.outcome.cause)
	}
	unit.retirement.retired = &nativePPPUnitCloseResult{cause: cause}
	if unit.retirement.done == nil {
		unit.retirement.done = make(chan struct{})
	}
	close(unit.retirement.done)
}
