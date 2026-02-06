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

	"codeberg.org/thomas-mangin/ze/internal/ipc"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// Plugin represents a ze plugin using the YANG RPC protocol.
type Plugin struct {
	name string

	// Two bidirectional connections (one per socket).
	engineConn   *rpc.Conn // Socket A: plugin → engine RPCs
	callbackConn *rpc.Conn // Socket B: engine → plugin RPCs

	// Callbacks for engine-initiated RPCs.
	onConfigure        func([]ConfigSection) error
	onShareRegistry    func([]RegistryCommand)
	onEvent            func(string) error
	onEncodeNLRI       func(family string, args []string) (string, error)
	onDecodeNLRI       func(family string, hex string) (string, error)
	onDecodeCapability func(code uint8, hex string) (string, error)
	onExecuteCommand   func(serial, command string, args []string, peer string) (status, data string, err error)
	onBye              func(string)

	// Capabilities to declare during Stage 3.
	capabilities []CapabilityDecl

	mu sync.Mutex
}

// NewWithConn creates a plugin with explicit connections (for testing).
// engineConn is the plugin side of Socket A, callbackConn is the plugin side of Socket B.
func NewWithConn(name string, engineConn, callbackConn net.Conn) *Plugin {
	return &Plugin{
		name:         name,
		engineConn:   rpc.NewConn(engineConn, engineConn),
		callbackConn: rpc.NewConn(callbackConn, callbackConn),
	}
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

	// Stage 5: ready
	if err := p.callEngine(ctx, "ze-plugin-engine:ready", nil); err != nil {
		return fmt.Errorf("stage 5 (ready): %w", err)
	}

	// Enter event loop
	return p.eventLoop(ctx)
}

// callEngine sends an RPC to the engine via Socket A and waits for response.
func (p *Plugin) callEngine(ctx context.Context, method string, params any) error {
	raw, err := p.engineConn.CallRPC(ctx, method, params)
	if err != nil {
		return err
	}
	return rpc.CheckResponse(raw)
}

// callEngineWithResult sends an RPC to the engine and returns the result payload.
func (p *Plugin) callEngineWithResult(ctx context.Context, method string, params any) (json.RawMessage, error) {
	raw, err := p.engineConn.CallRPC(ctx, method, params)
	if err != nil {
		return nil, err
	}
	return rpc.ParseResponse(raw)
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

// SubscribeEvents requests event delivery from the engine.
func (p *Plugin) SubscribeEvents(ctx context.Context, events, peers []string, format string) error {
	input := &rpc.SubscribeEventsInput{Events: events, Peers: peers, Format: format}
	return p.callEngine(ctx, "ze-plugin-engine:subscribe-events", input)
}

// UnsubscribeEvents stops event delivery from the engine.
func (p *Plugin) UnsubscribeEvents(ctx context.Context) error {
	return p.callEngine(ctx, "ze-plugin-engine:unsubscribe-events", nil)
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

// eventLoop handles runtime RPCs from the engine on Socket B.
func (p *Plugin) eventLoop(ctx context.Context) error {
	for {
		req, err := p.callbackConn.ReadRequest(ctx)
		if err != nil {
			// Context cancelled = clean shutdown
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("event loop read: %w", err)
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

	case "ze-plugin-callback:encode-nlri":
		return p.handleNLRICallback(ctx, req, p.encodeNLRIHandler())

	case "ze-plugin-callback:decode-nlri":
		return p.handleNLRICallback(ctx, req, p.decodeNLRIHandler())

	case "ze-plugin-callback:decode-capability":
		return p.handleNLRICallback(ctx, req, p.decodeCapabilityHandler())

	case "ze-plugin-callback:execute-command":
		return p.handleExecuteCommand(ctx, req)

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
