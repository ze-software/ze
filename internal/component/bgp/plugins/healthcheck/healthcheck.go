// Design: plan/spec-healthcheck-0-umbrella.md -- healthcheck plugin design
//
// Package healthcheck implements a service healthcheck plugin for Ze.
// It monitors service availability by running shell commands periodically
// and controls BGP route announcement/withdrawal via watchdog groups.
package healthcheck

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// SetLogger sets the package-level logger.
func SetLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// RunHealthcheckPlugin is the in-process entry point for the healthcheck plugin.
func RunHealthcheckPlugin(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-healthcheck", conn)
	defer func() { _ = p.Close() }()

	mgr := newProbeManager(p)

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			probes, err := parseConfig(section.Data)
			if err != nil {
				logger().Error("config parse failed", "error", err)
				return err
			}
			mgr.applyConfig(probes)
		}
		return nil
	})

	logger().Info("healthcheck plugin starting")
	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"bgp"},
	})
	if err != nil {
		logger().Error("healthcheck plugin failed", "error", err)
		return 1
	}
	return 0
}

// probeManager manages the lifecycle of healthcheck probes.
type probeManager struct {
	plugin *sdk.Plugin
	probes map[string]*runningProbe // name -> running probe
	mu     sync.Mutex
}

// runningProbe tracks a running probe goroutine.
type runningProbe struct {
	config ProbeConfig
	cancel context.CancelFunc
	done   chan struct{}
}

func newProbeManager(p *sdk.Plugin) *probeManager {
	return &probeManager{
		plugin: p,
		probes: make(map[string]*runningProbe),
	}
}

// applyConfig starts/stops probes based on new configuration.
func (m *probeManager) applyConfig(configs []ProbeConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	newConfigs := make(map[string]ProbeConfig, len(configs))
	for _, c := range configs {
		newConfigs[c.Name] = c
	}

	// Stop probes that are no longer in config or changed.
	for name, rp := range m.probes {
		newCfg, exists := newConfigs[name]
		if !exists || newCfg != rp.config {
			rp.cancel()
			<-rp.done
			delete(m.probes, name)
		}
	}

	// Start new or changed probes.
	for name, cfg := range newConfigs {
		if _, running := m.probes[name]; running {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		m.probes[name] = &runningProbe{config: cfg, cancel: cancel, done: done}
		go m.runProbe(ctx, cfg, done)
	}

	logger().Info("healthcheck config applied", "probes", len(m.probes))
}

// runProbe runs a single healthcheck probe loop.
func (m *probeManager) runProbe(ctx context.Context, cfg ProbeConfig, done chan struct{}) {
	defer close(done)

	fsm := newFSM(cfg.Rise, cfg.Fall)

	// If disabled at startup, enter DISABLED directly.
	if cfg.Disable {
		fsm.state = StateDisabled
		m.dispatchStateAction(ctx, cfg, fsm.state)
		logger().Info("probe started disabled", "name", cfg.Name)
	}

	for {
		interval := time.Duration(cfg.Interval) * time.Second
		if fsm.state == StateRising || fsm.state == StateFalling {
			interval = time.Duration(cfg.FastInterval) * time.Second
		}

		// Single-check mode: interval=0 means one check then dormant.
		if cfg.Interval == 0 && fsm.state != StateInit {
			fsm.state = StateEnd
			logger().Info("probe dormant (interval=0)", "name", cfg.Name)
			<-ctx.Done()
			m.handleExit(ctx, cfg)
			return
		}

		// Wait for interval or shutdown (skip on first iteration).
		if fsm.state != StateInit {
			select {
			case <-ctx.Done():
				m.handleExit(ctx, cfg)
				return
			case <-time.After(interval):
			}
		}

		// DISABLED: sleep on interval, don't execute check.
		if fsm.state == StateDisabled {
			select {
			case <-ctx.Done():
				m.handleExit(ctx, cfg)
				return
			case <-time.After(interval):
			}
			continue
		}

		// Run check.
		success := runProbeCommand(ctx, cfg.Command, cfg.Timeout)

		// FSM transition.
		prevState := fsm.state
		fsm.step(success)

		// Dispatch on state change (or always if debounce=false).
		stateChanged := fsm.state != prevState
		if stateChanged || !cfg.Debounce {
			m.dispatchStateAction(ctx, cfg, fsm.state)
		}

		if cfg.Interval == 0 {
			continue
		}
	}
}

// dispatchStateAction dispatches watchdog commands based on the current state.
func (m *probeManager) dispatchStateAction(ctx context.Context, cfg ProbeConfig, state State) {
	switch state {
	case StateUp:
		cmd := fmt.Sprintf("watchdog announce %s med %d", cfg.Group, cfg.UpMetric)
		m.dispatchCommand(ctx, cfg.Name, cmd)
	case StateDown:
		if cfg.WithdrawOnDown {
			m.dispatchCommand(ctx, cfg.Name, fmt.Sprintf("watchdog withdraw %s", cfg.Group))
		} else {
			m.dispatchCommand(ctx, cfg.Name, fmt.Sprintf("watchdog announce %s med %d", cfg.Group, cfg.DownMetric))
		}
	case StateDisabled:
		if cfg.WithdrawOnDown {
			m.dispatchCommand(ctx, cfg.Name, fmt.Sprintf("watchdog withdraw %s", cfg.Group))
		} else {
			m.dispatchCommand(ctx, cfg.Name, fmt.Sprintf("watchdog announce %s med %d", cfg.Group, cfg.DisabledMetric))
		}
	case StateExit:
		m.dispatchCommand(ctx, cfg.Name, fmt.Sprintf("watchdog withdraw %s", cfg.Group))
	case StateInit, StateRising, StateFalling, StateEnd:
		// No watchdog action for intermediate or terminal states.
	}
}

// handleExit handles probe shutdown: withdraw routes.
func (m *probeManager) handleExit(_ context.Context, cfg ProbeConfig) {
	exitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m.dispatchStateAction(exitCtx, cfg, StateExit)
	logger().Info("probe exited", "name", cfg.Name)
}

// dispatchCommand sends a command to the watchdog plugin via DispatchCommand.
func (m *probeManager) dispatchCommand(ctx context.Context, probeName, command string) {
	status, _, err := m.plugin.DispatchCommand(ctx, command)
	if err != nil {
		logger().Warn("dispatch failed", "probe", probeName, "command", command, "error", err)
		return
	}
	if status != "done" {
		logger().Warn("dispatch unexpected status", "probe", probeName, "command", command, "status", status)
	}
}
