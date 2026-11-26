// Design: docs/architecture/api/process-protocol.md — plugin process lifecycle
// Detail: delivery.go — event delivery pipeline
// Detail: manager.go — multi-process coordination and respawn
// Detail: exitwait_linux.go — the Linux fork, and the unreaped exit wait the stop's group kill relies on
// Detail: exitwait_other.go — the fork and stop on every other platform
// Related: ../sysproc_linux.go — Linux-specific process isolation
// Related: ../sysproc_other.go — non-Linux process isolation
// Related: ../ipc/tls.go — PluginAcceptor used by startExternal (ExpectPlugin then Wait for TLS connect-back)
// Related: ../../../../pkg/plugin/sdk/sdk.go — plugin-side SDK that connects back via TLS

package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/syncutil"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var errNoRawConnectionAvailable = errors.New("no raw connection available")

// loopbackHost is where a forked plugin dials its hub when the hub listens on
// an unspecified address. The hub's leaf carries it as a SAN for that reason.
const loopbackHost = "127.0.0.1"

// logger is the plugin subsystem logger (lazy initialization).
// Controlled by ze.log.plugin environment variable.
var logger = slogutil.LazyLogger("plugin")

// stderrLogger is used for relaying plugin stderr to engine logs (lazy initialization).
// Tagged with subsystem=plugin.relay to distinguish from engine logs.
// Level controlled by ze.log.plugin.relay env var.
var stderrLogger = slogutil.LazyLogger("plugin.relay")

// Process represents a plugin subprocess (internal goroutine or external fork).
//
// Lifecycle: Start (or StartWithContext) -> Stop -> Wait.
// Stop signals the process to exit; Wait blocks until all goroutines finish.
// Callers MUST call Wait after Stop to avoid leaking goroutines.
type Process struct {
	config plugin.PluginConfig
	// registryName is config.CompiledIdentity(), computed once at
	// construction: the compiled registration identity this process runs
	// (empty for a foreign external program). State grants, filter references
	// and lifecycle lookups read it instead of the operator's endpoint alias.
	registryName string
	index        int // Plugin index for coordinator (0-based)
	cmd          *exec.Cmd

	stderr io.ReadCloser

	// Raw connection for IPC. Set during startup, consumed by InitConns().
	rawConn net.Conn

	// TLS acceptor for external plugin connect-back (set by SetAcceptor).
	acceptor *ipc.PluginAcceptor

	// configEnvironment holds the `environment` block settings this fork is
	// given, nil for a process forked after the daemon applied the block
	// (set by SetConfigEnvironment).
	configEnvironment []string

	// RPC connection for YANG RPC protocol.
	// Created by InitConns() from rawConn, or set directly by tests via SetConn.
	conn *ipc.PluginConn

	running atomic.Bool

	// Session state (per-process API connection state)
	// Note: ACK is controlled by serial prefix (#N), not per-process state
	syncEnabled   atomic.Bool // Whether to wait for wire transmission (default: false)
	cacheConsumer atomic.Bool // Whether plugin participates in cache consumer tracking

	// High-level encoding and format (bgp plugin encoding/format commands)
	encoding       atomic.Value // string: "json" or "text" (default: "json")
	format         atomic.Value // string: "hex", "base64", "parsed", "full" (default: "hex")
	formatCacheKey atomic.Value // string: precomputed "format+encoding" for event dispatch cache lookup
	envelope       atomic.Bool  // true: wrap delivered events in an EventEnvelope (default: bare payload)

	// Plugin registration protocol (5-stage startup)
	stage        atomic.Int32                              // Current stage (PluginStage)
	registration atomic.Pointer[plugin.PluginRegistration] // Published Stage 1 registration data
	capabilities *plugin.PluginCapabilities                // Stage 3 capability declarations
	stageCh      chan struct{}                             // Signals stage completion
	stageMu      sync.Mutex                                // Protects stage transitions

	// startupFail records the first handshake or runtime restoration failure
	// for this exact lifetime. See SetStartupError.
	startupFail atomic.Pointer[startupFailure]

	// exit records how the process ended (stopped.go, ExitStatus): the
	// runner's return code for an internal plugin, the reaped state for a fork.
	exit exitRecord
	// terminal holds cleanup reporting custody for this exact Process only.
	terminal terminalCleanup

	// metricHooks holds the metrics callbacks the ProcessManager gives this
	// process (wireMetrics), nil when metrics are disabled or the manager took
	// them back (clearProcessCallbacks, Respawn). It is one atomic pointer
	// because the manager swaps it while the process runs: a restart of a
	// running plugin clears it while Deliver, the delivery loop and SetStage
	// read it, each from its own goroutine under its own lock or none.
	metricHooks atomic.Pointer[processMetricHooks]

	// Direct transport bridge for internal plugins (nil for external).
	// After 5-stage startup, events and RPCs bypass socket I/O via function calls.
	bridge *rpc.DirectBridge

	// Long-lived event delivery goroutine (see rules/goroutine-lifecycle.md).
	// Events are enqueued via Deliver() into a bounded growable lossless FIFO
	// ring queue and processed in batches by deliveryLoop().
	// eventMu protects queue publication and close: Deliver holds RLock only to
	// snapshot eventQueue, stopEventChan holds Lock to mark eventClosed.
	eventQueue  *eventQueue
	eventClosed bool
	eventMu     sync.RWMutex

	// projectionErr is the first failed event application in this Process lifetime.
	// Only the delivery goroutine accesses it; barrier receipts publish the error.
	// A successful batch MUST NOT clear it: only a new Process starts clean.
	projectionErr error

	// doctor holds what the doctor reads of this lifetime (doctor_records.go).
	doctor doctorRecords

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	// engineDone closes when an internal plugin's RunEngine has RETURNED, which is
	// after it has released what it installed. It is separate from wg on purpose: wg
	// also holds the event delivery loop, and a delivery still draining says nothing
	// about whether the engine gave its resources back (WaitEngine).
	//
	// Written by startInternal and read by WaitEngine without a lock, and the ordering
	// is what makes that safe: startInternal sets it before Start returns, and a
	// Process only becomes reachable from ProcessManager.processes after its Start has
	// returned (startConfigs, manager.go), so no reader can see the field being
	// assigned. Nil means "no engine goroutine here", which is every external plugin.
	engineDone chan struct{}
	// externalDone closes only after the actual external monitor has reaped
	// the child and drained its stderr. It is separate from wg: a fresh
	// WaitGroupWait can select an expired context before its waiter runs.
	// Allocated only after StartChild succeeds, before the monitor starts;
	// nil means no fork/monitor was created. A failed TLS connect-back still
	// owns that monitor. Start returns before shutdown readers see the field,
	// and the one-attempt Process never replaces the channel.
	externalDone              chan struct{}
	mu                        sync.Mutex
	started                   bool                  // One Process owns one attempt and one cleanup lifetime.
	selectedOwner             *plugin.SelectedOwner // Captured once while this exact connection is live.
	selectedCancel            context.CancelFunc    // Cancels only selected work; engine exit is distinct from operator Stop.
	peerDeclarationsPending   atomic.Bool           // Exact replacement output waits for its runtime peer authority.
	postStartupStarted        atomic.Bool           // One callback belongs to one exact Process, across racing fan-outs.
	runtimeRestorationPending atomic.Bool           // A replacement cannot deliver post-startup before joined restoration.
	cleanupOnce               sync.Once             // ensures connection cleanup runs exactly once
	runtimeCleanupOnce        sync.Once             // ensures server runtime cleanup runs exactly once
	runtimeCleanupDone        chan struct{}
}

// NewProcess creates a new process with the given configuration.
func NewProcess(config plugin.PluginConfig) *Process {
	proc := &Process{
		config:             config,
		registryName:       config.CompiledIdentity(),
		capabilities:       &plugin.PluginCapabilities{},
		stageCh:            make(chan struct{}),
		runtimeCleanupDone: make(chan struct{}),
	}
	proc.registration.Store(&plugin.PluginRegistration{})
	return proc
}

// Stage returns the current plugin startup stage.
func (p *Process) Stage() plugin.PluginStage {
	return plugin.PluginStage(p.stage.Load())
}

// startupFailure boxes a handshake or runtime restoration cause. The box exists
// because atomic.Value panics when successive Stores carry different concrete
// types, and startup causes are arbitrary error implementations (*fmt.wrapError,
// *errors.errorString, syscall.Errno, ...). An atomic.Pointer to this struct
// stores any of them without that hazard.
type startupFailure struct{ err error }

// SetStartupError records why this lifetime's handshake or later runtime
// restoration failed. The engine reports handshake failure from proc.Stage(),
// which says WHERE the handshake stopped but not WHY; without this the cause
// (e.g. `iface: create dummy "zdiag0": operation not permitted`) was logged at
// Debug and dropped, and ze exited with only "plugin interface failed during
// startup at stage Config" -- an error with no evidence and no corrective
// action (ai/rules/cli.md).
//
// Only the FIRST cause is kept: it stopped handshake or restoration,
// and later errors are usually its consequences (a closed pipe, an aborted
// stage). A nil err is ignored so a caller need not branch.
//
// Safe for concurrent use.
func (p *Process) SetStartupError(err error) {
	if err == nil {
		return
	}
	p.startupFail.CompareAndSwap(nil, &startupFailure{err: err})
}

// StartupError returns the cause recorded by SetStartupError, or nil when the
// handshake and restoration have no reported cause. Callers MUST treat
// nil as "no cause available" and still report the failure -- the absence of a
// cause is not evidence of success (ai/rules/evidence.md).
//
// Safe for concurrent use.
func (p *Process) StartupError() error {
	if f := p.startupFail.Load(); f != nil {
		return f.err
	}
	return nil
}

// SetStage sets the current stage and notifies waiters.
// If a stage-change metrics callback is set (by ProcessManager metrics), it is
// called after the stage is stored and waiters are notified, outside the lock.
func (p *Process) SetStage(stage plugin.PluginStage) {
	p.stageMu.Lock()
	// Safe: PluginStage has only values 0-6 (StageInit..StageRunning).
	p.stage.Store(int32(stage)) //nolint:gosec // G115: bounded enum
	// Close and recreate channel to notify all waiters
	close(p.stageCh)
	p.stageCh = make(chan struct{})
	p.stageMu.Unlock()
	// Call metrics callback outside the lock to avoid holding stageMu
	// during Prometheus internal lock acquisition.
	p.noteStageChange(stage)
}

// WaitForStage waits for the process to reach or pass the given stage.
// Returns error on context cancellation or timeout.
func (p *Process) WaitForStage(ctx context.Context, stage plugin.PluginStage) error {
	for {
		if p.Stage() >= stage {
			return nil
		}
		p.stageMu.Lock()
		ch := p.stageCh
		p.stageMu.Unlock()

		select {
		case <-ch:
			// Stage changed, check again
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Registration returns the borrowed plugin registration data (Stage 1).
// It is safe to call concurrently with SetRegistration. Callers must not mutate
// the returned record while another goroutine may read it. A zero Process or an
// explicit SetRegistration(nil) has no record; NewProcess starts with an empty one.
func (p *Process) Registration() *plugin.PluginRegistration {
	return p.registration.Load()
}

// Capabilities returns the plugin capability declarations (Stage 3).
func (p *Process) Capabilities() *plugin.PluginCapabilities {
	return p.capabilities
}

// Running returns true if the process is running.
func (p *Process) Running() bool {
	return p.running.Load()
}

// Conn returns the plugin RPC connection under the mutex.
// Returns nil if the connection has been closed (e.g. by Stop() or monitor()).
// Callers must check for nil before use to avoid racing with shutdown.
func (p *Process) Conn() *ipc.PluginConn {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.conn
}

// SetConn sets the plugin RPC connection. Used by test code.
func (p *Process) SetConn(conn *ipc.PluginConn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conn = conn
}

// SetAcceptor sets the TLS acceptor for external plugin connect-back.
func (p *Process) SetAcceptor(a *ipc.PluginAcceptor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.acceptor = a
}

// SetConfigEnvironment gives the fork of this process the settings of the
// config's `environment` block, for a plugin forked before the daemon applied
// the block to its own environment: the declared external plugins, which run
// before the full parse (ProcessManager.StartEach). entries are "key=value"
// under the dotted env key (config.EnvironmentEntries).
//
// The fork merges them under the daemon's environment for this child alone
// (inheritedEnvironment); the daemon's own environment is never written. A
// process forked after the full parse is given none: the daemon then holds
// the block's settings itself, and the child inherits them. That includes the
// process a respawn builds (ProcessManager.Respawn), which is a new Process.
//
// MUST be called before StartWithContext. It has no effect on an internal
// plugin, which runs in the daemon's own environment.
func (p *Process) SetConfigEnvironment(entries []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.configEnvironment = entries
}

// inheritedEnvironment answers the part of a forked plugin's environment that
// comes from the daemon's, given as daemon (os.Environ): every variable of
// it, then each `environment` block setting the fork was given whose key
// daemon gives no value (env.WithDefaults), each ze key under the name a
// shell passes on (env.ShellSafe).
//
// The block's settings come second because the operator's own environment
// wins over the config, which is the priority config.ApplyEnvConfig keeps
// when it applies the block to the daemon. A plugin forked before that
// therefore reads what one forked after it reads.
func (p *Process) inheritedEnvironment(daemon []string) []string {
	return env.ShellSafe(env.WithDefaults(daemon, p.configEnvironment))
}

// InitConns creates PluginConn connections from the raw engine-side connections.
// If PluginConns already exist (set directly by tests), returns immediately.
// Must be called exactly once before any reads from the connections.
//
// InitConns creates a MuxPluginConn from the raw connection.
// If already initialized (conn set by test), returns immediately.
func (p *Process) InitConns() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.rawConn == nil {
		if p.conn != nil {
			return nil // already initialized (e.g., set by test)
		}
		return errNoRawConnectionAvailable
	}

	raw := p.rawConn
	p.rawConn = nil

	rpcConn := rpc.NewConn(raw, raw)
	mux := rpc.NewMuxConn(rpcConn)
	p.conn = ipc.NewMuxPluginConn(mux)
	return nil
}

// isSyncEnabled returns true if sync mode is enabled for this process.
// When enabled, announce/withdraw waits for wire transmission before ACK.
func (p *Process) isSyncEnabled() bool {
	return p.syncEnabled.Load()
}

// SetSync enables or disables sync mode for this process.
func (p *Process) SetSync(enabled bool) {
	p.syncEnabled.Store(enabled)
}

// IsCacheConsumer returns whether this plugin participates in cache consumer tracking.
// Cache consumers must forward or release each UPDATE they receive.
func (p *Process) IsCacheConsumer() bool {
	return p.cacheConsumer.Load()
}

// SetCacheConsumer marks whether this plugin participates in cache consumer tracking.
// When no explicit delivery-queue-max was configured, updates the live delivery
// queue ceiling to the role-appropriate default (256 for cache consumers, 65536 otherwise).
func (p *Process) SetCacheConsumer(enabled bool) {
	p.cacheConsumer.Store(enabled)
	if p.config.DeliveryQueueMax > 0 {
		return
	}
	p.eventMu.RLock()
	q := p.eventQueue
	p.eventMu.RUnlock()
	if q != nil {
		q.setMaxCapacity(p.effectiveDeliveryQueueMax())
	}
}

// effectiveDeliveryQueueMax returns the configured delivery queue ceiling, or
// the role-appropriate default (256 for CacheConsumer plugins, 65536 otherwise).
func (p *Process) effectiveDeliveryQueueMax() int {
	if p.config.DeliveryQueueMax > 0 {
		return p.config.DeliveryQueueMax
	}
	if p.IsCacheConsumer() {
		return plugin.DefaultCacheConsumerQueueMax
	}
	return plugin.DefaultDeliveryQueueMax
}

// HasStructuredHandler reports whether this process supports structured event delivery.
// True when the process has a DirectBridge with a registered structured handler.
func (p *Process) HasStructuredHandler() bool {
	return p.bridge != nil && p.bridge.HasStructuredHandler()
}

// Encoding returns the high-level encoding (json or text).
func (p *Process) Encoding() string {
	if v := p.encoding.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return plugin.EncodingJSON // Default
}

// SetEncoding sets the high-level encoding (json or text).
func (p *Process) SetEncoding(enc string) {
	p.encoding.Store(enc)
	p.recomputeFormatCacheKey()
}

// Format returns the wire format (hex, base64, parsed, full).
func (p *Process) Format() string {
	if v := p.format.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return plugin.FormatParsed // Default: historically FormatHex fell through to FormatParsed
}

// SetFormat sets the wire format (hex, base64, parsed, full, summary).
func (p *Process) SetFormat(format string) {
	p.format.Store(format)
	p.recomputeFormatCacheKey()
}

// Envelope reports whether this process opted into enveloped event delivery
// (SubscribeEventsInput.Envelope). Default false: bare-payload delivery, the
// shape every pre-existing subscriber relies on.
func (p *Process) Envelope() bool {
	return p.envelope.Load()
}

// SetEnvelope records the process-wide enveloped-delivery preference. Like
// SetFormat/SetEncoding this is per-process state, not per-subscription: the
// last subscribe block wins for the whole process.
func (p *Process) SetEnvelope(v bool) {
	p.envelope.Store(v)
}

// FormatCacheKey returns the precomputed "format+encoding" string for event dispatch
// cache lookup. Avoids per-event string concatenation on the hot path.
func (p *Process) FormatCacheKey() string {
	if v := p.formatCacheKey.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	// Fallback: compute on first call (before SetFormat/SetEncoding).
	key := p.Format() + "+" + p.Encoding()
	p.formatCacheKey.Store(key)
	return key
}

// recomputeFormatCacheKey updates the cached format+encoding key.
// Called by SetFormat and SetEncoding after storing the new value.
func (p *Process) recomputeFormatCacheKey() {
	p.formatCacheKey.Store(p.Format() + "+" + p.Encoding())
}

// Name returns the process name from config.
func (p *Process) Name() string {
	return p.config.Name
}

// RegistrationName returns the compiled registration identity this process
// runs, or empty string for a foreign external program. State grants, doctor
// snapshots, and compiled filter references key on this identity regardless of
// endpoint alias or transport.
func (p *Process) RegistrationName() string {
	return p.registryName
}

// Index returns the plugin index for coordinator tracking.
func (p *Process) Index() int {
	return p.index
}

// Config returns the plugin configuration.
func (p *Process) Config() plugin.PluginConfig {
	return p.config
}

// SetIndex sets the plugin index for coordinator tracking.
func (p *Process) SetIndex(i int) {
	p.index = i
}

// SetRegistration publishes the plugin registration data (Stage 1).
// It is safe to call concurrently with Registration. The caller must finish
// initializing reg before publication and must not mutate it while readers may
// use it. Passing nil clears the record.
func (p *Process) SetRegistration(reg *plugin.PluginRegistration) {
	p.registration.Store(reg)
}

// SetCapabilities sets the plugin capability declarations (Stage 3).
func (p *Process) SetCapabilities(caps *plugin.PluginCapabilities) {
	p.capabilities = caps
}

// Cmd returns the underlying exec.Cmd for external plugins (nil for internal).
// Protected by mu since startExternal() writes p.cmd under the same lock.
func (p *Process) Cmd() *exec.Cmd {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmd
}

// Bridge returns the direct transport bridge for internal plugins (nil for external).
func (p *Process) Bridge() *rpc.DirectBridge {
	return p.bridge
}

// SetRunning sets the running state of the process.
func (p *Process) SetRunning(running bool) {
	p.running.Store(running)
}

// EngineDone closes when an in-process plugin's engine has returned, whether it
// finished, panicked, or was stopped. It returns nil for a forked plugin, which
// has no engine goroutine in this process: that one's exit closes the connection
// instead, and the engine's read of it is what reports the exit.
//
// The two exits are one question for a caller that supervises a plugin, so a
// select on this channel is what makes an in-process engine's end reach the same
// path a fork's end reaches (Server.superviseProcess, ../server/failure_policy.go).
// A nil channel blocks forever in a select, which is the right answer for a fork.
func (p *Process) EngineDone() <-chan struct{} {
	return p.engineDone
}

// PluginEnded reports whether the plugin behind this process has gone: its
// in-process engine returned, its connection ended, or the connection was already
// cleared. A nil engineDone (a fork) never fires, and a fork's exit reaches the
// connection. Stop closes the connection too, so a caller asking what the plugin
// itself did MUST ask before it stops the process. Safe for concurrent use.
func (p *Process) PluginEnded() bool {
	select {
	case <-p.engineDone:
		return true
	default:
	}
	conn := p.Conn()
	return conn == nil || conn.Err() != nil
}

// Done closes when Stop cancels this process.
// It returns nil before Start initializes the process context.
func (p *Process) Done() <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ctx == nil {
		return nil
	}
	return p.ctx.Done()
}

// RunRuntimeCleanupOnce runs cleanup at most once for this process generation.
// It is safe for concurrent use.
func (p *Process) RunRuntimeCleanupOnce(cleanup func()) {
	p.runtimeCleanupOnce.Do(func() {
		defer close(p.runtimeCleanupDone)
		cleanup()
	})
}

// WaitRuntimeCleanup waits until server runtime cleanup completes, with no
// bound: a cleanup that never runs holds the caller forever. A caller that
// holds a lock others wait for uses WaitRuntimeCleanupContext instead.
// Caller MUST stop a running process before it calls WaitRuntimeCleanup.
func (p *Process) WaitRuntimeCleanup() {
	<-p.runtimeCleanupDone
}

// WaitRuntimeCleanupContext waits until server runtime cleanup completes, or
// until ctx ends, and then answers ctx's error: the cleanup has not run, and
// the caller decides what that means for its own action. The caller picks the
// bound: the server's restart uses its restartCleanupGrace, the daemon's stop
// budget, because the cleanup runs once the process's calls into the engine
// have returned, and the plugin's exit, which StopJoinBound bounds, does not
// end those calls. Caller MUST stop a running process before it calls
// WaitRuntimeCleanupContext. Safe for concurrent use.
func (p *Process) WaitRuntimeCleanupContext(ctx context.Context) error {
	// A cleanup that already ran answers nil even when ctx has ended too: a
	// select over two ready cases picks one at random.
	select {
	case <-p.runtimeCleanupDone:
		return nil
	default:
	}
	select {
	case <-p.runtimeCleanupDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CloseConnForTest closes and nils the RPC connection under the mutex.
func (p *Process) CloseConnForTest() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		p.conn.Close() //nolint:errcheck,gosec // best-effort shutdown
		p.conn = nil
	}
}

// ClearConnForTest nils the connection pointer without closing the underlying connection.
// Used in tests to simulate a process dying between verify and apply phases.
func (p *Process) ClearConnForTest() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conn = nil
}

// Start spawns the process.
func (p *Process) Start() error {
	return p.StartWithContext(context.Background())
}

// StartWithContext spawns the process with the given context.
// Compiled-in plugins run in-daemon via goroutine unless SeparateProcess is
// set, in which case Ze forks its own binary over TLS connect-back. Foreign
// external programs fork via plugin.Shell.
func (p *Process) StartWithContext(ctx context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return opererr.New("Plugin "+p.config.Name+" already started",
			"A stopped or failed process cannot reuse its delivery and cleanup lifetime", "ze request plugin restart "+p.config.Name)
	}
	p.started = true

	p.ctx, p.cancel = context.WithCancel(ctx)

	// Start long-lived event delivery goroutine (see rules/goroutine-lifecycle.md).
	p.startDeliveryLocked()

	var err error
	switch {
	case p.config.RunsInDaemon():
		err = p.startInternal()
	case p.config.RunsSeparateProcess():
		err = p.startSeparateProcess()
	default:
		err = p.startExternal()
	}
	p.mu.Unlock()
	if err != nil {
		// A failed attempt is spent too. Stop outside p.mu so connection
		// cleanup cannot deadlock while releasing its delivery worker.
		p.Stop()
	}
	return err
}

// startInternal starts an internal plugin as a goroutine with a single net.Pipe.
// Creates a MuxPluginConn for bidirectional YANG RPC protocol.
func (p *Process) startInternal() error {
	name := p.config.CompiledIdentity()

	runner := plugin.GetInternalPluginRunner(name)
	if runner == nil {
		return fmt.Errorf("unknown internal plugin: %s", name)
	}
	compiled := registry.Lookup(name)
	if compiled != nil {
		if err := p.AdmitTerminalCleanup(compiled.ExitConfirmsCleanup); err != nil {
			return err
		}
	}

	// Create single bidirectional pipe for IPC.
	engineSide, pluginSide := net.Pipe()
	p.stderr = nil // Internal plugins don't have stderr
	p.running.Store(true)

	// Store raw connection for InitConns (creates MuxConn + MuxPluginConns).
	p.rawConn = engineSide

	// Create direct transport bridge for post-startup hot path.
	// The bridge carries through BridgedConn so the SDK can discover it
	// via type assertion after the 5-stage startup completes.
	p.bridge = rpc.NewDirectBridge()

	// Wrap plugin-side connection with bridge reference.
	bridgedPluginSide := rpc.NewBridgedConn(pluginSide, p.bridge)
	if p.terminal.owner != nil {
		bridgedPluginSide = &terminalCleanupConn{Conn: bridgedPluginSide, bridge: p.bridge, cleanup: &p.terminal}
	}

	// Closed by the goroutine below once the engine has returned. Stop waits on it,
	// because the engine's LAST act is releasing what it installed.
	p.engineDone = make(chan struct{})

	// Start the plugin in a goroutine.
	p.wg.Go(func() {
		// Registered first, so it runs LAST: the signal means the engine and every
		// defer of its own have finished, release included.
		defer p.endSelectedEngine()
		cleanExit := false
		terminalStatus := "a panic or an exit without a returned status"
		defer func() { p.finishTerminalCleanup(cleanExit, terminalStatus) }()
		defer p.running.Store(false)
		// The panic boundary: a panic in the plugin's own code is logged,
		// counted and crash-recorded, and the goroutine then returns as a
		// plugin that ended does. It covers this goroutine only: a goroutine
		// the plugin starts carries its own recover or none.
		defer runnerPanics().run.Contain(func() taskguard.Context {
			return taskguard.Context{Unit: "internal plugin " + p.config.Name}
		}, nil)
		// Runs before the panic boundary above, and only stores a status when
		// the runner did not return one: the runner panicked.
		defer p.exit.record("a panic in the plugin; its crash record names the cause")
		defer func() {
			if err := bridgedPluginSide.Close(); err != nil {
				logger().Debug("close plugin side", "error", err)
			}
		}()

		// The exit status is logged as exit-code, because `code` on every Warn
		// line is the doctor code that names the fault (here the plugin ending
		// unasked, which applyFailurePolicy then reports on the bus).
		exitCode := runner(bridgedPluginSide)
		var status textbuf.Buffer
		status.Str("exit code ").Int(int64(exitCode))
		terminalStatus = status.String()
		cleanExit = exitCode == 0
		p.exit.record(terminalStatus)
		if exitCode != 0 {
			logger().Warn("internal plugin exited with a non-zero status; read its log lines above for the cause",
				"plugin", p.config.Name, "exit-code", exitCode, "code", diagnostic.CodeDoctorPluginCrashed)
		}
	})

	return nil
}

// startFailure says why the fork produced no process.
//
// The run string is given to plugin.Shell, so a host with no shell fails every
// external plugin start for one reason. The error names the absent shell,
// because an error naming the plugin sends the operator to the plugin's own
// code, where there is nothing to repair (`ai/rules/principles.md`).
// `ze doctor` reports the same dependency before the daemon starts, under
// doctor-plugin-shell-missing.
//
// The shell answer is a parameter so a test states it: no test can take /bin/sh
// away from the host it runs on.
func startFailure(startErr, shellErr error) error {
	if shellErr != nil {
		return shellErr
	}
	return startErr
}

// startExternal starts a foreign external program via plugin.Shell.
// Passes ZE_PLUGIN_HUB_HOST/PORT/TOKEN env vars and waits for TLS connect-back.
func (p *Process) startExternal() error {
	if p.config.Run == "" {
		return fmt.Errorf("plugin %s: empty run command", p.config.Name)
	}
	// The fork is the platform's (newPluginShell, exitwait_*.go): on Linux it
	// carries no context, because a stop owes the plugin its own exit first and
	// superviseExit is what turns the end of p.ctx into the closed connection,
	// then the kill. Elsewhere the end of p.ctx kills the child at once.
	cmd := newPluginShell(p.ctx, p.config.Run)
	if p.config.WorkDir != "" {
		cmd.Dir = p.config.WorkDir
	}
	return p.startChildCmd(cmd, true)
}

// buildSeparateProcessCmd validates that p names a forkable compiled-in
// registration and builds the direct child command invoking Ze's own binary
// with ["plugin", p.registryName] (no shell invocation, no Run required).
func (p *Process) buildSeparateProcessCmd(ctx context.Context) (*exec.Cmd, error) {
	if p.registryName == "" {
		return nil, opererr.New(
			"Plugin "+p.config.Name+" has no compiled registration for separate process",
			"process separate applies only to a compiled-in plugin registration",
			"Set internal <registration> on endpoint "+p.config.Name,
		)
	}
	reg := registry.Lookup(p.registryName)
	if reg == nil {
		return nil, opererr.New(
			"Unknown compiled plugin "+p.registryName+" for separate process "+p.config.Name,
			"This binary carries no compiled plugin registered under "+p.registryName,
			"ze show plugin list",
		)
	}
	if reg.RunsInProcessOnly {
		return nil, opererr.New(
			"Plugin "+p.registryName+" cannot run as a separate process",
			"Registration "+p.registryName+" sets RunsInProcessOnly and runs in-daemon only",
			"Remove `process separate` from endpoint "+p.config.Name+" so "+p.registryName+" runs in-daemon",
		)
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("plugin %s: resolve ze executable: %w", p.config.Name, err)
	}
	cmd := newPluginExec(ctx, exe, "plugin", p.registryName)
	if p.config.WorkDir != "" {
		cmd.Dir = p.config.WorkDir
	}
	return cmd, nil
}

// startSeparateProcess starts a compiled-in plugin as a separate forked child
// of Ze's own binary over TLS connect-back, keeping its compiled identity.
func (p *Process) startSeparateProcess() error {
	cmd, err := p.buildSeparateProcessCmd(p.ctx)
	if err != nil {
		return err
	}
	return p.startChildCmd(cmd, false)
}

// startChildCmd wires TLS connect-back environment variables, stderr relay,
// process supervision, and connect-back waiting for a forked child command
// (either a foreign external shell command or a separate-process compiled
// plugin).
func (p *Process) startChildCmd(cmd *exec.Cmd, usesShell bool) error {
	if p.acceptor == nil {
		return fmt.Errorf("plugin %s: no TLS acceptor configured (hub config required for forked plugins)", p.config.Name)
	}

	// Extract host:port from acceptor address.
	addr := p.acceptor.Addr()
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return fmt.Errorf("plugin %s: parse acceptor address %s: %w", p.config.Name, addr, err)
	}
	// A plugin process runs beside the hub, so an unspecified listen address
	// (0.0.0.0 or ::) is reached on the loopback. Tell the child so, because
	// the served leaf carries no SAN for an unspecified address and the child
	// validates the chain against the hub root.
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		host = loopbackHost
	}

	p.cmd = cmd

	// Pass TLS connection info and plugin name via env vars.
	// The engine binary's directory goes on PATH as a fallback.
	// plugin.ChildPathEnv states why it is a fallback rather than an override,
	// and it is shared with the declaration query (../childenv.go).
	//
	// Each ze key is passed under its shell spelling (env.ShellSafe) so a
	// dotted or hyphenated key the daemon holds reaches the plugin rather than
	// leaving it on the key's default. A connect-back variable the daemon
	// inherited is renamed to the same ZE_PLUGIN_* name appended below, and
	// exec keeps only the last value of a duplicated name, so the daemon's own
	// value wins.
	//
	// A plugin forked before the full parse also gets the settings of the
	// config's `environment` block, which the daemon has not applied to itself
	// yet (SetConfigEnvironment).
	p.cmd.Env = p.inheritedEnvironment(os.Environ())
	if pathEnv := plugin.ChildPathEnv(plugin.EngineBinDir(), os.Getenv("PATH")); pathEnv != "" {
		p.cmd.Env = append(p.cmd.Env, pathEnv)
	}
	// Generate a per-plugin token for name-bound authentication.
	pluginToken, tokenErr := p.acceptor.TokenForPlugin(p.config.Name)
	if tokenErr != nil {
		return fmt.Errorf("plugin %s: generate token: %w", p.config.Name, tokenErr)
	}

	p.cmd.Env = append(p.cmd.Env,
		"ZE_PLUGIN_HUB_HOST="+host,
		"ZE_PLUGIN_HUB_PORT="+port,
		"ZE_PLUGIN_HUB_TOKEN="+pluginToken,
		"ZE_PLUGIN_CA_PEM="+string(p.acceptor.RootPEM()),
		"ZE_PLUGIN_NAME="+p.config.Name,
	)

	stderrRead, stderrWrite, pipeErr := attachStderrRelay(p.cmd)
	if pipeErr != nil {
		return fmt.Errorf("plugin %s: stderr pipe: %w", p.config.Name, pipeErr)
	}
	p.stderr = stderrRead

	// The waiter exists before the child does: a child that dials back and
	// authenticates before this goroutine reaches Wait is held for it, where a
	// waiter registered after Start would find the connection already closed
	// for having nobody to take it.
	//
	// The fork goes through plugin.StartChild, never p.cmd.Start: the Linux
	// death signal is bound to the forking thread, and StartChild forks from
	// the one thread ze never lets the runtime destroy.
	waiter := p.acceptor.ExpectPlugin(p.config.Name)
	if err := plugin.StartChild(p.cmd); err != nil {
		waiter.Cancel()
		stderrWrite.Close() //nolint:errcheck,gosec // cleanup on error
		stderrRead.Close()  //nolint:errcheck,gosec // cleanup on error
		if usesShell {
			err = startFailure(err, plugin.ShellAvailable())
		}
		return fmt.Errorf("plugin %s: start: %w", p.config.Name, err)
	}
	// The child holds its own descriptor now. Closing the parent's copy of the
	// write end is what turns the child's exit into EOF for the reader below.
	if closeErr := stderrWrite.Close(); closeErr != nil {
		logger().Debug("close plugin stderr write end", "plugin", p.config.Name, "error", closeErr)
	}

	p.running.Store(true)

	startedCmd := p.cmd
	p.externalDone = make(chan struct{})
	relayDone := make(chan struct{})
	reaped := make(chan struct{})
	relayRead := newStderrRelayReader(stderrRead, stderrDrainGrace)
	p.wg.Go(func() {
		defer close(relayDone)
		p.relayStderrFrom(relayRead)
	})
	pg := &pluginGroup{name: p.config.Name, pid: startedCmd.Process.Pid}
	p.wg.Go(func() { p.monitorCmd(startedCmd, relayDone, relayRead, reaped, pg) })
	if stopGivesExitGrace {
		p.wg.Go(func() { p.superviseExit(reaped, pg) })
	}

	// Wait for the child to connect back via TLS (bounded timeout).
	waitCtx, waitCancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer waitCancel()

	conn, waitErr := waiter.Wait(waitCtx)
	if waitErr != nil {
		// Stop the child, and everything it started, to prevent orphaning. A
		// plugin that never connected has no connection to close and nothing
		// to flush to a daemon that will not read it, so it gets no exit grace.
		pg.kill()
		// Stop delivery goroutine started by startDeliveryLocked to prevent leak.
		p.stopEventChan()
		return fmt.Errorf("plugin %s: TLS connect-back: %w", p.config.Name, waitErr)
	}
	p.rawConn = conn

	return nil
}

// stderrDrainGrace is how long the stderr relay waits for its NEXT byte once
// the plugin process has been reaped and EOF has not come.
//
// EOF arrives the moment the last writer closes the pipe, which is normally the
// plugin's own exit, so the relay reads to EOF and this never fires. It covers
// the one case where EOF does not come: a plugin that spawned a child of its
// own, which inherited the descriptor and outlives it. Without a bound the
// relay would hold Wait open for as long as that grandchild lives.
//
// It is an IDLE bound, refreshed before every read, and never a bound on the
// whole drain. A relay that is slow to read -- the engine log it writes to is
// slow, or the machine is loaded -- still finds every byte the plugin wrote
// waiting in the pipe, and reads it before the bound can fire.
const stderrDrainGrace = 2 * time.Second

// pluginExitGrace is how long a stopped external plugin is given to exit on its
// own, once its connection is closed, before its process group is killed. Only
// Linux gives it (stopGivesExitGrace, exitwait_*.go).
//
// A plugin's last lines are not in the daemon's pipe yet when the stop comes:
// every ze process points os.Stderr at a pipe of its own that a goroutine
// copies on (crashlog.Init), and only the plugin's orderly exit drains it
// (crashlog.Flush). A plugin whose SDK Run returns on the closed connection
// exits in milliseconds, so it never waits this long; only a plugin that goes
// on running after its connection is closed pays it, and then it is killed.
// ProcessManager.Stop and the respawn join wait past it (manager.go).
const pluginExitGrace = time.Second

// stopExitGrace is how long this process's goroutines can run on past a stop
// before its plugin is gone: pluginExitGrace for a forked plugin the stop
// gives that grace (external program or separate-process compiled plugin), and
// nothing for an in-daemon plugin, whose engine ends with its closed
// connection, or where the stop kills at once (stopGivesExitGrace).
// ProcessManager.Stop and the respawn join add it to their own waits.
func (p *Process) stopExitGrace() time.Duration {
	if p.config.RunsInDaemon() {
		return 0
	}
	if !stopGivesExitGrace {
		return 0
	}
	return pluginExitGrace
}

// StopJoinBound is how long a caller waits, after Stop, for this process to
// end: its exit grace (stopExitGrace) plus respawnJoinMargin, 2s for a forked
// plugin on Linux and 1s for one that runs in the daemon. Respawn joins the
// process it replaces for this long, and the server's restart waits this long
// for a replacement it stopped (rollbackReplacement). It bounds the end of the
// process only: the runtime cleanup that follows waits for the process's calls
// into the engine, which its exit does not end, so a caller of
// WaitRuntimeCleanupContext picks its own bound, as the server's restart does
// with restartCleanupGrace. Safe for concurrent use.
func (p *Process) StopJoinBound() time.Duration {
	return p.stopExitGrace() + respawnJoinMargin
}

// stderrRelayReader is the read end of a plugin's stderr pipe, as the relay
// reads it. Until the plugin is reaped a read waits as long as it must. After
// that, each read waits at most grace for data, so a descendant holding the
// write end open cannot hold the relay forever, while data already in the pipe
// is always read.
type stderrRelayReader struct {
	file   *os.File
	grace  time.Duration
	reaped atomic.Bool
}

func newStderrRelayReader(file *os.File, grace time.Duration) *stderrRelayReader {
	return &stderrRelayReader{file: file, grace: grace}
}

// Read reads from the pipe. Once the plugin is reaped it first moves the read
// deadline grace past now, so the bound measures silence and never backlog.
func (r *stderrRelayReader) Read(b []byte) (int, error) {
	if r.reaped.Load() {
		r.extendDeadline()
	}
	return r.file.Read(b)
}

// markReaped records that the plugin process has exited. It also bounds a read
// already waiting, which started before any deadline existed.
func (r *stderrRelayReader) markReaped() {
	r.reaped.Store(true)
	r.extendDeadline()
}

func (r *stderrRelayReader) extendDeadline() {
	if err := r.file.SetReadDeadline(time.Now().Add(r.grace)); err != nil {
		logger().Debug("set plugin stderr read deadline", "error", err)
	}
}

// attachStderrRelay gives cmd a stderr pipe whose read end THIS process owns,
// and returns both ends. The caller closes the write end after Start.
//
// Cmd.StderrPipe is deliberately NOT used. It registers the read end with the
// Cmd, and Cmd.Wait closes every pipe it registered as soon as the child exits:
// "it is thus incorrect to call Wait before all reads from the pipe have
// completed" (os/exec). monitorCmd calls Wait while relayStderrFrom is still
// reading, so anything the relay had not consumed was discarded -- and the line
// most exposed is the LAST one a plugin writes, which is the one that says how
// it finished. A pipe this package creates is not registered with the Cmd, so
// Wait cannot close it, and the reader sees every byte through to EOF.
func attachStderrRelay(cmd *exec.Cmd) (read, write *os.File, err error) {
	read, write, err = os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	cmd.Stderr = write
	return read, write, nil
}

// drainStderrRelay releases the read end of a plugin's stderr pipe once the
// relay goroutine has read it to its end: EOF, or grace of silence after the
// plugin was reaped (stderrRelayReader). It waits on the relay and on nothing
// else, so no byte the plugin wrote is discarded by a timer.
func drainStderrRelay(relayDone <-chan struct{}, read *stderrRelayReader) {
	read.markReaped()
	<-relayDone
	if err := read.file.Close(); err != nil {
		logger().Debug("close plugin stderr read end", "error", err)
	}
}

// relayStderrFrom reads plugin stderr and relays to engine logs.
// Plugin stderr format: time=... level=DEBUG msg="..." subsystem=gr ...
// When ze.log.relay=<level>, relays messages at or above that level.
// When disabled (empty/disabled), suppresses relay output but still captures
// declared terminal cleanup failures for the original resource warning.
// Takes an explicit io.Reader to avoid racing with monitor() on p.stderr.
//
// Go runtime panic blocks (lines beginning with "panic:" or "fatal error:",
// plus the goroutine stack that follows) are always relayed at ERROR level,
// regardless of the configured relay filter. Without this, a process-killing
// panic parses as LevelInfo (no level= tag) and gets silently dropped at the
// default WARN filter -- see known-failures entry "SDK NewFromTLSEnv missing
// initCallbackDefaults" for the bug this masked.
func (p *Process) relayStderrFrom(stderr io.Reader) {
	// Get configured relay level
	relayLevel, enabled := slogutil.RelayLevel()
	var inPanic bool
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		p.captureTerminalStderr(line)
		if !enabled {
			continue
		}
		var (
			level slog.Level
			msg   string
			attrs []any
			skip  bool
		)
		level, msg, attrs, inPanic, skip = classifyStderrLine(line, inPanic, relayLevel)
		if skip {
			continue
		}
		// Build args: plugin name + original attrs
		args := []any{reportSourcePlugin, p.config.Name}
		if len(attrs) > 0 {
			args = append(args, slog.Group("original", attrs...))
		}
		stderrLogger().Log(context.Background(), level, msg, args...)
	}

	// Scan returns false on EOF, on a read error, and on a line above
	// bufio.MaxScanTokenSize alike. Only EOF means the plugin's stderr ended:
	// the other two end the RELAY while the plugin keeps running, so the panic
	// block the paragraph above exists to carry never reaches the engine log.
	// A relay that stopped says so.
	err := scanner.Err()
	p.noteTerminalStderrFailure(err)
	if !enabled {
		return // Disabled relay keeps ordinary plugins' prior discard behavior.
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		// The plugin was reaped and a descendant still holds its stderr open
		// with nothing written for stderrDrainGrace (stderrRelayReader).
		logger().Debug("plugin stderr relay ended: the plugin exited and its stderr stayed silent",
			"plugin", p.config.Name)
		return
	}
	if err != nil {
		stderrLogger().Error("plugin stderr relay stopped, later plugin output is not relayed",
			"plugin", p.config.Name, "err", err, "code", CodeDoctorPluginStderrRelayStopped)
		raiseRelayStopped(p.config.Name, err)
	}
}

// classifyStderrLine inspects a single plugin stderr line and returns how it
// should be relayed. It parses slog format via slogutil.ParseLogLine, then
// applies three overrides:
//
//  1. Once a Go runtime panic prefix ("panic:" or "fatal error:") has been
//     seen, the returned level is forced to ERROR for that line and every
//     line after it until a valid slog-formatted line arrives. A valid slog
//     line resets inPanic to false, since it clearly belongs to the plugin's
//     normal logging again.
//  2. Non-empty plain stderr lines from plugins are treated as WARN. Test
//     helpers and ad-hoc plugins often print human text instead of slog
//     records; relaying those at INFO made them disappear behind the default
//     WARN filter even when the plugin intentionally wrote to stderr.
//  3. Lines whose computed level is below the configured relayLevel are
//     reported via skip=true so the caller can drop them.
//
// The panic-block forcing runs BEFORE the level filter, so a panic block is
// always relayed regardless of ze.log.relay. Extracted from relayStderrFrom
// so the classifier can be unit-tested without a live process.
// so the classifier can be unit-tested without a live process.
func classifyStderrLine(line string, inPanic bool, relayLevel slog.Level) (level slog.Level, msg string, attrs []any, nowInPanic, skip bool) {
	level, msg, attrs = slogutil.ParseLogLine(line)
	// A parseable slog line (has level= and msg=) ends any ongoing panic block.
	// ParseLogLine returns non-Info levels only when level= was actually
	// extracted; the "malformed text" path returns LevelInfo with attrs == nil.
	validSlog := strings.Contains(line, "level=") && strings.Contains(line, "msg=")
	if validSlog {
		inPanic = false
	} else if !inPanic && strings.TrimSpace(line) != "" {
		level = slog.LevelWarn
	}
	if !inPanic && isPanicStart(line) {
		inPanic = true
	}
	if inPanic {
		level = slog.LevelError
	}
	if level < relayLevel {
		return level, msg, attrs, inPanic, true
	}
	return level, msg, attrs, inPanic, false
}

// isPanicStart reports whether line marks the beginning of a Go runtime
// panic or fatal-error block. Matches the exact prefixes the runtime emits
// (see runtime/panic.go); plugin-emitted log messages containing the word
// "panic" in their msg= payload are not matched because the prefixes
// require the line to START with the sentinel.
func isPanicStart(line string) bool {
	return strings.HasPrefix(line, "panic:") ||
		strings.HasPrefix(line, "fatal error:")
}

// Stop signals the process to terminate. Does not block.
// For external plugins, closing the connections ends the plugin's SDK Run so
// the plugin exits on its own; on Linux superviseExit kills its process group
// only if it is still running pluginExitGrace later, and elsewhere the canceled
// context kills the group at once (newPluginShell).
// For internal plugins, closing RPC connections unblocks the plugin's reads and causes it to exit.
// Callers MUST call Wait after Stop to ensure all goroutines have exited.
func (p *Process) Stop() {
	if p.bridge != nil {
		p.bridge.StopDispatch()
	}
	if p.cancel != nil {
		p.cancel()
	}

	// Close event queue first — delivery goroutine drains remaining items
	// (which fail fast since context is canceled) then exits.
	p.stopEventChan()

	// Close bridge callback channel so the plugin's bridge event loop exits
	// and any pending SendCallback calls unblock with a closed-channel panic
	// guard (select on ctx.Done handles this gracefully).
	if p.bridge != nil {
		p.bridge.CloseCallbacks()
	}

	p.closeConns()
}

// closeConns closes all RPC connections exactly once. Safe to call from
// both Stop() and monitorCmd() concurrently — sync.Once ensures the
// cleanup runs only on the first call.
func (p *Process) closeConns() {
	p.cleanupOnce.Do(func() {
		p.mu.Lock()
		if p.conn != nil {
			p.conn.Close() //nolint:errcheck,gosec // best-effort shutdown
			p.conn = nil
		}
		if p.rawConn != nil {
			p.rawConn.Close() //nolint:errcheck,gosec // best-effort shutdown
			p.rawConn = nil
		}
		p.mu.Unlock()
	})
}

// SendShutdown sends a graceful shutdown signal (bye RPC) to the plugin.
// Returns true if the process was running. The bye RPC gives the plugin a
// chance to clean up before Stop() closes connections and kills the process.
func (p *Process) SendShutdown() bool {
	if !p.running.Load() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	conn := p.Conn()
	if conn == nil {
		return true
	}
	_ = conn.SendBye(ctx, "shutdown") //nolint:errcheck // best-effort graceful signal
	return true
}

// Wait blocks until all process goroutines have exited, or ctx expires.
// Must be called after Stop to avoid goroutine leaks.
func (p *Process) Wait(ctx context.Context) error {
	return syncutil.WaitGroupWait(ctx, &p.wg)
}

// superviseExit stops the external plugin when its process context ends:
// Stop cancels it, and so does ProcessManager.Stop through the parent, before
// any per-process Stop runs. The stop closes the plugin's connections, which
// ends its SDK Run and lets it exit on its own, writing out what it already
// wrote, and kills its process group only if it is still running
// pluginExitGrace later (pluginExitGrace states why the exit comes first).
//
// One goroutine per external Process, started by startExternal where a stop
// gives the plugin that grace (stopGivesExitGrace), and ended by reaped, which
// monitorCmd closes once the plugin has been reaped. A grace that runs out as
// the plugin exits is safe: group.kill sends nothing once monitorCmd has
// recorded the exit.
func (p *Process) superviseExit(reaped <-chan struct{}, group *pluginGroup) {
	select {
	case <-reaped:
		return
	case <-p.ctx.Done():
	}
	p.closeConns()

	grace := time.NewTimer(pluginExitGrace)
	defer grace.Stop()
	select {
	case <-reaped:
	case <-grace.C:
		group.kill()
	}
}

// pluginGroup is the process group an external plugin heads, and the one place
// the daemon signals it from on Linux (elsewhere the fork's own Cancel signals
// it as well, newPluginShell). exited is set once the plugin has exited, and
// no signal is sent after it: the reap frees the plugin's pid, and with it the
// group number, for the next process the kernel starts, so a kill sent after
// the reap can reach a stranger.
//
// Safe for concurrent use: superviseExit, startExternal's connect-back failure
// and monitorCmd all reach it.
type pluginGroup struct {
	name string
	pid  int

	mu     sync.Mutex
	exited bool
}

// kill SIGKILLs the plugin's group, unless the plugin has already exited.
func (g *pluginGroup) kill() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.exited {
		return
	}
	g.signalLocked()
}

// exit records that the plugin has exited, so no kill follows. stopped says
// the daemon was stopping the plugin when it exited: the plugin then left on
// its own, and the descendants still in its group get the signal a killed
// plugin's group gets. The caller MUST pass stopped only with the plugin still
// unreaped, while its pid and the group it heads are still its own.
func (g *pluginGroup) exit(stopped bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.exited {
		return
	}
	g.exited = true
	if stopped {
		g.signalLocked()
	}
}

// signalLocked sends the group SIGKILL. The caller MUST hold mu.
//
// A pid that is not positive is a guard, not a value: KillProcessGroup(0)
// signals the daemon's own process group, and a negative pid a group of the
// kernel's choosing. startExternal builds the group from a started process
// only, so no pid here is ever below one.
func (g *pluginGroup) signalLocked() {
	if g.pid < 1 {
		panic("BUG: pluginGroup with no started process")
	}
	if err := plugin.KillProcessGroup(g.pid); err != nil {
		logger().Debug("signal a plugin's process group", "plugin", g.name, "error", err)
	}
}

// monitorCmd waits for the process to exit.
// Takes cmd as a parameter to avoid racing on p.cmd with other goroutines.
//
// relayDone closes when relayStderrFrom returns, and stderr is the read end that
// goroutine is reading. Both are parameters for the same reason cmd is. reaped
// is closed once cmd.Wait has returned, which is what ends superviseExit, and
// group is where the exit is recorded before the reap. externalDone closes
// only after this monitor has also joined the actual stderr relay.
func (p *Process) monitorCmd(cmd *exec.Cmd, relayDone <-chan struct{}, stderr *stderrRelayReader, reaped chan<- struct{}, group *pluginGroup) {
	// The exit is recorded with the plugin still unreaped, so no kill can be
	// sent past the reap (pluginGroup), and a stopped plugin that exited on its
	// own has its descendants signaled while the group number is still its own.
	if waitExitedUnreaped(cmd.Process.Pid) {
		group.exit(p.ctx.Err() != nil)
	}

	waitErr := cmd.Wait()
	p.exit.record(forkExitStatus(cmd, waitErr))
	// Where waitExitedUnreaped had no answer the exit is recorded only now, past
	// the reap: a kill that raced it in between is the one this cannot exclude.
	group.exit(false)
	close(reaped)

	p.running.Store(false)

	// Cancel context. Safe even if Stop() already canceled it (cancel is idempotent).
	if p.cancel != nil {
		p.cancel()
	}

	// Close RPC connections via sync.Once — safe if Stop() races with monitor.
	// The delivery worker ranges its original queue, so cancellation alone
	// cannot join it after an unexpected exit. Drain that same queue here.
	p.stopEventChan()
	p.closeConns()

	// The stderr pipe is this package's, not the Cmd's, so Wait above did not
	// close it and the relay still holds every byte the plugin wrote on its way
	// out. Release the descriptor only once the relay has read them, which is
	// what keeps a plugin's last line -- its verdict, or its panic block.
	drainStderrRelay(relayDone, stderr)
	p.finishTerminalCleanup(actualForkCleanupSuccess(cmd, waitErr), forkExitStatus(cmd, waitErr))
	close(p.externalDone)
}

// forkExitStatus names how a forked plugin ended: the reaped process state
// ("exit status 1", "signal: killed"), or the wait error when the state was
// not recorded.
func forkExitStatus(cmd *exec.Cmd, waitErr error) string {
	if cmd.ProcessState != nil {
		return cmd.ProcessState.String()
	}
	if waitErr != nil {
		return "an unreaped exit: " + waitErr.Error()
	}
	return "an exit whose state was not recorded"
}
