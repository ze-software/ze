// Design: docs/research/l2tpv2-ze-integration.md -- kernel PPP backend ownership
// Related: subsystem.go -- acquisition precedes sessions; close follows readers

package l2tp

import (
	"errors"
	"reflect"

	"github.com/ze-software/ze/internal/component/iface"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/opererr"
)

// prepareKernelBackend MUST run under the subsystem lifecycle lock before
// publishing the owner to any reactor. Stop and unwindLocked MUST close it only
// after their sessions, event drains and stats poller have stopped.
func (s *Subsystem) prepareKernelBackend() error {
	if !nilKernelBackend(s.kernelBackend) {
		return nil
	}
	backend, err := s.openKernelBackend(iface.DefaultBackendName())
	if err != nil {
		return opererr.Wrap(err, "L2TP cannot open its kernel interface backend",
			"Linux PPP interfaces require the registered OS backend even when VPP handles other traffic.", "ze doctor")
	}
	if nilKernelBackend(backend) {
		return opererr.Wrap(errors.New("the interface factory returned no owner"),
			"L2TP cannot own its kernel interfaces", "No PPP session can safely configure its interface.", "ze doctor")
	}
	s.kernelBackend = backend
	return nil
}

// closeKernelBackend MUST run after every consumer stops. It relinquishes the
// single owned lease once; repeated Stop never closes another backend.
func (s *Subsystem) closeKernelBackend() error {
	if nilKernelBackend(s.kernelBackend) {
		return nil
	}
	owner := s.kernelBackend
	s.kernelBackend = nil
	return owner.Close()
}

// kernelBackendPreparation supplies the independently owned OS backend only
// when the platform can construct a worker. A worker constructor MUST invoke
// it before publication; the subsystem MUST keep its owner through all joins.
type kernelBackendPreparation func() (pppkernel.InterfaceOperations, error)

type kernelBackendPreparationFailure struct{ cause error }

func (failure *kernelBackendPreparationFailure) Error() string { return failure.cause.Error() }
func (failure *kernelBackendPreparationFailure) Unwrap() error { return failure.cause }

func (s *Subsystem) prepareKernelWorkerBackend() (pppkernel.InterfaceOperations, error) {
	if err := s.prepareKernelBackend(); err != nil {
		return nil, err
	}
	return s.kernelBackend, nil
}

// nilKernelBackend rejects an absent owner before publication or unwind can
// call its methods. A nonnil interface containing a nil pointer is absent too.
func nilKernelBackend(backend pppkernel.InterfaceOperations) bool {
	if backend == nil {
		return true
	}
	value := reflect.ValueOf(backend)
	// Only nil-capable representations can conceal a missing owner.
	//exhaustive:ignore
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	}
	return false
}
