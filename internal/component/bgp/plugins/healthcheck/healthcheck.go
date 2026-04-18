// Design: plan/spec-healthcheck-0-umbrella.md -- healthcheck plugin design
// Detail: config.go -- config parsing and validation
// Detail: fsm.go -- 8-state FSM with trigger shortcuts
// Detail: hooks.go -- async hook execution with timeout
// Detail: ip.go -- VIP management via iface
// Detail: probe.go -- shell command execution with process group kill
//
// Package healthcheck implements a service healthcheck plugin for Ze.
// It monitors service availability by running shell commands periodically
// and controls BGP route announcement/withdrawal via watchdog groups.
package healthcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

const (
	statusDone  = "done"
	statusError = "error"
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

	mgr := newProbeManager(p, true)

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
			if err := mgr.validateConfig(probes); err != nil {
				logger().Error("config validation failed", "error", err)
				return err
			}
			mgr.applyConfig(probes)
		}
		return nil
	})

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, string, error) {
		return mgr.handleCommand(command, args)
	})

	logger().Info("healthcheck plugin starting")
	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"bgp"},
		Commands: []sdk.CommandDecl{
			{Name: "healthcheck show", Description: "Show healthcheck probe status"},
			{Name: "healthcheck reset", Description: "Reset healthcheck probe to INIT"},
		},
	})
	if err != nil {
		logger().Error("healthcheck plugin failed", "error", err)
		return 1
	}
	return 0
}

// probeManager manages the lifecycle of healthcheck probes.
type probeManager struct {
	plugin     *sdk.Plugin
	probes     map[string]*runningProbe // name -> running probe
	mu         sync.Mutex
	internal   bool                                                              // true = goroutine mode (ip-setup allowed)
	dispatchFn func(ctx context.Context, command string) (string, string, error) // injectable for tests
	ipMgr      ipManager                                                         // injectable for tests
}

// runningProbe tracks a running probe goroutine.
type runningProbe struct {
	config   ProbeConfig
	cancel   context.CancelFunc
	done     chan struct{}
	fsmState atomic.Int32 // current FSM state, updated by probe goroutine
}

func newProbeManager(p *sdk.Plugin, internal bool) *probeManager {
	mgr := &probeManager{
		plugin:   p,
		probes:   make(map[string]*runningProbe),
		internal: internal,
		ipMgr:    realIPManager{},
	}
	mgr.dispatchFn = func(ctx context.Context, command string) (string, string, error) {
		return p.DispatchCommand(ctx, command)
	}
	return mgr
}

// validateConfig checks that the configuration is valid for the current plugin mode.
func (m *probeManager) validateConfig(configs []ProbeConfig) error {
	if m.internal {
		return nil
	}
	for i := range configs {
		if len(configs[i].IPs) > 0 || configs[i].IPInterface != "" {
			return fmt.Errorf("probe %q: ip-setup requires internal plugin mode (ip management needs in-process netlink access)", configs[i].Name)
		}
	}
	return nil
}

// applyConfig starts/stops probes based on new configuration.
func (m *probeManager) applyConfig(configs []ProbeConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	newConfigs := make(map[string]*ProbeConfig, len(configs))
	for i := range configs {
		newConfigs[configs[i].Name] = &configs[i]
	}

	// Stop probes that are no longer in config or changed.
	// INVARIANT: runProbe never acquires m.mu, so blocking on <-rp.done
	// while holding m.mu is safe (#4).
	for name, rp := range m.probes {
		newCfg, exists := newConfigs[name]
		if !exists || !newCfg.equal(rp.config) {
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
		rp := &runningProbe{config: *cfg, cancel: cancel, done: done}
		m.probes[name] = rp
		go m.runProbe(ctx, rp)
	}

	logger().Info("healthcheck config applied", "probes", len(m.probes))
}

// runProbe runs a single healthcheck probe loop.
// The runningProbe pointer is used to update the shared fsmState atomic.
func (m *probeManager) runProbe(ctx context.Context, rp *runningProbe) {
	defer close(rp.done)
	cfg := rp.config

	f := newFSM(cfg.Rise, cfg.Fall)

	updateState := func() { rp.fsmState.Store(int32(f.state)) }
	updateState()

	// IP management: add all IPs at startup (before first check),
	// but skip if probe starts disabled (#12).
	var ipt *ipTracker
	if len(cfg.IPs) > 0 && cfg.IPInterface != "" {
		ipt = newIPTracker(m.ipMgr, cfg.IPInterface, cfg.IPs)
		if !cfg.Disable {
			ipt.addAll()
		}
	}

	// If disabled at startup, enter DISABLED directly.
	if cfg.Disable {
		f.state = StateDisabled
		updateState()
		m.dispatchStateAction(ctx, cfg, f.state)
		logger().Info("probe started disabled", "name", cfg.Name)
	}

	for {
		interval := time.Duration(cfg.Interval) * time.Second
		if f.state == StateRising || f.state == StateFalling {
			interval = time.Duration(cfg.FastInterval) * time.Second
		}

		// Single-check mode: interval=0 means one check then dormant.
		if cfg.Interval == 0 && f.state != StateInit {
			f.state = StateEnd
			updateState()
			logger().Info("probe dormant (interval=0)", "name", cfg.Name)
			<-ctx.Done()
			m.handleExit(ctx, cfg, ipt)
			return
		}

		// DISABLED: sleep on interval, don't execute check (#2: check before general sleep).
		if f.state == StateDisabled {
			select {
			case <-ctx.Done():
				m.handleExit(ctx, cfg, ipt)
				return
			case <-time.After(interval):
			}
			continue
		}

		// Wait for interval or shutdown (skip on first iteration).
		if f.state != StateInit {
			select {
			case <-ctx.Done():
				m.handleExit(ctx, cfg, ipt)
				return
			case <-time.After(interval):
			}
		}

		// Run check.
		success := runProbeCommand(ctx, cfg.Command, cfg.Timeout)

		// FSM transition.
		prevState := f.state
		f.step(success)
		stateChanged := f.state != prevState
		if stateChanged {
			updateState()
		}

		// Dispatch watchdog action on state change (or always if debounce=false).
		if stateChanged || !cfg.Debounce {
			m.dispatchStateAction(ctx, cfg, f.state)
		}

		// IP management on state change.
		if stateChanged && ipt != nil {
			m.handleIPTransition(ipt, cfg, f.state)
		}

		// Hooks on state change (not on count increments like RISING->RISING).
		if stateChanged {
			runHooks(cfg, f.state)
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

// handleExit handles probe shutdown: withdraw routes, remove all IPs.
func (m *probeManager) handleExit(_ context.Context, cfg ProbeConfig, ipt *ipTracker) {
	exitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m.dispatchStateAction(exitCtx, cfg, StateExit)
	if ipt != nil {
		ipt.removeAll()
	}
	logger().Info("probe exited", "name", cfg.Name)
}

// handleIPTransition manages IP addresses on state changes.
func (m *probeManager) handleIPTransition(ipt *ipTracker, cfg ProbeConfig, state State) {
	switch state {
	case StateUp:
		if cfg.IPDynamic {
			ipt.addAll()
		}
	case StateDown, StateDisabled:
		if cfg.IPDynamic {
			ipt.removeAll()
		}
	case StateInit, StateRising, StateFalling, StateExit, StateEnd:
		// No IP action for these states (EXIT handled in handleExit).
	}
}

// handleCommand dispatches healthcheck CLI commands.
func (m *probeManager) handleCommand(command string, args []string) (string, string, error) {
	switch command {
	case "healthcheck show":
		return m.handleShow(args)
	case "healthcheck reset":
		return m.handleReset(args)
	}
	return statusError, "", fmt.Errorf("unknown healthcheck command: %s", command)
}

// handleShow returns probe status as JSON.
func (m *probeManager) handleShow(args []string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(args) > 0 {
		// Single probe detail with actual FSM state (#3).
		name := args[0]
		rp, exists := m.probes[name]
		if !exists {
			return statusError, "", fmt.Errorf("probe %q not found", name)
		}
		detail := struct {
			Name           string `json:"name"`
			Group          string `json:"group"`
			State          string `json:"state"`
			Command        string `json:"command"`
			Interval       uint32 `json:"interval"`
			Rise           uint32 `json:"rise"`
			Fall           uint32 `json:"fall"`
			UpMetric       uint32 `json:"up-metric"`
			DownMetric     uint32 `json:"down-metric"`
			DisabledMetric uint32 `json:"disabled-metric"`
		}{
			Name:           rp.config.Name,
			Group:          rp.config.Group,
			State:          stateName(State(rp.fsmState.Load())),
			Command:        rp.config.Command,
			Interval:       rp.config.Interval,
			Rise:           rp.config.Rise,
			Fall:           rp.config.Fall,
			UpMetric:       rp.config.UpMetric,
			DownMetric:     rp.config.DownMetric,
			DisabledMetric: rp.config.DisabledMetric,
		}
		data, err := json.Marshal(detail)
		if err != nil {
			return statusError, "", fmt.Errorf("marshal probe detail: %w", err)
		}
		return statusDone, string(data), nil
	}

	// All probes summary.
	type probeInfo struct {
		Name  string `json:"name"`
		Group string `json:"group"`
		State string `json:"state"`
	}
	probes := make([]probeInfo, 0, len(m.probes))
	for name, rp := range m.probes {
		probes = append(probes, probeInfo{Name: name, Group: rp.config.Group, State: stateName(State(rp.fsmState.Load()))})
	}
	data, err := json.Marshal(probes)
	if err != nil {
		return statusError, "", fmt.Errorf("marshal probes: %w", err)
	}
	return statusDone, string(data), nil
}

// handleReset withdraws the current route and resets the probe FSM to INIT.
// Holds the lock for the entire operation to prevent TOCTOU with concurrent applyConfig (#10).
func (m *probeManager) handleReset(args []string) (string, string, error) {
	if len(args) < 1 {
		return statusError, "", fmt.Errorf("missing probe name")
	}
	name := args[0]

	m.mu.Lock()
	defer m.mu.Unlock()

	rp, exists := m.probes[name]
	if !exists {
		return statusError, "", fmt.Errorf("probe %q not found", name)
	}

	if rp.config.Disable {
		return statusError, "", fmt.Errorf("probe %q is DISABLED (use 'ze config set ... disable false' to re-enable)", name)
	}

	// Cancel and wait for probe goroutine to exit.
	// Safe to block here: runProbe never acquires m.mu (#4 invariant).
	rp.cancel()
	<-rp.done

	// Restart from INIT.
	ctx, cancel := context.WithCancel(context.Background())
	newRP := &runningProbe{config: rp.config, cancel: cancel, done: make(chan struct{})}
	m.probes[name] = newRP
	go m.runProbe(ctx, newRP)

	data, _ := json.Marshal(map[string]string{"probe": name, "action": "reset"})
	return statusDone, string(data), nil
}

// dispatchCommand sends a command to the watchdog plugin via dispatchFn.
func (m *probeManager) dispatchCommand(ctx context.Context, probeName, command string) {
	status, _, err := m.dispatchFn(ctx, command)
	if err != nil {
		logger().Warn("dispatch failed", "probe", probeName, "command", command, "error", err)
		return
	}
	if status != statusDone {
		logger().Warn("dispatch unexpected status", "probe", probeName, "command", command, "status", status)
	}
}
