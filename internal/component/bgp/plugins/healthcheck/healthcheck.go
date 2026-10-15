// Design: docs/architecture/bgp/healthcheck-plugin.md -- healthcheck plugin design
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
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const configRootBGP = "bgp"

// watchdogMetricMED is the metric name the watchdog announce command takes.
// It is a CLI argument, so a misspelling reaches the watchdog as an unknown
// metric rather than as an error here.
const watchdogMetricMED = "med"

var errMissingProbeName = errors.New("missing probe name")

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

// commandDecls names the commands this plugin serves and states what each
// answer holds, so the engine publishes the operators a command supports and
// refuses the ones it cannot before the command is dispatched
// (pkg/plugin/rpc/types.go, CommandDecl).
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{
			Name:      "show bgp healthcheck",
			ShortHelp: "Show healthcheck probe status",
			// handleShow answers rows in both branches, so one declaration
			// describes the command whichever argument it takes. The shape is
			// "map" and not "tab" because the two branches carry DIFFERENT row
			// fields on purpose: three for the probe list and ten for one named
			// probe (TestHealthcheckNamedProbeAnswersRows pins both). One column
			// order cannot be read against both, and widening the list branch to
			// ten fields would change the answer every operator reads today.
			Shape: "map",
		},
		{
			Name:      "clear bgp healthcheck",
			ShortHelp: "Reset healthcheck probe to INIT",
			// handleReset answers a report of what it did rather than a data
			// set, and it is outside the population this spec measured, so it
			// keeps the derived-at-apply-time behavior every undeclared command
			// has.
		},
	}
}

// runHealthcheckPlugin is the in-process entry point for the healthcheck plugin.
func runHealthcheckPlugin(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-healthcheck", conn)
	defer func() { _ = p.Close() }()

	mgr := newProbeManager(p, true)

	// Stage 2 configure and the reload verify, apply and rollback: one parsed
	// probe set, installed by applyConfig (config_tx.go).
	newHealthcheckConfigTx(mgr).Register(p)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return mgr.handleCommand(command, args)
	})

	// Probes dispatch "request bgp watchdog announce/withdraw" -- a command owned
	// by the bgp-watchdog plugin -- so they may only run once the engine has
	// finished every startup phase and frozen the dispatcher command registry
	// (ai/rules/plugins.md, OnStarted vs OnAllPluginsReady). applyConfig
	// still starts the goroutines at stage 2; markReady is what lets them act.
	p.OnAllPluginsReady(func() error {
		mgr.markReady()
		return nil
	})

	logger().Info("healthcheck plugin starting")
	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, &sdk.Registration{
		WantsConfig: []string{configRootBGP},
		Commands:    commandDecls(),
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
	internal   bool                                                                                                   // true = goroutine mode (ip-setup allowed)
	dispatchFn func(ctx context.Context, command string, args []string, peer string) (string, json.RawMessage, error) // injectable for tests
	ipMgr      ipManager                                                                                              // injectable for tests

	// ready is closed once the plugin's 5-stage startup handshake has completed
	// (OnAllPluginsReady). Probe loops block on it before their first dispatch so
	// they cannot write a command frame into the stage-5 stream -- see waitReady.
	ready     chan struct{}
	readyOnce sync.Once

	// exiting holds the probes an apply or a reset canceled whose done has
	// not closed yet, so a probe started later that shares one's name or group
	// waits for its exit (startProbe). Guarded by mu. Bounded by the probes
	// canceled within one exit bound (runProbe): retire drops every entry
	// whose done has closed.
	exiting []*runningProbe
}

// runningProbe tracks a running probe goroutine.
type runningProbe struct {
	config   ProbeConfig
	cancel   context.CancelFunc
	done     chan struct{}
	fsmState atomic.Int32 // current FSM state, updated by probe goroutine

	// predecessors are the done channels of the canceled probes sharing this
	// probe's name or group when it started. runProbe waits on every one
	// before its first dispatch, so their exit withdraw and IP removal never
	// land after this probe's announce.
	predecessors []<-chan struct{}
}

// exited reports whether the probe goroutine has returned.
func (rp *runningProbe) exited() bool {
	select {
	case <-rp.done:
		return true
	default:
		return false
	}
}

func newProbeManager(p *sdk.Plugin, internal bool) *probeManager {
	mgr := &probeManager{
		plugin:   p,
		probes:   make(map[string]*runningProbe),
		internal: internal,
		ipMgr:    realIPManager{},
		ready:    make(chan struct{}),
	}
	mgr.dispatchFn = func(ctx context.Context, command string, args []string, peer string) (string, json.RawMessage, error) {
		return p.DispatchCommandArgs(ctx, command, args, peer)
	}
	return mgr
}

// markReady releases the probe loops. Idempotent: the engine sends the
// post-startup callback once per process, but a defensive second call (or a
// test calling it directly) must not panic on a double close.
func (m *probeManager) markReady() {
	m.readyOnce.Do(func() { close(m.ready) })
}

// waitReady blocks until the plugin's startup handshake has completed, or the
// probe is canceled. It reports false when the probe should stop.
//
// A probe MUST NOT dispatch before this returns true. applyConfig runs from
// OnConfigure -- stage 2 of the 5-stage startup protocol -- so a probe goroutine
// started there is live while stages 3, 4 and 5 are still on the wire. With
// interval 1 and rise 1 a probe reaches UP and dispatches inside that window;
// the dispatch frame then lands in the stream the engine is reading for the
// stage-5 `ready` RPC, which fails startup outright:
//
//	dispatch failed ... error="rpc error: expected ready, got ze-plugin-engine:dispatch-command-args"
//	plugin startup failed ... plugin=bgp-healthcheck stage=Ready
//
// after which every later dispatch gets "mux conn read error: EOF" and the
// plugin is dead for the process lifetime. Only load makes the window wide
// enough to hit, which is why it surfaced under internal/le/test/stressrepro/actions.go and
// not in a quiet run. ai/rules/plugins.md states the rule this restores:
// a DispatchCommand aimed at another plugin's command (here bgp-watchdog's
// "request bgp watchdog announce") belongs after the dispatcher command
// registry is frozen, which is what OnAllPluginsReady signals.
func (m *probeManager) waitReady(ctx context.Context) bool {
	select {
	case <-m.ready:
		return true
	case <-ctx.Done():
		return false
	}
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

// applyConfig makes configs the running probe set. A probe whose config is
// unchanged keeps running; a removed or changed probe is canceled and a new
// or changed one is started.
//
// It does not wait for a canceled probe to exit. The exit dispatches
// "request bgp watchdog withdraw" to bgp-watchdog, which during a reload is
// another participant of the same config transaction, so waiting would hold
// this plugin's apply reply on a cross-plugin round trip of up to 5 s per
// probe. The withdraw lands on its own; the probe that replaces a canceled
// one waits for that exit instead (startProbe). Safe for concurrent use.
func (m *probeManager) applyConfig(configs []ProbeConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	newConfigs := make(map[string]*ProbeConfig, len(configs))
	for i := range configs {
		newConfigs[configs[i].Name] = &configs[i]
	}

	var stopped []*runningProbe
	for name, rp := range m.probes {
		newCfg, exists := newConfigs[name]
		if exists && newCfg.equal(rp.config) {
			continue
		}
		stopped = append(stopped, rp)
		delete(m.probes, name)
	}
	m.retire(stopped)

	for name, cfg := range newConfigs {
		if _, running := m.probes[name]; running {
			continue
		}
		m.startProbe(*cfg)
	}

	logger().Info("healthcheck config applied", "probes", len(m.probes))
}

// retire cancels the stopped probes and records them as exiting, after
// dropping the exiting probes whose goroutine has returned. MUST be called
// with m.mu held, and before the startProbe calls that replace them.
func (m *probeManager) retire(stopped []*runningProbe) {
	m.exiting = slices.DeleteFunc(m.exiting, (*runningProbe).exited)
	for _, rp := range stopped {
		rp.cancel()
		m.exiting = append(m.exiting, rp)
	}
}

// startProbe runs cfg in a new probe goroutine that first waits for every
// exiting probe sharing its name or its group. MUST be called with m.mu held.
func (m *probeManager) startProbe(cfg ProbeConfig) {
	var predecessors []<-chan struct{}
	for _, old := range m.exiting {
		if old.config.Name == cfg.Name {
			predecessors = append(predecessors, old.done)
			continue
		}
		if old.config.Group == cfg.Group {
			predecessors = append(predecessors, old.done)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	rp := &runningProbe{config: cfg, cancel: cancel, done: make(chan struct{}), predecessors: predecessors}
	m.probes[cfg.Name] = rp
	go m.runProbe(ctx, rp)
}

// runProbe runs a single healthcheck probe loop.
// The runningProbe pointer is used to update the shared fsmState atomic.
func (m *probeManager) runProbe(ctx context.Context, rp *runningProbe) {
	defer close(rp.done)
	cfg := rp.config

	// Wait, outside m.mu, for the canceled probes this one replaces: their
	// exit withdraw and IP removal must land before this probe's first
	// announce, or a late withdraw leaves the route down. The wait does not
	// select on ctx, so a probe canceled while it waits still closes done
	// only after its predecessors closed theirs, which keeps a reload and its
	// rollback in order. Its bound is a sum: one predecessor can take its
	// in-flight check (cfg.Timeout), its in-flight hooks (hookTimeout each)
	// and handleExit's 5 s dispatch, and a chain of generations adds one such
	// term per generation.
	for _, predecessor := range rp.predecessors {
		<-predecessor
	}
	// A probe canceled while it waited does nothing more. Without this check
	// waitReady could still pass it: once m.ready is closed its select picks
	// at random between m.ready and ctx.Done().
	if ctx.Err() != nil {
		return
	}

	// Hold every probe until the startup handshake is done: this goroutine was
	// started from OnConfigure (stage 2) and must not put a command frame on the
	// wire while stages 3-5 are still using it. See waitReady.
	if !m.waitReady(ctx) {
		return
	}

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
		m.dispatchCommand(ctx, cfg.Name, "request bgp watchdog announce", []string{cfg.Group, watchdogMetricMED, textbuf.StringInt(int64(cfg.UpMetric))})
	case StateDown:
		if cfg.WithdrawOnDown {
			m.dispatchCommand(ctx, cfg.Name, "request bgp watchdog withdraw", []string{cfg.Group})
		} else {
			m.dispatchCommand(ctx, cfg.Name, "request bgp watchdog announce", []string{cfg.Group, watchdogMetricMED, textbuf.StringInt(int64(cfg.DownMetric))})
		}
	case StateDisabled:
		if cfg.WithdrawOnDown {
			m.dispatchCommand(ctx, cfg.Name, "request bgp watchdog withdraw", []string{cfg.Group})
		} else {
			m.dispatchCommand(ctx, cfg.Name, "request bgp watchdog announce", []string{cfg.Group, watchdogMetricMED, textbuf.StringInt(int64(cfg.DisabledMetric))})
		}
	case StateExit:
		m.dispatchCommand(ctx, cfg.Name, "request bgp watchdog withdraw", []string{cfg.Group})
	case StateInit, StateRising, StateFalling, StateEnd:
		// No watchdog action for intermediate or terminal states.
	default:
		panic("BUG: invalid healthcheck state")
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
	default:
		panic("BUG: invalid healthcheck IP transition state")
	}
}

// handleCommand dispatches healthcheck CLI commands.
func (m *probeManager) handleCommand(command string, args []string) (string, any, error) {
	switch command {
	case "show bgp healthcheck":
		return m.handleShow(args)
	case "clear bgp healthcheck":
		return m.handleReset(args)
	}
	return statusError, "", fmt.Errorf("unknown healthcheck command: %s", command)
}

// handleShow returns probe status as JSON.
//
// Both branches answer a row set, so the command has one shape whatever its
// argument and a single answer-shape declaration can describe it. The rows carry
// different fields in each branch, which is what makes the shape "map" rather than
// "tab" (internal/component/command/pipe_catalog.go).
func (m *probeManager) handleShow(args []string) (string, any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(args) > 0 {
		// Single probe detail with actual FSM state (#3).
		name := args[0]
		rp, exists := m.probes[name]
		if !exists {
			return statusError, "", fmt.Errorf("probe %q not found", name)
		}
		type probeDetail struct {
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
		}
		detail := []probeDetail{{
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
		}}
		return statusDone, detail, nil
	}

	// All probes summary.
	type probeInfo struct {
		Name  string `json:"name"`
		Group string `json:"group"`
		State string `json:"state"`
	}
	// Ascending probe name, not the Go map iteration order, so the row operators
	// this command's declared shape publishes -- "first", "last", "display" --
	// select the same probe on every call
	// (internal/component/command/answer_shape.go, rowSet). The name is the map
	// key, so it is unique and the order is total with no tie left to break. An
	// operator reads a probe list by name, and applyConfig keys this map by the
	// same name the configuration gives (config.go, ProbeConfig.Name). This runs
	// on the command goroutine, never on a probe loop.
	names := slices.Sorted(maps.Keys(m.probes))
	probes := make([]probeInfo, 0, len(names))
	for _, name := range names {
		rp := m.probes[name]
		probes = append(probes, probeInfo{Name: name, Group: rp.config.Group, State: stateName(State(rp.fsmState.Load()))})
	}
	return statusDone, probes, nil
}

// handleReset withdraws the current route and restarts the probe from INIT.
// Under m.mu it cancels the probe and starts its replacement, which waits for
// the old probe's exit before its first dispatch (startProbe). It does not
// wait for that exit itself: the replacement may still be waiting on older
// predecessors, and holding m.mu across that wait would stall applyConfig.
// So the reply can return before the old probe's withdraw has run.
func (m *probeManager) handleReset(args []string) (string, any, error) {
	if len(args) < 1 {
		return statusError, "", errMissingProbeName
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

	m.retire([]*runningProbe{rp})
	m.startProbe(rp.config)

	return statusDone, map[string]string{"probe": name, "action": "reset"}, nil
}

// dispatchCommand sends a command to the watchdog plugin via dispatchFn.
func (m *probeManager) dispatchCommand(ctx context.Context, probeName, command string, args []string) {
	status, _, err := m.dispatchFn(ctx, command, args, "")
	if err != nil {
		logger().Warn("dispatch failed", "probe", probeName, "command", command, "args", args, "error", err)
		return
	}
	if status != statusDone {
		logger().Warn("dispatch unexpected status", "probe", probeName, "command", command, "args", args, "status", status)
	}
}
