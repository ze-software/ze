// Package testsyslog provides a UDP syslog server for functional tests.
//
// Usage:
//
//	srv := syslog.New(0) // Dynamic port
//	srv.Start(ctx)
//	defer srv.Close()
//	// ... run test that sends syslog messages ...
//	if srv.Match("subsystem=server") {
//	    // expected log found
//	}
package syslog

import (
	"context"
	"errors"
	"net"
	"regexp"
	"slices"
	"sync"
	"time"
)

// Server is a UDP syslog server that captures messages for testing.
type Server struct {
	port     int
	conn     *net.UDPConn
	messages []string
	mu       sync.Mutex
	done     chan struct{}
}

// New creates a new test syslog server.
// Pass port=0 for dynamic port assignment.
func New(port int) *Server {
	return &Server{
		port: port,
		done: make(chan struct{}),
	}
}

// Start begins listening for UDP syslog messages.
// The server runs until Close() is called or context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	addr := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: s.port,
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	s.conn = conn

	// Update port if dynamically assigned
	if s.port == 0 {
		if udpAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			s.port = udpAddr.Port
		}
	}

	// Start receiver goroutine
	go s.receive(ctx)

	return nil
}

// receive reads messages until context is cancelled or connection is closed.
func (s *Server) receive(ctx context.Context) {
	defer close(s.done)

	buf := make([]byte, 65535) // Max UDP packet size

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Set read deadline to allow checking context periodically
		if err := s.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			return
		}

		n, _, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			// Check if it's a timeout (expected when checking context)
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			// Connection closed or other error
			return
		}

		if n > 0 {
			msg := string(buf[:n])
			s.mu.Lock()
			s.messages = append(s.messages, msg)
			s.mu.Unlock()
		}
	}
}

// Port returns the port the server is listening on.
func (s *Server) Port() int {
	return s.port
}

// Messages returns all captured messages.
func (s *Server) Messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Return a copy to avoid race conditions
	result := make([]string, len(s.messages))
	copy(result, s.messages)
	return result
}

// Match returns true if any captured message matches the regex pattern.
func (s *Server) Match(pattern string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.ContainsFunc(s.messages, re.MatchString)
}

// Close stops the server and releases resources.
func (s *Server) Close() error {
	if s.conn != nil {
		err := s.conn.Close()
		// Wait for receiver to finish (with timeout)
		select {
		case <-s.done:
		default:
		}
		return err
	}
	return nil
}
