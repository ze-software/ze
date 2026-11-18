// Design: docs/architecture/plugin-manager-wiring.md -- complete plugin startup
// Related: manager.go -- the engine joins plugins before starting subsystems

package plugin

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/ze"
)

type managerStartup struct{ done chan struct{} }

// SetStartupRunner binds the protocol owner's startup to StartAll. The runner
// MUST return only after its plugins initialize, and MUST honor cancellation.
// The caller MUST bind it before StartAll. StopAll cancels and joins that call.
// Safe for concurrent use; replacing an already-bound runner is refused.
func (m *Manager) SetStartupRunner(run func(context.Context) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return errAlreadyStarted
	}
	if m.starting != nil {
		return errAlreadyStarted
	}
	if m.startupRunner != nil {
		return errors.New("plugin startup runner is already bound")
	}
	if run == nil {
		return errors.New("plugin startup runner is nil")
	}
	m.startupRunner = run
	return nil
}

func (m *Manager) startAll(ctx context.Context, eventBus ze.EventBus, config ze.ConfigProvider) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return errAlreadyStarted
	}
	if m.starting != nil {
		m.mu.Unlock()
		return errAlreadyStarted
	}
	m.admissionMu.Lock()
	if m.admissionClosed {
		m.admissionMu.Unlock()
		m.mu.Unlock()
		return process.ErrDaemonStopping
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.admissionMu.Unlock()
	m.eventBus, m.config = eventBus, config
	m.started = true
	for _, state := range m.plugins {
		state.running = true
	}
	owner := &managerStartup{done: make(chan struct{})}
	m.starting = owner
	run, lifetime := m.startupRunner, m.ctx
	m.mu.Unlock()

	var failure error
	if run != nil {
		failure = run(lifetime)
	}
	if failure == nil {
		failure = lifetime.Err()
	}

	m.mu.Lock()
	if failure != nil {
		// StopAll may have canceled this same startup while it ran. No new
		// StartAll can replace its identity until the done receipt is closed.
		if m.starting == owner {
			m.stopAllLocked()
		}
	}
	if m.starting == owner {
		m.starting = nil
	}
	close(owner.done)
	m.mu.Unlock()
	if failure != nil {
		return fmt.Errorf("plugin startup: %w", failure)
	}
	return nil
}

func (m *Manager) stopAllLocked() {
	m.admissionMu.Lock()
	if m.admissionClosed {
		// The server has selected an ordered terminal join. Canceling this parent
		// here would close providers before dependents finish their cleanup.
		m.admissionMu.Unlock()
		return
	}
	// Cancellation must precede the process drain: a startup runner may itself
	// be waiting for the process whose shutdown the drain joins.
	if m.cancel != nil {
		m.cancel()
	}
	m.admissionMu.Unlock()
	if m.procManager != nil {
		m.procManager.Stop()
		m.admissionMu.Lock()
		if m.admissionClosed {
			// A terminal seal may have captured this owner while reusable Stop
			// was joining it. Keep the captured pointer available to its caller.
			m.admissionMu.Unlock()
			return
		}
		m.procManager = nil
		m.admissionMu.Unlock()
	}
	// The held plugins went with the process manager, so their record goes too.
	m.declared = nil
	if m.acceptor != nil {
		m.acceptor.Stop()
		m.acceptor = nil
	}
	for _, state := range m.plugins {
		state.running = false
	}
	m.started = false
}
