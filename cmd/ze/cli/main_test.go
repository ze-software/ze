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

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// mockServer simulates the API server using NUL-framed JSON RPC.
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

	reader := rpc.NewFrameReader(conn)
	writer := rpc.NewFrameWriter(conn)

	for {
		msg, err := reader.Read()
		if err != nil {
			return
		}

		var req rpc.Request
		if err := json.Unmarshal(msg, &req); err != nil {
			resp := &rpc.RPCError{Error: "invalid-json"}
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
	default:
		return &rpc.RPCError{Error: "unknown-method"}
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
	defer client.Close() //nolint:errcheck // test cleanup

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
	defer client.Close() //nolint:errcheck // test cleanup

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
	defer client.Close() //nolint:errcheck // test cleanup

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
	defer client.Close() //nolint:errcheck // test cleanup

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
	// Note: "rib" commands moved to bgp-rib plugin (SDK protocol), not in AllBuiltinRPCs().
	topLevel := []string{"daemon", "peer", "system"}
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
	if _, ok := peer.Children["show"]; !ok {
		t.Error("peer missing show subcommand")
	}

	// Note: rib commands are registered by the bgp-rib plugin via SDK protocol,
	// not via AllBuiltinRPCs(), so they don't appear in BuildCommandTree().
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

// TestHistoryUpDown verifies Up/Down arrow navigation through command history.
//
// VALIDATES: History recall via Up/Down arrows works correctly.
// PREVENTS: History browsing returning wrong entries or panicking.
func TestHistoryUpDown(t *testing.T) {
	m := model{
		textInput:  textinput.New(),
		historyIdx: -1,
		history:    []string{"peer list", "daemon status", "system help"},
	}

	// Up once → most recent ("system help")
	m = m.handleHistoryUp()
	if m.textInput.Value() != "system help" {
		t.Errorf("first Up = %q, want 'system help'", m.textInput.Value())
	}
	if m.historyIdx != 2 {
		t.Errorf("historyIdx = %d, want 2", m.historyIdx)
	}

	// Up again → "daemon status"
	m = m.handleHistoryUp()
	if m.textInput.Value() != "daemon status" {
		t.Errorf("second Up = %q, want 'daemon status'", m.textInput.Value())
	}

	// Up again → "peer list"
	m = m.handleHistoryUp()
	if m.textInput.Value() != "peer list" {
		t.Errorf("third Up = %q, want 'peer list'", m.textInput.Value())
	}

	// Up at top → stays at "peer list"
	m = m.handleHistoryUp()
	if m.textInput.Value() != "peer list" {
		t.Errorf("Up at top = %q, want 'peer list'", m.textInput.Value())
	}

	// Down → "daemon status"
	m = m.handleHistoryDown()
	if m.textInput.Value() != "daemon status" {
		t.Errorf("Down = %q, want 'daemon status'", m.textInput.Value())
	}

	// Down → "system help"
	m = m.handleHistoryDown()
	if m.textInput.Value() != "system help" {
		t.Errorf("Down = %q, want 'system help'", m.textInput.Value())
	}

	// Down past end → restores original input
	m = m.handleHistoryDown()
	if m.textInput.Value() != "" {
		t.Errorf("Down past end = %q, want empty (original)", m.textInput.Value())
	}
	if m.historyIdx != -1 {
		t.Errorf("historyIdx = %d, want -1 after restoring", m.historyIdx)
	}
}

// TestHistoryPreservesInput verifies current input is saved when browsing history.
//
// VALIDATES: Partial input is restored when pressing Down past the end.
// PREVENTS: Losing user's in-progress input when browsing history.
func TestHistoryPreservesInput(t *testing.T) {
	m := model{
		textInput:  textinput.New(),
		historyIdx: -1,
		history:    []string{"peer list"},
	}
	m.textInput.SetValue("daemon st")

	// Up → recalls "peer list", saves "daemon st"
	m = m.handleHistoryUp()
	if m.textInput.Value() != "peer list" {
		t.Errorf("Up = %q, want 'peer list'", m.textInput.Value())
	}
	if m.historyTmp != "daemon st" {
		t.Errorf("historyTmp = %q, want 'daemon st'", m.historyTmp)
	}

	// Down → restores "daemon st"
	m = m.handleHistoryDown()
	if m.textInput.Value() != "daemon st" {
		t.Errorf("Down = %q, want 'daemon st'", m.textInput.Value())
	}
}

// TestHistoryEmpty verifies Up/Down on empty history is a no-op.
//
// VALIDATES: No crash when browsing history with no entries.
// PREVENTS: Index out of bounds on empty history.
func TestHistoryEmpty(t *testing.T) {
	m := model{
		textInput:  textinput.New(),
		historyIdx: -1,
	}
	m.textInput.SetValue("test")

	m = m.handleHistoryUp()
	if m.textInput.Value() != "test" {
		t.Errorf("Up on empty history = %q, want 'test'", m.textInput.Value())
	}

	m = m.handleHistoryDown()
	if m.textInput.Value() != "test" {
		t.Errorf("Down on empty history = %q, want 'test'", m.textInput.Value())
	}
}

// TestHistoryDedup verifies consecutive duplicate commands are not stored twice.
//
// VALIDATES: Duplicate consecutive commands produce single history entry.
// PREVENTS: History filling with repeated identical commands.
func TestHistoryDedup(t *testing.T) {
	m := model{
		textInput:  textinput.New(),
		historyIdx: -1,
	}

	enterKey := tea.KeyMsg{Type: tea.KeyEnter}

	// Type "peer list" and press Enter three times through Update().
	for range 3 {
		m.textInput.SetValue("peer list")
		updated, _ := m.Update(enterKey)
		m, _ = updated.(model) //nolint:errcheck // test: type is always model
	}

	if len(m.history) != 1 {
		t.Errorf("history len = %d, want 1 (dedup)", len(m.history))
	}

	// Different command should be added.
	m.textInput.SetValue("daemon status")
	updated, _ := m.Update(enterKey)
	m, _ = updated.(model) //nolint:errcheck // test: type is always model

	if len(m.history) != 2 {
		t.Errorf("history len = %d, want 2", len(m.history))
	}
}

// TestTabCycleDoesNotAppend verifies Tab cycling replaces rather than appends.
//
// VALIDATES: Pressing Tab multiple times cycles through completions at the same position.
// PREVENTS: Tab appending suggestions repeatedly (e.g., "peer plugin peer plugin...").
func TestTabCycleDoesNotAppend(t *testing.T) {
	m := model{
		textInput:  textinput.New(),
		historyIdx: -1,
		suggestions: []suggestion{
			{text: "peer", description: "Peer management"},
			{text: "plugin", description: "Plugin management"},
		},
		selected: -1,
	}
	m.textInput.SetValue("p")

	tabKey := tea.KeyMsg{Type: tea.KeyTab}

	// First Tab: should select "peer" (index 0 after increment from -1→0)
	updated, _ := m.Update(tabKey)
	m, _ = updated.(model) //nolint:errcheck // test: type is always model
	if got := m.textInput.Value(); got != "peer " {
		t.Fatalf("first Tab = %q, want %q", got, "peer ")
	}

	// Second Tab: should replace with "plugin", not append
	updated, _ = m.Update(tabKey)
	m, _ = updated.(model) //nolint:errcheck // test: type is always model
	if got := m.textInput.Value(); got != "plugin " {
		t.Fatalf("second Tab = %q, want %q", got, "plugin ")
	}

	// Third Tab: should cycle back to "peer"
	updated, _ = m.Update(tabKey)
	m, _ = updated.(model) //nolint:errcheck // test: type is always model
	if got := m.textInput.Value(); got != "peer " {
		t.Fatalf("third Tab = %q, want %q", got, "peer ")
	}
}

// TestTabSingleSuggestion verifies Tab with one suggestion applies once, not twice.
//
// VALIDATES: Single suggestion Tab applies once, subsequent Tabs are no-ops.
// PREVENTS: Tab producing "peer peer " when only one suggestion matches.
func TestTabSingleSuggestion(t *testing.T) {
	m := model{
		textInput:  textinput.New(),
		historyIdx: -1,
		suggestions: []suggestion{
			{text: "peer", description: "Peer management"},
		},
		selected: -1,
	}
	m.textInput.SetValue("pee")

	tabKey := tea.KeyMsg{Type: tea.KeyTab}

	// First Tab: applies "peer"
	updated, _ := m.Update(tabKey)
	m, _ = updated.(model) //nolint:errcheck // test: type is always model
	if got := m.textInput.Value(); got != "peer " {
		t.Fatalf("first Tab = %q, want %q", got, "peer ")
	}

	// Second Tab: should be no-op (only one suggestion)
	updated, _ = m.Update(tabKey)
	m, _ = updated.(model) //nolint:errcheck // test: type is always model
	if got := m.textInput.Value(); got != "peer " {
		t.Fatalf("second Tab = %q, want %q (should be no-op)", got, "peer ")
	}
}

// TestResolveCommand verifies text command to wire method mapping.
//
// VALIDATES: CLI commands resolve to correct wire methods.
// PREVENTS: Wrong RPC method being called for a text command.
func TestResolveCommand(t *testing.T) {
	client := &cliClient{
		cmdMap: map[string]string{
			"bgp peer list":           "ze-bgp:peer-list",
			"bgp peer show":           "ze-bgp:peer-show",
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
		{"peer_show_with_arg", "peer show 10.0.0.1", "ze-bgp:peer-show", []string{"10.0.0.1"}},
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
