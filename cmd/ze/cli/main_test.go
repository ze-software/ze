package cli

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	unicli "codeberg.org/thomas-mangin/ze/internal/component/cli"
)

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

// TestPrintFormatted verifies response formatting.
//
// VALIDATES: Different output formats render correctly.
// PREVENTS: Formatting bugs causing garbled output.
func TestPrintFormatted(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		format   string
		contains []string
	}{
		{
			name:     "empty_output",
			output:   "",
			format:   "yaml",
			contains: []string{"OK"},
		},
		{
			name:     "json_data_yaml_format",
			output:   `{"version":"1.0"}`,
			format:   "yaml",
			contains: []string{"version", "1.0"},
		},
		{
			name:     "json_data_json_format",
			output:   `{"version":"1.0"}`,
			format:   "json",
			contains: []string{"version", "1.0"},
		},
		{
			name:     "plain_text",
			output:   "some plain text",
			format:   "yaml",
			contains: []string{"some plain text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(t, false, func() {
				printFormatted(tt.output, tt.format)
			})

			for _, want := range tt.contains {
				if !strings.Contains(output, want) {
					t.Errorf("printFormatted() output = %q, want to contain %q", output, want)
				}
			}
		})
	}
}

// TestPrintFormattedNestedData verifies nested data formatting.
//
// VALIDATES: Nested maps and arrays format with proper indentation.
// PREVENTS: Nested data being flattened or misformatted.
func TestPrintFormattedNestedData(t *testing.T) {
	data := map[string]any{
		"peers": []any{
			map[string]any{"Address": "10.0.0.1", "State": "established"},
			map[string]any{"Address": "10.0.0.2", "State": "idle"},
		},
		"config": map[string]any{
			"local": map[string]any{"as": 65000},
		},
		"empty-list": []any{},
	}
	jsonBytes, _ := json.Marshal(data)

	output := captureOutput(t, false, func() {
		printFormatted(string(jsonBytes), "yaml")
	})

	// Check peer formatting
	if !strings.Contains(output, "10.0.0.1") {
		t.Errorf("output missing peer address: %q", output)
	}

	// Check empty list handling
	if !strings.Contains(output, "[]") {
		t.Errorf("output should show '[]' for empty list: %q", output)
	}

	// Check nested map
	if !strings.Contains(output, "local") {
		t.Errorf("output missing nested config: %q", output)
	}
}

// TestPrintFormattedStringList verifies string list formatting.
//
// VALIDATES: String arrays format as bullet points.
// PREVENTS: String lists being printed as Go slice syntax.
func TestPrintFormattedStringList(t *testing.T) {
	data := map[string]any{
		"commands": []any{
			"daemon shutdown",
			"peer list",
			"system help",
		},
	}
	jsonBytes, _ := json.Marshal(data)

	output := captureOutput(t, false, func() {
		printFormatted(string(jsonBytes), "yaml")
	})

	if !strings.Contains(output, "daemon shutdown") {
		t.Errorf("output missing command in list: %q", output)
	}

	if !strings.Contains(output, "- ") {
		t.Errorf("output should format list items with '- ': %q", output)
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
	for _, c := range topLevel {
		if _, ok := tree.Children[c]; !ok {
			t.Errorf("missing top-level command: %s", c)
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
		t.Fatal("bgp rib command missing")
		return
	}
	for _, sub := range []string{"status", "routes", "best", "clear"} {
		if _, ok := rib.Children[sub]; !ok {
			t.Errorf("bgp rib missing subcommand: %s", sub)
		}
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

// TestBuildRuntimeTree_FallbackToStatic verifies that buildRuntimeTree falls back
// to the static command tree when the daemon is unreachable.
//
// VALIDATES: buildRuntimeTree returns static tree on SSH error.
// PREVENTS: nil tree or panic when daemon not reachable.
func TestBuildRuntimeTree_FallbackToStatic(t *testing.T) {
	// Client with invalid credentials — SendCommand will fail
	client := &cliClient{}

	tree := buildRuntimeTree(client)
	if tree == nil {
		t.Fatal("buildRuntimeTree returned nil")
		return
	}

	// Should fall back to static tree which has standard commands
	if _, ok := tree.Children["peer"]; !ok {
		t.Error("expected 'peer' in fallback tree")
	}
	if _, ok := tree.Children["daemon"]; !ok {
		t.Error("expected 'daemon' in fallback tree")
	}
}

// TestHistoryUpDown verifies Up/Down arrow navigation through command history
// using the unified cli.Model.
//
// VALIDATES: History recall via Up/Down arrows works correctly.
// PREVENTS: History browsing returning wrong entries or panicking.
func TestHistoryUpDown(t *testing.T) {
	m := unicli.NewCommandModel()
	upKey := tea.KeyPressMsg{Code: tea.KeyUp}
	downKey := tea.KeyPressMsg{Code: tea.KeyDown}
	enterKey := tea.KeyPressMsg{Code: tea.KeyEnter}

	// Populate history by executing commands.
	for _, c := range []string{"peer list", "daemon status", "system help"} {
		m.SetInput(c)
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
	upKey := tea.KeyPressMsg{Code: tea.KeyUp}
	downKey := tea.KeyPressMsg{Code: tea.KeyDown}
	enterKey := tea.KeyPressMsg{Code: tea.KeyEnter}

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
		updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
		m = updated.(unicli.Model) //nolint:forcetypeassert,errcheck // test
		if m.InputValue() != "test" {
			t.Errorf("Up on empty history = %q, want 'test'", m.InputValue())
		}
	})

	t.Run("down", func(t *testing.T) {
		m := unicli.NewCommandModel()
		m.SetInput("test")
		updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
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
	enterKey := tea.KeyPressMsg{Code: tea.KeyEnter}
	upKey := tea.KeyPressMsg{Code: tea.KeyUp}

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

// VALIDATES: AC-9 — BuildCommandTree wires family ValueHints to rib node.
// PREVENTS: missing family completions in both CLI and shell.
func TestBuildCommandTreeFamilyValueHints(t *testing.T) {
	tree := BuildCommandTree(false)

	rib := tree.Children["rib"]
	if rib == nil {
		t.Fatal("bgp rib node missing from command tree")
	}

	if rib.ValueHints == nil {
		t.Fatal("bgp rib node should have ValueHints for address families")
	}

	hints := rib.ValueHints()
	if len(hints) == 0 {
		t.Fatal("bgp rib ValueHints returned no families")
	}

	// Check that both builtin and plugin-registered families are present.
	found := make(map[string]bool)
	for _, h := range hints {
		found[h.Text] = true
		if h.Type != "value" {
			t.Errorf("family hint %q should have Type 'value', got %q", h.Text, h.Type)
		}
	}
	for _, want := range []string{"ipv4/unicast", "ipv6/unicast", "ipv4/mpls-vpn", "l2vpn/evpn"} {
		if !found[want] {
			t.Errorf("bgp rib ValueHints missing family %q, got %v", want, hints)
		}
	}
}

// VALIDATES: AC-3 — BuildCommandTree wires log level ValueHints to log set node.
// PREVENTS: missing log level completions.
func TestBuildCommandTreeLogLevelValueHints(t *testing.T) {
	tree := BuildCommandTree(false)

	logNode := tree.Children["log"]
	if logNode == nil {
		t.Fatal("log node missing from command tree")
	}

	setNode := logNode.Children["set"]
	if setNode == nil {
		t.Fatal("log set node missing from command tree")
	}

	if setNode.ValueHints == nil {
		t.Fatal("log set node should have ValueHints for log levels")
	}

	hints := setNode.ValueHints()
	found := make(map[string]bool)
	for _, h := range hints {
		found[h.Text] = true
		if h.Type != "value" {
			t.Errorf("level hint %q should have Type 'value', got %q", h.Text, h.Type)
		}
	}
	for _, want := range []string{"debug", "info", "warn", "err", "disabled"} {
		if !found[want] {
			t.Errorf("log set ValueHints missing level %q", want)
		}
	}
}

// Tab completion tests (TestTabCycleDoesNotAppend, TestTabSingleSuggestion)
// were removed: they tested the old local model's suggestion cycling behavior.
// The unified cli.Model uses a different completion system (dropdown overlay
// with ghost text). Equivalent behavior is tested in
// internal/component/cli/model_test.go (TestTabOnListKeyShowsChildrenImmediately
// and the headless .et functional tests).
