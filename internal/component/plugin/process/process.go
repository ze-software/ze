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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/syncutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var errNoRawConnectionAvailable = errors.New("no raw connection available")

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
	envelope       atomic.Bool  // true: wrap delivered events in an EventEnvelope (default: bare payload)

	// Registered plugin commands (tracked for cleanup on death)
	registeredCommands []string
	registeredMu       sync.Mutex

	// Plugin registration protocol (5-stage startup)
	stage        atomic.Int32               // Current stage (PluginStage)
	registration *plugin.PluginRegistration // Stage 1 registration data
	capabilities *plugin.PluginCapabilities // Stage 3 capability declarations
	stageCh      chan struct{}              // Signals stage completion
	stageMu      sync.Mutex                 // Protects stage transitions

	// startupFail records WHY the 5-stage handshake stopped before
	// StageRunning, so the engine can report the cause instead of only the
	// stage it stopped at. See SetStartupError.
	startupFail atomic.Pointer[startupFailure]

	// Metrics callbacks (set by ProcessManager, nil when metrics disabled).
	onStageChange func(plugin.PluginStage) // Called after stage transition
	deliveryInc   func()                   // Called after successful Deliver enqueue

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

// startupFailure boxes the cause of a failed 5-stage handshake. The box exists
// because atomic.Value panics when successive Stores carry different concrete
// types, and startup causes are arbitrary error implementations (*fmt.wrapError,
// *errors.errorString, syscall.Errno, ...). An atomic.Pointer to this struct
// stores any of them without that hazard.
type startupFailure struct{ err error }

// SetStartupError records why this plugin's 5-stage startup handshake ended
// before StageRunning. The engine reports startup failure from proc.Stage(),
// which says WHERE the handshake stopped but not WHY; without this the cause
// (e.g. `iface: create dummy "zdiag0": operation not permitted`) was logged at
// Debug and dropped, and ze exited with only "plugin interface failed during
// startup at stage Config" -- an error with no evidence and no corrective
// action (ai/rules/cli.md).
//
// Only the FIRST cause is kept: it is the one that actually stopped startup,
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
// handshake never failed or failed without a reported cause. Callers MUST treat
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
// If an onStageChange callback is set (by ProcessManager metrics), it is called
// after the stage is stored and waiters are notified, outside the lock.
func (p *Process) SetStage(stage plugin.PluginStage) {
	var cb func(plugin.PluginStage)
	p.stageMu.Lock()
	// Safe: PluginStage has only values 0-6 (StageInit..StageRunning).
	p.stage.Store(int32(stage)) //nolint:gosec // G115: bounded enum
	cb = p.onStageChange
	// Close and recreate channel to notify all waiters
	close(p.stageCh)
	p.stageCh = make(chan struct{})
	p.stageMu.Unlock()
	// Call metrics callback outside the lock to avoid holding stageMu
	// during Prometheus internal lock acquisition.
	if cb != nil {
		cb(stage)
	}
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
		return errNoRawConnectionAvailable
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
	// Use the Run value as the runner lookup name. For "use" configs, Run is
	// the bare plugin name (e.g., "bgp-rib"). For "run" configs, Run is a
	// command string (e.g., "ze.bgp-rs" or "ze plugin bgp-rs") that needs
	// resolution. Try direct registry lookup first, then ResolvePlugin.
	if p.config.Run != "" {
		if plugin.IsInternalPlugin(p.config.Run) {
			name = p.config.Run
		} else if res, err := plugin.ResolvePlugin(p.config.Run); err == nil && res.Type == plugin.PluginTypeInternal {
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

// execBinDir returns the directory of the running binary, preferring the
// argv[0] path (which preserves symlinks) over os.Executable (which resolves
// them). This matters when ze runs from a symlink in a QEMU VM: the resolved
// path can land in a directory that also contains a host-architecture binary.
func execBinDir() string {
	if arg0 := os.Args[0]; filepath.IsAbs(arg0) {
		return filepath.Dir(arg0)
	} else if abs, err := filepath.Abs(arg0); err == nil {
		return filepath.Dir(abs)
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	return ""
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
	// Use os.Args[0] instead of os.Executable(): the latter follows symlinks,
	// which can resolve to a directory containing a different-architecture
	// binary with the same name (QEMU 9p mount with both host and VM binaries).
	p.cmd.Env = os.Environ()
	if binDir := execBinDir(); binDir != "" {
		p.cmd.Env = append(p.cmd.Env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
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
		"ZE_PLUGIN_CERT_FP="+p.acceptor.CertFP(),
		"ZE_PLUGIN_NAME="+p.config.Name,
	)

	stderrRead, stderrWrite, pipeErr := attachStderrRelay(p.cmd)
	if pipeErr != nil {
		return fmt.Errorf("plugin %s: stderr pipe: %w", p.config.Name, pipeErr)
	}
	p.stderr = stderrRead

	p.cmd.SysProcAttr = newSysProcAttr()

	if err := p.cmd.Start(); err != nil {
		stderrWrite.Close() //nolint:errcheck,gosec // cleanup on error
		stderrRead.Close()  //nolint:errcheck,gosec // cleanup on error
		return fmt.Errorf("plugin %s: start: %w", p.config.Name, err)
	}
	// The child holds its own descriptor now. Closing the parent's copy of the
	// write end is what turns the child's exit into EOF for the reader below.
	if closeErr := stderrWrite.Close(); closeErr != nil {
		logger().Debug("close plugin stderr write end", "plugin", p.config.Name, "error", closeErr)
	}

	p.running.Store(true)

	cmd := p.cmd
	relayDone := make(chan struct{})
	p.wg.Go(func() {
		defer close(relayDone)
		p.relayStderrFrom(stderrRead)
	})
	p.wg.Go(func() { p.monitorCmd(cmd, relayDone, stderrRead) })

	// Wait for the child to connect back via TLS (bounded timeout).
	waitCtx, waitCancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer waitCancel()

	conn, waitErr := p.acceptor.WaitForPlugin(waitCtx, p.config.Name)
	if waitErr != nil {
		// Kill the child process to prevent orphaning.
		if p.cmd != nil && p.cmd.Process != nil {
			p.cmd.Process.Kill() //nolint:errcheck,gosec // cleanup on connect-back failure
		}
		// Stop delivery goroutine started by startDeliveryLocked to prevent leak.
		p.stopEventChan()
		return fmt.Errorf("plugin %s: TLS connect-back: %w", p.config.Name, waitErr)
	}
	p.rawConn = conn

	return nil
}

// stderrDrainGrace bounds how long monitorCmd waits, after the plugin process
// has been reaped, for the relay to finish reading what that process wrote.
//
// EOF arrives the moment the last writer closes the pipe, which is normally the
// plugin's own exit, so the wait is microseconds. The bound covers the one case
// where it is not: a plugin that spawned a child of its own, which inherited the
// descriptor and outlives it. Without a bound that goroutine would hold Wait
// open for as long as the grandchild lives.
const stderrDrainGrace = 2 * time.Second

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

// drainStderrRelay releases the read end of a plugin's stderr pipe: as soon as
// the relay goroutine has read it to EOF, or after grace when EOF never comes.
// Closing it unblocks a relay still waiting on a descriptor a grandchild holds.
func drainStderrRelay(relayDone <-chan struct{}, read io.Closer, grace time.Duration) {
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-relayDone:
	case <-timer.C:
	}
	if err := read.Close(); err != nil {
		logger().Debug("close plugin stderr read end", "error", err)
	}
}

// relayStderrFrom reads plugin stderr and relays to engine logs.
// Plugin stderr format: time=... level=DEBUG msg="..." subsystem=gr ...
// When ze.log.relay=<level>, relays messages at or above that level.
// When disabled (empty/disabled), discards plugin stderr silently.
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
	if !enabled {
		// Discard all stderr when relay disabled. A scan that ends early loses
		// nothing here: discarding is the contract, so there is no error to
		// read back.
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			// Read but discard
		}
		return
	}

	var inPanic bool
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
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
		args := []any{"plugin", p.config.Name}
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
	if err := scanner.Err(); err != nil {
		stderrLogger().Error("plugin stderr relay stopped, later plugin output is not relayed",
			"plugin", p.config.Name, "err", err)
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

// monitorCmd waits for the process to exit.
// Takes cmd as a parameter to avoid racing on p.cmd with other goroutines.
//
// relayDone closes when relayStderrFrom returns, and stderr is the read end that
// goroutine is reading. Both are parameters for the same reason cmd is.
func (p *Process) monitorCmd(cmd *exec.Cmd, relayDone <-chan struct{}, stderr io.Closer) {
	// Wait for process to exit
	_ = cmd.Wait()

	p.running.Store(false)

	// Cancel context. Safe even if Stop() already canceled it (cancel is idempotent).
	if p.cancel != nil {
		p.cancel()
	}

	// Close RPC connections via sync.Once — safe if Stop() races with monitor.
	p.closeConns()

	// The stderr pipe is this package's, not the Cmd's, so Wait above did not
	// close it and the relay still holds every byte the plugin wrote on its way
	// out. Release the descriptor only once the relay has read them, which is
	// what keeps a plugin's last line -- its verdict, or its panic block.
	drainStderrRelay(relayDone, stderr, stderrDrainGrace)
}
