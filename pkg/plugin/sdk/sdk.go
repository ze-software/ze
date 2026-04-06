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

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
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
	onConfigure     func([]ConfigSection) error
	onShareRegistry func([]RegistryCommand)
	onStarted       func(context.Context) error

	// Direct delivery handlers for bridge hot path (bypasses callback channel).
	// onEvent is also captured by the deliver-event/deliver-batch map entries.
	onEvent           func(string) error
	onStructuredEvent func([]any) error

	// Startup subscriptions: included in the "ready" RPC so the engine
	// registers them atomically before SignalAPIReady, avoiding the race
	// between reactor sending routes and the plugin subscribing.
	startupSubscription *rpc.SubscribeEventsInput

	// Capabilities to declare during Stage 3.
	capabilities []CapabilityDecl

	// Listen sockets received from engine via SCM_RIGHTS during startup.
	// Populated automatically between Stage 1 and Stage 2 when connection-handlers
	// are declared. Access via Listeners() after startup.
	listeners []net.Listener

	mu sync.Mutex
}

// NewWithConn creates a plugin with a single bidirectional connection.
// MuxConn is created immediately for bidirectional RPC multiplexing.
// For internal plugins, conn may be a BridgedConn carrying a DirectBridge
// reference for post-startup direct transport.
func NewWithConn(name string, conn net.Conn) *Plugin {
	rc := rpc.NewConn(conn, conn)
	p := &Plugin{
		name:       name,
		engineConn: rc,
		engineMux:  rpc.NewMuxConn(rc),
	}
	p.initCallbackDefaults()
	// Discover bridge via type assertion (internal plugins only).
	if bridger, ok := conn.(rpc.Bridger); ok {
		p.bridge = bridger.Bridge()
	}
	return p
}

// NewWithIO creates a plugin from separate reader and writer streams.
// Use this for non-TCP transports (SSH channels, stdin/stdout pipes) where
// a net.Conn is not available. MuxConn is created immediately for
// bidirectional RPC multiplexing.
func NewWithIO(name string, reader io.ReadCloser, writer io.WriteCloser) *Plugin {
	rc := rpc.NewConn(reader, writer)
	p := &Plugin{
		name:       name,
		engineConn: rc,
		engineMux:  rpc.NewMuxConn(rc),
	}
	p.initCallbackDefaults()
	return p
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

// NewFromTLSEnv creates a plugin by reading ze.plugin.hub.host, ze.plugin.hub.port,
// ze.plugin.hub.token, and ze.plugin.cert.fp env vars (dot or underscore notation).
// Connects to the engine via TLS, authenticates, and returns a single-conn plugin.
// ze.plugin.hub.host defaults to 127.0.0.1, ze.plugin.hub.port defaults to 12700.
// ze.plugin.hub.token is required.
// If ze.plugin.cert.fp is set, the TLS handshake verifies the server cert fingerprint.
func NewFromTLSEnv(name string) (*Plugin, error) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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

	// Pass the existing rpc.Conn to MuxConn (reuses reader, no new goroutine).
	engineMux := rpc.NewMuxConn(engineConn)
	return &Plugin{
		name:       name,
		engineConn: engineConn,
		engineMux:  engineMux,
	}, nil
}

// Listeners returns listen sockets received from the engine during startup.
// Returns nil if no connection-handlers were declared.
func (p *Plugin) Listeners() []net.Listener {
	return p.listeners
}

// Close closes the underlying connections and any received listeners,
// unblocking any goroutines waiting on Read(). Must be called when the
// plugin is done to prevent goroutine and socket leaks.
// Safe to call multiple times.
func (p *Plugin) Close() error {
	// Close received listeners first -- they hold open TCP sockets.
	for _, ln := range p.listeners {
		ln.Close() //nolint:errcheck,gosec // best-effort cleanup of handed-off listeners
	}
	p.listeners = nil

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
