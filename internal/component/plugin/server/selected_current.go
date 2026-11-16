// Design: docs/architecture/api/process-protocol.md -- exact selecting connection.
// Related: ipc/current_selection.go -- independent wire and typed callback.
package server

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// CurrentSelection queries only the authenticated owner's current connection.
// Configured-name discovery MUST NOT authorize a replacement process. The
// incarnation and connection are checked before and after the callback, whose
// reply MUST echo the exact requested destination, session and native keys.
func (s *Server) CurrentSelection(ctx context.Context, owner plugin.SelectedOwner, input rpc.CurrentSelectionInput) (*rpc.CurrentSelectionOutput, error) {
	connection, err := s.selectedConnection(owner)
	if err != nil {
		return nil, err
	}
	output, err := connection.SendCurrentSelection(ctx, input)
	if err != nil {
		return nil, err
	}
	current, err := s.selectedConnection(owner)
	if err != nil {
		return nil, err
	}
	if current != connection {
		return nil, errors.New("current-selection: selecting connection changed; reconcile from the current process incarnation")
	}
	if err := output.Validate(input); err != nil {
		return nil, err
	}
	return output, nil
}

func (s *Server) selectedConnection(owner plugin.SelectedOwner) (*ipc.PluginConn, error) {
	if err := owner.Current(); err != nil {
		return nil, err
	}
	name, named := owner.Sender().Process()
	if !named {
		return nil, errors.New("current-selection: caller is not a selecting process; use the authenticated selected operation")
	}
	manager := s.procManager.Load()
	if manager == nil {
		return nil, errors.New("current-selection: process manager is unavailable; restart ze before reconciling selected output")
	}
	proc := manager.GetProcess(name)
	if proc == nil {
		return nil, errors.New("current-selection: selecting process is absent; reconcile from the current process incarnation")
	}
	if proc.Done() != owner.Done() {
		return nil, errors.New("current-selection: selecting process incarnation changed; reconcile from the current process incarnation")
	}
	if proc.EngineDone() != owner.Ended() {
		return nil, errors.New("current-selection: selecting engine incarnation changed; reconcile from the current process incarnation")
	}
	connection := proc.Conn()
	if connection == nil {
		return nil, errors.New("current-selection: selecting connection has ended; reconcile from the current process incarnation")
	}
	if err := owner.Current(); err != nil {
		return nil, err
	}
	return connection, nil
}
