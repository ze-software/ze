// Design: docs/architecture/api/process-protocol.md — the Stage-1 declaration a plugin makes about its own failure
// Overview: restart.go — restartPlugin, which this file calls to start a plugin again
// Related: ../process/manager.go — ProcessManager.Respawn, and the bounds a restart runs under
// Related: system.go — handleDaemonShutdown, the daemon-stop route a fatal plugin reuses

package server

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
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
	s.handleSingleProcessCommandsRPC(proc)
	s.applyFailurePolicy(proc)
}

// pluginFailurePolicy reports what ze does when this plugin fails. It answers
// one of the three concrete outcomes and never rpc.FailureUnspecified, so it is
// the single place a silent plugin is given a meaning.
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
// Three states are not a failure, and each one returns:
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
	pm := s.procManager.Load()
	if pm == nil {
		return
	}
	name := proc.Name()
	if pm.GetProcess(name) != proc {
		return
	}

	// Reported before the policy is read, because a plugin that asked not to be
	// started again still crashed, and `ze doctor` owes an operator that row
	// whatever ze does next.
	pm.ReportCrash(name)

	switch pluginFailurePolicy(proc) {
	case rpc.FailureRestart:
		logger().Warn("plugin exited, starting it again", "plugin", name)
		if err := s.restartPlugin(name); err != nil {
			logger().Error("plugin exited and could not be started again, so it stays down",
				"plugin", name, "error", err)
		}
	case rpc.FailureFatal:
		s.stopDaemonForPlugin(name, "the plugin declares that its failure stops ze")
	default:
		// rpc.FailureIgnore, and nothing else: pluginFailurePolicy answers only
		// the three outcomes and never the unspecified sentinel.
		logger().Warn("plugin exited and is not to be started again, so ze continues without it",
			"plugin", name)
	}
}

// stopDaemonOnStartupFailure stops the daemon when a plugin's startup failure is
// one ze must not carry on through. It is called for a process that did not
// reach StageRunning, after the phase has rolled it back.
//
// The two reasons are read in this order because a disagreement is refused
// BEFORE the registration is stored, so the process carries no declared policy
// to read at that point.
func (s *Server) stopDaemonOnStartupFailure(proc *process.Process, cause error) {
	if errors.Is(cause, errRespawnDisagreement) {
		s.stopDaemonForPlugin(proc.Name(), errRespawnDisagreement.Error())
		return
	}
	if pluginFailurePolicy(proc) == rpc.FailureFatal {
		s.stopDaemonForPlugin(proc.Name(), "the plugin declares that its failure stops ze")
	}
}

// stopDaemonForPlugin asks the daemon to stop, and says which plugin asked for
// it. It takes the route `request shutdown` takes for a daemon with no BGP
// reactor (handleDaemonShutdown, system.go): shutdownFunc injects the SIGTERM
// the daemon's own teardown reads, and signalShutdownRequested releases
// Server.Wait, which is what waitForServerDone (cmd/ze/hub/main.go) blocks on.
//
// Both are called because they end different waits, and a daemon wires both. A
// Server with no shutdownFunc, which is an embedded or ad-hoc one, is still
// stopped by the second: nothing here can succeed silently.
func (s *Server) stopDaemonForPlugin(name, reason string) {
	logger().Error("stopping ze because a plugin failed", "plugin", name, "reason", reason)
	if s.shutdownFunc != nil {
		s.shutdownFunc()
	}
	s.signalShutdownRequested()
}
