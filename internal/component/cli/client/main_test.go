package client

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	unicli "github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/env"
	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
	"github.com/ze-software/ze/internal/core/textbuf"
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

// TestCLIFormatFlagBecomesAPipe proves that --format travels to the daemon as a
// format pipe, and that a format operator the operator typed still beats it.
//
// VALIDATES: spec-fixit-cli-format-default-everywhere AC-5. `--format json`
//
//	reaches the daemon as `| json` whatever the configured default is,
//	and no flag appends nothing, which is what lets the daemon apply
//	`environment cli format default`.
//
// PREVENTS:  two defects at once. The first is the client deciding the format
//
//	itself: nothing on its startup path loads the configuration, so it
//	can only ever see the registered default, and `ze cli -c "show
//	version"` answered YAML however the operator had configured the
//	daemon. The second is the flag overriding an explicit pipe: the
//	flag's old "yaml" default fed the pipe's JSON back into a local
//	renderer, so every consumer that asked for JSON on the command line
//	got YAML with exit code 0
//	(plan/journal/silent-fall-through.md, 2026-08-14).
func TestCLIFormatFlagBecomesAPipe(t *testing.T) {
	tests := []struct {
		name    string
		command string
		format  string
		want    string
	}{
		{
			name:    "no flag leaves the command alone",
			command: "show version",
			format:  "",
			want:    "show version",
		},
		{
			name:    "the flag becomes a format pipe",
			command: "show version",
			format:  "json",
			want:    "show version | json",
		},
		{
			name:    "the flag joins a chain that names no format",
			command: "show bgp peer list | match established",
			format:  "table",
			want:    "show bgp peer list | match established | table",
		},
		{
			name:    "an explicit format pipe beats the flag",
			command: "show bgp peer list | json compact",
			format:  "yaml",
			want:    "show bgp peer list | json compact",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commandWithFormat(tt.command, tt.format); got != tt.want {
				t.Errorf("commandWithFormat(%q, %q) = %q, want %q", tt.command, tt.format, got, tt.want)
			}
		})
	}
}

// TestTheOperatorSeesTheDaemonRenderingWhileItArrives holds the other half of
// the client's job: it prints, and it does not format.
//
// The answer is written in pieces, as the daemon produces it, so each case is
// driven twice: once as one write and once split byte by byte. A shape that
// depends on holding the whole answer fails the second run, which is what makes
// this a control on the streaming rather than a repeat of the collected form.
//
// VALIDATES: spec-fixit-cli-format-default-everywhere AC-7, and AC-3 of the
//
//	streaming answer protocol. An empty answer prints "OK" when the
//	command names no format operator, and prints nothing when it names
//	one, because "OK" is not valid JSON. A non-empty answer reaches the
//	caller unchanged, with exactly one trailing newline.
//
// PREVENTS:  the client re-rendering an answer the daemon already rendered, an
//
//	empty command losing the "OK" that tells a human it worked, and a
//	streamed answer gaining or losing a newline at either edge.
func TestTheOperatorSeesTheDaemonRenderingWhileItArrives(t *testing.T) {
	tests := []struct {
		name    string
		command string
		output  string
		want    string
	}{
		{
			name:    "empty answer with no format pipe says OK",
			command: "request reload",
			output:  "",
			want:    "OK\n",
		},
		{
			name:    "empty answer with a format pipe says nothing",
			command: "request reload | json",
			output:  "",
			want:    "",
		},
		{
			name:    "a whitespace-only answer is an empty one",
			command: "request reload",
			output:  "\n\n",
			want:    "OK\n",
		},
		{
			name:    "the daemon rendering is printed unchanged",
			command: "show version",
			output:  "version  ze 26.08.18",
			want:    "version  ze 26.08.18\n",
		},
		{
			name:    "a trailing newline is not doubled",
			command: "show version",
			output:  "version  ze 26.08.18\n",
			want:    "version  ze 26.08.18\n",
		},
		{
			name:    "a table rendering keeps its interior newlines",
			command: "show bgp peer list | table",
			output:  "┌─────┐\n│ a   │\n└─────┘\n",
			want:    "┌─────┐\n│ a   │\n└─────┘\n",
		},
		{
			name:    "a blank line inside a rendering survives",
			command: "show version",
			output:  "first\n\nlast\n\n\n",
			want:    "first\n\nlast\n",
		},
	}

	for _, tt := range tests {
		for _, split := range []struct {
			name  string
			write func(w io.Writer, s string)
		}{
			{name: "one write", write: func(w io.Writer, s string) {
				if s != "" {
					w.Write([]byte(s)) //nolint:errcheck // a bytes.Buffer never fails
				}
			}},
			{name: "one byte at a time", write: func(w io.Writer, s string) {
				for i := range len(s) {
					w.Write([]byte{s[i]}) //nolint:errcheck // a bytes.Buffer never fails
				}
			}},
		} {
			t.Run(tt.name+", "+split.name, func(t *testing.T) {
				var got bytes.Buffer
				out := newDaemonOutput(&got, tt.command, nil)
				split.write(out, tt.output)
				if err := out.Close(); err != nil {
					t.Fatalf("Close: %v", err)
				}
				if got.String() != tt.want {
					t.Errorf("the operator saw %q, want %q", got.String(), tt.want)
				}
			})
		}
	}
}

// TestTheTranscriptRecordsWhatTheOperatorSaw pins the one caller that keeps a
// copy of a streamed answer.
//
// VALIDATES: the transcript still holds the rendering after the client stopped
//
//	collecting it.
//
// PREVENTS:  a session recording holding an empty answer for every command,
//
//	which is what a streaming client that kept no copy would record.
func TestTheTranscriptRecordsWhatTheOperatorSaw(t *testing.T) {
	var transcript textbuf.Buffer
	var got bytes.Buffer
	out := newDaemonOutput(&got, "show version", &transcript)
	for _, piece := range []string{"version ", " ze", " 26.08.18\n"} {
		if _, err := out.Write([]byte(piece)); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if recorded := out.Transcript(); recorded != "version  ze 26.08.18" {
		t.Errorf("the transcript recorded %q, want %q", recorded, "version  ze 26.08.18")
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
	// Client with invalid credentials — the transport will fail
	client := newCLIClient(sshclient.Credentials{})

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

// rawOnlyJSONClient builds a client whose transport behaves like a daemon with
// `environment cli format default table` committed: it answers JSON only to a
// caller that asked for the raw pipe, and a table rendering to everybody else.
//
// The returned slice records every command that reached the channel, so a test
// can assert which of the two a caller asked for rather than inferring it.
func rawOnlyJSONClient(jsonAnswer string) (*cliClient, *[]string) {
	sent := new([]string)
	answer := func(command string) string {
		*sent = append(*sent, command)
		if strings.HasSuffix(command, "| raw") {
			return jsonAnswer
		}
		return "┌─────────┐\n│ rendered │\n└─────────┘"
	}
	send := func(_ sshclient.Credentials, command string) (string, error) {
		return answer(command), nil
	}
	// The streaming transport answers the same bytes to the same commands. A
	// caller that prints takes it, a caller that parses takes send, and the
	// recorded command list is shared so a test reads either one.
	stream := func(_ sshclient.Credentials, command string, body io.Writer) (sshclient.Answer, error) {
		if _, writeErr := io.WriteString(body, answer(command)); writeErr != nil {
			return sshclient.Answer{}, writeErr
		}
		return sshclient.Answer{Verdict: "done", Count: 1}, nil
	}
	return &cliClient{send: send, stream: stream}, sent
}

// TestBuildRuntimeTreeAsksForTheDispatcherJSON covers the `system command list`
// call site.
//
// VALIDATES: spec-fixit-cli-format-default-everywhere AC-9, AC-10 -- the runtime
// command tree is built from the daemon's answer even when the operator
// configured a display format.
// PREVENTS: the completion tree silently reverting to the compile-time static
// tree, which is what buildRuntimeTree does on a parse failure. Plugin commands
// then vanish from tab completion with no error anywhere.
func TestBuildRuntimeTreeAsksForTheDispatcherJSON(t *testing.T) {
	client, sent := rawOnlyJSONClient(`{"commands":[{"value":"zz-runtime-only","help":"proves the runtime answer was parsed"}]}`)

	tree := buildRuntimeTree(client)
	if tree == nil {
		t.Fatal("buildRuntimeTree returned nil")
	}
	if len(*sent) != 1 || (*sent)[0] != "system command list | raw" {
		t.Fatalf("buildRuntimeTree sent %q, want [\"system command list | raw\"]", *sent)
	}
	if _, ok := tree.Children["zz-runtime-only"]; !ok {
		t.Error("the runtime command list was not parsed: the tree fell back to the static one")
	}
}

// TestFetchPeerSelectorsAsksForTheDispatcherJSON covers the peer-completion call
// site.
//
// VALIDATES: spec-fixit-cli-format-default-everywhere AC-9, AC-10.
// PREVENTS: `peer <TAB>` offering nothing, which is how fetchPeerSelectors
// reports a parse failure.
func TestFetchPeerSelectorsAsksForTheDispatcherJSON(t *testing.T) {
	peerCache = peerSelectorCache{} // the TTL cache is package state; start cold.
	t.Cleanup(func() { peerCache = peerSelectorCache{} })

	client, sent := rawOnlyJSONClient(`{"peers":{"192.0.2.1":{"name":"transit-a"}}}`)

	suggestions := fetchPeerSelectors(client)
	if len(*sent) != 1 || (*sent)[0] != "show bgp peer list | raw" {
		t.Fatalf("fetchPeerSelectors sent %q, want [\"show bgp peer list | raw\"]", *sent)
	}
	var texts []string
	for _, s := range suggestions {
		texts = append(texts, s.Text)
	}
	if !slices.Contains(texts, "192.0.2.1") || !slices.Contains(texts, "transit-a") {
		t.Errorf("peer selectors = %q, want the address and the name from the answer", texts)
	}
}

// TestModelExecutorAsksForTheDispatcherJSON covers the interactive session's
// executor and the dashboard poller, the two call sites that hand their answer
// to internal/component/cli's Model.
//
// VALIDATES: spec-fixit-cli-format-default-everywhere AC-9, AC-10 -- both give
// the Model the dispatcher's JSON, which is what it renders from.
// PREVENTS: an interactive `show bgp peer list | json` answering the configured
// default, and the dashboard failing to parse its own poll.
func TestModelExecutorAsksForTheDispatcherJSON(t *testing.T) {
	const answer = `{"router-id":"192.0.2.1"}`

	client, sent := rawOnlyJSONClient(answer)
	output, err := client.modelExecutor()("show bgp")
	if err != nil {
		t.Fatalf("modelExecutor: %v", err)
	}
	if output.Text != answer {
		t.Errorf("modelExecutor answered %q, want the dispatcher's JSON %q", output.Text, answer)
	}
	if len(*sent) != 1 || (*sent)[0] != "show bgp | raw" {
		t.Errorf("modelExecutor sent %q, want [\"show bgp | raw\"]", *sent)
	}

	client, sent = rawOnlyJSONClient(answer)
	poll, err := client.dashboardPoller()
	if err != nil {
		t.Fatalf("dashboardPoller: %v", err)
	}
	got, err := poll()
	if err != nil {
		t.Fatalf("dashboard poll: %v", err)
	}
	if got != answer {
		t.Errorf("dashboard poll answered %q, want the dispatcher's JSON %q", got, answer)
	}
	if len(*sent) != 1 || (*sent)[0] != "show bgp | raw" {
		t.Errorf("dashboardPoller sent %q, want [\"show bgp | raw\"]", *sent)
	}
}

// TestExecuteKeepsTheOperatorSurface is the other half of the classification: a
// caller that PRINTS must not ask for raw, or `ze cli -c` would answer JSON and
// the whole spec would be undone.
//
// VALIDATES: spec-fixit-cli-format-default-everywhere AC-10 -- the human-output
// call sites are unchanged by this phase.
// PREVENTS: the raw helper spreading to every call site because it looked safer.
func TestExecuteKeepsTheOperatorSurface(t *testing.T) {
	client, sent := rawOnlyJSONClient(`{"version":"1.2.3"}`)

	var code int
	captureOutput(t, false, func() { code = client.Execute("show version", "") })
	if code != 0 {
		t.Fatalf("Execute exit = %d, want 0", code)
	}
	if len(*sent) != 1 || (*sent)[0] != "show version" {
		t.Errorf("Execute sent %q, want [\"show version\"]", *sent)
	}
}

// TestPipeHelpSectionsComeFromCatalog compares every rendered entry with its
// catalog class and local-process contract.
func TestPipeHelpSectionsComeFromCatalog(t *testing.T) {
	sections := pipeHelpSections()
	if len(sections) != 3 {
		t.Fatalf("pipe help sections = %d, want global, data, stream", len(sections))
	}

	catalog := command.PipeOperatorCatalog()
	entryCount := 0
	for _, section := range sections {
		entryCount += len(section.Entries)
	}
	if entryCount != len(catalog) {
		t.Fatalf("pipe help entries = %d, catalog = %d", entryCount, len(catalog))
	}
	for _, op := range catalog {
		var desc, title string
		for _, section := range sections {
			for _, entry := range section.Entries {
				if strings.Contains(entry.Name, "| "+op.Name) {
					desc = entry.Desc
					title = section.Title
				}
			}
		}
		if desc == "" {
			t.Errorf("help omits catalog operator %q", op.Name)
			continue
		}
		className := map[command.PipeClass]string{
			command.ClassGlobal: "Global",
			command.ClassData:   "Data",
			command.ClassStream: "Stream",
		}[op.Class]
		if !strings.HasPrefix(title, className) {
			t.Errorf("%s appears under %q, want %s class", op.Name, title, className)
		}
		hasLocalWording := strings.Contains(desc, "local process only")
		if hasLocalWording != op.LocalOnly {
			t.Errorf("%s local-only wording = %v, want %v", op.Name, hasLocalWording, op.LocalOnly)
		}
	}
}

func monitorTestClient(events []string, streamErr error) *cliClient {
	return &cliClient{
		monitorStream: func(_ sshclient.Credentials, _ string, callback func(string) error) error {
			for _, event := range events {
				if err := callback(event); err != nil {
					return err
				}
			}
			return streamErr
		},
	}
}

// TestStreamMonitorSavesDisplayedEventsInOrder drives two events through the
// default renderer and compares both atomic destinations with stdout.
func TestStreamMonitorSavesDisplayedEventsInOrder(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.out")
	secondPath := filepath.Join(dir, "second.out")
	events := []string{
		`{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"remote":{"address":"192.0.2.1","as":65001}},"state":"established"}}`,
		`{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"remote":{"address":"192.0.2.2","as":65002}},"state":"down"}}`,
	}
	client := monitorTestClient(events, nil)
	commandLine := "monitor event | save " + firstPath + " | save " + secondPath

	var code int
	shown := captureOutput(t, false, func() { code = client.StreamMonitor(commandLine) })
	if code != 0 {
		t.Fatalf("StreamMonitor exit = %d, want 0", code)
	}
	if shown == "" || shown == strings.Join(events, "\n")+"\n" {
		t.Fatalf("default renderer did not run before save: %q", shown)
	}
	firstOffset := strings.Index(shown, "192.0.2.1")
	secondOffset := strings.Index(shown, "192.0.2.2")
	if firstOffset < 0 || secondOffset <= firstOffset {
		t.Fatalf("displayed event order = %q", shown)
	}
	for _, path := range []string{firstPath, secondPath} {
		saved, err := os.ReadFile(path) //nolint:gosec // test-owned temp path
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(saved) != shown {
			t.Errorf("%s saved %q, terminal showed %q", path, saved, shown)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("%s mode = %o, want 600", path, info.Mode().Perm())
		}
	}
}

// TestStreamMonitorFailureKeepsDestinationAndRemovesTemp ends the transport
// after one event and inspects the destination directory.
func TestStreamMonitorFailureKeepsDestinationAndRemovesTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "answer.out")
	if err := os.WriteFile(path, []byte("previous"), 0o600); err != nil {
		t.Fatalf("seed destination: %v", err)
	}
	client := monitorTestClient([]string{`{"seq":1}`}, errors.New("transport failed"))

	var code int
	captureOutput(t, true, func() {
		captureOutput(t, false, func() {
			code = client.StreamMonitor("monitor event | raw | save " + path)
		})
	})
	if code != 1 {
		t.Fatalf("StreamMonitor exit = %d, want 1", code)
	}
	saved, err := os.ReadFile(path) //nolint:gosec // test-owned temp path
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(saved) != "previous" {
		t.Errorf("failed stream replaced destination with %q", saved)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read temp directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".ze-save-") {
			t.Errorf("failed stream left temporary file %s", entry.Name())
		}
	}
}
