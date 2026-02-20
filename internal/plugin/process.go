package plugin

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// stderrLogger is used for relaying plugin stderr to engine logs (lazy initialization).
// Tagged with subsystem=relay to distinguish from engine logs.
// Level controlled by ze.log.relay env var.
var stderrLogger = slogutil.LazyLogger("relay")

const (
	// RespawnLimit is max respawns per RespawnWindow before disabling.
	// ExaBGP: respawn_number=5 per ~63 seconds.
	RespawnLimit = 5

	// RespawnWindow is the time window for respawn limit tracking.
	RespawnWindow = 60 * time.Second
)

// Respawn errors.
var (
	ErrRespawnLimitExceeded = errors.New("respawn limit exceeded")
	ErrProcessDisabled      = errors.New("process disabled due to respawn limit")
	ErrProcessNotFound      = errors.New("process not found")
)

// EventDelivery represents a work item for the per-process delivery goroutine.
// The long-lived goroutine reads these from Process.eventChan and calls SendDeliverEvent.
type EventDelivery struct {
	Output string             // Pre-formatted event payload
	Result chan<- EventResult // Caller-provided result channel (nil if fire-and-forget)
}

// EventResult is sent back to the caller after delivery completes.
type EventResult struct {
	ProcName      string // Process name (for logging)
	Err           error  // nil on success
	CacheConsumer bool   // true if delivery succeeded AND process is a cache consumer
}

const (
	// eventDeliveryCapacity is the buffer size for per-process event delivery channels.
	// Each item is small (string + channel pointer). 64 provides headroom without
	// excessive memory use. If a plugin is slow, backpressure propagates naturally.
	eventDeliveryCapacity = 64
)

// Process represents an external subprocess.
type Process struct {
	config PluginConfig
	index  int // Plugin index for coordinator (0-based)
	cmd    *exec.Cmd

	stderr io.ReadCloser

	// Socket pairs for IPC (internal plugins use net.Pipe, external use socketpair)
	sockets *DualSocketPair

	// RPC connections for YANG RPC protocol (per-socket wiring).
	// Set for internal plugins (socket pairs) and external RPC plugins.
	engineConnA *PluginConn // Socket A: reads/writes plugin→engine RPCs
	engineConnB *PluginConn // Socket B: reads/writes engine→plugin callbacks

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
	stage        atomic.Int32        // Current stage (PluginStage)
	registration *PluginRegistration // Stage 1 registration data
	capabilities *PluginCapabilities // Stage 3 capability declarations
	stageCh      chan struct{}       // Signals stage completion
	stageMu      sync.Mutex          // Protects stage transitions

	// Long-lived event delivery goroutine (see rules/goroutine-lifecycle.md).
	// Events are enqueued via Deliver() and processed by deliveryLoop().
	// eventMu protects channel close: Deliver holds RLock, stopEventChan holds Lock.
	eventChan   chan EventDelivery
	eventClosed bool
	eventMu     sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// NewProcess creates a new process with the given configuration.
func NewProcess(config PluginConfig) *Process {
	return &Process{
		config:       config,
		registration: &PluginRegistration{},
		capabilities: &PluginCapabilities{},
		stageCh:      make(chan struct{}),
	}
}

// Stage returns the current plugin startup stage.
func (p *Process) Stage() PluginStage {
	return PluginStage(p.stage.Load())
}

// SetStage sets the current stage and notifies waiters.
func (p *Process) SetStage(stage PluginStage) {
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
func (p *Process) WaitForStage(ctx context.Context, stage PluginStage) error {
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
func (p *Process) Registration() *PluginRegistration {
	return p.registration
}

// Capabilities returns the plugin capability declarations (Stage 3).
func (p *Process) Capabilities() *PluginCapabilities {
	return p.capabilities
}

// Running returns true if the process is running.
func (p *Process) Running() bool {
	return p.running.Load()
}

// ConnA returns the plugin→engine RPC connection under the mutex.
// Returns nil if the connection has been closed (e.g. by Stop() or monitor()).
// Callers must check for nil before use to avoid racing with shutdown.
func (p *Process) ConnA() *PluginConn {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.engineConnA
}

// ConnB returns the engine→plugin callback connection under the mutex.
// Returns nil if the connection has been closed (e.g. by Stop() or monitor()).
// Callers must check for nil before use to avoid racing with shutdown.
func (p *Process) ConnB() *PluginConn {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.engineConnB
}

// SetConnB sets the engine→plugin callback connection.
// Used by test code to inject mock connections.
func (p *Process) SetConnB(conn *PluginConn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.engineConnB = conn
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
func (p *Process) WireEncodingIn() WireEncoding {
	// Safe: only values 0-3 are ever stored (WireEncodingHex..WireEncodingText).
	return WireEncoding(p.wireEncodingIn.Load()) //nolint:gosec // Bounded to 0-3
}

// WireEncodingOut returns the outbound wire encoding (commands Process→ze).
func (p *Process) WireEncodingOut() WireEncoding {
	// Safe: only values 0-3 are ever stored (WireEncodingHex..WireEncodingText).
	return WireEncoding(p.wireEncodingOut.Load()) //nolint:gosec // Bounded to 0-3
}

// SetWireEncodingIn sets the inbound wire encoding.
func (p *Process) SetWireEncodingIn(enc WireEncoding) {
	p.wireEncodingIn.Store(uint32(enc))
}

// SetWireEncodingOut sets the outbound wire encoding.
func (p *Process) SetWireEncodingOut(enc WireEncoding) {
	p.wireEncodingOut.Store(uint32(enc))
}

// SetWireEncoding sets both inbound and outbound wire encoding.
func (p *Process) SetWireEncoding(enc WireEncoding) {
	p.wireEncodingIn.Store(uint32(enc))
	p.wireEncodingOut.Store(uint32(enc))
}

// Encoding returns the high-level encoding (json or text).
func (p *Process) Encoding() string {
	if v := p.encoding.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return EncodingJSON // Default
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
	return FormatHex // Default
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

// Start spawns the process.
func (p *Process) Start() error {
	return p.StartWithContext(context.Background())
}

// Deliver enqueues an event for the long-lived delivery goroutine.
// Returns true if the event was enqueued, false if the process is stopping.
// Thread-safe: uses RLock to allow parallel sends from multiple callers.
func (p *Process) Deliver(d EventDelivery) bool {
	p.eventMu.RLock()
	defer p.eventMu.RUnlock()

	if p.eventClosed || p.eventChan == nil {
		return false
	}

	select {
	case p.eventChan <- d:
		return true
	case <-p.ctx.Done():
		return false
	}
}

// deliveryLoop is the long-lived goroutine that processes event deliveries.
// It reads from eventChan and calls SendDeliverEvent on ConnB.
// Exits when eventChan is closed (by stopEventChan during Stop).
func (p *Process) deliveryLoop() {
	for req := range p.eventChan {
		connB := p.ConnB()

		var result EventResult
		result.ProcName = p.config.Name

		if connB == nil {
			result.Err = errors.New("connection closed")
		} else {
			ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
			result.Err = connB.SendDeliverEvent(ctx, req.Output)
			cancel()
		}

		if result.Err == nil && p.IsCacheConsumer() {
			result.CacheConsumer = true
		}

		if req.Result != nil {
			req.Result <- result
		}
	}
}

// stopEventChan closes the event channel, causing deliveryLoop to drain and exit.
// Uses write lock to prevent concurrent Deliver calls from sending to a closed channel.
func (p *Process) stopEventChan() {
	p.eventMu.Lock()
	defer p.eventMu.Unlock()

	if !p.eventClosed && p.eventChan != nil {
		p.eventClosed = true
		close(p.eventChan)
	}
}

// startDeliveryLocked starts the event delivery goroutine.
// Caller must hold p.mu.
func (p *Process) startDeliveryLocked() {
	p.eventChan = make(chan EventDelivery, eventDeliveryCapacity)
	p.wg.Go(p.deliveryLoop)
}

// StartDelivery starts only the event delivery goroutine.
// Used by tests that inject connections via SetConnB without starting a real process.
func (p *Process) StartDelivery(ctx context.Context) {
	p.eventMu.Lock()
	if p.eventChan != nil {
		p.eventMu.Unlock()
		return
	}
	p.eventMu.Unlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ctx == nil {
		p.ctx, p.cancel = context.WithCancel(ctx)
	}

	p.startDeliveryLocked()
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
	runner := GetInternalPluginRunner(p.config.Name)
	if runner == nil {
		return fmt.Errorf("unknown internal plugin: %s", p.config.Name)
	}

	// Create socket pairs for bidirectional IPC
	pairs, err := NewInternalSocketPairs()
	if err != nil {
		return fmt.Errorf("creating socket pairs: %w", err)
	}
	p.sockets = pairs
	p.stderr = nil // Internal plugins don't have stderr
	p.running.Store(true)

	// Create per-socket PluginConns for YANG RPC protocol.
	// Per-socket wiring: each PluginConn reads+writes on the same socket,
	// which is compatible with the SDK's bidirectional per-socket pattern.
	p.engineConnA = NewPluginConn(pairs.Engine.EngineSide, pairs.Engine.EngineSide)
	p.engineConnB = NewPluginConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide)

	// Start the plugin in a goroutine
	// Plugin side: read from callback socket, write to engine socket
	p.wg.Go(func() {
		defer p.running.Store(false)
		defer func() { _ = pairs.Engine.PluginSide.Close() }()
		defer func() { _ = pairs.Callback.PluginSide.Close() }()

		_ = runner(pairs.Engine.PluginSide, pairs.Callback.PluginSide)
	})

	return nil
}

// startExternal starts an external plugin via exec.Command.
// All external plugins use YANG RPC protocol via inherited socket pair FDs
// (ZE_ENGINE_FD/ZE_CALLBACK_FD env vars).
func (p *Process) startExternal() error {
	// Create Unix socketpairs before starting the process.
	// These have real FDs that can be inherited by the subprocess via ExtraFiles.
	pairs, err := NewExternalSocketPairs()
	if err != nil {
		return fmt.Errorf("creating socket pairs: %w", err)
	}
	pluginEngineFile, pluginCallbackFile, err := pairs.PluginFiles()
	if err != nil {
		pairs.Close()
		return fmt.Errorf("getting plugin files: %w", err)
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

	// Create new process group
	p.cmd.SysProcAttr = nil // Default is fine for now

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

	// Create PluginConns on engine side for YANG RPC protocol.
	p.sockets = pairs
	p.engineConnA = NewPluginConn(pairs.Engine.EngineSide, pairs.Engine.EngineSide)
	p.engineConnB = NewPluginConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide)

	p.running.Store(true)

	// Relay plugin stderr based on ze.log.relay setting
	go p.relayStderr()

	// Monitor process
	p.wg.Add(1)
	go p.monitor()

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

// relayStderr reads plugin stderr and relays to engine logs.
// Plugin stderr format: time=... level=DEBUG msg="..." subsystem=gr ...
// When ze.log.relay=<level>, relays messages at or above that level.
// When disabled (empty/disabled), discards plugin stderr silently.
func (p *Process) relayStderr() {
	// Get configured relay level
	relayLevel, enabled := slogutil.RelayLevel()
	if !enabled {
		// Discard all stderr when relay disabled
		scanner := bufio.NewScanner(p.stderr)
		for scanner.Scan() {
			// Read but discard
		}
		return
	}

	scanner := bufio.NewScanner(p.stderr)
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

	// Close connections to unblock pending reads and writes.
	// Internal plugins: closing connections is the primary stop mechanism.
	// External RPC plugins: context cancellation kills the process via exec.CommandContext;
	// closing connections unblocks any goroutines waiting on socket I/O.
	p.mu.Lock()
	if p.engineConnA != nil {
		p.engineConnA.Close() //nolint:errcheck,gosec // best-effort shutdown
		p.engineConnA = nil
	}
	if p.engineConnB != nil {
		p.engineConnB.Close() //nolint:errcheck,gosec // best-effort shutdown
		p.engineConnB = nil
	}
	p.mu.Unlock()
}

// SendShutdown sends a graceful shutdown signal (bye RPC) to the plugin.
// Returns true if the process was running. The bye RPC gives the plugin a
// chance to clean up before Stop() closes connections and kills the process.
func (p *Process) SendShutdown() bool {
	if !p.running.Load() {
		return false
	}
	connB := p.ConnB()
	if connB == nil {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = connB.SendBye(ctx, "shutdown") //nolint:errcheck // best-effort graceful signal
	return true
}

// Wait waits for the process to exit.
func (p *Process) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// monitor waits for the process to exit.
func (p *Process) monitor() {
	defer p.wg.Done()

	// Wait for process to exit
	_ = p.cmd.Wait()

	p.running.Store(false)

	// Cancel context. Safe even if Stop() already canceled it (cancel is idempotent).
	if p.cancel != nil {
		p.cancel()
	}

	// Close stderr pipe and RPC connections.
	// Nil after close to prevent double-close if Stop() races with monitor().
	p.mu.Lock()
	if p.stderr != nil {
		_ = p.stderr.Close()
		p.stderr = nil
	}
	if p.engineConnA != nil {
		_ = p.engineConnA.Close()
		p.engineConnA = nil
	}
	if p.engineConnB != nil {
		_ = p.engineConnB.Close()
		p.engineConnB = nil
	}
	p.mu.Unlock()
}

// ProcessManager manages multiple external processes.
type ProcessManager struct {
	configs   []PluginConfig
	processes map[string]*Process

	// Respawn tracking: name -> list of respawn timestamps
	respawnTimes map[string][]time.Time

	// Disabled processes (respawn limit exceeded)
	disabled map[string]bool

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
}

// NewProcessManager creates a new process manager.
func NewProcessManager(configs []PluginConfig) *ProcessManager {
	return &ProcessManager{
		configs:      configs,
		processes:    make(map[string]*Process),
		respawnTimes: make(map[string][]time.Time),
		disabled:     make(map[string]bool),
	}
}

// GetProcess returns a process by name, or nil if not found.
func (pm *ProcessManager) GetProcess(name string) *Process {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.processes[name]
}

// Start starts all configured processes.
func (pm *ProcessManager) Start() error {
	return pm.StartWithContext(context.Background())
}

// StartWithContext starts all configured processes with the given context.
func (pm *ProcessManager) StartWithContext(ctx context.Context) error {
	pm.ctx, pm.cancel = context.WithCancel(ctx)

	for i, cfg := range pm.configs {
		proc := NewProcess(cfg)
		proc.index = i // Set plugin index for coordinator
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
	var cfg *PluginConfig
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

	// Check limit
	if len(validTimes) >= RespawnLimit {
		pm.disabled[name] = true
		fmt.Fprintf(os.Stderr, "PROCESS %s: respawn limit exceeded (%d in %v), process disabled\n",
			name, RespawnLimit, RespawnWindow)
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

	// Start new process
	newProc := NewProcess(*cfg)
	if err := newProc.StartWithContext(pm.ctx); err != nil {
		return err
	}
	pm.processes[name] = newProc

	return nil
}
