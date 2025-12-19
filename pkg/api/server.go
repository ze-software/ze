package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Server manages API connections and command dispatch.
type Server struct {
	config     *ServerConfig
	reactor    ReactorInterface
	dispatcher *Dispatcher
	encoder    *JSONEncoder

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
		config:     config,
		reactor:    reactor,
		dispatcher: NewDispatcher(),
		encoder:    NewJSONEncoder("6.0.0"),
		clients:    make(map[string]*Client),
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

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.listener = listener
	s.running.Store(true)

	// Start accept loop
	s.wg.Add(1)
	go s.acceptLoop()

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
	_ = os.Remove(s.config.SocketPath)
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
		Reactor: s.reactor,
		Encoder: s.encoder,
	}

	resp, err := s.dispatcher.Dispatch(ctx, command)
	if err != nil {
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
