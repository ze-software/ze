package cli

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
	defer conn.Close() //nolint:errcheck // test cleanup

	reader := bufio.NewReader(conn)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		cmd := strings.TrimSpace(line)
		resp := s.handleCommand(cmd)

		data, err := json.Marshal(resp)
		if err != nil {
			return
		}
		if _, err := conn.Write(append(data, '\n')); err != nil {
			return
		}
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
	default: // handle unknown commands gracefully in test
		return map[string]any{
			"status": "done",
		}
	}
}

func (s *mockServer) Close() {
	s.listener.Close() //nolint:errcheck,gosec // test cleanup
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

	w.Close() //nolint:errcheck,gosec // test cleanup
	if isStderr {
		os.Stderr = old
	} else {
		os.Stdout = old
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}
	return string(out)
}

func TestCLIClient_SendCommand(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	client, err := newCLIClient(server.path)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close() //nolint:errcheck // test cleanup

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
	defer client.Close() //nolint:errcheck // test cleanup

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
	defer client.Close() //nolint:errcheck // test cleanup

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
	defer client.Close() //nolint:errcheck // test cleanup

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

// TestRun_RunFlag verifies cli --run executes a single command and exits.
//
// VALIDATES: cli --run "<command>" executes command and returns result.
// PREVENTS: regression when run command is merged into cli.
func TestRun_RunFlag(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	// Test successful command
	var code int
	output := captureOutput(t, false, func() {
		code = Run([]string{"--socket", server.path, "--run", "system version"})
	})

	if code != 0 {
		t.Errorf("Run(--run) returned %d, want 0", code)
	}

	if !strings.Contains(output, "version") {
		t.Errorf("Run(--run) output = %q, want to contain 'version'", output)
	}
}

// TestRun_RunFlagError verifies cli --run returns error code on failure.
//
// VALIDATES: cli --run returns non-zero on error.
// PREVENTS: error status being swallowed.
func TestRun_RunFlagError(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	var code int
	captureOutput(t, true, func() {
		code = Run([]string{"--socket", server.path, "--run", "bad command"})
	})

	if code != 1 {
		t.Errorf("Run(--run error) returned %d, want 1", code)
	}
}

// TestRun_BgpSubsystem verifies explicit bgp subsystem works.
//
// VALIDATES: ze cli bgp --run works with explicit subsystem.
// PREVENTS: subsystem dispatch breaking.
func TestRun_BgpSubsystem(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	var code int
	output := captureOutput(t, false, func() {
		code = Run([]string{"bgp", "--socket", server.path, "--run", "system version"})
	})

	if code != 0 {
		t.Errorf("Run(bgp --run) returned %d, want 0", code)
	}

	if !strings.Contains(output, "version") {
		t.Errorf("Run(bgp --run) output = %q, want to contain 'version'", output)
	}
}

// TestRun_HelpFlags verifies all help flag variants work.
//
// VALIDATES: ze cli help, ze cli --help, ze cli -h all show usage.
// PREVENTS: help flags being mishandled or causing errors.
func TestRun_HelpFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"help", []string{"help"}},
		{"--help", []string{"--help"}},
		{"-h", []string{"-h"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var code int
			output := captureOutput(t, true, func() {
				code = Run(tt.args)
			})

			if code != 0 {
				t.Errorf("Run(%v) returned %d, want 0", tt.args, code)
			}

			if !strings.Contains(output, "Usage:") {
				t.Errorf("Run(%v) output = %q, want to contain 'Usage:'", tt.args, output)
			}

			if !strings.Contains(output, "ze cli") {
				t.Errorf("Run(%v) output = %q, want to contain 'ze cli'", tt.args, output)
			}
		})
	}
}

// TestRun_ConnectionFailure verifies error handling when daemon not running.
//
// VALIDATES: Graceful error message when socket connection fails.
// PREVENTS: Cryptic errors or panics on connection failure.
func TestRun_ConnectionFailure(t *testing.T) {
	var code int
	output := captureOutput(t, true, func() {
		code = Run([]string{"--socket", "/nonexistent/socket.sock", "--run", "test"})
	})

	if code != 1 {
		t.Errorf("Run() with bad socket returned %d, want 1", code)
	}

	if !strings.Contains(output, "cannot connect") {
		t.Errorf("Run() stderr = %q, want to contain 'cannot connect'", output)
	}

	if !strings.Contains(output, "hint:") {
		t.Errorf("Run() stderr = %q, want to contain 'hint:'", output)
	}
}

// TestRun_BgpSubsystemConnectionFailure verifies error with explicit subsystem.
//
// VALIDATES: ze cli bgp handles connection failure gracefully.
// PREVENTS: Subsystem dispatch masking connection errors.
func TestRun_BgpSubsystemConnectionFailure(t *testing.T) {
	var code int
	output := captureOutput(t, true, func() {
		code = Run([]string{"bgp", "--socket", "/nonexistent/socket.sock", "--run", "test"})
	})

	if code != 1 {
		t.Errorf("Run(bgp) with bad socket returned %d, want 1", code)
	}

	if !strings.Contains(output, "cannot connect") {
		t.Errorf("Run(bgp) stderr = %q, want to contain 'cannot connect'", output)
	}
}

// TestCLIClient_PrintResponse verifies response formatting.
//
// VALIDATES: Different response types format correctly.
// PREVENTS: Formatting bugs causing garbled output.
func TestCLIClient_PrintResponse(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	client, err := newCLIClient(server.path)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close() //nolint:errcheck // test cleanup

	tests := []struct {
		name     string
		resp     *cliResponse
		wantOut  string
		wantErr  bool
		contains []string
	}{
		{
			name:     "ok_no_data",
			resp:     &cliResponse{Status: "done"},
			contains: []string{"OK"},
		},
		{
			name: "with_data",
			resp: &cliResponse{
				Status: "done",
				Data:   map[string]any{"version": "1.0"},
			},
			contains: []string{"version", "1.0"},
		},
		{
			name: "error_response",
			resp: &cliResponse{
				Status: "error",
				Error:  "something failed",
			},
			wantErr:  true,
			contains: []string{"error", "something failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output string
			if tt.wantErr {
				output = captureOutput(t, true, func() {
					client.PrintResponse(tt.resp)
				})
			} else {
				output = captureOutput(t, false, func() {
					client.PrintResponse(tt.resp)
				})
			}

			for _, want := range tt.contains {
				if !strings.Contains(output, want) {
					t.Errorf("PrintResponse() output = %q, want to contain %q", output, want)
				}
			}
		})
	}
}

// TestCLIClient_PrintResponseNestedData verifies nested data formatting.
//
// VALIDATES: Nested maps and arrays format with proper indentation.
// PREVENTS: Nested data being flattened or misformatted.
func TestCLIClient_PrintResponseNestedData(t *testing.T) {
	// Create a minimal client just for testing PrintResponse
	// (doesn't need actual connection)
	client := &cliClient{}

	resp := &cliResponse{
		Status: "done",
		Data: map[string]any{
			"peers": []any{
				map[string]any{"Address": "10.0.0.1", "State": "established"},
				map[string]any{"Address": "10.0.0.2", "State": "idle"},
			},
			"config": map[string]any{
				"local_as": 65000,
			},
			"empty_list": []any{},
		},
	}

	output := captureOutput(t, false, func() {
		client.PrintResponse(resp)
	})

	// Check peer formatting (special case with Address)
	if !strings.Contains(output, "10.0.0.1") {
		t.Errorf("output missing peer address: %q", output)
	}

	// Check empty list handling
	if !strings.Contains(output, "(none)") {
		t.Errorf("output should show '(none)' for empty list: %q", output)
	}

	// Check nested map
	if !strings.Contains(output, "local_as") {
		t.Errorf("output missing nested config: %q", output)
	}
}

// TestCommandTree verifies command tree structure.
//
// VALIDATES: Command tree has expected commands and hierarchy.
// PREVENTS: Typos in command names or broken hierarchy.
func TestCommandTree(t *testing.T) {
	tree := buildCommandTree()

	// Check top-level commands exist
	topLevel := []string{"daemon", "peer", "rib", "system"}
	for _, cmd := range topLevel {
		if _, ok := tree.Children[cmd]; !ok {
			t.Errorf("missing top-level command: %s", cmd)
		}
	}

	// Check daemon subcommands
	daemon := tree.Children["daemon"]
	if daemon == nil {
		t.Fatal("daemon command missing")
	}
	if _, ok := daemon.Children["shutdown"]; !ok {
		t.Error("daemon missing shutdown subcommand")
	}
	if _, ok := daemon.Children["status"]; !ok {
		t.Error("daemon missing status subcommand")
	}

	// Check peer subcommands
	peer := tree.Children["peer"]
	if peer == nil {
		t.Fatal("peer command missing")
	}
	if _, ok := peer.Children["list"]; !ok {
		t.Error("peer missing list subcommand")
	}
	if _, ok := peer.Children["show"]; !ok {
		t.Error("peer missing show subcommand")
	}

	// Check rib hierarchy
	rib := tree.Children["rib"]
	if rib == nil {
		t.Fatal("rib command missing")
	}
	ribShow := rib.Children["show"]
	if ribShow == nil {
		t.Fatal("rib show command missing")
	}
	if _, ok := ribShow.Children["in"]; !ok {
		t.Error("rib show missing 'in' subcommand")
	}
	if _, ok := ribShow.Children["out"]; !ok {
		t.Error("rib show missing 'out' subcommand")
	}
}

// TestCLIClient_ResponseWithStringList verifies string list formatting.
//
// VALIDATES: String arrays format as bullet points.
// PREVENTS: String lists being printed as Go slice syntax.
func TestCLIClient_ResponseWithStringList(t *testing.T) {
	// Create a minimal client just for testing PrintResponse
	client := &cliClient{}

	resp := &cliResponse{
		Status: "done",
		Data: map[string]any{
			"commands": []any{
				"daemon shutdown",
				"peer list",
				"system help",
			},
		},
	}

	output := captureOutput(t, false, func() {
		client.PrintResponse(resp)
	})

	// Should contain list items formatted as "- item"
	if !strings.Contains(output, "daemon shutdown") {
		t.Errorf("output missing command in list: %q", output)
	}

	if !strings.Contains(output, "- ") {
		t.Errorf("output should format list items with '- ': %q", output)
	}
}
