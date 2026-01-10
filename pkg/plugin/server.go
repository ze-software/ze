package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/message"
)

// Default stage timeout for plugin registration protocol.
// Each stage must complete within this duration.
const defaultStageTimeout = 5 * time.Second

// stageTransition handles coordinator stage completion and waiting.
// Returns true if transition succeeded, false if failed (caller should return true to stop processing).
func (s *Server) stageTransition(proc *Process, pluginName string, completeStage, waitStage PluginStage) bool {
	if s.coordinator == nil {
		return true
	}

	s.coordinator.StageComplete(proc.Index(), completeStage)

	stageCtx, cancel := context.WithTimeout(s.ctx, defaultStageTimeout)
	err := s.coordinator.WaitForStage(stageCtx, waitStage)
	cancel()

	if err != nil {
		slog.Error("stage timeout", "plugin", pluginName, "waiting_for", waitStage, "error", err)
		s.coordinator.PluginFailed(proc.Index(), fmt.Sprintf("stage timeout: %v", err))
		proc.Stop()
		return false
	}
	return true
}

// stageProgression defines a two-step stage transition with an intermediate delivery.
type stageProgression struct {
	from, mid, to PluginStage
	deliver       func(*Process)
}

// progressThroughStages handles the common pattern of two stage transitions with delivery between.
func (s *Server) progressThroughStages(proc *Process, name string, p stageProgression) {
	// First transition: from → mid
	if !s.stageTransition(proc, name, p.from, p.mid) {
		return
	}
	proc.SetStage(p.mid)

	// Deliver content
	if p.deliver != nil {
		p.deliver(proc)
	}

	// Second transition: mid → to
	if !s.stageTransition(proc, name, p.mid, p.to) {
		return
	}
	proc.SetStage(p.to)
}

// handlePluginConflict logs and handles plugin registration conflicts.
func (s *Server) handlePluginConflict(proc *Process, name, msg string, err error) {
	if s.coordinator != nil {
		s.coordinator.PluginFailed(proc.Index(), err.Error())
	}
	slog.Error(msg, "plugin", name, "error", err)
	proc.Stop()
}

// Server manages API connections and command dispatch.
type Server struct {
	config        *ServerConfig
	reactor       ReactorInterface
	dispatcher    *Dispatcher
	encoder       *JSONEncoder
	commitManager *CommitManager
	procManager   *ProcessManager

	// Plugin registration protocol
	coordinator *StartupCoordinator // Stage synchronization
	registry    *PluginRegistry     // Command/capability registry
	capInjector *CapabilityInjector // Capability injection for OPEN

	listener net.Listener
	clients  map[string]*Client
	clientID atomic.Uint64

	running atomic.Bool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

// Client represents a connected API client.
type Client struct {
	id     string
	conn   net.Conn
	server *Server

	ctx    context.Context
	cancel context.CancelFunc
}

// NewServer creates a new API server.
func NewServer(config *ServerConfig, reactor ReactorInterface) *Server {
	s := &Server{
		config:        config,
		reactor:       reactor,
		dispatcher:    NewDispatcher(),
		encoder:       NewJSONEncoder("6.0.0"),
		commitManager: NewCommitManager(),
		registry:      NewPluginRegistry(),
		capInjector:   NewCapabilityInjector(),
		clients:       make(map[string]*Client),
	}

	// Register default handlers
	RegisterDefaultHandlers(s.dispatcher)

	return s
}

// Running returns true if the server is running.
func (s *Server) Running() bool {
	return s.running.Load()
}

// ClientCount returns the number of connected clients.
func (s *Server) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// Start begins accepting connections.
func (s *Server) Start() error {
	return s.StartWithContext(context.Background())
}

// StartWithContext begins accepting connections with the given context.
func (s *Server) StartWithContext(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Start external plugins if configured
	if len(s.config.Plugins) > 0 {
		// Create coordinator for staged startup
		s.coordinator = NewStartupCoordinator(len(s.config.Plugins))

		s.procManager = NewProcessManager(s.config.Plugins)
		if err := s.procManager.StartWithContext(s.ctx); err != nil {
			return err
		}

		// Start command handlers for each process
		s.wg.Add(1)
		go s.handleProcessCommands()
	}

	// Only start socket listener if socket path is configured
	if s.config.SocketPath != "" {
		// Remove existing socket if present
		if err := os.Remove(s.config.SocketPath); err != nil && !os.IsNotExist(err) {
			return err
		}

		// Create listener
		var lc net.ListenConfig
		listener, err := lc.Listen(ctx, "unix", s.config.SocketPath)
		if err != nil {
			return err
		}

		s.listener = listener
		s.running.Store(true)

		// Start accept loop
		s.wg.Add(1)
		go s.acceptLoop()
	} else {
		s.running.Store(true)
	}

	return nil
}

// Stop signals the server to stop.
func (s *Server) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// Wait waits for the server to stop.
func (s *Server) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// acceptLoop accepts incoming connections.
func (s *Server) acceptLoop() {
	defer s.wg.Done()
	defer s.cleanup()

	for {
		// Check for shutdown
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Accept with timeout to check for shutdown
		if ul, ok := s.listener.(*net.UnixListener); ok {
			_ = ul.SetDeadline(time.Now().Add(100 * time.Millisecond))
		}

		conn, err := s.listener.Accept()
		if err != nil {
			// Check if we're shutting down
			select {
			case <-s.ctx.Done():
				return
			default:
				// Transient error, continue
				continue
			}
		}

		// Handle new client
		s.handleClient(conn)
	}
}

// cleanup closes listener and removes socket.
func (s *Server) cleanup() {
	s.running.Store(false)

	// Stop processes
	if s.procManager != nil {
		s.procManager.Stop()
	}

	// Close listener
	if s.listener != nil {
		_ = s.listener.Close()
	}

	// Close all clients
	s.mu.Lock()
	for _, client := range s.clients {
		client.cancel()
		_ = client.conn.Close()
	}
	s.clients = make(map[string]*Client)
	s.mu.Unlock()

	// Remove socket file
	if s.config.SocketPath != "" {
		_ = os.Remove(s.config.SocketPath)
	}
}

// handleProcessCommands reads and handles commands from all spawned processes.
func (s *Server) handleProcessCommands() {
	defer s.wg.Done()

	// Get all processes from the manager
	s.procManager.mu.RLock()
	processes := make([]*Process, 0, len(s.procManager.processes))
	for _, p := range s.procManager.processes {
		processes = append(processes, p)
	}
	s.procManager.mu.RUnlock()

	// Start a goroutine to handle each process
	var procWg sync.WaitGroup
	for _, proc := range processes {
		procWg.Add(1)
		go func(p *Process) {
			defer procWg.Done()
			s.handleSingleProcessCommands(p)
		}(proc)
	}

	procWg.Wait()
}

// parseSerial extracts #N prefix from command line.
// Returns (serial, command) where serial is empty if no prefix.
// Only recognizes numeric serials: "#1 cmd", "#123 cmd", not "# comment".
func parseSerial(line string) (string, string) {
	if !strings.HasPrefix(line, "#") {
		return "", line
	}
	// Find first space
	idx := strings.Index(line, " ")
	if idx <= 1 {
		return "", line // No space after # or just "#"
	}
	// Check if characters between # and space are all digits
	serial := line[1:idx]
	for _, c := range serial {
		if c < '0' || c > '9' {
			return "", line // Not a numeric serial
		}
	}
	return serial, line[idx+1:]
}

// isComment returns true if line is a comment (starts with "# ").
func isComment(line string) bool {
	return strings.HasPrefix(line, "# ")
}

// encodeAlphaSerial converts a number to alpha serial by shifting digits.
// 0->a, 1->b, ..., 9->j. Example: 123 -> "bcd", 0 -> "a", 99 -> "jj".
// Used for ZeBGP-initiated requests to avoid collision with numeric process serials.
func encodeAlphaSerial(n uint64) string {
	if n == 0 {
		return "a"
	}
	var result []byte
	for n > 0 {
		digit := n % 10
		result = append([]byte{byte('a' + digit)}, result...)
		n /= 10
	}
	return string(result)
}

// isAlphaSerial returns true if serial uses alpha encoding (a-j digits).
func isAlphaSerial(serial string) bool {
	if serial == "" {
		return false
	}
	for _, c := range serial {
		if c < 'a' || c > 'j' {
			return false
		}
	}
	return true
}

// handleSingleProcessCommands handles commands from a single process.
func (s *Server) handleSingleProcessCommands(proc *Process) {
	// Cleanup on exit
	defer s.cleanupProcess(proc)

	// Initialize process to registration stage
	proc.SetStage(StageRegistration)

	cmdCtx := &CommandContext{
		Reactor:       s.reactor,
		Encoder:       s.encoder,
		CommitManager: s.commitManager,
		Dispatcher:    s.dispatcher,
		Process:       proc, // For session state (ack, sync)
		Peer:          "*",  // Default to all peers
	}

	for proc.Running() {
		// Check for shutdown
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Read command from process stdout with timeout
		readCtx, cancel := context.WithTimeout(s.ctx, 100*time.Millisecond)
		line, err := proc.ReadCommand(readCtx)
		cancel()

		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				// Timeout, check if process is still running and try again
				continue
			}
			// Process probably exited
			return
		}

		if line == "" {
			continue
		}

		// Check for @serial response (plugin command response)
		if serial, respType, data, ok := parsePluginResponse(line); ok {
			s.handlePluginResponse(proc, serial, respType, data)
			continue
		}

		// Handle "ready failed" at any stage - plugin is signaling startup failure
		if strings.HasPrefix(line, "ready failed ") {
			s.handlePluginFailed(proc, line)
			return
		}

		// Parse #N serial prefix
		serial, cmd := parseSerial(line)
		cmdCtx.Serial = serial

		// Handle based on current stage
		stage := proc.Stage()
		switch stage {
		case StageRegistration:
			// Stage 1: Parse registration commands
			if s.handleRegistrationLine(proc, line) {
				continue
			}
			// Fall through to normal dispatch if not a registration command

		case StageCapability:
			// Stage 3: Parse capability commands
			if s.handleCapabilityLine(proc, line) {
				continue
			}
			// Fall through to normal dispatch if not a capability command

		case StageInit, StageConfig, StageRegistry, StageReady, StageRunning:
			// Other stages: fall through to normal dispatch
		}

		// Check for register/unregister before normal dispatch (legacy/runtime)
		tokens := tokenize(cmd)
		if len(tokens) > 0 {
			switch strings.ToLower(tokens[0]) {
			case "register":
				s.handleRegisterCommand(proc, serial, tokens[1:])
				continue
			case "unregister":
				s.handleUnregisterCommand(proc, serial, tokens[1:])
				continue
			}
		}

		// Handle "ready" command (Stage 5)
		if cmd == "ready" {
			// Signal Stage 5 complete
			if s.coordinator != nil {
				s.coordinator.StageComplete(proc.Index(), StageReady)

				// Wait for all plugins to be ready before signaling reactor
				stageCtx, cancel := context.WithTimeout(s.ctx, defaultStageTimeout)
				err := s.coordinator.WaitForStage(stageCtx, StageRunning)
				cancel()
				if err != nil {
					slog.Error("stage timeout waiting for running stage", "plugin", proc.Name(), "error", err)
					s.coordinator.PluginFailed(proc.Index(), fmt.Sprintf("stage timeout: %v", err))
					return
				}
			}

			proc.SetStage(StageRunning)
			if s.reactor != nil {
				s.reactor.SignalAPIReady()
			}
			continue
		}

		// Dispatch command
		resp, err := s.dispatcher.Dispatch(cmdCtx, cmd)
		if err != nil {
			// ErrSilent means suppress response entirely
			if errors.Is(err, ErrSilent) {
				continue
			}
			resp = &Response{Status: "error", Data: err.Error()}
		}

		// Send response only if serial present (serial = ack)
		if serial != "" && resp != nil {
			resp.Serial = serial
			respJSON, _ := json.Marshal(resp)
			_ = proc.WriteEvent(string(respJSON))
		}
	}
}

// handleRegistrationLine handles Stage 1 registration commands.
// Returns true if line was handled, false if should fall through to normal dispatch.
func (s *Server) handleRegistrationLine(proc *Process, line string) bool {
	reg := proc.Registration()
	if err := reg.ParseLine(line); err != nil {
		return false
	}
	if !reg.Done {
		return true
	}

	reg.Name = proc.config.Name
	if err := s.registry.Register(reg); err != nil {
		s.handlePluginConflict(proc, reg.Name, "plugin registration conflict", err)
		return true
	}

	s.progressThroughStages(proc, reg.Name, stageProgression{
		from: StageRegistration, mid: StageConfig, to: StageCapability,
		deliver: s.deliverConfig,
	})
	return true
}

// handleCapabilityLine handles Stage 3 capability commands.
// Returns true if line was handled, false if should fall through to normal dispatch.
func (s *Server) handleCapabilityLine(proc *Process, line string) bool {
	caps := proc.Capabilities()
	if err := caps.ParseLine(line); err != nil {
		return false
	}
	if !caps.Done {
		return true
	}

	caps.PluginName = proc.config.Name
	if err := s.capInjector.AddPluginCapabilities(caps); err != nil {
		s.handlePluginConflict(proc, caps.PluginName, "plugin capability conflict", err)
		return true
	}

	s.progressThroughStages(proc, caps.PluginName, stageProgression{
		from: StageCapability, mid: StageRegistry, to: StageReady,
		deliver: s.deliverRegistry,
	})
	return true
}

// GetPluginCapabilities returns all plugin-declared capabilities for OPEN injection.
func (s *Server) GetPluginCapabilities() []InjectedCapability {
	if s.capInjector == nil {
		return nil
	}
	return s.capInjector.GetCapabilities()
}

// deliverConfig sends matching configuration to a plugin (Stage 2).
// Matches registered config patterns against peer capability configs.
func (s *Server) deliverConfig(proc *Process) {
	reg := proc.Registration()
	if len(reg.ConfigPatterns) == 0 || s.reactor == nil {
		_ = proc.WriteEvent("config done")
		return
	}

	// Get peer capability configs from reactor
	peerConfigs := s.reactor.GetPeerCapabilityConfigs()

	// For each peer, try to match patterns and deliver config
	for _, peerCfg := range peerConfigs {
		context := "peer " + peerCfg.Address

		for _, pattern := range reg.ConfigPatterns {
			// Try to match pattern against known config paths
			matches := matchConfigPattern(pattern, peerCfg)
			for name, value := range matches {
				if value != "" {
					line := FormatConfigDelivery(context, name, value)
					_ = proc.WriteEvent(line)
				}
			}
		}
	}

	_ = proc.WriteEvent("config done")
}

// matchConfigPattern tries to match a config pattern against peer config.
// Returns map of capture-name → value for any matches.
// Uses the flexible Values map to support any capability type.
func matchConfigPattern(pattern *ConfigPattern, cfg PeerCapabilityConfig) map[string]string {
	result := make(map[string]string)

	// Try to match pattern against each capability value
	// Pattern format: "peer * capability <type> <capture>"
	for capName, capValue := range cfg.Values {
		if capValue == "" {
			continue
		}

		// Build config path string to match against pattern
		path := "peer " + cfg.Address + " capability " + capName + " " + capValue

		match := pattern.Match(path)
		if match != nil {
			for name, val := range match.Captures {
				result[name] = val
			}
		}
	}

	return result
}

// deliverRegistry sends the command registry to a plugin (Stage 4).
func (s *Server) deliverRegistry(proc *Process) {
	reg := proc.Registration()
	allCommands := s.registry.BuildCommandInfo()
	lines := FormatRegistrySharing(reg.Name, allCommands)

	for _, line := range lines {
		_ = proc.WriteEvent(line)
	}
}

// handlePluginFailed handles a "ready failed" message from a plugin.
// This can occur at any stage and indicates startup failure.
// Signals the coordinator to abort startup for all plugins.
func (s *Server) handlePluginFailed(proc *Process, line string) {
	// Parse: "ready failed text <message>" or "ready failed b64 <message>"
	parts := strings.SplitN(line, " ", 4)
	errMsg := "plugin startup failed"
	if len(parts) >= 4 {
		errMsg = parts[3]
	}

	// Log the failure with structured logging
	slog.Error("plugin startup failed",
		"plugin", proc.Name(),
		"stage", proc.Stage().String(),
		"error", errMsg,
	)

	// Signal coordinator to abort startup
	if s.coordinator != nil {
		s.coordinator.PluginFailed(proc.Index(), errMsg)
	}

	// Stop the process
	proc.Stop()
}

// handleRegisterCommand processes a register command from a process.
func (s *Server) handleRegisterCommand(proc *Process, serial string, tokens []string) {
	def, err := parseRegisterCommand(tokens)
	if err != nil {
		if serial != "" {
			resp := &Response{Serial: serial, Status: "error", Data: err.Error()}
			respJSON, _ := json.Marshal(resp)
			_ = proc.WriteEvent(string(respJSON))
		}
		return
	}

	results := s.dispatcher.Registry().Register(proc, []CommandDef{*def})
	result := results[0]

	if result.OK {
		proc.AddRegisteredCommand(def.Name)
	}

	if serial != "" {
		var resp *Response
		if result.OK {
			resp = &Response{Serial: serial, Status: "done"}
		} else {
			resp = &Response{Serial: serial, Status: "error", Data: result.Error}
		}
		respJSON, _ := json.Marshal(resp)
		_ = proc.WriteEvent(string(respJSON))
	}
}

// handleUnregisterCommand processes an unregister command from a process.
func (s *Server) handleUnregisterCommand(proc *Process, serial string, tokens []string) {
	name, err := parseUnregisterCommand(tokens)
	if err != nil {
		if serial != "" {
			resp := &Response{Serial: serial, Status: "error", Data: err.Error()}
			respJSON, _ := json.Marshal(resp)
			_ = proc.WriteEvent(string(respJSON))
		}
		return
	}

	s.dispatcher.Registry().Unregister(proc, []string{name})
	proc.RemoveRegisteredCommand(name)

	if serial != "" {
		resp := &Response{Serial: serial, Status: "done"}
		respJSON, _ := json.Marshal(resp)
		_ = proc.WriteEvent(string(respJSON))
	}
}

// handlePluginResponse handles a response from a plugin process.
func (s *Server) handlePluginResponse(_ *Process, serial, respType, data string) {
	pending := s.dispatcher.Pending()

	switch respType {
	case statusDone:
		var respData any
		if data != "" {
			// Try to parse as JSON
			if err := json.Unmarshal([]byte(data), &respData); err != nil {
				respData = data // Use as string if not valid JSON
			}
		}
		pending.Complete(serial, &Response{Status: statusDone, Data: respData})

	case statusError:
		pending.Complete(serial, &Response{Status: statusError, Data: data})

	case "partial":
		var respData any
		if data != "" {
			if err := json.Unmarshal([]byte(data), &respData); err != nil {
				respData = data
			}
		}
		pending.Partial(serial, &Response{Status: statusDone, Partial: true, Data: respData})
	}
}

// cleanupProcess handles cleanup when a process exits.
func (s *Server) cleanupProcess(proc *Process) {
	// Unregister all commands from this process
	s.dispatcher.Registry().UnregisterAll(proc)

	// Cancel all pending requests
	s.dispatcher.Pending().CancelAll(proc)
}

// handleClient creates and manages a client connection.
func (s *Server) handleClient(conn net.Conn) {
	id := s.clientID.Add(1)
	clientID := string(rune('0'+id%10)) + conn.RemoteAddr().String()

	clientCtx, clientCancel := context.WithCancel(s.ctx)

	client := &Client{
		id:     clientID,
		conn:   conn,
		server: s,
		ctx:    clientCtx,
		cancel: clientCancel,
	}

	s.mu.Lock()
	s.clients[clientID] = client
	s.mu.Unlock()

	s.wg.Add(1)
	go s.clientLoop(client)
}

// clientLoop reads and processes commands from a client.
func (s *Server) clientLoop(client *Client) {
	defer s.wg.Done()
	defer s.removeClient(client)
	defer func() { _ = client.conn.Close() }()

	reader := bufio.NewReader(client.conn)

	for {
		select {
		case <-client.ctx.Done():
			return
		default:
		}

		// Read line
		line, err := reader.ReadString('\n')
		if err != nil {
			return // Client disconnected
		}

		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Skip comments (lines starting with "# ")
		if isComment(line) {
			continue
		}

		// Process command (handles #N serial prefix)
		s.processCommand(client, line)
	}
}

// processCommand dispatches a command and sends response.
func (s *Server) processCommand(client *Client, line string) {
	// Parse #N serial prefix
	serial, command := parseSerial(line)

	ctx := &CommandContext{
		Reactor:       s.reactor,
		Encoder:       s.encoder,
		CommitManager: s.commitManager,
		Dispatcher:    s.dispatcher,
		Serial:        serial,
		// Note: Process is nil for socket clients - session commands are no-ops
	}

	resp, err := s.dispatcher.Dispatch(ctx, command)
	if err != nil {
		// ErrSilent means suppress response entirely
		if errors.Is(err, ErrSilent) {
			return
		}
		// Send error response
		resp = &Response{
			Status: "error",
			Data:   err.Error(),
		}
	}

	// Socket clients always get responses, serial in JSON body
	if resp != nil {
		resp.Serial = serial
		s.sendResponse(client, resp)
	}
}

// sendResponse sends a JSON response to the client.
// Serial is included in JSON body, not as prefix.
func (s *Server) sendResponse(client *Client, resp *Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		// Fallback error response
		data = []byte(`{"status":"error","data":"json marshal failed"}`)
	}

	data = append(data, '\n')
	_, _ = client.conn.Write(data)
}

// removeClient removes a client from tracking.
func (s *Server) removeClient(client *Client) {
	s.mu.Lock()
	delete(s.clients, client.id)
	s.mu.Unlock()
}

// OnMessageReceived handles raw BGP messages from peers.
// Forwards to processes based on per-peer API bindings.
// Implements reactor.MessageReceiver interface.
//
// This is called for ALL message types (UPDATE, OPEN, NOTIFICATION, KEEPALIVE).
// Each peer can have different bindings with different encodings and message filters.
func (s *Server) OnMessageReceived(peer PeerInfo, msg RawMessage) {
	if s.procManager == nil {
		return
	}

	// Get peer-specific API bindings from reactor
	bindings := s.reactor.GetPeerProcessBindings(peer.Address)
	if len(bindings) == 0 {
		return
	}

	for _, binding := range bindings {
		if !wantsMessageType(binding, msg.Type) {
			continue
		}

		proc := s.procManager.GetProcess(binding.PluginName)
		if proc == nil {
			continue
		}

		// Format using THIS BINDING's config (empty overrideDir = use msg.Direction)
		output := s.formatMessage(peer, msg, binding, "")
		_ = proc.WriteEvent(output)
	}
}

// wantsMessageType checks if binding wants this message type.
// State events are NOT BGP messages - handled separately via OnPeerStateChange.
func wantsMessageType(binding PeerProcessBinding, msgType message.MessageType) bool {
	switch msgType { //nolint:exhaustive // Only handle supported types
	case message.TypeUPDATE:
		return binding.ReceiveUpdate
	case message.TypeOPEN:
		return binding.ReceiveOpen
	case message.TypeNOTIFICATION:
		return binding.ReceiveNotification
	case message.TypeKEEPALIVE:
		return binding.ReceiveKeepalive
	default:
		return false
	}
}

// formatMessage formats a BGP message using the binding's encoding and format.
// overrideDir overrides msg.Direction if non-empty (used for sent messages).
func (s *Server) formatMessage(peer PeerInfo, msg RawMessage, binding PeerProcessBinding, overrideDir string) string {
	// Build ContentConfig from binding
	content := ContentConfig{
		Encoding: binding.Encoding,
		Format:   binding.Format,
	}.WithDefaults()

	// Compute effective direction
	direction := msg.Direction
	if overrideDir != "" {
		direction = overrideDir
	}

	switch msg.Type { //nolint:exhaustive // Only handle supported types
	case message.TypeUPDATE:
		// UPDATE messages support format (parsed/raw/full)
		return FormatMessage(peer, msg, content, overrideDir)

	case message.TypeOPEN:
		// Other message types only use encoding (json/text)
		decoded := DecodeOpen(msg.RawBytes)
		if content.Encoding == EncodingJSON {
			return s.encoder.Open(peer, decoded, direction, msg.MessageID)
		}
		return FormatOpen(peer, decoded, direction, msg.MessageID)

	case message.TypeNOTIFICATION:
		decoded := DecodeNotification(msg.RawBytes)
		if content.Encoding == EncodingJSON {
			return s.encoder.Notification(peer, decoded, direction, msg.MessageID)
		}
		return FormatNotification(peer, decoded, direction, msg.MessageID)

	case message.TypeKEEPALIVE:
		if content.Encoding == EncodingJSON {
			return s.encoder.Keepalive(peer, direction, msg.MessageID)
		}
		return FormatKeepalive(peer, direction, msg.MessageID)

	default:
		return ""
	}
}

// OnPeerStateChange handles peer state transitions.
// Called by reactor when peer state changes (not a BGP message).
// State events are separate from BGP protocol messages.
func (s *Server) OnPeerStateChange(peer PeerInfo, state string) {
	if s.procManager == nil {
		return
	}

	bindings := s.reactor.GetPeerProcessBindings(peer.Address)
	for _, binding := range bindings {
		if !binding.ReceiveState {
			continue
		}

		proc := s.procManager.GetProcess(binding.PluginName)
		if proc == nil {
			continue
		}

		encoding := binding.Encoding
		if encoding == "" {
			encoding = EncodingText
		}

		output := FormatStateChange(peer, state, encoding)
		_ = proc.WriteEvent(output)
	}
}

// OnMessageSent handles BGP messages sent to peers.
// Forwards to processes that subscribed to sent events.
// Called by reactor after successfully sending UPDATE to peer.
func (s *Server) OnMessageSent(peer PeerInfo, msg RawMessage) {
	if s.procManager == nil {
		return
	}

	// Only forward UPDATE messages for now (sent events)
	if msg.Type != message.TypeUPDATE {
		return
	}

	// Get peer-specific API bindings from reactor
	bindings := s.reactor.GetPeerProcessBindings(peer.Address)
	if len(bindings) == 0 {
		return
	}

	for _, binding := range bindings {
		if !binding.ReceiveSent {
			continue
		}

		proc := s.procManager.GetProcess(binding.PluginName)
		if proc == nil {
			continue
		}

		// Format using THIS BINDING's config
		output := s.formatSentMessage(peer, msg, binding)
		_ = proc.WriteEvent(output)
	}
}

// formatSentMessage formats a sent UPDATE message.
func (s *Server) formatSentMessage(peer PeerInfo, msg RawMessage, binding PeerProcessBinding) string {
	// Build ContentConfig from binding
	content := ContentConfig{
		Encoding: binding.Encoding,
		Format:   binding.Format,
	}.WithDefaults()

	// For sent events, use "sent" type instead of "update"
	// The message body is the same format as update events
	return FormatSentMessage(peer, msg, content)
}
