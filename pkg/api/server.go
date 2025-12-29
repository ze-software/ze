package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/message"
)

// Server manages API connections and command dispatch.
type Server struct {
	config        *ServerConfig
	reactor       ReactorInterface
	dispatcher    *Dispatcher
	encoder       *JSONEncoder
	commitManager *CommitManager
	procManager   *ProcessManager

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

	// Start external processes if configured
	if len(s.config.Processes) > 0 {
		s.procManager = NewProcessManager(s.config.Processes)
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

// handleSingleProcessCommands handles commands from a single process.
func (s *Server) handleSingleProcessCommands(proc *Process) {
	cmdCtx := &CommandContext{
		Reactor:       s.reactor,
		Encoder:       s.encoder,
		CommitManager: s.commitManager,
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

		// Dispatch command
		resp, err := s.dispatcher.Dispatch(cmdCtx, line)
		if err != nil {
			// ErrSilent means suppress response entirely
			if errors.Is(err, ErrSilent) {
				continue
			}
			resp = &Response{Status: "error", Error: err.Error()}
		}

		// Send response back to process stdin (if ack enabled or error)
		if resp != nil && (resp.Status == "error" || proc.AckEnabled()) {
			respJSON, _ := json.Marshal(resp)
			_ = proc.WriteEvent(strings.TrimSuffix(string(respJSON), "\n"))
		}
	}
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

		// Skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Process command
		s.processCommand(client, line)
	}
}

// processCommand dispatches a command and sends response.
func (s *Server) processCommand(client *Client, command string) {
	ctx := &CommandContext{
		Reactor:       s.reactor,
		Encoder:       s.encoder,
		CommitManager: s.commitManager,
		// Note: Process is nil for socket clients - session commands are no-ops
	}

	resp, err := s.dispatcher.Dispatch(ctx, command)
	if err != nil {
		// ErrSilent means suppress response entirely
		if errors.Is(err, ErrSilent) {
			return
		}
		// Send error response
		errResp := &Response{
			Status: "error",
			Error:  err.Error(),
		}
		s.sendResponse(client, errResp)
		return
	}

	s.sendResponse(client, resp)
}

// sendResponse sends a JSON response to the client.
func (s *Server) sendResponse(client *Client, resp *Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		// Fallback error response
		data = []byte(`{"status":"error","error":"json marshal failed"}`)
	}

	// Append newline
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
// Forwards to processes based on their content config (encoding + format).
// Implements reactor.MessageReceiver interface.
//
// This is called for ALL message types (UPDATE, OPEN, NOTIFICATION, KEEPALIVE).
// Processes can filter by message type via their receive config.
func (s *Server) OnMessageReceived(peer PeerInfo, msg RawMessage) {
	if s.procManager == nil {
		return
	}

	switch msg.Type { //nolint:exhaustive // ROUTE-REFRESH not yet supported
	case message.TypeUPDATE:
		routes := DecodeUpdateRoutes(msg.RawBytes)
		s.forwardUpdateToProcesses(peer, routes)
	case message.TypeOPEN:
		decoded := DecodeOpen(msg.RawBytes)
		s.forwardOpenToProcesses(peer, decoded)
	case message.TypeNOTIFICATION:
		decoded := DecodeNotification(msg.RawBytes)
		s.forwardNotificationToProcesses(peer, decoded)
	case message.TypeKEEPALIVE:
		s.forwardKeepaliveToProcesses(peer)
	}
}

// forwardUpdateToProcesses forwards parsed UPDATE routes to all subscribed processes.
func (s *Server) forwardUpdateToProcesses(peer PeerInfo, routes []ReceivedRoute) {
	// Pre-convert to RouteUpdate once for all JSON processes
	var updates []RouteUpdate
	if len(routes) > 0 {
		updates = make([]RouteUpdate, len(routes))
		for i, r := range routes {
			updates[i] = r.ToRouteUpdate()
		}
	}

	s.procManager.mu.RLock()
	defer s.procManager.mu.RUnlock()

	for name, proc := range s.procManager.processes {
		cfg := s.getProcessConfigByName(name)
		if cfg == nil || !cfg.ReceiveUpdate {
			continue
		}

		// Format based on encoding config
		var output string
		if cfg.Encoder == EncodingJSON {
			output = s.encoder.RouteAnnounce(peer, updates)
		} else {
			if len(routes) > 0 {
				output = FormatReceivedUpdate(peer.Address, routes)
			} else {
				output = "neighbor " + peer.Address.String() + " receive update\n"
			}
		}

		_ = proc.WriteEvent(output)
	}
}

// forwardOpenToProcesses forwards parsed OPEN to all subscribed processes.
func (s *Server) forwardOpenToProcesses(peer PeerInfo, open DecodedOpen) {
	s.procManager.mu.RLock()
	defer s.procManager.mu.RUnlock()

	for name, proc := range s.procManager.processes {
		cfg := s.getProcessConfigByName(name)
		if cfg == nil || !cfg.ReceiveUpdate { // TODO: Add ReceiveOpen flag in Phase 1
			continue
		}

		var output string
		if cfg.Encoder == EncodingJSON {
			output = s.encoder.Open(peer, open)
		} else {
			output = FormatOpen(peer.Address, open)
		}

		_ = proc.WriteEvent(output)
	}
}

// forwardNotificationToProcesses forwards parsed NOTIFICATION to all subscribed processes.
func (s *Server) forwardNotificationToProcesses(peer PeerInfo, notify DecodedNotification) {
	s.procManager.mu.RLock()
	defer s.procManager.mu.RUnlock()

	for name, proc := range s.procManager.processes {
		cfg := s.getProcessConfigByName(name)
		if cfg == nil || !cfg.ReceiveUpdate { // TODO: Add ReceiveNotification flag in Phase 1
			continue
		}

		var output string
		if cfg.Encoder == EncodingJSON {
			output = s.encoder.Notification(peer, notify)
		} else {
			output = FormatNotification(peer.Address, notify)
		}

		_ = proc.WriteEvent(output)
	}
}

// forwardKeepaliveToProcesses forwards KEEPALIVE event to all subscribed processes.
func (s *Server) forwardKeepaliveToProcesses(peer PeerInfo) {
	s.procManager.mu.RLock()
	defer s.procManager.mu.RUnlock()

	for name, proc := range s.procManager.processes {
		cfg := s.getProcessConfigByName(name)
		if cfg == nil || !cfg.ReceiveUpdate { // TODO: Add ReceiveKeepalive flag in Phase 1
			continue
		}

		var output string
		if cfg.Encoder == EncodingJSON {
			output = s.encoder.Keepalive(peer)
		} else {
			output = FormatKeepalive(peer.Address)
		}

		_ = proc.WriteEvent(output)
	}
}

// getProcessConfigByName finds a process config by name.
func (s *Server) getProcessConfigByName(name string) *ProcessConfig {
	for i := range s.config.Processes {
		if s.config.Processes[i].Name == name {
			return &s.config.Processes[i]
		}
	}
	return nil
}
