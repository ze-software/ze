// Design: docs/architecture/api/process-protocol.md — plugin process lifecycle
// Detail: delivery.go — event delivery pipeline
// Detail: manager.go — multi-process coordination and respawn
// Detail: sysproc_linux.go — Linux-specific process isolation
// Detail: sysproc_other.go — non-Linux process isolation

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
// Tagged with subsystem=relay to distinguish from engine logs.
// Level controlled by ze.log.relay env var.
var stderrLogger = slogutil.LazyLogger("relay")

// Process represents an external subprocess.
type Process struct {
	config plugin.PluginConfig
	index  int // Plugin index for coordinator (0-based)
	cmd    *exec.Cmd

	stderr io.ReadCloser

	// Socket pairs for IPC (internal plugins use net.Pipe, external use socketpair)
	sockets *ipc.DualSocketPair

	// Raw engine-side connections for protocol mode auto-detection.
	// Stored during startup; consumed by initConns() which peeks the
	// first byte on Socket A to detect JSON vs text mode.
	rawEngineA   net.Conn // Socket A engine side (plugin→engine)
	rawCallbackB net.Conn // Socket B engine side (engine→plugin)

	// RPC connections for YANG RPC protocol (per-socket wiring).
	// Created by initConns() after protocol mode detection, or set directly by tests.
	engineConnA *ipc.PluginConn // Socket A: reads/writes plugin→engine RPCs
	engineConnB *ipc.PluginConn // Socket B: reads/writes engine→plugin callbacks

	// Text-mode Socket B connection for event delivery (nil for JSON-mode plugins).
	// Set by handleTextProcessStartup() after text handshake completes.
	textConnB *rpc.TextConn

	running atomic.Bool

	// Session state (per-process API connection state)
	// Note: ACK is controlled by serial prefix (#N), not per-process state
	syncEnabled   atomic.Bool // Whether to wait for wire transmission (default: false)
	cacheConsumer atomic.Bool // Whether plugin participates in cache consumer tracking

	// Wire encoding for API messages (default: WireEncodingHex = 0)
	wireEncodingIn  atomic.Uint32 // Inbound: events ze→Process
	wireEncodingOut atomic.Uint32 // Outbound: commands Process→ze

	// High-level encoding and format (bgp plugin encoding/format commands)
	encoding atomic.Value // string: "json" or "text" (default: "json")
	format   atomic.Value // string: "hex", "base64", "parsed", "full" (default: "hex")

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

// ConnA returns the plugin→engine RPC connection under the mutex.
// Returns nil if the connection has been closed (e.g. by Stop() or monitor()).
// Callers must check for nil before use to avoid racing with shutdown.
func (p *Process) ConnA() *ipc.PluginConn {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.engineConnA
}

// ConnB returns the engine→plugin callback connection under the mutex.
// Returns nil if the connection has been closed (e.g. by Stop() or monitor()).
// Callers must check for nil before use to avoid racing with shutdown.
func (p *Process) ConnB() *ipc.PluginConn {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.engineConnB
}

// TextConnB returns the text-mode Socket B connection for event delivery.
// Returns nil for JSON-mode plugins. Callers must check for nil.
func (p *Process) TextConnB() *rpc.TextConn {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.textConnB
}

// SetTextConnB sets the text-mode Socket B connection for event delivery.
// Called by handleTextProcessStartup after the text handshake completes.
func (p *Process) SetTextConnB(tc *rpc.TextConn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.textConnB = tc
}

// SetConnB sets the engine→plugin callback connection.
// Used by test code to inject mock connections.
func (p *Process) SetConnB(conn *ipc.PluginConn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.engineConnB = conn
}

// initConns detects the protocol mode by peeking the first byte on Socket A,
// then creates the appropriate typed connections.
//
// For JSON mode: creates PluginConns, stores them in the Process. Returns ModeJSON.
// For text mode: returns ModeText and the raw conns for the caller to wrap as TextConns.
//
// If PluginConns already exist (set directly by tests), returns ModeJSON immediately.
// Must be called exactly once before any reads from the connections.
func (p *Process) InitConns() (rpc.ConnMode, net.Conn, net.Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.rawEngineA == nil {
		if p.engineConnA != nil {
			return rpc.ModeJSON, nil, nil, nil // already initialized
		}
		return "", nil, nil, fmt.Errorf("no raw connections available")
	}

	mode, peekedA, err := rpc.PeekMode(p.rawEngineA)
	if err != nil {
		return "", nil, nil, fmt.Errorf("detect protocol mode: %w", err)
	}

	rawB := p.rawCallbackB
	p.rawEngineA = nil
	p.rawCallbackB = nil

	if mode == rpc.ModeJSON {
		p.engineConnA = ipc.NewPluginConn(peekedA, peekedA)
		p.engineConnB = ipc.NewPluginConn(rawB, rawB)
		return rpc.ModeJSON, nil, nil, nil
	}

	// Text mode: return raw conns for caller to create TextConns.
	return rpc.ModeText, peekedA, rawB, nil
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

// SetSockets sets the socket pairs for IPC.
func (p *Process) SetSockets(s *ipc.DualSocketPair) {
	p.sockets = s
}

// Sockets returns the socket pairs for IPC.
func (p *Process) Sockets() *ipc.DualSocketPair {
	return p.sockets
}

// SetConnA sets the plugin→engine RPC connection under the mutex.
func (p *Process) SetConnA(conn *ipc.PluginConn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.engineConnA = conn
}

// SetRunning sets the running state of the process.
func (p *Process) SetRunning(running bool) {
	p.running.Store(running)
}

// SetRawConns sets the raw engine-side connections for protocol mode auto-detection.
func (p *Process) SetRawConns(engineA, callbackB net.Conn) {
	p.rawEngineA = engineA
	p.rawCallbackB = callbackB
}

// CloseConnB closes and nils the engine→plugin callback connection under the mutex.
func (p *Process) CloseConnB() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.engineConnB != nil {
		p.engineConnB.Close() //nolint:errcheck,gosec // best-effort shutdown
		p.engineConnB = nil
	}
}

// ClearConnB nils the ConnB pointer without closing the underlying connection.
// Used in tests to simulate a process dying between verify and apply phases.
func (p *Process) ClearConnB() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.engineConnB = nil
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

// startInternal starts an internal plugin as a goroutine with socket pairs.
// Uses NewInternalSocketPairs() for in-memory bidirectional connections.
// Creates per-socket PluginConns for YANG RPC protocol.
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

	// Create socket pairs for bidirectional IPC
	pairs, err := ipc.NewInternalSocketPairs()
	if err != nil {
		return fmt.Errorf("creating socket pairs: %w", err)
	}
	p.sockets = pairs
	p.stderr = nil // Internal plugins don't have stderr
	p.running.Store(true)

	// Store raw engine-side connections for protocol mode auto-detection.
	// PluginConns are created later by initConns() after peeking Socket A
	// to detect JSON vs text mode.
	p.rawEngineA = pairs.Engine.EngineSide
	p.rawCallbackB = pairs.Callback.EngineSide

	// Create direct transport bridge for post-startup hot path.
	// The bridge carries through BridgedConn so the SDK can discover it
	// via type assertion after the 5-stage startup completes.
	p.bridge = rpc.NewDirectBridge()

	// Wrap plugin-side connections with bridge reference.
	enginePluginSide := rpc.NewBridgedConn(pairs.Engine.PluginSide, p.bridge)
	callbackPluginSide := rpc.NewBridgedConn(pairs.Callback.PluginSide, p.bridge)

	// Start the plugin in a goroutine
	// Plugin side: read from callback socket, write to engine socket
	p.wg.Go(func() {
		defer p.running.Store(false)
		defer func() {
			if rec := recover(); rec != nil {
				logger().Error("internal plugin panic", "plugin", p.config.Name, "panic", rec, "stack", string(debug.Stack()))
			}
		}()
		defer func() {
			if err := enginePluginSide.Close(); err != nil {
				logger().Debug("close engine plugin side", "error", err)
			}
		}()
		defer func() {
			if err := callbackPluginSide.Close(); err != nil {
				logger().Debug("close callback plugin side", "error", err)
			}
		}()

		if code := runner(enginePluginSide, callbackPluginSide); code != 0 {
			logger().Warn("internal plugin exited with non-zero code", "plugin", p.config.Name, "code", code)
		}
	})

	return nil
}

// startExternal starts an external plugin via exec.Command.
// All external plugins use YANG RPC protocol via inherited socket pair FDs
// (ZE_ENGINE_FD/ZE_CALLBACK_FD env vars).
func (p *Process) startExternal() error {
	// Create Unix socketpairs before starting the process.
	// These have real FDs that can be inherited by the subprocess via ExtraFiles.
	pairs, err := ipc.NewExternalSocketPairs()
	if err != nil {
		return fmt.Errorf("creating socket pairs: %w", err)
	}
	pluginEngineFile, pluginCallbackFile, err := pairs.PluginFiles()
	if err != nil {
		pairs.Close()
		return fmt.Errorf("getting plugin files: %w", err)
	}

	// Validate Run command is not empty
	if p.config.Run == "" {
		closeFiles(pluginEngineFile, pluginCallbackFile)
		pairs.Close()
		return fmt.Errorf("plugin %s: empty run command", p.config.Name)
	}

	// Create command
	// #nosec G204 - Run command is from trusted configuration, not user input
	p.cmd = exec.CommandContext(p.ctx, "/bin/sh", "-c", p.config.Run)

	// Set working directory if specified
	if p.config.WorkDir != "" {
		p.cmd.Dir = p.config.WorkDir
	}

	// Pass socket FDs via ExtraFiles and env vars.
	// ExtraFiles[0] = FD 3 (engine socket), ExtraFiles[1] = FD 4 (callback socket).
	p.cmd.ExtraFiles = []*os.File{pluginEngineFile, pluginCallbackFile}
	p.cmd.Env = append(os.Environ(), "ZE_ENGINE_FD=3", "ZE_CALLBACK_FD=4")

	p.stderr, err = p.cmd.StderrPipe()
	if err != nil {
		closeFiles(pluginEngineFile, pluginCallbackFile)
		pairs.Close()
		return err
	}

	// Apply process resource limits (platform-specific)
	p.cmd.SysProcAttr = newSysProcAttr()

	// Start process
	if err := p.cmd.Start(); err != nil {
		p.stderr.Close() //nolint:errcheck,gosec // cleanup on error
		closeFiles(pluginEngineFile, pluginCallbackFile)
		pairs.Close()
		return err
	}

	// After Start(), close plugin-side FD handles (subprocess inherited copies via ExtraFiles).
	closeFiles(pluginEngineFile, pluginCallbackFile)
	pairs.Engine.PluginSide.Close()   //nolint:errcheck,gosec // subprocess owns these now
	pairs.Callback.PluginSide.Close() //nolint:errcheck,gosec // subprocess owns these now

	// Store raw engine-side connections for protocol mode auto-detection.
	// PluginConns are created later by initConns() after peeking Socket A.
	p.sockets = pairs
	p.rawEngineA = pairs.Engine.EngineSide
	p.rawCallbackB = pairs.Callback.EngineSide

	p.running.Store(true)

	// Capture stderr and cmd locally before starting goroutines.
	// relayStderr and monitor both access p.stderr — capturing here avoids
	// a data race between relayStderr reading the field and monitor nil-ing it.
	// Similarly, cmd is captured so monitor() doesn't race on p.cmd.
	stderr := p.stderr
	cmd := p.cmd

	// Relay plugin stderr based on ze.log.relay setting.
	// Tracked by wg so Wait() blocks until relay drains.
	p.wg.Go(func() { p.relayStderrFrom(stderr) })

	// Monitor process — tracked by wg so Wait() blocks until cleanup completes.
	p.wg.Go(func() { p.monitorCmd(cmd) })

	return nil
}

// closeFiles closes os.File handles, ignoring nil values.
func closeFiles(files ...*os.File) {
	for _, f := range files {
		if f != nil {
			f.Close() //nolint:errcheck,gosec // best-effort cleanup
		}
	}
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
		// Parse the slog line and relay with subsystem=relay
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

// Stop terminates the process.
// For external plugins, canceling context kills the process via exec.CommandContext.
// For internal plugins, closing RPC connections unblocks the plugin's reads and causes it to exit.
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
		if p.engineConnA != nil {
			p.engineConnA.Close() //nolint:errcheck,gosec // best-effort shutdown
			p.engineConnA = nil
		}
		if p.engineConnB != nil {
			p.engineConnB.Close() //nolint:errcheck,gosec // best-effort shutdown
			p.engineConnB = nil
		}
		if p.textConnB != nil {
			p.textConnB.Close() //nolint:errcheck,gosec // best-effort shutdown
			p.textConnB = nil
		}
		if p.rawEngineA != nil {
			p.rawEngineA.Close() //nolint:errcheck,gosec // best-effort shutdown
			p.rawEngineA = nil
		}
		if p.rawCallbackB != nil {
			p.rawCallbackB.Close() //nolint:errcheck,gosec // best-effort shutdown
			p.rawCallbackB = nil
		}
		p.mu.Unlock()
	})
}

// SendShutdown sends a graceful shutdown signal (bye RPC) to the plugin.
// Returns true if the process was running. The bye RPC gives the plugin a
// chance to clean up before Stop() closes connections and kills the process.
// For text-mode plugins, sends "bye shutdown" as a plain text line.
func (p *Process) SendShutdown() bool {
	if !p.running.Load() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Text-mode: send "bye" line on textConnB.
	if tc := p.TextConnB(); tc != nil {
		_ = tc.WriteLine(ctx, "bye shutdown") //nolint:errcheck // best-effort graceful signal
		return true
	}

	// JSON-mode: send bye RPC on connB.
	connB := p.ConnB()
	if connB == nil {
		return true
	}
	_ = connB.SendBye(ctx, "shutdown") //nolint:errcheck // best-effort graceful signal
	return true
}

// Wait waits for the process to exit.
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
