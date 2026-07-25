// Design: docs/architecture/api/process-protocol.md — plugin SDK
// Detail: sdk_callbacks.go — On*/Set* callback registration methods
// Detail: sdk_engine.go — plugin-to-engine RPC methods
// Detail: sdk_dispatch.go — event loop and callback dispatch
// Detail: sdk_types.go — re-exported RPC type aliases
// Detail: union.go — event stream correlation
// Related: ../../../internal/component/plugin/ipc/tls.go — TLS transport and auth (SendAuth, PluginAcceptor)
// Related: ../../../internal/component/plugin/process/process.go — engine-side process lifecycle (startExternal forks + WaitForPlugin)
//
// Package sdk provides a high-level SDK for creating ze plugins using the
// YANG RPC protocol over a single bidirectional connection.
//
// Plugins communicate with the ze engine via a single connection (net.Pipe
// for internal plugins, TLS for external). MuxConn multiplexes bidirectional
// RPCs by distinguishing responses (verb=ok/error) from requests (verb=method).
//
// The SDK handles the 5-stage startup protocol and event loop automatically.
//
// Basic usage:
//
//	p := sdk.NewFromEnv("my-plugin")
//	p.OnEvent(func(event string) error { ... })
//	p.OnConfigure(func(sections []sdk.ConfigSection) error { ... })
//	p.Run(ctx, sdk.Registration{
//	    Families: []sdk.FamilyDecl{{Name: "ipv4/flow", Mode: "both"}},
//	})
package sdk

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// callbackHandler is the uniform signature for all runtime callback handlers.
// Returns result JSON (nil for status-only OK responses) and error.
// Registered via On* methods, dispatched by both pipe and bridge event loops.
type callbackHandler func(json.RawMessage) (json.RawMessage, error)

// Plugin represents a ze plugin using the YANG RPC protocol.
type Plugin struct {
	name string

	// Single bidirectional connection, multiplexed for concurrent RPCs.
	engineConn *rpc.Conn    // Underlying connection (reads/writes)
	engineMux  *rpc.MuxConn // Multiplexed for concurrent RPCs + inbound request routing

	// Direct transport bridge for internal plugins (nil for external).
	// Discovered via type assertion on conn in NewWithConn.
	// After startup, DeliverEvents bypasses the connection and callEngineRaw
	// dispatches through bridge.DispatchRPC instead of engineMux.CallRPC.
	bridge *rpc.DirectBridge

	// Runtime callback registry: method name -> handler.
	// On* methods register typed wrappers here. Both event loops dispatch
	// through this map -- no switch statements, no transport-specific handlers.
	// Adding a new callback = adding one On* method. Zero dispatch changes.
	callbacks map[string]callbackHandler

	// Startup-only callbacks (stages 2, 4, post-startup). Not in the map
	// because they run during the sequential startup protocol, not the event loop.
	onConfigure     ConfigureHandler
	onShareRegistry ShareRegistryHandler
	onStarted       StartedHandler

	// onAllPluginsReady fires via the event loop after the engine sends the
	// post-startup callback. Unlike onStarted (which runs synchronously before
	// the event loop and therefore before other phases may have loaded their
	// plugins), onAllPluginsReady runs only after every plugin across every
	// startup phase is running and both registries are frozen. This is the
	// only safe place to dispatch commands to other plugins during startup.
	onAllPluginsReady AllPluginsReadyHandler

	// Direct delivery handlers for bridge hot path (bypasses callback channel).
	// onEvent is also captured by the deliver-event/deliver-batch map entries.
	onEvent           EventHandler
	onStructuredEvent StructuredEventHandler
	onExecuteCommand  ExecuteCommandHandler

	// Startup subscriptions: included in the "ready" RPC so the engine
	// registers them atomically before SignalAPIReady, avoiding the race
	// between reactor sending routes and the plugin subscribing.
	startupSubscription *rpc.SubscribeEventsInput

	// Capabilities to declare during Stage 3.
	capabilities []CapabilityDecl

	mu sync.Mutex
}

// newPlugin allocates a Plugin around an already-constructed rpc.Conn and
// installs default callback handlers. All public constructors route through
// this helper so that adding a new constructor cannot forget to initialize
// the callbacks map (a nil map panics the first On* call). See known-failures
// entry "SDK NewFromTLSEnv missing initCallbackDefaults" for the original bug.
func newPlugin(name string, rc *rpc.Conn) *Plugin {
	p := &Plugin{
		name:       name,
		engineConn: rc,
		engineMux:  rpc.NewMuxConn(rc),
	}
	p.initCallbackDefaults()
	return p
}

// NewWithConn creates a plugin with a single bidirectional connection.
// MuxConn is created immediately for bidirectional RPC multiplexing.
// For internal plugins, conn may be a BridgedConn carrying a DirectBridge
// reference for post-startup direct transport.
func NewWithConn(name string, conn net.Conn) *Plugin {
	p := newPlugin(name, rpc.NewConn(conn, conn))
	// Discover bridge via type assertion (internal plugins only).
	if bridger, ok := conn.(rpc.Bridger); ok {
		p.bridge = bridger.Bridge()
	}
	return p
}

// IsInternal reports whether this plugin instance is running in-process with
// the engine (a DirectBridge was discovered on conn in NewWithConn) as
// opposed to a forked subprocess talking over TLS. True exactly when the
// engine started this plugin via the "internal" invocation mode (process.go's
// startInternal, which wraps the net.Pipe() end with rpc.NewBridgedConn).
//
// A plugin that calls another in-process package's plain Go functions
// directly -- bypassing DirectBridge/DispatchCommand, e.g. as112 calling
// iface.RegisterOwnedAddresses -- MUST check this and refuse to start if
// false. Such a call is syntactically valid and returns no error when run
// external: it silently operates on the subprocess's own copy of the
// target package's state, never the engine process's.
func (p *Plugin) IsInternal() bool {
	return p.bridge != nil
}

// NewWithIO creates a plugin from separate reader and writer streams.
// Use this for non-TCP transports (SSH channels, stdin/stdout pipes) where
// a net.Conn is not available. MuxConn is created immediately for
// bidirectional RPC multiplexing.
func NewWithIO(name string, reader io.ReadCloser, writer io.WriteCloser) *Plugin {
	return newPlugin(name, rpc.NewConn(reader, writer))
}

// NewFromEnv creates a plugin by reading ZE_PLUGIN_HUB_HOST, ZE_PLUGIN_HUB_PORT, and
// ZE_PLUGIN_HUB_TOKEN environment variables. Connects to the engine via TLS.
func NewFromEnv(name string) (*Plugin, error) {
	return NewFromTLSEnv(name)
}

// Env var registrations for plugin transport.
var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.plugin.hub.host", Type: "string", Default: "127.0.0.1", Description: "TLS host for plugin-to-engine connection"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.plugin.hub.port", Type: "string", Default: "12700", Description: "TLS port for plugin-to-engine connection"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.plugin.hub.token", Type: "string", Description: "Auth token for plugin-to-engine TLS (required for external plugins)", Private: true, Secret: true})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.plugin.cert.fp", Type: "string", Description: "SHA-256 fingerprint of engine TLS cert for pinning"})
)

// Default plugin transport address (matches hub config default listen address).
const (
	DefaultPluginHost = "127.0.0.1"
	DefaultPluginPort = "12700"
)

// dialAndAuth reads the ze.plugin.hub.* env vars, dials the engine hub via
// TLS, and sends the auth request -- the portion of the TLS bootstrap shared
// by NewFromTLSEnv (which wraps the result in rpc.Conn for immediate SDK
// use) and DialTLSEnvRaw (which hands back the still-unwrapped net.Conn for
// a caller that builds its own rpc.Conn later). The auth RESPONSE is not
// read here; each caller reads it its own way.
func dialAndAuth(ctx context.Context, name string) (net.Conn, error) {
	host := env.Get("ze.plugin.hub.host")
	if host == "" {
		host = DefaultPluginHost
	}
	port := env.Get("ze.plugin.hub.port")
	if port == "" {
		port = DefaultPluginPort
	}
	token := env.Get("ze.plugin.hub.token")
	if token == "" {
		return nil, fmt.Errorf("ze.plugin.hub.token must be set")
	}

	certFP := env.Get("ze.plugin.cert.fp")

	addr := net.JoinHostPort(host, port)
	tlsConf := ipc.TLSConfigWithFingerprint(certFP)

	conn, err := (&tls.Dialer{Config: tlsConf}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("TLS dial %s: %w", addr, err)
	}

	// Disable Nagle's algorithm for plugin IPC. Plugin RPCs are
	// small request-response messages; Nagle adds latency without
	// batching benefit.
	if tc, ok := conn.(interface{ NetConn() net.Conn }); ok {
		if tcp, ok := tc.NetConn().(*net.TCPConn); ok {
			_ = tcp.SetNoDelay(true)
		}
	}

	// Send auth request directly (no rpc.Conn to avoid reader goroutine leak).
	if authErr := ipc.SendAuth(ctx, conn, token, name); authErr != nil {
		conn.Close() //nolint:errcheck,gosec // cleanup on auth failure
		return nil, fmt.Errorf("auth: %w", authErr)
	}

	return conn, nil
}

// NewFromTLSEnv creates a plugin by reading ze.plugin.hub.host, ze.plugin.hub.port,
// ze.plugin.hub.token, and ze.plugin.cert.fp env vars (dot or underscore notation).
// Connects to the engine via TLS, authenticates, and returns a single-conn plugin.
// ze.plugin.hub.host defaults to 127.0.0.1, ze.plugin.hub.port defaults to 12700.
// ze.plugin.hub.token is required.
// If ze.plugin.cert.fp is set, the TLS handshake verifies the server cert fingerprint.
func NewFromTLSEnv(name string) (*Plugin, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := dialAndAuth(ctx, name)
	if err != nil {
		return nil, err
	}

	// Create ONE rpc.Conn for this connection. ReadRequest starts the
	// persistent reader. MuxConn reuses the same reader via sync.Once
	// -- no competing goroutines.
	engineConn := rpc.NewConn(conn, conn)
	resp, readErr := engineConn.ReadRequest(ctx)
	if readErr != nil {
		conn.Close() //nolint:errcheck,gosec // cleanup on read failure
		return nil, fmt.Errorf("read auth response: %w", readErr)
	}
	if resp.Method == "error" {
		conn.Close() //nolint:errcheck,gosec // cleanup on auth rejection
		return nil, fmt.Errorf("auth rejected: %s", string(resp.Params))
	}

	return newPlugin(name, engineConn), nil
}

// DialTLSEnvRaw performs the same TLS dial + auth handshake as
// NewFromTLSEnv (same env vars, same defaults) but returns the raw,
// still-unwrapped net.Conn instead of an already-initialized *Plugin. The
// auth response is read via ipc.ReadLineRaw (byte-by-byte, no bufio
// buffering-ahead -- see ipc.Authenticate's doc comment for why this
// matters) instead of wrapping the connection in rpc.Conn, so the returned
// conn is left perfectly clean for a caller that builds its own *Plugin via
// NewWithConn later (e.g. a registry.Registration.RunEngine(conn net.Conn)
// func, which every plugin already implements this way for its INTERNAL
// invocation path).
//
// Test-only today: internal/test/cli's `ze-test plugin-external <name>`
// command is the only caller, launching a registered engine plugin's own
// RunEngine as a genuine external subprocess to prove its
// IsInternal()-guarded refuse/warn behavior actually fires outside a
// synthetic net.Pipe() unit test (plan/learned/1045-plugin-process-boundary.md).
// Production external plugins should be built as standalone binaries using
// pkg/plugin (see examples/plugin/go/main.go), not this generic launcher.
func DialTLSEnvRaw(name string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := dialAndAuth(ctx, name)
	if err != nil {
		return nil, err
	}

	if deadline, ok := ctx.Deadline(); ok {
		if derr := conn.SetReadDeadline(deadline); derr != nil {
			conn.Close() //nolint:errcheck,gosec // cleanup on error path
			return nil, fmt.Errorf("set auth-response deadline: %w", derr)
		}
	}
	line, err := ipc.ReadLineRaw(conn, ipc.MaxAuthFrameSize)
	if err != nil {
		conn.Close() //nolint:errcheck,gosec // cleanup on read failure
		return nil, fmt.Errorf("read auth response: %w", err)
	}
	if clearErr := conn.SetReadDeadline(time.Time{}); clearErr != nil {
		conn.Close() //nolint:errcheck,gosec // cleanup on error path
		return nil, fmt.Errorf("clear auth-response deadline: %w", clearErr)
	}

	_, verb, payload, parseErr := rpc.ParseLine(line)
	if parseErr != nil {
		conn.Close() //nolint:errcheck,gosec // cleanup on parse failure
		return nil, fmt.Errorf("parse auth response: %w", parseErr)
	}
	if verb == "error" {
		conn.Close() //nolint:errcheck,gosec // cleanup on auth rejection
		return nil, fmt.Errorf("auth rejected: %s", rpc.ExtractErrorMessage(payload))
	}

	return conn, nil
}

// Close closes the underlying connections, unblocking any goroutines waiting
// on Read(). Must be called when the plugin is done to prevent goroutine and
// socket leaks. Safe to call multiple times.
func (p *Plugin) Close() error {
	// Close MuxConn first -- its background reader must stop before
	// closing the underlying engineConn (which it reads from).
	if p.engineMux != nil {
		if err := p.engineMux.Close(); err != nil {
			return err
		}
	}
	return p.engineConn.Close()
}

// Run executes the 5-stage startup protocol and enters the event loop.
// Returns nil on clean shutdown (bye received), or error on failure.
func (p *Plugin) Run(ctx context.Context, reg Registration) error {
	// Auto-set WantsValidateOpen if callback is registered.
	p.mu.Lock()
	if p.callbacks[callbackValidateOpen] != nil {
		reg.WantsValidateOpen = true
	}
	p.mu.Unlock()

	// Stage 1: declare-registration
	if err := p.callEngine(ctx, "ze-plugin-engine:declare-registration", &reg); err != nil {
		return fmt.Errorf("stage 1 (declare-registration): %w", err)
	}

	// Stage 2: wait for configure from engine
	if err := p.serveOne(ctx, "ze-plugin-callback:configure", p.handleConfigure); err != nil {
		return fmt.Errorf("stage 2 (configure): %w", err)
	}

	// Stage 3: declare-capabilities
	p.mu.Lock()
	caps := &DeclareCapabilitiesInput{Capabilities: p.capabilities}
	p.mu.Unlock()

	if err := p.callEngine(ctx, "ze-plugin-engine:declare-capabilities", caps); err != nil {
		return fmt.Errorf("stage 3 (declare-capabilities): %w", err)
	}

	// Stage 4: wait for share-registry from engine
	if err := p.serveOne(ctx, "ze-plugin-callback:share-registry", p.handleShareRegistry); err != nil {
		return fmt.Errorf("stage 4 (share-registry): %w", err)
	}

	// Stage 5: ready (with optional startup subscriptions and transport negotiation)
	p.mu.Lock()
	readyInput := &rpc.ReadyInput{}
	if p.startupSubscription != nil {
		readyInput.Subscribe = p.startupSubscription
	}
	if p.bridge != nil {
		readyInput.Transport = "bridge"
	}
	p.mu.Unlock()

	if err := p.callEngine(ctx, "ze-plugin-engine:ready", readyInput); err != nil {
		// Connection closed during stage 5 is a clean shutdown: the engine
		// received the ready request and may have closed the pipe before
		// the write-deadline clear completes on this side.
		if isConnectionClosed(err) {
			return nil
		}
		return fmt.Errorf("stage 5 (ready): %w", err)
	}

	// Activate direct transport bridge if discovered during construction.
	// Register the plugin's event handler so the engine can call it directly
	// instead of going through the connection. Signal ready so the engine side
	// switches from SendDeliverBatch to bridge.DeliverEvents.
	if p.bridge != nil {
		p.mu.Lock()
		onEventFn := p.onEvent
		onStructuredFn := p.onStructuredEvent
		onExecuteFn := p.onExecuteCommand
		p.mu.Unlock()

		p.bridge.SetDeliverEvents(func(events []string) error {
			if onEventFn == nil {
				return nil
			}
			for _, event := range events {
				if err := onEventFn(event); err != nil {
					return err
				}
			}
			return nil
		})
		if onStructuredFn != nil {
			p.bridge.SetDeliverStructured(onStructuredFn)
		}
		if onExecuteFn != nil {
			p.bridge.SetExecuteCommand(func(serial, command string, args []string, peer string) (*rpc.ExecuteCommandOutput, error) {
				return executeCommandOutput(onExecuteFn, serial, command, args, peer)
			})
		}
		p.bridge.SetReady()
	}

	// Post-startup: safe to make engine calls.
	// The engine's runtime handler starts reading after all plugins
	// complete startup, so writes are buffered briefly then handled.
	p.mu.Lock()
	startedFn := p.onStarted
	p.mu.Unlock()

	if startedFn != nil {
		if err := startedFn(ctx); err != nil {
			return fmt.Errorf("post-startup: %w", err)
		}
	}

	// Enter event loop.
	if p.bridge != nil {
		// Bridge mode: close the pipe (MuxConn readLoop exits), run bridge-only loop.
		// All engine->plugin callbacks arrive via bridge.CallbackCh().
		_ = p.engineMux.Close()
		return p.bridgeEventLoop(ctx)
	}
	// Pipe mode (external plugins): callbacks arrive via MuxConn.Requests().
	return p.eventLoop(ctx)
}

// callEngine sends an RPC to the engine and waits for response.
// Dispatches via DirectBridge (internal) or MuxConn (external).
func (p *Plugin) callEngine(ctx context.Context, method string, params any) error {
	_, err := p.callEngineRaw(ctx, method, params)
	return err
}

// callEngineWithResult sends an RPC to the engine and returns the result payload.
func (p *Plugin) callEngineWithResult(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return p.callEngineRaw(ctx, method, params)
}

// callEngineRaw sends an RPC and returns the result payload.
// Dispatches to: DirectBridge (internal plugins post-startup) or MuxConn.
func (p *Plugin) callEngineRaw(ctx context.Context, method string, params any) (json.RawMessage, error) {
	// Direct bridge path: bypass JSON framing and socket I/O entirely.
	// Params are still marshaled to json.RawMessage for the bridge handler.
	if p.bridge != nil && p.bridge.Ready() {
		var paramsRaw json.RawMessage
		if params != nil {
			var err error
			paramsRaw, err = json.Marshal(params)
			if err != nil {
				return nil, fmt.Errorf("marshal params: %w", err)
			}
		}
		return p.bridge.DispatchRPC(method, paramsRaw)
	}
	return p.engineMux.CallRPC(ctx, method, params)
}

// --- Callback handlers ---

// --- RPC Types (aliases to canonical types in pkg/plugin/rpc) ---
