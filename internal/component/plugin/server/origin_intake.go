// Design: docs/architecture/api/process-protocol.md -- optional deciding callback.
// Related: selected_current.go -- exact process connection validation.
package server

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// OriginIntakeOwner captures a declared optional callback from the configured
// instance. Call only outside reactor, peer and socket locks. False with nil
// error means one fully declared, live legacy process explicitly lacks intake.
// Unknown, pre-Stage1, absent or stopped owners never authorize direct fallback.
func (s *Server) OriginIntakeOwner(name string) (plugin.SelectedOwner, bool, error) {
	manager := s.procManager.Load()
	if manager == nil {
		return plugin.SelectedOwner{}, false, originOwnerUnavailable(name, "has no live process manager")
	}
	proc := manager.GetProcess(name)
	if proc == nil {
		return plugin.SelectedOwner{}, false, originOwnerUnavailable(name, "is absent or restarting")
	}
	registration := proc.Registration()
	if registration == nil {
		return plugin.SelectedOwner{}, false, originOwnerUnavailable(name, "has not published its registration")
	}
	if !registration.Done {
		return plugin.SelectedOwner{}, false, originOwnerUnavailable(name, "has not completed Stage 1 registration")
	}
	if proc.Stage() != plugin.StageRunning {
		return plugin.SelectedOwner{}, false, originOwnerUnavailable(name, "has not finished startup")
	}
	owner, err := proc.SelectedOwner()
	if err != nil {
		return plugin.SelectedOwner{}, registration.OriginIntake, err
	}
	if _, err := s.selectedConnection(owner); err != nil {
		return plugin.SelectedOwner{}, registration.OriginIntake, err
	}
	// Only an actually declared, live legacy owner may omit this capability.
	return owner, registration.OriginIntake, nil
}

func originOwnerUnavailable(name, condition string) error {
	return opererr.New("BGP route origination is waiting for deciding process "+name,
		"The configured owner "+condition+"; direct output is refused so its candidate state cannot be bypassed",
		"ze show doctor")
}

// OriginIntake checks the exact manager incarnation on both sides of a joined
// callback. A delayed acknowledgment cannot migrate to a same-name replacement.
func (s *Server) OriginIntake(ctx context.Context, owner plugin.SelectedOwner, input rpc.OriginIntakeInput) (*rpc.UpdateRouteOutput, error) {
	connection, err := s.selectedConnection(owner)
	if err != nil {
		return nil, err
	}
	if !owner.HasOriginIntake() {
		return nil, errors.New("deciding process did not register origin-intake")
	}
	output, callErr := connection.SendOriginIntake(ctx, input)
	current, err := s.selectedConnection(owner)
	if err != nil {
		return nil, errors.Join(callErr, err)
	}
	if current != connection {
		return nil, errors.New("origin-intake deciding connection changed")
	}
	if callErr == nil || output != nil {
		if err := rpc.ValidateOriginIntakeOutput(input, output); err != nil {
			return nil, err
		}
	}
	return output, callErr
}
