// Design: docs/research/l2tpv2-ze-integration.md -- kernel PPP backend ownership
// Related: subsystem.go -- acquisition precedes sessions; close follows readers

package l2tp

import (
	"errors"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/opererr"
)

// prepareKernelBackend MUST run under the subsystem lifecycle lock before
// publishing the owner to any reactor. Stop and unwindLocked MUST close it only
// after their sessions, event drains and stats poller have stopped.
func (s *Subsystem) prepareKernelBackend() error {
	if s.kernelBackend != nil {
		return nil
	}
	backend, err := s.openKernelBackend(iface.DefaultBackendName())
	if err != nil {
		return opererr.Wrap(err, "L2TP cannot open its kernel interface backend",
			"Linux PPP interfaces require the registered OS backend even when VPP handles other traffic.", "ze doctor")
	}
	if backend == nil {
		return opererr.Wrap(errors.New("the interface factory returned no owner"),
			"L2TP cannot own its kernel interfaces", "No PPP session can safely configure its interface.", "ze doctor")
	}
	s.kernelBackend = backend
	return nil
}

// closeKernelBackend MUST run after every consumer stops. It relinquishes the
// single owned lease once; repeated Stop never closes another backend.
func (s *Subsystem) closeKernelBackend() error {
	if s.kernelBackend == nil {
		return nil
	}
	owner := s.kernelBackend
	s.kernelBackend = nil
	return owner.Close()
}
