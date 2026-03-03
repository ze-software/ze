// Design: docs/architecture/api/process-protocol.md — client connection management
// Overview: server.go — Server struct and lifecycle

package server

import (
	"context"
	"encoding/json"
	"net"

	"codeberg.org/thomas-mangin/ze/internal/core/ipc"
)

// Client represents a connected API client.
type Client struct {
	id     string
	conn   net.Conn
	server *Server

	ctx    context.Context
	cancel context.CancelFunc
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

// clientLoop reads NUL-framed JSON RPC requests and dispatches them.
func (s *Server) clientLoop(client *Client) {
	defer s.wg.Done()
	defer s.removeClient(client)
	defer client.conn.Close() //nolint:errcheck // best-effort cleanup on defer

	reader := ipc.NewFrameReader(client.conn)
	writer := ipc.NewFrameWriter(client.conn)

	for {
		select {
		case <-client.ctx.Done():
			return
		default: // non-blocking context check
		}

		msg, err := reader.Read()
		if err != nil {
			return // Client disconnected or read error
		}

		if len(msg) == 0 {
			continue
		}

		var req ipc.Request
		if err := json.Unmarshal(msg, &req); err != nil {
			errResp := &ipc.RPCError{Error: "invalid-json"}
			if writeErr := s.writeRPCResponse(writer, errResp); writeErr != nil {
				return
			}
			continue
		}

		result := s.rpcDispatcher.Dispatch(&req)
		if writeErr := s.writeRPCResponse(writer, result); writeErr != nil {
			return
		}
	}
}

// writeRPCResponse marshals and writes an RPC response via NUL-framed writer.
// Returns error if the write fails (caller should close connection).
func (s *Server) writeRPCResponse(writer *ipc.FrameWriter, result any) error {
	respJSON, err := json.Marshal(result)
	if err != nil {
		logger().Warn("failed to marshal RPC response", "error", err)
		return err
	}
	return writer.Write(respJSON)
}

// removeClient removes a client from tracking.
func (s *Server) removeClient(client *Client) {
	s.mu.Lock()
	delete(s.clients, client.id)
	s.mu.Unlock()
}
