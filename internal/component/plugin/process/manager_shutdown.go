// Design: docs/architecture/api/process-protocol.md -- whole-daemon shutdown
// Related: manager.go -- process admission, stop ownership and final group joins
package process

import (
	"context"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/syncutil"
)

// managerShutdown is the immutable terminal fence and shared cleanup deadline.
// It is published under mu with stop ownership, never replaced or cleared.
type managerShutdown struct{ ctx context.Context }

// ShutdownOrderFix reads the stop's cause before the operator reports or fixes
// the plugin dependency declarations or server wiring that refused ordering.
const ShutdownOrderFix = "journalctl -u ze"

// RecordShutdownOrderFailure retains the first genuine ordering failure of the
// terminal stop, logs it and raises its error event. The detail is bounded by
// the report bus's message bound; repeated stop entries add no state or events.
// A resolver MUST call this when it returns forced simultaneous tiers because
// dependency resolution failed. Safe for concurrent use.
func (pm *ProcessManager) RecordShutdownOrderFailure(cause error) {
	if cause == nil {
		return // No refusal was supplied: this is not an ordering failure.
	}
	failure := opererr.New(
		report.FitMessage("plugin shutdown cannot preserve dependency order: "+cause.Error()),
		"providers may stop before the consumers that need them finish cleanup",
		ShutdownOrderFix)
	pm.mu.Lock()
	if pm.shutdownOrderFailure != nil {
		pm.mu.Unlock()
		return
	}
	pm.shutdownOrderFailure = failure
	pm.mu.Unlock()
	logger().Warn("plugin shutdown cannot preserve dependency order; stopping every plugin together",
		"error", failure, "fix", ShutdownOrderFix, "code", CodeDoctorPluginShutdownOrder)
	report.RaiseErrorFix(reportSourcePlugin, CodeDoctorPluginShutdownOrder, "plugin-shutdown",
		report.FitMessage(failure.Error()), ShutdownOrderFix)
}

// ShutdownOrderFailure answers the immutable first refusal retained by the
// terminal stop, or nil when that stop resolved its dependency order. The
// daemon owner MUST read this after StopInOrder returns and write a last-stop
// record before its store closes. Safe for concurrent use.
func (pm *ProcessManager) ShutdownOrderFailure() *opererr.Error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.shutdownOrderFailure
}

// startProcess admits a generation before Start. Failure cleanup is mandatory
// even when no terminal owner exists yet: Start may have forked a real child.
func (pm *ProcessManager) startProcess(cfg plugin.PluginConfig, environment []string) error {
	pm.mu.Lock()
	if pm.shutdown.Load() != nil {
		pm.mu.Unlock()
		return ErrDaemonStopping
	}
	if pm.stopActive {
		pm.mu.Unlock()
		return ErrDaemonStopping
	}
	proc := pm.admitSpawnLocked(cfg)
	proc.SetConfigEnvironment(environment)
	ctx := pm.ctx
	pm.mu.Unlock()
	defer pm.spawns.Done()
	return pm.startAdmitted(proc, ctx)
}

// admitSpawnLocked MUST be called with mu and open admission. Every Start site,
// Respawn included, registers its owner before releasing that publication lock.
func (pm *ProcessManager) admitSpawnLocked(cfg plugin.PluginConfig) *Process {
	proc := NewProcess(cfg)
	pm.wireMetrics(proc)
	if pm.acceptor != nil {
		if !cfg.Internal {
			proc.SetAcceptor(pm.acceptor)
		}
	}
	pm.stopping.Store(false)
	if pm.starting == nil {
		pm.starting = make(map[*Process]struct{})
	}
	pm.starting[proc] = struct{}{}
	pm.spawns.Add(1)
	return proc
}

func (pm *ProcessManager) startAdmitted(proc *Process, ctx context.Context) error {
	if err := proc.StartWithContext(ctx); err != nil {
		pm.retireRejectedStart(proc)
		return err
	}
	pm.mu.Lock()
	if pm.shutdown.Load() != nil {
		pm.mu.Unlock()
		pm.retireRejectedStart(proc)
		return ErrDaemonStopping
	}
	pm.processes[proc.Name()] = proc
	delete(pm.starting, proc)
	pm.mu.Unlock()
	return nil
}

// retireRejectedStart MUST run for every failed or refused admitted generation.
// Wait joins the actual RunEngine or external monitor/stderr workers, not just
// its closed connection. A terminal owner supplies the existing shared deadline.
func (pm *ProcessManager) retireRejectedStart(proc *Process) {
	var ctx context.Context
	if owner := pm.shutdown.Load(); owner != nil {
		ctx = owner.ctx
	} else {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), pluginStopGrace)
		defer cancel()
	}
	proc.Stop()
	if err := proc.Wait(ctx); err != nil {
		RecordStopUnfinished(proc, pluginStopGrace, err)
		return
	}
	pm.mu.Lock()
	delete(pm.starting, proc)
	pm.mu.Unlock()
}

// joinStarting runs after stop ownership has fenced admission and stopped the
// acceptor. No Add can now race this wait. Never cancel a Process under mu:
// Start holds its own lock while connecting back; let that call finish first.
func (pm *ProcessManager) joinStarting(ctx context.Context) []string {
	spawnErr := syncutil.WaitGroupWait(ctx, &pm.spawns)
	pm.mu.RLock()
	owners := make([]*Process, 0, len(pm.starting))
	for proc := range pm.starting {
		owners = append(owners, proc)
	}
	pm.mu.RUnlock()
	var late []string
	for _, proc := range owners {
		if spawnErr != nil {
			late = append(late, proc.Name())
			continue
		}
		// These are failed owners retained after a prior join timeout; all Start
		// calls have returned, so Process.Wait cannot race goroutine admission.
		if err := proc.Wait(ctx); err != nil {
			late = append(late, proc.Name())
			continue
		}
		pm.mu.Lock()
		delete(pm.starting, proc)
		pm.mu.Unlock()
	}
	return late
}

// stopEngines signals a whole tier before joining its engines. Every tier
// shares one deadline, and every remaining tier is signaled after expiry.
// Ordered stop waits for real external process exit, not its no-op WaitEngine.
func (pm *ProcessManager) stopEngines(ctx context.Context, processes []*Process, order func([]plugin.PluginConfig) [][]string, terminal bool) []string {
	configs := make([]plugin.PluginConfig, 0, len(processes))
	byName := make(map[string]*Process, len(processes))
	names := make([]string, 0, len(processes))
	for _, proc := range processes {
		configs = append(configs, proc.Config())
		byName[proc.Name()] = proc
		names = append(names, proc.Name())
	}
	var tiers [][]string
	if order != nil {
		tiers = order(configs)
	}
	if len(tiers) == 0 {
		tiers = [][]string{names}
	}
	var late []string
	for _, tier := range slices.Backward(tiers) {
		for _, name := range tier {
			byName[name].Stop()
		}
		// One join per process lifetime, concurrent within this tier. A stuck
		// peer must not spend every other peer's opportunity to join before
		// the shared deadline. Providers still wait for this complete tier.
		var joined sync.WaitGroup
		var lateMu sync.Mutex
		for _, name := range tier {
			proc := byName[name]
			joined.Go(func() {
				if err := waitShutdownProcess(ctx, proc, terminal); err != nil {
					lateMu.Lock()
					late = append(late, proc.Name())
					lateMu.Unlock()
				}
			})
		}
		joined.Wait()
	}
	return late
}

// A fork first joins through the actual Process.Wait, never its no-op
// WaitEngine. If the shared deadline won, its monitor's physical completion
// signal proves reaping and stderr already finished even when the new group
// waiter was not scheduled. Internal engines have the analogous EngineDone
// boundary. Final process-group joins remain the stop owner's responsibility.
func waitShutdownProcess(ctx context.Context, proc *Process, terminal bool) error {
	if terminal {
		if !proc.Config().Internal {
			if err := proc.Wait(ctx); err != nil {
				select {
				case <-proc.externalDone:
					return nil
				default:
					return err
				}
			}
			return nil
		}
	}
	if err := proc.WaitEngine(ctx); err != nil {
		select {
		case <-proc.EngineDone():
			return nil
		default:
			return err
		}
	}
	return nil
}
