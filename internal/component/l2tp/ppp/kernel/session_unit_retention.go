// Design: docs/research/l2tpv2-ze-integration.md -- original unit retention join
// Related: session_release.go -- the admitted policy seal releases this duplicate

package kernel

import (
	"errors"
	"os"
	"sync"
)

// nativePPPUnitHold is the one duplicate created by BindNativeReleaseSeal from
// the SAME original unit file. It is not reconstructible from a descriptor or
// snapshot. Closing it is attempted exactly once, with its outcome retained.
// outcome and the parent retirement are protected by unit.mu; once owns close.
// Zero and nil are invalid. Only retainNativeUnit creates a production hold.
type nativePPPUnitHold struct {
	unit       *CreatedUnit
	descriptor int
	once       sync.Once
	closeErr   error
	outcome    *nativePPPUnitCloseResult
}

func (unit *CreatedUnit) retainNativeUnit(channelFD int, duplicate func(int) (int, error), closeFD func(int) error) (hold *nativePPPUnitHold, err error) {
	if duplicate == nil {
		return nil, errors.New("ppp: original unit has no duplicate operation")
	}
	if closeFD == nil {
		return nil, errors.New("ppp: retained original unit has no close operation")
	}
	err = unit.withOriginalUnitDescriptor(func(original int) (err error) {
		unit.mu.Lock()
		if prior := unit.retirement.retained; prior != nil {
			if prior.outcome == nil {
				unit.mu.Unlock()
				return errors.New("ppp: original unit already has a retained duplicate")
			}
			if prior.outcome.cause != nil {
				unit.mu.Unlock()
				return prior.outcome.cause
			}
		}
		hold = &nativePPPUnitHold{unit: unit, descriptor: -1}
		unit.retirement.retained = hold
		unit.mu.Unlock()
		defer func() {
			if failure := recover(); failure != nil {
				hold.finish(sessionPanicFailure(failure))
				panic(failure)
			}
		}()
		fd, duplicateErr := duplicate(original)
		if fd < 0 {
			hold.finish(nil) // No new descriptor exists on the native fcntl failure path.
			if duplicateErr != nil {
				return duplicateErr
			}
			return errors.New("ppp: native unit retention returned an invalid duplicate")
		}
		if fd == original {
			hold.finish(nil)
			return errors.Join(duplicateErr, errors.New("ppp: native unit retention returned the original descriptor"))
		}
		if fd == channelFD {
			hold.finish(nil)
			return errors.Join(duplicateErr, errors.New("ppp: native unit retention returned the original channel"))
		}
		hold.descriptor = fd
		if duplicateErr != nil {
			return errors.Join(duplicateErr, hold.close(closeFD))
		}
		unit.mu.Lock()
		closed := unit.closed
		unit.mu.Unlock()
		if closed {
			return errors.Join(os.ErrClosed, hold.close(closeFD))
		}
		return nil
	})
	return hold, err
}

func (hold *nativePPPUnitHold) close(closeFD func(int) error) error {
	hold.once.Do(func() {
		hold.closeErr = closeSessionStream(nil, hold.descriptor, closeFD)
		hold.finish(hold.closeErr)
	})
	return hold.closeErr
}

func (hold *nativePPPUnitHold) finish(cause error) {
	hold.unit.mu.Lock()
	defer hold.unit.mu.Unlock()
	if hold.outcome != nil {
		return
	}
	hold.outcome = &nativePPPUnitCloseResult{cause: cause}
	hold.unit.publishNativeUnitRetirementLocked()
}
