// Design: docs/architecture/api/process-protocol.md -- whole-daemon shutdown
// Related: startup.go -- selectedPluginGraph resolves implementation edges to processes
package server

import (
	"context"

	"github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// shutdownPluginTiers binds resolution to the actual manager whose sealed
// process snapshot it receives. Startup owns dependency declarations; the
// manager retains a graph refusal before its forced simultaneous stop. The
// server MUST persist that refusal after StopInOrder returns.
func shutdownPluginTiers(pm *process.ProcessManager) func([]plugin.PluginConfig) [][]string {
	return func(configs []plugin.PluginConfig) [][]string {
		tiers, err := registry.TopologicalGraphTiers(selectedPluginGraph(configs))
		if err != nil {
			pm.RecordShutdownOrderFailure(err)
			return nil
		}
		return tiers
	}
}

// closeAdmission fences future reloads and captures the current owner in one
// critical section. setCancel covers an owner whose handle is still pending.
func (l *txLock) closeAdmission() (context.CancelCauseFunc, <-chan struct{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	return l.cancel, l.done
}

// admissionError distinguishes terminal refusal from ordinary contention.
func (l *txLock) admissionError() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return transaction.ErrShutdown
	}
	return ErrReloadInProgress
}

// SetProcessSpawner MUST bind the fixed spawner before Start. Its required
// seal closes publication without waiting for SpawnMore's startup lock; the
// existing getter then returns the stable process owner for ordered cleanup.
func (s *Server) sealProcessAdmission() func() {
	if s.spawner == nil {
		// A directly injected ProcessManager is still joined by cleanup.
		return func() {}
	}
	finish := s.spawner.SealProcessAdmission()
	if pm, ok := s.spawner.GetProcessManager().(*process.ProcessManager); ok {
		if pm != nil {
			s.procManager.Store(pm)
		}
	}
	return finish
}
