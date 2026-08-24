// Design: docs/architecture/api/process-protocol.md — the 5-stage handshake a restart re-runs
// Overview: startup.go — runPluginPhase, the same handshake under a tier barrier
// Related: ../process/manager.go — ProcessManager.Respawn, which spawns the replacement process
// Related: reload_tx.go — restartPlugin, the one caller a broken-plugin rollback reaches

package server

import (
	"github.com/ze-software/ze/internal/component/command"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// releasePluginRegistrations removes every engine-side registration a stopped
// process made, and nothing else. The caller MUST have stopped proc first: the
// runtime wait below is what orders the removal against the plugin's own exit
// path, and it cannot complete while the process still runs.
//
// It is separate from rollbackStartupProcess because a RESTART needs exactly
// this half. A restart keeps the plugin's ProcessManager slot, because the
// respawn has already put the replacement process there, and keeps the plugin
// marked loaded, because it never stopped being part of this daemon.
//
// Everything removed here is keyed by the plugin NAME, so it MUST run before the
// replacement declares the same names: PluginRegistry.Register refuses a name it
// already holds, with no exemption for the same plugin registering again. The
// process's OWN exit path removes only what is keyed by the process POINTER
// (cleanupProcess, dispatch.go), which is what stops that path taking a
// replacement's registration away from it.
func (s *Server) releasePluginRegistrations(proc *process.Process) {
	if proc.Stage() >= plugin.StageRunning {
		proc.WaitRuntimeCleanup()
	} else {
		s.cleanupProcess(proc)
	}

	if s.registry != nil {
		s.registry.Unregister(proc.Name())
	}
	if s.capInjector != nil {
		s.capInjector.RemovePluginCapabilities(proc.Name())
	}
	s.removePluginFamilies(proc.Name())
	// The pipe aliases leave with the plugin, on all three paths that reach
	// here: a startup that failed at any stage, a running plugin the operator
	// removed from the config, and a plugin being restarted. A name that
	// outlived its plugin answers a command nobody serves, and it refuses that
	// plugin its own name when it starts again.
	command.UnregisterPluginAliases(proc.Name())
	// The answer shapes leave on the same three paths, and each command path
	// returns to what it held before the plugin declared. A shape that outlived
	// its plugin publishes operators for an answer nobody produces, and it
	// refuses that plugin its own declaration when it starts again.
	command.UnregisterPluginShapes(proc.Name())
}

// runUnbarrieredStartupHandshake drives the 5-stage handshake for a process that
// belongs to no tier: an ad-hoc session, or a replacement process a respawn
// produced. The coordinator is nilled for the call, which makes every stage
// barrier an immediate success, and restored afterwards.
//
// A tier barrier counts the processes of ONE phase and releases when they have
// all reached a stage. A process that joined outside a phase is not in that
// count, so waiting on the barrier would either hang or release a phase early.
func (s *Server) runUnbarrieredStartupHandshake(proc *process.Process) {
	s.coordinatorMu.Lock()
	saved := s.coordinator
	s.coordinator = nil
	s.coordinatorMu.Unlock()

	s.handleProcessStartupRPC(proc)

	s.coordinatorMu.Lock()
	s.coordinator = saved
	s.coordinatorMu.Unlock()
}

// restartHandshake brings a respawned process into the engine: it runs the
// 5-stage handshake, then starts the runtime handler that serves the plugin's
// engine-bound RPCs.
//
// Without it a respawn produces a process the engine never speaks to. Stage 1
// is what records the plugin's registration, Stage 2 is what delivers its config
// AND the exclusive-role claims other plugins hold (startup_claims.go), and
// Stage 5 is what registers its commands and its event subscriptions. A process
// that ran none of them holds a live pipe nobody reads: it stores nothing,
// answers no command, and has been told nothing about the roles it must stand
// its own default behavior down for.
//
// The runtime handler runs one goroutine per process lifecycle, which is the
// same shape runPluginPhase step (d) uses (ai/rules/goroutine-lifecycle.md).
func (s *Server) restartHandshake(proc *process.Process) error {
	s.runUnbarrieredStartupHandshake(proc)

	if proc.Stage() < plugin.StageRunning {
		err := startupFailureError(proc)
		logger().Error("plugin restart failed during handshake",
			"plugin", proc.Name(), "stage", proc.Stage(), "error", err)
		s.rollbackStartupProcess(proc)
		return err
	}

	s.wg.Go(func() {
		s.handleSingleProcessCommandsRPC(proc)
	})
	return nil
}
