// Design: docs/architecture/plugin/component-boundaries.md -- PluginManager implementation
// Design: docs/architecture/plugin-manager-wiring.md -- two-phase startup
// Related: ../server/startup.go — Server calls SpawnMore/GetProcessManager during handshake
// Related: ../acceptor.go — NewHubAcceptor, the shared TLS acceptor lifecycle ensureAcceptor uses

package plugin

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	parent "github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

var (
	errAlreadyStarted = errors.New("already started")
	errNotStarted     = errors.New("not started")
	// errDeclaredAlreadyStarted is a Ze defect, never an operator's: boot
	// calls StartDeclared once.
	errDeclaredAlreadyStarted = errors.New("the declared external plugins were already started once")
)

// logger is the plugin manager subsystem logger (lazy initialization).
var logger = slogutil.LazyLogger("plugin.manager")

// Manager implements ze.PluginManager.
// It owns plugin process lifecycle: registration, spawning, and shutdown.
// The 5-stage protocol handshake stays in pluginserver.Server (Phase 2).
//
// Two-phase startup:
//   - Before either, at boot (StartDeclared): fork the external plugins the
//     config declares, so each can send the Stage 1 declaration that carries
//     its YANG module before the full config is parsed
//   - Phase 1 (StartAll): spawn processes — no Server needed
//   - Phase 2 (Server.StartWithContext): handshake with spawned processes
//
// MUST call SetHubConfig before StartAll if external plugins exist.
type Manager struct {
	mu            sync.RWMutex
	plugins       map[string]*pluginState
	started       bool
	startupRunner func(context.Context) error
	starting      *managerStartup

	// Hub config for TLS acceptor (external plugins).
	hubConfig *parent.HubConfig

	// The certificate authority the TLS acceptor takes its leaf from, and whose
	// root every plugin process validates that leaf against. Injected because
	// internal/component/pki already reaches internal/component/plugin, so this
	// package cannot import it. nil means no acceptor can be built, and
	// ensureAcceptor says so rather than self-signing.
	hubAuthority parent.Authority

	// Single shared ProcessManager — every spawn phase appends to the same
	// pm.processes map so AllProcesses returns every running plugin regardless
	// of which phase started it. Created on the first spawnProcesses call.
	procManager *process.ProcessManager

	// admissionMu protects terminal admission and every procManager publication.
	// It is never held across process Start, connect-back, Stop or a handshake.
	admissionMu     sync.Mutex
	admissionClosed bool

	// TLS acceptor for external plugin connect-back (shared across phases).
	acceptor *pluginipc.PluginAcceptor

	// declared holds the outcome of each external plugin StartDeclared ran
	// before the full config was parsed, by plugin name: nil while its process
	// is held with its Stage 1 declaration unanswered, and the reason when it
	// is not running. SpawnMore takes an entry once, so a later start of the
	// same name (a reload, a restart) forks as it always did.
	declared map[string]error

	// Context for process management.
	ctx    context.Context
	cancel context.CancelFunc

	// Stored references.
	eventBus        ze.EventBus
	config          ze.ConfigProvider
	metricsRegistry metrics.Registry
}

// pluginState tracks a single plugin's registration and lifecycle.
type pluginState struct {
	config  ze.PluginConfig
	running bool
}

// NewManager creates a new Manager.
func NewManager() *Manager {
	return &Manager{
		plugins: make(map[string]*pluginState),
	}
}

// SetHubConfig sets the TLS transport config for external plugins.
// Must be called before StartAll if external plugins are configured.
func (m *Manager) SetHubConfig(cfg *parent.HubConfig) {
	m.hubConfig = cfg
}

// SetHubAuthority sets the certificate authority the TLS acceptor takes its
// leaf from. Must be called before StartAll whenever an acceptor can be needed,
// which is whenever external plugins or an explicit hub server block are
// configured. Without it ensureAcceptor fails rather than serving a certificate
// no peer can validate.
func (m *Manager) SetHubAuthority(ca parent.Authority) {
	m.hubAuthority = ca
}

// SetMetricsRegistry stores the registry and forwards it to an existing
// ProcessManager. The registry can become available after processes start.
func (m *Manager) SetMetricsRegistry(reg metrics.Registry) {
	m.mu.Lock()
	m.metricsRegistry = reg
	pm := m.procManager
	m.mu.Unlock()
	if pm != nil {
		pm.SetMetricsRegistry(reg)
	}
}

// Register adds a plugin for startup. Returns error if the name
// is already registered or if StartAll has already been called.
func (m *Manager) Register(config ze.PluginConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("cannot register plugin %q: already started", config.Name)
	}

	if _, exists := m.plugins[config.Name]; exists {
		return fmt.Errorf("plugin %q already registered", config.Name)
	}

	m.plugins[config.Name] = &pluginState{config: config}
	return nil
}

// StartAll prepares process management, then joins the bound startup runner.
// The runner discovers and spawns plugins through SpawnMore. The hub MUST bind
// that runner before StartAll so subsystems cannot start before plugin setup.
func (m *Manager) StartAll(ctx context.Context, eventBus ze.EventBus, config ze.ConfigProvider) error {
	return m.startAll(ctx, eventBus, config)
}

// SpawnMore spawns additional processes for auto-loaded plugins.
// Called by Server during Phase 2 when auto-load discovers new plugins.
func (m *Manager) SpawnMore(configs []parent.PluginConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.processAdmissionError(); err != nil {
		return err
	}

	if !m.started {
		return errNotStarted
	}
	if len(configs) == 0 {
		return nil
	}

	// A plugin StartDeclared already ran is never forked a second time: its
	// held process is the one the phase handshakes with, and a plugin that
	// did not start then is refused with the reason kept from that attempt.
	configs, err := m.takeDeclared(configs)
	if err != nil {
		return err
	}
	if len(configs) == 0 {
		return nil
	}

	return m.spawnProcesses(configs)
}

// StartDeclared forks the external plugins the config declares, before the
// full config can be parsed and so before StartAll: running a declared plugin
// is what lets it send the Stage 1 declaration that carries its YANG module.
// It answers, by plugin name, the error of each plugin that did not start; a
// plugin absent from the answer is running, connected and held.
//
// Each plugin it ran is recorded, and the SpawnMore that later names it takes
// the held process and forks nothing, or answers the kept failure. The caller
// MUST read each held plugin's Stage 1 request with
// PluginConn.HoldStartupRequest and answer none: the startup phase answers it.
// A held plugin the caller gives up on goes through FailDeclared, and one the
// parsed config no longer names through DropDeclaredExcept.
//
// environment holds the settings of the config's `environment` block, which
// the daemon applies to itself only once the full parse accepted the config:
// each plugin forked here is given them for its own environment
// (ProcessManager.StartEach), and the daemon's environment is not written.
//
// MUST be called at most once, after SetHubConfig and SetHubAuthority and
// before StartAll. The processes live until StopAll, under ctx. An error
// (no listener for the plugins to connect back to) means nothing was forked
// and nothing recorded, so the startup phase reports it as it always did.
func (m *Manager) StartDeclared(ctx context.Context, configs []parent.PluginConfig, environment []string) (map[string]error, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.processAdmissionError(); err != nil {
		return nil, err
	}

	if m.started {
		return nil, errAlreadyStarted
	}
	if m.declared != nil {
		return nil, errDeclaredAlreadyStarted
	}
	if len(configs) == 0 {
		return map[string]error{}, nil
	}
	if err := m.ensureAcceptor(configs); err != nil {
		return nil, err
	}
	pm, _, err := m.prepareProcessManager(nil)
	if err != nil {
		return nil, err
	}

	failures := pm.StartEach(ctx, configs, environment)
	m.declared = make(map[string]error, len(configs))
	for _, cfg := range configs {
		failure := failures[cfg.Name]
		m.declared[cfg.Name] = failure
		if failure != nil {
			continue
		}
		m.plugins[cfg.Name] = &pluginState{
			config:  ze.PluginConfig{Name: cfg.Name, Internal: cfg.RunsInDaemon()},
			running: true,
		}
	}
	logger().Debug("declared external plugins started", "count", len(configs), "failed", len(failures))
	return failures, nil
}

// FailDeclared stops a plugin StartDeclared holds and keeps cause as the
// reason it is not running, so the SpawnMore that names it answers cause and
// forks nothing. The caller uses it for a held plugin that sent no Stage 1
// declaration within its bound. cause MUST say what is wrong, why, and the
// command that fixes it: it is the text the operator reads. A name that is
// not held is left alone.
func (m *Manager) FailDeclared(ctx context.Context, name string, cause error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	outcome, recorded := m.declared[name]
	if !recorded {
		return
	}
	if outcome != nil {
		return
	}
	m.declared[name] = cause
	m.stopHeldLocked(ctx, name)
}

// DropDeclaredExcept stops every plugin StartDeclared ran that keep does not
// name, and forgets its outcome. The caller uses it once the full config is
// parsed: a config recovered from history can declare other plugins than the
// text the first pass read, and a held plugin nothing will start again would
// wait on its Stage 1 answer for the life of the daemon.
func (m *Manager) DropDeclaredExcept(ctx context.Context, keep []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, outcome := range m.declared {
		if slices.Contains(keep, name) {
			continue
		}
		delete(m.declared, name)
		if outcome == nil {
			m.stopHeldLocked(ctx, name)
		}
	}
}

// takeDeclared answers the configs SpawnMore still has to fork: every config
// but the ones StartDeclared already ran, each of which is taken from the
// record here, once. When one of them did not start, the phase cannot run, so
// the held plugins of this same call are stopped (nothing would answer them)
// and the kept reasons are the error. Every plugin the refused phase leaves
// down is then recorded down, so ze doctor names it (recordDeclaredDownLocked).
// Must be called with m.mu held.
func (m *Manager) takeDeclared(configs []parent.PluginConfig) ([]parent.PluginConfig, error) {
	if len(m.declared) == 0 {
		return configs, nil
	}

	remaining := make([]parent.PluginConfig, 0, len(configs))
	var held []string
	var failed []string
	var failures []error
	for _, cfg := range configs {
		outcome, recorded := m.declared[cfg.Name]
		if !recorded {
			remaining = append(remaining, cfg)
			continue
		}
		delete(m.declared, cfg.Name)
		if outcome != nil {
			failed = append(failed, cfg.Name)
			failures = append(failures, outcome)
			continue
		}
		held = append(held, cfg.Name)
	}
	if len(failures) == 0 {
		return remaining, nil
	}
	for _, name := range held {
		m.stopHeldLocked(m.ctx, name)
	}
	// Recorded after the stops, because a stop removes the process and a
	// removal clears the record (ProcessManager.RemoveProcess).
	for i, name := range failed {
		m.recordDeclaredDownLocked(name, failures[i].Error())
	}
	for _, name := range held {
		m.recordDeclaredDownLocked(name, declaredPhaseRefusedDetail)
	}
	return nil, errors.Join(failures...)
}

// declaredPhaseRefusedDetail is why a held plugin that did declare is down:
// another plugin of its startup phase did not, and one failure stops the phase.
const declaredPhaseRefusedDetail = "its startup phase was refused because another external plugin " +
	"sent no declaration before the config was parsed; the log names that plugin"

// recordDeclaredDownLocked records that the declared plugin name is not
// running and why, as the startup phase records a handshake that failed
// (process.StopHandshakeFailed). It is the only record of a plugin that was
// refused before its phase could run: the doctor-plugin-stopped check reads
// it and names the plugin, detail and the command that starts it again.
// Must be called with m.mu held.
func (m *Manager) recordDeclaredDownLocked(name, detail string) {
	if m.procManager == nil {
		// StartDeclared creates the process manager before it writes the
		// record this plugin was taken from, and StopAll clears both, so only
		// a ze defect reaches here. It is said rather than skipped: with no
		// process manager there is no record for doctor to read.
		logger().Error("a declared external plugin is not running and ze holds no process manager to record it in, "+
			"so ze doctor does not name it. This is a Ze defect: report it",
			"plugin", name, "detail", detail, "code", diagnostic.CodeDoctorInternalDefect)
		return
	}
	m.procManager.RecordStopped(name, process.PluginStop{Cause: process.StopHandshakeFailed, Detail: detail})
}

// stopHeldLocked stops the held process of name, waits for its goroutines
// within the process's own stop bound, and removes it from the process
// manager. A wait that expires is raised as doctor-plugin-stop-unfinished
// (process.RecordStopUnfinished) before the removal, which is the last point
// the manager knows the process. Must be called with m.mu held.
func (m *Manager) stopHeldLocked(ctx context.Context, name string) {
	if ps, ok := m.plugins[name]; ok {
		ps.running = false
	}
	if m.procManager == nil {
		return
	}
	proc := m.procManager.GetProcess(name)
	if proc == nil {
		return
	}
	proc.Stop()
	bound := proc.StopJoinBound()
	waitCtx, cancel := context.WithTimeout(ctx, bound)
	defer cancel()
	if err := proc.Wait(waitCtx); err != nil {
		process.RecordStopUnfinished(proc, bound, err)
	}
	m.procManager.RemoveProcess(name)
}

// spawnProcesses appends configs to the shared ProcessManager and starts them.
// On the first call it creates the pm; subsequent calls reuse it via StartMore
// so AllProcesses returns every plugin from every phase. Must be called with
// m.mu held.
func (m *Manager) spawnProcesses(configs []parent.PluginConfig) error {
	// Set up TLS acceptor for external plugins (shared across all spawn phases).
	if err := m.ensureAcceptor(configs); err != nil {
		return err
	}
	pm, first, err := m.prepareProcessManager(configs)
	if err != nil {
		return err
	}

	if first {
		if err := pm.StartWithContext(m.ctx); err != nil {
			return fmt.Errorf("start processes: %w", err)
		}
	} else {
		if err := pm.StartMore(configs); err != nil {
			return fmt.Errorf("start more processes: %w", err)
		}
	}

	// Mark spawned plugins as running. Auto-loaded plugins may not have been
	// pre-registered via Register() — add them to the map so Plugin()/Plugins()
	// queries return them.
	for _, cfg := range configs {
		if ps, ok := m.plugins[cfg.Name]; ok {
			ps.running = true
		} else {
			m.plugins[cfg.Name] = &pluginState{
				config:  ze.PluginConfig{Name: cfg.Name, Internal: cfg.RunsInDaemon()},
				running: true,
			}
		}
	}

	logger().Debug("processes spawned", "count", len(configs))
	return nil
}

// ensureAcceptor creates a TLS acceptor if external plugins exist and no acceptor yet.
// Must be called with m.mu held.
func (m *Manager) ensureAcceptor(configs []parent.PluginConfig) error {
	if m.acceptor != nil {
		return nil // Already created
	}

	hasForked := false
	for _, cfg := range configs {
		if cfg.RunsForked() {
			hasForked = true
			break
		}
	}
	// An explicitly configured hub server block always listens: out-of-process
	// clients (plugin CLI, the .ci engine-step runner) authenticate with the
	// configured secret without any external plugin being declared. Only the
	// AUTO-generated hub (random secret nobody outside this process can know)
	// stays gated on forked plugins existing.
	hasExplicitHub := m.hubConfig != nil && len(m.hubConfig.Servers) > 0
	if !hasForked && !hasExplicitHub {
		return nil
	}

	// Creation (auto-generated server block, issued leaf, listener, per-client
	// secret lookup, accept loop) is shared with the subsystem path -- see
	// parent.NewHubAcceptor. Only the policy above is engine-local.
	// clock.RealClock{} is the option written out at the call site: it decides
	// when the served leaf is reissued, and a daemon runs on wall time.
	acceptor, err := parent.NewHubAcceptor(m.hubConfig, m.hubAuthority, clock.RealClock{})
	if err != nil {
		return fmt.Errorf("plugin TLS acceptor: %w", err)
	}
	m.acceptor = acceptor

	return nil
}

// GetProcessManager returns the shared ProcessManager that holds every plugin
// process from every spawn phase.
// Returns nil if no processes have been spawned.
// Returns any to satisfy plugin.ProcessSpawner interface (Server type-asserts).
// Safe for concurrent use, including while SpawnMore waits for connect-back.
// A terminal seal keeps this owner stable until its ordered cleanup completes.
func (m *Manager) GetProcessManager() any {
	m.admissionMu.Lock()
	defer m.admissionMu.Unlock()

	if m.procManager == nil {
		return nil
	}
	return m.procManager
}

// StopAll cancels and joins startup, stops spawned processes and cleans up.
// A startup runner MUST finish its canceled call before StopAll can return.
func (m *Manager) StopAll(_ context.Context) error {
	m.mu.Lock()
	owner := m.starting
	m.stopAllLocked()
	m.mu.Unlock()
	if owner != nil {
		<-owner.done
	}
	return nil
}

// Plugin looks up a plugin by name.
func (m *Manager) Plugin(name string) (ze.PluginProcess, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ps, ok := m.plugins[name]
	if !ok {
		return ze.PluginProcess{}, false
	}

	return ze.PluginProcess{
		Name:    ps.config.Name,
		Running: ps.running,
	}, true
}

// Plugins returns all registered plugins.
func (m *Manager) Plugins() []ze.PluginProcess {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ze.PluginProcess, 0, len(m.plugins))
	for _, ps := range m.plugins {
		result = append(result, ze.PluginProcess{
			Name:    ps.config.Name,
			Running: ps.running,
		})
	}
	return result
}
