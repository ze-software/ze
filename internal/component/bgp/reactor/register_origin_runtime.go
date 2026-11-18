// Design: docs/architecture/api/process-protocol.md -- original replay after process startup.
// Related: register_source_lease.go -- joined teardown before replacement.
package reactor

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func init() {
	if err := pluginserver.RegisterProcessGenerationReady(originGenerationReady); err != nil {
		panic("BUG: origin process-ready registration failed: " + err.Error())
	}
}

func originGenerationReady(parent context.Context, server *pluginserver.Server, proc *process.Process) error {
	registration := proc.Registration()
	if registration == nil {
		return nil
	}
	if !registration.OriginIntake {
		return nil
	}
	manager := server.ProcessManager()
	if manager == nil {
		return errors.New("origin provider readiness has no process manager")
	}
	if manager.GetProcess(proc.Name()) != proc {
		return errors.New("origin provider readiness belongs to a superseded process")
	}
	owner, err := proc.SelectedOwner()
	if err != nil {
		return err
	}
	replay, supported := server.Reactor().(interface {
		ReplayOriginOwner(context.Context, plugin.SelectedOwner) error
	})
	if !supported {
		return nil
	}
	ctx, cancel := context.WithTimeout(parent, rpc.CancellableRequestTimeout+rpc.RequestCancelTimeout)
	defer cancel()
	return replay.ReplayOriginOwner(ctx, owner)
}
