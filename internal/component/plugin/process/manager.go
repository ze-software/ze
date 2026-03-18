// Design: docs/architecture/api/process-protocol.md — multi-process coordination and respawn
// Overview: process.go — Process struct and lifecycle
// Related: delivery.go — event delivery pipeline

package process

import (
	"context"
	"errors"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
)

const (
	// RespawnLimit is max respawns per RespawnWindow before disabling.
	// ExaBGP: respawn_number=5 per ~63 seconds.
	RespawnLimit = 5

	// RespawnWindow is the time window for respawn limit tracking.
	RespawnWindow = 60 * time.Second

	// MaxTotalRespawns is the cumulative respawn limit before permanent disable.
	// Prevents a permanently broken plugin from cycling indefinitely across windows.
	MaxTotalRespawns = 20
)

// Respawn errors.
var (
	ErrRespawnLimitExceeded = errors.New("respawn limit exceeded")
	ErrProcessDisabled      = errors.New("process disabled due to respawn limit")
	ErrProcessNotFound      = errors.New("process not found")
)

// ProcessManager manages multiple external processes.
type ProcessManager struct {
	configs   []plugin.PluginConfig
	processes map[string]*Process

	// Respawn tracking: name -> list of respawn timestamps
	respawnTimes map[string][]time.Time

	// Cumulative respawn counts (never reset)
	totalRespawns map[string]int

	// Disabled processes (respawn limit exceeded)
	disabled map[string]bool

	// TLS acceptor for external plugin connect-back (nil = use socketpairs).
	acceptor *ipc.PluginAcceptor

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
}

// SetAcceptor sets the TLS acceptor for external plugin connect-back.
// Must be called before StartWithContext.
func (pm *ProcessManager) SetAcceptor(a *ipc.PluginAcceptor) {
	pm.acceptor = a
}

// NewProcessManager creates a new process manager.
func NewProcessManager(configs []plugin.PluginConfig) *ProcessManager {
	return &ProcessManager{
		configs:       configs,
		processes:     make(map[string]*Process),
		respawnTimes:  make(map[string][]time.Time),
		totalRespawns: make(map[string]int),
		disabled:      make(map[string]bool),
	}
}

// Start starts all configured processes.
func (pm *ProcessManager) Start() error {
	return pm.StartWithContext(context.Background())
}

// StartWithContext starts all configured processes with the given context.
func (pm *ProcessManager) StartWithContext(ctx context.Context) error {
	pm.ctx, pm.cancel = context.WithCancel(ctx)

	for _, cfg := range pm.configs {
		proc := NewProcess(cfg)
		// Pass TLS acceptor to external plugins for connect-back.
		if pm.acceptor != nil && !cfg.Internal {
			proc.SetAcceptor(pm.acceptor)
		}
		if err := proc.StartWithContext(pm.ctx); err != nil {
			// Stop already started processes
			pm.Stop()
			return err
		}

		pm.mu.Lock()
		pm.processes[cfg.Name] = proc
		pm.mu.Unlock()
	}

	return nil
}

// Stop stops all processes.
// Cancels context and closes connections immediately, which unblocks plugin
// reads on net.Pipe and causes prompt exit. No bye round-trip — closing the
// connection is the shutdown signal for internal plugins, and context
// cancellation kills external plugins via exec.CommandContext.
func (pm *ProcessManager) Stop() {
	// Cancel context and close all connections immediately.
	// For internal plugins: closing engine-side net.Pipe unblocks the plugin's
	// ReadRequest, causing it to return an error and exit the event loop.
	// For external plugins: context cancellation kills the subprocess.
	if pm.cancel != nil {
		pm.cancel()
	}

	// Stop TLS acceptor if running (closes listener, cancels accept loop).
	if pm.acceptor != nil {
		pm.acceptor.Stop()
	}

	pm.mu.Lock()
	for _, proc := range pm.processes {
		proc.Stop()
	}
	pm.mu.Unlock()

	// Wait briefly for processes to exit. Should be near-instant since
	// closing connections immediately unblocks plugin reads.
	pm.mu.RLock()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	var waitWg sync.WaitGroup
	for _, proc := range pm.processes {
		waitWg.Add(1)
		go func(p *Process) {
			defer waitWg.Done()
			_ = p.Wait(waitCtx)
		}(proc)
	}
	pm.mu.RUnlock()
	waitWg.Wait()
	waitCancel()

	pm.mu.Lock()
	pm.processes = make(map[string]*Process)
	pm.mu.Unlock()
}

// Wait waits for all processes to stop.
func (pm *ProcessManager) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		pm.mu.RLock()
		for _, proc := range pm.processes {
			_ = proc.Wait(ctx)
		}
		pm.mu.RUnlock()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetProcess returns a process by name, or nil if not found.
func (pm *ProcessManager) GetProcess(name string) *Process {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.processes[name]
}

// AddProcess registers a process by name. Used by tests to inject mock processes.
func (pm *ProcessManager) AddProcess(name string, proc *Process) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.processes[name] = proc
}

// AllProcesses returns a snapshot of all processes.
// Caller may iterate and filter the returned slice without holding the lock.
func (pm *ProcessManager) AllProcesses() []*Process {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*Process, 0, len(pm.processes))
	for _, proc := range pm.processes {
		result = append(result, proc)
	}
	return result
}

// ProcessCount returns the number of running processes.
func (pm *ProcessManager) ProcessCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	count := 0
	for _, proc := range pm.processes {
		if proc.Running() {
			count++
		}
	}
	return count
}

// IsRunning returns true if the named process is running.
func (pm *ProcessManager) IsRunning(name string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	proc, ok := pm.processes[name]
	if !ok {
		return false
	}
	return proc.Running()
}

// IsDisabled returns true if the named process is disabled due to respawn limit.
func (pm *ProcessManager) IsDisabled(name string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.disabled[name]
}

// Respawn restarts a process, enforcing respawn limits.
// Returns ErrRespawnLimitExceeded if limit exceeded within window.
// Returns ErrProcessDisabled if process was previously disabled.
// Returns ErrProcessNotFound if process name not in configuration.
// Returns error if ProcessManager was not started (ctx is nil).
func (pm *ProcessManager) Respawn(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if ProcessManager was started
	if pm.ctx == nil {
		return errors.New("process manager not started")
	}

	// Check if already disabled
	if pm.disabled[name] {
		return ErrProcessDisabled
	}

	// Find config
	var cfg *plugin.PluginConfig
	for i := range pm.configs {
		if pm.configs[i].Name == name {
			cfg = &pm.configs[i]
			break
		}
	}
	if cfg == nil {
		return ErrProcessNotFound
	}

	// Check respawn enabled
	if !cfg.RespawnEnabled && !cfg.Respawn {
		return nil // Respawn not enabled, nothing to do
	}

	now := time.Now()

	// Clean up old respawn times (outside window)
	var validTimes []time.Time
	for _, t := range pm.respawnTimes[name] {
		if now.Sub(t) < RespawnWindow {
			validTimes = append(validTimes, t)
		}
	}

	// Check per-window limit
	if len(validTimes) >= RespawnLimit {
		pm.disabled[name] = true
		logger().Warn("respawn limit exceeded, process disabled",
			"process", name, "limit", RespawnLimit, "window", RespawnWindow)
		return ErrRespawnLimitExceeded
	}

	// Check cumulative limit (prevents cycling indefinitely across windows)
	pm.totalRespawns[name]++
	if pm.totalRespawns[name] > MaxTotalRespawns {
		pm.disabled[name] = true
		logger().Warn("cumulative respawn limit exceeded, process disabled",
			"process", name, "total", pm.totalRespawns[name], "limit", MaxTotalRespawns)
		return ErrRespawnLimitExceeded
	}

	// Record this respawn
	validTimes = append(validTimes, now)
	pm.respawnTimes[name] = validTimes

	// Stop existing process if running
	if proc, ok := pm.processes[name]; ok && proc.Running() {
		proc.Stop()
		// Wait briefly for stop
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = proc.Wait(ctx)
		cancel()
	}

	// Start new process with acceptor if configured.
	newProc := NewProcess(*cfg)
	if pm.acceptor != nil && !cfg.Internal {
		newProc.SetAcceptor(pm.acceptor)
	}
	if err := newProc.StartWithContext(pm.ctx); err != nil {
		return err
	}
	pm.processes[name] = newProc

	return nil
}
