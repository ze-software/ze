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
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"

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

	// Underlying Socket B net.Conn for SCM_RIGHTS fd passing (ReceiveListener).
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

// NewFromEnv creates a plugin by reading ZE_ENGINE_FD and ZE_CALLBACK_FD
// environment variables. This is the primary constructor for external plugins
// launched as subprocesses by the ze engine.
func NewFromEnv(name string) (*Plugin, error) {
	engineFD, err := envFD("ZE_ENGINE_FD")
	if err != nil {
		return nil, err
	}
	callbackFD, err := envFD("ZE_CALLBACK_FD")
	if err != nil {
		return nil, err
	}
	return NewFromFDs(name, engineFD, callbackFD)
}

func envFD(name string) (int, error) {
	s := os.Getenv(name)
	if s == "" {
		return 0, fmt.Errorf("environment variable %s not set", name)
	}
	fd, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("environment variable %s: %w", name, err)
	}
	return fd, nil
}

// ReceiveListener receives a listen socket fd from the engine via SCM_RIGHTS
// on the callback connection (Socket B) and returns it as a net.Listener.
// This is used after Stage 1 when the plugin declared connection-handlers.
// Only works for external plugins (real Unix sockets, not net.Pipe).
func (p *Plugin) ReceiveListener() (net.Listener, error) {
	received, err := ipc.ReceiveFD(p.rawCallbackB)
	if err != nil {
		return nil, fmt.Errorf("receive listener fd: %w", err)
	}
	ln, err := net.FileListener(received)
	received.Close() //nolint:errcheck,gosec // fd ownership transferred to listener
	if err != nil {
		return nil, fmt.Errorf("convert fd to listener: %w", err)
	}
	return ln, nil
}

// Listeners returns the listen sockets received from the engine during startup.
// The engine creates these sockets and sends them via SCM_RIGHTS between
// Stage 1 and Stage 2, one per connection-handler declared in the registration.
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
	callbackErr := p.callbackConn.Close()
	if engineErr != nil {
		return engineErr
	}
	return callbackErr
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

	// Auto-receive listen socket fds sent by engine between Stage 1 and Stage 2.
	// The engine sends one fd per connection-handler via SCM_RIGHTS on Socket B.
	// Must happen before Stage 2 (which starts FrameReader on callbackConn).
	for range reg.ConnectionHandlers {
		ln, err := p.ReceiveListener()
		if err != nil {
			return fmt.Errorf("receive connection handler listener: %w", err)
		}
		p.listeners = append(p.listeners, ln)
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
	// The 5-stage startup uses engineConn.CallRPC (sequential, one-at-a-time).
	// MuxConn takes ownership of the reader for concurrent multiplexed RPCs.
	p.engineMux = rpc.NewMuxConn(p.engineConn)

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
