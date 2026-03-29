// Design: docs/architecture/api/process-protocol.md — plugin process lifecycle
// Detail: delivery.go — event delivery pipeline
// Detail: manager.go — multi-process coordination and respawn
// Detail: sysproc_linux.go — Linux-specific process isolation
// Detail: sysproc_other.go — non-Linux process isolation
// Related: ../ipc/tls.go — PluginAcceptor used by startExternal (WaitForPlugin for TLS connect-back)
// Related: ../../../../pkg/plugin/sdk/sdk.go — plugin-side SDK that connects back via TLS

package process

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/core/syncutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

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
	index  int // Plugin index for coordinator (0-based)
	cmd    *exec.Cmd

	stderr io.ReadCloser

	// Raw connection for IPC. Set during startup, consumed by InitConns().
	rawConn net.Conn

	// TLS acceptor for external plugin connect-back (set by SetAcceptor).
	acceptor *ipc.PluginAcceptor

	// RPC connection for YANG RPC protocol.
	// Created by InitConns() from rawConn, or set directly by tests via SetConn.
	conn *ipc.PluginConn

	running atomic.Bool

	// Session state (per-process API connection state)
	// Note: ACK is controlled by serial prefix (#N), not per-process state
	syncEnabled   atomic.Bool // Whether to wait for wire transmission (default: false)
	cacheConsumer atomic.Bool // Whether plugin participates in cache consumer tracking

	// Wire encoding for API messages (default: WireEncodingHex = 0)
	wireEncodingIn  atomic.Uint32 // Inbound: events ze→Process
	wireEncodingOut atomic.Uint32 // Outbound: commands Process→ze

	// High-level encoding and format (bgp plugin encoding/format commands)
	encoding       atomic.Value // string: "json" or "text" (default: "json")
	format         atomic.Value // string: "hex", "base64", "parsed", "full" (default: "hex")
	formatCacheKey atomic.Value // string: precomputed "format+encoding" for event dispatch cache lookup

	// Registered plugin commands (tracked for cleanup on death)
	registeredCommands []string
	registeredMu       sync.Mutex

	// Plugin registration protocol (5-stage startup)
	stage        atomic.Int32               // Current stage (PluginStage)
	registration *plugin.PluginRegistration // Stage 1 registration data
	capabilities *plugin.PluginCapabilities // Stage 3 capability declarations
	stageCh      chan struct{}              // Signals stage completion
	stageMu      sync.Mutex                 // Protects stage transitions

	// Direct transport bridge for internal plugins (nil for external).
	// After 5-stage startup, events and RPCs bypass socket I/O via function calls.
	bridge *rpc.DirectBridge

	// Long-lived event delivery goroutine (see rules/goroutine-lifecycle.md).
	// Events are enqueued via Deliver() and processed by deliveryLoop().
	// eventMu protects channel close: Deliver holds RLock, stopEventChan holds Lock.
	eventChan   chan EventDelivery
	eventClosed bool
	eventMu     sync.RWMutex

	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.Mutex
	cleanupOnce sync.Once // ensures connection cleanup runs exactly once
}

// NewProcess creates a new process with the given configuration.
func NewProcess(config plugin.PluginConfig) *Process {
	return &Process{
		config:       config,
		registration: &plugin.PluginRegistration{},
		capabilities: &plugin.PluginCapabilities{},
		stageCh:      make(chan struct{}),
	}
}

// Stage returns the current plugin startup stage.
func (p *Process) Stage() plugin.PluginStage {
	return plugin.PluginStage(p.stage.Load())
}

// SetStage sets the current stage and notifies waiters.
func (p *Process) SetStage(stage plugin.PluginStage) {
	p.stageMu.Lock()
	defer p.stageMu.Unlock()
	// Safe: PluginStage has only values 0-6 (StageInit..StageRunning).
	p.stage.Store(int32(stage)) //nolint:gosec // G115: bounded enum
	// Close and recreate channel to notify all waiters
	close(p.stageCh)
	p.stageCh = make(chan struct{})
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

// Registration returns the plugin registration data (Stage 1).
func (p *Process) Registration() *plugin.PluginRegistration {
	return p.registration
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
		return fmt.Errorf("no raw connection available")
	}

	raw := p.rawConn
	p.rawConn = nil

	rpcConn := rpc.NewConn(raw, raw)
	mux := rpc.NewMuxConn(rpcConn)
	p.conn = ipc.NewMuxPluginConn(mux)
	return nil
}

// SyncEnabled returns true if sync mode is enabled for this process.
// When enabled, announce/withdraw waits for wire transmission before ACK.
func (p *Process) SyncEnabled() bool {
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
func (p *Process) SetCacheConsumer(enabled bool) {
	p.cacheConsumer.Store(enabled)
}

// WireEncodingIn returns the inbound wire encoding (events ze→Process).
func (p *Process) WireEncodingIn() plugin.WireEncoding {
	// Safe: only values 0-3 are ever stored (WireEncodingHex..WireEncodingText).
	return plugin.WireEncoding(p.wireEncodingIn.Load()) //nolint:gosec // Bounded to 0-3
}

// WireEncodingOut returns the outbound wire encoding (commands Process→ze).
func (p *Process) WireEncodingOut() plugin.WireEncoding {
	// Safe: only values 0-3 are ever stored (WireEncodingHex..WireEncodingText).
	return plugin.WireEncoding(p.wireEncodingOut.Load()) //nolint:gosec // Bounded to 0-3
}

// SetWireEncodingIn sets the inbound wire encoding.
func (p *Process) SetWireEncodingIn(enc plugin.WireEncoding) {
	p.wireEncodingIn.Store(uint32(enc))
}

// SetWireEncodingOut sets the outbound wire encoding.
func (p *Process) SetWireEncodingOut(enc plugin.WireEncoding) {
	p.wireEncodingOut.Store(uint32(enc))
}

// SetWireEncoding sets both inbound and outbound wire encoding.
func (p *Process) SetWireEncoding(enc plugin.WireEncoding) {
	p.wireEncodingIn.Store(uint32(enc))
	p.wireEncodingOut.Store(uint32(enc))
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

// AddRegisteredCommand tracks a command registered by this process.
func (p *Process) AddRegisteredCommand(name string) {
	p.registeredMu.Lock()
	defer p.registeredMu.Unlock()
	p.registeredCommands = append(p.registeredCommands, name)
}

// RemoveRegisteredCommand removes a command from tracking.
func (p *Process) RemoveRegisteredCommand(name string) {
	p.registeredMu.Lock()
	defer p.registeredMu.Unlock()
	for i, cmd := range p.registeredCommands {
		if cmd == name {
			p.registeredCommands = append(p.registeredCommands[:i], p.registeredCommands[i+1:]...)
			return
		}
	}
}

// RegisteredCommands returns a copy of the registered command names.
func (p *Process) RegisteredCommands() []string {
	p.registeredMu.Lock()
	defer p.registeredMu.Unlock()
	result := make([]string, len(p.registeredCommands))
	copy(result, p.registeredCommands)
	return result
}

// Name returns the process name from config.
func (p *Process) Name() string {
	return p.config.Name
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

// SetRegistration sets the plugin registration data (Stage 1).
func (p *Process) SetRegistration(reg *plugin.PluginRegistration) {
	p.registration = reg
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

// CloseConn closes and nils the RPC connection under the mutex.
func (p *Process) CloseConn() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		p.conn.Close() //nolint:errcheck,gosec // best-effort shutdown
		p.conn = nil
	}
}

// ClearConn nils the connection pointer without closing the underlying connection.
// Used in tests to simulate a process dying between verify and apply phases.
func (p *Process) ClearConn() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conn = nil
}

// Start spawns the process.
func (p *Process) Start() error {
	return p.StartWithContext(context.Background())
}

// StartWithContext spawns the process with the given context.
// For internal plugins (config.Internal=true), runs in-process via goroutine.
// For external plugins, forks via exec.Command.
func (p *Process) StartWithContext(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ctx, p.cancel = context.WithCancel(ctx)

	// Start long-lived event delivery goroutine (see rules/goroutine-lifecycle.md).
	p.startDeliveryLocked()

	// Internal plugins run in-process via goroutine
	if p.config.Internal {
		return p.startInternal()
	}

	return p.startExternal()
}

// startInternal starts an internal plugin as a goroutine with a single net.Pipe.
// Creates a MuxPluginConn for bidirectional YANG RPC protocol.
func (p *Process) startInternal() error {
	name := p.config.Name
	// If run specifies an internal plugin (ze.X or ze plugin X), use that name
	// for runner lookup. This allows config name ("rr") to differ from
	// internal name ("bgp-rs").
	if p.config.Run != "" {
		if res, err := plugin.ResolvePlugin(p.config.Run); err == nil && res.Type == plugin.PluginTypeInternal {
			name = res.Name
		}
	}

	runner := plugin.GetInternalPluginRunner(name)
	if runner == nil {
		return fmt.Errorf("unknown internal plugin: %s", name)
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

	// Start the plugin in a goroutine.
	p.wg.Go(func() {
		defer p.running.Store(false)
		defer func() {
			if rec := recover(); rec != nil {
				logger().Error("internal plugin panic", "plugin", p.config.Name, "panic", rec, "stack", string(debug.Stack()))
			}
		}()
		defer func() {
			if err := bridgedPluginSide.Close(); err != nil {
				logger().Debug("close plugin side", "error", err)
			}
		}()

		if code := runner(bridgedPluginSide); code != 0 {
			logger().Warn("internal plugin exited with non-zero code", "plugin", p.config.Name, "code", code)
		}
	})

	return nil
}

// startExternal starts an external plugin via exec.Command.
// Passes ZE_PLUGIN_HUB_HOST/PORT/TOKEN env vars and waits for TLS connect-back.
func (p *Process) startExternal() error {
	if p.config.Run == "" {
		return fmt.Errorf("plugin %s: empty run command", p.config.Name)
	}
	if p.acceptor == nil {
		return fmt.Errorf("plugin %s: no TLS acceptor configured (hub config required for external plugins)", p.config.Name)
	}

	// Extract host:port from acceptor address.
	addr := p.acceptor.Addr()
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return fmt.Errorf("plugin %s: parse acceptor address %s: %w", p.config.Name, addr, err)
	}

	// #nosec G204 - Run command is from trusted configuration, not user input
	p.cmd = exec.CommandContext(p.ctx, "/bin/sh", "-c", p.config.Run)
	if p.config.WorkDir != "" {
		p.cmd.Dir = p.config.WorkDir
	}

	// Pass TLS connection info and plugin name via env vars.
	// Prepend the engine binary's directory to PATH so that run commands
	// like "ze plugin bgp-rib" can find the ze binary even when it is
	// not installed system-wide (e.g., running from ./bin/ze in dev/test).
	p.cmd.Env = os.Environ()
	if exe, exeErr := os.Executable(); exeErr == nil {
		binDir := filepath.Dir(exe)
		p.cmd.Env = append(p.cmd.Env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	p.cmd.Env = append(p.cmd.Env,
		"ZE_PLUGIN_HUB_HOST="+host,
		"ZE_PLUGIN_HUB_PORT="+port,
		"ZE_PLUGIN_HUB_TOKEN="+p.acceptor.Token(),
		"ZE_PLUGIN_NAME="+p.config.Name,
	)

	p.stderr, err = p.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("plugin %s: stderr pipe: %w", p.config.Name, err)
	}

	p.cmd.SysProcAttr = newSysProcAttr()

	if err := p.cmd.Start(); err != nil {
		p.stderr.Close() //nolint:errcheck,gosec // cleanup on error
		return fmt.Errorf("plugin %s: start: %w", p.config.Name, err)
	}

	p.running.Store(true)

	stderr := p.stderr
	cmd := p.cmd
	p.wg.Go(func() { p.relayStderrFrom(stderr) })
	p.wg.Go(func() { p.monitorCmd(cmd) })

	// Wait for the child to connect back via TLS (bounded timeout).
	waitCtx, waitCancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer waitCancel()

	conn, waitErr := p.acceptor.WaitForPlugin(waitCtx, p.config.Name)
	if waitErr != nil {
		// Kill the child process to prevent orphaning.
		if p.cmd != nil && p.cmd.Process != nil {
			p.cmd.Process.Kill() //nolint:errcheck,gosec // cleanup on connect-back failure
		}
		return fmt.Errorf("plugin %s: TLS connect-back: %w", p.config.Name, waitErr)
	}
	p.rawConn = conn

	return nil
}

// relayStderrFrom reads plugin stderr and relays to engine logs.
// Plugin stderr format: time=... level=DEBUG msg="..." subsystem=gr ...
// When ze.log.relay=<level>, relays messages at or above that level.
// When disabled (empty/disabled), discards plugin stderr silently.
// Takes an explicit io.Reader to avoid racing with monitor() on p.stderr.
func (p *Process) relayStderrFrom(stderr io.Reader) {
	// Get configured relay level
	relayLevel, enabled := slogutil.RelayLevel()
	if !enabled {
		// Discard all stderr when relay disabled
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			// Read but discard
		}
		return
	}

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		// Parse the slog line and relay with subsystem=plugin.relay
		level, msg, attrs := slogutil.ParseLogLine(line)
		// Filter by configured relay level
		if level < relayLevel {
			continue
		}
		// Build args: plugin name + original attrs
		args := []any{"plugin", p.config.Name}
		if len(attrs) > 0 {
			args = append(args, slog.Group("original", attrs...))
		}
		stderrLogger().Log(context.Background(), level, msg, args...)
	}
}

// Stop signals the process to terminate. Does not block.
// For external plugins, canceling context kills the process via exec.CommandContext.
// For internal plugins, closing RPC connections unblocks the plugin's reads and causes it to exit.
// Callers MUST call Wait after Stop to ensure all goroutines have exited.
func (p *Process) Stop() {
	if p.cancel != nil {
		p.cancel()
	}

	// Close event channel first — delivery goroutine drains remaining items
	// (which fail fast since context is canceled) then exits.
	p.stopEventChan()

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

// monitorCmd waits for the process to exit.
// Takes cmd as a parameter to avoid racing on p.cmd with other goroutines.
func (p *Process) monitorCmd(cmd *exec.Cmd) {
	// Wait for process to exit
	_ = cmd.Wait()

	p.running.Store(false)

	// Cancel context. Safe even if Stop() already canceled it (cancel is idempotent).
	if p.cancel != nil {
		p.cancel()
	}

	// Close RPC connections via sync.Once — safe if Stop() races with monitor.
	// Note: p.stderr is NOT closed here — relayStderrFrom owns a captured copy
	// of the reader and will exit when cmd.Wait() closes the pipe.
	p.closeConns()
}
