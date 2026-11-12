// Design: docs/architecture/api/process-protocol.md — the 5-stage handshake a restart re-runs
// Overview: startup.go — runPluginPhase, the same handshake under a tier barrier
// Related: ../process/manager.go — ProcessManager.Respawn, which spawns the replacement process
// Related: reload_tx.go — restartPlugin, which a broken-plugin rollback reaches
// Related: failure_policy.go — superviseProcess, which decides what a plugin's exit means

package server

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// restartCleanupGrace bounds how long a restart waits for the runtime cleanup
// of the process it stopped. That cleanup runs once the process's in-flight
// calls into the engine have returned (handleSingleProcessCommandsRPC defers
// it behind them), and nothing else bounds such a call: a call stuck in the
// engine, or the plugin's own request to restart itself, would hold the
// restart, and the config transaction lock with it, for good.
//
// It is the daemon's 3-second shutdown budget, which txShutdownGrace already
// names, and which process.pluginStopGrace gives every plugin goroutine to
// finish once the connections are closed: this wait is that same wait, for
// the server-side goroutine that serves the plugin's calls into the engine.
// Process.StopJoinBound is not this bound. It is how long the stopped plugin
// itself gets to exit (its exit grace plus a margin), which Respawn has
// already waited out before this wait starts; a call into the engine is not
// ended by the plugin's exit, so a bound sized for that exit would fail a
// restart whose engine call the daemon's own stop would still wait for.
const restartCleanupGrace = txShutdownGrace

// errReplacedCleanupLate says the process a restart stopped has not run its
// runtime cleanup. That cleanup removes what is keyed by the plugin's NAME
// (its routes, its cleanup hooks' state), so a replacement registered before
// it runs would lose those to it. It says only what is known: the cleanup
// waits behind the process's in-flight calls into the engine, and a cleanup
// that has started but is slow expires the wait too.
var errReplacedCleanupLate = errors.New("a stopped process of the plugin has not finished its cleanup: " +
	"it may be waiting for a call that process made into ze, or still be running")

// awaitReplacedCleanup waits, at most restartCleanupGrace, for the runtime
// cleanup of old, a process the restart has stopped. A process that never
// reached running has no runtime handler, so releasePluginRegistrations runs
// its cleanup itself and there is nothing to wait for.
//
// An expired wait is a failed restart, never a continue: the caller MUST NOT
// register a replacement under the plugin's name. old is then kept in
// lateCleanups until its cleanup runs, so a later restart of the name is
// refused instead of starting a generation that cleanup would strip.
func (s *Server) awaitReplacedCleanup(old *process.Process) error {
	if old.Stage() < plugin.StageRunning {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), restartCleanupGrace)
	defer cancel()
	if err := old.WaitRuntimeCleanupContext(ctx); err == nil {
		return nil
	}
	s.lateCleanups.owe(old)
	logger().Error("plugin restart failed: the stopped process did not finish its cleanup",
		"plugin", old.Name(), "waited", restartCleanupGrace, "code", diagnostic.CodeDoctorPluginStopped)
	return fmt.Errorf("%w (waited %s)", errReplacedCleanupLate, restartCleanupGrace)
}

// handshakeScope answers the context a restart runs its replacement's
// startup handshake under, and the cancel that releases it. restartHandshake
// cancels it once the handshake has returned.
type handshakeScope func(proc *process.Process) (context.Context, context.CancelFunc)

// restartHandshakeStages is the number of stages a replacement's handshake
// runs, Registration through Ready: each is what the tiered startup bounds
// with a stage timeout.
const restartHandshakeStages = time.Duration(plugin.StageRunning - plugin.StageRegistration)

// errRestartHandshakeLate says a restart's replacement did not reach running
// within restartHandshakeBound.
var errRestartHandshakeLate = errors.New("the new process did not finish its startup handshake in time")

// restartHandshakeBound is how long a restart that holds the config
// transaction lock lets its replacement's handshake run: one stage timeout
// for each of its five stages. The stage timeout is the one the tiered
// startup uses for a stage that does not answer (stageTransition): the
// plugin's own `stage-timeout`, else ze.plugin.stage.timeout, else
// defaultStageTimeout, so 25s by default. The unbarriered handshake has no
// stall timeout of its own (runUnbarrieredStartupHandshake), so without this
// a replacement that connects and never sends Stage 1, or never answers its
// Configure, would hold the lock for as long as the daemon runs.
func restartHandshakeBound(proc *process.Process) time.Duration {
	stage := proc.Config().StageTimeout
	if stage == 0 {
		stage = stageTimeoutFromEnv()
	}
	return restartHandshakeStages * stage
}

// boundedRestartHandshake is the handshakeScope of a restart that holds the
// config transaction lock (the operator's command, the reload rollback):
// restartHandshakeBound, inside the daemon's lifetime.
func (s *Server) boundedRestartHandshake(proc *process.Process) (context.Context, context.CancelFunc) {
	return context.WithTimeout(s.ctx, restartHandshakeBound(proc))
}

// lifetimeRestartHandshake is the handshakeScope of the failure policy's
// restart, which holds no lock: its handshake runs as long as the daemon
// does, as it always has, so a replacement that is slow but progressing is
// never failed for its pace. Its cancel releases nothing.
func (s *Server) lifetimeRestartHandshake(*process.Process) (context.Context, context.CancelFunc) {
	return s.ctx, func() {}
}

// rollbackReplacement stops and releases a replacement a restart could not
// bring into the engine: its handshake failed, or the process it replaces
// still owed its cleanup. Such a process never reached running, so it has no
// runtime handler, and releasePluginRegistrations runs its cleanup itself.
//
// The wait for its end is bounded by its own stop bound (StopJoinBound), as
// Respawn's join of the process it replaces is, because the operator's
// restart and the reload rollback call this with the config transaction lock
// held. A wait that expires is logged and the rollback goes on to release
// the registrations: nothing that follows needs the process to have ended.
// The stop was ordered either way. For a forked plugin superviseExit kills
// its process group once pluginExitGrace has passed (on Linux; elsewhere the
// stop kills it at once), and what can still run
// at this bound is its reaper, its stderr relay (held open by a process the
// plugin started) or its event delivery. A plugin that runs in the daemon has
// no group to kill: an engine that does not return on its closed connection
// keeps running until it returns, because nothing ends a goroutine from
// outside, and only a restart of the ze daemon ends it then.
//
// The failure policy's restart takes the same bound. For it the bound
// changes only a rollback whose process never ends: it is released and
// recorded stopped at the bound, where it used to wait until the daemon
// stopped, and was never recorded stopped.
func (s *Server) rollbackReplacement(proc *process.Process) {
	s.takeAndStopProcess(proc)
	bound := proc.StopJoinBound()
	ctx, cancel := context.WithTimeout(s.Context(), bound)
	err := proc.Wait(ctx)
	cancel()
	if err != nil {
		logger().Warn("plugin restart: the new process ze stopped did not end within its stop bound, so ze releases it without waiting",
			"plugin", proc.Name(), "bound", bound, "error", err, "code", diagnostic.CodeDoctorPluginStopped)
	}
	s.releaseStoppedProcess(proc)
}

// lateCleanupRecord holds, per plugin name, a stopped process whose runtime
// cleanup outlived the restart that stopped it. Its zero value is empty and
// ready. Safe for concurrent use.
type lateCleanupRecord struct {
	mu    sync.Mutex
	procs map[string]*process.Process
}

// owe records proc as a process whose runtime cleanup has not run yet.
func (r *lateCleanupRecord) owe(proc *process.Process) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.procs == nil {
		r.procs = make(map[string]*process.Process)
	}
	r.procs[proc.Name()] = proc
}

// owed reports whether a stopped process of name still owes its runtime
// cleanup. One whose cleanup has run since is forgotten.
func (r *lateCleanupRecord) owed(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	proc, recorded := r.procs[name]
	// refuse-by-default: guards restartPluginProcess -- a name with no late cleanup recorded owes none, so its restart may run
	if !recorded {
		return false
	}
	// A probe, not a wait: an ended context answers at once, and the wait
	// answers nil whenever the cleanup has run, even with ctx already ended.
	probe, cancel := context.WithCancel(context.Background())
	cancel()
	if proc.WaitRuntimeCleanupContext(probe) != nil {
		return true
	}
	delete(r.procs, name)
	return false
}

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
//
// With no barrier, no stall timeout applies either: ctx is the handshake's
// only bound, reaching every read, send and Configure wait. An ad-hoc session
// passes the daemon's lifetime; a restart passes its handshakeScope's answer.
func (s *Server) runUnbarrieredStartupHandshake(ctx context.Context, proc *process.Process) {
	s.coordinatorMu.Lock()
	saved := s.coordinator
	s.coordinator = nil
	s.coordinatorMu.Unlock()

	s.handleProcessStartupRPCWithin(ctx, proc)

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
//
// handshake decides how long the handshake may take. A handshake that runs
// out of it fails like any other: the replacement is stopped and released by
// rollbackReplacement, and the error says the bound expired.
func (s *Server) restartHandshake(proc *process.Process, handshake handshakeScope) error {
	ctx, cancel := handshake(proc)
	s.runUnbarrieredStartupHandshake(ctx, proc)
	expired := errors.Is(ctx.Err(), context.DeadlineExceeded)
	cancel()

	if proc.Stage() < plugin.StageRunning {
		err := startupFailureError(proc)
		if expired {
			err = fmt.Errorf("%w (bound %s): %w", errRestartHandshakeLate, restartHandshakeBound(proc), err)
		}
		logger().Error("plugin restart failed during handshake",
			"plugin", proc.Name(), "stage", proc.Stage(), "error", err, "code", diagnostic.CodeDoctorPluginStopped)
		s.rollbackReplacement(proc)
		return err
	}

	s.wg.Go(func() {
		s.superviseProcess(proc)
	})

	// A replacement that joins after signalStartupComplete has run reaches no
	// other delivery, because sendPostStartupToAll fans out once and no later
	// phase calls it. Without this line its OnAllPluginsReady handler never
	// runs, which is the handler where a plugin takes an exclusive role over
	// from the plugin that holds it (bgp-rs claimReplayOwnership,
	// bgp/plugins/rs/server_handlers.go). A restarted plugin would come back
	// registered, configured and subscribed, and still not do the one piece of
	// work that needs every other plugin present.
	//
	// It is the same delivery a config reload makes for a plugin it starts
	// mid-life (autoLoadForNewConfigPaths, startup_autoload.go), for the same
	// reason and by the same call.
	//
	// A restart DURING startup takes neither branch's work twice: the
	// replacement sits in the process manager under the plugin's name, so the
	// coming sendPostStartupToAll reaches it, and it reaches it at the point the
	// callback's contract names, with the registries frozen and every phase's
	// commands registered. Delivering here as well would run that plugin's
	// OnAllPluginsReady twice, and those handlers are written for a single call
	// (poststartup.go). A plugin's own exit reaches this path from
	// applyFailurePolicy, whose supervisors start at the end of each
	// runPluginPhase, so the window is real rather than theoretical.
	if s.startupComplete() {
		s.sendPostStartupToNames([]string{proc.Name()})
	}
	return nil
}
