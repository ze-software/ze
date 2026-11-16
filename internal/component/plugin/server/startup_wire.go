// Design: docs/architecture/api/process-protocol.md -- bounded startup stages
// Related: startup_driver.go -- each wire wait has one lifecycle owner
// Related: startup_budget.go -- accepted initialization and apply allowance

package server

import (
	"context"
	"errors"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A process sink supplies its existing stage policy. Minimal, unconfigured
// sinks use the normal stage allowance and explicitly refuse nested engine RPCs.
type startupWireOwner interface {
	runStartupWire(context.Context, plugin.PluginStage, int, func(context.Context) error) error
	serveInitializingRPC(context.Context, *pluginipc.PluginConn, *rpc.Request) error
}

func readStartupStage(ctx context.Context, sink startupSink, connection *pluginipc.PluginConn, stage plugin.PluginStage) (*rpc.Request, error) {
	var request *rpc.Request
	err := runStartupWire(ctx, sink, stage, 0, func(startup context.Context) error {
		var err error
		request, err = connection.ReadRequest(startup)
		return err
	})
	return request, err
}

func runStartupWire(ctx context.Context, sink startupSink, stage plugin.PluginStage, allowance int, operation func(context.Context) error) error {
	if owner, ok := sink.(startupWireOwner); ok {
		return owner.runStartupWire(ctx, stage, allowance, operation)
	}
	stageCtx, cancel := startupWireContext(ctx, stageTimeoutFromEnv(), false, stage, allowance)
	defer cancel()
	return operation(stageCtx)
}

func startupWireContext(parent context.Context, timeout time.Duration, explicit bool, stage plugin.PluginStage, allowance int) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		// A nonpositive configured stall deliberately disables the local bound;
		// the owner still retains cancellation of the entire startup lifetime.
		return context.WithCancel(parent)
	}
	if !explicit {
		if startupStageHasAllowance(stage) {
			timeout = max(timeout, declaredConfigureBudget(&plugin.PluginRegistration{ApplyBudget: allowance}))
		}
	}
	return context.WithTimeout(parent, timeout)
}

func startupStageHasAllowance(stage plugin.PluginStage) bool {
	if stage == plugin.StageConfig {
		return true
	}
	return stage == plugin.StageRunning
}

func (e *engineStartupSink) runStartupWire(ctx context.Context, stage plugin.PluginStage, allowance int, operation func(context.Context) error) error {
	e.s.coordinatorMu.Lock()
	coordinator := e.s.coordinator
	e.s.coordinatorMu.Unlock()
	if coordinator == nil {
		// Ad-hoc and replacement startup retain the caller's lifetime or bounded
		// restart scope as their sole authority, including declared initialization.
		return operation(ctx)
	}
	timeout := e.proc.Config().StageTimeout
	explicit := timeout != 0
	if !explicit {
		timeout = stageTimeoutFromEnv()
	}
	// Initialization is this plugin's declared work, after the last tier
	// barrier. It takes its accepted allowance, not a new tier-stage number.
	if stage == plugin.StageRunning {
		stageCtx, cancel := startupWireContext(ctx, timeout, explicit, stage, allowance)
		defer cancel()
		return operation(stageCtx)
	}
	if !explicit {
		if stage == plugin.StageConfig {
			timeout = coordinator.ConfigureStall(timeout)
		}
	}
	stageCtx, cancel := context.WithCancelCause(ctx)
	done := make(chan struct{})
	// One watcher owns this finite stage wait. Existing coordinator progress
	// renews the inactivity window, so a busy tier never gets a flat deadline.
	go func() {
		defer close(done)
		if err := coordinator.WaitForStageInactivity(stageCtx, stage+1, timeout); err != nil {
			cancel(err)
		}
	}()
	err := operation(stageCtx)
	// Capture the watchdog's cause before canceling our completed operation.
	cause := context.Cause(stageCtx)
	cancel(nil)
	<-done
	if err != nil {
		return errors.Join(err, cause)
	}
	return cause
}

func (hs *hubStartupSink) runStartupWire(ctx context.Context, stage plugin.PluginStage, allowance int, operation func(context.Context) error) error {
	timeout := hs.h.proc.Config().StageTimeout
	explicit := timeout != 0
	if !explicit {
		timeout = stageTimeoutFromEnv()
	}
	stageCtx, cancel := startupWireContext(ctx, timeout, explicit, stage, allowance)
	defer cancel()
	return operation(stageCtx)
}

func (e *engineStartupSink) serveInitializingRPC(ctx context.Context, connection *pluginipc.PluginConn, request *rpc.Request) (err error) {
	defer engineOpPanics().request.Contain(
		func() taskguard.Context {
			return taskguard.Context{Unit: "initialization request from plugin " + e.proc.Name(), MessageType: request.Method}
		},
		func(failure *taskguard.Panic) {
			err = connection.SendError(ctx, request.ID, engineOpPanicMessage(request.Method, failure))
		},
	)
	if operation := lookupEngineOp(request.Method); operation != nil {
		return e.s.serveEngineOpJSONContext(ctx, ctx, e.proc, connection, request, operation)
	}
	if codec, ok := e.s.getRPCHandlers()[request.Method]; ok {
		result, failure := codec(request.Params)
		return answerEngineOpJSON(ctx, e.proc, connection, request, result, failure)
	}
	return connection.SendError(ctx, request.ID, "unknown method: "+request.Method)
}

func (hs *hubStartupSink) serveInitializingRPC(ctx context.Context, connection *pluginipc.PluginConn, request *rpc.Request) error {
	return connection.SendError(ctx, request.ID, "method "+request.Method+" is unavailable from this subsystem host")
}

func joinStartupInitialization(ctx context.Context, sink startupSink, connection *pluginipc.PluginConn, allowance int) error {
	return runStartupWire(ctx, sink, plugin.StageRunning, allowance, func(startup context.Context) error {
		return connection.SendRuntimeReadyServing(startup, func(request *rpc.Request) error {
			if owner, ok := sink.(startupWireOwner); ok {
				return owner.serveInitializingRPC(startup, connection, request)
			}
			return connection.SendError(startup, request.ID, "method "+request.Method+" is unavailable from this startup host")
		})
	})
}
