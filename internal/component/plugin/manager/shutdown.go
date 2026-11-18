// Design: docs/architecture/plugin-manager-wiring.md -- process lifetime ownership
// Related: manager.go -- process publication, startup_runner.go -- parent cancellation
package plugin

import (
	parent "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// SealProcessAdmission seals process creation for server-selected teardown.
// The caller MUST read GetProcessManager after sealing and invoke StopInOrder
// on that process manager, if any,
// before calling finish. That manager still owns cleanup and its deadline.
// Finish cancels the manager parent only after provider cleanup has finished.
// The seal never waits for mu, so it reaches an owner while SpawnMore waits for
// connect-back under that lock. Safe for concurrent use; terminal, never reused.
func (m *Manager) SealProcessAdmission() func() {
	m.admissionMu.Lock()
	defer m.admissionMu.Unlock()
	m.admissionClosed = true
	cancel := m.cancel
	return func() {
		if cancel != nil {
			cancel()
		}
	}
}

func (m *Manager) processAdmissionError() error {
	m.admissionMu.Lock()
	defer m.admissionMu.Unlock()
	if m.admissionClosed {
		return process.ErrDaemonStopping
	}
	return nil
}

// prepareProcessManager runs under mu after acceptor preparation. Publication
// and the terminal seal share admissionMu; every published owner is completely
// wired before its first Start, and no process can be created after the seal.
func (m *Manager) prepareProcessManager(configs []parent.PluginConfig) (*process.ProcessManager, bool, error) {
	m.admissionMu.Lock()
	defer m.admissionMu.Unlock()
	if m.admissionClosed {
		// Acceptor preparation precedes the tiny publication section and can
		// lose to the seal. It has started no process; retire its listener.
		if m.acceptor != nil {
			m.acceptor.Stop()
		}
		return nil, false, process.ErrDaemonStopping
	}
	first := m.procManager == nil
	if first {
		m.procManager = process.NewProcessManager(configs)
		if m.metricsRegistry != nil {
			m.procManager.SetMetricsRegistry(m.metricsRegistry)
		}
	}
	if m.acceptor != nil {
		m.procManager.SetAcceptor(m.acceptor)
	}
	return m.procManager, first, nil
}
