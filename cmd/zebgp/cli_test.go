package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockServer simulates the API server for testing.
type mockServer struct {
	listener net.Listener
	path     string
	done     chan struct{}
}

func newMockServer(t *testing.T) *mockServer {
	t.Helper()

	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "test.sock")

	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	s := &mockServer{
		listener: listener,
		path:     sockPath,
		done:     make(chan struct{}),
	}

	go s.serve()

	return s
}

func (s *mockServer) serve() {
	defer close(s.done)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *mockServer) handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	reader := bufio.NewReader(conn)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		cmd := strings.TrimSpace(line)
		resp := s.handleCommand(cmd)

		data, _ := json.Marshal(resp)
		_, _ = conn.Write(append(data, '\n'))
	}
}

func (s *mockServer) handleCommand(cmd string) map[string]any {
	switch cmd {
	case "system version":
		return map[string]any{
			"status": "done",
			"data": map[string]any{
				"version": "0.1.0",
			},
		}
	case "system help":
		return map[string]any{
			"status": "done",
			"data": map[string]any{
				"commands": []string{
					"daemon shutdown",
					"daemon status",
					"peer list",
					"system help",
					"system version",
				},
			},
		}
	case "daemon status":
		return map[string]any{
			"status": "done",
			"data": map[string]any{
				"uptime":     "1h30m",
				"peer_count": 2,
			},
		}
	case "peer list":
		return map[string]any{
			"status": "done",
			"data": map[string]any{
				"peers": []any{
					map[string]any{
						"Address": "10.0.0.1",
						"State":   "established",
					},
					map[string]any{
						"Address": "10.0.0.2",
						"State":   "idle",
					},
				},
			},
		}
	case "bad command":
		return map[string]any{
			"status": "error",
			"error":  "unknown command",
		}
	default:
		return map[string]any{
			"status": "done",
		}
	}
}

func (s *mockServer) Close() {
	_ = s.listener.Close()
	<-s.done
}

// captureOutput captures stdout or stderr during a function call.
func captureOutput(t *testing.T, isStderr bool, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	var old *os.File
	if isStderr {
		old = os.Stderr
		os.Stderr = w
	} else {
		old = os.Stdout
		os.Stdout = w
	}

	fn()

	_ = w.Close()
	if isStderr {
		os.Stderr = old
	} else {
		os.Stdout = old
	}

	out, _ := io.ReadAll(r)
	return string(out)
}

func TestCLIClient_SendCommand(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	client, err := newCLIClient(server.path)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	tests := []struct {
		name    string
		command string
		wantErr bool
		status  string
	}{
		{"version", "system version", false, "done"},
		{"help", "system help", false, "done"},
		{"status", "daemon status", false, "done"},
		{"peers", "peer list", false, "done"},
		{"error", "bad command", false, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.SendCommand(tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if resp.Status != tt.status {
				t.Errorf("SendCommand() status = %v, want %v", resp.Status, tt.status)
			}
		})
	}
}

func TestCLIClient_Execute(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	client, err := newCLIClient(server.path)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	var code int
	output := captureOutput(t, false, func() {
		code = client.Execute("system version")
	})

	if code != 0 {
		t.Errorf("Execute() returned %d, want 0", code)
	}

	if !strings.Contains(output, "version") {
		t.Errorf("Execute() output = %q, want to contain 'version'", output)
	}
}

func TestCLIClient_ExecuteError(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	client, err := newCLIClient(server.path)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	var code int
	output := captureOutput(t, true, func() {
		code = client.Execute("bad command")
	})

	if code != 1 {
		t.Errorf("Execute() returned %d, want 1", code)
	}

	if !strings.Contains(output, "error") {
		t.Errorf("Execute() stderr = %q, want to contain 'error'", output)
	}
}

func TestCLIClient_ConnectionError(t *testing.T) {
	_, err := newCLIClient("/nonexistent/socket.sock")
	if err == nil {
		t.Error("newCLIClient() should fail for nonexistent socket")
	}
}

func TestCLIClient_MultipleCommands(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	client, err := newCLIClient(server.path)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	// Send multiple commands on same connection
	commands := []string{"system version", "daemon status", "peer list"}

	for _, cmd := range commands {
		resp, err := client.SendCommand(cmd)
		if err != nil {
			t.Errorf("SendCommand(%q) error = %v", cmd, err)
		}
		if resp.Status != "done" {
			t.Errorf("SendCommand(%q) status = %v, want done", cmd, resp.Status)
		}
	}
}

// TestCmdCLI_RunFlag verifies cli --run executes a single command and exits.
//
// VALIDATES: cli --run "<command>" executes command and returns result.
// PREVENTS: regression when run command is merged into cli.
func TestCmdCLI_RunFlag(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	// Test successful command
	var code int
	output := captureOutput(t, false, func() {
		code = cmdCLI([]string{"--socket", server.path, "--run", "system version"})
	})

	if code != 0 {
		t.Errorf("cmdCLI(--run) returned %d, want 0", code)
	}

	if !strings.Contains(output, "version") {
		t.Errorf("cmdCLI(--run) output = %q, want to contain 'version'", output)
	}
}

// TestCmdCLI_RunFlagError verifies cli --run returns error code on failure.
//
// VALIDATES: cli --run returns non-zero on error.
// PREVENTS: error status being swallowed.
func TestCmdCLI_RunFlagError(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	var code int
	_ = captureOutput(t, true, func() {
		code = cmdCLI([]string{"--socket", server.path, "--run", "bad command"})
	})

	if code != 1 {
		t.Errorf("cmdCLI(--run error) returned %d, want 1", code)
	}
}
