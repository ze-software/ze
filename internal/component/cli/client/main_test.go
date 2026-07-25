package client

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	unicli "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/core/env"
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
	topLevel := []string{"peer", "show", "clear", "request", "system"}
	for _, c := range topLevel {
		if _, ok := tree.Children[c]; !ok {
			t.Errorf("missing top-level command: %s", c)
		}
	}

	// Check lifecycle commands under request and show
	reqNode := tree.Children["request"]
	if reqNode == nil {
		t.Fatal("request command missing")
		return
	}
	if _, ok := reqNode.Children["shutdown"]; !ok {
		t.Error("request missing shutdown subcommand")
	}
	showNode := tree.Children["show"]
	if showNode == nil {
		t.Fatal("show command missing")
		return
	}
	if _, ok := showNode.Children["status"]; !ok {
		t.Error("daemon missing status subcommand")
	}

	show := tree.Children["show"]
	if show == nil {
		t.Fatal("show command missing")
		return
	}

	// Check peer command families
	peer := tree.Children["peer"]
	if peer == nil {
		t.Fatal("peer command missing")
		return
	}
	reqPeer := reqNode.Children["peer"]
	if reqPeer == nil {
		t.Fatal("request peer command missing")
	} else if _, ok := reqPeer.Children["teardown"]; !ok {
		t.Error("request peer missing teardown subcommand")
	}
	if show.Children["bgp"] == nil || show.Children["bgp"].Children["peer"] == nil {
		t.Fatal("show bgp peer command missing")
		return
	}
	showPeer := show.Children["bgp"].Children["peer"]
	if _, ok := showPeer.Children["list"]; !ok {
		t.Error("show bgp peer missing list subcommand")
	}
	if _, ok := showPeer.Children["detail"]; !ok {
		t.Error("show bgp peer missing detail subcommand")
	}

	// Check RIB subcommands (proxy handlers forwarding to bgp-rib plugin).
	if show.Children["bgp"] == nil || show.Children["bgp"].Children["rib"] == nil {
		t.Fatal("show bgp rib command missing")
		return
	}
	rib := show.Children["bgp"].Children["rib"]
	for _, sub := range []string{"status", "best", "rpf"} {
		if _, ok := rib.Children[sub]; !ok {
			t.Errorf("show bgp rib missing subcommand: %s", sub)
		}
	}
	clear := tree.Children["clear"]
	if clear == nil || clear.Children["bgp"] == nil || clear.Children["bgp"].Children["rib"] == nil {
		t.Fatal("clear bgp rib command missing")
		return
	}
	for _, sub := range []string{"in", "out"} {
		if _, ok := clear.Children["bgp"].Children["rib"].Children[sub]; !ok {
			t.Errorf("clear bgp rib missing subcommand: %s", sub)
		}
	}
	request := tree.Children["request"]
	if request == nil || request.Children["bgp"] == nil || request.Children["bgp"].Children["rib"] == nil {
		t.Fatal("request bgp rib command missing")
		return
	}
	for _, sub := range []string{"inject", "withdraw"} {
		if _, ok := request.Children["bgp"].Children["rib"].Children[sub]; !ok {
			t.Errorf("request bgp rib missing subcommand: %s", sub)
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
	if _, ok := tree.Children["request"]; !ok {
		t.Error("expected 'request' in fallback tree")
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

// VALIDATES: AC-9 — BuildCommandTree wires family ValueHints to show bgp rib node.
// PREVENTS: missing family completions in both CLI and shell.
func TestBuildCommandTreeFamilyValueHints(t *testing.T) {
	tree := BuildCommandTree(false)

	show := tree.Children["show"]
	if show == nil || show.Children["bgp"] == nil || show.Children["bgp"].Children["rib"] == nil {
		t.Fatal("show bgp rib node missing from command tree")
	}
	rib := show.Children["bgp"].Children["rib"]

	if rib.ValueHints == nil {
		t.Fatal("show bgp rib node should have ValueHints for address families")
	}

	hints := rib.ValueHints()
	if len(hints) == 0 {
		t.Fatal("show bgp rib ValueHints returned no families")
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
			t.Errorf("show bgp rib ValueHints missing family %q, got %v", want, hints)
		}
	}
}

// VALIDATES: log set node has level ArgDef with enum values for completion.
// PREVENTS: missing log level completions after YANG migration.
func TestBuildCommandTreeLogLevelArgDefs(t *testing.T) {
	tree := BuildCommandTree(false)

	reqNode := tree.Children["request"]
	if reqNode == nil {
		t.Fatal("request node missing from command tree")
	}

	logNode := reqNode.Children["log"]
	if logNode == nil {
		t.Fatal("request log node missing from command tree")
	}

	levelNode := logNode.Children["level"]
	if levelNode == nil {
		t.Fatal("request log level node missing from command tree")
	}

	if len(levelNode.ArgDefs) == 0 {
		t.Fatal("request log level node should have ArgDefs for log levels")
	}

	found := make(map[string]bool)
	for _, def := range levelNode.ArgDefs {
		for _, v := range def.EnumValues {
			found[v] = true
		}
	}
	for _, want := range []string{"debug", "info", "warn", "err", "disabled"} {
		if !found[want] {
			t.Errorf("request log level ArgDefs missing level %q", want)
		}
	}
}

// VALIDATES: AC-1 — show env get and show env registered nodes get env-key ValueHints.
// PREVENTS: env-key completion missing from operational CLI and shell show path.
func TestBuildCommandTreeEnvValueHints(t *testing.T) {
	tree := BuildCommandTree(false)

	show := tree.Children["show"]
	if show == nil {
		t.Fatal("show command missing from command tree")
	}

	envNode := show.Children["env"]
	if envNode == nil {
		t.Fatal("show env node missing from command tree")
	}

	for _, sub := range []string{"get", "registered"} {
		node := envNode.Children[sub]
		if node == nil {
			t.Errorf("show env %s node missing from command tree", sub)
			continue
		}
		if node.ValueHints == nil {
			t.Errorf("show env %s node should have ValueHints for env keys", sub)
			continue
		}
		hints := node.ValueHints()
		if len(hints) == 0 {
			t.Errorf("show env %s ValueHints returned no entries", sub)
			continue
		}

		hasPublic := false
		for _, h := range hints {
			if h.Text == "ze.test.environ.public" || strings.HasPrefix(h.Text, "ze.") {
				hasPublic = true
			}
			if h.Type != "value" {
				t.Errorf("env hint %q should have Type 'value', got %q", h.Text, h.Type)
			}
		}
		if !hasPublic {
			t.Errorf("show env %s ValueHints should include public env keys", sub)
		}
	}
}

// VALIDATES: AC-7 — env ValueHints never include Private entries.
// PREVENTS: private env vars leaking through completion.
func TestBuildCommandTreeEnvHintsExcludePrivate(t *testing.T) {
	tree := BuildCommandTree(false)
	show := tree.Children["show"]
	if show == nil || show.Children["env"] == nil || show.Children["env"].Children["get"] == nil {
		t.Fatal("show env get node not available in command tree")
	}
	node := show.Children["env"].Children["get"]
	if node.ValueHints == nil {
		t.Fatal("show env get should have ValueHints")
	}

	privateKeys := make(map[string]bool)
	for _, e := range env.AllEntries() {
		if e.Private {
			privateKeys[e.Key] = true
		}
	}

	for _, h := range node.ValueHints() {
		if privateKeys[h.Text] {
			t.Errorf("env ValueHints should not contain Private entry %q", h.Text)
		}
	}
}

// Tab completion tests (TestTabCycleDoesNotAppend, TestTabSingleSuggestion)
// were removed: they tested the old local model's suggestion cycling behavior.
// The unified cli.Model uses a different completion system (dropdown overlay
// with ghost text). Equivalent behavior is tested in
// internal/component/cli/model_test.go (TestTabOnListKeyShowsChildrenImmediately
// and the headless .et functional tests).
