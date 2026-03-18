// Design: docs/architecture/api/process-protocol.md — plugin SDK
// Detail: sdk_callbacks.go — On*/Set* callback registration methods
// Detail: sdk_engine.go — plugin-to-engine RPC methods
// Detail: sdk_dispatch.go — event loop and callback dispatch
// Detail: sdk_types.go — re-exported RPC type aliases
//
// Package sdk provides a high-level SDK for creating ze plugins using the
// YANG RPC protocol over dual socket pairs.
//
// Plugins communicate with the ze engine via two Unix sockets:
//   - Socket A (engine conn): plugin calls engine (registration, routes, subscribe)
//   - Socket B (callback conn): engine calls plugin (config, events, encode/decode, bye)
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
	"net"
	"os"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// Plugin represents a ze plugin using the YANG RPC protocol.
type Plugin struct {
	name string

	// Two bidirectional connections (one per socket).
	engineConn   *rpc.Conn    // Socket A: plugin -> engine RPCs
	callbackConn *rpc.Conn    // Socket B: engine -> plugin RPCs
	engineMux    *rpc.MuxConn // Multiplexed Socket A for concurrent post-startup RPCs

	// Underlying Socket B net.Conn (dual-conn mode only).
	rawCallbackB net.Conn

	// Direct transport bridge for internal plugins (nil for external).
	// Discovered via type assertion on engineConn in NewWithConn.
	// After startup, DeliverEvents bypasses socket B and callEngineRaw
	// dispatches through bridge.DispatchRPC instead of engineMux.CallRPC.
	bridge *rpc.DirectBridge

	// Callbacks for engine-initiated RPCs.
	onConfigure        func([]ConfigSection) error
	onShareRegistry    func([]RegistryCommand)
	onEvent            func(string) error
	onStructuredEvent  func([]any) error
	onEncodeNLRI       func(family string, args []string) (string, error)
	onDecodeNLRI       func(family string, hex string) (string, error)
	onDecodeCapability func(code uint8, hex string) (string, error)
	onExecuteCommand   func(serial, command string, args []string, peer string) (status, data string, err error)
	onConfigVerify     func([]ConfigSection) error
	onConfigApply      func([]ConfigDiffSection) error
	onValidateOpen     func(*ValidateOpenInput) *ValidateOpenOutput
	onBye              func(string)

	// Post-startup callback (runs after Stage 5, before event loop).
	// Safe to make engine calls (Socket A) here -- startup is complete.
	onStarted func(context.Context) error

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

// NewWithConn creates a plugin with explicit connections (for testing).
// engineConn is the plugin side of Socket A, callbackConn is the plugin side of Socket B.
// For internal plugins, engineConn may be a BridgedConn carrying a DirectBridge
// reference for post-startup direct transport.
func NewWithConn(name string, engineConn, callbackConn net.Conn) *Plugin {
	p := &Plugin{
		name:         name,
		engineConn:   rpc.NewConn(engineConn, engineConn),
		callbackConn: rpc.NewConn(callbackConn, callbackConn),
		rawCallbackB: callbackConn,
	}
	// Discover bridge via type assertion (internal plugins only).
	if bridger, ok := engineConn.(rpc.Bridger); ok {
		p.bridge = bridger.Bridge()
	}
	return p
}

// NewFromFDs creates a plugin from inherited file descriptors.
// engineFD is the plugin's end of Socket A (plugin->engine calls).
// callbackFD is the plugin's end of Socket B (engine->plugin calls).
func NewFromFDs(name string, engineFD, callbackFD int) (*Plugin, error) {
	engineConn, err := connFromFD(engineFD, "ze-engine")
	if err != nil {
		return nil, fmt.Errorf("engine fd %d: %w", engineFD, err)
	}

	callbackConn, err := connFromFD(callbackFD, "ze-callback")
	if err != nil {
		engineConn.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return nil, fmt.Errorf("callback fd %d: %w", callbackFD, err)
	}

	return NewWithConn(name, engineConn, callbackConn), nil
}

// connFromFD wraps an inherited file descriptor as a net.Conn.
func connFromFD(fd int, name string) (net.Conn, error) {
	f := os.NewFile(uintptr(fd), name)
	if f == nil {
		return nil, fmt.Errorf("invalid fd %d", fd)
	}
	conn, err := net.FileConn(f)
	if closeErr := f.Close(); closeErr != nil && err == nil {
		if conn != nil {
			conn.Close() //nolint:errcheck,gosec // best-effort cleanup
		}
		return nil, fmt.Errorf("close file: %w", closeErr)
	}
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// NewFromEnv creates a plugin by reading ZE_PLUGIN_HUB_HOST, ZE_PLUGIN_HUB_PORT, and
// ZE_PLUGIN_TOKEN environment variables. Connects to the engine via TLS.
func NewFromEnv(name string) (*Plugin, error) {
	return NewFromTLSEnv(name)
}

// Default plugin transport address (matches hub config default listen address).
const (
	DefaultPluginHost = "127.0.0.1"
	DefaultPluginPort = "12700"
)

// NewFromTLSEnv creates a plugin by reading ZE_PLUGIN_HUB_HOST, ZE_PLUGIN_HUB_PORT,
// and ZE_PLUGIN_TOKEN env vars. Connects to the engine via TLS, authenticates,
// and returns a single-conn plugin.
// ZE_PLUGIN_HUB_HOST defaults to 127.0.0.1, ZE_PLUGIN_HUB_PORT defaults to 12700.
// ZE_PLUGIN_TOKEN is required.
func NewFromTLSEnv(name string) (*Plugin, error) {
	host := os.Getenv("ZE_PLUGIN_HUB_HOST")
	if host == "" {
		host = DefaultPluginHost
	}
	port := os.Getenv("ZE_PLUGIN_HUB_PORT")
	if port == "" {
		port = DefaultPluginPort
	}
	token := os.Getenv("ZE_PLUGIN_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("ZE_PLUGIN_TOKEN must be set")
	}

	addr := net.JoinHostPort(host, port)
	tlsConf := &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // engine uses self-signed cert
		MinVersion:         tls.VersionTLS13,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := (&tls.Dialer{Config: tlsConf}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("TLS dial %s: %w", addr, err)
	}

	// Send auth request directly (no rpc.Conn to avoid reader goroutine leak).
	if authErr := ipc.SendAuth(ctx, conn, token, name); authErr != nil {
		conn.Close() //nolint:errcheck,gosec // cleanup on auth failure
		return nil, fmt.Errorf("auth: %w", authErr)
	}

	// Create ONE rpc.Conn for this connection. ReadRequest starts the
	// persistent reader. MuxConn (created in NewWithSingleConn) reuses
	// the same reader via sync.Once -- no competing goroutines.
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

// NewWithSingleConn creates a plugin with a single bidirectional connection.
// Used for TLS external plugins. MuxConn is created immediately to handle
// bidirectional RPC from the start (both engine calls and callbacks).
func NewWithSingleConn(name string, conn net.Conn) *Plugin {
	engineConn := rpc.NewConn(conn, conn)
	engineMux := rpc.NewMuxConn(engineConn)
	return &Plugin{
		name:       name,
		engineConn: engineConn,
		engineMux:  engineMux,
	}
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
	engineErr := p.engineConn.Close()
	if p.callbackConn != nil {
		if callbackErr := p.callbackConn.Close(); callbackErr != nil && engineErr == nil {
			return callbackErr
		}
	}
	return engineErr
}

// Run executes the 5-stage startup protocol and enters the event loop.
// Returns nil on clean shutdown (bye received), or error on failure.
func (p *Plugin) Run(ctx context.Context, reg Registration) error {
	// Auto-set WantsValidateOpen if callback is registered.
	p.mu.Lock()
	if p.onValidateOpen != nil {
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

	// Stage 5: ready (with optional startup subscriptions)
	p.mu.Lock()
	var readyInput *rpc.ReadyInput
	if p.startupSubscription != nil {
		readyInput = &rpc.ReadyInput{Subscribe: p.startupSubscription}
	}
	p.mu.Unlock()

	if err := p.callEngine(ctx, "ze-plugin-engine:ready", readyInput); err != nil {
		return fmt.Errorf("stage 5 (ready): %w", err)
	}

	// Startup complete: create MuxConn for concurrent engine calls.
	// In single-conn mode, engineMux was created in the constructor.
	// In dual-conn mode, create it now from the sequential engineConn.
	if p.engineMux == nil {
		p.engineMux = rpc.NewMuxConn(p.engineConn)
	}

	// Activate direct transport bridge if discovered during construction.
	// Register the plugin's event handler so the engine can call it directly
	// instead of going through socket B. Signal ready so the engine side
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

	// Post-startup: safe to make engine calls (Socket A is free).
	// The engine's runtime handler starts reading Socket A after all
	// plugins complete startup, so writes are buffered briefly then handled.
	p.mu.Lock()
	startedFn := p.onStarted
	p.mu.Unlock()

	if startedFn != nil {
		if err := startedFn(ctx); err != nil {
			return fmt.Errorf("post-startup: %w", err)
		}
	}

	// Enter event loop: still needed for non-event callbacks (bye, encode/decode,
	// config-verify, config-apply, validate-open, execute-command).
	// With direct bridge, deliver-batch events bypass the event loop entirely.
	return p.eventLoop(ctx)
}

// callEngine sends an RPC to the engine via Socket A and waits for response.
// Uses MuxConn when available (post-startup), falls back to Conn (during startup).
// Since CallRPC returns RPC errors as *rpc.RPCCallError, this just forwards the error.
func (p *Plugin) callEngine(ctx context.Context, method string, params any) error {
	_, err := p.callEngineRaw(ctx, method, params)
	return err
}

// callEngineWithResult sends an RPC to the engine and returns the result payload.
// Uses MuxConn when available (post-startup), falls back to Conn (during startup).
// Since CallRPC returns the result payload directly (RPC errors as Go errors),
// this just forwards the return values.
func (p *Plugin) callEngineWithResult(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return p.callEngineRaw(ctx, method, params)
}

// callEngineRaw sends an RPC and returns the result payload.
// Dispatches to: DirectBridge (direct function call, internal plugins post-startup),
// MuxConn (concurrent socket, post-startup), or Conn (sequential socket, startup).
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
	if p.engineMux != nil {
		return p.engineMux.CallRPC(ctx, method, params)
	}
	return p.engineConn.CallRPC(ctx, method, params)
}

// --- Callback handlers ---

// --- RPC Types (aliases to canonical types in pkg/plugin/rpc) ---
