package cli

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	unicli "codeberg.org/thomas-mangin/ze/internal/component/cli"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// mockServer simulates the API server using NUL-framed JSON RPC.
type mockServer struct {
	listener net.Listener
	path     string
	done     chan struct{}
	commands []any // Override ze-system:command-list response (nil = use default)
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
	defer conn.Close() //nolint:forcetypeassert,errcheck // test cleanup

	reader := rpc.NewFrameReader(conn)
	writer := rpc.NewFrameWriter(conn)

	for {
		msg, err := reader.Read()
		if err != nil {
			return
		}

		var req rpc.Request
		if err := json.Unmarshal(msg, &req); err != nil {
			resp := rpc.NewError(nil, "invalid-json", "invalid JSON request")
			data, merr := json.Marshal(resp)
			if merr != nil {
				return
			}
			if werr := writer.Write(data); werr != nil {
				return
			}
			continue
		}

		result := s.handleRPC(req.Method)
		data, err := json.Marshal(result)
		if err != nil {
			return
		}
		if err := writer.Write(data); err != nil {
			return
		}
	}
}

func (s *mockServer) handleRPC(method string) any {
	switch method {
	case "ze-system:version-software":
		v, d := pluginserver.GetVersion()
		return &rpc.RPCResult{Result: mustJSON(map[string]any{"version": v, "build-date": d})}
	case "ze-system:help":
		return &rpc.RPCResult{Result: mustJSON(map[string]any{
			"commands": []string{"daemon shutdown", "peer list", "system help"},
		})}
	case "ze-system:daemon-status":
		return &rpc.RPCResult{Result: mustJSON(map[string]any{
			"uptime":     "1h30m",
			"peer-count": 2,
		})}
	case "ze-bgp:peer-list":
		return &rpc.RPCResult{Result: mustJSON(map[string]any{
			"peers": []any{
				map[string]any{"Address": "10.0.0.1", "State": "established"},
				map[string]any{"Address": "10.0.0.2", "State": "idle"},
			},
		})}
	case "ze-system:command-list":
		return &rpc.RPCResult{Result: mustJSON(s.commandListResponse())}
	default:
		return rpc.NewError(nil, "unknown-method", "unknown method")
	}
}

// commandListResponse returns the command list for ze-system:command-list.
// By default includes "rib status" but omits "rib show" to test proxy filtering.
func (s *mockServer) commandListResponse() map[string]any {
	if s.commands != nil {
		return map[string]any{"commands": s.commands}
	}
	// Default: only "rib status" is available (not "rib show")
	return map[string]any{
		"commands": []any{
			map[string]any{"value": "rib status"},
		},
	}
}

func mustJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic("mustJSON: " + err.Error())
	}
	return data
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

// TestCLIClient_SendCommand verifies the CLI sends JSON RPC and receives responses.
//
// VALIDATES: CLI correctly maps text commands to wire methods and decodes responses.
// PREVENTS: Protocol mismatch between CLI and API server.
func TestCLIClient_SendCommand(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	client, err := newCLIClient(server.path)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close() //nolint:forcetypeassert,errcheck // test cleanup

	tests := []struct {
		name     string
		command  string
		wantErr  string // non-empty means expect this error
		wantData bool   // expect result data
	}{
		{"help", "system help", "", true},
		{"status", "daemon status", "", true},
		{"peers", "peer list", "", true},
		{"unknown", "nonexistent command", "unknown command", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.SendCommand(tt.command)
			if err != nil {
				t.Fatalf("SendCommand() unexpected error: %v", err)
			}
			if tt.wantErr != "" {
				if !strings.Contains(resp.Error, tt.wantErr) {
					t.Errorf("SendCommand() error = %q, want containing %q", resp.Error, tt.wantErr)
				}
			} else {
				if resp.Error != "" {
					t.Errorf("SendCommand() unexpected error: %s", resp.Error)
				}
				if tt.wantData && len(resp.Result) == 0 {
					t.Error("SendCommand() expected result data, got none")
				}
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
	defer client.Close() //nolint:forcetypeassert,errcheck // test cleanup

	var code int
	output := captureOutput(t, false, func() {
		code = client.Execute("system help", "yaml")
	})

	if code != 0 {
		t.Errorf("Execute() returned %d, want 0", code)
	}

	if !strings.Contains(output, "commands") {
		t.Errorf("Execute() output = %q, want to contain 'commands'", output)
	}
}

func TestCLIClient_ExecuteError(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	client, err := newCLIClient(server.path)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close() //nolint:forcetypeassert,errcheck // test cleanup

	var code int
	output := captureOutput(t, true, func() {
		code = client.Execute("nonexistent command", "yaml")
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
	defer client.Close() //nolint:forcetypeassert,errcheck // test cleanup

	// Send multiple commands on same connection
	commands := []string{"system help", "daemon status", "peer list"}

	for _, cmd := range commands {
		resp, err := client.SendCommand(cmd)
		if err != nil {
			t.Errorf("SendCommand(%q) error = %v", cmd, err)
		}
		if resp.Error != "" {
			t.Errorf("SendCommand(%q) unexpected error: %s", cmd, resp.Error)
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
		code = Run([]string{"--socket", server.path, "--run", "system help"})
	})

	if code != 0 {
		t.Errorf("Run(--run) returned %d, want 0", code)
	}

	if !strings.Contains(output, "commands") {
		t.Errorf("Run(--run) output = %q, want to contain 'commands'", output)
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
		code = Run([]string{"--socket", server.path, "--run", "nonexistent command"})
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
		code = Run([]string{"bgp", "--socket", server.path, "--run", "system help"})
	})

	if code != 0 {
		t.Errorf("Run(bgp --run) returned %d, want 0", code)
	}

	if !strings.Contains(output, "commands") {
		t.Errorf("Run(bgp --run) output = %q, want to contain 'commands'", output)
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
	client := &cliClient{}

	tests := []struct {
		name     string
		resp     *rpcResponse
		wantErr  bool
		contains []string
	}{
		{
			name:     "ok_no_data",
			resp:     &rpcResponse{},
			contains: []string{"OK"},
		},
		{
			name:     "with_data",
			resp:     &rpcResponse{Result: mustJSON(map[string]any{"version": "1.0"})},
			contains: []string{"version", "1.0"},
		},
		{
			name:     "error_response",
			resp:     &rpcResponse{Error: "something failed"},
			wantErr:  true,
			contains: []string{"error", "something failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output string
			if tt.wantErr {
				output = captureOutput(t, true, func() {
					client.printFormatted(tt.resp, "yaml")
				})
			} else {
				output = captureOutput(t, false, func() {
					client.printFormatted(tt.resp, "yaml")
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
	client := &cliClient{}

	resp := &rpcResponse{
		Result: mustJSON(map[string]any{
			"peers": []any{
				map[string]any{"Address": "10.0.0.1", "State": "established"},
				map[string]any{"Address": "10.0.0.2", "State": "idle"},
			},
			"config": map[string]any{
				"local-as": 65000,
			},
			"empty-list": []any{},
		}),
	}

	output := captureOutput(t, false, func() {
		client.printFormatted(resp, "yaml")
	})

	// Check peer formatting (special case with Address)
	if !strings.Contains(output, "10.0.0.1") {
		t.Errorf("output missing peer address: %q", output)
	}

	// Check empty list handling
	if !strings.Contains(output, "[]") {
		t.Errorf("output should show '[]' for empty list: %q", output)
	}

	// Check nested map
	if !strings.Contains(output, "local-as") {
		t.Errorf("output missing nested config: %q", output)
	}
}

// TestCommandTree verifies command tree structure.
//
// VALIDATES: Command tree has expected commands and hierarchy.
// PREVENTS: Typos in command names or broken hierarchy.
func TestCommandTree(t *testing.T) {
	tree := BuildCommandTree(false)

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
		return
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
		return
	}
	if _, ok := peer.Children["list"]; !ok {
		t.Error("peer missing list subcommand")
	}
	if _, ok := peer.Children["detail"]; !ok {
		t.Error("peer missing detail subcommand")
	}

	// Check rib subcommands (proxy handlers forwarding to bgp-rib plugin)
	rib := tree.Children["rib"]
	if rib == nil {
		t.Fatal("rib command missing")
		return
	}
	for _, sub := range []string{"status", "routes", "best", "clear"} {
		if _, ok := rib.Children[sub]; !ok {
			t.Errorf("rib missing subcommand: %s", sub)
		}
	}
}

// TestCLIClient_ResponseWithStringList verifies string list formatting.
//
// VALIDATES: String arrays format as bullet points.
// PREVENTS: String lists being printed as Go slice syntax.
func TestCLIClient_ResponseWithStringList(t *testing.T) {
	client := &cliClient{}

	resp := &rpcResponse{
		Result: mustJSON(map[string]any{
			"commands": []any{
				"daemon shutdown",
				"peer list",
				"system help",
			},
		}),
	}

	output := captureOutput(t, false, func() {
		client.printFormatted(resp, "yaml")
	})

	// Should contain list items formatted as "- item"
	if !strings.Contains(output, "daemon shutdown") {
		t.Errorf("output missing command in list: %q", output)
	}

	if !strings.Contains(output, "- ") {
		t.Errorf("output should format list items with '- ': %q", output)
	}
}

// TestErrorMessage verifies rpcResponse.ErrorMessage prefers Params.message over Error code.
//
// VALIDATES: Structured error detail is used for display when available.
// PREVENTS: Kebab-case error codes shown to users instead of readable messages.
func TestErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		resp rpcResponse
		want string
	}{
		{
			name: "params_message",
			resp: rpcResponse{
				Error:  "command-not-available",
				Params: json.RawMessage(`{"message":"command \"bgp rib routes\" not available (plugin may not be running)"}`),
			},
			want: `command "bgp rib routes" not available (plugin may not be running)`,
		},
		{
			name: "fallback_to_error",
			resp: rpcResponse{Error: "unknown-method"},
			want: "unknown-method",
		},
		{
			name: "empty_params_message",
			resp: rpcResponse{
				Error:  "some-error",
				Params: json.RawMessage(`{"message":""}`),
			},
			want: "some-error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resp.ErrorMessage()
			if got != tt.want {
				t.Errorf("ErrorMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestHistoryUpDown verifies Up/Down arrow navigation through command history
// using the unified cli.Model.
//
// VALIDATES: History recall via Up/Down arrows works correctly.
// PREVENTS: History browsing returning wrong entries or panicking.
func TestHistoryUpDown(t *testing.T) {
	m := unicli.NewCommandModel()
	upKey := tea.KeyMsg{Type: tea.KeyUp}
	downKey := tea.KeyMsg{Type: tea.KeyDown}
	enterKey := tea.KeyMsg{Type: tea.KeyEnter}

	// Populate history by executing commands.
	for _, cmd := range []string{"peer list", "daemon status", "system help"} {
		m.SetInput(cmd)
		updated, _ := m.Update(enterKey)
		m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
	}

	// Up once → most recent ("system help")
	updated, _ := m.Update(upKey)
	m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
	if m.InputValue() != "system help" {
		t.Errorf("first Up = %q, want 'system help'", m.InputValue())
	}

	// Up again → "daemon status"
	updated, _ = m.Update(upKey)
	m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
	if m.InputValue() != "daemon status" {
		t.Errorf("second Up = %q, want 'daemon status'", m.InputValue())
	}

	// Up again → "peer list"
	updated, _ = m.Update(upKey)
	m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
	if m.InputValue() != "peer list" {
		t.Errorf("third Up = %q, want 'peer list'", m.InputValue())
	}

	// Up at top → stays at "peer list"
	updated, _ = m.Update(upKey)
	m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
	if m.InputValue() != "peer list" {
		t.Errorf("Up at top = %q, want 'peer list'", m.InputValue())
	}

	// Down → "daemon status"
	updated, _ = m.Update(downKey)
	m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
	if m.InputValue() != "daemon status" {
		t.Errorf("Down = %q, want 'daemon status'", m.InputValue())
	}

	// Down → "system help"
	updated, _ = m.Update(downKey)
	m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
	if m.InputValue() != "system help" {
		t.Errorf("Down = %q, want 'system help'", m.InputValue())
	}

	// Down past end → restores original input
	updated, _ = m.Update(downKey)
	m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
	if m.InputValue() != "" {
		t.Errorf("Down past end = %q, want empty (original)", m.InputValue())
	}
}

// TestHistoryPreservesInput verifies current input is saved when browsing history
// using the unified cli.Model.
//
// VALIDATES: Partial input is restored when pressing Down past the end.
// PREVENTS: Losing user's in-progress input when browsing history.
func TestHistoryPreservesInput(t *testing.T) {
	m := unicli.NewCommandModel()
	upKey := tea.KeyMsg{Type: tea.KeyUp}
	downKey := tea.KeyMsg{Type: tea.KeyDown}
	enterKey := tea.KeyMsg{Type: tea.KeyEnter}

	// Populate history
	m.SetInput("peer list")
	updated, _ := m.Update(enterKey)
	m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test

	// Type partial input
	m.SetInput("daemon st")

	// Up → recalls "peer list", saves "daemon st"
	updated, _ = m.Update(upKey)
	m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
	if m.InputValue() != "peer list" {
		t.Errorf("Up = %q, want 'peer list'", m.InputValue())
	}

	// Down → restores "daemon st"
	updated, _ = m.Update(downKey)
	m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
	if m.InputValue() != "daemon st" {
		t.Errorf("Down = %q, want 'daemon st'", m.InputValue())
	}
}

// TestHistoryEmpty verifies Up/Down on empty history is a no-op
// using the unified cli.Model.
//
// VALIDATES: No crash when browsing history with no entries.
// PREVENTS: Index out of bounds on empty history.
func TestHistoryEmpty(t *testing.T) {
	t.Run("up", func(t *testing.T) {
		m := unicli.NewCommandModel()
		m.SetInput("test")
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
		if m.InputValue() != "test" {
			t.Errorf("Up on empty history = %q, want 'test'", m.InputValue())
		}
	})

	t.Run("down", func(t *testing.T) {
		m := unicli.NewCommandModel()
		m.SetInput("test")
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
		if m.InputValue() != "test" {
			t.Errorf("Down on empty history = %q, want 'test'", m.InputValue())
		}
	})
}

// TestHistoryDedup verifies consecutive duplicate commands are not stored twice
// using the unified cli.Model.
//
// VALIDATES: Duplicate consecutive commands produce single history entry.
// PREVENTS: History filling with repeated identical commands.
func TestHistoryDedup(t *testing.T) {
	m := unicli.NewCommandModel()
	enterKey := tea.KeyMsg{Type: tea.KeyEnter}
	upKey := tea.KeyMsg{Type: tea.KeyUp}

	// Type "peer list" and press Enter three times.
	for range 3 {
		m.SetInput("peer list")
		updated, _ := m.Update(enterKey)
		m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
	}

	// Navigate history: Up should give "peer list", next Up should stay (only one entry).
	updated, _ := m.Update(upKey)
	m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
	if m.InputValue() != "peer list" {
		t.Errorf("first Up = %q, want 'peer list'", m.InputValue())
	}
	updated, _ = m.Update(upKey)
	m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
	if m.InputValue() != "peer list" {
		t.Errorf("second Up = %q, want 'peer list' (should stay, single entry)", m.InputValue())
	}

	// Different command should be added.
	m.SetInput("daemon status")
	updated, _ = m.Update(enterKey)
	m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test

	// Up → "daemon status", Up → "peer list"
	updated, _ = m.Update(upKey)
	m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
	if m.InputValue() != "daemon status" {
		t.Errorf("Up after second cmd = %q, want 'daemon status'", m.InputValue())
	}
	updated, _ = m.Update(upKey)
	m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
	if m.InputValue() != "peer list" {
		t.Errorf("second Up = %q, want 'peer list'", m.InputValue())
	}
}

// Tab completion tests (TestTabCycleDoesNotAppend, TestTabSingleSuggestion)
// were removed: they tested the old local model's suggestion cycling behavior.
// The unified cli.Model uses a different completion system (dropdown overlay
// with ghost text). Equivalent behavior is tested in
// internal/component/cli/model_test.go (TestTabOnListKeyShowsChildrenImmediately
// and the headless .et functional tests).

// TestResolveCommand verifies text command to wire method mapping.
//
// VALIDATES: CLI commands resolve to correct wire methods.
// PREVENTS: Wrong RPC method being called for a text command.
func TestResolveCommand(t *testing.T) {
	client := &cliClient{
		cmdMap: map[string]string{
			"bgp peer list":           "ze-bgp:peer-list",
			"bgp peer detail":         "ze-bgp:peer-detail",
			"daemon status":           "ze-system:daemon-status",
			"system help":             "ze-system:help",
			"system version software": "ze-system:version-software",
		},
	}
	// Build sorted keys (longest first)
	keys := make([]string, 0, len(client.cmdMap))
	for k := range client.cmdMap {
		keys = append(keys, k)
	}
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if len(keys[j]) > len(keys[i]) {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	client.cmdKeys = keys

	tests := []struct {
		name       string
		input      string
		wantMethod string
		wantArgs   []string
	}{
		{"peer_list", "peer list", "ze-bgp:peer-list", nil},
		{"peer_detail_with_arg", "peer detail 10.0.0.1", "ze-bgp:peer-detail", []string{"10.0.0.1"}},
		{"daemon_status", "daemon status", "ze-system:daemon-status", nil},
		{"system_help", "system help", "ze-system:help", nil},
		{"unknown", "nonexistent", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, args := client.resolveCommand(tt.input)
			if method != tt.wantMethod {
				t.Errorf("resolveCommand(%q) method = %q, want %q", tt.input, method, tt.wantMethod)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("resolveCommand(%q) args = %v, want %v", tt.input, args, tt.wantArgs)
			}
		})
	}
}

// TestBuildRuntimeTree verifies proxy commands are filtered when plugin is not running.
//
// VALIDATES: buildRuntimeTree excludes proxy commands whose PluginCommand is not in daemon's command list.
// PREVENTS: Completions offered for commands that will fail because plugin is not running.
func TestBuildRuntimeTree(t *testing.T) {
	server := newMockServer(t)
	defer server.Close()

	client, err := newCLIClient(server.path)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close() //nolint:forcetypeassert,errcheck // test cleanup

	tree := buildRuntimeTree(client)
	if tree == nil {
		t.Fatal("buildRuntimeTree returned nil")
	}

	// "rib status" is in the mock's command list → rib.status should be present
	rib := tree.Children["rib"]
	if rib == nil {
		t.Fatal("expected 'rib' in runtime tree")
	}
	if _, ok := rib.Children["status"]; !ok {
		t.Error("expected 'rib status' in runtime tree (plugin command available)")
	}

	// "rib show" is NOT in the mock's command list → rib.routes should be absent
	if _, ok := rib.Children["routes"]; ok {
		t.Error("expected 'rib routes' to be filtered out (plugin command 'rib show' not available)")
	}

	// Non-proxy commands should always be present
	if _, ok := tree.Children["peer"]; !ok {
		t.Error("expected 'peer' in runtime tree (not a proxy command)")
	}
	if _, ok := tree.Children["daemon"]; !ok {
		t.Error("expected 'daemon' in runtime tree (not a proxy command)")
	}
}
