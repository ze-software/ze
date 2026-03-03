// Design: docs/architecture/api/process-protocol.md — plugin SDK
// Detail: sdk_text.go — text-mode startup and event loop
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
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/core/ipc"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// Plugin represents a ze plugin using the YANG RPC protocol.
type Plugin struct {
	name string

	// Two bidirectional connections (one per socket).
	engineConn   *rpc.Conn    // Socket A: plugin → engine RPCs
	callbackConn *rpc.Conn    // Socket B: engine → plugin RPCs
	engineMux    *rpc.MuxConn // Multiplexed Socket A for concurrent post-startup RPCs

	// Text mode: line-based framing instead of NUL-delimited JSON-RPC.
	// When textMode is true, text* fields are used instead of engineConn/callbackConn.
	textMode     bool
	textConnA    *rpc.TextConn    // Socket A text framing (startup + textMux owner)
	textConnB    *rpc.TextConn    // Socket B text framing (event loop)
	textMux      *rpc.TextMuxConn // Multiplexed Socket A for concurrent post-startup text RPCs
	rawEngineA   net.Conn         // Underlying Socket A (for Close)
	rawCallbackB net.Conn         // Underlying Socket B (for Close)

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
	// Safe to make engine calls (Socket A) here — startup is complete.
	onStarted func(context.Context) error

	// Startup subscriptions: included in the "ready" RPC so the engine
	// registers them atomically before SignalAPIReady, avoiding the race
	// between reactor sending routes and the plugin subscribing.
	startupSubscription *rpc.SubscribeEventsInput

	// Capabilities to declare during Stage 3.
	capabilities []CapabilityDecl

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
	}
	// Discover bridge via type assertion (internal plugins only).
	if bridger, ok := engineConn.(rpc.Bridger); ok {
		p.bridge = bridger.Bridge()
	}
	return p
}

// NewFromFDs creates a plugin from inherited file descriptors.
// engineFD is the plugin's end of Socket A (plugin→engine calls).
// callbackFD is the plugin's end of Socket B (engine→plugin calls).
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

// Close closes the underlying connections, unblocking any goroutines waiting
// on Read(). Must be called when the plugin is done to prevent goroutine leaks.
// Safe to call multiple times.
func (p *Plugin) Close() error {
	if p.textMode {
		return p.closeText()
	}
	// Close MuxConn first — its background reader must stop before
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

// OnConfigure sets the handler for Stage 2 config delivery.
func (p *Plugin) OnConfigure(fn func([]ConfigSection) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onConfigure = fn
}

// OnShareRegistry sets the handler for Stage 4 registry delivery.
func (p *Plugin) OnShareRegistry(fn func([]RegistryCommand)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onShareRegistry = fn
}

// OnEvent sets the handler for runtime event delivery.
func (p *Plugin) OnEvent(fn func(string) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onEvent = fn
}

// OnStructuredEvent sets the handler for structured event delivery via DirectBridge.
// When registered, the bridge delivers structured events directly (no text formatting).
// The handler receives []any where each element is a *rpc.StructuredUpdate.
func (p *Plugin) OnStructuredEvent(fn func([]any) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onStructuredEvent = fn
}

// OnBye sets the handler for shutdown notification.
func (p *Plugin) OnBye(fn func(string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onBye = fn
}

// OnEncodeNLRI sets the handler for NLRI encoding requests.
// The handler receives the address family and arguments, and returns hex-encoded NLRI.
func (p *Plugin) OnEncodeNLRI(fn func(family string, args []string) (string, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onEncodeNLRI = fn
}

// OnDecodeNLRI sets the handler for NLRI decoding requests.
// The handler receives the address family and hex-encoded NLRI, and returns JSON.
func (p *Plugin) OnDecodeNLRI(fn func(family string, hex string) (string, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onDecodeNLRI = fn
}

// OnDecodeCapability sets the handler for capability decoding requests.
// The handler receives the capability code and hex-encoded bytes, and returns JSON.
func (p *Plugin) OnDecodeCapability(fn func(code uint8, hex string) (string, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onDecodeCapability = fn
}

// OnExecuteCommand sets the handler for command execution requests.
// The handler receives serial, command, args, peer and returns (status, data, error).
func (p *Plugin) OnExecuteCommand(fn func(serial, command string, args []string, peer string) (string, string, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onExecuteCommand = fn
}

// OnConfigVerify sets the handler for config verification requests (reload pipeline).
// The handler receives the full candidate config sections and returns nil to accept
// or an error to reject. If no handler is registered, config-verify returns OK (no-op).
func (p *Plugin) OnConfigVerify(fn func([]ConfigSection) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onConfigVerify = fn
}

// OnConfigApply sets the handler for config apply requests (reload pipeline).
// The handler receives diff sections describing what changed and returns nil to accept
// or an error to reject. If no handler is registered, config-apply returns OK (no-op).
func (p *Plugin) OnConfigApply(fn func([]ConfigDiffSection) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onConfigApply = fn
}

// OnValidateOpen sets the handler for OPEN validation requests.
// The handler receives both local and remote OPEN messages and returns accept/reject.
// When registered, WantsValidateOpen is automatically set in Stage 1 registration.
// If no handler is registered, validate-open returns accept (no-op).
func (p *Plugin) OnValidateOpen(fn func(*ValidateOpenInput) *ValidateOpenOutput) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onValidateOpen = fn
}

// OnStarted sets a callback that runs after the 5-stage startup completes
// but before the event loop begins. This is the safe place to make engine
// calls (e.g., SubscribeEvents) because Socket A is no longer blocked by
// the startup coordinator. Do NOT make engine calls inside OnShareRegistry
// or OnConfigure — those run while the engine is waiting on Socket B,
// causing a cross-socket deadlock.
func (p *Plugin) OnStarted(fn func(ctx context.Context) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onStarted = fn
}

// SetStartupSubscriptions sets event subscriptions to include in the "ready" RPC.
// The engine registers these atomically before SignalAPIReady, ensuring the plugin
// receives events from the very first route send. Must be called before Run().
//
// This replaces the pattern of calling SubscribeEvents in OnStarted, which had a
// race condition: SignalAPIReady triggered route sends before the subscription RPC
// could be processed.
func (p *Plugin) SetStartupSubscriptions(events, peers []string, format string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.startupSubscription = &rpc.SubscribeEventsInput{
		Events: events,
		Peers:  peers,
		Format: format,
	}
}

// SetEncoding sets the event encoding preference ("json" or "text").
// Must be called after SetStartupSubscriptions and before Run().
// Text encoding uses space-delimited output parseable by strings.Fields
// instead of nested JSON requiring json.Unmarshal.
func (p *Plugin) SetEncoding(enc string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.startupSubscription == nil {
		p.startupSubscription = &rpc.SubscribeEventsInput{}
	}
	p.startupSubscription.Encoding = enc
}

// SetCapabilities sets the capabilities to declare during Stage 3.
// Must be called before Run().
func (p *Plugin) SetCapabilities(caps []CapabilityDecl) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.capabilities = caps
}

// Run executes the 5-stage startup protocol and enters the event loop.
// Returns nil on clean shutdown (bye received), or error on failure.
func (p *Plugin) Run(ctx context.Context, reg Registration) error {
	// Text mode: delegate to text-specific startup and event loop.
	if p.textMode {
		return p.runText(ctx, reg)
	}

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
func (p *Plugin) callEngine(ctx context.Context, method string, params any) error {
	raw, err := p.callEngineRaw(ctx, method, params)
	if err != nil {
		return err
	}
	return rpc.CheckResponse(raw)
}

// callEngineWithResult sends an RPC to the engine and returns the result payload.
// Uses MuxConn when available (post-startup), falls back to Conn (during startup).
func (p *Plugin) callEngineWithResult(ctx context.Context, method string, params any) (json.RawMessage, error) {
	raw, err := p.callEngineRaw(ctx, method, params)
	if err != nil {
		return nil, err
	}
	return rpc.ParseResponse(raw)
}

// callEngineRaw sends an RPC and returns the raw response frame.
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

// UpdateRoute injects a route update to matching peers via the engine.
// Returns the number of peers affected and routes sent.
func (p *Plugin) UpdateRoute(ctx context.Context, peerSelector, command string) (peersAffected, routesSent uint32, err error) {
	input := &rpc.UpdateRouteInput{PeerSelector: peerSelector, Command: command}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:update-route", input)
	if err != nil {
		return 0, 0, err
	}
	var out rpc.UpdateRouteOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return 0, 0, fmt.Errorf("unmarshal update-route result: %w", err)
	}
	return out.PeersAffected, out.RoutesSent, nil
}

// DispatchCommand dispatches a command through the engine's command dispatcher.
// Returns the status and data from the target handler's response. This enables
// inter-plugin communication: the engine routes the command to the target plugin
// via longest-match registry lookup and returns the full structured response.
func (p *Plugin) DispatchCommand(ctx context.Context, command string) (status, data string, err error) {
	input := &rpc.DispatchCommandInput{Command: command}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:dispatch-command", input)
	if err != nil {
		return "", "", err
	}
	var out rpc.DispatchCommandOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return "", "", fmt.Errorf("unmarshal dispatch-command result: %w", err)
	}
	return out.Status, out.Data, nil
}

// SubscribeEvents requests event delivery from the engine.
func (p *Plugin) SubscribeEvents(ctx context.Context, events, peers []string, format string) error {
	input := &rpc.SubscribeEventsInput{Events: events, Peers: peers, Format: format}
	return p.callEngine(ctx, "ze-plugin-engine:subscribe-events", input)
}

// UnsubscribeEvents stops event delivery from the engine.
func (p *Plugin) UnsubscribeEvents(ctx context.Context) error {
	return p.callEngine(ctx, "ze-plugin-engine:unsubscribe-events", nil)
}

// DecodeNLRI requests NLRI decoding from the engine via the plugin registry.
// The engine routes the request to the in-process decoder for the given family.
// Returns the JSON representation of the decoded NLRI.
func (p *Plugin) DecodeNLRI(ctx context.Context, family, hex string) (string, error) {
	input := &rpc.DecodeNLRIInput{Family: family, Hex: hex}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:decode-nlri", input)
	if err != nil {
		return "", err
	}
	var out rpc.DecodeNLRIOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return "", fmt.Errorf("unmarshal decode-nlri result: %w", err)
	}
	return out.JSON, nil
}

// EncodeNLRI requests NLRI encoding from the engine via the plugin registry.
// The engine routes the request to the in-process encoder for the given family.
// Returns hex-encoded NLRI bytes.
func (p *Plugin) EncodeNLRI(ctx context.Context, family string, args []string) (string, error) {
	input := &rpc.EncodeNLRIInput{Family: family, Args: args}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:encode-nlri", input)
	if err != nil {
		return "", err
	}
	var out rpc.EncodeNLRIOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return "", fmt.Errorf("unmarshal encode-nlri result: %w", err)
	}
	return out.Hex, nil
}

// DecodeMPReach requests MP_REACH_NLRI decoding from the engine.
// The engine parses the attribute value (AFI+SAFI+NH+NLRI) and returns the family,
// next-hop, and decoded NLRI. RFC 4760 Section 3.
func (p *Plugin) DecodeMPReach(ctx context.Context, hex string, addPath bool) (*rpc.DecodeMPReachOutput, error) {
	input := &rpc.DecodeMPReachInput{Hex: hex, AddPath: addPath}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:decode-mp-reach", input)
	if err != nil {
		return nil, err
	}
	var out rpc.DecodeMPReachOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, fmt.Errorf("unmarshal decode-mp-reach result: %w", err)
	}
	return &out, nil
}

// DecodeMPUnreach requests MP_UNREACH_NLRI decoding from the engine.
// The engine parses the attribute value (AFI+SAFI+Withdrawn) and returns the family
// and decoded withdrawn NLRI. RFC 4760 Section 4.
func (p *Plugin) DecodeMPUnreach(ctx context.Context, hex string, addPath bool) (*rpc.DecodeMPUnreachOutput, error) {
	input := &rpc.DecodeMPUnreachInput{Hex: hex, AddPath: addPath}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:decode-mp-unreach", input)
	if err != nil {
		return nil, err
	}
	var out rpc.DecodeMPUnreachOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, fmt.Errorf("unmarshal decode-mp-unreach result: %w", err)
	}
	return &out, nil
}

// DecodeUpdate requests full UPDATE message decoding from the engine.
// The engine parses the UPDATE body (after 19-byte BGP header) and returns
// the ze-bgp JSON representation. RFC 4271 Section 4.3.
func (p *Plugin) DecodeUpdate(ctx context.Context, hex string, addPath bool) (string, error) {
	input := &rpc.DecodeUpdateInput{Hex: hex, AddPath: addPath}
	result, err := p.callEngineWithResult(ctx, "ze-plugin-engine:decode-update", input)
	if err != nil {
		return "", err
	}
	var out rpc.DecodeUpdateOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return "", fmt.Errorf("unmarshal decode-update result: %w", err)
	}
	return out.JSON, nil
}

// serveOne reads one request from Socket B, dispatches it, and sends the response.
func (p *Plugin) serveOne(ctx context.Context, expectedMethod string, handler func(json.RawMessage) error) error {
	req, err := p.callbackConn.ReadRequest(ctx)
	if err != nil {
		return fmt.Errorf("read request: %w", err)
	}

	if req.Method != expectedMethod {
		return fmt.Errorf("expected method %q, got %q", expectedMethod, req.Method)
	}

	if err := handler(req.Params); err != nil {
		return p.callbackConn.SendError(ctx, req.ID, err.Error())
	}

	return p.callbackConn.SendOK(ctx, req.ID)
}

// isConnectionClosed reports whether err indicates a closed connection.
// During shutdown the engine closes the socket, producing EOF or
// "use of closed network connection" — both are clean exit signals.
func isConnectionClosed(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	// net.Pipe and Unix sockets surface this as an opaque string.
	return strings.Contains(err.Error(), "use of closed network connection")
}

// eventLoop handles runtime RPCs from the engine on Socket B.
func (p *Plugin) eventLoop(ctx context.Context) error {
	for {
		req, err := p.callbackConn.ReadRequest(ctx)
		if err != nil {
			// Context canceled or connection closed = clean shutdown.
			// The engine closes the socket to signal internal plugins to exit;
			// this races with context cancellation, so check both.
			if ctx.Err() == nil && !isConnectionClosed(err) {
				return fmt.Errorf("event loop read: %w", err)
			}
			return nil //nolint:nilerr // EOF/context-cancel during shutdown is not an error
		}

		if err := p.dispatchCallback(ctx, req); err != nil {
			return err
		}

		// bye signals clean shutdown
		if req.Method == "ze-plugin-callback:bye" {
			return nil
		}
	}
}

// dispatchCallback routes a callback request to the appropriate handler and sends the response.
func (p *Plugin) dispatchCallback(ctx context.Context, req *ipc.Request) error {
	switch req.Method {
	case "ze-plugin-callback:deliver-event":
		if handleErr := p.handleDeliverEvent(req.Params); handleErr != nil {
			return p.callbackConn.SendError(ctx, req.ID, handleErr.Error())
		}
		return p.callbackConn.SendOK(ctx, req.ID)

	case "ze-plugin-callback:deliver-batch":
		if handleErr := p.handleDeliverBatch(req.Params); handleErr != nil {
			return p.callbackConn.SendError(ctx, req.ID, handleErr.Error())
		}
		return p.callbackConn.SendOK(ctx, req.ID)

	case "ze-plugin-callback:encode-nlri":
		return p.handleNLRICallback(ctx, req, p.encodeNLRIHandler())

	case "ze-plugin-callback:decode-nlri":
		return p.handleNLRICallback(ctx, req, p.decodeNLRIHandler())

	case "ze-plugin-callback:decode-capability":
		return p.handleNLRICallback(ctx, req, p.decodeCapabilityHandler())

	case "ze-plugin-callback:execute-command":
		return p.handleExecuteCommand(ctx, req)

	case "ze-plugin-callback:config-verify":
		return p.handleConfigVerify(ctx, req)

	case "ze-plugin-callback:config-apply":
		return p.handleConfigApply(ctx, req)

	case "ze-plugin-callback:validate-open":
		return p.handleValidateOpen(ctx, req)

	case "ze-plugin-callback:bye":
		return p.handleByeAndRespond(ctx, req)
	}

	// Ze's fail-on-unknown rule: reject unknown methods with an error response.
	errMsg := fmt.Sprintf("unknown method: %s", req.Method)
	return p.callbackConn.SendError(ctx, req.ID, errMsg)
}

// handleByeAndRespond handles bye by responding first, then invoking callback.
func (p *Plugin) handleByeAndRespond(ctx context.Context, req *ipc.Request) error {
	// Send response before invoking callback
	if err := p.callbackConn.SendOK(ctx, req.ID); err != nil {
		return err
	}

	var input struct {
		Reason string `json:"reason,omitempty"`
	}
	if req.Params != nil {
		// Non-fatal: bye still processed even if params fail to parse
		_ = json.Unmarshal(req.Params, &input) //nolint:errcheck // best-effort
	}

	p.mu.Lock()
	fn := p.onBye
	p.mu.Unlock()

	if fn != nil {
		fn(input.Reason)
	}

	return nil
}

// --- Callback handlers ---

func (p *Plugin) handleConfigure(params json.RawMessage) error {
	var input struct {
		Sections []ConfigSection `json:"sections"`
	}
	if params != nil {
		if err := json.Unmarshal(params, &input); err != nil {
			return fmt.Errorf("unmarshal configure: %w", err)
		}
	}

	p.mu.Lock()
	fn := p.onConfigure
	p.mu.Unlock()

	if fn != nil {
		return fn(input.Sections)
	}

	return nil
}

func (p *Plugin) handleShareRegistry(params json.RawMessage) error {
	var input struct {
		Commands []RegistryCommand `json:"commands"`
	}
	if params != nil {
		if err := json.Unmarshal(params, &input); err != nil {
			return fmt.Errorf("unmarshal share-registry: %w", err)
		}
	}

	p.mu.Lock()
	fn := p.onShareRegistry
	p.mu.Unlock()

	if fn != nil {
		fn(input.Commands)
	}

	return nil
}

func (p *Plugin) handleDeliverEvent(params json.RawMessage) error {
	var input struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return fmt.Errorf("unmarshal deliver-event: %w", err)
	}

	p.mu.Lock()
	fn := p.onEvent
	p.mu.Unlock()

	if fn != nil {
		return fn(input.Event)
	}

	return nil
}

// handleDeliverBatch processes a batched event delivery by dispatching each
// event to the onEvent handler. Short-circuits on the first handler error.
func (p *Plugin) handleDeliverBatch(params json.RawMessage) error {
	events, err := ipc.ParseBatchEvents(params)
	if err != nil {
		return err
	}

	p.mu.Lock()
	fn := p.onEvent
	p.mu.Unlock()

	if fn == nil {
		return nil
	}

	for _, raw := range events {
		var eventStr string
		if err := json.Unmarshal(raw, &eventStr); err != nil {
			return fmt.Errorf("unmarshal batch event: %w", err)
		}
		if err := fn(eventStr); err != nil {
			return err
		}
	}

	return nil
}

// handleNLRICallback handles an NLRI-related RPC by unmarshalling params, invoking
// a handler function, and sending either the result or an error response.
func (p *Plugin) handleNLRICallback(ctx context.Context, req *ipc.Request, handler func(json.RawMessage) (any, error)) error {
	if handler == nil {
		return p.callbackConn.SendError(ctx, req.ID, req.Method+" not supported")
	}

	result, err := handler(req.Params)
	if err != nil {
		return p.callbackConn.SendError(ctx, req.ID, err.Error())
	}

	return p.callbackConn.SendResult(ctx, req.ID, result)
}

func (p *Plugin) encodeNLRIHandler() func(json.RawMessage) (any, error) {
	p.mu.Lock()
	fn := p.onEncodeNLRI
	p.mu.Unlock()

	if fn == nil {
		return nil
	}

	return func(params json.RawMessage) (any, error) {
		var input rpc.EncodeNLRIInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal encode-nlri: %w", err)
		}
		hex, err := fn(input.Family, input.Args)
		if err != nil {
			return nil, err
		}
		return struct {
			Hex string `json:"hex"`
		}{Hex: hex}, nil
	}
}

func (p *Plugin) decodeNLRIHandler() func(json.RawMessage) (any, error) {
	p.mu.Lock()
	fn := p.onDecodeNLRI
	p.mu.Unlock()

	if fn == nil {
		return nil
	}

	return func(params json.RawMessage) (any, error) {
		var input rpc.DecodeNLRIInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal decode-nlri: %w", err)
		}
		jsonResult, err := fn(input.Family, input.Hex)
		if err != nil {
			return nil, err
		}
		return struct {
			JSON string `json:"json"`
		}{JSON: jsonResult}, nil
	}
}

func (p *Plugin) decodeCapabilityHandler() func(json.RawMessage) (any, error) {
	p.mu.Lock()
	fn := p.onDecodeCapability
	p.mu.Unlock()

	if fn == nil {
		return nil
	}

	return func(params json.RawMessage) (any, error) {
		var input rpc.DecodeCapabilityInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("unmarshal decode-capability: %w", err)
		}
		jsonResult, err := fn(input.Code, input.Hex)
		if err != nil {
			return nil, err
		}
		return struct {
			JSON string `json:"json"`
		}{JSON: jsonResult}, nil
	}
}

func (p *Plugin) handleExecuteCommand(ctx context.Context, req *ipc.Request) error {
	p.mu.Lock()
	fn := p.onExecuteCommand
	p.mu.Unlock()

	if fn == nil {
		return p.callbackConn.SendError(ctx, req.ID, "execute-command not supported")
	}

	var input rpc.ExecuteCommandInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		return p.callbackConn.SendError(ctx, req.ID, fmt.Sprintf("unmarshal execute-command: %v", err))
	}

	status, data, err := fn(input.Serial, input.Command, input.Args, input.Peer)
	if err != nil {
		return p.callbackConn.SendError(ctx, req.ID, err.Error())
	}

	return p.callbackConn.SendResult(ctx, req.ID, &rpc.ExecuteCommandOutput{
		Status: status,
		Data:   data,
	})
}

// handleConfigRPC is a shared handler for config-verify and config-apply RPCs.
// Both follow the same pattern: unmarshal params, call handler, return status/error result.
// The handler function receives raw params and returns an error (reject) or nil (accept).
func (p *Plugin) handleConfigRPC(ctx context.Context, req *ipc.Request, handler func(json.RawMessage) error) error {
	if handler == nil {
		// No handler = graceful no-op (not all plugins care about config).
		return p.callbackConn.SendResult(ctx, req.ID, &struct {
			Status string `json:"status"`
		}{Status: "ok"})
	}

	if err := handler(req.Params); err != nil {
		return p.callbackConn.SendResult(ctx, req.ID, &struct {
			Status string `json:"status"`
			Error  string `json:"error,omitempty"`
		}{Status: "error", Error: err.Error()})
	}

	return p.callbackConn.SendResult(ctx, req.ID, &struct {
		Status string `json:"status"`
	}{Status: "ok"})
}

func (p *Plugin) handleConfigVerify(ctx context.Context, req *ipc.Request) error {
	p.mu.Lock()
	fn := p.onConfigVerify
	p.mu.Unlock()

	var handler func(json.RawMessage) error
	if fn != nil {
		handler = func(params json.RawMessage) error {
			var input rpc.ConfigVerifyInput
			if err := json.Unmarshal(params, &input); err != nil {
				return fmt.Errorf("unmarshal config-verify: %w", err)
			}
			return fn(input.Sections)
		}
	}

	return p.handleConfigRPC(ctx, req, handler)
}

func (p *Plugin) handleConfigApply(ctx context.Context, req *ipc.Request) error {
	p.mu.Lock()
	fn := p.onConfigApply
	p.mu.Unlock()

	var handler func(json.RawMessage) error
	if fn != nil {
		handler = func(params json.RawMessage) error {
			var input rpc.ConfigApplyInput
			if err := json.Unmarshal(params, &input); err != nil {
				return fmt.Errorf("unmarshal config-apply: %w", err)
			}
			return fn(input.Sections)
		}
	}

	return p.handleConfigRPC(ctx, req, handler)
}

func (p *Plugin) handleValidateOpen(ctx context.Context, req *ipc.Request) error {
	p.mu.Lock()
	fn := p.onValidateOpen
	p.mu.Unlock()

	if fn == nil {
		// No handler = accept (no-op).
		return p.callbackConn.SendResult(ctx, req.ID, &rpc.ValidateOpenOutput{Accept: true})
	}

	var input rpc.ValidateOpenInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		return p.callbackConn.SendResult(ctx, req.ID, &rpc.ValidateOpenOutput{
			Accept: false, Reason: fmt.Sprintf("unmarshal validate-open: %v", err),
		})
	}

	output := fn(&input)
	return p.callbackConn.SendResult(ctx, req.ID, output)
}

// --- RPC Types (aliases to canonical types in pkg/plugin/rpc) ---

// Registration is the SDK name for the declare-registration input (Stage 1).
type Registration = rpc.DeclareRegistrationInput

// FamilyDecl declares an address family the plugin handles.
type FamilyDecl = rpc.FamilyDecl

// CommandDecl declares a command the plugin provides.
type CommandDecl = rpc.CommandDecl

// SchemaDecl declares the YANG schema the plugin provides.
type SchemaDecl = rpc.SchemaDecl

// DeclareCapabilitiesInput is the input for declare-capabilities (Stage 3).
type DeclareCapabilitiesInput = rpc.DeclareCapabilitiesInput

// CapabilityDecl declares a BGP capability for OPEN injection.
type CapabilityDecl = rpc.CapabilityDecl

// ConfigSection is a config section delivered during Stage 2.
type ConfigSection = rpc.ConfigSection

// RegistryCommand is a command in the shared registry (Stage 4).
type RegistryCommand = rpc.RegistryCommand

// UpdateRouteOutput is the output for update-route (runtime).
type UpdateRouteOutput = rpc.UpdateRouteOutput

// ExecuteCommandOutput is the output for execute-command (runtime).
type ExecuteCommandOutput = rpc.ExecuteCommandOutput

// DispatchCommandOutput is the output for dispatch-command (runtime).
type DispatchCommandOutput = rpc.DispatchCommandOutput

// ConfigDiffSection describes what changed in a single config root (reload).
type ConfigDiffSection = rpc.ConfigDiffSection

// ConfigVerifyOutput is the output for config-verify (reload).
type ConfigVerifyOutput = rpc.ConfigVerifyOutput

// ConfigApplyOutput is the output for config-apply (reload).
type ConfigApplyOutput = rpc.ConfigApplyOutput

// ValidateOpenInput is the input for validate-open (OPEN validation).
type ValidateOpenInput = rpc.ValidateOpenInput

// ValidateOpenOutput is the output for validate-open (OPEN validation).
type ValidateOpenOutput = rpc.ValidateOpenOutput

// ValidateOpenMessage represents one side of the OPEN exchange.
type ValidateOpenMessage = rpc.ValidateOpenMessage

// ValidateOpenCapability is a single capability from an OPEN message.
type ValidateOpenCapability = rpc.ValidateOpenCapability

// DecodeNLRIOutput is the output for decode-nlri (plugin→engine).
type DecodeNLRIOutput = rpc.DecodeNLRIOutput

// EncodeNLRIOutput is the output for encode-nlri (plugin→engine).
type EncodeNLRIOutput = rpc.EncodeNLRIOutput

// DecodeMPReachOutput is the output for decode-mp-reach (plugin→engine).
type DecodeMPReachOutput = rpc.DecodeMPReachOutput

// DecodeMPUnreachOutput is the output for decode-mp-unreach (plugin→engine).
type DecodeMPUnreachOutput = rpc.DecodeMPUnreachOutput

// DecodeUpdateOutput is the output for decode-update (plugin→engine).
type DecodeUpdateOutput = rpc.DecodeUpdateOutput
