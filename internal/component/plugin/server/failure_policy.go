// Design: docs/architecture/api/process-protocol.md — the Stage-1 declaration a plugin makes about its own failure
// Overview: restart.go — restartPlugin, which this file calls to start a plugin again
// Related: ../process/manager.go — ProcessManager.Respawn, and the bounds a restart runs under
// Related: system.go — handleDaemonShutdown, the daemon-stop route a fatal plugin reuses

package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// errRespawnDisagreement marks a Stage-1 refusal the daemon must not run
// through. It is a sentinel because the refusal travels as an ordinary startup
// error: runStartupHandshake returns a sink error unwrapped and
// startupFailureError wraps it with %w, so runPluginPhase can still tell this
// failure from every other one.
var errRespawnDisagreement = errors.New("the configuration and the plugin disagree about respawn")

// superviseProcess serves one plugin generation's runtime RPCs and, when that
// generation ends, does what the plugin asked ze to do about its own failure.
// It is the whole life of one plugin process.
//
// One goroutine runs one generation. A restart starts the NEXT generation's
// goroutine and this one then returns, so a crash loop costs one goroutine at a
// time rather than a growing chain, and ProcessManager.Respawn bounds how many
// generations there can be.
func (s *Server) superviseProcess(proc *process.Process) {
	// The generation's drain caller lives exactly as long as the generation:
	// it is out of the caller set and has returned before the failure policy
	// can respawn the plugin, so a respawn never meets the old caller.
	stopDrainCaller := s.startDrainCaller(proc)
	// This generation reached running, so the plugin is no longer down: a
	// stop record from the generation before it (a failed handshake, the
	// respawn that started this one) is cleared with its bus warning.
	if pm := s.procManager.Load(); pm != nil {
		pm.ClearStopped(proc.Name())
	}
	s.handleSingleProcessCommandsRPC(proc)
	stopDrainCaller()
	s.applyFailurePolicy(proc)
}

// pluginFailurePolicy reports what ze does when this plugin fails. It maps an
// unspecified declaration to rpc.FailureIgnore; other typed declarations pass
// through, so callers must also handle unknown plugin policies.
//
// A plugin that declared nothing, and a plugin that failed before it could
// declare, are both read as rpc.FailureIgnore. That is what ze did before a
// plugin could declare anything, so no plugin's behavior changes without its
// author asking for it, and silence is never read as consent to a restart.
//
// The `respawn` leaf can only ask for LESS than the declaration permits. A block
// that declined the respawn of a plugin willing to be restarted leaves it
// stopped. A block that ASKED for one the declaration refuses never reaches
// here, because refuseRespawnDisagreement stopped the daemon at startup.
//
// A process carries a registration from the moment NewProcess builds it, and
// SetRegistration fills it at Stage 1. So a plugin that failed earlier than that
// reads here as an EMPTY registration, whose FailurePolicy is the unspecified
// sentinel, rather than as a missing one.
func pluginFailurePolicy(proc *process.Process) rpc.FailurePolicy {
	declared := proc.Registration().FailurePolicy

	if declared == rpc.FailureUnspecified {
		return rpc.FailureIgnore
	}
	if declared.AllowsRestart() {
		if proc.Config().Respawn == plugin.RespawnDeclined {
			return rpc.FailureIgnore
		}
	}
	return declared
}

// refuseRespawnDisagreement rejects a plugin whose declaration cannot meet the
// respawn its configuration block asked for. The owner settled what happens
// then: "if a plugin declare that it can not restart then we can not start"
// (2026-09-06).
//
// The two sides come from different places and only meet here, at Stage 1: the
// leaf is the operator's, read by ExtractPluginsFromTree, and the policy is the
// plugin's, declared in the message being handled. So this cannot be a config
// validator, and the message has to name both sides or an operator is left with
// a daemon that will not start and no way to tell which half to change.
func refuseRespawnDisagreement(name string, asked plugin.RespawnRequest, declared rpc.FailurePolicy) error {
	if asked != plugin.RespawnAsked {
		return nil
	}
	if declared.AllowsRestart() {
		return nil
	}
	return fmt.Errorf("%w: plugin %q declares failure-policy %q, so it must not be started again, "+
		"and its configuration block asks for respawn: remove the respawn leaf, "+
		"or run a plugin that declares %q",
		errRespawnDisagreement, name, declaredPolicyName(declared), rpc.FailureRestart)
}

// declaredPolicyName is how a declaration is named in a message to an operator.
// A plugin that declared nothing has no policy to quote, so the message says
// that rather than printing the sentinel's own spelling.
func declaredPolicyName(declared rpc.FailurePolicy) string {
	if declared == rpc.FailureUnspecified {
		return "none (the plugin declared no failure policy)"
	}
	return declared.String()
}

// applyFailurePolicy acts on a plugin generation that has ended.
//
// It MUST be called after handleSingleProcessCommandsRPC has RETURNED, never
// from inside it: releasePluginRegistrations waits on the runtime cleanup that
// function defers, so calling it earlier waits for a cleanup that cannot run.
//
// An acknowledged removal and these states are not failures:
//
//   - The daemon is stopping. Every plugin exits then, and none of them failed.
//   - The manager no longer holds this process under this name. A config reload
//     that removed the plugin, and a startup rollback, both take the slot away
//     (rollbackStartupProcess, startup.go).
//   - Another path already replaced it. The name then holds a different process,
//     which is the live one.
func (s *Server) applyFailurePolicy(proc *process.Process) {
	if s.ctx.Err() != nil {
		return
	}
	if s.applyRemovalFailurePolicy(proc) {
		return
	}
	pm := s.procManager.Load()
	if pm == nil {
		return
	}
	name := proc.Name()
	if pm.GetProcess(name) != proc {
		return
	}

	// Reported before the policy is acted on, because a plugin that asked not
	// to be started again still crashed, and `ze doctor` owes an operator that
	// row whatever ze does next.
	policy := pluginFailurePolicy(proc)
	exitStatus := proc.ExitStatus()
	pm.ReportCrash(name, exitStatus, policy.String())

	switch policy {
	case rpc.FailureRestart:
		logger().Warn("plugin exited, starting it again", "plugin", name, "code", diagnostic.CodeDoctorPluginCrashed)
		// The restart records the stop itself when it fails. This path holds
		// no lock, so the replacement's handshake runs as long as the daemon
		// does (restartPluginAfterExit).
		if err := s.restartPluginAfterExit(proc); err != nil {
			logger().Error("plugin exited and could not be started again, so it stays down",
				"plugin", name, "error", err, "code", diagnostic.CodeDoctorPluginStopped)
		}
	case rpc.FailureFatal:
		s.stopDaemonForPlugin(name, "the plugin declares that its failure stops ze")
	case rpc.FailureIgnore, rpc.FailureUnspecified:
		pm.RecordStopped(name, process.PluginStop{Cause: process.StopPolicyExit, Policy: policy.String(), ExitStatus: exitStatus})
	default:
		// Typed plugin registrations are open; unknown policies do not authorize a restart.
		pm.RecordStopped(name, process.PluginStop{Cause: process.StopPolicyExit, Policy: policy.String(), ExitStatus: exitStatus})
	}
}

// applyRemovalFailurePolicy settles a pending removal against its actual Bye
// result. The supervisor MUST call it after runtime cleanup; waiting for Bye
// inside the RPC handler would keep that callback from settling on disconnect.
func (s *Server) applyRemovalFailurePolicy(proc *process.Process) bool {
	s.removalMu.Lock()
	attempt := s.removals[proc]
	s.removalMu.Unlock()
	if attempt == nil {
		return false
	}
	select {
	case <-attempt.done:
		// Nonfatal removal recovery remains owned by its existing waiter.
		s.stopDaemonOnRemovalFailure(proc, attempt)
	case <-s.ctx.Done():
	}
	return true
}

// stopDaemonOnRemovalFailure classifies a failed fatal owner once, before its
// manager slot can be retired. Callers MUST observe attempt.done first and
// establish that the generation ended or its connection failed. A successful
// Bye excludes later faults. The result forbids recovery even while daemon
// shutdown is still being delivered asynchronously.
func (s *Server) stopDaemonOnRemovalFailure(proc *process.Process, attempt *pluginRemoval) bool {
	if attempt.err == nil {
		return false
	}
	if pluginFailurePolicy(proc) != rpc.FailureFatal {
		return false
	}
	attempt.failure.Do(func() {
		if s.ctx.Err() != nil {
			return
		}
		pm := s.procManager.Load()
		if pm == nil {
			return
		}
		if pm.GetProcess(proc.Name()) != proc {
			return
		}
		pm.ReportCrash(proc.Name(), proc.ExitStatus(), pluginFailurePolicy(proc).String())
		s.stopDaemonForPlugin(proc.Name(), "the fatal plugin failed before acknowledging removal")
	})
	return true
}

// stopDaemonOnStartupFailure stops the daemon when a plugin's startup failure is
// one ze must not carry on through. It is called for a process that did not
// reach StageRunning, after the phase has rolled it back.
//
// The two reasons are read in this order because a disagreement is refused
// BEFORE the registration is stored, so the process carries no declared policy
// to read at that point.
//
// ctx is the phase's own context. Under a reload's scope, a handshake that ctx
// ended, or one a sibling's failure ended at the barrier, says nothing about the
// plugin: the reload is refused and the running daemon carries on with the state
// it had (startupEndedOutsidePlugin). Boot has no prior state to keep, so a fatal
// plugin that did not reach running stops ze however its handshake ended: running
// without it is what the plugin declared must not happen.
func (s *Server) stopDaemonOnStartupFailure(ctx context.Context, proc *process.Process, cause error) {
	if errors.Is(cause, errRespawnDisagreement) {
		s.stopDaemonForPlugin(proc.Name(), errRespawnDisagreement.Error())
		return
	}
	if scope, scoped := s.pluginConfigureConfig(proc); scoped && scope.recovering {
		return
	}
	if pluginFailurePolicy(proc) != rpc.FailureFatal {
		return
	}
	if reloadOwnsStartup(ctx) && startupEndedOutsidePlugin(ctx, cause) {
		// The plugin is left not running, which the phase recorded through
		// RecordStopped before calling here (startup.go), raising
		// doctor-plugin-stopped on the bus. The refused reload itself is the
		// caller's to record (startAutoLoaded).
		logger().Warn("plugin startup was ended by its tier or its caller, not by the plugin, so ze keeps running without the plugin; correct the failure the log names and reload",
			"plugin", proc.Name(), "error", cause, "code", diagnostic.CodeDoctorPluginStopped)
		return
	}
	s.stopDaemonForPlugin(proc.Name(), "the plugin declares that its failure stops ze, and its startup ended: "+
		cause.Error()+"; correct that failure and start ze again")
}

// reloadOwnsStartup reports whether a startup phase runs inside a reload's
// transaction. reloadConfig puts its acceptance scope on the context of everything
// it starts, and boot's phases run under the server's own context, which never
// carries one.
func reloadOwnsStartup(ctx context.Context) bool {
	pending, _ := ctx.Value(reloadAcceptanceKey{}).(*reloadAcceptance)
	return pending != nil
}

// startupEndedOutsidePlugin reports whether cause says nothing about the plugin.
// It MUST be asked only under a reload's scope (reloadOwnsStartup).
//
// A barrier abort never does, under either state of ctx: the plugin finished its
// stage and was waiting for its tier, so a sibling's failure or the caller ended
// the wait. A sibling's failure can also reach the plugin one step before the
// wait, when its registration's configure allowance meets the failed tier, and
// stageTransition answers that the same way and records nothing of the sibling's.
// On both routes stageTransition has already recorded errStartupPluginEnded as
// the cause of a plugin whose connection had ended, and the first recorded cause
// stands, so an abort that reaches here belongs to a plugin that was still
// connected.
//
// A context error counts only once ctx has really ended. The two halves are both
// needed: ctx alone would forgive a plugin that crashed while a sibling held the
// tier until ctx ended, and the cause alone would forgive a plugin that outlasted a
// timeout of its own under a live ctx.
func startupEndedOutsidePlugin(ctx context.Context, cause error) bool {
	if errors.Is(cause, errStartupBarrierAborted) {
		return true
	}
	if ctx.Err() == nil {
		return false
	}
	if errors.Is(cause, context.Canceled) {
		return true
	}
	return errors.Is(cause, context.DeadlineExceeded)
}

// stopDaemonForPlugin asks the daemon to stop, and says which plugin asked for
// it. It takes the route `request shutdown` takes for a daemon with no BGP
// reactor (handleDaemonShutdown, system.go): shutdownFunc injects the SIGTERM
// the daemon's own teardown reads, and SignalShutdownRequested releases
// Server.Wait, which is what waitForServerDone (cmd/ze/hub/main.go) blocks on.
//
// Both are called because they end different waits, and a daemon wires both. A
// Server with no shutdownFunc, which is an embedded or ad-hoc one, is still
// stopped by the second: nothing here can succeed silently.
//
// The reason is written to the last-stop record first (last_stop.go), because
// nothing else survives the stop and the next `ze doctor` names it from there.
func (s *Server) stopDaemonForPlugin(name, reason string) {
	s.recordStop(LastStopEntry{Reason: LastStopFatalPolicy, Plugin: name, Detail: reason})
	logger().Error("stopping ze because a plugin failed; run ze doctor after the restart for the recorded cause",
		"plugin", name, "reason", reason, "code", diagnostic.CodeDoctorPluginFatalStop)
	if s.shutdownFunc != nil {
		s.shutdownFunc()
	}
	s.SignalShutdownRequested()
}
